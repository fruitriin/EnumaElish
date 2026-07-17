# Plan 0029: hook 評価結果ログ永続化と `ccchain stats`（Issue #17）

## 実装状況: 未着手

## 背景

Issue [#17](https://github.com/fruitriin/EnumaElish/issues/17)（オーナー ADDF ドッグフーディング報告、2026-07-17 05:26）:

- 現状、ccchain の評価結果（allow/ask/deny・マッチしたルール・コマンド）は hook の都度出力のみで
  **永続化されない**
- 実運用で「何がどれだけ deny/ask されたか」の実績を知るには Claude Code のセッショントランスクリプト
  （JSONL）を発掘するしかない
- 実例: dev ビルドの「dynamic command detected」deny が直近セッション群で21件発生していたことを、
  transcript 横断 grep + 直前 tool_use との突合で調べた（主因は for ループ、v0.2.0 で解消見込み）
- **`ccchain stats` 一発でできると、conf チューニングのループが速くなる**

## スコープ

1. **`settings: log:` オプトイン**: `settings.log: .ccchain/log.jsonl` 等で hook 評価結果を
   JSONL 追記する
   - 1行スキーマ: `{timestamp, tool_name, command (先頭 N 字で切り詰め), action, matched_rule, message, permission_mode, session_id, cwd}`
   - デフォルト無効（後方互換）
   - パーミッション 0600、ディレクトリ 0700（承認ストアと同じ方針）
2. **`ccchain stats [--since <duration>] [--group-by action|rule|command]`**:
   ログを読んで action 別・ルール別・コマンド別集計を表示
   - `--since 24h`, `--since 7d` 等の時間フィルタ
   - トップ N 件表示（デフォルト 20）
   - JSON 出力オプション（`--json`）で他ツール連携
3. **既存 `internal/audit/` の統合か新設か**: 既に監査ログ機構があるので、そこへの評価結果統合が
   自然。ただし audit は「ルール展開の可視化」なのでスコープが違う → 新規パッケージ
   `internal/evallog/` を推奨（設計時に判断）
4. **gitignore 推奨**: 「コマンド文字列に秘密が混ざりうるため」ログはリポジトリ内デフォルト
   gitignore 推奨。`ccchain init` の Next steps で案内、`.gitignore` テンプレートに追記

## スコープ外

- ログの自動ローテーション（別 Plan または新機能として後日）
- Web UI やダッシュボード（CLI 出力で十分）
- 承認ストアとの統合（audit.jsonl は既に承認履歴を持つが、ログはより低頻度の集計向け）

## 設計原則

- **オプトイン**: デフォルトオフ。既存ユーザーの挙動は変わらない
- **軽量**: hook 経路のオーバーヘッドは最小（JSONL 追記のみ、集計はオフライン）
- **秘密漏洩の抑制**: command の切り詰めデフォルトは 200 字、`command_length_limit` で調整可
- **fail-open**: ログ書き込み失敗は hook の allow/deny 判定に影響させない（stderr に警告のみ）

## Phase 分割

- **Phase 1**: `internal/evallog/` パッケージと `settings.log:` パーサー、hook 経路統合
- **Phase 2**: `ccchain stats` サブコマンド（--since / --group-by / --json）
- **Phase 3**: ドキュメント（`docs/reference/stats.md`）と README/CHANGELOG 反映
- **Phase 4**（要検討）: `ccchain stats --tail`（`tail -f` 相当）と `--filter action=deny` 等

## テスト戦略

- `evallog_test.go`: 並行書き込み、パーミッション、切り詰め、fail-open 挙動
- `stats_test.go`: 集計ロジック（time filter、group-by、JSON 出力）
- 実機: ADDF ドッグフーディングで1日ログを取り、Issue #17 の実例（deny 21件の内訳）が
  再現できることを確認

## 参照

- Issue: https://github.com/fruitriin/EnumaElish/issues/17
- 関連コード: `internal/audit/` (既存、参考にする)
