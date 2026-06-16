package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
)

func TestGitRefResolverTier0(t *testing.T) {
	const manifest = `{
		"id": "acme/webhook",
		"name": "Webhook",
		"version": "1.0.0",
		"tier": 0,
		"apiVersion": "gosite-plugin/1",
		"permissions": ["network.outbound"],
		"capabilities": {"hooks": ["http.request"]}
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "manifest.json") {
			_, _ = w.Write([]byte(manifest))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	gh := GitHubResolver{Client: &http.Client{Transport: rawGitHubTransport{target: srv}}}
	resolver := GitRefResolver{GitHub: gh}
	plan, preview, err := resolver.Resolve(context.Background(), types.Source{
		Type: "git-ref",
		Repo: "acme/webhook",
		Tag:  "v1.0.0",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(plan.InlineArtifact) == 0 || preview.Tier != 0 {
		t.Fatalf("expected inline tier-0 artifact, got tier=%d bytes=%d", preview.Tier, len(plan.InlineArtifact))
	}
}

func TestGitRefResolverRejectsNonTier0(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"acme/x","version":"1.0.0","tier":1}`))
	}))
	defer srv.Close()
	gh := GitHubResolver{Client: &http.Client{Transport: rawGitHubTransport{target: srv}}}
	resolver := GitRefResolver{GitHub: gh}
	_, _, err := resolver.Resolve(context.Background(), types.Source{Type: "git-ref", Repo: "acme/x", Tag: "v1"})
	if err == nil {
		t.Fatal("expected tier error")
	}
}
