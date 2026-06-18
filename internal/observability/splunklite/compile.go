package splunklite

import (
	"fmt"
	"strings"
)

// SourceKind identifies which repository schema compiles a filter.
type SourceKind int

const (
	SourceKindAudit SourceKind = iota
	SourceKindJob
	SourceKindLogAccess
	SourceKindLogError
)

var auditFields = map[string]string{
	"user":          "user_email",
	"user_email":    "user_email",
	"action":        "action",
	"resource_type": "resource_type",
	"resource_id":   "resource_id",
	"domain":        "domain",
	"status":        "status",
	"message":       "message",
}

var jobFields = map[string]string{
	"job_type": "job_type",
	"type":     "job_type",
	"name":     "name",
	"status":   "status",
	"output":   "output",
	"error":    "error",
	"message":  "name",
}

var logAccessFields = map[string]string{
	"site":        "site",
	"status":      "status_code",
	"status_code": "status_code",
	"message":     "raw_preview",
	"preview":     "raw_preview",
}

var logErrorFields = map[string]string{
	"site":    "site",
	"message": "raw_preview",
	"preview": "raw_preview",
}

func fieldsForKind(kind SourceKind) map[string]string {
	switch kind {
	case SourceKindAudit:
		return auditFields
	case SourceKindJob:
		return jobFields
	case SourceKindLogAccess:
		return logAccessFields
	case SourceKindLogError:
		return logErrorFields
	default:
		return nil
	}
}

func freeTextColumns(kind SourceKind) []string {
	switch kind {
	case SourceKindAudit:
		return []string{"user_email", "action", "message"}
	case SourceKindJob:
		return []string{"name", "output", "error", "job_type"}
	case SourceKindLogAccess, SourceKindLogError:
		return []string{"site", "raw_preview"}
	default:
		return nil
	}
}

// SQLFilter is a compiled WHERE clause for SQLite repositories.
type SQLFilter struct {
	Wheres     []string
	Args       []interface{}
	Applicable bool
}

// CompileFilter turns a filter AST into SQL WHERE fragments for kind.
func CompileFilter(expr FilterExpr, kind SourceKind) (SQLFilter, error) {
	if expr == nil {
		return SQLFilter{Applicable: true}, nil
	}
	fields := fieldsForKind(kind)
	if fields == nil {
		return SQLFilter{}, fmt.Errorf("unknown source kind")
	}
	if !exprApplicable(expr, fields) {
		return SQLFilter{Applicable: false}, nil
	}
	where, args, err := compileNode(expr, kind, fields)
	if err != nil {
		return SQLFilter{}, err
	}
	if where == "" {
		return SQLFilter{Applicable: true}, nil
	}
	return SQLFilter{Wheres: []string{where}, Args: args, Applicable: true}, nil
}

func isComparison(op Op) bool {
	switch op {
	case OpGt, OpGte, OpLt, OpLte:
		return true
	default:
		return false
	}
}

func exprApplicable(expr FilterExpr, allowed map[string]string) bool {
	switch e := expr.(type) {
	case *AndExpr:
		if len(e.Children) == 0 {
			return false
		}
		for _, c := range e.Children {
			if !exprApplicable(c, allowed) {
				return false
			}
		}
		return true
	case *OrExpr:
		for _, c := range e.Children {
			if exprApplicable(c, allowed) {
				return true
			}
		}
		return false
	case *NotExpr:
		return exprApplicable(e.Child, allowed)
	case *PredExpr:
		if e.Field == "" {
			return true
		}
		col, ok := allowed[e.Field]
		if !ok {
			return false
		}
		if isComparison(e.Op) && col != "status_code" {
			return false
		}
		return true
	default:
		return false
	}
}

func compileNode(expr FilterExpr, kind SourceKind, fields map[string]string) (string, []interface{}, error) {
	switch e := expr.(type) {
	case *AndExpr:
		parts := make([]string, 0, len(e.Children))
		args := make([]interface{}, 0)
		for _, child := range e.Children {
			w, a, err := compileNode(child, kind, fields)
			if err != nil {
				return "", nil, err
			}
			if w == "" {
				continue
			}
			parts = append(parts, w)
			args = append(args, a...)
		}
		if len(parts) == 0 {
			return "", nil, nil
		}
		return "(" + strings.Join(parts, " AND ") + ")", args, nil
	case *OrExpr:
		parts := make([]string, 0, len(e.Children))
		args := make([]interface{}, 0)
		for _, child := range e.Children {
			w, a, err := compileNode(child, kind, fields)
			if err != nil {
				return "", nil, err
			}
			if w == "" {
				continue
			}
			parts = append(parts, w)
			args = append(args, a...)
		}
		if len(parts) == 0 {
			return "", nil, nil
		}
		return "(" + strings.Join(parts, " OR ") + ")", args, nil
	case *NotExpr:
		w, a, err := compileNode(e.Child, kind, fields)
		if err != nil {
			return "", nil, err
		}
		if w == "" {
			return "", nil, nil
		}
		return "(NOT (" + w + "))", a, nil
	case *PredExpr:
		return compilePred(e, kind, fields)
	default:
		return "", nil, fmt.Errorf("unknown filter node")
	}
}

