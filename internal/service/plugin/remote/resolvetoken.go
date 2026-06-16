package remote

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/jahrulnr/gosite/internal/service/plugin/remote/failures"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote/types"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

type cachedResolve struct {
	Source    types.Source
	Plan      types.FetchPlan
	ExpiresAt time.Time
}

// ResolveCache stores short-lived resolve tokens for install TOCTOU protection.
type ResolveCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]cachedResolve
}

// NewResolveCache returns an in-memory resolve token cache.
func NewResolveCache(ttl time.Duration) *ResolveCache {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &ResolveCache{ttl: ttl, items: map[string]cachedResolve{}}
}

// Issue stores a resolve plan and returns an opaque token.
func (c *ResolveCache) Issue(source types.Source, plan types.FetchPlan) (string, time.Time, error) {
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().UTC().Add(c.ttl)
	c.mu.Lock()
	c.items[token] = cachedResolve{Source: source, Plan: plan, ExpiresAt: expires}
	c.mu.Unlock()
	return token, expires, nil
}

// Consume returns a cached plan or an error if missing/expired.
func (c *ResolveCache) Consume(token string) (types.Source, types.FetchPlan, error) {
	c.mu.Lock()
	entry, ok := c.items[token]
	if ok {
		delete(c.items, token)
	}
	c.mu.Unlock()
	if !ok {
		return types.Source{}, types.FetchPlan{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveStale)
	}
	if time.Now().UTC().After(entry.ExpiresAt) {
		return types.Source{}, types.FetchPlan{}, apperror.New(apperror.CodeInvalidInput, failures.ResolveStale)
	}
	return entry.Source, entry.Plan, nil
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
