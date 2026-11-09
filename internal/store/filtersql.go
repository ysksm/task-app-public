package store

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
)

// FilterSQL converts a domain.Filter into a SQL boolean expression plus args.
// The expression assumes these table aliases in the surrounding query:
//
//	i  = issues, p = projects, st = workflow_states, a = users (assignee, LEFT JOIN)
//
// meUserID is substituted for the value "me" on user fields.
func FilterSQL(f domain.Filter, meUserID string) (string, []any, error) {
	b := &builder{me: meUserID}
	expr, err := b.node(f)
	if err != nil {
		return "", nil, err
	}
	return expr, b.args, nil
}

type builder struct {
	me   string
	args []any
}

func (b *builder) arg(v any) string {
	b.args = append(b.args, v)
	return "?"
}

func (b *builder) node(f domain.Filter) (string, error) {
	switch {
	case f.And != nil:
		return b.group(f.And, " AND ", "1=1")
	case f.Or != nil:
		return b.group(f.Or, " OR ", "1=0")
	case f.Not != nil:
		inner, err := b.node(*f.Not)
		if err != nil {
			return "", err
		}
		return "NOT (" + inner + ")", nil
	case f.Field != "":
		return b.cond(f)
	}
	return "1=1", nil
}

func (b *builder) group(children []domain.Filter, sep, empty string) (string, error) {
	if len(children) == 0 {
		return empty, nil
	}
	parts := make([]string, 0, len(children))
	for _, c := range children {
		s, err := b.node(c)
		if err != nil {
			return "", err
		}
		parts = append(parts, "("+s+")")
	}
	return strings.Join(parts, sep), nil
}

func (b *builder) cond(f domain.Filter) (string, error) {
	switch f.Field {
	case domain.FieldText:
		v, _ := f.Scalar()
		like := "%" + escapeLike(v) + "%"
		return "(i.title LIKE " + b.arg(like) + " ESCAPE '\\' OR i.description LIKE " + b.arg(like) + " ESCAPE '\\' OR i.key = " + b.arg(strings.ToUpper(v)) + ")", nil
	case domain.FieldTitle:
		v, _ := f.Scalar()
		return "i.title LIKE " + b.arg("%"+escapeLike(v)+"%") + " ESCAPE '\\'", nil
	case domain.FieldLabel:
		return b.labelCond(f)
	case domain.FieldAncestor:
		v, _ := f.Scalar()
		return `i.id IN (WITH RECURSIVE sub(id) AS (
				SELECT c.id FROM issues r CROSS JOIN issues c ON c.parent_id = r.id WHERE r.key = ` + b.arg(strings.ToUpper(v)) + ` AND c.deleted_at IS NULL
				UNION ALL SELECT c.id FROM sub CROSS JOIN issues c ON c.parent_id = sub.id WHERE c.deleted_at IS NULL)
			SELECT id FROM sub)`, nil
	}

	var expr string
	conv := func(s string) (any, error) { return s, nil }
	switch f.Field {
	case domain.FieldProject:
		expr = "p.key"
		conv = func(s string) (any, error) { return strings.ToUpper(s), nil }
	case domain.FieldProjectTier:
		expr = "p.tier"
		conv = func(s string) (any, error) {
			if !domain.ValidTier(s) {
				return nil, fmt.Errorf("unknown project tier %q (focus, normal, side)", s)
			}
			return s, nil
		}
	case domain.FieldKey:
		expr = "i.key"
		conv = func(s string) (any, error) { return strings.ToUpper(s), nil }
	case domain.FieldType:
		expr = "i.type"
		conv = func(s string) (any, error) {
			if !domain.IssueType(s).Valid() {
				return nil, fmt.Errorf("unknown issue type %q", s)
			}
			return s, nil
		}
	case domain.FieldState:
		expr = "i.state_id"
		conv = func(s string) (any, error) {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("state must be a state id: %q", s)
			}
			return n, nil
		}
	case domain.FieldStateName:
		expr = "LOWER(st.name)"
		conv = func(s string) (any, error) { return strings.ToLower(s), nil }
	case domain.FieldStateCategory:
		expr = "st.category"
	case domain.FieldAssignee:
		return b.userCond(f, "i.assignee_id")
	case domain.FieldAssigneeKind:
		expr = "a.kind"
	case domain.FieldReporter:
		return b.userCond(f, "i.reporter_id")
	case domain.FieldPriority:
		expr = "i.priority"
		conv = func(s string) (any, error) {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 || n > 4 {
				return nil, fmt.Errorf("priority must be 0-4: %q", s)
			}
			return n, nil
		}
	case domain.FieldParent:
		expr = "i.parent_id"
		if f.Op == domain.OpEmpty || f.Op == domain.OpNotEmpty {
			break
		}
		return b.keyRefCond(f, expr)
	case domain.FieldDueDate:
		expr = "i.due_date"
	case domain.FieldCreated:
		expr = "i.created_at"
	case domain.FieldUpdated:
		expr = "i.updated_at"
	default:
		return "", fmt.Errorf("unsupported field %q", f.Field)
	}
	return b.compare(f, expr, conv)
}

