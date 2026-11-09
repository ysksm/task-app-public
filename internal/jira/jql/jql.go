// Package jql parses the subset of Jira Query Language described in
// docs/01-requirements.md §4.10.2 and translates it into a domain.Filter.
//
// The package is pure: it does not touch the database. Values that need a
// lookup (users, projects, statuses by name) are passed through as filter
// values that the store layer already understands.
package jql

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ErrorPrefix starts every error message, as in Jira. ORCA looks for "JQL"
// in the message to show its query hint.
const ErrorPrefix = "Error in the JQL Query: "

// Error is a JQL syntax or semantic error.
type Error struct{ Msg string }

func (e *Error) Error() string { return ErrorPrefix + e.Msg }

func errorf(format string, args ...any) error { return &Error{Msg: fmt.Sprintf(format, args...)} }

// ---- AST ----

// Expr is a parsed JQL expression.
type Expr interface{ isExpr() }

// And, Or and Not combine expressions.
type (
	And struct{ Items []Expr }
	Or  struct{ Items []Expr }
	Not struct{ Item Expr }
)

// Clause is a single `field op value` condition.
type Clause struct {
	Field  string  // lower-cased field name
	Op     string  // = != ~ !~ > >= < <= in "not in" "is" "is not"
	Values []Value // one value, or the list for in / not in; empty for is [not] empty
}

// Value is a literal or a function call such as currentUser().
type Value struct {
	Text     string
	Function bool // Text holds the lower-cased function name
	Args     []string
}

func (And) isExpr()    {}
func (Or) isExpr()     {}
func (Not) isExpr()    {}
func (Clause) isExpr() {}

// OrderBy is one sort term.
type OrderBy struct {
	Field string // lower-cased
	Desc  bool
}

// Query is a parsed JQL query. Where is nil when there is no condition.
type Query struct {
	Where Expr
	Order []OrderBy
}

// ---- lexer ----

type tokKind int

const (
	tEOF tokKind = iota
	tWord
	tString
	tOp
	tLParen
	tRParen
	tComma
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func lex(s string) ([]token, error) {
	var toks []token
	rs := []rune(s)
	i := 0
	for i < len(rs) {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '(':
			toks = append(toks, token{tLParen, "(", i})
			i++
		case r == ')':
			toks = append(toks, token{tRParen, ")", i})
			i++
		case r == ',':
			toks = append(toks, token{tComma, ",", i})
			i++
		case r == '"' || r == '\'':
			start := i
			i++
			var b strings.Builder
			for {
				if i >= len(rs) {
					return nil, errorf("The quoted string starting at character %d is not terminated.", start+1)
				}
				if rs[i] == '\\' && i+1 < len(rs) {
					b.WriteRune(rs[i+1])
					i += 2
					continue
				}
				if rs[i] == r {
					i++
					break
				}
				b.WriteRune(rs[i])
				i++
			}
			toks = append(toks, token{tString, b.String(), start})
		case strings.ContainsRune("=!~<>", r):
			start := i
			op := string(r)
			if i+1 < len(rs) && (rs[i+1] == '=' || (r == '!' && rs[i+1] == '~')) {
				op += string(rs[i+1])
			}
			i += len([]rune(op))
			if op == "!" {
				toks = append(toks, token{tWord, "not", start})
				continue
			}
			toks = append(toks, token{tOp, op, start})
		default:
			start := i
			for i < len(rs) && !unicode.IsSpace(rs[i]) && !strings.ContainsRune("(),=!~<>\"'", rs[i]) {
				i++
			}
			if i == start {
				return nil, errorf("The character '%c' at position %d is not allowed.", r, i+1)
			}
			toks = append(toks, token{tWord, string(rs[start:i]), start})
		}
	}
	return append(toks, token{tEOF, "", len(rs)}), nil
}

// ---- parser ----

type parser struct {
	toks []token
	i    int
}

// Parse parses a JQL query.
func Parse(s string) (Query, error) {
	toks, err := lex(s)
	if err != nil {
		return Query{}, err
	}
	p := &parser{toks: toks}
	var q Query
	if !p.atKeyword("order") && p.peek().kind != tEOF {
		if q.Where, err = p.or(); err != nil {
			return Query{}, err
		}
	}
	if p.atKeyword("order") {
		p.next()
		if !p.atKeyword("by") {
			return Query{}, p.expected("'by'")
		}
		p.next()
		for {
			t := p.next()
			if t.kind != tWord && t.kind != tString {
				return Query{}, p.expectedAt(t, "a field name")
			}
			ob := OrderBy{Field: strings.ToLower(t.text)}
			if p.atKeyword("asc") {
				p.next()
			} else if p.atKeyword("desc") {
				p.next()
				ob.Desc = true
			}
			q.Order = append(q.Order, ob)
			if p.peek().kind != tComma {
				break
			}
			p.next()
		}
	}
	if t := p.peek(); t.kind != tEOF {
		return Query{}, errorf("Expecting either 'OR' or 'AND' but got '%s'.", t.text)
	}
	return q, nil
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) atKeyword(kw string) bool {
	t := p.peek()
	return t.kind == tWord && strings.EqualFold(t.text, kw)
}

