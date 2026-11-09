# taskapp

AI エージェントと人間が一緒に使うタスク管理アプリです。ローカルで動く単一バイナリで、次の 3 つの入口を持ちます。

- **Web UI**（Linear ライク）: エピック → ストーリー → フィーチャー → タスク（タスクの入れ子も可）の階層、リスト / ボード / ツリー / マインドマップ / ロードマップ、QA インボックス、受入検査
- **MCP サーバー**: Claude Code などのエージェントがタスクを取得し、判断基準つきのチェックポイントを記録し、選択式の QA で人間に確認する
- **Jira REST API v3 互換**: ORCA（Jira 連携の Cloud モード）などの Jira クライアントからそのまま使える

使い方は [利用ガイド](docs/06-guide.md) を見てください。

## インストールと起動

リリースのアーカイブ（`taskapp_<version>_<os>_<arch>`）を展開して起動します。追加のミドルウェアは不要です（データは SQLite）。

```bash
./taskapp serve                 # http://127.0.0.1:7700 （データは ~/.taskapp）
./taskapp seed --preset demo --data /tmp/taskapp-demo && ./taskapp serve --data /tmp/taskapp-demo   # デモデータで試す
```

ソースからビルドする場合（Go 1.26+、Node 22+、pnpm）:

```bash
pnpm install && make build      # bin/taskapp
make demo-fresh                 # /tmp/taskapp-demo にデモデータを作って起動
make dist                       # macOS / Linux / Windows 向けアーカイブを dist/ に作成
```


## コマンド

| コマンド | 内容 |
|---|---|
| `taskapp serve [--port 7700] [--data DIR] [--log-requests]` | Web UI・API・MCP（HTTP）・Jira 互換 API を起動（既定で 127.0.0.1 のみ）。1 日ごとに自動バックアップ（`--backup-interval` / `--backup-keep` / `--backup-dir`） |
| `taskapp serve --host 0.0.0.0` | ほかの端末（スマホ・別の PC）からも使う。画面はパスワードでログイン。通信は暗号化されないので LAN か VPN の中で |
| `taskapp serve --behind-proxy --public-url URL` | 同じマシンのリバースプロキシ（`tailscale serve` など）の後ろで動かす。自動ログインは止まる |
| `taskapp passwd --user NAME` | 画面ログイン用のパスワードを設定（画面の「設定 › アカウント」からも可） |
| `taskapp mcp [--data DIR]` | MCP サーバー（stdio）。`TASKAPP_TOKEN` で認証 |
| `taskapp token create --user NAME --name LABEL` | API トークンを発行（画面の「設定」からも発行可） |
| `taskapp export --out FILE` / `import --in FILE` | 全データの JSON エクスポート / 空のデータディレクトリへの取り込み |
| `taskapp backup [--out FILE]` | SQLite ファイルのバックアップ（起動中でも可。`--out` なしは自動バックアップと同じフォルダへ） |
| `taskapp restore --from FILE` | バックアップから戻す（サーバーを止めてから。いまの DB は退避して残す） |
| `taskapp seed --preset demo\|test\|perf` | デモ・テスト・性能確認用のデータ |

## 開発

```bash
make test        # Go の単体テスト + 実装に依存しない API テスト（apitest/、Vitest）
make test-e2e    # 主要画面のテスト（e2e/、Playwright）。初回は cd e2e && pnpm exec playwright install chromium
make perf        # 約 1 万件のデータで主要な問い合わせの時間を計測
make dev-api     # API を :7700 で起動 / make dev-web で Vite（:5173）
```

API テストと画面テストは起動したバイナリを外から操作するだけなので、バックエンドを別の言語で作り直しても `make test-api TASKAPP_BIN=...` / `make test-e2e TASKAPP_BIN=...` で同じテストを流せます。詳しくは [テストの使い方](docs/07-testing.md)。

## ドキュメント

| # | ドキュメント | 内容 |
|---|---|---|
| 01 | [要求仕様](docs/01-requirements.md) | 目的、アクター、機能要求、Jira 互換範囲、MCP ツール、非機能要求 |
| 02 | [アーキテクチャ](docs/02-architecture.md) | 構成図、技術スタック、パッケージ構成、設計判断 |
| 03 | [画面設計](docs/03-screens.md) | 画面一覧、画面イメージ、画面遷移図 |
| 04 | [ER 図](docs/04-er-diagram.md) | テーブル定義、インデックス、代表クエリ |
| 05 | [実装計画](docs/05-implementation-plan.md) | フェーズと確認ポイント、テスト戦略、リスク |
| 06 | [利用ガイド](docs/06-guide.md) | インストール、エージェント / ORCA の接続、日々の使い方、自動バックアップと復元、ほかの端末から使う |
| 07 | [テストの使い方](docs/07-testing.md) | 単体・API・画面テストの流し方、失敗の調べ方、書き足し方 |

各確認ポイントの手順は [docs/checkpoints/](docs/checkpoints/) にあります。

## スライド

ブラウザで開くと表示されます（← → / Space で移動、P で印刷・PDF 出力）。一覧は [docs/slides/index.html](docs/slides/index.html)。

| スライド | 元ドキュメント |
|---|---|
| [要求仕様](docs/slides/requirements.html) | 01 |
| [アーキテクチャ設計](docs/slides/architecture.html) | 02 |
| [画面設計](docs/slides/screens.html) | 03 |
| [ER 図・データモデル](docs/slides/er.html) | 04 |
| [実装計画](docs/slides/plan.html) | 05 |
| [利用ガイド](docs/slides/guide.html) | 06 |
| [テストの使い方](docs/slides/testing.html) | 07 |

元データは `docs/slides/src/<deck>/`（`deck.json` と 1 スライド 1 ファイル）。編集後は `python3 scripts/build_slides.py` で全デッキと一覧を再生成します。

## ライセンス

[MIT](LICENSE)
