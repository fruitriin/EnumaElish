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
- [ ] `args:` パターンのクォート含む引数への対応（Security Low）
- [ ] `args:` パターン最大長制限の追加（Security Info）

## 状態: 未着手
