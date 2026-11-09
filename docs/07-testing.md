# 07. テストの使い方

taskapp のテストの種類、流し方、書き足し方をまとめます。方針（なぜこの構成か）は [05 実装計画](05-implementation-plan.md) の「3. テスト戦略」 を見てください。

## 1. テストの種類

| 種類 | 場所 | 技術 | 件数 | 見ているもの |
|---|---|---|---|---|
| 単体テスト | `internal/**/*_test.go` | Go `testing` | — | 階層規則、JQL パーサ、ADF 変換、自動バックアップの命名・削除、この PC かどうかの判定、パスワードのハッシュ形式 |
| API テスト | `apitest/tests/` | TypeScript + Vitest | 180 | `/api/v1`、Jira 互換 API、MCP（HTTP / stdio）、ログイン、リバースプロキシの後ろ、CLI（export / import / backup / restore / passwd） |
| 画面テスト | `e2e/tests/` | Playwright（Chromium） | 27 | 主要画面の操作。ブラウザで操作し、結果を API で確かめる |

API テストと画面テストは **ビルドしたバイナリを起動して外から操作するだけ** です。バックエンドを Rust などで作り直しても、`TASKAPP_BIN` を差し替えれば同じテストで確かめられます。

## 2. 流し方

前提: Go 1.24+、Node 22+、pnpm。最初に `pnpm install` を実行します。

```bash
make test        # 単体テスト + API テスト（10 秒ほど）
make test-go     # 単体テストだけ
make test-api    # API テストだけ（Go のバイナリをビルドしてから実行）
make test-e2e    # 画面テスト（Web UI を含めてビルドしてから実行。約 10 秒）
```

画面テストを初めて流すときは、Playwright のブラウザを入れます（1 回だけ）。

```bash
cd e2e && pnpm exec playwright install chromium
```

### 一部だけ流す

```bash
cd apitest && TASKAPP_BIN=../bin/taskapp pnpm vitest run tests/backup.test.ts
cd e2e && pnpm exec playwright test tests/acceptance.spec.ts      # ファイル単位
cd e2e && pnpm exec playwright test -g "drag"                       # テスト名で絞る
```

`make test-e2e` を一度流してバイナリができていれば、`e2e/` の中で `pnpm exec playwright test` を直接実行できます（既定で `../bin/taskapp` を使います）。

### 別の実装を確かめる

```bash
make test-api TASKAPP_BIN=/path/to/other/taskapp
make test-e2e TASKAPP_BIN=/path/to/other/taskapp
```

## 3. 画面テストが失敗したとき

| やりたいこと | コマンド |
|---|---|
| 失敗の一覧と、そのときのスクリーンショット・トレースを見る | `cd e2e && pnpm exec playwright show-report` |
| 1 件のトレース（操作ごとの画面・DOM・通信）を見る | `pnpm exec playwright show-trace test-results/<テスト名>/trace.zip` |
| ブラウザを表示しながら流す | `pnpm exec playwright test --headed` |
| 1 ステップずつ止めながら流す | `pnpm exec playwright test --debug -g "テスト名"` |
| 対話的に選んで流す | `pnpm exec playwright test --ui` |
| 不安定なテストを探す | `pnpm exec playwright test --repeat-each=5` |

失敗したテストのスクリーンショットとトレースは `e2e/test-results/` に残ります（git には入りません）。

## 4. 画面テストの仕組み

- **テストごとに新しいサーバー** を起動します。空いているポートで、`seed --preset demo` のデータ（プロジェクト `APP`、イシュー 21 件、QA 2 件、受入待ち 1 件）が入った状態から始まります。テスト同士が影響しないので、並列に流れます。
- サーバーの起動は API テストと同じ `apitest/src/harness.ts` を使います。
- ブラウザは自動でオーナーとしてログインします（この PC からの接続のため）。ほかの端末からの接続を再現したいときは `test.use({ serveArgs: ["--require-login"] })` で自動ログインを止めます（`login.spec.ts`）。
- 言語は `ja-JP`、タイムゾーンは `Asia/Tokyo`、画面サイズは 1440×900 です。

### いまある画面テスト

