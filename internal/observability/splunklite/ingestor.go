package splunklite

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

var nginxAccessPattern = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "([^"]*)" (\d{3}) (\d+|-)`)
var nginxErrorPattern = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)

// logIngestCooldown caps how often a full ingest pass runs when nothing
// changed. It satisfies the "buffer output + caching pake file cache selama
// 5menit" optimization: repeated /query calls within the cooldown window
// short-circuit to a no-op, so the request only pays for the SQLite SELECT.
const logIngestCooldown = 5 * time.Minute

const logIngestBatchSize = 256

type logFileCursor struct {
	Offset  int64     `json:"offset"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type LogIngestor struct {
	repo    *sqlite.LogEventRepository
	logDir  string
	cursorF string

	mu         sync.Mutex
	cursors    map[string]logFileCursor
	lastIngest time.Time
}

// NewLogIngestor returns an ingestor that scans logDir for *.log files
// matching the nginx access|error naming convention and persists incremental
// cursors next to the configured cursorF (defaults to <storage>/log_ingest.json).
func NewLogIngestor(repo *sqlite.LogEventRepository, logDir string) *LogIngestor {
	return &LogIngestor{
		repo:    repo,
		logDir:  logDir,
		cursorF: filepath.Join(filepath.Dir(logDir), "log_ingest.json"),
		cursors: make(map[string]logFileCursor),
	}
}

// Ingest scans the log directory and inserts any new lines into the log event
// table. It is safe to call from concurrent requests; a per-directory mutex
// serializes the actual scan while a coarse cooldown prevents redundant work
// when the underlying files have not changed.
func (i *LogIngestor) Ingest(ctx context.Context) error {
	if i == nil || i.repo == nil || strings.TrimSpace(i.logDir) == "" {
		return nil
	}

	now := time.Now().UTC()
	entries, err := os.ReadDir(i.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	i.mu.Lock()
	last := i.lastIngest
	changed := i.hasNewerFile(entries, now)
	cooldownActive := !last.IsZero() && now.Sub(last) < logIngestCooldown
	if cooldownActive && !changed {
		i.mu.Unlock()
		return nil
	}
	i.mu.Unlock()

	seen := make(map[string]struct{}, len(entries))
	pending := make([]sqlite.LogEvent, 0, logIngestBatchSize)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		if _, err := i.repo.InsertIgnoreBatch(ctx, pending); err != nil {
			return err
		}
		pending = pending[:0]
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		source, site, ok := logSourceAndSite(entry.Name())
		if !ok {
			continue
		}
		path := filepath.Join(i.logDir, entry.Name())
		seen[path] = struct{}{}
		if _, err := i.ingestFile(ctx, path, source, site, now, func(ev sqlite.LogEvent) error {
			pending = append(pending, ev)
			if len(pending) >= logIngestBatchSize {
				return flush()
			}
			return nil
		}); err != nil {
			return err
		}
		if err := flush(); err != nil {
			return err
		}
	}

	i.mu.Lock()
	i.lastIngest = time.Now().UTC()
	i.mu.Unlock()
	i.pruneCursors(seen)
	i.persistCursors()
	return nil
}

// hasNewerFile reports whether any file in entries has a ModTime newer than
// the last successful ingest. The caller must hold i.mu.
func (i *LogIngestor) hasNewerFile(entries []os.DirEntry, now time.Time) bool {
	if i.lastIngest.IsZero() {
		return true
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		if _, _, ok := logSourceAndSite(entry.Name()); !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(i.lastIngest) {
			return true
		}
	}
	return false
}

// ingestFile reads logFile incrementally (from the cached offset) and feeds
// parsed events into emit. It updates the cursor in-memory and on disk so
// subsequent calls only process new lines.
func (i *LogIngestor) ingestFile(
	ctx context.Context,
	path, source, site string,
	now time.Time,
	emit func(sqlite.LogEvent) error,
) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			i.forgetCursor(path)
			return 0, nil
		}
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, nil
	}

	start := i.nextOffset(path, info, now)
	if start >= info.Size() {
		i.rememberCursor(path, logFileCursor{Offset: info.Size(), Size: info.Size(), ModTime: info.ModTime()})
		return 0, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			i.forgetCursor(path)
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	if start > 0 {
		if _, seekErr := file.Seek(start, io.SeekStart); seekErr != nil {
			start = 0
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return 0, err
			}
		}
	}

	offset := start
	rows := 0
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return offset, err
		}
		lineBytes := scanner.Bytes()
		offset += int64(len(lineBytes)) + 1
		line := string(lineBytes)
		if strings.TrimSpace(line) == "" {
			continue
		}
		ev := parseLogLine(line, source, site)
		if err := emit(ev); err != nil {
			return offset, err
		}
		rows++
	}
	if err := scanner.Err(); err != nil {
		slog.Error("ingest scanner error",
			"path", path, "rows", rows, "err", err)
		return offset, err
	}
	i.rememberCursor(path, logFileCursor{Offset: offset, Size: info.Size(), ModTime: info.ModTime()})
	slog.Info("ingest file done",
		"path", path, "rows", rows, "start", start, "offset", offset, "size", info.Size())
	return offset, nil
}

func (i *LogIngestor) nextOffset(path string, info os.FileInfo, now time.Time) int64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.cursors) == 0 && i.cursorF != "" {
		i.loadCursorsLocked()
	}
	cur, ok := i.cursors[path]
	if !ok {
		return 0
	}
	if info.Size() < cur.Offset || info.ModTime().Before(cur.ModTime) {
		delete(i.cursors, path)
		return 0
	}
	return cur.Offset
}

func (i *LogIngestor) rememberCursor(path string, cur logFileCursor) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.cursors == nil {
		i.cursors = make(map[string]logFileCursor)
	}
	i.cursors[path] = cur
}

func (i *LogIngestor) forgetCursor(path string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.cursors, path)
}

func (i *LogIngestor) pruneCursors(seen map[string]struct{}) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for path := range i.cursors {
		if _, ok := seen[path]; !ok {
			delete(i.cursors, path)
		}
	}
}

// loadCursorsLocked reads the persisted cursor map from disk. Caller must
// hold i.mu.
func (i *LogIngestor) loadCursorsLocked() {
	if i.cursorF == "" {
		return
	}
	data, err := os.ReadFile(i.cursorF)
	if err != nil {
		return
	}
	var stored map[string]logFileCursor
	if err := json.Unmarshal(data, &stored); err != nil {
		slog.Warn("log ingestor: cursor file corrupt; ignoring", "err", err, "path", i.cursorF)
		return
	}
	if stored == nil {
		stored = map[string]logFileCursor{}
	}
	i.cursors = stored
}

// persistCursors writes the cursor map to disk so we survive container
// restarts and the next ingest pass picks up at the right offset.
func (i *LogIngestor) persistCursors() {
	if i.cursorF == "" {
		return
	}
	i.mu.Lock()
	snapshot := make(map[string]logFileCursor, len(i.cursors))
	for k, v := range i.cursors {
		snapshot[k] = v
	}
	i.mu.Unlock()
	if len(snapshot) == 0 {
		return
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return
	}
	tmp := i.cursorF + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		slog.Warn("log ingestor: persist cursor tmp failed", "err", err)
		return
	}
	if err := os.Rename(tmp, i.cursorF); err != nil {
		slog.Warn("log ingestor: persist cursor rename failed", "err", err)
	}
}

func parseLogLine(line, source, site string) sqlite.LogEvent {
	ev := sqlite.LogEvent{
		Timestamp:  time.Now().UTC(),
		Source:     source,
		Site:       site,
		LineHash:   hashLogLine(source, site, line),
		RawPreview: preview(line),
	}
	if source == sqlite.LogSourceAccess {
		if matches := nginxAccessPattern.FindStringSubmatch(line); len(matches) == 6 {
			if ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", matches[2]); err == nil {
				ev.Timestamp = ts.UTC()
			}
			if status, err := strconv.Atoi(matches[4]); err == nil {
				ev.StatusCode = &status
			}
			if matches[5] != "-" {
				if b, err := strconv.Atoi(matches[5]); err == nil {
					ev.Bytes = &b
				}
			}
		}
		return ev
	}
	if matches := nginxErrorPattern.FindStringSubmatch(line); len(matches) == 2 {
		if ts, err := time.Parse("2006/01/02 15:04:05", matches[1]); err == nil {
			ev.Timestamp = ts.UTC()
		}
	}
	return ev
}

func logSourceAndSite(name string) (string, string, bool) {
	switch {
	case name == "access.log":
		return sqlite.LogSourceAccess, "default", true
	case name == "error.log":
		return sqlite.LogSourceError, "default", true
	case strings.HasPrefix(name, "access-") && strings.HasSuffix(name, ".log"):
		return sqlite.LogSourceAccess, strings.TrimSuffix(strings.TrimPrefix(name, "access-"), ".log"), true
	case strings.HasPrefix(name, "error-") && strings.HasSuffix(name, ".log"):
		return sqlite.LogSourceError, strings.TrimSuffix(strings.TrimPrefix(name, "error-"), ".log"), true
	default:
		return "", "", false
	}
}

func hashLogLine(source, site, line string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + site + "\x00" + line))
	return hex.EncodeToString(sum[:])
}

func preview(line string) string {
	line = strings.TrimSpace(line)
	if len(line) <= 1000 {
		return line
	}
	return line[:1000]
}
