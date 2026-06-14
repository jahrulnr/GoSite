package auth

import (
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

// Store is an in-memory session store with cookie helpers.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
	secure   bool
}

// NewStore returns an in-memory session store.
func NewStore(ttl time.Duration) *Store {
	return NewStoreWithOptions(ttl, true)
}

// NewStoreWithOptions returns a session store with cookie secure flag.
func NewStoreWithOptions(ttl time.Duration, secure bool) *Store {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	return &Store{
		sessions: make(map[string]Session),
		ttl:      ttl,
		secure:   secure,
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

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	return session, nil
}

// Get returns the session for id when it exists and has not expired.
func (s *Store) Get(id string) (Session, bool) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		s.Delete(id)
		return Session{}, false
	}
	return session, true
}

// Delete removes a session by id.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// SetCookie writes the session id to the response as a secure HTTP-only cookie.
func (s *Store) SetCookie(w http.ResponseWriter, session Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
}

// ClearCookie removes the session cookie from the client.
func (s *Store) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
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
