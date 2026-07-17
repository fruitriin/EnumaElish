# Plan 0027: v0.2.0 セマンティクス回帰の切り分けと修正（Issue #15）

## 実装状況: 未着手

## 背景

Issue [#15](https://github.com/fruitriin/EnumaElish/issues/15)（オーナー ADDF ドッグフーディング報告、2026-07-17 05:25）:

- 旧 dev ビルド（2026-07-14 頃 main）用に調整した `.ccchain.conf`（3 templates / 31 rules）をそのまま `ccchain@v0.2.0` に持ち込むと、`sed -n '60,80p' file` のような**基本コマンドが `no matching rule (fallback)`** に落ちる
- `ccchain check` は **`config OK: 3 templates, 31 rules`** を返し、構文互換は通っている
- fallback: `ask` + Claude Code auto permission mode の組み合わせで **hook 経由の Bash が全ブロック**、旧バイナリロールバックで復旧
- **check 通過 ≠ セマンティクス互換** というギャップが v0.2.0 リリースサイクルで踏まれた

これはリリース回帰であり、v0.2.0 の直後緊急パッチ（v0.2.1）の候補。

## 想定される原因（切り分け対象）

Phase 4（sentinel プリセット）・Plan 0025（for ループ + `unanalyzable_action`）・
Stage 2 レビュー修正（canonical hash に `\x1f` argSep、`applyArgsRules` の `Unattended` 伝播、
コマンド名先頭 `\` エスケープ剥がし、`bodyRedefinesVar` へのビルトイン追加、
sentinel/default preset の `.ccchain.conf` Write/Edit deny 追加、
`emitResponse` の exit 2 stderr 追記、`Store.SetDefaultDirForTest` 化、
ロック stale reap 等）で導入され得た影響を、以下の順で切り分ける:

1. **`ResolveTemplates`**: 旧 conf の `template primitive` / `template safeRead` / `template bulkExec` 等
   の解決経路が変わっていないか（`internal/dsl/template.go` の変更、`extends:` チェーン）
2. **template `next:`**: `next: primitive` などによるルール連鎖が Phase 1〜4 のどこかで壊れていないか
   （`internal/dsl/lookup.go`）
3. **`matchCommand` / `matchInPipeContext` / `matchInExecContext`**: Rule 構造体に `Unattended` フィールドを
   追加した影響で構造体コピーやマッチ判定が変わっていないか
4. **`parseRule` / `parseTemplate`**: `unattended:` キーワード追加により、ルール block 内の
   親子関係パースがずれていないか（インデント処理・child block の consumer 順序）
5. **`applyArgsRules` の Unattended 伝播修正**: baseResult のコピー先変更で args: サブルール
   マッチ後の Result 生成が壊れていないか
6. **`bodyRedefinesVar` の false positive**: for ループボディに read/mapfile/declare 等が
   出現するとフォールバックに落とすが、これがコマンド名マッチ経路に影響していないか
   （通常は `for` ループの外側なので無関係のはずだが要確認）
7. **canonical hash の `\x1f` 変更**: これは承認ストア専用の変更であり、通常マッチには影響しないはず。
   念のためコード grep で影響範囲を確認
8. **default preset (`init_cmd.go`)** と **sentinel preset (`internal/preset/sentinel.conf`)** の
   `.ccchain.conf` 保護追加が、旧 conf ロード時に何らかの副作用を持たないか（通常は無関係だが、
   testdata や統合テストへの影響を含めて grep する）
9. **`emitResponse` の deny 時 exit 2 stderr 追加**: allow/ask 経路は無変更のはずなので、
   fallback ask 経路自体には影響ない見立てだが、実機での hook 挙動を1度確認する
10. **fallback の再解釈**: `EvaluateTopology` の fallback 生成に workspace scope 適用等の
    副作用が入ったのが Phase 3/4 なので、旧 conf に workspace: がある場合の副作用を確認

## 再現手順

1. ADDF 側 `/Users/riin/workspace/AutomatonDevDriveFramework/.ccchain.conf`（実運用 conf）を借用
2. v0.2.0 バイナリで `ccchain check --config /path/to/conf -v` → 「config OK: 3 templates, N rules」を確認
3. `echo '{"tool_name":"Bash","tool_input":{"command":"sed -n \"60,80p\" foo.txt"},"permission_mode":"default"}' | ccchain hook pre --config /path/to/conf`
   → 実際に fallback に落ちるか、それとも allow でヒットするかを確認
4. 落ちる場合は `ccchain audit --config /path/to/conf` で expansion を出し、`sed` に到達するルールが実在するかを確認
5. 実在するが到達不能なら、`ccchain eval "sed -n '60,80p' foo.txt" --config /path/to/conf` で
   デバッグ（audit と eval の解釈差を洗う）

## スコープ

1. **原因特定**（診断）: 上記10候補から実測で1〜複数を絞り込む
2. **修正**: 見つかった原因に応じてピンポイント修正（Plan 0022/0025 由来の後退なら本 Plan 内で決着、
   旧仕様前提の別問題なら別 Plan に切り出す）
3. **`ccchain check` の警告強化**: 「セマンティクス変更でルールが実質無効化されている可能性」を
   検出できる範囲で警告を出す（Plan 記載の希望項目2）。実装コストと精度に応じて Phase 分割可
4. **CHANGELOG / README にマイグレーション注記**: v0.2.0 で意味論が変わった箇所（もしあれば）を
   明記し、旧 conf からの書き換え指針を添える

**スコープ外**:
- `ccchain check` の**完全な**セマンティクス静的解析（テスト時にしか判らないケースもある）
- `ccchain test` の網羅性拡張（このカテゴリの回帰を防ぐには test 側の充実が必要だが、別 Plan 化）

## 設計原則

- 修正は**最小変更**。v0.2.0 で意図した挙動（deny-first 強化）は維持する
- 旧 conf の後方互換を優先しつつ、意図された変更（例: hook 出力形式）は変えない
- `ccchain check` 警告は false positive を避ける（過検出で狼少年化しない）

## Phase 分割

- **Phase 0 (診断ゲート)**: 上記の再現手順を実施し、原因を最大2件までに絞り込む
- **Phase 1 (修正)**: 特定された原因を修正、ADDF 実 conf で回帰確認
- **Phase 2 (check 警告強化)**: `ccchain check` に「到達不能ルール」検出などの警告を追加
- **Phase 3 (パッチリリース)**: v0.2.1 として CHANGELOG 追記、タグ・バイナリ・GitHub Release

## テスト戦略

- ADDF 実 conf を `testdata/eval/` にコピー（ADDF 側の秘密が入っていないか確認のうえ）し、
  代表的な「旧 conf で allow → v0.2.0 で fallback」ケースを回帰テストとして固定
- fixture テスト（`fixture-based-testing.md`）の既存パターンに沿って rules-oldstyle.conf を追加

## 参照

- Issue: https://github.com/fruitriin/EnumaElish/issues/15
- 前提リリース: v0.2.0（2026-07-17）
- 関連 knowhow: `plan-0022-0025-lessons.md`（Phase 4 sentinel と default preset の差分）
