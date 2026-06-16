package resolver

import (
	"context"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Resolver turns a Source into a FetchPlan without downloading the full artifact.
type Resolver interface {
	Supports(source types.Source) bool
	Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error)
}

// URLResolver handles source.type=url (G1).
type URLResolver struct{}

func (URLResolver) Supports(source types.Source) bool {
	return strings.EqualFold(strings.TrimSpace(source.Type), "url")
}

func (URLResolver) Resolve(_ context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	rawURL := strings.TrimSpace(source.URL)
	sha := strings.ToLower(strings.TrimSpace(source.SHA256))
	if rawURL == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	if sha == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "sha256 required for url source")
	}
	plan := types.FetchPlan{
		URL:            rawURL,
		SHA256:         sha,
		SourceType:     "url",
		SourceRef:      rawURL,
		InstallPath:    "release",
		ResolvedDigest: sha,
	}
	preview := types.ResolvePreview{
		SHA256:      sha,
		URL:         rawURL,
		SourceType:  "url",
		SourceRef:   rawURL,
		InstallPath: "release",
	}
	return plan, preview, nil
}

// Registry dispatches to the first matching resolver.
type Registry struct {
	resolvers []Resolver
}

// NewRegistry returns a resolver registry with wave G source types.
func NewRegistry(cfg types.Config, extra ...Resolver) *Registry {
	gh := GitHubResolver{Token: cfg.GitHubToken, Timeout: cfg.Timeout, BuildEnabled: cfg.BuildEnabled}
	gl := GitLabResolver{Token: cfg.GitLabToken, Timeout: cfg.Timeout, BuildEnabled: cfg.BuildEnabled}
	base := []Resolver{
		URLResolver{},
		gh,
		gl,
		GitHubBuildResolver{GitHub: gh, BuildEnabled: cfg.BuildEnabled},
		GitLabBuildResolver{GitLab: gl, BuildEnabled: cfg.BuildEnabled},
		GitRefResolver{GitHub: gh, GitLab: gl},
	}
	return &Registry{resolvers: append(base, extra...)}
}

// Resolve finds a resolver and returns plan + preview.
func (r *Registry) Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	for _, resolver := range r.resolvers {
		if resolver.Supports(source) {
			return resolver.Resolve(ctx, source)
		}
	}
	return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
}
