package resolver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
)

func TestGitHubResolverResolve(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	indexJSON := fmt.Sprintf(`{
		"id": "acme/demo",
		"repository": "https://github.com/acme/demo",
		"distribution": {
			"releases": [{
				"version": "v1.0.0",
				"minGoSiteVersion": "0.1.0",
				"assets": [{
					"os": %q,
					"arch": %q,
					"url": "https://cdn.example.com/demo-1.0.0.zip",
					"sha256": "deadbeef"
				}]
			}]
		}
	}`, goos, goarch)
	const manifestJSON = `{
		"tier": 2,
		"permissions": ["network.outbound"],
		"capabilities": { "hooks": ["http.request"] }
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "gosite.plugin.json"):
			_, _ = w.Write([]byte(indexJSON))
		case strings.HasSuffix(r.URL.Path, "manifest.json"):
			_, _ = w.Write([]byte(manifestJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	resolver := GitHubResolver{Client: &http.Client{Transport: rawGitHubTransport{target: srv}}}
	plan, preview, err := resolver.Resolve(context.Background(), types.Source{
		Type: "github-release",
		Repo: "acme/demo",
		Tag:  "v1.0.0",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if plan.SHA256 != "deadbeef" {
		t.Fatalf("sha256: %q", plan.SHA256)
	}
	if preview.PluginID != "acme/demo" || preview.Version != "1.0.0" {
		t.Fatalf("preview id/version: %s %s", preview.PluginID, preview.Version)
	}
	if len(preview.Permissions) != 1 || preview.Tier != 2 {
		t.Fatalf("manifest hints: tier=%d perms=%v", preview.Tier, preview.Permissions)
	}
}

func TestGitHubResolverInvalidRepo(t *testing.T) {
	resolver := GitHubResolver{}
	_, _, err := resolver.Resolve(context.Background(), types.Source{Type: "github-release", Repo: "bad-repo", Tag: "v1"})
	if err == nil {
		t.Fatal("expected error for invalid repo")
	}
}

type rawGitHubTransport struct {
	target *httptest.Server
}

func (t rawGitHubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(strings.TrimPrefix(t.target.URL, "https://"), "http://")
	return http.DefaultTransport.RoundTrip(req)
}
