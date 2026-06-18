package splunklite

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type searchError string

func (e searchError) Error() string { return string(e) }

func errSearch(msg string) error { return searchError(msg) }

// ParseSearch parses a Splunk-style search string into a filter AST and pipe commands.
func ParseSearch(q string) (FilterExpr, []PipeCmd, error) {
	q = strings.TrimSpace(q)
	if q == "" || q == "*" {
		return nil, nil, nil
	}
	searchPart, pipeParts, err := splitPipes(q)
	if err != nil {
		return nil, nil, err
	}
	var pipes []PipeCmd
	for _, part := range pipeParts {
		cmd, err := parsePipe(part)
		if err != nil {
			return nil, nil, err
		}
		pipes = append(pipes, cmd)
	}
	if strings.TrimSpace(searchPart) == "" {
		return nil, pipes, nil
	}
	lex := newLexer(searchPart)
	p := &parser{lex: lex}
	expr, err := p.parseSearch()
	if err != nil {
		return nil, nil, err
	}
	if lex.peek().kind != tokEOF {
		return nil, nil, errSearch("unexpected token " + lex.peek().text)
	}
	return expr, pipes, nil
}

func splitPipes(q string) (search string, pipes []string, err error) {
	var parts []string
	var buf strings.Builder
	inQuote := false
	inRegex := false
	escaped := false
	for i := 0; i < len(q); i++ {
		ch := q[i]
		if escaped {
			buf.WriteByte(ch)
			escaped = false
			continue
		}
		if inRegex {
			buf.WriteByte(ch)
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '/' && i > 0 {
				inRegex = false
			}
			continue
		}
		if inQuote {
			buf.WriteByte(ch)
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inQuote = false
			}
			continue
		}
		switch ch {
		case '"':
			inQuote = true
			buf.WriteByte(ch)
		case '/':
			inRegex = true
			buf.WriteByte(ch)
		case '|':
			parts = append(parts, strings.TrimSpace(buf.String()))
			buf.Reset()
		default:
			buf.WriteByte(ch)
		}
	}
	if inQuote {
		return "", nil, errSearch("unterminated quote")
	}
	if inRegex {
		return "", nil, errSearch("unterminated regex")
	}
	parts = append(parts, strings.TrimSpace(buf.String()))
	if len(parts) == 0 {
		return "", nil, nil
	}
	search = parts[0]
	for _, p := range parts[1:] {
		if p != "" {
			pipes = append(pipes, p)
		}
	}
	return search, pipes, nil
}

func parsePipe(part string) (PipeCmd, error) {
	fields := strings.Fields(part)
	if len(fields) == 0 {
		return PipeCmd{}, errSearch("empty pipe command")
	}
	name := strings.ToLower(fields[0])
	switch name {
	case "head":
		if len(fields) < 2 {
			return PipeCmd{}, errSearch("head requires a limit")
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil || n <= 0 {
			return PipeCmd{}, errSearch("head limit must be a positive integer")
		}
		return PipeCmd{Name: "head", Limit: n, Args: fields[1:]}, nil
	case "sort":
		if len(fields) < 2 {
			return PipeCmd{}, errSearch("sort requires a field")
		}
		field := fields[1]
		desc := false
		if strings.HasPrefix(field, "-") {
			desc = true
			field = strings.TrimPrefix(field, "-")
		}
		if field == "" {
			return PipeCmd{}, errSearch("sort field required")
		}
		return PipeCmd{Name: "sort", Field: field, Desc: desc, Args: fields[1:]}, nil
	default:
		return PipeCmd{}, errSearch("unknown pipe command " + name)
	}
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokWord
	tokQuoted
	tokRegex
	tokLParen
	tokRParen
	tokAND
	tokOR
	tokNOT
)

type token struct {
	kind tokenKind
	text string
}

type lexer struct {
	input string
	pos   int
}

func newLexer(input string) *lexer {
	return &lexer{input: strings.TrimSpace(input)}
}

func (l *lexer) peek() token {
	tok, _ := l.read(false)
	return tok
}

func (l *lexer) next() token {
	tok, _ := l.read(true)
	return tok
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *lexer) read(advance bool) (token, error) {
	l.skipSpace()
	if l.pos >= len(l.input) {
		return token{kind: tokEOF}, nil
	}
	start := l.pos
	ch := l.input[l.pos]

	switch ch {
	case '(':
		if advance {
			l.pos++
		}
		return token{kind: tokLParen, text: "("}, nil
	case ')':
		if advance {
			l.pos++
		}
		return token{kind: tokRParen, text: ")"}, nil
	case '"':
		return l.readQuoted(advance)
	case '/':
		return l.readRegex(advance)
	}

	// word or keyword or field predicate chunk
	for l.pos < len(l.input) {
		c := l.input[l.pos]
		if c == '(' || c == ')' || c == '"' || c == '/' || unicode.IsSpace(rune(c)) {
			break
		}
		l.pos++
	}
	word := l.input[start:l.pos]
	if !advance {
		l.pos = start
	}
	upper := strings.ToUpper(word)
	switch upper {
	case "AND":
		return token{kind: tokAND, text: word}, nil
	case "OR":
		return token{kind: tokOR, text: word}, nil
	case "NOT":
		return token{kind: tokNOT, text: word}, nil
	}
	return token{kind: tokWord, text: word}, nil
}

func (l *lexer) readQuoted(advance bool) (token, error) {
	start := l.pos
	l.pos++ // opening "
	escaped := false
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if escaped {
			escaped = false
			l.pos++
			continue
		}
		if ch == '\\' {
			escaped = true
			l.pos++
			continue
		}
		if ch == '"' {
			l.pos++
			text := l.input[start+1 : l.pos-1]
			if !advance {
				l.pos = start
			}
			return token{kind: tokQuoted, text: unescapeQuoted(text)}, nil
		}
		l.pos++
	}
	return token{}, errSearch("unterminated quote")
}

