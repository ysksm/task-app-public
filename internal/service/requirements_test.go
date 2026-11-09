package service

import "testing"

func TestParseReqKeys(t *testing.T) {
	for _, c := range []struct {
		raw    string
		prefix string
		n      int
		ok     bool
	}{
		{"TAPP-ST12", "ST", 12, true},
		{"st12", "ST", 12, true},
		{" tapp-ac3 ", "AC", 3, true},
		{"TAPP-ST0", "ST", 0, false},
		{"TAPP-STx", "ST", 0, false},
		{"OPS-ST1", "ST", 0, false},  // another project's key
		{"TAPP-A1-2", "A", 0, false}, // a step, not an activity
		{"TAPP-P1", "ST", 0, false},
	} {
		n, ok := ParseReqNum("TAPP", c.prefix, c.raw)
		if n != c.n || ok != c.ok {
			t.Errorf("ParseReqNum(%q, %q) = %d, %v; want %d, %v", c.raw, c.prefix, n, ok, c.n, c.ok)
		}
	}
	for _, c := range []struct {
		raw  string
		a, n int
		ok   bool
	}{
		{"TAPP-A1-2", 1, 2, true},
		{"a10-3", 10, 3, true},
		{"TAPP-A1", 0, 0, false},
		{"TAPP-A0-1", 0, 1, false},
		{"TAPP-ST1", 0, 0, false},
	} {
		a, n, ok := ParseStepKey("TAPP", c.raw)
		if ok != c.ok || (ok && (a != c.a || n != c.n)) {
			t.Errorf("ParseStepKey(%q) = %d, %d, %v; want %d, %d, %v", c.raw, a, n, ok, c.a, c.n, c.ok)
		}
	}
}

func TestStoryStatus(t *testing.T) {
	is := func(cats ...string) []StoryIssue {
		out := []StoryIssue{}
		for _, c := range cats {
			out = append(out, StoryIssue{StateCategory: c})
		}
		return out
	}
	for _, c := range []struct {
		criteria int
		issues   []StoryIssue
		want     string
	}{
		{0, is("completed"), "draft"},
		{1, is(), "ready"},
		{1, is("unstarted", "backlog"), "ready"},
		{1, is("started"), "in_progress"},
		{1, is("completed", "unstarted"), "in_progress"},
		{1, is("completed", "canceled"), "done"},
		{1, is("canceled"), "ready"},
	} {
		if got := storyStatus(Story{CriteriaCount: c.criteria, Issues: c.issues}); got != c.want {
			t.Errorf("storyStatus(%d criteria, %v) = %s; want %s", c.criteria, c.issues, got, c.want)
		}
	}
}
