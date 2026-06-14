package auth

import (
	"context"
	"sync"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Lockscreen tracks per-session lock state for the panel lockscreen overlay.
type Lockscreen struct {
	mu     sync.RWMutex
	locked map[string]bool
}

// NewLockscreen returns an empty lockscreen registry.
func NewLockscreen() *Lockscreen {
	return &Lockscreen{locked: make(map[string]bool)}
}

// Lock marks a session as locked.
func (l *Lockscreen) Lock(sessionID string) {
	if sessionID == "" {
		return
	}
	l.mu.Lock()
	l.locked[sessionID] = true
	l.mu.Unlock()
}

// Unlock clears the lock for a session.
func (l *Lockscreen) Unlock(sessionID string) {
	if sessionID == "" {
		return
	}
	l.mu.Lock()
	delete(l.locked, sessionID)
	l.mu.Unlock()
}

// IsLocked reports whether the session is locked.
func (l *Lockscreen) IsLocked(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.locked[sessionID]
}

// LoginMetadata describes auth-related panel configuration for the login page.
type LoginMetadata struct {
	LockscreenEnabled bool       `json:"lockscreen_enabled"`
	BasicAuthEnabled  bool       `json:"basic_auth_enabled"`
	LockAfterSeconds  int        `json:"lock_after_seconds"`
	WebRoot           string     `json:"web_root"`
	FileRoots         []FileRoot `json:"file_roots"`
}

// FileRoot is a browsable top-level directory in the file manager.
type FileRoot struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

// LockscreenStatus is returned by GET /auth/lockscreen.
type LockscreenStatus struct {
	Locked bool     `json:"locked"`
	User   *UserDTO `json:"user,omitempty"`
}

// WithLockscreen attaches a lockscreen registry to the auth service.
func WithLockscreen(lock *Lockscreen) ServiceOption {
	return func(s *Service) {
		s.lockscreen = lock
	}
}

// LoginMetadataFromConfig builds public login metadata from runtime config.
func LoginMetadataFromConfig(lockscreenEnabled, basicAuthEnabled bool, lockAfterSeconds int, webPath, storagePath string) LoginMetadata {
	if lockAfterSeconds < 0 {
		lockAfterSeconds = 0
	}
	if webPath == "" {
		webPath = "/www"
	}
	if storagePath == "" {
		storagePath = "/storage"
	}
	return LoginMetadata{
		LockscreenEnabled: lockscreenEnabled,
		BasicAuthEnabled:  basicAuthEnabled,
		LockAfterSeconds:  lockAfterSeconds,
		WebRoot:           webPath,
		FileRoots: []FileRoot{
			{Label: "Websites", Path: webPath},
			{Label: "Storage", Path: storagePath},
			{Label: "Temp", Path: "/tmp"},
		},
	}
}

// LockSession marks the current session locked (client idle timeout).
func (s *Service) LockSession(token string) error {
	if token == "" {
		return apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}
	if _, ok := s.sessions.Get(token); !ok {
		return apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}
	if s.lockscreen != nil {
		s.lockscreen.Lock(token)
	}
	return nil
}

// LockscreenStatus returns lock state and user profile for an active session.
func (s *Service) LockscreenStatus(ctx context.Context, token string) (LockscreenStatus, error) {
	if token == "" {
		return LockscreenStatus{}, apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}
	if _, ok := s.sessions.Get(token); !ok {
		return LockscreenStatus{}, apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}

	locked := false
	if s.lockscreen != nil {
		locked = s.lockscreen.IsLocked(token)
	}

	user, err := s.Me(ctx, token)
	if err != nil {
		return LockscreenStatus{}, err
	}

	status := LockscreenStatus{Locked: locked}
	if locked {
		status.User = &user
	}
	return status, nil
}

// Unlock validates the password and clears the session lock.
func (s *Service) Unlock(ctx context.Context, token, password string) error {
	if token == "" {
		return apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}
	session, ok := s.sessions.Get(token)
	if !ok {
		return apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}
	if s.lockscreen == nil || !s.lockscreen.IsLocked(token) {
		return apperror.New(apperror.CodeValidation, "session is not locked")
	}

	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "lookup user failed", err)
	}
	if !VerifyPassword(password, user.Password) {
		return apperror.New(apperror.CodeAuthInvalidCredentials, "invalid password")
	}

	s.lockscreen.Unlock(token)
	return nil
}
