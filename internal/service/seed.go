package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ysksm/task-app/internal/domain"
)

// Seed presets. Tokens are fixed so that tests and local trials are
// reproducible; never use these presets for real data.
//
//	test: users owner (human), alice (human), claude (agent), codex (agent)
//	      with tokens tk_test_<username>. No projects.
//	demo: the same users with tokens tk_demo_<username> plus a sample
//	      project APP covering every issue type and several states.
func (s *Service) Seed(ctx context.Context, preset string) error {
	switch preset {
	case "test", "demo", "perf":
	default:
		return Errorf(CodeInvalidArgument, "unknown seed preset %q (use test, demo or perf)", preset)
	}
	users := []CreateUserInput{
		{Kind: "human", Username: "owner", DisplayName: "Owner", Email: "owner@taskapp.local", IsOwner: true},
		{Kind: "human", Username: "alice", DisplayName: "Alice", Email: "alice@taskapp.local"},
		{Kind: "agent", Username: "claude", DisplayName: "Claude", Email: "claude@taskapp.local", AgentModel: "claude-opus-5-5"},
		{Kind: "agent", Username: "codex", DisplayName: "Codex", Email: "codex@taskapp.local", AgentModel: "codex"},
	}
	byName := map[string]User{}
	for _, in := range users {
		u, err := s.CreateUser(ctx, in)
		if err != nil {
			return fmt.Errorf("seed user %s: %w", in.Username, err)
		}
		byName[u.Username] = u
		if _, _, err := s.CreateToken(ctx, u.ID, preset, fmt.Sprintf("tk_%s_%s", preset, u.Username)); err != nil {
			return err
		}
	}
	switch preset {
	case "demo":
		return s.seedDemo(ctx, byName)
	case "perf":
		return s.seedPerf(ctx, byName)
	}
	return nil
}

