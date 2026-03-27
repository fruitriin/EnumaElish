# TODO

`docs/plans/` の完了状態・優先度をトラックする。
`docs/plans/` と TODO が一致しなければ TODO を編集する。

## 現在のフェーズ: Phase 9（mode プロパティ + ドキュメント正確性）

## バックログ

セキュリティレビュー推奨順序で並べ替え済み。

| 優先度 | Phase | 計画ファイル | 状態 | セキュリティ根拠 |
|---|---|---|---|---|
| 1 | 1 | `docs/plans/0001-bpt-dsl-design-and-parser.md` | 完了 | |
| 2 | 2 | `docs/plans/0002-bpt-shell-ast-analysis.md` | 完了 | |
| 3 | 3 | `docs/plans/0003-bpt-evaluation-and-hook-integration.md` | 完了 | |
| 4 | 4 | `docs/plans/0004-bpt-audit-and-addf-integration.md` | 完了 | |
| 5 | 5 | `docs/plans/0005-security-hardening.md` | 完了 | |
| 6 | 6 | `docs/plans/0006-code-quality-refactoring.md` | 完了 | |
| 7 | 7 | `docs/plans/0007-documentation-and-polish.md` | 完了 | |
| 8 | 8 | `docs/plans/0008-args-rule-evaluation.md` | 完了 | |
| 9 | 9 | `docs/plans/0009-mode-property-and-doc-accuracy.md` | 完了 | mode: 誤動作を早期修正、ドキュメント誤誘導排除 |
| 10 | 14 | `docs/plans/0014-multi-tool-control.md` | 完了 | Read/Edit の hook 枠組みを先に作る（0011 の前提） |
| 11 | 11 | `docs/plans/0011-workspace-scope-access-control.md` | 完了 | 0014 があれば Bash + Read/Edit 両方にスコープ適用可 |
| 12 | 16 | `docs/plans/0016-deny-redirect.md` | 未着手 | 0014 依存。マルチツール制御後に実装 |
| 13 | 10 | `docs/plans/0010-settings-compat-and-ruleset-enhancement.md` | 未着手 | 0011 完成後に安全なデフォルトを設計できる |
| 14 | 13 | `docs/plans/0013-command-semantics-table.md` | 未着手 | 0010 のデフォルトルールと統合 |
| 15 | 15 | `docs/plans/0015-project-auto-detect.md` | 未着手 | 0013 のテーブルを活用できる |
| 16 | 12 | `docs/plans/0012-deny-message-templates.md` | 未着手 | 基盤が固まってから最後に実装 |

## ロードマップ（未計画）

- PostToolUse ターンカウント（max_repeat, on_exceed）
- source / . コマンドの追跡（原理的に不可能、ドキュメント明記のみ）

---

## アーカイブ

| Phase | 計画ファイル | 状態 |
|---|---|---|