| ファイル | 内容 |
|---|---|
| `issues.spec.ts` | マイタスク → 詳細パネル、ステータス変更、`C` で新規作成、エージェントの変更が再読み込みなしで反映される |
| `qa.spec.ts` | 選択式の QA に回答して未回答一覧から消える、必須の残り件数の表示 |
| `acceptance.spec.ts` | 警告の対象一覧からイシューを開く、全基準 OK で合格、NG で差し戻し |
| `palette.spec.ts` | `⌘K` でキー検索して開く、パレットから画面へ移動、`G` → `A` / `Q`、`?` |
| `views.spec.ts` | フィルタをビューとして保存し、サイドバーから開く |
| `map.spec.ts` | 深さで折りたたみ、ドラッグで親の付け替え、規則違反のエラー、PNG 保存 |
| `settings.spec.ts` | トークン発行（発行したトークンで API が使える）、JSON エクスポートのダウンロード、自動バックアップの表示と「今すぐバックアップ」 |
| `login.spec.ts` | ログイン画面へ移って元の画面に戻る、間違ったパスワード、オーナー以外にはオーナー用の設定を出さない、ログアウト、設定でのパスワード設定、ログアウト後の「このPCのオーナーとして入る」、ほかの人のパスワード設定、ほかの端末向けの案内 |

## 5. 画面テストを書き足す

`e2e/tests/` に `*.spec.ts` を置きます。`test` と `expect` は `../fixtures.js` から読み込みます。

```ts
import { expect, test } from "../fixtures.js";

test("エージェントが担当を自分に変えると、マイタスクから消える", async ({ page, demo }) => {
  await page.goto("/my");
  await expect(page.getByText("Jira互換API")).toBeVisible();

  // 準備や結果の確認は API で（エージェントとしての操作もこれで書ける）
  const res = await demo.api("claude").patch("/api/v1/issues/APP-12", { assignee: "claude" });
  expect(res.status).toBe(200);

  await expect(page.getByText("Jira互換API")).toHaveCount(0);
});
```

| 使えるもの | 内容 |
|---|---|
| `page` | Playwright のページ。`page.goto("/my")` のように相対パスで開ける |
| `demo.api(who)` | デモユーザー（`owner` / `alice` / `claude` / `codex`）として API を呼ぶクライアント |
| `demo.server` | 起動中のサーバー（`baseUrl`、`dataDir`、`cli(...)` で CLI を実行、`restart()` で同じデータのまま起動し直す。ポートが変わるので `baseUrl` も変わる） |
| `peek(page)` | 右側の詳細パネル。`toHaveAttribute("data-issue-key", "APP-3")` で開いているイシューを確かめる |

書くときの決まり:

- 要素は **画面に出ている文言とロール** で探します（`getByRole("button", { name: "回答する" })`、`getByPlaceholder("タイトル")`）。CSS クラスには頼りません。文言を変えたらテストも直します。
- `data-testid` は文言で特定しにくい場所にだけ付けます（いまは詳細パネル `issue-detail` と自動バックアップ欄 `auto-backup`）。
- 画面で操作したら、**API でも結果を確かめます**（`expect.poll` で反映を待つ）。画面の表示だけでは保存されたか分からないためです。
- 一覧のボタンを `.all()` で数えるときは、先に何かが表示されるのを `expect` で待ちます（`.all()` は待たないため）。
- `window.confirm` を出す操作（削除・トークン失効）は、Playwright が自動で「キャンセル」します。確かめたいときは `page.once("dialog", (d) => d.accept())` を先に書きます。
- 時刻の表示（「3 分前」など）は実行のたびに変わるので、文言では確かめません。

## 6. API テストを書き足す

`apitest/tests/` に `*.test.ts` を置きます。`startServer()` でサーバーを起動し、`srv.as("owner")` などで呼びます（`test` プリセットのトークン）。レスポンスの形は `assertOperation("operationId", status, body)` で OpenAPI 定義と照らし合わせます。

```ts
const srv = await startServer({ preset: "demo", serveArgs: ["--backup-interval", "0"] });
```

| オプション | 内容 |
|---|---|
| `preset` | `test`（既定）/ `demo` / `perf` / `none` |
| `setup(cli)` | サーバー起動前に CLI を実行する（`import` や `restore` など） |
| `serveArgs` | `serve` に渡す追加のフラグ |

API を変えたときは `api/openapi.yaml` を直し、`pnpm --filter taskapp-apitest gen:types` と `pnpm --filter taskapp-web gen:types` で型を作り直します。
