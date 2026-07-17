# TODO

`.claude/addf/plans/` の完了状態・優先度をトラックする。
`.claude/addf/plans/` と TODO が一致しなければ TODO を編集する。

## 現在のフェーズ: v0.2.0 リリース後の緊急対応 — Plan 0027（Issue #15 セマンティクス回帰）を最優先

## バックログ

セキュリティレビュー推奨順序で並べ替え済み。

| 優先度 | Phase | 計画ファイル | 状態 | セキュリティ根拠 |
|---|---|---|---|---|
| 1 | 1 | `.claude/addf/plans/0001-bpt-dsl-design-and-parser.md` | 完了 | |
| 2 | 2 | `.claude/addf/plans/0002-bpt-shell-ast-analysis.md` | 完了 | |
| 3 | 3 | `.claude/addf/plans/0003-bpt-evaluation-and-hook-integration.md` | 完了 | |
| 4 | 4 | `.claude/addf/plans/0004-bpt-audit-and-addf-integration.md` | 完了 | |
| 5 | 5 | `.claude/addf/plans/0005-security-hardening.md` | 完了 | |
| 6 | 6 | `.claude/addf/plans/0006-code-quality-refactoring.md` | 完了 | |
| 7 | 7 | `.claude/addf/plans/0007-documentation-and-polish.md` | 完了 | |
| 8 | 8 | `.claude/addf/plans/0008-args-rule-evaluation.md` | 完了 | |
| 9 | 9 | `.claude/addf/plans/0009-mode-property-and-doc-accuracy.md` | 完了 | mode: 誤動作を早期修正、ドキュメント誤誘導排除 |
| 10 | 14 | `.claude/addf/plans/0014-multi-tool-control.md` | 完了 | Read/Edit の hook 枠組みを先に作る（0011 の前提） |
| 11 | 11 | `.claude/addf/plans/0011-workspace-scope-access-control.md` | 完了 | 0014 があれば Bash + Read/Edit 両方にスコープ適用可 |
| 12 | 16 | `.claude/addf/plans/0016-deny-redirect.md` | 完了 | 0014 依存。マルチツール制御後に実装 |
| 13 | 10 | `.claude/addf/plans/0010-settings-compat-and-ruleset-enhancement.md` | 完了 | 0011 完成後に安全なデフォルトを設計できる |
| 14 | 13 | `.claude/addf/plans/0013-command-semantics-table.md` | 完了 | 0010 のデフォルトルールと統合 |
| 15 | 15 | `.claude/addf/plans/0015-project-auto-detect.md` | 完了 | 0013 のテーブルを活用できる |
| 16 | 12 | `.claude/addf/plans/0012-deny-message-templates.md` | 完了 | 基盤が固まってから最後に実装 |
| 17 | 18 | `.claude/addf/plans/0018-doc-sync-and-security-fixes.md` | 完了 | ドキュメント同期 Critical 4件 + セキュリティ Medium 4件 |
| 18 | 17 | `.claude/addf/plans/0017-bash-args-scope.md` | 完了 | Bash コマンド引数にもワークスペーススコープ適用 |
| 19 | 22 | `.claude/addf/plans/0022-ask-strategy-and-deny-first-sentinel.md` | 完了 | auto/dontAsk/headless の ask 降格・ccchain approve・sentinel プリセット・hook I/O 現代化。2026-07-17 完了、Stage 2 レビュー主題外は Plan 0026 に切り出し |
| 20 | 20 | `.claude/addf/plans/0020-ci-pipeline.md` | 完了 | CI パイプライン（Go テスト + ドキュメントビルド）。PR #12（2026-07-06） |
| 21 | 19 | `.claude/addf/plans/0019-eval-repl-and-dx.md` | 一部完了 | diff は PR #11 で実装済み（2026-07-06）。REPL / stats が残り |
| 22 | 23 | `.claude/addf/plans/0023-addf-hook-integration.md` | 未着手（スタブ） | 0022 依存。/addf-init での hook 自動設営 |
| 23 | 24 | `.claude/addf/plans/0024-remote-approval.md` | 未着手（スタブ） | 0022 Phase 3 依存。Slack/リモート承認連携 |
| 24 | 25 | `.claude/addf/plans/0025-loop-body-static-analysis.md` | 完了 | リテラル `for` ループの部分解析・`unanalyzable_action` 設定・ADDF 実地確認済み。2026-07-17 完了 |
| 25 | 26 | `.claude/addf/plans/0026-approve-store-hardening.md` | 未着手 | Plan 0022/0025 レビューで検出された主題外 7 項目（承認ストア HMAC・Scope 型安全化・Phase 0 再検証・git config 保護補完 等）。0022 とは独立 |
| 26 | 27 | `.claude/addf/plans/0027-v020-semantics-regression.md` | 完了 | Issue #15。v0.2.1 リリースで対応（check 警告 + default preset 拡張 + マイグレーションガイド）。2026-07-17 |
| 27 | 28 | `.claude/addf/plans/0028-ask-strategy-visibility.md` | 未着手 | Issue #16。ask_strategy / unattended: の可視化・docs 導線強化。実装は既存（Plan 0022 Phase 2）、主にドキュメント |
| 28 | 29 | `.claude/addf/plans/0029-hook-eval-log-and-stats.md` | 未着手 | Issue #17。hook 評価結果の JSONL 永続化と ccchain stats 集計。conf チューニングのループを速くする |

## 割り込み対応

| 計画ファイル | 状態 | 概要 |
|---|---|---|
| `.claude/addf/plans/0021-test-smell-fixes.md` | 完了 | savanna-smell-detector 検出の8件テストスメル修正 |

## ロードマップ（未計画）

- PostToolUse ターンカウント（max_repeat, on_exceed）
- source / . コマンドの追跡（原理的に不可能、ドキュメント明記のみ）

---

## アーカイブ

| Phase | 計画ファイル | 状態 |
|---|---|---|
