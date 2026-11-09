package domain

import (
	"encoding/json"
	"fmt"
)

// Filter is the common intermediate representation for issue searches. UI
// filters, saved views, MCP search and (later) JQL are all converted into a
// Filter before SQL is built from it.
//
// JSON form (exactly one of the groups must be set):
//
//	{"and": [Filter, ...]}
//	{"or":  [Filter, ...]}
//	{"not": Filter}
//	{"field": "type", "op": "in", "value": ["story", "task"]}
type Filter struct {
	And   []Filter `json:"and,omitempty"`
	Or    []Filter `json:"or,omitempty"`
	Not   *Filter  `json:"not,omitempty"`
	Field string   `json:"field,omitempty"`
	Op    string   `json:"op,omitempty"`
	Value any      `json:"value,omitempty"`
}

// Filter fields.
const (
	FieldProject       = "project"        // project key
	FieldProjectTier   = "project_tier"   // focus|normal|side
	FieldKey           = "key"            // issue key
	FieldType          = "type"           // issue type
	FieldState         = "state"          // workflow state id
	FieldStateName     = "state_name"     // workflow state name (case-insensitive)
	FieldStateCategory = "state_category" // backlog|unstarted|started|completed|canceled
	FieldAssignee      = "assignee"       // username, email, user id, or "me"
	FieldAssigneeKind  = "assignee_kind"  // human|agent
	FieldReporter      = "reporter"       // username, email, user id, or "me"
	FieldPriority      = "priority"       // 0-4
	FieldLabel         = "label"          // label name
	FieldParent        = "parent"         // direct parent issue key
	FieldAncestor      = "ancestor"       // any ancestor issue key (descendants of)
	FieldText          = "text"           // title/description contains
	FieldTitle         = "title"          // title contains
	FieldDueDate       = "due_date"       // YYYY-MM-DD
	FieldCreated       = "created"        // RFC3339 or YYYY-MM-DD
	FieldUpdated       = "updated"        // RFC3339 or YYYY-MM-DD
)

// Filter operators.
const (
	OpEq       = "eq"
	OpNeq      = "neq"
	OpIn       = "in"
	OpNotIn    = "not_in"
	OpEmpty    = "empty"
	OpNotEmpty = "not_empty"
	OpContains = "contains"
	OpLt       = "lt"
	OpLte      = "lte"
	OpGt       = "gt"
	OpGte      = "gte"
)

var fieldOps = map[string][]string{
	FieldProject:       {OpEq, OpNeq, OpIn, OpNotIn},
	FieldProjectTier:   {OpEq, OpNeq, OpIn, OpNotIn},
	FieldKey:           {OpEq, OpNeq, OpIn, OpNotIn},
	FieldType:          {OpEq, OpNeq, OpIn, OpNotIn},
	FieldState:         {OpEq, OpNeq, OpIn, OpNotIn},
	FieldStateName:     {OpEq, OpNeq, OpIn, OpNotIn},
	FieldStateCategory: {OpEq, OpNeq, OpIn, OpNotIn},
	FieldAssignee:      {OpEq, OpNeq, OpIn, OpNotIn, OpEmpty, OpNotEmpty},
	FieldAssigneeKind:  {OpEq, OpNeq},
	FieldReporter:      {OpEq, OpNeq, OpIn, OpNotIn, OpEmpty, OpNotEmpty},
	FieldPriority:      {OpEq, OpNeq, OpIn, OpNotIn, OpLt, OpLte, OpGt, OpGte},
	FieldLabel:         {OpEq, OpNeq, OpIn, OpNotIn, OpEmpty, OpNotEmpty},
	FieldParent:        {OpEq, OpNeq, OpIn, OpNotIn, OpEmpty, OpNotEmpty},
	FieldAncestor:      {OpEq},
	FieldText:          {OpContains},
	FieldTitle:         {OpContains},
	FieldDueDate:       {OpEq, OpLt, OpLte, OpGt, OpGte, OpEmpty, OpNotEmpty},
	FieldCreated:       {OpLt, OpLte, OpGt, OpGte},
	FieldUpdated:       {OpLt, OpLte, OpGt, OpGte},
}

// ParseFilter decodes and validates a JSON filter. An empty string means
// "match everything".
func ParseFilter(s string) (Filter, error) {
	var f Filter
	if s == "" {
		return f, nil
	}
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return f, fmt.Errorf("filter is not valid JSON: %w", err)
	}
	return f, f.Validate()
}

// IsEmpty reports whether the filter matches everything.
func (f Filter) IsEmpty() bool {
	return len(f.And) == 0 && len(f.Or) == 0 && f.Not == nil && f.Field == ""
}

// Validate checks structure, fields, operators and value shapes.
func (f Filter) Validate() error {
	groups := 0
	if f.And != nil {
		groups++
	}
	if f.Or != nil {
		groups++
	}
	if f.Not != nil {
		groups++
	}
	if f.Field != "" {
		groups++
	}
	if groups > 1 {
		return fmt.Errorf("a filter node must have exactly one of and/or/not/field")
	}
	for _, c := range append(append([]Filter{}, f.And...), f.Or...) {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	if f.Not != nil {
		return f.Not.Validate()
	}
	if f.Field == "" {
		return nil
	}
	ops, ok := fieldOps[f.Field]
	if !ok {
		return fmt.Errorf("unknown filter field %q", f.Field)
	}
	if !contains(ops, f.Op) {
		return fmt.Errorf("operator %q is not supported for field %q", f.Op, f.Field)
	}
	switch f.Op {
	case OpEmpty, OpNotEmpty:
		return nil
	case OpIn, OpNotIn:
		if _, err := f.Values(); err != nil {
			return err
		}
	default:
		if _, err := f.Scalar(); err != nil {
			return err
		}
	}
	return nil
}

// Values returns the value as a list of strings (for in/not_in).
func (f Filter) Values() ([]string, error) {
	list, ok := f.Value.([]any)
	if !ok {
		return nil, fmt.Errorf("field %q with %q needs a list value", f.Field, f.Op)
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		s, err := scalarString(v)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", f.Field, err)
		}
		out = append(out, s)
	}
	return out, nil
}

// Scalar returns the value as a string.
func (f Filter) Scalar() (string, error) {
	s, err := scalarString(f.Value)
	if err != nil {
		return "", fmt.Errorf("field %q: %w", f.Field, err)
	}
	return s, nil
}

func scalarString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case float64:
		return fmt.Sprintf("%g", x), nil
	case bool:
		return fmt.Sprintf("%t", x), nil
	case nil:
		return "", fmt.Errorf("value is required")
	}
	return "", fmt.Errorf("value must be a string or number")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
