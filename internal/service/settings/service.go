package settings

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository loads and updates admin users.
type UserRepository interface {
	FindByID(ctx context.Context, id int64) (sqlite.User, error)
	UpdateProfile(ctx context.Context, id int64, name, email, password string) (sqlite.User, error)
}

// ProfileInput is the payload for profile updates.
type ProfileInput struct {
	ID       int64
	Name     string
	Email    string
	Password string
}

// ProfileResult is the public user payload after update.
type ProfileResult struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Service handles panel settings mutations.
type Service struct {
	users UserRepository
}

// NewService returns a settings service.
func NewService(users UserRepository) *Service {
	return &Service{users: users}
}

// UpdateProfile updates the authenticated user's name, email, and optional password.
func (s *Service) UpdateProfile(ctx context.Context, in ProfileInput) (ProfileResult, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.TrimSpace(in.Email)
	if in.ID <= 0 {
		return ProfileResult{}, apperror.New(apperror.CodeInvalidInput, "invalid user id")
	}
	if name == "" {
		return ProfileResult{}, apperror.New(apperror.CodeValidation, "name is required")
	}
	if email == "" || !strings.Contains(email, "@") {
		return ProfileResult{}, apperror.New(apperror.CodeValidation, "valid email is required")
	}

	password := in.Password
	if password != "" && utf8.RuneCountInString(password) < 6 {
		return ProfileResult{}, apperror.New(apperror.CodeValidation, "password must be at least 6 characters")
	}

	if _, err := s.users.FindByID(ctx, in.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isNotFound(err) {
			return ProfileResult{}, apperror.New(apperror.CodeNotFound, "account does not exist")
		}
		return ProfileResult{}, apperror.Wrap(apperror.CodeDatabase, "lookup user failed", err)
	}

	hash := ""
	if password != "" {
		normalized, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return ProfileResult{}, apperror.Wrap(apperror.CodeInternal, "hash password failed", err)
		}
		hash = strings.Replace(string(normalized), "$2a$", "$2y$", 1)
	}

	updated, err := s.users.UpdateProfile(ctx, in.ID, name, email, hash)
	if err != nil {
		return ProfileResult{}, apperror.Wrap(apperror.CodeDatabase, "update profile failed", err)
	}

	return ProfileResult{
		ID:    updated.ID,
		Name:  updated.Name,
		Email: updated.Email,
	}, nil
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "no rows")
}

// ToAuthUserDTO converts a profile result to auth.UserDTO.
func ToAuthUserDTO(p ProfileResult) auth.UserDTO {
	return auth.UserDTO{
		ID:    p.ID,
		Name:  p.Name,
		Email: p.Email,
	}
}
