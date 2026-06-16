package remote

import (
	"context"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/fetch"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/resolver"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service orchestrates resolve and fetch for wave G.
type Service struct {
	cfg      types.Config
	resolver *resolver.Registry
	fetcher  *fetch.Fetcher
}

// NewService returns a remote install orchestrator.
func NewService(cfg types.Config) *Service {
	return &Service{
		cfg:      cfg,
		resolver: resolver.NewRegistry(),
		fetcher: fetch.New(fetch.Config{
			AllowedHosts: cfg.AllowedHosts,
			MaxBytes:     cfg.MaxBytes,
			Timeout:      cfg.Timeout,
			MaxRedirects: cfg.MaxRedirects,
		}),
	}
}

// Resolve returns a lightweight preview without downloading the full zip.
func (s *Service) Resolve(ctx context.Context, source types.Source) (types.ResolvePreview, error) {
	if !s.cfg.Enabled {
		return types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.RemoteInstallDisabled)
	}
	_, preview, err := s.resolver.Resolve(ctx, source)
	if err != nil {
		return types.ResolvePreview{}, err
	}
	if size, err := s.fetcher.HeadSize(ctx, preview.URL); err == nil && size > 0 {
		preview.Size = size
	}
	return preview, nil
}

// Fetch downloads artifact bytes for a resolved plan.
func (s *Service) Fetch(ctx context.Context, plan types.FetchPlan) ([]byte, error) {
	if !s.cfg.Enabled {
		return nil, apperror.New(apperror.CodeInvalidInput, failures.RemoteInstallDisabled)
	}
	return s.fetcher.Fetch(ctx, plan.URL, plan.SHA256)
}

// ResolveAndFetch resolves source then downloads bytes.
func (s *Service) ResolveAndFetch(ctx context.Context, source types.Source) (types.FetchPlan, []byte, error) {
	plan, _, err := s.resolver.Resolve(ctx, source)
	if err != nil {
		return types.FetchPlan{}, nil, err
	}
	data, err := s.Fetch(ctx, plan)
	if err != nil {
		return plan, nil, err
	}
	return plan, data, nil
}

// SourceType normalizes source type string.
func SourceType(source types.Source) string {
	return strings.ToLower(strings.TrimSpace(source.Type))
}
