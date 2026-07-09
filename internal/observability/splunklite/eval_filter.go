package splunklite

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EvalFilter evaluates a filter AST against a normalized query event (live tail).
func EvalFilter(expr FilterExpr, ev QueryEvent, kind SourceKind) bool {
	if expr == nil {
		return true
	}
	fields := fieldsForKind(kind)
	if fields == nil {
		return false
	}
	if !exprApplicable(expr, fields) {
		return false
	}
	return evalNode(expr, ev, kind, fields)
}

func evalNode(expr FilterExpr, ev QueryEvent, kind SourceKind, fields map[string]string) bool {
	switch e := expr.(type) {
	case *AndExpr:
		for _, child := range e.Children {
			if !evalNode(child, ev, kind, fields) {
				return false
			}
		}
		return true
	case *OrExpr:
		for _, child := range e.Children {
			if evalNode(child, ev, kind, fields) {
				return true
			}
		}
		return false
	case *NotExpr:
		return !evalNode(e.Child, ev, kind, fields)
	case *PredExpr:
		return evalPred(e, ev, kind, fields)
	default:
		return false
	}
}

func evalPred(p *PredExpr, ev QueryEvent, kind SourceKind, fields map[string]string) bool {
	if p.Field == "" {
		return evalFreeText(p, ev, kind)
	}
	if _, ok := fields[p.Field]; !ok {
		return false
	}
	value, ok := eventFieldValue(ev, p.Field, kind)
	if !ok {
		return false
	}
	switch p.Op {
	case OpRegexp:
		matched, err := regexp.MatchString(p.Value, value)
		return err == nil && matched
	case OpLike:
		return matchWildcard(value, p.Value)
	case OpEq:
		if strings.Contains(p.Value, "*") {
			return matchWildcard(value, p.Value)
		}
		return strings.EqualFold(value, p.Value) || strings.Contains(strings.ToLower(value), strings.ToLower(p.Value))
	case OpNe:
		return !strings.EqualFold(value, p.Value)
	case OpGt, OpGte, OpLt, OpLte:
		num, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return false
		}
		cmp, err := strconv.Atoi(strings.TrimSpace(p.Value))
		if err != nil {
			return false
		}
		switch p.Op {
		case OpGt:
			return num > cmp
		case OpGte:
			return num >= cmp
		case OpLt:
			return num < cmp
		case OpLte:
			return num <= cmp
		}
	case OpContains:
		return strings.Contains(strings.ToLower(value), strings.ToLower(p.Value))
	}
	return false
}

func evalFreeText(p *PredExpr, ev QueryEvent, kind SourceKind) bool {
	cols := freeTextColumns(kind)
	for _, col := range cols {
		value, ok := eventColumnValue(ev, col, kind)
		if !ok {
			continue
		}
		switch p.Op {
		case OpRegexp:
			if matched, err := regexp.MatchString(p.Value, value); err == nil && matched {
				return true
			}
		case OpLike:
			if matchWildcard(value, p.Value) {
				return true
			}
		default:
			if strings.Contains(strings.ToLower(value), strings.ToLower(p.Value)) {
				return true
			}
		}
	}
	return false
}

func eventFieldValue(ev QueryEvent, field string, kind SourceKind) (string, bool) {
	field = strings.ToLower(strings.TrimSpace(field))
	switch field {
	case "status", "status_code":
		if kind == SourceKindLogAccess {
			if ev.Meta != nil {
				if v, ok := ev.Meta["status_code"]; ok {
					return fmt.Sprint(v), true
				}
			}
			return "", false
		}
		if kind == SourceKindAudit || kind == SourceKindJob {
			if ev.Meta != nil {
				if v, ok := ev.Meta["status"]; ok {
					return fmt.Sprint(v), true
				}
			}
			if kind == SourceKindJob && ev.Message != "" {
				return "", true
			}
		}
		return "", false
	case "site":
		if ev.Meta != nil {
			if v, ok := ev.Meta["site"]; ok {
				return fmt.Sprint(v), true
			}
		}
		return "", false
	case "source":
		return ev.Source, true
	case "action", "job_type", "type":
		return ev.Action, true
	case "user", "user_email", "email":
		return ev.User, true
	case "message", "output", "error", "name", "preview":
		return ev.Message, true
	default:
		if ev.Meta == nil {
			return "", false
		}
		value, ok := ev.Meta[field]
		if !ok {
			return "", false
		}
		return fmt.Sprint(value), true
	}
}

func eventColumnValue(ev QueryEvent, column string, kind SourceKind) (string, bool) {
	switch column {
	case "user_email":
		return ev.User, true
	case "action":
		return ev.Action, true
	case "message", "raw_preview":
		return ev.Message, true
	case "name", "output", "error", "job_type":
		return ev.Message, true
	case "site":
		if ev.Meta != nil {
			if v, ok := ev.Meta["site"]; ok {
				return fmt.Sprint(v), true
			}
		}
		return "", false
	default:
		return eventFieldValue(ev, column, kind)
	}
}

func matchWildcard(value, pattern string) bool {
	value = strings.ToLower(value)
	pattern = strings.ToLower(pattern)
	if strings.Contains(pattern, "*") {
		re := regexp.QuoteMeta(pattern)
		re = strings.ReplaceAll(re, `\*`, ".*")
		matched, err := regexp.MatchString("^"+re+"$", value)
		return err == nil && matched
	}
	return value == pattern || strings.Contains(value, pattern)
}
