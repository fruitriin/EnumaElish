# テンプレート

テンプレートで再利用可能なルールセットを定義し、複数のコマンドで共有できます。

## テンプレートの定義

```
template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq
```

これで `primitive` テンプレートにパイプコンテキストのルールが定義されました。

## `next:` による委譲

`next:` でコマンドにテンプレートを割り当てます:

```
allow ls
  next: primitive

allow find
  next: primitive
```

これで `ls | cat` も `find | cat` も primitive テンプレートのルールで評価されます。

## `extends:` による継承

テンプレートは他のテンプレートを継承できます:

```
template primitive
  |,>>
    allow cat, echo, head, tail, wc

template safeRead
  next: primitive
  |,>>
    allow grep, awk, sed

template bulkExec
  extends: safeRead
  |,>>
    deny rm  "破壊的コマンドにパイプしないでください"
  exec:
    deny rm  "一時ファイルに展開してください"
    allow cp, mv, touch
```

### 継承チェーン

```
primitive ← safeRead ← bulkExec
```

`bulkExec` を使うコマンド:
```
allow find
  next: bulkExec
```

パイプコンテキストで得られるルール:
1. `allow cat, echo, head, tail, wc` (primitive から)
2. `allow grep, awk, sed` (safeRead から)
3. `deny rm` (bulkExec 自身)

exec コンテキストで得られるルール:
1. `deny rm`, `allow cp, mv, touch` (bulkExec 自身)

## last-rule-wins とテンプレート

テンプレートのルールはコマンド自身のルールより先に評価されます。そのためコマンド側でテンプレートのルールを上書きできます:

```
allow grep
  next: bulkExec
  |,>>
    allow rm  # bulkExec の deny rm を上書き
```

`grep | rm` → bulkExec の `deny rm` が先にマッチ → コマンド自身の `allow rm` が後にマッチ → **last-rule-wins で allow**

## 循環参照の検出

`extends:` の循環参照はパース時にエラーになります:

```
template a
  extends: b

template b
  extends: a
# → error: circular extends detected: [a] -> b
```

## テンプレート設計のベストプラクティス

1. **安全な出力先を `primitive` にまとめる** — `cat`, `head`, `wc` 等の読み取り専用コマンド
2. **処理系を `safeRead` にまとめる** — `grep`, `awk`, `sed` 等のフィルター
3. **破壊的操作の保護を `bulkExec` にまとめる** — `rm` の deny、`-exec` の制御
4. **プロジェクト固有のテンプレートは別名で** — `myProjectSafe` 等
