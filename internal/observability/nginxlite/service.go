package nginxlite

import (
	"context"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service exposes nginx stub_status and VTS metrics for the panel.
type Service struct {
	repo    *sqlite.NginxStatusRepository
	vtsRepo *sqlite.NginxVTSRepository
	vtsURL  string
	nowFn   func() time.Time
}

// NewService returns an nginx metrics service.
func NewService(repo *sqlite.NginxStatusRepository, vtsRepo *sqlite.NginxVTSRepository, vtsURL string) *Service {
	return &Service{
		repo:    repo,
		vtsRepo: vtsRepo,
		vtsURL:  strings.TrimSpace(vtsURL),
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

// CurrentResponse is returned by GET /metrics/nginx/current.
type CurrentResponse struct {
	SampleTS            string   `json:"sample_ts"`
	Active              int      `json:"active"`
	Reading             int      `json:"reading"`
	Writing             int      `json:"writing"`
	Waiting             int      `json:"waiting"`
	Accepts             int64    `json:"accepts"`
	Handled             int64    `json:"handled"`
	Requests            int64    `json:"requests"`
	DroppedConnections  int64    `json:"dropped_connections"`
	RequestRatePerSec   *float64 `json:"request_rate_per_sec"`
	AcceptRatePerSec    *float64 `json:"accept_rate_per_sec"`
	HandledRatePerSec   *float64 `json:"handled_rate_per_sec"`
	CounterReset        bool     `json:"counter_reset"`
	Available           bool     `json:"available"`
}

// SeriesResponse is returned by GET /metrics/nginx/series.
type SeriesResponse struct {
	Step          string                   `json:"step"`
	Active        []SeriesPoint            `json:"active"`
	Reading       []SeriesPoint            `json:"reading"`
	Writing       []SeriesPoint            `json:"writing"`
	Waiting       []SeriesPoint            `json:"waiting"`
	RequestRate   []SeriesPoint            `json:"request_rate"`
}

// VTSStatusResponse describes VTS availability.
type VTSStatusResponse struct {
	Enabled bool   `json:"enabled"`
	Hint    string `json:"hint,omitempty"`
}

// VTSServerRowResponse is returned by GET /metrics/nginx/vts/servers.
type VTSServerRowResponse struct {
	ServerName  string  `json:"server_name"`
	Requests    int     `json:"requests"`
	InBytes     int     `json:"in_bytes"`
	OutBytes    int     `json:"out_bytes"`
	RequestMsec float64 `json:"request_msec"`
	SampleTS    string  `json:"sample_ts,omitempty"`
}

// VTSUpstreamRowResponse is returned by GET /metrics/nginx/vts/upstreams.
type VTSUpstreamRowResponse struct {
	UpstreamName string  `json:"upstream_name"`
	ServerAddr   string  `json:"server_addr"`
	Requests     int     `json:"requests"`
	InBytes      int     `json:"in_bytes"`
	OutBytes     int     `json:"out_bytes"`
	ResponseMsec float64 `json:"response_msec"`
	Down         bool    `json:"down"`
	SampleTS     string  `json:"sample_ts,omitempty"`
}

// Current returns the latest sample and request rate vs the previous sample.
func (s *Service) Current(ctx context.Context) (CurrentResponse, error) {
	if s.repo == nil {
		return CurrentResponse{Available: false}, nil
	}
	latest, ok, err := s.repo.LatestSample(ctx)
	if err != nil {
		return CurrentResponse{}, apperror.Wrap(apperror.CodeDatabase, "latest nginx status", err)
	}
	if !ok {
		return CurrentResponse{Available: false}, nil
	}
	var reqRate, acceptRate, handledRate *float64
	counterReset := false
	if prev, hasPrev, err := s.repo.PreviousSample(ctx, latest.SampleTS); err != nil {
		return CurrentResponse{}, apperror.Wrap(apperror.CodeDatabase, "previous nginx status", err)
	} else if hasPrev {
		req, accept, handled := ratesFromPair(prev, latest)
		if req.reset || accept.reset || handled.reset {
			counterReset = true
		} else {
			reqRate = ratePtr(req)
			acceptRate = ratePtr(accept)
			handledRate = ratePtr(handled)
		}
	}
	return CurrentResponse{
		SampleTS:           latest.SampleTS.UTC().Format(time.RFC3339),
		Active:             latest.Active,
		Reading:            latest.Reading,
		Writing:            latest.Writing,
		Waiting:            latest.Waiting,
		Accepts:            latest.Accepts,
		Handled:            latest.Handled,
		Requests:           latest.Requests,
		DroppedConnections: latest.Accepts - latest.Handled,
		RequestRatePerSec:  reqRate,
		AcceptRatePerSec:   acceptRate,
		HandledRatePerSec:  handledRate,
		CounterReset:       counterReset,
		Available:          true,
	}, nil
}

// Series returns connection and request-rate time series for the requested range.
func (s *Service) Series(ctx context.Context, rangeSpec string) (SeriesResponse, error) {
	from, to, step, err := parseRange(rangeSpec, s.nowFn())
	if err != nil {
		return SeriesResponse{}, err
	}
	samples, err := s.repo.ListSamples(ctx, from, to)
	if err != nil {
		return SeriesResponse{}, apperror.Wrap(apperror.CodeDatabase, "list nginx status samples", err)
	}
	out := SeriesResponse{Step: step}
	for i, sample := range samples {
		ts := sample.SampleTS.UTC().Format(time.RFC3339)
		out.Active = append(out.Active, SeriesPoint{ts, sample.Active})
		out.Reading = append(out.Reading, SeriesPoint{ts, sample.Reading})
		out.Writing = append(out.Writing, SeriesPoint{ts, sample.Writing})
		out.Waiting = append(out.Waiting, SeriesPoint{ts, sample.Waiting})
		if i == 0 {
			zero := 0.0
			out.RequestRate = append(out.RequestRate, SeriesPoint{ts, zero})
			continue
		}
		r := counterRate(samples[i-1], sample, func(s sqlite.NginxStatusSample) int64 { return s.Requests })
		if r.reset || !r.ok {
			out.RequestRate = append(out.RequestRate, SeriesPoint{ts, nil})
		} else {
			out.RequestRate = append(out.RequestRate, SeriesPoint{ts, round2(r.value)})
		}
	}
	return out, nil
}

// VTSStatus reports whether VTS collection is enabled.
func (s *Service) VTSStatus() VTSStatusResponse {
	if s.vtsURL != "" {
		return VTSStatusResponse{Enabled: true}
	}
	return VTSStatusResponse{
		Enabled: false,
		Hint:    "VTS requires a custom nginx build with nginx-module-vts. See docs/implementation/WAVE-SA-8.md.",
	}
}

// VTSServers returns ranked server zones from the latest VTS snapshot.
func (s *Service) VTSServers(ctx context.Context, limit int) ([]VTSServerRowResponse, error) {
	if s.vtsRepo == nil || s.vtsURL == "" {
		return []VTSServerRowResponse{}, nil
	}
	rows, err := s.vtsRepo.TopServersAtLatest(ctx, limit)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "vts servers", err)
	}
	out := make([]VTSServerRowResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, VTSServerRowResponse{
			ServerName:  row.ServerName,
			Requests:    row.Requests,
			InBytes:     row.InBytes,
			OutBytes:    row.OutBytes,
			RequestMsec: round2(row.RequestMsec),
			SampleTS:    row.SampleTS.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// VTSUpstreams returns ranked upstream peers from the latest VTS snapshot.
func (s *Service) VTSUpstreams(ctx context.Context, limit int) ([]VTSUpstreamRowResponse, error) {
	if s.vtsRepo == nil || s.vtsURL == "" {
		return []VTSUpstreamRowResponse{}, nil
	}
	rows, err := s.vtsRepo.TopUpstreamsAtLatest(ctx, limit)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "vts upstreams", err)
	}
	out := make([]VTSUpstreamRowResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, VTSUpstreamRowResponse{
			UpstreamName: row.UpstreamName,
			ServerAddr:   row.ServerAddr,
			Requests:     row.Requests,
			InBytes:      row.InBytes,
			OutBytes:     row.OutBytes,
			ResponseMsec: round2(row.ResponseMsec),
			Down:         row.Down,
			SampleTS:     row.SampleTS.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

type rateResult struct {
	value float64
	reset bool
	ok    bool
}

func counterRate(prev, latest sqlite.NginxStatusSample, counter func(sqlite.NginxStatusSample) int64) rateResult {
	delta := float64(counter(latest) - counter(prev))
	if delta < 0 {
		return rateResult{reset: true}
	}
	secs := latest.SampleTS.Sub(prev.SampleTS).Seconds()
	if secs <= 0 {
		return rateResult{}
	}
	return rateResult{value: delta / secs, ok: true}
}

func ratesFromPair(prev, latest sqlite.NginxStatusSample) (req, accept, handled rateResult) {
	return counterRate(prev, latest, func(s sqlite.NginxStatusSample) int64 { return s.Requests }),
		counterRate(prev, latest, func(s sqlite.NginxStatusSample) int64 { return s.Accepts }),
		counterRate(prev, latest, func(s sqlite.NginxStatusSample) int64 { return s.Handled })
}

func ratePtr(r rateResult) *float64 {
	if !r.ok {
		return nil
	}
	v := round2(r.value)
	return &v
}

func parseRange(spec string, now time.Time) (from, to time.Time, step string, err error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" {
		spec = "1h"
	}
	to = now.UTC()
	step = "30s"
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

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
