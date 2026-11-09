package service

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ysksm/task-app/internal/domain"
)

// seedPerf creates about 10,000 issues for the performance check (NFR-03):
// 4 projects × (10 epics × 5 stories × 5 features × 9 tasks) = 10,240 issues.
// Values are derived from the index, so the data is the same on every run.
func (s *Service) seedPerf(ctx context.Context, users map[string]User) error {
	owner := WithActor(ctx, Actor{User: users["owner"], Source: domain.SourceSystem})
	actor, _ := ActorFrom(owner)
	assignees := []string{"owner", "alice", "claude", "codex", ""}
	labelSets := [][]string{nil, {"frontend"}, {"backend"}, {"frontend", "urgent"}, nil}
	for p := 1; p <= 4; p++ {
		key := fmt.Sprintf("P%d", p)
		if _, err := s.CreateProject(owner, CreateProjectInput{Key: key, Name: fmt.Sprintf("性能確認 %d", p)}); err != nil {
			return err
		}
		states, err := s.ListStates(owner, key)
		if err != nil {
			return err
		}
		var regular []State
		for _, st := range states {
			if st.AppliesTo == "all" {
				regular = append(regular, st)
			}
		}
		n := 0
		mk := func(tx *sql.Tx, typ domain.IssueType, title string, parent *string) (string, error) {
			n++
			st := regular[n%len(regular)].ID
			pr := n % 5
			in := CreateIssueInput{ProjectKey: key, Type: string(typ), Title: title, ParentKey: parent, StateID: &st, Priority: &pr,
				Labels: labelSets[n%len(labelSets)], Description: fmt.Sprintf("性能確認用のイシュー %d。\n\n- 項目 A\n- 項目 B", n)}
			if a := assignees[n%len(assignees)]; a != "" {
				in.Assignee = &a
			}
			c, err := s.createIssueTx(owner, tx, actor, in)
			return c.key, err
		}
		// One transaction per epic keeps memory flat and the seed fast.
		for e := 1; e <= 10; e++ {
			err := s.withTx(owner, func(tx *sql.Tx) error {
				epic, err := mk(tx, domain.TypeEpic, fmt.Sprintf("エピック %d-%d", p, e), nil)
				if err != nil {
					return err
				}
				for st := 1; st <= 5; st++ {
					story, err := mk(tx, domain.TypeStory, fmt.Sprintf("ストーリー %d-%d-%d", p, e, st), &epic)
					if err != nil {
						return err
					}
					for f := 1; f <= 5; f++ {
						feature, err := mk(tx, domain.TypeFeature, fmt.Sprintf("フィーチャー %d-%d-%d-%d", p, e, st, f), &story)
						if err != nil {
							return err
						}
						for t := 1; t <= 9; t++ {
							if _, err := mk(tx, domain.TypeTask, fmt.Sprintf("タスク %d-%d-%d-%d-%d ログイン", p, e, st, f, t), &feature); err != nil {
								return err
							}
						}
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
