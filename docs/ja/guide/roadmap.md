# ロードマップ

ccchain の現状と、近い将来のリスト（およびリストに乗っていないもの）。リリースの約束ではなくステータスページです。

## 現在の実装状況

以下の機能は実装済みです:

- 構造的コンテキスト評価（パイプ / チェーン / サブシェル / リテラル `for` ループ）
- Bash およびツール（Read / Edit / Write / MCP）に対する `allow` / `deny` / `warn` / `ask` / `hint`
- `args:` による引数レベル正規表現ルール（長大引数のエスカレーション付き）
- ルール単位の `scope:` によるワークスペースアクセス制御（inside / outside-read / outside-write）— Bash / Read / Edit / Write / MCP に適用
- 最新の hook I/O — exit 0 + `hookSpecificOutput.permissionDecision` JSON、`permission_mode` / `session_id` / `cwd` の読取
- [`ask_strategy`](../reference/dsl#ask_strategy) とルール単位の [`unattended:`](../reference/dsl#unattended) — 非対話モードでの ask 降格
- [承認トークン](../reference/approve)（`ccchain approve --last` / `--list` / `<hash-prefix>` / `--revoke-all`、TTL、session+cwd スコープ、ワンショット消費）
- [sentinel プリセット](../reference/sentinel)（`ccchain init --sentinel`）— auto / dontAsk / headless 向けの deny-first キュレートルール
- セマンティクステーブルとプロジェクト自動検出（`ccchain generate-rules` / `ccchain detect`）
- メッセージテンプレート（`{command}` / `{args}` / `{id}` / `{cwd}`、サニタイズ付き）
- CLI: `check` / `eval` / `test` / `diff` / `suggest` / `audit` / `init [--sentinel]` / `hook pre|post` / `approve` / `detect` / `generate-rules` / `version`
- 複数ファイル設定のマージ順（プロジェクト → ローカル → グローバル）

---

## 完了した Phase（履歴）

以下の Phase はすべて実装済みで、対応する機能はリンク先のリファレンスに記載されています。

| Phase | 機能 | リファレンス |
|---|---|---|
| 9 | DSL 一貫性修正（`mode:` 非推奨化） | [DSL 構文](../reference/dsl) |
| 10 | settings 互換 + デフォルトルール強化 | [設定ファイル](../reference/config) |
| 11 | ワークスペーススコープ | [DSL `scope:`](../reference/dsl#scope) |
| 12 | メッセージテンプレート | [DSL メッセージ](../reference/dsl#メッセージ) |
| 13 | コマンドセマンティクステーブル | `ccchain generate-rules` |
| 14 | マルチツール制御（Read / Edit / Write / MCP） | [設定 Hook 入力フォーマット](../reference/config#hook-入力フォーマット) |
| 15 | プロジェクト自動検出 | `ccchain detect` |
| 16 | Deny リダイレクト / read-write 分離 | [DSL `scope:` outside-write](../reference/dsl#scope) |
| 22 | Ask 戦略 + deny-first sentinel（Plan 0022） | [`ask_strategy`](../reference/dsl#ask_strategy) / [承認トークン](../reference/approve) / [sentinel プリセット](../reference/sentinel) |
| 25 | リテラル `for` ループ展開 + `unanalyzable_action` | [DSL `unanalyzable_action`](../reference/dsl#unanalyzable_action) |

---

## 進行中 / 未計画

- **ADDF 統合**（Plan 0023）— `/addf-init` での hook 自動設営
- **リモート / Slack 承認連携**（Plan 0024）— 承認トークンのネットワーク経由配送
- **REPL / stats サブコマンド** — Plan 0019 DX 改善の残り（`diff` は実装済み）
- **PostToolUse ターンカウント** — 同一ツールの連続呼び出しを制限
- **`source` / `.` コマンドの追跡** — 静的解析の原理的限界。技術的解決策ではなくドキュメント明記が計画の成果物
