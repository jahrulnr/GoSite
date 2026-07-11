package filesystem

import (
	"path/filepath"
	"strings"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Validator resolves and validates file manager paths.
// It rejects empty input, path traversal ("..") and relative paths.
// Any absolute path is accepted; the OS permissions decide what can
// actually be read or written.
type Validator struct{}

// NewValidator returns a path validator. The roots argument is kept for
// backwards compatibility but is no longer used for access control.
func NewValidator(roots ...string) *Validator {
	_ = roots
	return &Validator{}
}

// Validate rejects traversal and non-absolute paths.
func (v *Validator) Validate(raw string) error {
	_, err := v.Resolve(raw)
	return err
}

// Resolve returns a cleaned absolute path.
func (v *Validator) Resolve(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", apperror.New(apperror.CodePathInvalid, "path not valid")
	}
	if strings.Contains(raw, "..") {
		return "", apperror.New(apperror.CodePathTraversal, "path traversal rejected")
	}

	clean := filepath.Clean(raw)
	if !filepath.IsAbs(clean) {
		return "", apperror.New(apperror.CodePathInvalid, "path must be absolute")
	}
	return clean, nil
}
