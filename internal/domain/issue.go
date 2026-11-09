// Package domain holds the core concepts and rules. It has no dependencies on
// storage or transport.
package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// IssueType is the kind of an issue.
type IssueType string

const (
	TypeEpic    IssueType = "epic"
	TypeStory   IssueType = "story"
	TypeFeature IssueType = "feature"
	TypeTask    IssueType = "task"
	TypeQA      IssueType = "qa"
)

// IssueTypes lists all types in hierarchy order.
var IssueTypes = []IssueType{TypeEpic, TypeStory, TypeFeature, TypeTask, TypeQA}

// Valid reports whether t is a known type.
func (t IssueType) Valid() bool {
	for _, v := range IssueTypes {
		if v == t {
			return true
		}
	}
	return false
}

// StateCategory groups workflow states (Linear style).
type StateCategory string

const (
	CategoryBacklog   StateCategory = "backlog"
	CategoryUnstarted StateCategory = "unstarted"
	CategoryStarted   StateCategory = "started"
	CategoryCompleted StateCategory = "completed"
	CategoryCanceled  StateCategory = "canceled"
)

// Closed reports whether issues in this category count as finished.
func (c StateCategory) Closed() bool {
	return c == CategoryCompleted || c == CategoryCanceled
}

// Priority: 0 none, 1 urgent, 2 high, 3 medium, 4 low (Linear numbering).
const (
	PriorityNone   = 0
	PriorityUrgent = 1
	PriorityHigh   = 2
	PriorityMedium = 3
	PriorityLow    = 4
)

// Source is the channel a change came through.
type Source string

const (
	SourceUI     Source = "ui"
	SourceAPI    Source = "api"
	SourceJira   Source = "jira"
	SourceMCP    Source = "mcp"
	SourceSystem Source = "system"
)

var projectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// ValidateProjectKey checks a project key such as "APP".
func ValidateProjectKey(key string) error {
	if !projectKeyRe.MatchString(key) {
		return fmt.Errorf("project key must be 2-10 uppercase letters or digits starting with a letter: %q", key)
	}
	return nil
}

// IssueKey builds "APP-12".
func IssueKey(projectKey string, number int) string {
	return fmt.Sprintf("%s-%d", projectKey, number)
}

// SplitIssueKey parses "APP-12" into its project key and number.
func SplitIssueKey(key string) (string, int, bool) {
	i := strings.LastIndexByte(key, '-')
	if i <= 0 {
		return "", 0, false
	}
	var n int
	if _, err := fmt.Sscanf(key[i+1:], "%d", &n); err != nil || n <= 0 {
		return "", 0, false
	}
	return key[:i], n, true
}

// Project tiers: how much attention a project gets.
const (
	TierFocus  = "focus"  // 注力
	TierNormal = "normal" // 通常
	TierSide   = "side"   // その他: low priority, things started casually
)

// ValidTier reports whether t is a project tier.
func ValidTier(t string) bool {
	return t == TierFocus || t == TierNormal || t == TierSide
}

// Requirements modes: how a project writes its requirements (docs/09 §6).
const (
	ReqModeStructured = "structured" // 整理して書く: option A
	ReqModeQuick      = "quick"      // まず書く: option D
)

// ValidRequirementsMode reports whether m is a requirements mode.
func ValidRequirementsMode(m string) bool {
	return m == ReqModeStructured || m == ReqModeQuick
}
