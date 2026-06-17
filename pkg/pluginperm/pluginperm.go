package pluginperm

import "strings"

// Known scopes for integration token validation and UI registry.
var known = map[string]struct{}{
	"system:read":      {},
	"dashboard:read":   {},
	"ui:read":          {},
	"websites:read":    {},
	"websites:write":   {},
	"ssl:read":         {},
	"ssl:write":        {},
	"nginx:read":       {},
	"nginx:manage":     {},
	"docker:read":      {},
	"docker:manage":    {},
	"files:read":       {},
	"files:manage":     {},
	"mount:read":       {},
	"mount:manage":     {},
	"cron:read":        {},
	"cron:write":       {},
	"jobs:read":        {},
	"jobs:manage":      {},
	"query:read":       {},
	"query:manage":     {},
	"metrics:read":     {},
	"settings:write":   {},
	"plugins:read":     {},
	"plugins:manage":   {},
	"terminal:manage":  {},
	"network.outbound": {},
	"secrets:receive":  {},
}

// Valid reports whether scope is a known permission string.
func Valid(scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return false
	}
	_, ok := known[scope]
	return ok
}

// All returns a sorted copy of known scopes.
func All() []string {
	out := make([]string, 0, len(known))
	for s := range known {
		out = append(out, s)
	}
	stringsSort(out)
	return out
}

func stringsSort(items []string) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// SubsetOfManifest returns true when every scope is in manifestPermissions.
func SubsetOfManifest(scopes, manifestPermissions []string) bool {
	allowed := make(map[string]struct{}, len(manifestPermissions))
	for _, p := range manifestPermissions {
		p = strings.TrimSpace(p)
		if p != "" {
			allowed[p] = struct{}{}
		}
	}
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			return false
		}
		if _, ok := allowed[s]; !ok {
			return false
		}
	}
	return true
}

// HasScope returns true when required is present in scopes.
func HasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if strings.TrimSpace(s) == required {
			return true
		}
	}
	return false
}

// Intersect returns scopes present in both slices preserving order from scopes.
func Intersect(scopes, manifestPermissions []string) []string {
	allowed := make(map[string]struct{}, len(manifestPermissions))
	for _, p := range manifestPermissions {
		p = strings.TrimSpace(p)
		if p != "" {
			allowed[p] = struct{}{}
		}
	}
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := allowed[s]; ok {
			out = append(out, s)
		}
	}
	return out
}
