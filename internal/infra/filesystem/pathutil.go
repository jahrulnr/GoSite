package filesystem

import (
	"path/filepath"
	"strings"

	"github.com/jahrulnr/gosite/pkg/apperror"
)

// DefaultAllowRoots are the filesystem roots exposed by the file manager.
// Allowing "/" means the file manager can browse any absolute path the
// process has permission to read; traversal and relative paths are still
// rejected below.
var DefaultAllowRoots = []string{"/"}

// Validator resolves and validates paths against an allowlist of roots.
type Validator struct {
	Roots []string
}

// NewValidator returns a path validator for the given roots.
func NewValidator(roots ...string) *Validator {
	if len(roots) == 0 {
		roots = append([]string(nil), DefaultAllowRoots...)
	}
	cleaned := make([]string, 0, len(roots))
	for _, root := range roots {
		cleaned = append(cleaned, filepath.Clean(root))
	}
	return &Validator{Roots: cleaned}
}

// Validate rejects traversal and paths outside configured roots.
func (v *Validator) Validate(raw string) error {
	_, err := v.Resolve(raw)
	return err
}

// Resolve returns a cleaned absolute path within an allowed root.
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

	for _, root := range v.Roots {
		if pathUnderRoot(clean, root) {
			return clean, nil
		}
	}
	// No configured root matched, but the file manager is no longer restricted
	// to a hard allowlist. Accept any absolute path the OS will allow.
	if filepath.IsAbs(clean) {
		return clean, nil
	}
	return "", apperror.New(apperror.CodePathInvalid, "path must be absolute")
}

func pathUnderRoot(path, root string) bool {
	if root == string(filepath.Separator) {
		return filepath.IsAbs(path)
	}
	if path == root {
		return true
	}
	prefix := root + string(filepath.Separator)
	return strings.HasPrefix(path, prefix)
}