func compilePred(p *PredExpr, kind SourceKind, fields map[string]string) (string, []interface{}, error) {
	if p.Field == "" {
		cols := freeTextColumns(kind)
		switch p.Op {
		case OpRegexp:
			w, a := freeTextRegexp(cols, p.Value)
			return w, a, nil
		case OpContains, OpLike, OpEq:
			w, a := freeTextContains(cols, p.Value, p.Op)
			return w, a, nil
		default:
			return "", nil, fmt.Errorf("unsupported free-text operator")
		}
	}
	col, ok := fields[p.Field]
	if !ok {
		return "", nil, nil
	}
	switch p.Op {
	case OpRegexp:
		return regexpSQL(col) + ` REGEXP ?`, []interface{}{p.Value}, nil
	case OpLike:
		w, a := likeSQL(col, p.Value)
		return w, []interface{}{a}, nil
	case OpEq:
		if !strings.Contains(p.Value, "*") && col != "status_code" {
			if col == "raw_preview" && p.Op == OpEq {
				return col + ` LIKE ?`, []interface{}{"%" + p.Value + "%"}, nil
			}
			return col + ` = ?`, []interface{}{p.Value}, nil
		}
		if !strings.Contains(p.Value, "*") && col == "status_code" {
			return col + ` = ?`, []interface{}{p.Value}, nil
		}
		w, a := likeSQL(col, p.Value)
		return w, []interface{}{a}, nil
	case OpNe:
		return col + ` != ?`, []interface{}{p.Value}, nil
	case OpGt, OpGte, OpLt, OpLte:
		if col != "status_code" {
			return "", nil, nil
		}
		op := map[Op]string{OpGt: ">", OpGte: ">=", OpLt: "<", OpLte: "<="}[p.Op]
		return `CAST(` + col + ` AS INTEGER) ` + op + ` ?`, []interface{}{p.Value}, nil
	default:
		w, a := freeTextContains([]string{col}, p.Value, OpContains)
		return w, a, nil
	}
}

func regexpSQL(column string) string {
	if column == "status_code" {
		return `CAST(status_code AS TEXT)`
	}
	return column
}

func likeSQL(column, pattern string) (string, interface{}) {
	matchCol := column
	if column == "status_code" {
		matchCol = `CAST(status_code AS TEXT)`
	}
	if strings.Contains(pattern, "*") {
		escaped := strings.ReplaceAll(pattern, `%`, `\%`)
		escaped = strings.ReplaceAll(escaped, `_`, `\_`)
		escaped = strings.ReplaceAll(escaped, `*`, `%`)
		return matchCol + ` LIKE ? ESCAPE '\'`, escaped
	}
	return matchCol + ` = ?`, pattern
}

func freeTextContains(columns []string, value string, op Op) (string, []interface{}) {
	parts := make([]string, 0, len(columns))
	args := make([]interface{}, 0, len(columns))
	for _, col := range columns {
		if op == OpLike || strings.Contains(value, "*") {
			w, a := likeSQL(col, value)
			parts = append(parts, w)
			args = append(args, a)
			continue
		}
		parts = append(parts, col+` LIKE ?`)
		args = append(args, "%"+value+"%")
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func freeTextRegexp(columns []string, pattern string) (string, []interface{}) {
	parts := make([]string, 0, len(columns))
	args := make([]interface{}, 0, len(columns))
	for _, col := range columns {
		parts = append(parts, regexpSQL(col)+` REGEXP ?`)
		args = append(args, pattern)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

// WithSiteScope ANDs a site equality predicate onto expr.
func WithSiteScope(expr FilterExpr, site string) FilterExpr {
	site = strings.TrimSpace(site)
	if site == "" {
		return expr
	}
	sitePred := &PredExpr{Field: "site", Op: OpEq, Value: site}
	return And(expr, sitePred)
}

func sourceKindForLog(source string) SourceKind {
	if source == SourceError {
		return SourceKindLogError
	}
	return SourceKindLogAccess
}

func sourceKindForName(source string) SourceKind {
	switch strings.ToLower(source) {
	case SourceAudit:
		return SourceKindAudit
	case SourceJob:
		return SourceKindJob
	case SourceAccess:
		return SourceKindLogAccess
	case SourceError:
		return SourceKindLogError
	default:
		return SourceKindAudit
	}
}
