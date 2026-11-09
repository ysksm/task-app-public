package service

import (
	"context"
	"strings"

	"github.com/ysksm/task-app/internal/domain"
)

// TreeNode is an issue with its descendants.
type TreeNode struct {
	Key         string      `json:"key"`
	Type        string      `json:"type"`
	Title       string      `json:"title"`
	State       string      `json:"state"`
	Category    string      `json:"state_category"`
	Assignee    string      `json:"assignee,omitempty"`
	Progress    Progress    `json:"progress"`
	OpenQACount int         `json:"open_qa_count"`
	Children    []*TreeNode `json:"children"`
}

// IssueTree returns the issue and its descendants up to maxDepth levels (0 = unlimited).
func (s *Service) IssueTree(ctx context.Context, key string, maxDepth int) (*TreeNode, error) {
	root, err := s.GetIssue(ctx, key)
	if err != nil {
		return nil, err
	}
	res, err := s.ListIssues(ctx, ListOptions{Filter: domain.Filter{Field: domain.FieldAncestor, Op: domain.OpEq, Value: root.Key},
		Sort: "manual", Limit: 5000})
	if err != nil {
		return nil, err
	}
	nodes := map[string]*TreeNode{root.Key: toNode(root)}
	for _, is := range res.Issues {
		nodes[is.Key] = toNode(is)
	}
	// Issues come in manual (sibling) order, so children keep that order.
	for _, is := range res.Issues {
		if p, ok := nodes[deref(is.ParentKey)]; ok {
			p.Children = append(p.Children, nodes[is.Key])
		}
	}
	var prune func(n *TreeNode, depth int)
	prune = func(n *TreeNode, depth int) {
		if maxDepth > 0 && depth >= maxDepth {
			n.Children = []*TreeNode{}
			return
		}
		for _, c := range n.Children {
			prune(c, depth+1)
		}
	}
	prune(nodes[root.Key], 0)
	return nodes[root.Key], nil
}

func toNode(is Issue) *TreeNode {
	n := &TreeNode{Key: is.Key, Type: is.Type, Title: is.Title, State: is.State.Name, Category: is.State.Category,
		Progress: is.Progress, OpenQACount: is.OpenQACount, Children: []*TreeNode{}}
	if is.Assignee != nil {
		n.Assignee = is.Assignee.Username
	}
	return n
}

// TransitionIssue moves an issue to the state with the given name (case-insensitive).
func (s *Service) TransitionIssue(ctx context.Context, key, stateName string) (Issue, error) {
	is, err := s.GetIssue(ctx, key)
	if err != nil {
		return Issue{}, err
	}
	states, err := s.ListStates(ctx, is.ProjectKey)
	if err != nil {
		return Issue{}, err
	}
	applies := "all"
	if is.Type == string(domain.TypeQA) {
		applies = "qa"
	}
	names := []string{}
	for _, st := range states {
		if st.AppliesTo != applies {
			continue
		}
		names = append(names, st.Name)
		if strings.EqualFold(st.Name, strings.TrimSpace(stateName)) {
			return s.UpdateIssue(ctx, key, UpdateIssueInput{StateID: Some(st.ID)})
		}
	}
	return Issue{}, Errorf(CodeInvalidArgument, "unknown state %q; valid states: %s", stateName, strings.Join(names, ", "))
}

// RequestAcceptance moves a story or feature to the acceptance state (受入待ち).
func (s *Service) RequestAcceptance(ctx context.Context, key string) (Issue, error) {
	is, err := s.GetIssue(ctx, key)
	if err != nil {
		return Issue{}, err
	}
	if is.Type == string(domain.TypeQA) {
		return Issue{}, Errorf(CodeInvalidArgument, "QA issues are not accepted")
	}
	states, err := s.ListStates(ctx, is.ProjectKey)
	if err != nil {
		return Issue{}, err
	}
	for _, st := range states {
		if st.IsAcceptance {
			return s.UpdateIssue(ctx, key, UpdateIssueInput{StateID: Some(st.ID)})
		}
	}
	return Issue{}, Errorf(CodeInvalidArgument, "project %s has no acceptance state", is.ProjectKey)
}
