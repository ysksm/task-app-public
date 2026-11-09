# ✅ 確認ポイント 3（P3: QA・チェックポイント・MCP）

確認ポイント 1 のレビューを受けて、P2（Jira 互換）より先に P3 を実装しました。

## 起動方法

```bash
pnpm install && make build
make demo-fresh        # /tmp/taskapp-demo に新しいデモデータを作って起動（http://127.0.0.1:7700）
```

確認ポイント 1 で作った `~/.taskapp` のデモデータは、QA に質問が入っていない古い形式です。今回の確認には `make demo-fresh` を使ってください（`~/.taskapp` には触れません）。

## 確認していただきたい操作

### 1. QA に回答する（人間）

- サイドバー「QA インボックス」。デモには Claude と Codex が起票した QA が 2 件あります。
- **APP-8**（Claude、ブロッキング）: 1 つの QA に 3 つの質問。
  - 単一選択（推奨つき。「その他」に自由記述も可）
  - OK / NG
  - 複数選択
- **APP-16**（Codex）: はい / いいえ と、任意のテキスト質問。
- 各質問に「補足」を付けられます。必須の質問がすべて埋まると「回答済」になります。一部だけ先に保存することもでき、クローズするまでは「回答を修正」で直せます。
- 回答すると、対象イシュー（APP-3）の赤い「ブロッキング QA」バッジが消えます。

### 2. エージェントの記録を見る

- イシュー詳細のアクティビティに **チェックポイント** が表示されます（APP-3、APP-14）。
  - 判断・**判断基準**・代替案・次のステップ・成果物（ブランチ / コミット）・進捗
  - 確信度「低」は赤枠で表示
- サイドバー「エージェント活動」: エージェントごとのチェックポイント・質問・ステータス変更。「確信度『低』のみ」で要確認のものだけ表示できます。

### 3. エージェントを接続する（MCP）

1. 「設定」→ API トークンで、エージェント（例: Claude）のトークンを発行する。
2. 「エージェントの接続（MCP）」に表示されるコマンドをコピーして実行する（バイナリとデータの場所は入力済み）。

   ```bash
   claude mcp add taskapp -e TASKAPP_TOKEN=<token> -- /path/to/bin/taskapp mcp --data /tmp/taskapp-demo
   ```

3. Claude Code に「taskapp の自分の担当を確認して、APP-4 を進めて。判断はチェックポイントに残し、迷ったら ask_question で質問して」などと頼む。

MCP ツール（14 個）: `list_projects`, `list_my_issues`, `search_issues`, `get_issue`, `get_issue_tree`, `create_issue`, `update_issue`, `transition_issue`（状態名で指定）, `add_comment`, `record_checkpoint`, `ask_question`, `get_question_answers`, `close_question`, `request_acceptance`。ほかにリソース `taskapp://issue/{key}` とプロンプト `work_on_issue` があります。

## 実際の Claude Code での動作確認（実施済み）

この環境の Claude Code（v2.1.296）を stdio で接続し、次の流れを通しました。

1. エージェント: `list_my_issues` → APP-4 を In Progress に → `record_checkpoint`（Cookie 属性の方針、判断基準・代替案つき）→ `ask_question`（有効期限の単一選択 + 「ログイン状態を保持する」の OK/NG）
2. 人間（ブラウザ）: QA インボックスで「7日」と「OK」を選び、補足「共有端末では OFF を既定に」を付けて回答
3. エージェント: `get_question_answers` で補足まで読み取り、回答に沿った判断を `record_checkpoint` に記録 → `close_question`

## できていること

| 要求 | 内容 |
|---|---|
| AGT-01〜06 | チェックポイント（判断基準は必須）、タイムライン表示、エージェント活動フィード、確信度低の絞り込み、エージェントによる子タスク作成 |
| QA-01〜07 | 1 チケット複数質問、5 つの回答形式（はい/いいえ・OK/NG・単一選択・複数選択・テキスト）、補足、その他、部分回答・修正・クローズ、ブロッキング表示、インボックスと件数バッジ |
| MCP-01〜06 | stdio（`taskapp mcp`）と Streamable HTTP（`/mcp`）、トークン認証、14 ツール、リソース、プロンプト。stdio での変更も開いている画面に即時反映 |
| 設定 | メンバー一覧、エージェント追加、API トークンの発行・失効、MCP 接続コマンド |

## テスト

`make test` で API テスト 105 件（MCP は公式 TypeScript クライアントで stdio / HTTP の両方を検証）。実装に依存しないので、Rust 版でもそのまま使えます。

## まだできていないこと

- Jira 互換 API v3（P2、次に着手）
- 受入検査画面・マインドマップ・保存ビュー（P4）
- 画面から QA を起票する専用フォーム（いまは API / MCP から。画面の「新規」で QA 種別を選ぶと質問なしの QA になる）

## 確認したいこと

1. QA の回答画面（選択肢カード、OK/NG ボタン、複数選択、その他、補足）の使い勝手
2. チェックポイントの項目と見せ方（判断基準の強調、確信度「低」の扱い）
3. 画面から人間が QA を起票するフォームは必要か（エージェント→人間の質問だけでよいか）
4. 次は P2（Jira v3 / ORCA 連携）に進めてよいか