func (b *builder) compare(f domain.Filter, expr string, conv func(string) (any, error)) (string, error) {
	switch f.Op {
	case domain.OpEmpty:
		return expr + " IS NULL", nil
	case domain.OpNotEmpty:
		return expr + " IS NOT NULL", nil
	case domain.OpIn, domain.OpNotIn:
		vals, err := f.Values()
		if err != nil {
			return "", err
		}
		if len(vals) == 0 {
			if f.Op == domain.OpIn {
				return "1=0", nil
			}
			return "1=1", nil
		}
		ph := make([]string, len(vals))
		for i, v := range vals {
			cv, err := conv(v)
			if err != nil {
				return "", err
			}
			ph[i] = b.arg(cv)
		}
		if f.Op == domain.OpIn {
			return expr + " IN (" + strings.Join(ph, ",") + ")", nil
		}
		return "(" + expr + " IS NULL OR " + expr + " NOT IN (" + strings.Join(ph, ",") + "))", nil
	}
	v, err := f.Scalar()
	if err != nil {
		return "", err
	}
	cv, err := conv(v)
	if err != nil {
		return "", err
	}
	ops := map[string]string{domain.OpEq: "=", domain.OpLt: "<", domain.OpLte: "<=", domain.OpGt: ">", domain.OpGte: ">="}
	if f.Op == domain.OpNeq {
		return "(" + expr + " IS NULL OR " + expr + " <> " + b.arg(cv) + ")", nil
	}
	op, ok := ops[f.Op]
	if !ok {
		return "", fmt.Errorf("unsupported operator %q", f.Op)
	}
	return expr + " " + op + " " + b.arg(cv), nil
}

// userCond compares a user id column with users given by username, email or
// id ("me" = current user).
func (b *builder) userCond(f domain.Filter, col string) (string, error) {
	switch f.Op {
	case domain.OpEmpty:
		return col + " IS NULL", nil
	case domain.OpNotEmpty:
		return col + " IS NOT NULL", nil
	}
	vals, err := valuesOf(f)
	if err != nil {
		return "", err
	}
	idSet := make([]string, 0, len(vals))
	var refs []string
	for _, v := range vals {
		if v == "me" {
			idSet = append(idSet, b.arg(b.me))
		} else {
			refs = append(refs, v)
		}
	}
	// Each placeholder binds one argument, so every list gets its own.
	list := func(conv func(string) string) string {
		ph := make([]string, len(refs))
		for i, v := range refs {
			ph[i] = b.arg(conv(v))
		}
		return strings.Join(ph, ",")
	}
	same := func(v string) string { return v }
	match := []string{}
	if len(idSet) > 0 {
		match = append(match, col+" IN ("+strings.Join(idSet, ",")+")")
	}
	if len(refs) > 0 {
		match = append(match, col+" IN (SELECT id FROM users WHERE username IN ("+list(same)+") OR id IN ("+list(same)+
			") OR (email <> '' AND LOWER(email) IN ("+list(strings.ToLower)+")))")
	}
	expr := "1=0"
	if len(match) > 0 {
		expr = "(" + strings.Join(match, " OR ") + ")"
	}
	if f.Op == domain.OpNeq || f.Op == domain.OpNotIn {
		return "(" + col + " IS NULL OR NOT " + expr + ")", nil
	}
	return expr, nil
}

// keyRefCond compares an issue id column with issue keys.
func (b *builder) keyRefCond(f domain.Filter, col string) (string, error) {
	vals, err := valuesOf(f)
	if err != nil {
		return "", err
	}
	ph := make([]string, len(vals))
	for i, v := range vals {
		ph[i] = b.arg(strings.ToUpper(v))
	}
	expr := col + " IN (SELECT id FROM issues WHERE key IN (" + strings.Join(ph, ",") + "))"
	if len(vals) == 0 {
		expr = "1=0"
	}
	if f.Op == domain.OpNeq || f.Op == domain.OpNotIn {
		return "(" + col + " IS NULL OR NOT " + expr + ")", nil
	}
	return expr, nil
}

func (b *builder) labelCond(f domain.Filter) (string, error) {
	const has = "EXISTS (SELECT 1 FROM issue_labels il JOIN labels l ON l.id = il.label_id WHERE il.issue_id = i.id"
	switch f.Op {
	case domain.OpEmpty:
		return "NOT " + has + ")", nil
	case domain.OpNotEmpty:
		return has + ")", nil
	}
	vals, err := valuesOf(f)
	if err != nil {
		return "", err
	}
	ph := make([]string, len(vals))
	for i, v := range vals {
		ph[i] = b.arg(v)
	}
	expr := has + " AND l.name IN (" + strings.Join(ph, ",") + "))"
	if len(vals) == 0 {
		expr = "1=0"
	}
	if f.Op == domain.OpNeq || f.Op == domain.OpNotIn {
		return "NOT " + expr, nil
	}
	return expr, nil
}

func valuesOf(f domain.Filter) ([]string, error) {
	if f.Op == domain.OpIn || f.Op == domain.OpNotIn {
		return f.Values()
	}
	v, err := f.Scalar()
	if err != nil {
		return nil, err
	}
	return []string{v}, nil
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
