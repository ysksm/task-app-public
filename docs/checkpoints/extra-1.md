# ✅ 確認: 画面テストと自動バックアップ（確認ポイント 5 のあとの追加作業）

P5 は main にマージ済みです（PR #3）。この追加作業はブランチ `feat/e2e-backup` です。

## 起動

```bash
make build
./bin/taskapp seed --preset demo --data /tmp/taskapp-extra
./bin/taskapp serve --data /tmp/taskapp-extra      # http://127.0.0.1:7700（前のサーバーは止めてから）
```

## 確認していただきたい操作

### 1. 自動バックアップ

- 設定 › データ の「自動バックアップ」に、起動時に取った 1 件が出ていること（保存先は `/tmp/taskapp-extra/backups`）。
- 「今すぐバックアップ」で 1 件増えること。
- 間隔を短くして動きを見るなら: `./bin/taskapp serve --data /tmp/taskapp-extra --backup-interval 1m --backup-keep 3` で起動し、画面で何か変更して 1〜2 分待つと増えます（変更しなければ増えません）。

### 2. 復元

```bash
# サーバーを止めてから
./bin/taskapp restore --data /tmp/taskapp-extra --from /tmp/taskapp-extra/backups/<ファイル名>
./bin/taskapp serve --data /tmp/taskapp-extra
```

- バックアップを取ったあとの変更が消えて、取ったときの状態に戻ること。
- `/tmp/taskapp-extra/taskapp.db.before-restore-<日時>` が残ること。
- サーバーを動かしたまま `restore` すると断られること。

### 3. 画面テスト

```bash
cd e2e && pnpm exec playwright install chromium   # 初回だけ
cd .. && make test-e2e                             # 20 件、10 秒ほど
cd e2e && pnpm exec playwright test --headed       # ブラウザを表示しながら流す
```

使い方は [07 テストの使い方](../07-testing.md)、バックアップは [06 利用ガイド](../06-guide.md) の「6. バックアップと復元」にまとめました。

## 確認済みのこと

- 単体テスト（`internal/backup`）、API テスト 164 件、画面テスト 20 件がすべて成功。画面テストは 4 回繰り返しても安定。
- 自動バックアップ: 起動時の 1 件、DB が変わっていなければ取らない、保持数を超えた古いものから削除、他のファイルや `.tmp` には触れない、同じ秒に取ったときは `-1` を付ける。
- 手動の `taskapp backup` は、`--backup-keep` を付けない限り古いバックアップを消さない。
- `restore`: 起動中は断る、taskapp の DB でないファイルを断る、いまの DB を退避してから置き換える。

## 確認できていないこと・制限

- 24 時間間隔は実際の 1 日では動かしていません（1 秒間隔で確認）。判定は 1 分ごとで、スリープ明けにも追いつく作りです。
- Windows では未確認です。設定画面の復元コマンドは保存先と `/` でつないでいます。
- `restore` は `taskapp serve` が動いているかだけを確認します。MCP（stdio）でつながっているエージェントは止めてから実行してください。
- 既定の保存先はデータと同じディスク（`<データ>/backups`）です。ディスクの故障に備えるなら `--backup-dir` で別の場所を指定してください。

## 確認したいこと

1. 既定値（24 時間ごと・7 件・`<データ>/backups`）でよいか
2. 既定の保存先を、同期フォルダなど別の場所にすすめる書き方にするか
3. 画面テストで、ほかに押さえておきたい操作があるか

## 確認の結果（2026-10-12）

1. 既定値（24 時間ごと・7 件・`<データ>/backups`）のままでよい。
2. 保存先は `--backup-dir` でパスを指定できればよい。ガイドの書き方は今のまま（6.1 に例を記載済み）。
3. 画面テストは今の範囲でよい。
