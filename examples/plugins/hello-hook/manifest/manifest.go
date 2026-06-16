// Package manifest provides a small, stable manifest.json for the
// hello-hook reference plugin. The host reads this file during install
// to determine capabilities, permissions, and entrypoints.
package manifest

import _ "embed"

//go:embed manifest.json
var Raw string
