package domain

import "fmt"

// HierarchyError explains why a parent/child pair is not allowed.
type HierarchyError struct {
	Parent IssueType // empty when the child would be a root
	Child  IssueType
	Reason string
}

func (e *HierarchyError) Error() string {
	parent := string(e.Parent)
	if parent == "" {
		parent = "(root)"
	}
	return fmt.Sprintf("%s cannot be placed under %s: %s", e.Child, parent, e.Reason)
}

// CanBeChild checks the parent/child rules from docs/01-requirements.md §4.2.
// parent is nil when the child is a root issue. allowSkip permits skipping
// levels (e.g. a task directly under an epic).
func CanBeChild(parent *IssueType, child IssueType, allowSkip bool) error {
	if parent == nil {
		return nil // every type may be a root
	}
	p := *parent
	deny := func(reason string) error {
		return &HierarchyError{Parent: p, Child: child, Reason: reason}
	}
	if p == TypeQA {
		return deny("QA issues cannot have children")
	}
	switch child {
	case TypeEpic:
		return deny("epics must be root issues")
	case TypeQA:
		return nil // a question can be attached to any non-QA issue
	case TypeStory:
		if p == TypeEpic {
			return nil
		}
		return deny("stories can only be placed under epics")
	case TypeFeature:
		if p == TypeStory || (p == TypeEpic && allowSkip) {
			return nil
		}
		return deny(skipReason("features go under stories", allowSkip, p == TypeEpic))
	case TypeTask:
		if p == TypeFeature || p == TypeTask || (allowSkip && (p == TypeStory || p == TypeEpic)) {
			return nil
		}
		return deny(skipReason("tasks go under features or tasks", allowSkip, p == TypeStory || p == TypeEpic))
	}
	return deny("unknown issue type")
}

func skipReason(base string, allowSkip, wouldSkip bool) string {
	if wouldSkip && !allowSkip {
		return base + " (skipping levels is disabled for this project)"
	}
	return base
}
