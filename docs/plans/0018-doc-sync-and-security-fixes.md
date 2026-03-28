# Plan 0018: ドキュメント同期 + セキュリティ Medium 修正

## 背景

ドキュメントレビュー（Critical 4件）とセキュリティレビュー（Medium 4件）の指摘を対応する。

## ドキュメントレビュー Critical

### C-1: detect/test/generate-rules が CLI ドキュメント・printUsage に未記載

- `printUsage()` に 3 コマンドを追加
- `docs/guide/cli.md` (EN/JA) に各セクション追加
- README (EN/JA) のサブコマンド一覧に追加

### C-2/C-3/C-4: ロードマップが実装済み機能を「未実装」と記載

- Phase 9〜16 の全てを「実装済み」ステータスに更新
- 「現在の実装状況」セクションに全機能を追加
- `{timestamp}` 変数をドキュメント追加
- 推奨実装順序テーブルを「完了」に更新

## ドキュメントレビュー Warning

### W-1: config.md のマージ動作説明の誤り

`.ccchain.local.conf` にルールを置いてもグローバル設定に上書きされる件を正しく説明

### W-2: DSL リファレンスに workspace: 未記載

settings: ブロックの例に workspace: を追加

### W-3: README サブコマンド一覧不完全（C-1 と同根）

### W-4: ロードマップ「現在の実装状況」が不完全（C-2〜4 と同根）

## ドキュメントレビュー Suggestion

### S-1: how-it-works.md の Decision Output に hint 欠落

### S-2: 日本語版 permissive-mode.md に関数定義の行がない

### S-3: README の「4つのアクション」→「5つのアクション」

## セキュリティレビュー Medium

### M-1: scope.go の相対パスを ScopeInside と見なす設計

→ Plan 0017 で対応予定（Bash 引数スコープ）。本 Plan では `CLAUDE_PROJECT_DIR` 参照を追加

### M-2: tool.go のスコープ外が ask 止まり

→ ドキュメントに「スコープ外は ask（deny ではない）」を明記

### M-3: MCP ツールの引数がスコープ・args: 非対応

→ best-effort で file_path/path/url キーを解析

### M-4: semantics/table.go の複合サブコマンドの regex バグ

→ スペースを `\\s+` に変換

## 変更対象

| ファイル | 変更 |
|---|---|
| `cmd/ccchain/main.go` | printUsage に 3 コマンド追加 |
| `cmd/ccchain/hook.go` | MCP 引数の best-effort 解析 |
| `internal/semantics/table.go` | 複合サブコマンドの regex 修正 |
| `internal/eval/scope.go` | CLAUDE_PROJECT_DIR 参照追加 |
| `docs/guide/cli.md` (EN/JA) | detect/test/generate-rules セクション |
| `docs/guide/roadmap.md` (EN/JA) | 全 Phase を実装済みに更新 |
| `docs/reference/dsl.md` (EN/JA) | workspace: 追加 |
| `docs/reference/config.md` | マージ動作の説明修正 |
| `README.md`, `README.en.md` | サブコマンド一覧 + 5アクション |
