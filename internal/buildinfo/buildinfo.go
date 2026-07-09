package buildinfo

import "strings"

// Version is injected by release builds with -ldflags.
// Local builds use X.Y.Z-dev; GitHub release / production images use X.Y.Z (no suffix).
var Version = "1.0.0-dev"

// IsDev reports whether Version is a local (non-release) build.
func IsDev() bool {
	return strings.HasSuffix(strings.TrimSpace(Version), "-dev")
}
