# Plan 0028: ask_strategy 可視化とドキュメント導線強化（Issue #16）

## 実装状況: 未着手

## 背景

Issue [#16](https://github.com/fruitriin/EnumaElish/issues/16)（オーナー ADDF ドッグフーディング報告、2026-07-17 05:26）:

- v0.2.0 の auto モードで `git push` 等の `ask` が「即ブロック」になり、自律エージェント運用
  （cron ループ・/goal 自走）で ask の温度感（「人が近くにいるときは確認・いないときは通す/止める」）
  が使い分けられない、との要望
- **提案された `ask_fallback: deny|allow|warn`（ルール単位上書き含む）は、実は v0.2.0 で
  ほぼ実装済み** — `settings: ask_strategy: degrade|passthrough|deny-all` + `ask_degrade_default: deny|allow`
  + ルール子ブロック `unattended: deny|allow`
- ただしオーナーは提案時点でこの機能に気づいていない → **v0.2.0 のドキュメント導線が弱く、
  実装済みの機能が実運用者に届いていない**

## スコープ

1. **命名の可視化**: `ask_strategy` / `ask_degrade_default` / `unattended:` を README のクイックスタート、
   `docs/reference/dsl.md`、`docs/reference/actions.md` の該当箇所により目立つ形で追記する
2. **典型的な運用ケースを例示**: Issue #16 の「`git push` は非対話でも warn で通し、
   `git reset --hard` は deny のまま」レベルの conf 例を README / docs に載せる
3. **`ccchain check --verbose` に settings ダイジェスト表示**: 現在の check は
   `config OK: N templates, N rules` のみ。verbose 時に `ask_strategy: degrade`
   `ask_degrade_default: deny` などの現在有効な設定を1画面にまとめて出す
4. **降格 hint メッセージに関連ドキュメントへのリンク**: 降格 deny の hint 末尾に
   `see docs: .../ask-strategy` 相当の追記（一度案内が届けば以降は覚える）
5. **`warn` 方向の追加**: `unattended: warn` を導入すべきかの検討（オーナー提案どおり）。
   実質は `allow+reason` の既存降格挙動と同じだが、命名が実運用者に伝わりやすいかもしれない
6. **複合コマンド deny の README 注記**: Issue #16 補足の「`A && B && git push` は全体が
   実行前評価で止まる」を README hook 節に明記（利用者が「push だけ分離する」運用に早く辿り着ける）

## スコープ外

- 実装本体の変更（機能はほぼ揃っており本 Plan は主に可視化）
- Issue #16 の「新命名 `ask_fallback`」の採用（既に `ask_strategy` / `unattended:` で
  意味論は網羅されており、命名変更は破壊的変更のため見送り。ドキュメントで**意図と対応**を伝える）

## 設計原則

- 実装済みの機能をユーザーに **発見してもらう** ことがゴール。追加コードは最小
- ドキュメント導線は「困った実運用者が README の目に入る場所から2ホップ以内で辿り着ける」ように

## Phase 分割

- **Phase 1**: README（ja/en）に auto モード運用の章を追加（`ask_strategy` / `unattended:` の
  典型 conf、複合コマンド deny の注記）
- **Phase 2**: `ccchain check --verbose` に settings ダイジェスト表示
- **Phase 3**: 降格 hint メッセージにドキュメント URL 追記（Plan 0022 の hint 設計を尊重して、
  URL のみ、コマンドやトークンは載せない）
- **Phase 4**（要検討）: `unattended: warn` の追加

## テスト戦略

- Phase 1: `docs:build` 通過
- Phase 2: `check_test.go` に verbose 出力テスト追加
- Phase 3: hook 出力に URL が含まれるかの単体テスト

## 参照

- Issue: https://github.com/fruitriin/EnumaElish/issues/16
- 実装済み参照: Plan 0022 Phase 2（`internal/eval/degrade.go`）
