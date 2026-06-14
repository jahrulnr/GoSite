package splunklite

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

// ParseQuery splits a mini Splunk query into field:value clauses.
// Bare tokens without ":" are treated as free-text search across common fields.
// A token wrapped in `/.../` is treated as a Go regexp; bareword regex
// targets `_text`, `field:/.../` targets the named column.
func ParseQuery(q string) ([]sqlite.FieldClause, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}

	var clauses []sqlite.FieldClause
	for _, raw := range strings.Fields(q) {
		token := raw
		// If the token is a field:value pair, route through field-aware
		// parsing so the colon isn't dropped before we see it.
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 && parts[0] != "" {
			field := strings.ToLower(strings.TrimSpace(parts[0]))
			value := strings.TrimSpace(parts[1])
			if !isValidFieldName(field) {
				return nil, errInvalidQuery("invalid field name " + field)
			}
			if value == "" {
				return nil, errInvalidQuery("empty field or value in " + raw)
			}
			if isRegexToken(value) {
				pattern := value[1 : len(value)-1]
				if err := validateRegexp(pattern); err != nil {
					return nil, errInvalidQuery(err.Error())
				}
				clauses = append(clauses, sqlite.RegexpClause(field, pattern))
				continue
			}
			clauses = append(clauses, sqlite.LikeClause(field, value))
			continue
		}

		// bareword
		if isRegexToken(token) {
			pattern := token[1 : len(token)-1]
			if err := validateRegexp(pattern); err != nil {
				return nil, errInvalidQuery(err.Error())
			}
			clauses = append(clauses, sqlite.RegexpClause("_text", pattern))
			continue
		}
		value := strings.TrimSpace(token)
		if value == "" {
			continue
		}
		clauses = append(clauses, sqlite.LikeClause("_text", value))
	}
	return clauses, nil
}

// isRegexToken returns true when token is wrapped in `/.../`.
func isRegexToken(token string) bool {
	return len(token) >= 2 && strings.HasPrefix(token, "/") && strings.HasSuffix(token, "/")
}

func validateRegexp(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := regexp.Compile(pattern)
	return err
}

func isValidFieldName(s string) bool {
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

type queryError string

func (e queryError) Error() string { return string(e) }

func errInvalidQuery(msg string) error {
	return queryError(msg)
}
