package fetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Config is fetcher-specific settings.
type Config struct {
	AllowedHosts []string
	MaxBytes     int64
	Timeout      time.Duration
	MaxRedirects int
}

// Fetcher downloads remote artifacts with host policy enforcement.
type Fetcher struct {
	client       *http.Client
	allowedHosts []string
	maxBytes     int64
	maxRedirects int
}

// New returns a configured artifact fetcher.
func New(cfg Config) *Fetcher {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	maxRedirects := cfg.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 3
	}
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 64 << 20
	}
	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		allowedHosts: cfg.AllowedHosts,
		maxBytes:     maxBytes,
		maxRedirects: maxRedirects,
	}
}

// Fetch downloads url and verifies pinned sha256 when expected is non-empty.
func (f *Fetcher) Fetch(ctx context.Context, rawURL, expectedSHA256 string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" {
		return nil, apperror.New(apperror.CodeInvalidInput, "url must be https")
	}
	if err := f.ensureHostAllowed(parsed.Host); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePluginOperation, failures.FetchFailed, err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePluginOperation, failures.FetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.Request != nil && resp.Request.URL != nil {
		if err := f.ensureHostAllowed(resp.Request.URL.Host); err != nil {
			return nil, err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apperror.New(apperror.CodePluginOperation, failures.FetchFailed)
	}

	limited := io.LimitReader(resp.Body, f.maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodePluginOperation, failures.FetchFailed, err)
	}
	if int64(len(data)) > f.maxBytes {
		return nil, apperror.New(apperror.CodePluginOperation, failures.FetchTooLarge)
	}

	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	expected := strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expected != "" && actual != expected {
		return nil, apperror.New(apperror.CodePluginOperation, failures.FetchDigestMismatch)
	}
	return data, nil
}

// HeadSize returns Content-Length when available (optional resolve hint).
func (f *Fetcher) HeadSize(ctx context.Context, rawURL string) (int64, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" {
		return 0, apperror.New(apperror.CodeInvalidInput, "url must be https")
	}
	if err := f.ensureHostAllowed(parsed.Host); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, parsed.String(), nil)
	if err != nil {
		return 0, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.Request != nil && resp.Request.URL != nil {
		if err := f.ensureHostAllowed(resp.Request.URL.Host); err != nil {
			return 0, err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("head status %d", resp.StatusCode)
	}
	return resp.ContentLength, nil
}

func (f *Fetcher) ensureHostAllowed(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return apperror.New(apperror.CodeInvalidInput, "url host required")
	}
	if len(f.allowedHosts) == 0 {
		return nil
	}
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	for _, allowed := range f.allowedHosts {
		if hostMatchesAllowed(hostname, allowed) {
			return nil
		}
	}
	return apperror.New(apperror.CodeInvalidInput, "host not in PLUGIN_INSTALL_ALLOWED_HOSTS")
}

func hostMatchesAllowed(hostname, allowed string) bool {
	allowed = strings.ToLower(strings.TrimSpace(allowed))
	if allowed == "" {
		return false
	}
	if ah, _, err := net.SplitHostPort(allowed); err == nil {
		allowed = ah
	}
	if allowed == hostname {
		return true
	}
	if strings.HasPrefix(allowed, "*.") {
		suffix := strings.TrimPrefix(allowed, "*")
		return strings.HasSuffix(hostname, suffix) && hostname != strings.TrimPrefix(suffix, ".")
	}
	return false
}
