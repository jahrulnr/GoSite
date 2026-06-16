package remote

import (
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
)

func TestResolveCacheIssueConsume(t *testing.T) {
	cache := NewResolveCache(time.Minute)
	source := types.Source{Type: "url", URL: "https://example.com/p.zip", SHA256: "abc"}
	plan := types.FetchPlan{URL: source.URL, SHA256: "abc", SourceType: "url"}

	token, expires, err := cache.Issue(source, plan)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if token == "" || expires.Before(time.Now().UTC()) {
		t.Fatalf("unexpected token/expiry: %q %v", token, expires)
	}

	gotSource, gotPlan, err := cache.Consume(token)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if !sameSource(gotSource, source) {
		t.Fatalf("source mismatch: %+v", gotSource)
	}
	if gotPlan.URL != plan.URL {
		t.Fatalf("plan mismatch: %+v", gotPlan)
	}

	if _, _, err := cache.Consume(token); err == nil {
		t.Fatal("expected stale on second consume")
	}
}

func TestResolveCacheConsumeExpired(t *testing.T) {
	cache := NewResolveCache(time.Millisecond)
	source := types.Source{Type: "url", URL: "https://example.com/p.zip"}
	plan := types.FetchPlan{URL: source.URL}

	token, _, err := cache.Issue(source, plan)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, _, err := cache.Consume(token); err == nil {
		t.Fatal("expected expired token error")
	}
}
