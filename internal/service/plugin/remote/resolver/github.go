package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/index"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// GitHubResolver resolves source.type=github-release via gosite.plugin.json at tag.
type GitHubResolver struct {
	Token        string
	Timeout      time.Duration
	Client       *http.Client
	BuildEnabled bool
}

func (g GitHubResolver) Supports(source types.Source) bool {
	t := strings.ToLower(strings.TrimSpace(source.Type))
	return t == "github-release" || t == "github"
}

func (g GitHubResolver) Resolve(ctx context.Context, source types.Source) (types.FetchPlan, types.ResolvePreview, error) {
	repo := strings.TrimSpace(source.Repo)
	tag := strings.TrimSpace(source.Tag)
	if repo == "" || tag == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveFailed)
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.New(apperror.CodeInvalidInput, "repo must be owner/name")
	}
	owner, name := parts[0], parts[1]

	indexBytes, err := g.fetchRaw(ctx, owner, name, tag, "gosite.plugin.json")
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, err
	}
	idx, err := index.Parse(indexBytes)
	if err != nil {
		return types.FetchPlan{}, types.ResolvePreview{}, apperror.Wrap(apperror.CodeInvalidInput, failures.ResolveFailed, err)
	}

	perms, hooks, tier := g.loadManifestHints(ctx, owner, name, tag)
	sourceRef := fmt.Sprintf("%s/%s@%s", owner, name, tag)
	return resolveIndexRelease(indexResolveInput{
		Idx:          idx,
		Tag:          tag,
		SourceType:   "github-release",
		SourceRef:    sourceRef,
		BuildEnabled: g.BuildEnabled,
		ForceBuild:   false,
		BuildToken:   g.Token,
		BuildVCS:     "github",
		Hints:        manifestHints{Permissions: perms, Hooks: hooks, Tier: tier},
	})
}

func (g GitHubResolver) loadManifestHints(ctx context.Context, owner, name, tag string) (perms, hooks []string, tier int) {
	data, err := g.fetchRaw(ctx, owner, name, tag, "manifest.json")
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

func (g GitHubResolver) fetchRaw(ctx context.Context, owner, name, tag, path string) ([]byte, error) {
	client := g.client()
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, name, tag, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
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

func (g GitHubResolver) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &http.Client{Timeout: timeout}
}
