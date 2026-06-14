package grafanalite

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

const bucketDuration = 5 * time.Minute

var (
	combinedLogRE = regexp.MustCompile(`"[^"]*"\s+(\d{3})\s+(\d+|-)\s+"`)
	nginxTimeRE   = regexp.MustCompile(`\[([^\]]+)\]`)
)

// OffsetState tracks byte offsets per access log file.
type OffsetState struct {
	Files map[string]int64 `json:"files"`
}

// Collector incrementally parses nginx access logs into 5-minute buckets.
type Collector struct {
	logDir      string
	offsetPath  string
	metrics     *sqlite.TrafficMetricsRepository
	retention   int
	nowFn       func() time.Time
}

// NewCollector returns a traffic metrics collector.
func NewCollector(logDir, offsetPath string, metrics *sqlite.TrafficMetricsRepository, retentionDays int) *Collector {
	if retentionDays <= 0 {
		retentionDays = 14
	}
	return &Collector{
		logDir:     logDir,
		offsetPath: offsetPath,
		metrics:    metrics,
		retention:  retentionDays,
		nowFn:      time.Now,
	}
}

// SetNowFunc overrides time source (tests).
func (c *Collector) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		c.nowFn = fn
	}
}

// Collect reads new log bytes since the saved offset and upserts buckets.
func (c *Collector) Collect(ctx context.Context) error {
	offsets, err := c.loadOffsets()
	if err != nil {
		return err
	}
	if offsets.Files == nil {
		offsets.Files = map[string]int64{}
	}

	entries, err := os.ReadDir(c.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read log dir: %w", err)
	}

	buckets := map[bucketKey]*sqlite.TrafficBucket{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "access") || !strings.HasSuffix(name, ".log") {
			continue
		}
		path := filepath.Join(c.logDir, name)
		site := siteFromAccessLogName(name)
		offset := offsets.Files[name]

		newOffset, parsed, err := c.readFromOffset(path, offset, site, buckets)
		if err != nil {
			return err
		}
		offsets.Files[name] = newOffset
		_ = parsed
	}

	for _, b := range buckets {
		if err := c.metrics.UpsertBucket(ctx, *b); err != nil {
			return err
		}
	}

	if err := c.saveOffsets(offsets); err != nil {
		return err
	}
	return c.purgeRetention(ctx)
}

type bucketKey struct {
	ts   time.Time
	site string
}

func (c *Collector) readFromOffset(path string, offset int64, site string, buckets map[bucketKey]*sqlite.TrafficBucket) (int64, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return offset, 0, nil
		}
		return offset, 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return offset, 0, err
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return offset, 0, err
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	parsed := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		ts, status, bytes, ok := parseAccessLine(line)
		if !ok {
			continue
		}
		key := bucketKey{ts: truncateBucket(ts), site: site}
		b := buckets[key]
		if b == nil {
			b = &sqlite.TrafficBucket{BucketTS: key.ts, Site: site}
			buckets[key] = b
		}
		b.Requests++
		b.Bytes += bytes
		switch {
		case status >= 200 && status < 300:
			b.S2xx++
		case status >= 300 && status < 400:
			b.S3xx++
		case status >= 400 && status < 500:
			b.S4xx++
		case status >= 500:
			b.S5xx++
		}
		parsed++
	}
	if err := scanner.Err(); err != nil {
		return offset, parsed, err
	}
	newOffset, err := f.Seek(0, 1)
	if err != nil {
		return offset, parsed, err
	}
	return newOffset, parsed, nil
}

func parseAccessLine(line string) (ts time.Time, status, bytes int, ok bool) {
	m := nginxTimeRE.FindStringSubmatch(line)
	if len(m) < 2 {
		return time.Time{}, 0, 0, false
	}
	parsedTS, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[1])
	if err != nil {
		return time.Time{}, 0, 0, false
	}
	sm := combinedLogRE.FindStringSubmatch(line)
	if len(sm) < 3 {
		return parsedTS, 0, 0, true
	}
	status = atoi(sm[1])
	if sm[2] != "-" {
		bytes = atoi(sm[2])
	}
	return parsedTS, status, bytes, true
}

func truncateBucket(ts time.Time) time.Time {
	ts = ts.UTC()
	min := ts.Minute() - (ts.Minute() % 5)
	return time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), min, 0, 0, time.UTC)
}

func siteFromAccessLogName(name string) string {
	base := strings.TrimSuffix(name, ".log")
	base = strings.TrimPrefix(base, "access")
	base = strings.TrimPrefix(base, "-")
	if base == "" {
		return "default"
	}
	return base
}

func (c *Collector) loadOffsets() (OffsetState, error) {
	data, err := os.ReadFile(c.offsetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return OffsetState{Files: map[string]int64{}}, nil
		}
		return OffsetState{}, err
	}
	var state OffsetState
	if err := json.Unmarshal(data, &state); err != nil {
		return OffsetState{}, fmt.Errorf("parse offsets: %w", err)
	}
	if state.Files == nil {
		state.Files = map[string]int64{}
	}
	return state, nil
}

func (c *Collector) saveOffsets(state OffsetState) error {
	if err := os.MkdirAll(filepath.Dir(c.offsetPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.offsetPath, data, 0o644)
}

func (c *Collector) purgeRetention(ctx context.Context) error {
	if c.retention <= 0 || c.metrics == nil {
		return nil
	}
	cutoff := c.nowFn().UTC().Add(-time.Duration(c.retention) * 24 * time.Hour)
	_, err := c.metrics.PurgeOlderThan(ctx, cutoff)
	return err
}

// PurgeRetention removes metrics buckets older than the retention window.
func (c *Collector) PurgeRetention(ctx context.Context) error {
	return c.purgeRetention(ctx)
}

// LoadOffsets exposes offset state for tests.
func (c *Collector) LoadOffsets() (OffsetState, error) {
	return c.loadOffsets()
}

// TruncateBucket exposes bucket rounding for tests.
func TruncateBucket(ts time.Time) time.Time {
	return truncateBucket(ts)
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
