package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFetcher_AllowlistAndDigest(t *testing.T) {
	const body = "zip-bytes"
	sum := sha256.Sum256([]byte(body))
	digest := hex.EncodeToString(sum[:])

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	f := New(Config{
		AllowedHosts: []string{u.Host},
		MaxBytes:     1024,
		Timeout:      5 * time.Second,
		MaxRedirects: 3,
	})
	f.client = srv.Client()

	got, err := f.Fetch(context.Background(), srv.URL, digest)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(got) != body {
		t.Fatalf("body mismatch")
	}
}

func TestFetcher_RejectsHost(t *testing.T) {
	f := New(Config{AllowedHosts: []string{"example.com"}})
	_, err := f.Fetch(context.Background(), "https://evil.test/file.zip", "")
	if err == nil {
		t.Fatal("expected host rejection")
	}
}

func TestFetcher_DigestMismatch(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("other"))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	f := New(Config{AllowedHosts: []string{u.Host}, MaxBytes: 1024, Timeout: 5 * time.Second})
	f.client = srv.Client()

	_, err = f.Fetch(context.Background(), srv.URL, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected digest mismatch")
	}
}
