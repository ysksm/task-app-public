package jql

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 12, 15, 30, 0, 0, time.UTC)

func compile(t *testing.T, q string) Result {
	t.Helper()
	res, err := Compile(q, Options{Now: now, Location: time.UTC, ProjectKey: func(s string) string {
		if s == "01PROJECTID" {
			return "APP"
		}
		return s
	}})
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	if err := res.Filter.Validate(); err != nil {
		t.Fatalf("%s: invalid filter: %v", q, err)
	}
	return res
}

func filterJSON(t *testing.T, r Result) string {
	b, _ := json.Marshal(r.Filter)
	return string(b)
}

func TestOrcaQueries(t *testing.T) {
	cases := []struct{ jql, filter, sort string }{
		{
			"assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC",
			`{"and":[{"field":"assignee","op":"eq","value":"me"},{"field":"state_category","op":"not_in","value":["completed","canceled"]}]}`,
			"updated",
		},
		{
			"reporter = currentUser() AND resolution = Unresolved ORDER BY updated DESC",
			`{"and":[{"field":"reporter","op":"eq","value":"me"},{"field":"state_category","op":"not_in","value":["completed","canceled"]}]}`,
			"updated",
		},
		{
			"assignee = currentUser() AND resolution IS NOT EMPTY ORDER BY updated DESC",
			`{"and":[{"field":"assignee","op":"eq","value":"me"},{"field":"state_category","op":"in","value":["completed","canceled"]}]}`,
			"updated",
		},
		{
			"resolution = Unresolved ORDER BY updated DESC",
			`{"field":"state_category","op":"not_in","value":["completed","canceled"]}`,
			"updated",
		},
		{
			"project = ABC AND statusCategory != Done",
			`{"and":[{"field":"project","op":"eq","value":"ABC"},{"field":"state_category","op":"not_in","value":["completed","canceled"]}]}`,
			"",
		},
		{`key = "APP-1"`, `{"field":"key","op":"eq","value":"APP-1"}`, ""},
		{`text ~ "foo*"`, `{"field":"text","op":"contains","value":"foo"}`, ""},
	}
	for _, c := range cases {
		r := compile(t, c.jql)
		if got := filterJSON(t, r); got != c.filter {
			t.Errorf("%s\n got %s\nwant %s", c.jql, got, c.filter)
		}
		if r.Sort != c.sort {
			t.Errorf("%s: sort %q, want %q", c.jql, r.Sort, c.sort)
		}
	}
}

func TestFieldsAndOperators(t *testing.T) {
	cases := map[string]string{
		`issuetype in (Epic, "Story", 10003)`:                `{"field":"type","op":"in","value":["epic","story","task"]}`,
		`type = Question`:                                    `{"field":"type","op":"eq","value":"qa"}`,
		`status = "In Progress"`:                             `{"field":"state_name","op":"eq","value":"In Progress"}`,
		`status in (5, Todo)`:                                `{"or":[{"field":"state","op":"eq","value":"5"},{"field":"state_name","op":"eq","value":"Todo"}]}`,
		`status not in ("Done", Canceled)`:                   `{"field":"state_name","op":"not_in","value":["Done","Canceled"]}`,
		`statusCategory = "In Progress"`:                     `{"field":"state_category","op":"eq","value":"started"}`,
		`statusCategory in (new, done)`:                      `{"field":"state_category","op":"in","value":["backlog","unstarted","completed","canceled"]}`,
		`assignee is EMPTY`:                                  `{"field":"assignee","op":"empty"}`,
		`assignee in (currentUser(), "alice@taskapp.local")`: `{"field":"assignee","op":"in","value":["me","alice@taskapp.local"]}`,
		`assignee != alice`:                                  `{"field":"assignee","op":"neq","value":"alice"}`,
		`priority = High`:                                    `{"field":"priority","op":"eq","value":"2"}`,
		`priority in (Highest, 3)`:                           `{"field":"priority","op":"in","value":["1","3"]}`,
		`priority is EMPTY`:                                  `{"field":"priority","op":"eq","value":"0"}`,
		`labels = backend`:                                   `{"field":"label","op":"eq","value":"backend"}`,
		`labels is not EMPTY`:                                `{"field":"label","op":"not_empty"}`,
		`parent = APP-3`:                                     `{"field":"parent","op":"eq","value":"APP-3"}`,
		`parentEpic = APP-1`:                                 `{"field":"ancestor","op":"eq","value":"APP-1"}`,
		`summary ~ login`:                                    `{"field":"title","op":"contains","value":"login"}`,
		`text !~ "draft"`:                                    `{"not":{"field":"text","op":"contains","value":"draft"}}`,
		`project = 01PROJECTID`:                              `{"field":"project","op":"eq","value":"APP"}`,
		`resolution = "Won't Do"`:                            `{"field":"state_category","op":"in","value":["canceled"]}`,
		`resolution = Done`:                                  `{"field":"state_category","op":"in","value":["completed"]}`,
		`resolution is EMPTY`:                                `{"field":"state_category","op":"not_in","value":["completed","canceled"]}`,
		`resolution != Unresolved`:                           `{"not":{"field":"state_category","op":"not_in","value":["completed","canceled"]}}`,
		`NOT (type = epic OR type = story) AND key != APP-1`: `{"and":[{"not":{"or":[{"field":"type","op":"eq","value":"epic"},{"field":"type","op":"eq","value":"story"}]}},{"field":"key","op":"neq","value":"APP-1"}]}`,
		`assignee = EMPTY`:                                   `{"field":"assignee","op":"empty"}`,
		`project = APP and type = task or type = epic`:       `{"or":[{"and":[{"field":"project","op":"eq","value":"APP"},{"field":"type","op":"eq","value":"task"}]},{"field":"type","op":"eq","value":"epic"}]}`,
	}
	for q, want := range cases {
		if got := filterJSON(t, compile(t, q)); got != want {
			t.Errorf("%s\n got %s\nwant %s", q, got, want)
		}
	}
}

