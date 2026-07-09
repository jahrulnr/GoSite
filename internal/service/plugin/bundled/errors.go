package bundled

import "errors"

var (
	// ErrNotFound means the plugin id is not in the bundled index.
	ErrNotFound = errors.New("bundled plugin not found")
	// ErrArtifactsUnavailable means artifact files are not on disk.
	ErrArtifactsUnavailable = errors.New("bundled plugin artifacts unavailable")
)
