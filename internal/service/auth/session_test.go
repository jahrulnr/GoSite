package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/stretchr/testify/assert"
)

func TestSession_ClearCookie(t *testing.T) {
	t.Parallel()

	store := auth.NewStore(0)
	rec := httptest.NewRecorder()
	store.ClearCookie(rec)

	cookies := rec.Result().Cookies()
	requireCookie := func(t *testing.T) *http.Cookie {
		t.Helper()
		for _, c := range cookies {
			if c.Name == auth.SessionCookieName {
				return c
			}
		}
		t.Fatal("session cookie not set")
		return nil
	}
	c := requireCookie(t)
	assert.Equal(t, "", c.Value)
	assert.Equal(t, -1, c.MaxAge)
}

func TestSession_SessionFromRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Equal(t, "", auth.SessionFromRequest(req))

	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "abc123"})
	assert.Equal(t, "abc123", auth.SessionFromRequest(req))
}

func TestSession_CreateForRememberExtendsTTL(t *testing.T) {
	t.Parallel()

	store := auth.NewStore(time.Hour)
	short, err := store.CreateFor(1, false)
	if err != nil {
		t.Fatalf("create short session: %v", err)
	}
	long, err := store.CreateFor(2, true)
	if err != nil {
		t.Fatalf("create remember session: %v", err)
	}

	shortTTL := short.ExpiresAt.Sub(short.CreatedAt)
	longTTL := long.ExpiresAt.Sub(long.CreatedAt)
	assert.InDelta(t, float64(time.Hour), float64(shortTTL), float64(time.Minute))
	assert.InDelta(t, float64(30*24*time.Hour), float64(longTTL), float64(time.Hour))
}

func TestSession_SetCookie(t *testing.T) {
	t.Parallel()

	store := auth.NewStore(0)
	session, err := store.Create(1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	rec := httptest.NewRecorder()
	store.SetCookie(rec, session)

	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == auth.SessionCookieName {
			found = true
			assert.Equal(t, session.ID, c.Value)
			assert.True(t, c.HttpOnly)
		}
	}
	assert.True(t, found)
}
