package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	// SessionCookieName is the HTTP-only cookie used to identify panel sessions.
	SessionCookieName = "gosite_session"
	defaultSessionTTL = 2 * time.Hour
	rememberSessionTTL = 30 * 24 * time.Hour
)

// Session holds authenticated user state for a single browser session.
type Session struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Persister is the persistence layer used by Store. The session repo
// (internal/repository/sqlite) implements this interface.
type Persister interface {
	Create(ctx context.Context, rec SessionRecord) error
	Get(ctx context.Context, id string) (SessionRecord, bool)
	Delete(ctx context.Context, id string) error
}

// SessionRecord is the persistence-layer view of a Session. Re-exported from
// the sqlite package to keep auth self-contained.
type SessionRecord struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store is the in-memory cache + cookie helper for active panel sessions.
// Every mutation is mirrored to the Persister (SQLite) so the session survives
// container restarts and nginx/Go crashes.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
	secure   bool
	persister Persister
}

// NewStore returns an in-memory session store without persistence.
func NewStore(ttl time.Duration) *Store {
	return NewStoreWithOptions(ttl, true, nil)
}

// NewStoreWithOptions returns a session store with cookie secure flag and
// optional SQLite-backed persister. If persister is nil, the store is purely
// in-memory.
func NewStoreWithOptions(ttl time.Duration, secure bool, persister Persister) *Store {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	return &Store{
		sessions:  make(map[string]Session),
		ttl:       ttl,
		secure:    secure,
		persister: persister,
	}
}

// Create registers a new session for userID and returns the session record.
func (s *Store) Create(userID int64) (Session, error) {
	return s.CreateFor(userID, false)
}

// CreateFor registers a session; remember extends cookie lifetime to 30 days.
func (s *Store) CreateFor(userID int64, remember bool) (Session, error) {
	ttl := s.ttl
	if remember {
		ttl = rememberSessionTTL
	}

	id, err := newSessionID()
	if err != nil {
		return Session{}, err
	}

	now := time.Now().UTC()
	session := Session{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	if s.persister != nil {
		if err := s.persister.Create(context.Background(), SessionRecord{
			ID:        session.ID,
			UserID:    session.UserID,
			CreatedAt: session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
		}); err != nil {
			return Session{}, err
		}
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	return session, nil
}

// Get returns the session for id when it exists and has not expired. On a
// cache miss with a persister configured, it falls through to SQLite so a
// session created by another process or after a restart still resolves.
func (s *Store) Get(id string) (Session, bool) {
	if id == "" {
		return Session{}, false
	}
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	if ok {
		if time.Now().UTC().After(session.ExpiresAt) {
			s.Delete(id)
			return Session{}, false
		}
		return session, true
	}
	if s.persister == nil {
		return Session{}, false
	}
	rec, ok := s.persister.Get(context.Background(), id)
	if !ok {
		return Session{}, false
	}
	session = Session{ID: rec.ID, UserID: rec.UserID, CreatedAt: rec.CreatedAt, ExpiresAt: rec.ExpiresAt}
	if time.Now().UTC().After(session.ExpiresAt) {
		return Session{}, false
	}
	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()
	return session, true
}

// Delete removes a session by id from both cache and persister.
func (s *Store) Delete(id string) {
	if id == "" {
		return
	}
	if s.persister != nil {
		_ = s.persister.Delete(context.Background(), id)
	}
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// SetCookie writes the session id to the response as an HTTP-only cookie.
// Secure follows the client-facing scheme (X-Forwarded-Proto from nginx) so FE and BE stay aligned.
func (s *Store) SetCookie(w http.ResponseWriter, r *http.Request, session Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
}

// ClearCookie removes the session cookie from the client.
func (s *Store) ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// cookieSecure mirrors the scheme the browser uses (via reverse proxy), not the upstream TLS hop.
func (s *Store) cookieSecure(r *http.Request) bool {
	if r == nil {
		return s.secure
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto == "https"
	}
	if r.TLS != nil {
		return s.secure
	}
	return false
}

// SessionFromRequest reads the session cookie value from a request.
func SessionFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func newSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
