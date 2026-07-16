# Plan 0016: deny-redirect アクション

## 背景

Dippy の `deny-redirect` アクション: 特定パスへのアクセスを**拒否ではなく別の場所にリダイレクト**する。

```
deny-redirect **/.env* "Never write secrets, ask me to do it"
```

ccchain では deny メッセージで代替を誘導するが、「このパスの代わりにこのパスを使え」という構造的なリダイレクト指示は未実装。

## ユースケース

```
# .env ファイルの直接編集を禁止し、.env.example の編集を誘導
redirect .env
  to: .env.example
  message: ".env を直接編集せず .env.example を更新してください"

# node_modules 内の直接編集を禁止
redirect node_modules/
  action: deny
  message: "node_modules 内の直接編集は禁止。package.json を変更してください"

# ビルド成果物の直接編集を禁止
redirect dist/, build/, out/
  action: deny
  message: "ビルド成果物は直接編集せず、ソースコードを変更してください"
```

## 設計

### DSL 構文

```
redirect <path-pattern>
  to: <alternative-path>    # オプション
  message: "理由と代替案"
  action: deny              # deny（デフォルト）or ask
```

### 実装

Read/Edit/Write ツールの入力パスに対してパターンマッチし、deny + 代替パスの提案を返す。
Plan 0014（マルチツール制御）の Read/Edit 制御と自然に統合できる。

### Bash コマンドへの適用

Bash の引数にパスが含まれる場合も redirect を適用:

```bash
cat .env              # → deny: ".env.example を参照してください"
vim node_modules/x.js # → deny: "package.json を変更してください"
```

## 実装量: 小（パスパターンマッチ + メッセージ生成）
## 依存: Plan 0014（マルチツール制御）
