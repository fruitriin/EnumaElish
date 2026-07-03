# Plan 0009: mode プロパティ設計と warn/hint ドキュメント修正

## 背景

ドキュメントレビュー（2026-03-28）で以下の不整合が発見された:

1. `mode:` プロパティは DSL でパースされ `Rule.Mode` フィールドに格納されるが、`evaluate.go` では一切参照されない
2. `actions.md` の warn 例が `allow curl` + `mode: warn` と記載されているが、実際には `allow` として評価される
3. `hint` 例も同様に `allow WebFetch` + `mode: hint` で、`mode` は無視される

## 問題の本質

`mode:` は DSL の文法に存在するが概念に実装が追いついていない。2つの方向性がある:

### 選択肢 A: mode を廃止し、アクション自体で表現する

```
warn curl  "WebFetch の使用を検討してください"
```

- `warn` をトップレベルアクションとして使う（`allow`/`deny`/`ask` と同列）
- `mode:` プロパティは deprecated → 削除
- シンプルで一貫性がある

### 選択肢 B: mode を評価エンジンに実装する

```
allow curl
  mode: warn
  message: "WebFetch の使用を検討してください"
```

- `allow` でマッチした後、`mode: warn` で出力形式を変更
- 「許可するが警告を出す」というセマンティクスを明示的に表現
- 複雑だが表現力が高い

## 推奨: 選択肢 A

現在 `warn curl "msg"` で十分に同じ動作を実現でき、`mode:` の付加価値が不明確。

## タスク

### Phase 1: ドキュメント修正（即座）
- [ ] `docs/reference/actions.md` (EN/JA): warn の例を `warn curl "..."` に修正
- [ ] `docs/reference/actions.md` (EN/JA): hint の例はそのまま（PostToolUse 未実装の注記あり）
- [ ] `docs/reference/dsl.md` (EN/JA): `mode:` プロパティに「パースされるが現在評価に影響しない」旨を注記

### Phase 2: 設計判断（オーナー決定待ち）
- [ ] 選択肢 A or B を決定
- [ ] A の場合: `Rule.Mode` フィールドを deprecated 化、パーサーで warning を出力
- [ ] B の場合: `evaluate.go` に mode 対応を実装

### Phase 3: 関連整理
- [x] `args:` パターンのクォート含む引数への対応（Security Low）
- [x] `args:` パターン最大長制限の追加（Security Info）

## 実装状況: 完了（2026-03-28）

選択肢 A を採用: mode: を deprecated 化
- パーサーで mode: 使用時に stderr 警告出力
- ドキュメント（EN/JA）に deprecated 注記追加
- warn は既にトップレベルアクションとして動作済み

## Phase 3 実装状況: 完了（2026-07-04, speculative/args-hardening）

- **クォート対応（Security Low）**: 検証の結果、`wordToString` が `syntax.Printer` でソース表記のまま出力しておりクォートは剥がれて**いなかった**（`curl -X "POST"` は `-X POST` に不マッチ。さらに `"rm"` がコマンド名ルールを回避できた）。`wordToString` にシェル同様の静的クォート除去を実装（`SglQuoted`/`DblQuoted` のみ。動的パート `$VAR` 等は表記のまま維持し動的判定を壊さない）。単体・統合テストで保証を固定
- **最大長制限（Security Info）**: `maxArgsLen = 4096` を導入。超過時は親アクションへのフォールバックではなく **ask へエスカレーション**（親が ask/deny ならそちらを維持）。パディングによるエスカレーション系 args: ルールのバイパスを防ぐ。fail-open は ccchain 自身のエラーに限る方針（structural-context.md）と整合。`EvaluateTool` の args: マッチにも同制限を適用
