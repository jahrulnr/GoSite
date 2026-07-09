package splunklite

// Op is a predicate operator in the search filter AST.
type Op int

const (
	OpContains Op = iota
	OpEq
	OpNe
	OpLike
	OpRegexp
	OpGt
	OpGte
	OpLt
	OpLte
)

// FilterExpr is the root of a Splunk-style search filter (before pipes).
type FilterExpr interface {
	filterExpr()
}

// AndExpr joins child expressions with AND.
type AndExpr struct {
	Children []FilterExpr
}

func (*AndExpr) filterExpr() {}

// OrExpr joins child expressions with OR.
type OrExpr struct {
	Children []FilterExpr
}

func (*OrExpr) filterExpr() {}

// NotExpr negates a child expression.
type NotExpr struct {
	Child FilterExpr
}

func (*NotExpr) filterExpr() {}

// PredExpr is a leaf predicate (field filter or free-text search).
type PredExpr struct {
	Field string // empty = search indexed text columns
	Op    Op
	Value string
}

func (*PredExpr) filterExpr() {}

// PipeCmd is a post-search command (| head, | sort).
type PipeCmd struct {
	Name   string
	Args   []string
	Limit  int    // head N
	Field  string // sort field
	Desc   bool   // sort -field
}

// And merges two expressions with AND, flattening nested And nodes.
func And(a, b FilterExpr) FilterExpr {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	var kids []FilterExpr
	for _, child := range flattenAnd(a) {
		kids = append(kids, child)
	}
	for _, child := range flattenAnd(b) {
		kids = append(kids, child)
	}
	if len(kids) == 1 {
		return kids[0]
	}
	return &AndExpr{Children: kids}
}

func flattenAnd(e FilterExpr) []FilterExpr {
	if e == nil {
		return nil
	}
	if and, ok := e.(*AndExpr); ok {
		return and.Children
	}
	return []FilterExpr{e}
}
