package domain

import "testing"

func TestCanBeChild(t *testing.T) {
	ptr := func(t IssueType) *IssueType { return &t }
	cases := []struct {
		parent    *IssueType
		child     IssueType
		allowSkip bool
		ok        bool
	}{
		{nil, TypeEpic, true, true},
		{nil, TypeTask, true, true},
		{nil, TypeQA, true, true},
		{ptr(TypeEpic), TypeEpic, true, false},
		{ptr(TypeEpic), TypeStory, false, true},
		{ptr(TypeEpic), TypeFeature, true, true},
		{ptr(TypeEpic), TypeFeature, false, false},
		{ptr(TypeEpic), TypeTask, true, true},
		{ptr(TypeEpic), TypeTask, false, false},
		{ptr(TypeStory), TypeStory, true, false},
		{ptr(TypeStory), TypeFeature, false, true},
		{ptr(TypeStory), TypeTask, true, true},
		{ptr(TypeStory), TypeTask, false, false},
		{ptr(TypeFeature), TypeStory, true, false},
		{ptr(TypeFeature), TypeFeature, true, false},
		{ptr(TypeFeature), TypeTask, false, true},
		{ptr(TypeTask), TypeTask, false, true},
		{ptr(TypeTask), TypeFeature, true, false},
		{ptr(TypeTask), TypeQA, false, true},
		{ptr(TypeEpic), TypeQA, false, true},
		{ptr(TypeQA), TypeTask, true, false},
		{ptr(TypeQA), TypeQA, true, false},
	}
	for _, c := range cases {
		err := CanBeChild(c.parent, c.child, c.allowSkip)
		parent := "(root)"
		if c.parent != nil {
			parent = string(*c.parent)
		}
		if (err == nil) != c.ok {
			t.Errorf("CanBeChild(%s, %s, skip=%v) = %v, want ok=%v", parent, c.child, c.allowSkip, err, c.ok)
		}
	}
}

func TestSplitIssueKey(t *testing.T) {
	k, n, ok := SplitIssueKey("APP-12")
	if !ok || k != "APP" || n != 12 {
		t.Fatalf("got %q %d %v", k, n, ok)
	}
	for _, bad := range []string{"APP", "-1", "APP-0", "APP-x"} {
		if _, _, ok := SplitIssueKey(bad); ok {
			t.Errorf("SplitIssueKey(%q) should fail", bad)
		}
	}
}
