package jql

import (
	"strconv"
	"strings"
	"time"

	"github.com/ysksm/task-app/internal/domain"
)

// Options supplies the context a translation needs.
type Options struct {
	Now      time.Time      // for relative dates; defaults to time.Now()
	Location *time.Location // time zone of date literals; defaults to time.Local
	// ProjectKey maps a project id or key to its key. Unknown values are
	// passed through unchanged (and then simply match nothing).
	ProjectKey func(idOrKey string) string
}

// Result is a translated query.
type Result struct {
	Filter domain.Filter
	Sort   string // a service.ListOptions sort key
}

// Compile parses and translates a JQL query.
func Compile(s string, opt Options) (Result, error) {
	q, err := Parse(s)
	if err != nil {
		return Result{}, err
	}
	return Translate(q, opt)
}

// Translate converts a parsed query into a filter and a sort key.
func Translate(q Query, opt Options) (Result, error) {
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	if opt.Location == nil {
		opt.Location = time.Local
	}
	t := translator{opt}
	var res Result
	if q.Where != nil {
		f, err := t.expr(q.Where)
		if err != nil {
			return Result{}, err
		}
		res.Filter = f
	}
	sort, err := sortKey(q.Order)
	if err != nil {
		return Result{}, err
	}
	res.Sort = sort
	return res, nil
}

// IssueTypes maps issue type names and fixed ids (docs/02 §7.1) to internal types.
var IssueTypes = map[string]domain.IssueType{
	"epic": domain.TypeEpic, "10000": domain.TypeEpic,
	"story": domain.TypeStory, "10001": domain.TypeStory,
	"feature": domain.TypeFeature, "10002": domain.TypeFeature,
	"task": domain.TypeTask, "10003": domain.TypeTask,
	"question": domain.TypeQA, "qa": domain.TypeQA, "10004": domain.TypeQA,
}

// Priorities maps Jira priority names and ids to internal priorities (0 = none).
var Priorities = map[string]int{
	"highest": domain.PriorityUrgent, "1": domain.PriorityUrgent,
	"high": domain.PriorityHigh, "2": domain.PriorityHigh,
	"medium": domain.PriorityMedium, "3": domain.PriorityMedium,
	"low": domain.PriorityLow, "4": domain.PriorityLow,
	"lowest": domain.PriorityNone, "none": domain.PriorityNone, "5": domain.PriorityNone,
}

// statusCategories maps Jira status categories to internal categories.
var statusCategories = map[string][]string{
	"new": {"backlog", "unstarted"}, "to do": {"backlog", "unstarted"}, "todo": {"backlog", "unstarted"}, "2": {"backlog", "unstarted"},
	"indeterminate": {"started"}, "in progress": {"started"}, "4": {"started"},
	"done": {"completed", "canceled"}, "complete": {"completed", "canceled"}, "3": {"completed", "canceled"},
}

var resolutions = map[string][]string{
	"done": {"completed"}, "fixed": {"completed"},
	"won't do": {"canceled"}, "wont do": {"canceled"}, "won't fix": {"canceled"}, "cancelled": {"canceled"}, "canceled": {"canceled"},
}

var closedCategories = []string{"completed", "canceled"}

type translator struct{ opt Options }

func (t translator) expr(e Expr) (domain.Filter, error) {
	switch e := e.(type) {
	case And:
		fs, err := t.list(e.Items)
		return domain.Filter{And: fs}, err
	case Or:
		fs, err := t.list(e.Items)
		return domain.Filter{Or: fs}, err
	case Not:
		f, err := t.expr(e.Item)
		return domain.Filter{Not: &f}, err
	case Clause:
		return t.clause(e)
	}
	return domain.Filter{}, errorf("Unsupported expression.")
}