func (l *lexer) readRegex(advance bool) (token, error) {
	start := l.pos
	l.pos++
	escaped := false
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if escaped {
			escaped = false
			l.pos++
			continue
		}
		if ch == '\\' {
			escaped = true
			l.pos++
			continue
		}
		if ch == '/' {
			l.pos++
			pattern := l.input[start+1 : l.pos-1]
			if err := validateRegexp(pattern); err != nil {
				return token{}, errSearch(err.Error())
			}
			if !advance {
				l.pos = start
			}
			return token{kind: tokRegex, text: pattern}, nil
		}
		l.pos++
	}
	return token{}, errSearch("unterminated regex")
}

func unescapeQuoted(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

type parser struct {
	lex *lexer
}

func (p *parser) parseSearch() (FilterExpr, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (FilterExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.lex.peek().kind == tokOR {
		p.lex.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		if or, ok := left.(*OrExpr); ok {
			or.Children = append(or.Children, right)
		} else {
			left = &OrExpr{Children: []FilterExpr{left, right}}
		}
	}
	return left, nil
}

func (p *parser) parseAnd() (FilterExpr, error) {
	var parts []FilterExpr
	for {
		if p.lex.peek().kind == tokEOF || p.lex.peek().kind == tokRParen || p.lex.peek().kind == tokOR {
			break
		}
		if p.lex.peek().kind == tokAND {
			p.lex.next()
		}
		if p.lex.peek().kind == tokEOF || p.lex.peek().kind == tokRParen || p.lex.peek().kind == tokOR {
			break
		}
		part, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return nil, errSearch("expected search term")
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return &AndExpr{Children: parts}, nil
}

func (p *parser) parseUnary() (FilterExpr, error) {
	if p.lex.peek().kind == tokNOT {
		p.lex.next()
		child, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &NotExpr{Child: child}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (FilterExpr, error) {
	tok := p.lex.peek()
	switch tok.kind {
	case tokLParen:
		p.lex.next()
		inner, err := p.parseSearch()
		if err != nil {
			return nil, err
		}
		if p.lex.next().kind != tokRParen {
			return nil, errSearch("expected )")
		}
		return inner, nil
	case tokQuoted:
		p.lex.next()
		return &PredExpr{Op: OpContains, Value: tok.text}, nil
	case tokRegex:
		p.lex.next()
		return &PredExpr{Op: OpRegexp, Value: tok.text}, nil
	case tokWord:
		return p.parseWordPredicate(tok.text)
	default:
		return nil, errSearch("expected search term")
	}
}

func (p *parser) parseWordPredicate(word string) (FilterExpr, error) {
	p.lex.next()
	if pred, ok := parseFieldPredicate(word); ok {
		return pred, nil
	}
	if field, op, ok := fieldOpSuffix(word); ok {
		switch p.lex.peek().kind {
		case tokRegex:
			pattern := p.lex.next().text
			return &PredExpr{Field: field, Op: OpRegexp, Value: pattern}, nil
		case tokQuoted:
			value := p.lex.next().text
			if op == OpEq && strings.Contains(value, "*") {
				op = OpLike
			}
			return &PredExpr{Field: field, Op: op, Value: value}, nil
		case tokWord:
			// field=value where value is another bare token (no spaces).
			value := p.lex.next().text
			if op == OpEq && strings.Contains(value, "*") {
				op = OpLike
			}
			return &PredExpr{Field: field, Op: op, Value: value}, nil
		}
	}
	return &PredExpr{Op: OpContains, Value: word}, nil
}

func fieldOpSuffix(word string) (field string, op Op, ok bool) {
	ops := []struct {
		sep string
		op  Op
	}{
		{">=", OpGte},
		{"<=", OpLte},
		{"!=", OpNe},
		{">", OpGt},
		{"<", OpLt},
		{"=", OpEq},
		{":", OpEq},
	}
	for _, item := range ops {
		if !strings.HasSuffix(word, item.sep) {
			continue
		}
		field = strings.ToLower(strings.TrimSpace(word[:len(word)-len(item.sep)]))
		if isValidFieldName(field) {
			return field, item.op, true
		}
	}
	return "", 0, false
}

func parseFieldPredicate(word string) (*PredExpr, bool) {
	ops := []struct {
		sep string
		op  Op
	}{
		{">=", OpGte},
		{"<=", OpLte},
		{"!=", OpNe},
		{">", OpGt},
		{"<", OpLt},
		{"=", OpEq},
		{":", OpEq},
	}
	for _, item := range ops {
		idx := strings.Index(word, item.sep)
		if idx <= 0 {
			continue
		}
		field := strings.ToLower(strings.TrimSpace(word[:idx]))
		if !isValidFieldName(field) {
			continue
		}
		value := strings.TrimSpace(word[idx+len(item.sep):])
		if value == "" {
			return nil, false
		}
		op := item.op
		if op == OpEq && strings.Contains(value, "*") {
			op = OpLike
		}
		if strings.HasPrefix(value, "/") && strings.HasSuffix(value, "/") && len(value) >= 2 {
			pattern := value[1 : len(value)-1]
			if err := validateRegexp(pattern); err != nil {
				return nil, false
			}
			return &PredExpr{Field: field, Op: OpRegexp, Value: pattern}, true
		}
		return &PredExpr{Field: field, Op: op, Value: value}, true
	}
	return nil, false
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
	return s != ""
}

func validateRegexp(pattern string) error {
	if pattern == "" {
		return nil
	}
	_, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	return nil
}