func (s *Service) seedDemo(ctx context.Context, users map[string]User) error {
	as := func(name string) context.Context {
		return WithActor(ctx, Actor{User: users[name], Source: domain.SourceSystem})
	}
	owner := as("owner")
	if _, err := s.CreateProject(owner, CreateProjectInput{Key: "APP", Name: "タスク管理アプリ",
		Description: "AIエージェントと人間の協働タスク管理アプリ"}); err != nil {
		return err
	}
	states, err := s.ListStates(owner, "APP")
	if err != nil {
		return err
	}
	stateID := map[string]int64{}
	for _, st := range states {
		stateID[st.AppliesTo+":"+st.Name] = st.ID
	}

	type node struct {
		typ      domain.IssueType
		title    string
		state    string
		assignee string
		priority int
		labels   []string
		desc     string
		reporter string
		qa       *CreateQAInput
		children []node
	}
	no := false
	tree := []node{
		{typ: domain.TypeEpic, title: "認証基盤", state: "In Progress", assignee: "owner", priority: 2,
			desc: "ログイン・セッション・権限など、認証まわりの土台をまとめる。", children: []node{
				{typ: domain.TypeStory, title: "ユーザーとしてログインしたい", state: "受入待ち", assignee: "owner", priority: 2,
					desc: "## 受入条件\n\n- メールアドレスとパスワードでログインできる\n- 失敗したときは**日本語**でエラーが表示される\n- ログイン状態はブラウザを閉じても保持される", children: []node{
						{typ: domain.TypeFeature, title: "ログイン画面を実装する", state: "In Progress", assignee: "claude", priority: 2, labels: []string{"frontend"},
							desc: "デザイン案に沿ってログインフォームを作る。\n\n1. フォームの雛形\n2. 入力バリデーション（`zod`）\n3. API 呼び出しとエラー表示", children: []node{
								{typ: domain.TypeTask, title: "セッションCookieの設定", state: "In Progress", assignee: "claude"},
								{typ: domain.TypeTask, title: "入力バリデーション", state: "Done", assignee: "claude", children: []node{
									{typ: domain.TypeTask, title: "メール形式チェック", state: "Done", assignee: "claude"},
									{typ: domain.TypeTask, title: "パスワード長チェック", state: "Done", assignee: "claude"},
								}},
								{typ: domain.TypeQA, reporter: "claude", qa: &CreateQAInput{
									Title: "ログイン画面の仕様確認（3件）", IsBlocking: true,
									Context:             "要件にパスワード規則とエラー表示の記載がないため確認させてください。",
									ProvisionalDecision: "8文字以上・日本語エラーメッセージで実装を進めています。",
									Items: []QAItemInput{
										{Question: "パスワード規則はどれにしますか？", AnswerType: AnswerSingleChoice, AllowOther: true, Options: []QAOptionInput{
											{ID: "len8", Label: "8文字以上", Description: "一般的で実装が容易", Recommended: true},
											{ID: "len12sym", Label: "12文字以上 + 記号必須"},
											{ID: "len8mix", Label: "8文字以上 + 英数字混在"},
										}},
										{Question: "エラーメッセージは日本語のみで OK ですか？", AnswerType: AnswerOkNg, Options: []QAOptionInput{{ID: "ok", Recommended: true}}},
										{Question: "ログイン画面に表示したい要素をすべて選んでください", AnswerType: AnswerMultiChoice, Options: []QAOptionInput{
											{ID: "remember", Label: "ログイン状態を保持する"},
											{ID: "forgot", Label: "パスワードを忘れた方へ"},
											{ID: "signup", Label: "新規登録リンク"},
											{ID: "sso", Label: "SSO ボタン"},
										}},
									}}},
							}},
						{typ: domain.TypeFeature, title: "ログイン失敗時のロック", state: "Done", assignee: "codex", labels: []string{"backend"}},
					}},
				{typ: domain.TypeStory, title: "パスワードを再設定したい", state: "Backlog", priority: 3, children: []node{
					{typ: domain.TypeFeature, title: "再設定メール送信", state: "Todo", labels: []string{"backend"}},
				}},
			}},
		{typ: domain.TypeEpic, title: "Jira互換API", state: "Todo", assignee: "owner", priority: 1, children: []node{
			{typ: domain.TypeStory, title: "ORCAからタスク一覧を取得したい", state: "In Progress", assignee: "owner", children: []node{
				{typ: domain.TypeTask, title: "JQLパーサ", state: "In Progress", assignee: "codex", priority: 2, labels: []string{"backend"}},
				{typ: domain.TypeTask, title: "search API", state: "Todo", assignee: "codex", labels: []string{"backend"}},
				{typ: domain.TypeQA, reporter: "codex", qa: &CreateQAInput{
					Context: "ORCA が送る JQL を調べたところ、ORDER BY のキーは 1 つだけでした。",
					Items: []QAItemInput{
						{Question: "ORDER BY の複数キー対応は今回必要ですか？", AnswerType: AnswerYesNo, Options: []QAOptionInput{{ID: "no", Recommended: true}}},
						{Question: "他に対応してほしい JQL の書き方があれば教えてください", AnswerType: AnswerText, Required: &no},
					}}},
			}},
			{typ: domain.TypeStory, title: "ORCAからステータスを更新したい", state: "Backlog"},
		}},
		{typ: domain.TypeEpic, title: "マインドマップ表示", state: "Backlog", priority: 3, children: []node{
			{typ: domain.TypeStory, title: "全体像を俯瞰したい", state: "Backlog", children: []node{
				{typ: domain.TypeTask, title: "React Flow の導入検証", state: "Todo", assignee: "claude"},
			}},
		}},
		{typ: domain.TypeTask, title: "README を整備する", state: "Todo", assignee: "alice", priority: 4},
	}

	var create func(parent *string, n node) error
	create = func(parent *string, n node) error {
		reporter := n.reporter
		if reporter == "" {
			reporter = "owner"
		}
		applies := "all"
		if n.typ == domain.TypeQA {
			applies = "qa"
		}
		sid := stateID[applies+":"+n.state]
		in := CreateIssueInput{ProjectKey: "APP", Type: string(n.typ), Title: n.title, ParentKey: parent,
			StateID: &sid, Priority: &n.priority, Labels: n.labels, Description: n.desc}
		if n.assignee != "" {
			a := n.assignee
			in.Assignee = &a
		}
		if n.qa != nil {
			n.qa.TargetKey = parent
			_, err := s.CreateQA(as(reporter), *n.qa)
			return err
		}
		is, err := s.CreateIssue(as(reporter), in)
		if err != nil {
			return fmt.Errorf("seed %q: %w", n.title, err)
		}
		for _, c := range n.children {
			if err := create(&is.Key, c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range tree {
		if err := create(nil, n); err != nil {
			return err
		}
	}
	if _, err := s.AddComment(as("owner"), "APP-3", "Formik ではなく React Hook Form で問題ありません。"); err != nil {
		return err
	}
	p := func(n int) *int { return &n }
	checkpoints := []CheckpointInput{
		{Summary: "ログインフォームの雛形と API 呼び出しを実装", Decision: "フォームの状態管理に React Hook Form を採用",
			Rationale:    "既存の依存に含まれている / バリデーションライブラリとの連携が容易 / 再レンダリングが少ない",
			Alternatives: []string{"Formik（依存追加が必要なため不採用）", "自前の useState 管理（バリデーションの重複が増えるため不採用）"},
			Confidence:   "high", Progress: p(30), NextSteps: "入力バリデーションを子タスクに分けて実装する",
			Artifacts: map[string]any{"branch": "feat/login", "commits": []string{"a1b2c3d"}}},
		{Summary: "入力バリデーション（メール形式・パスワード長）を実装", Decision: "パスワードは暫定で 8 文字以上とする",
			Rationale:  "要件に規則の記載がない。一般的な基準で進め、QA で確認する",
			Confidence: "low", Progress: p(60), NextSteps: "QA の回答を待ってパスワード規則を確定する。並行してセッション Cookie を設定",
			Artifacts: map[string]any{"branch": "feat/login", "commits": []string{"d4e5f6a", "b7c8d9e"}, "files": []string{"web/src/login/LoginForm.tsx"}}},
	}
	for _, cp := range checkpoints {
		if _, err := s.RecordCheckpoint(as("claude"), "APP-3", cp); err != nil {
			return err
		}
	}
	_, err = s.RecordCheckpoint(as("codex"), "APP-14", CheckpointInput{Summary: "JQL の字句解析を実装", Decision: "ORDER BY は単一キーのみ対応で先に進める",
		Rationale: "ORCA の送る JQL は単一キーのみ。複数キーは QA で要否を確認中", Confidence: "medium", Progress: p(40),
		NextSteps: "構文解析と Filter への変換"})
	if err != nil {
		return err
	}

	// Acceptance: APP-2 was sent back once and is waiting for round 2.
	acc, err := s.SetCriteria(owner, "APP-2", []CriterionInput{
		{Description: "正しいメールアドレスとパスワードでログインできる"},
		{Description: "誤ったパスワードで 5 回失敗するとロックされる"},
		{Description: "エラーメッセージが日本語で表示される"},
	})
	if err != nil {
		return err
	}
	c := acc.Criteria
	if _, err := s.ReviewAcceptance(owner, "APP-2", ReviewInput{Verdict: "rejected", Summary: "エラーメッセージが英語のまま。ロック後の解除方法も未定",
		Items: []ReviewItemInput{{CriterionID: c[0].ID, Result: "ok"}, {CriterionID: c[1].ID, Result: "ng", Note: "ロックはされるが解除手段がない"},
			{CriterionID: c[2].ID, Result: "ng", Note: "「Invalid credentials」と表示される"}}}); err != nil {
		return err
	}
	if _, err := s.UpdateIssue(as("claude"), "APP-2", UpdateIssueInput{StateID: Some(stateID["all:受入待ち"])}); err != nil {
		return err
	}
	if _, err := s.SetCriteria(owner, "APP-17", []CriterionInput{{Description: "ORCA でステータスを変えると taskapp に即時反映される"}}); err != nil {
		return err
	}

	if err := s.seedDemoStoryMap(owner); err != nil {
		return err
	}

	// Saved views.
	app := "APP"
	if _, err := s.CreateView(owner, SavedViewInput{Name: "エージェントの作業中", ProjectKey: &app,
		Filter: domain.Filter{And: []domain.Filter{
			{Field: domain.FieldAssigneeKind, Op: domain.OpEq, Value: "agent"},
			{Field: domain.FieldStateCategory, Op: domain.OpIn, Value: []any{"started"}},
		}}, Display: json.RawMessage(`{"view":"board"}`)}); err != nil {
		return err
	}
	_, err = s.CreateView(owner, SavedViewInput{Name: "全体マップ（未完了）", ProjectKey: &app,
		Filter:  domain.Filter{Field: domain.FieldStateCategory, Op: domain.OpNotIn, Value: []any{"completed", "canceled"}},
		Display: json.RawMessage(`{"view":"map"}`)})
	return err
}

// seedDemoStoryMap fills the requirements layer of APP (docs/08-devcycle.md):
// two personas, a job, two activities with steps, and stories in every status.
func (s *Service) seedDemoStoryMap(ctx context.Context) error {
	str := func(v string) *string { return &v }
	keyOf := func(title string) (string, error) {
		var k string
		err := s.db.QueryRowContext(ctx, `SELECT key FROM issues WHERE title = ? AND deleted_at IS NULL`, title).Scan(&k)
		return k, err
	}
	if _, err := s.CreatePersona(ctx, "APP", PersonaInput{Name: "オーナー", Description: "複数のプロジェクトを並行して進め、作業を AI に任せる"}); err != nil {
		return err
	}
	if _, err := s.CreatePersona(ctx, "APP", PersonaInput{Name: "AI エージェント", Kind: "agent", Description: "MCP で作業を拾い、迷ったら質問する"}); err != nil {
		return err
	}
	for _, a := range []struct {
		name  string
		steps []string
	}{
		{"ログインする", []string{"ログイン画面を開く", "失敗から立ち直る"}},
		{"ORCA から使う", []string{"タスク一覧を取る", "ステータスを変える"}},
	} {
		act, err := s.CreateActivity(ctx, "APP", StoryActivityInput{Name: a.name, PersonaKey: str("APP-P1")})
		if err != nil {
			return err
		}
		for _, st := range a.steps {
			if _, err := s.CreateStep(ctx, "APP", act.Key, StepInput{Name: st}); err != nil {
				return err
			}
		}
	}
	if _, err := s.CreateJob(ctx, "APP", JobInput{PersonaKey: str("APP-P1"), Situation: "別の端末から使い始めるとき",
		Motivation: "すぐに自分の画面に入りたい", Outcome: "作業を止めずに済む", ActivityKeys: []string{"APP-A1"}}); err != nil {
		return err
	}
	stories := []struct {
		in       StoryInput
		criteria []ReqCriterionInput
		issues   []string
	}{
		{StoryInput{Title: "メールアドレスとパスワードでログインできる", IWant: "いつものアカウントで入りたい", SoThat: "どの端末でも同じ画面を使える", Release: "R1", StepKeys: []string{"APP-A1-1"}},
			[]ReqCriterionInput{{Given: "登録済みのユーザー", When: "正しいメールアドレスとパスワードを入れる", Then: "マイタスクが開く"},
				{Given: "ログイン画面", When: "誤ったパスワードを入れる", Then: "日本語のエラーが表示される"}}, []string{"ログイン画面を実装する"}},
		{StoryInput{Title: "失敗を繰り返すとロックされる", IWant: "総当たりで入られたくない", SoThat: "安心して外から使える", Release: "R1", StepKeys: []string{"APP-A1-2"}},
			[]ReqCriterionInput{{When: "5 回続けて失敗する", Then: "しばらくログインできなくなる"}}, []string{"ログイン失敗時のロック"}},
		{StoryInput{Title: "ORCA からタスク一覧を取得できる", Release: "R1", StepKeys: []string{"APP-A2-1"}},
			[]ReqCriterionInput{{Given: "ORCA に接続設定済み", When: "一覧を開く", Then: "自分の担当が表示される"}}, []string{"JQLパーサ", "search API"}},
		{StoryInput{Title: "パスワードを再設定できる", IWant: "忘れても自分で戻したい", Release: "R2", StepKeys: []string{"APP-A1-2"}},
			[]ReqCriterionInput{{When: "再設定を依頼する", Then: "メールで手順が届く"}}, nil},
		{StoryInput{Title: "ORCA からステータスを変えられる", Release: "R2", StepKeys: []string{"APP-A2-2"}}, nil, nil},
	}
	for _, st := range stories {
		st.in.PersonaKey = str("APP-P1")
		created, err := s.CreateStory(ctx, "APP", st.in)
		if err != nil {
			return err
		}
		for _, c := range st.criteria {
			if _, err := s.CreateCriterion(ctx, "APP", created.Key, c); err != nil {
				return err
			}
		}
		for _, title := range st.issues {
			k, err := keyOf(title)
			if err != nil {
				return fmt.Errorf("demo story map: issue %q: %w", title, err)
			}
			if _, err := s.LinkStoryIssue(ctx, "APP", created.Key, k); err != nil {
				return err
			}
		}
	}
	// Release slices (§5.8): name the outcome of each row. R1 is the walking skeleton.
	for name, patch := range map[string]ReleasePatch{
		"R1": {Goal: str("ログインして、ORCA から自分のタスクを読める"), Status: str(ReleaseBuilding), Metric: str("毎朝 ORCA から一覧を取っている")},
		"R2": {Goal: str("自分で立ち直れて、ORCA から状態も変えられる")},
	} {
		if _, err := s.UpsertRelease(ctx, "APP", name, patch); err != nil {
			return err
		}
	}
	// Option D: two stories written in one line from the quick input, not sorted yet
	// (no persona, step or release). One has a one-line criterion.
	quick := []struct {
		title, body string
		criteria    []string
	}{
		{"朝の画面で、AI に任せたものだけを見たい", "マイタスクを開いたとき、自分が起票して AI が担当しているものだけを並べたい。\n回答待ちの質問があれば一緒に見えるとよい。", []string{"AI が担当している自分の起票だけが並ぶ"}},
		{"週末に滞りを見たい", "金曜の夕方に、止まっているものをまとめて見たい。どこに置くかはまだ決めていない。", nil},
	}
	for _, q := range quick {
		created, err := s.CreateStory(ctx, "APP", StoryInput{Title: q.title, Body: q.body})
		if err != nil {
			return err
		}
		for _, c := range q.criteria {
			if _, err := s.CreateCriterion(ctx, "APP", created.Key, ReqCriterionInput{Then: c}); err != nil {
				return err
			}
		}
	}
	return nil
}