func TestDates(t *testing.T) {
	cases := map[string]string{
		`created >= -7d`:            `{"field":"created","op":"gte","value":"2026-10-05T15:30:00.000Z"}`,
		`updated > "2026-10-01"`:    `{"field":"updated","op":"gte","value":"2026-10-02T00:00:00.000Z"}`,
		`updated < 2026/10/01`:      `{"field":"updated","op":"lt","value":"2026-10-01T00:00:00.000Z"}`,
		`created = "2026-10-01"`:    `{"and":[{"field":"created","op":"gte","value":"2026-10-01T00:00:00.000Z"},{"field":"created","op":"lt","value":"2026-10-02T00:00:00.000Z"}]}`,
		`updated >= startOfDay()`:   `{"field":"updated","op":"gte","value":"2026-10-12T00:00:00.000Z"}`,
		`updated >= startOfDay(-1)`: `{"field":"updated","op":"gte","value":"2026-10-11T00:00:00.000Z"}`,
		`created >= startOfWeek()`:  `{"field":"created","op":"gte","value":"2026-10-11T00:00:00.000Z"}`,
		`created >= startOfMonth()`: `{"field":"created","op":"gte","value":"2026-10-01T00:00:00.000Z"}`,
		`duedate <= "2026-10-20"`:   `{"field":"due_date","op":"lt","value":"2026-10-21"}`,
		`duedate is EMPTY`:          `{"field":"due_date","op":"empty"}`,
		`due = 2026-10-20`:          `{"field":"due_date","op":"eq","value":"2026-10-20"}`,
	}
	for q, want := range cases {
		if got := filterJSON(t, compile(t, q)); got != want {
			t.Errorf("%s\n got %s\nwant %s", q, got, want)
		}
	}
}

func TestOrderBy(t *testing.T) {
	cases := map[string]string{
		"ORDER BY created":           "created_asc",
		"order by created DESC":      "created",
		"project = APP ORDER BY key": "key",
		"ORDER BY key desc, updated": "key_desc",
		"ORDER BY priority DESC":     "priority",
		"ORDER BY rank":              "manual",
		"ORDER BY status":            "",
		"ORDER BY updated ASC":       "updated_asc",
		"":                           "",
	}
	for q, want := range cases {
		if got := compile(t, q).Sort; got != want {
			t.Errorf("%q: sort %q, want %q", q, got, want)
		}
	}
	if !compile(t, "").Filter.IsEmpty() {
		t.Error("empty query should match everything")
	}
}

func TestErrors(t *testing.T) {
	cases := map[string]string{
		`assignee = `:              "reached the end of the query",
		`project = APP AND`:        "reached the end of the query",
		`summary = "x`:             "not terminated",
		`sprint = 5`:               "Field 'sprint' does not exist",
		`issuetype = Bug`:          "The value 'Bug' does not exist for the field 'issuetype'",
		`assignee = membersOf(x)`:  "Unable to find JQL function 'membersof()'",
		`project = APP ORDER BY x`: "Not able to sort using field 'x'",
		`project = APP project`:    "Expecting either 'OR' or 'AND'",
		`summary = x`:              "operator '=' is not supported by the 'summary' field",
		`created = garbage`:        "Date value 'garbage'",
		`(project = APP`:           "Expecting ')'",
	}
	for q, want := range cases {
		_, err := Compile(q, Options{Now: now})
		if err == nil {
			t.Errorf("%s: expected an error", q)
			continue
		}
		if !strings.HasPrefix(err.Error(), ErrorPrefix) || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %q, want it to contain %q", q, err.Error(), want)
		}
	}
}
