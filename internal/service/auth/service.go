package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository loads users for authentication.
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (sqlite.User, error)
	FindByID(ctx context.Context, id int64) (sqlite.User, error)
}

// UserDTO is the public user payload returned by auth endpoints.
type UserDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// LoginResult is returned after a successful login.
type LoginResult struct {
	Token   string  `json:"token"`
	User    UserDTO `json:"user"`
	Session Session `json:"-"`
}

// Service handles panel authentication.
type Service struct {
	users      UserRepository
	sessions   *Store
	lockscreen *Lockscreen
}

// ServiceOption configures optional auth service dependencies.
type ServiceOption func(*Service)

// NewService returns an auth service backed by users and sessions.
func NewService(users UserRepository, sessions *Store, opts ...ServiceOption) *Service {
	s := &Service{
		users:    users,
		sessions: sessions,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Login validates credentials and creates a session.
func (s *Service) Login(ctx context.Context, email, password string, remember bool) (LoginResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return LoginResult{}, apperror.New(apperror.CodeAuthInvalidCredentials, "email and password are required")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNotFound(err) {
			return LoginResult{}, apperror.New(apperror.CodeAuthInvalidCredentials, "invalid email or password")
		}
		return LoginResult{}, apperror.Wrap(apperror.CodeDatabase, "lookup user failed", err)
	}

	if !VerifyPassword(password, user.Password) {
		return LoginResult{}, apperror.New(apperror.CodeAuthInvalidCredentials, "invalid email or password")
	}

	session, err := s.sessions.CreateFor(user.ID, remember)
	if err != nil {
		return LoginResult{}, apperror.Wrap(apperror.CodeInternal, "create session failed", err)
	}

	return LoginResult{
		Token: session.ID,
		User:  toUserDTO(user),
		Session: session,
	}, nil
}

// Logout removes the session identified by token.
func (s *Service) Logout(token string) {
	if token == "" {
		return
	}
	s.sessions.Delete(token)
}

// Me returns the authenticated user for token.
func (s *Service) Me(ctx context.Context, token string) (UserDTO, error) {
	session, ok := s.sessions.Get(token)
	if !ok {
		return UserDTO{}, apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}

	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNotFound(err) {
			s.sessions.Delete(token)
			return UserDTO{}, apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
		}
		return UserDTO{}, apperror.Wrap(apperror.CodeDatabase, "lookup user failed", err)
	}

	return toUserDTO(user), nil
}

// SessionUserID returns the user id for a valid session token.
func (s *Service) SessionUserID(token string) (int64, bool) {
	session, ok := s.sessions.Get(token)
	if !ok {
		return 0, false
	}
	return session.UserID, true
}

// VerifyPassword checks a plaintext password against a Laravel-compatible bcrypt hash.
func VerifyPassword(password, hash string) bool {
	normalized := strings.Replace(hash, "$2y$", "$2a$", 1)
	return bcrypt.CompareHashAndPassword([]byte(normalized), []byte(password)) == nil
}

func toUserDTO(user sqlite.User) UserDTO {
	return UserDTO{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "no rows")
}
