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

func TestGitLabResolverResolve(t *testing.T) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	indexJSON := fmt.Sprintf(`{
		"id": "acme/demo",
		"distribution": {
			"releases": [{
				"version": "v1.0.0",
				"assets": [{
					"os": %q,
					"arch": %q,
					"url": "https://cdn.example.com/demo.zip",
					"sha256": "deadbeef"
				}]
			}]
		}
	}`, goos, goarch)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "gosite.plugin.json") {
			_, _ = w.Write([]byte(indexJSON))
			return
		}
		if strings.Contains(r.URL.Path, "manifest.json") {
			_, _ = w.Write([]byte(`{"tier":1,"permissions":[],"capabilities":{"hooks":[]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	resolver := GitLabResolver{Client: &http.Client{Transport: gitlabRawTransport{target: srv}}}
	plan, preview, err := resolver.Resolve(context.Background(), types.Source{
		Type: "gitlab-release",
		Repo: "acme/demo",
		Tag:  "v1.0.0",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if plan.SHA256 != "deadbeef" || preview.PluginID != "acme/demo" {
		t.Fatalf("unexpected plan/preview: %+v %+v", plan, preview)
	}
}

type gitlabRawTransport struct {
	target *httptest.Server
}

func (t gitlabRawTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(strings.TrimPrefix(t.target.URL, "https://"), "http://")
	return http.DefaultTransport.RoundTrip(req)
}
