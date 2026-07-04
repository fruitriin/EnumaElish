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
  mode: block | warn | hint  # 非推奨: パースされるが評価に影響しない。warn/hint アクションを直接使用
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
  strict_config_error: true | false   # 設定ロード失敗時に deny (デフォルト false = fail-open)
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

パターンは Go の正規表現で、**引数を結合した文字列**（`strings.Join(args, " ")`）に対してマッチします。

**注意事項:**
- デフォルトは**部分マッチ**。完全一致には `^` と `$` アンカーを使用
- 引数に動的展開（`$VAR`, `` `cmd` ``）が含まれる場合、args: 評価はスキップされ親ルールのアクションが使用される
- 複数の args: パターンがマッチした場合は last-rule-wins
- args: ルールがマッチすると親ルールのアクションを上書きする

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
  max_context_depth: 2         # audit 展開の最大深度
  max_rules_per_cmd: 5         # audit でのコマンドあたりルール数上限
  fallback: ask                # マッチしないコマンドのデフォルト動作
  workspace: ~/workspace       # ワークスペーススコープ（カンマ区切りで複数指定可）
  strict_config_error: true    # 設定ロード失敗時に fail-closed（deny）にする。デフォルト: false
```

### `strict_config_error`

デフォルトでは ccchain は fail-open — 設定ロードの失敗（ファイル欠損、
パースエラー、テンプレート未解決など）は stderr にログ出力しつつコマンドを
**許可**します（[エラー処理（Fail-Open）](./config.md#エラー処理fail-open) 参照）。

`strict_config_error: true` を **すでに読み込めた設定ファイル**（例:
グローバルの `~/.claude/ccchain.conf`）で有効にすると、後続の設定ファイルが
ロードに失敗したときに PreToolUse フックが exit 2 で deny します。

設定ファイルが1つも読めなかった場合は、環境変数
`CCCHAIN_STRICT_CONFIG_ERROR=1`（または `true`）だけが strict モードへの
オプトイン手段になります。

fail-open が許容できない unattended 運用・高セキュリティ環境で使用してください。
デプロイ前の CI で `ccchain check` と組み合わせると効果的です。

**⚠️ 復旧手順（self-DoS への注意）:** strict モードが有効で、かつ設定ファイルが
壊れている状態では、**すべての** PreToolUse フック呼び出しが exit 2 になります。
Bash だけでなく Read/Edit/Write も塞がれるため、Claude 単体では壊れた設定を
直せなくなります。復旧には Claude Code の外側からのシェル操作が必要です。

1. 事前予防を最優先: strict モード有効化前に `ccchain check` を実行し、
   設定変更時は CI でも `ccchain check` を回す。
2. ロックアウトされた場合の復旧手段:
   - `CCCHAIN_STRICT_CONFIG_ERROR` で有効化していた場合: シェルで `unset` するか、
     この環境変数を外して Claude Code を再起動する。
   - 設定ファイルで有効化していた場合: 通常のターミナル（ccchain フックを
     経由しない）で壊れた設定ファイルを直接編集するか、壊れたファイルを
     一時的にリネームして探索パスの stat で見つからない状態にする。

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