func (t translator) list(items []Expr) ([]domain.Filter, error) {
	out := make([]domain.Filter, 0, len(items))
	for _, it := range items {
		f, err := t.expr(it)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func leaf(field, op string, v any) domain.Filter {
	return domain.Filter{Field: field, Op: op, Value: v}
}

func anyList(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// equality turns =, !=, in and not in into a filter over mapped values.
func (t translator) equality(c Clause, field string, mapValue func(Value) ([]string, error)) (domain.Filter, error) {
	var vals []string
	for _, v := range c.Values {
		mv, err := mapValue(v)
		if err != nil {
			return domain.Filter{}, err
		}
		vals = append(vals, mv...)
	}
	switch c.Op {
	case "=", "in":
		if len(vals) == 1 {
			return leaf(field, domain.OpEq, vals[0]), nil
		}
		return leaf(field, domain.OpIn, anyList(vals)), nil
	case "!=", "not in":
		if len(vals) == 1 {
			return leaf(field, domain.OpNeq, vals[0]), nil
		}
		return leaf(field, domain.OpNotIn, anyList(vals)), nil
	}
	return domain.Filter{}, opError(c)
}

func opError(c Clause) error {
	return errorf("The operator '%s' is not supported by the '%s' field.", strings.ToUpper(c.Op), c.Field)
}

func literal(c Clause) func(Value) ([]string, error) {
	return func(v Value) ([]string, error) {
		if v.Function {
			return nil, errorf("Unable to find JQL function '%s()' for the '%s' field.", v.Text, c.Field)
		}
		return []string{v.Text}, nil
	}
}

func (t translator) clause(c Clause) (domain.Filter, error) {
	isEmpty := c.Op == "is"
	emptyTest := c.Op == "is" || c.Op == "is not"
	emptyOp := domain.OpNotEmpty
	if isEmpty {
		emptyOp = domain.OpEmpty
	}

	switch c.Field {
	case "project":
		if emptyTest {
			return domain.Filter{}, opError(c)
		}
		return t.equality(c, domain.FieldProject, func(v Value) ([]string, error) {
			vals, err := literal(c)(v)
			if err != nil || t.opt.ProjectKey == nil {
				return vals, err
			}
			return []string{t.opt.ProjectKey(vals[0])}, nil
		})

	case "key", "issuekey", "issue", "id":
		if emptyTest {
			return domain.Filter{}, opError(c)
		}
		return t.equality(c, domain.FieldKey, literal(c))

	case "issuetype", "type":
		if emptyTest {
			return domain.Filter{}, opError(c)
		}
		return t.equality(c, domain.FieldType, func(v Value) ([]string, error) {
			typ, ok := IssueTypes[strings.ToLower(v.Text)]
			if !ok || v.Function {
				return nil, errorf("The value '%s' does not exist for the field 'issuetype'.", v.Text)
			}
			return []string{string(typ)}, nil
		})

	case "status":
		if emptyTest {
			return domain.Filter{}, opError(c)
		}
		// A status is given by id (numeric) or by name; split the two.
		var ids, names []Value
		for _, v := range c.Values {
			if _, err := strconv.ParseInt(v.Text, 10, 64); err == nil && !v.Function {
				ids = append(ids, v)
			} else {
				names = append(names, v)
			}
		}
		parts := []domain.Filter{}
		for _, grp := range []struct {
			field string
			vals  []Value
		}{{domain.FieldState, ids}, {domain.FieldStateName, names}} {
			if len(grp.vals) == 0 {
				continue
			}
			sub := c
			sub.Values = grp.vals
			if sub.Op == "=" && len(grp.vals) > 1 {
				sub.Op = "in"
			}
			f, err := t.equality(sub, grp.field, literal(c))
			if err != nil {
				return domain.Filter{}, err
			}
			parts = append(parts, f)
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
		if c.Op == "!=" || c.Op == "not in" {
			return domain.Filter{And: parts}, nil
		}
		return domain.Filter{Or: parts}, nil

	case "statuscategory":
		if emptyTest {
			return domain.Filter{}, opError(c)
		}
		f, err := t.equality(c, domain.FieldStateCategory, func(v Value) ([]string, error) {
			cats, ok := statusCategories[strings.ToLower(v.Text)]
			if !ok || v.Function {
				return nil, errorf("The value '%s' does not exist for the field 'statusCategory'.", v.Text)
			}
			return cats, nil
		})
		return f, err

	case "resolution":
		// Unresolved = any state that is not completed/canceled.
		if emptyTest {
			if isEmpty {
				return leaf(domain.FieldStateCategory, domain.OpNotIn, anyList(closedCategories)), nil
			}
			return leaf(domain.FieldStateCategory, domain.OpIn, anyList(closedCategories)), nil
		}
		if c.Op != "=" && c.Op != "!=" && c.Op != "in" && c.Op != "not in" {
			return domain.Filter{}, opError(c)
		}
		var cats []string
		unresolved := false
		for _, v := range c.Values {
			key := strings.ToLower(v.Text)
			if key == "unresolved" {
				unresolved = true
				continue
			}
			rc, ok := resolutions[key]
			if !ok || v.Function {
				return domain.Filter{}, errorf("The value '%s' does not exist for the field 'resolution'.", v.Text)
			}
			cats = append(cats, rc...)
		}
		negate := c.Op == "!=" || c.Op == "not in"
		parts := []domain.Filter{}
		if unresolved {
			parts = append(parts, leaf(domain.FieldStateCategory, domain.OpNotIn, anyList(closedCategories)))
		}
		if len(cats) > 0 {
			parts = append(parts, leaf(domain.FieldStateCategory, domain.OpIn, anyList(cats)))
		}
		f := parts[0]
		if len(parts) > 1 {
			f = domain.Filter{Or: parts}
		}
		if negate {
			return domain.Filter{Not: &f}, nil
		}
		return f, nil

	case "assignee", "reporter", "creator":
		field := domain.FieldAssignee
		if c.Field != "assignee" {
			field = domain.FieldReporter
		}
		if emptyTest {
			return leaf(field, emptyOp, nil), nil
		}
		return t.equality(c, field, func(v Value) ([]string, error) {
			if v.Function {
				if v.Text == "currentuser" {
					return []string{"me"}, nil
				}
				return nil, errorf("Unable to find JQL function '%s()' for the '%s' field.", v.Text, c.Field)
			}
			return []string{v.Text}, nil
		})

	case "priority":
		if emptyTest {
			if isEmpty {
				return leaf(domain.FieldPriority, domain.OpEq, "0"), nil
			}
			return leaf(domain.FieldPriority, domain.OpNeq, "0"), nil
		}
		return t.equality(c, domain.FieldPriority, func(v Value) ([]string, error) {
			p, ok := Priorities[strings.ToLower(v.Text)]
			if !ok || v.Function {
				return nil, errorf("The value '%s' does not exist for the field 'priority'.", v.Text)
			}
			return []string{strconv.Itoa(p)}, nil
		})

	case "labels", "label":
		if emptyTest {
			return leaf(domain.FieldLabel, emptyOp, nil), nil
		}
		return t.equality(c, domain.FieldLabel, literal(c))

	case "parent", "epic link", "parent link", "parentepic":
		if emptyTest {
			return leaf(domain.FieldParent, emptyOp, nil), nil
		}
		if c.Field == "parentepic" {
			if c.Op != "=" && c.Op != "in" {
				return domain.Filter{}, opError(c)
			}
			vals, err := literal(c)(c.Values[0])
			if err != nil {
				return domain.Filter{}, err
			}
			return leaf(domain.FieldAncestor, domain.OpEq, vals[0]), nil
		}
		return t.equality(c, domain.FieldParent, literal(c))

	case "text", "summary", "description", "comment":
		if c.Op != "~" && c.Op != "!~" {
			return domain.Filter{}, opError(c)
		}
		v := c.Values[0]
		if v.Function {
			return domain.Filter{}, errorf("Unable to find JQL function '%s()' for the '%s' field.", v.Text, c.Field)
		}
		// Jira wildcards: trailing * and ? match prefixes; contains already does.
		term := strings.Trim(v.Text, "*?")
		field := domain.FieldText
		if c.Field == "summary" {
			field = domain.FieldTitle
		}
		f := leaf(field, domain.OpContains, term)
		if c.Op == "!~" {
			return domain.Filter{Not: &f}, nil
		}
		return f, nil

	case "created", "createddate", "updated", "updateddate", "duedate", "due":
		field := map[string]string{
			"created": domain.FieldCreated, "createddate": domain.FieldCreated,
			"updated": domain.FieldUpdated, "updateddate": domain.FieldUpdated,
			"duedate": domain.FieldDueDate, "due": domain.FieldDueDate,
		}[c.Field]
		return t.date(c, field)
	}
	return domain.Filter{}, errorf("Field '%s' does not exist or you do not have permission to view it.", c.Field)
}

const tsFormat = "2006-01-02T15:04:05.000Z"

func (t translator) date(c Clause, field string) (domain.Filter, error) {
	dateOnly := field == domain.FieldDueDate
	if c.Op == "is" || c.Op == "is not" {
		if !dateOnly {
			return domain.Filter{}, opError(c)
		}
		op := domain.OpNotEmpty
		if c.Op == "is" {
			op = domain.OpEmpty
		}
		return leaf(field, op, nil), nil
	}
	if len(c.Values) != 1 {
		return domain.Filter{}, opError(c)
	}
	at, wholeDay, err := t.dateValue(c.Values[0], c.Field)
	if err != nil {
		return domain.Filter{}, err
	}
	format := func(tm time.Time) string {
		if dateOnly {
			return tm.In(t.opt.Location).Format("2006-01-02")
		}
		return tm.UTC().Format(tsFormat)
	}
	end := at.Add(time.Minute)
	if wholeDay {
		end = at.AddDate(0, 0, 1)
	}
	switch c.Op {
	case ">=":
		return leaf(field, domain.OpGte, format(at)), nil
	case ">":
		if dateOnly || wholeDay {
			return leaf(field, domain.OpGte, format(end)), nil
		}
		return leaf(field, domain.OpGt, format(at)), nil
	case "<":
		return leaf(field, domain.OpLt, format(at)), nil
	case "<=":
		if dateOnly || wholeDay {
			return leaf(field, domain.OpLt, format(end)), nil
		}
		return leaf(field, domain.OpLte, format(at)), nil
	case "=":
		if dateOnly {
			return leaf(field, domain.OpEq, format(at)), nil
		}
		return domain.Filter{And: []domain.Filter{leaf(field, domain.OpGte, format(at)), leaf(field, domain.OpLt, format(end))}}, nil
	case "!=":
		if dateOnly {
			f := leaf(field, domain.OpEq, format(at))
			return domain.Filter{Not: &f}, nil
		}
		return domain.Filter{Or: []domain.Filter{leaf(field, domain.OpLt, format(at)), leaf(field, domain.OpGte, format(end))}}, nil
	}
	return domain.Filter{}, opError(c)
}

// dateValue resolves a date literal, relative date or date function. wholeDay
// reports whether the value names a calendar day rather than an instant.
func (t translator) dateValue(v Value, field string) (time.Time, bool, error) {
	loc, now := t.opt.Location, t.opt.Now.In(t.opt.Location)
	if v.Function {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		var base time.Time
		switch v.Text {
		case "now":
			return now, false, nil
		case "startofday":
			base = day
		case "endofday":
			base = day.AddDate(0, 0, 1).Add(-time.Millisecond)
		case "startofweek":
			base = day.AddDate(0, 0, -int(day.Weekday()))
		case "endofweek":
			base = day.AddDate(0, 0, 7-int(day.Weekday())).Add(-time.Millisecond)
		case "startofmonth":
			base = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		case "endofmonth":
			base = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, loc).Add(-time.Millisecond)
		case "startofyear":
			base = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)
		case "endofyear":
			base = time.Date(now.Year()+1, 1, 1, 0, 0, 0, 0, loc).Add(-time.Millisecond)
		default:
			return time.Time{}, false, errorf("Unable to find JQL function '%s()' for the '%s' field.", v.Text, field)
		}
		if len(v.Args) > 0 {
			// startOfDay(-1) moves by the function's own unit; "-1w" names one.
			arg := v.Args[0]
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "+")); err == nil {
				switch {
				case strings.HasSuffix(v.Text, "day"):
					base = base.AddDate(0, 0, n)
				case strings.HasSuffix(v.Text, "week"):
					base = base.AddDate(0, 0, 7*n)
				case strings.HasSuffix(v.Text, "month"):
					base = base.AddDate(0, n, 0)
				default:
					base = base.AddDate(n, 0, 0)
				}
			} else if moved, ok := relative(arg, base); ok {
				base = moved
			} else {
				return time.Time{}, false, errorf("The argument '%s' of the function '%s()' is not valid.", arg, v.Text)
			}
		}
		return base, false, nil
	}
	if at, ok := relative(v.Text, now); ok {
		return at, false, nil
	}
	for _, layout := range []string{"2006-01-02", "2006/01/02"} {
		if at, err := time.ParseInLocation(layout, v.Text, loc); err == nil {
			return at, true, nil
		}
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006/01/02 15:04", time.RFC3339} {
		if at, err := time.ParseInLocation(layout, v.Text, loc); err == nil {
			return at, false, nil
		}
	}
	return time.Time{}, false, errorf("Date value '%s' for field '%s' is invalid. Valid formats include: 'yyyy/MM/dd HH:mm', 'yyyy-MM-dd HH:mm', 'yyyy/MM/dd', 'yyyy-MM-dd', or a period format e.g. '-5d', '4w 2d'.", v.Text, field)
}

var sortKeys = map[string][2]string{ // field -> {asc, desc}
	"updated": {"updated_asc", "updated"}, "updateddate": {"updated_asc", "updated"},
	"created": {"created_asc", "created"}, "createddate": {"created_asc", "created"},
	"key": {"key", "key_desc"}, "issuekey": {"key", "key_desc"}, "id": {"key", "key_desc"},
	"priority": {"priority", "priority"},
	"duedate":  {"due_date", "due_date"}, "due": {"due_date", "due_date"},
	"rank": {"manual", "manual"},
}

// sortKey uses the first ORDER BY term; later terms only break ties in Jira
// and are ignored here.
func sortKey(order []OrderBy) (string, error) {
	if len(order) == 0 {
		return "", nil
	}
	ob := order[0]
	keys, ok := sortKeys[ob.Field]
	if !ok {
		if _, known := map[string]bool{"status": true, "assignee": true, "reporter": true, "summary": true, "issuetype": true, "project": true}[ob.Field]; known {
			return "", nil // accepted but not supported: default order
		}
		return "", errorf("Not able to sort using field '%s'.", ob.Field)
	}
	if ob.Desc {
		return keys[1], nil
	}
	return keys[0], nil
}
