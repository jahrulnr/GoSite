package sqlite

import "strings"

// ClauseKind identifies the SQL operator a FieldClause should emit.
type ClauseKind int

const (
	// LikeClauseKind emits `column LIKE ?` (or `=` for an exact match).
	LikeClauseKind ClauseKind = iota
	// RegexpClauseKind emits `column REGEXP ?` using Go regexp syntax.
	RegexpClauseKind
)

// FieldClause is a parsed field:value filter token.
type FieldClause struct {
	Field string
	Value string
	Kind  ClauseKind
}

// LikeClause builds a LIKE-based FieldClause.
func LikeClause(field, value string) FieldClause {
	return FieldClause{Field: field, Value: value, Kind: LikeClauseKind}
}

// RegexpClause builds a regexp-based FieldClause.
func RegexpClause(field, value string) FieldClause {
	return FieldClause{Field: field, Value: value, Kind: RegexpClauseKind}
}

func likeClause(column, pattern string) (string, interface{}) {
	if strings.Contains(pattern, "*") {
		escaped := strings.ReplaceAll(pattern, `%`, `\%`)
		escaped = strings.ReplaceAll(escaped, `_`, `\_`)
		escaped = strings.ReplaceAll(escaped, `*`, `%`)
		return column + ` LIKE ? ESCAPE '\'`, escaped
	}
	return column + ` = ?`, pattern
}

// regexpClause emits a Go-regexp REGEXP match. The pattern is the raw
// expression; SQLite's REGEXP uses Go's regexp engine (modernc.org/sqlite).
func regexpClause(column, pattern string) (string, interface{}) {
	return column + ` REGEXP ?`, pattern
}

func freeTextLike(column, value string) (string, interface{}) {
	if strings.Contains(value, "*") {
		return likeClause(column, value)
	}
	return column + ` LIKE ?`, "%" + value + "%"
}

// freeTextRegexp ORs a REGEXP match across multiple columns.
func freeTextRegexp(columns []string, pattern string) (string, []interface{}) {
	parts := make([]string, 0, len(columns))
	args := make([]interface{}, 0, len(columns))
	for _, col := range columns {
		w, a := regexpClause(col, pattern)
		parts = append(parts, w)
		args = append(args, a)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// BuildFreeTextWhere ORs a LIKE match across multiple columns.
func BuildFreeTextWhere(columns []string, value string) (string, []interface{}) {
	parts := make([]string, 0, len(columns))
	args := make([]interface{}, 0, len(columns))
	for _, col := range columns {
		w, a := freeTextLike(col, value)
		parts = append(parts, w)
		args = append(args, a)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// BuildFreeTextRegexpWhere ORs a REGEXP match across multiple columns.
func BuildFreeTextRegexpWhere(columns []string, pattern string) (string, []interface{}) {
	return freeTextRegexp(columns, pattern)
}
