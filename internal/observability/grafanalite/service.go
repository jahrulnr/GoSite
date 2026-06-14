package grafanalite

import (
	"context"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service exposes pre-aggregated traffic metrics for charting.
type Service struct {
	metrics *sqlite.TrafficMetricsRepository
	nowFn   func() time.Time
}

// NewService returns a Grafana Lite metrics service.
func NewService(metrics *sqlite.TrafficMetricsRepository) *Service {
	return &Service{
		metrics: metrics,
		nowFn:   time.Now,
	}
}

// SetNowFunc overrides time source (tests).
func (s *Service) SetNowFunc(fn func() time.Time) {
	if fn != nil {
		s.nowFn = fn
	}
}

// SeriesPoint is a [timestamp, value] pair for chart libraries.
type SeriesPoint = []interface{}

// SeriesResponse is returned by the traffic series endpoint.
type SeriesResponse struct {
	Step     string                   `json:"step"`
	Requests map[string][]SeriesPoint `json:"requests"`
	Bytes    map[string][]SeriesPoint `json:"bytes"`
}

// TopSiteRow is a ranked site entry.
type TopSiteRow struct {
	Site     string `json:"site"`
	Requests int    `json:"requests"`
	Bytes    int    `json:"bytes"`
}

// StatusCodesResponse breaks down HTTP status families.
type StatusCodesResponse struct {
	S2xx int `json:"s2xx"`
	S3xx int `json:"s3xx"`
	S4xx int `json:"s4xx"`
	S5xx int `json:"s5xx"`
}

// SummaryResponse is a dashboard snapshot.
type SummaryResponse struct {
	Range    string `json:"range"`
	Requests int    `json:"requests"`
	Bytes    int    `json:"bytes"`
}

// TrafficSeries returns request/byte time series for the requested range.
func (s *Service) TrafficSeries(ctx context.Context, rangeSpec, site string) (SeriesResponse, error) {
	from, to, step, err := parseRange(rangeSpec, s.nowFn())
	if err != nil {
		return SeriesResponse{}, err
	}
	buckets, err := s.metrics.ListBuckets(ctx, from, to, site)
	if err != nil {
		return SeriesResponse{}, apperror.Wrap(apperror.CodeDatabase, "list traffic buckets", err)
	}

	reqSeries := map[string][]SeriesPoint{}
	byteSeries := map[string][]SeriesPoint{}
	for _, b := range buckets {
		ts := b.BucketTS.UTC().Format(time.RFC3339)
		reqSeries[b.Site] = append(reqSeries[b.Site], SeriesPoint{ts, b.Requests})
		byteSeries[b.Site] = append(byteSeries[b.Site], SeriesPoint{ts, b.Bytes})
	}
	return SeriesResponse{
		Step:     step,
		Requests: reqSeries,
		Bytes:    byteSeries,
	}, nil
}

// TopSites returns ranked sites by request volume.
func (s *Service) TopSites(ctx context.Context, rangeSpec string, limit int) ([]TopSiteRow, error) {
	from, to, _, err := parseRange(rangeSpec, s.nowFn())
	if err != nil {
		return nil, err
	}
	rows, err := s.metrics.TopSites(ctx, from, to, limit)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "top sites", err)
	}
	out := make([]TopSiteRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, TopSiteRow{Site: row.Site, Requests: row.Requests, Bytes: row.Bytes})
	}
	return out, nil
}

// StatusCodes returns aggregated status code families.
func (s *Service) StatusCodes(ctx context.Context, rangeSpec, site string) (StatusCodesResponse, error) {
	from, to, _, err := parseRange(rangeSpec, s.nowFn())
	if err != nil {
		return StatusCodesResponse{}, err
	}
	s2, s3, s4, s5, err := s.metrics.StatusCodeTotals(ctx, from, to, site)
	if err != nil {
		return StatusCodesResponse{}, apperror.Wrap(apperror.CodeDatabase, "status code totals", err)
	}
	return StatusCodesResponse{S2xx: s2, S3xx: s3, S4xx: s4, S5xx: s5}, nil
}

// Summary returns total requests and bytes for dashboard cards.
func (s *Service) Summary(ctx context.Context, rangeSpec string) (SummaryResponse, error) {
	from, to, _, err := parseRange(rangeSpec, s.nowFn())
	if err != nil {
		return SummaryResponse{}, err
	}
	requests, bytes, err := s.metrics.SummaryTotals(ctx, from, to)
	if err != nil {
		return SummaryResponse{}, apperror.Wrap(apperror.CodeDatabase, "summary totals", err)
	}
	return SummaryResponse{
		Range:    strings.TrimSpace(rangeSpec),
		Requests: requests,
		Bytes:    bytes,
	}, nil
}

func parseRange(spec string, now time.Time) (from, to time.Time, step string, err error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" {
		spec = "1h"
	}
	to = now.UTC()
	step = "5m"
	switch spec {
	case "1h":
		from = to.Add(-1 * time.Hour)
	case "6h":
		from = to.Add(-6 * time.Hour)
	case "24h", "1d":
		from = to.Add(-24 * time.Hour)
	case "7d":
		from = to.Add(-7 * 24 * time.Hour)
	default:
		return time.Time{}, time.Time{}, "", apperror.New(apperror.CodeInvalidInput, "invalid range")
	}
	return from, to, step, nil
}
