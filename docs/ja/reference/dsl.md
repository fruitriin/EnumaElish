# DSL 構文リファレンス

ccchain はインデントベースのテキスト DSL を使用します。

## 文法

```
# コメント（# で始まる行）

# トップレベルルール
<action> <command>[, command2, ...] ["メッセージ"]
  # コンテキスト修飾子（インデントで子要素）
  |,>>
    <action> <command>[, command2, ...] ["メッセージ"]
  exec:
    <action> <command>[, command2, ...] ["メッセージ"]
  args:
    <パターン>: <action>
  # プロパティ
  mode: block | warn | hint
  message: "..."
  next: <テンプレート名>

# テンプレート定義
template <名前>
  extends: <親テンプレート>
  # ルールと同じ構造

# Hook セクション
preToolUse
  # PreToolUse 用ルール群
postToolUse
  # PostToolUse 用ルール群

# 設定
settings:
  max_context_depth: <整数>
  max_rules_per_cmd: <整数>
  fallback: <action>
```

## アクション

| アクション | 意味 |
|---|---|
| `allow` | コマンドを許可 |
| `deny` | コマンドをブロック（exit 2 + メッセージを Claude に通知） |
| `warn` | 許可するが Claude に警告を送信 |
| `ask` | Claude Code の標準パーミッションダイアログに委譲 |
| `hint` | PostToolUse: 次のアクションを Claude に提案 |

## コンテキスト修飾子

### `|,>>`

パイプ先またはリダイレクト先として現れるコマンドに適用するルール:

```
allow find
  |,>>
    allow grep, sort
    deny rm  "find と rm をパイプで繋がないでください"
```

`|` のみ（パイプ限定）、`>>` のみ（リダイレクト限定）も指定可能。

### `exec:`

`-exec`、`xargs`、`bash -c` 等でネストされたコマンドに適用するルール:

```
allow find
  exec:
    deny rm  "一時ファイルに展開してください"
    allow cp, mv
```

### `args:`

コマンド引数に対するパターンベースのルール（正規表現）:

```
allow curl
  args:
    -X GET: allow
    -X POST: ask
```

> **注意:** `args:` ルールは現在パースされますが、**評価エンジンでは未実装**です。将来のリリースで対応予定。セキュリティ上の判断に `args:` ルールを使用しないでください。

## テンプレート

### 定義

```
template <名前>
  |,>>
    <ルール群>
  exec:
    <ルール群>
```

### 継承

```
template child
  extends: parent    # parent の全ルールを継承
  |,>>
    allow extra-cmd  # 追加ルール
```

### 委譲

```
allow find
  next: bulkExec    # bulkExec のパイプ/exec ルールを使用
```

## 設定

```
settings:
  max_context_depth: 2    # audit 展開の最大深度
  max_rules_per_cmd: 5    # audit でのコマンドあたりルール数上限
  fallback: ask           # マッチしないコマンドのデフォルト動作
```

## 複数コマンドの一行記法

カンマ区切りで同じルールを共有:

```
allow cat, echo, head, tail, wc
```

## メッセージ

コマンドの後にクォートされた文字列で deny/warn メッセージを指定:

```
deny rm  "trash コマンドを使ってください"
deny eval  "eval は静的解析できません"
```

## インデント

- スペース（2 または 4）またはタブを使用
- タブは 4 スペースとして扱う
- ブロック内のインデントは統一すること
- 深いインデント = 上の行の子要素
