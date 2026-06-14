package splunklite

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

var nginxAccessPattern = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "([^"]*)" (\d{3}) (\d+|-)`)
var nginxErrorPattern = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})`)

type LogIngestor struct {
	repo   *sqlite.LogEventRepository
	logDir string
}

func NewLogIngestor(repo *sqlite.LogEventRepository, logDir string) *LogIngestor {
	return &LogIngestor{repo: repo, logDir: logDir}
}

func (i *LogIngestor) Ingest(ctx context.Context) error {
	if i == nil || i.repo == nil || strings.TrimSpace(i.logDir) == "" {
		return nil
	}
	entries, err := os.ReadDir(i.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		source, site, ok := logSourceAndSite(entry.Name())
		if !ok {
			continue
		}
		if err := i.ingestFile(ctx, filepath.Join(i.logDir, entry.Name()), source, site); err != nil {
			return err
		}
	}
	return nil
}

func (i *LogIngestor) ingestFile(ctx context.Context, path, source, site string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		ev := parseLogLine(line, source, site)
		if err := i.repo.InsertIgnore(ctx, ev); err != nil {
			return err
		}
	}
	return scanner.Err()
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
				if bytes, err := strconv.Atoi(matches[5]); err == nil {
					ev.Bytes = &bytes
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
