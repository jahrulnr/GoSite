package nginxlite

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

// VTSCollector polls nginx VTS JSON and stores snapshots.
type VTSCollector struct {
	statusURL string
	client    HTTPDoer
	repo      *sqlite.NginxVTSRepository
	retention int
	nowFn     func() time.Time
}

// NewVTSCollector returns a VTS metrics collector.
func NewVTSCollector(statusURL string, repo *sqlite.NginxVTSRepository, retentionDays int) *VTSCollector {
	if retentionDays <= 0 {
		retentionDays = 14
	}
	return &VTSCollector{
		statusURL: statusURL,
		client:    &http.Client{Timeout: 5 * time.Second},
		repo:      repo,
		retention: retentionDays,
		nowFn:     time.Now,
	}
}

// SetHTTPClient overrides the HTTP client (tests).
func (c *VTSCollector) SetHTTPClient(client HTTPDoer) {
	if client != nil {
		c.client = client
	}
}

// SetNowFunc overrides time source (tests).
func (c *VTSCollector) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		c.nowFn = fn
	}
}

// Collect fetches VTS JSON and inserts samples.
func (c *VTSCollector) Collect(ctx context.Context) error {
	if c.repo == nil || c.statusURL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.statusURL, nil)
	if err != nil {
		return fmt.Errorf("build vts request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch vts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vts status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return fmt.Errorf("read vts: %w", err)
	}
	parsed, err := ParseVTSJSON(body)
	if err != nil {
		return err
	}
	now := c.nowFn().UTC()
	for _, row := range parsed.Servers {
		if err := c.repo.InsertServerSample(ctx, sqlite.VTSServerSample{
			SampleTS:    now,
			ServerName:  row.ServerName,
			Requests:    row.Requests,
			InBytes:     row.InBytes,
			OutBytes:    row.OutBytes,
			RequestMsec: row.RequestMsec,
		}); err != nil {
			return err
		}
	}
	for _, row := range parsed.Upstreams {
		if err := c.repo.InsertUpstreamSample(ctx, sqlite.VTSUpstreamSample{
			SampleTS:     now,
			UpstreamName: row.UpstreamName,
			ServerAddr:   row.ServerAddr,
			Requests:     row.Requests,
			InBytes:      row.InBytes,
			OutBytes:     row.OutBytes,
			ResponseMsec: row.ResponseMsec,
			Down:         row.Down,
		}); err != nil {
			return err
		}
	}
	return c.purgeRetention(ctx)
}

func (c *VTSCollector) purgeRetention(ctx context.Context) error {
	if c.retention <= 0 || c.repo == nil {
		return nil
	}
	cutoff := c.nowFn().UTC().Add(-time.Duration(c.retention) * 24 * time.Hour)
	_, err := c.repo.PurgeOlderThan(ctx, cutoff)
	return err
}

// PurgeRetention removes samples older than the retention window.
func (c *VTSCollector) PurgeRetention(ctx context.Context) error {
	return c.purgeRetention(ctx)
}