func (p *parser) expected(what string) error { return p.expectedAt(p.peek(), what) }

func (p *parser) expectedAt(t token, what string) error {
	if t.kind == tEOF {
		return errorf("Expecting %s but reached the end of the query.", what)
	}
	return errorf("Expecting %s but got '%s'.", what, t.text)
}

func (p *parser) or() (Expr, error) {
	first, err := p.and()
	if err != nil {
		return nil, err
	}
	items := []Expr{first}
	for p.atKeyword("or") || (p.peek().kind == tWord && p.peek().text == "||") {
		p.next()
		e, err := p.and()
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if len(items) == 1 {
		return first, nil
	}
	return Or{items}, nil
}

func (p *parser) and() (Expr, error) {
	first, err := p.not()
	if err != nil {
		return nil, err
	}
	items := []Expr{first}
	for p.atKeyword("and") || (p.peek().kind == tWord && p.peek().text == "&&") {
		p.next()
		e, err := p.not()
		if err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	if len(items) == 1 {
		return first, nil
	}
	return And{items}, nil
}

func (p *parser) not() (Expr, error) {
	if p.atKeyword("not") {
		p.next()
		e, err := p.not()
		if err != nil {
			return nil, err
		}
		return Not{e}, nil
	}
	if p.peek().kind == tLParen {
		p.next()
		e, err := p.or()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tRParen {
			return nil, p.expected("')'")
		}
		p.next()
		return e, nil
	}
	return p.clause()
}

func (p *parser) clause() (Expr, error) {
	ft := p.next()
	if ft.kind != tWord && ft.kind != tString {
		return nil, p.expectedAt(ft, "a field name")
	}
	c := Clause{Field: strings.ToLower(ft.text)}
	switch t := p.next(); {
	case t.kind == tOp:
		c.Op = t.text
	case t.kind == tWord && strings.EqualFold(t.text, "in"):
		c.Op = "in"
	case t.kind == tWord && strings.EqualFold(t.text, "not"):
		if !p.atKeyword("in") {
			return nil, p.expected("'in'")
		}
		p.next()
		c.Op = "not in"
	case t.kind == tWord && strings.EqualFold(t.text, "is"):
		c.Op = "is"
		if p.atKeyword("not") {
			p.next()
			c.Op = "is not"
		}
		if !p.atKeyword("empty") && !p.atKeyword("null") {
			return nil, p.expected("'EMPTY'")
		}
		p.next()
		return c, nil
	default:
		return nil, p.expectedAt(t, "an operator")
	}
	if c.Op == "in" || c.Op == "not in" {
		if p.peek().kind != tLParen {
			return nil, p.expected("'('")
		}
		p.next()
		for {
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			c.Values = append(c.Values, v)
			if p.peek().kind == tComma {
				p.next()
				continue
			}
			if p.peek().kind != tRParen {
				return nil, p.expected("')'")
			}
			p.next()
			break
		}
		return c, nil
	}
	if p.atKeyword("empty") || p.atKeyword("null") {
		p.next()
		switch c.Op {
		case "=":
			c.Op = "is"
		case "!=":
			c.Op = "is not"
		default:
			return nil, errorf("The operator '%s' does not support the value EMPTY.", c.Op)
		}
		return c, nil
	}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	c.Values = []Value{v}
	return c, nil
}

func (p *parser) value() (Value, error) {
	t := p.next()
	switch t.kind {
	case tString:
		return Value{Text: t.text}, nil
	case tWord:
		if p.peek().kind != tLParen {
			return Value{Text: t.text}, nil
		}
		p.next()
		v := Value{Text: strings.ToLower(t.text), Function: true}
		for p.peek().kind != tRParen {
			a := p.next()
			switch a.kind {
			case tWord, tString:
				v.Args = append(v.Args, a.text)
			case tComma:
			default:
				return Value{}, p.expectedAt(a, "')'")
			}
		}
		p.next()
		return v, nil
	}
	return Value{}, p.expectedAt(t, "a value")
}

// ---- helpers shared with the translator ----

// relative parses Jira relative dates such as "-7d", "2w", "-4h", "30m".
func relative(s string, now time.Time) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return time.Time{}, false
	}
	sign := 1
	body := s
	if body[0] == '-' || body[0] == '+' {
		if body[0] == '-' {
			sign = -1
		}
		body = body[1:]
	}
	unit := body[len(body)-1]
	n := 0
	for _, r := range body[:len(body)-1] {
		if r < '0' || r > '9' {
			return time.Time{}, false
		}
		n = n*10 + int(r-'0')
	}
	if len(body) < 2 {
		return time.Time{}, false
	}
	n *= sign
	switch unicode.ToLower(rune(unit)) {
	case 'm':
		return now.Add(time.Duration(n) * time.Minute), true
	case 'h':
		return now.Add(time.Duration(n) * time.Hour), true
	case 'd':
		return now.AddDate(0, 0, n), true
	case 'w':
		return now.AddDate(0, 0, 7*n), true
	}
	return time.Time{}, false
}
