package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/index"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// GitLabResolver resolves source.type=gitlab-release via gosite.plugin.json at tag.
type GitLabResolver struct {
	Token        string
	BaseURL      string
	Timeout      time.Duration
	Client       *http.Client
	BuildEnabled bool
}

func (g GitLabResolver) Supports(source types.Source) bool {
	t := strings.ToLower(strings.TrimSpace(source.Type))
	return t == "gitlab-release" || t == "gitlab"
}

func (g GitLabResolver) Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	repo := strings.TrimSpace(source.Repo)
	tag := strings.TrimSpace(source.Tag)
	if repo == "" || tag == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "repo must be group/name")
	}
	group, name := parts[0], parts[1]

	indexBytes, err := g.fetchRaw(ctx, group, name, tag, "gosite.plugin.json")
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, err
	}
	idx, err := index.Parse(indexBytes)
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodeInvalidInput, failures.ResolveFailed, err)
	}

	perms, hooks, tier := g.loadManifestHints(ctx, group, name, tag)
	sourceRef := fmt.Sprintf("%s/%s@%s", group, name, tag)
	return resolveIndexRelease(indexResolveInput{
		Idx:          idx,
		Tag:          tag,
		SourceType:   "gitlab-release",
		SourceRef:    sourceRef,
		BuildEnabled: g.BuildEnabled,
		ForceBuild:   false,
		BuildToken:   g.Token,
		BuildVCS:     "gitlab",
		Hints:        manifestHints{Permissions: perms, Hooks: hooks, Tier: tier},
	})
}

func (g GitLabResolver) loadManifestHints(ctx context.Context, group, name, tag string) (perms, hooks []string, tier int) {
	data, err := g.fetchRaw(ctx, group, name, tag, "manifest.json")
	if err != nil {
		return nil, nil, 0
	}
	var m struct {
		Tier        int      `json:"tier"`
		Permissions []string `json:"permissions"`
		Capabilities struct {
			Hooks []string `json:"hooks"`
		} `json:"capabilities"`
	}
	if json.Unmarshal(data, &m) != nil {
		return nil, nil, 0
	}
	return m.Permissions, m.Capabilities.Hooks, m.Tier
}

func (g GitLabResolver) fetchRaw(ctx context.Context, group, name, tag, path string) ([]byte, error) {
	client := g.client()
	base := strings.TrimRight(strings.TrimSpace(g.BaseURL), "/")
	if base == "" {
		base = "https://gitlab.com"
	}
	rawURL := fmt.Sprintf("%s/%s/%s/-/raw/%s/%s", base, url.PathEscape(group), url.PathEscape(name), url.PathEscape(tag), path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if g.Token != "" {
		req.Header.Set("PRIVATE-TOKEN", g.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePluginOperation, failures.FetchFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, apperror.New(apperror.CodePluginOperation, failures.AuthTokenExpired)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, apperror.New(apperror.CodePluginOperation, failures.ResolveFailed)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apperror.New(apperror.CodePluginOperation, failures.FetchFailed)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePluginOperation, failures.FetchFailed, err)
	}
	return data, nil
}

func (g GitLabResolver) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &http.Client{Timeout: timeout}
}
