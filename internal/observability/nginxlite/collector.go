package nginxlite

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

// HTTPDoer fetches stub_status (tests may stub).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Collector polls nginx stub_status and stores samples.
type Collector struct {
	statusURL string
	client    HTTPDoer
	repo      *sqlite.NginxStatusRepository
	retention int
	nowFn     func() time.Time
}

// NewCollector returns a stub_status collector.
func NewCollector(statusURL string, repo *sqlite.NginxStatusRepository, retentionDays int) *Collector {
	if retentionDays <= 0 {
		retentionDays = 14
	}
	return &Collector{
		statusURL: statusURL,
		client:    &http.Client{Timeout: 5 * time.Second},
		repo:      repo,
		retention: retentionDays,
		nowFn:     time.Now,
	}
}

// SetHTTPClient overrides the HTTP client (tests).
func (c *Collector) SetHTTPClient(client HTTPDoer) {
	if client != nil {
		c.client = client
	}
}

// SetNowFunc overrides time source (tests).
func (c *Collector) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		c.nowFn = fn
	}
}

// Collect fetches stub_status and inserts a sample.
func (c *Collector) Collect(ctx context.Context) error {
	if c.repo == nil || c.statusURL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.statusURL, nil)
	if err != nil {
		return fmt.Errorf("build stub_status request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch stub_status: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stub_status status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return fmt.Errorf("read stub_status: %w", err)
	}
	parsed, err := ParseStubStatus(string(body))
	if err != nil {
		return err
	}
	now := c.nowFn().UTC()
	if err := c.repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: now,
		Active:   parsed.Active,
		Accepts:  parsed.Accepts,
		Handled:  parsed.Handled,
		Requests: parsed.Requests,
		Reading:  parsed.Reading,
		Writing:  parsed.Writing,
		Waiting:  parsed.Waiting,
	}); err != nil {
		return err
	}
	return c.purgeRetention(ctx)
}

func (c *Collector) purgeRetention(ctx context.Context) error {
	if c.retention <= 0 || c.repo == nil {
		return nil
	}
	cutoff := c.nowFn().UTC().Add(-time.Duration(c.retention) * 24 * time.Hour)
	_, err := c.repo.PurgeOlderThan(ctx, cutoff)
	return err
}

// PurgeRetention removes samples older than the retention window.
func (c *Collector) PurgeRetention(ctx context.Context) error {
	return c.purgeRetention(ctx)
}
