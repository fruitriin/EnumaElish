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
  scope:
    inside: <action> ["メッセージ"]
    outside: <action> ["メッセージ"]         # read/write 両方に適用
    outside-read: <action> ["メッセージ"]    # outside より優先
    outside-write: <action> ["メッセージ"]   # outside より優先
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
  workspace: <path>[, <path> ...]   # ワークスペーススコープのルート（`scope:` 参照）
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

### `scope:`

ルール単位のワークスペーススコープ制御。`settings: workspace: <paths>` の指定が前提。コマンドの引数からパスを抽出し、workspace 内外+read/write の組み合わせで別々のアクションを適用できる:

```
allow cp
  scope:
    inside: allow
    outside-read: allow          # 外部からの読み取りは OK
    outside-write: deny  "workspace 外への書き込みは禁止"

allow rm
  scope:
    inside: ask  "削除を確認"
    outside-write: deny  "workspace 外のファイル削除は禁止"

allow cat
  scope:
    inside: allow
    outside: ask  "workspace 外のファイルです"   # read/write 両方に適用
```

**優先順位**（各パス引数について）:
- パスが workspace 内 → `inside:`（設定されていれば）
- パスが workspace 外かつ書き込み引数 → `outside-write:` > `outside:` の順で採用
- パスが workspace 外かつ読み取り引数 → `outside-read:` > `outside:` の順で採用

**read/write の判定**は組み込みセマンティクステーブル（[`internal/semantics/table.go`](../../../internal/semantics/table.go)）で行う:

| コマンド | 判定 |
|---|---|
| `cat`, `head`, `tail`, `less`, `more`, `grep`, `awk`, `wc`, `file`, `stat`, `diff`, `cmp`, `md5sum`, `sha256sum`, `rg` | 全パス = read |
| `rm`, `rmdir`, `shred`, `tee`, `touch`, `mkdir`, `unlink` | 全パス = write |
| `cp`, `mv`, `ln` | 最後のパス = write、それ以外 = read |
| 上記以外 | **unknown** — `outside-write` / `outside-read` / `outside` すべてを候補として最も制限的なものを採用 |

**GNU coreutils の `-t` フラグ**: `cp -t DIR src...` / `cp --target-directory=DIR src...` / `cp -tDIR src...` を認識し、`DIR` を write、それ以外の path 引数を read として分類する。`mv` / `ln` も同様（Critical C2）。

**シェルの書き込みリダイレクト**: `>`, `>>`, `>|`, `&>`, `&>>` のリダイレクト先は、コマンド自身の引数と独立して write として扱う。`cat /ws/x > /outside/y` は `cat` の引数が read でも `outside-write` に該当する。読み取りリダイレクト（`<`, `<<`）は write として追跡しない（Critical C1）。

**未知コマンド**: セマンティクステーブルに無いコマンドの path 引数は `PathKindUnknown` になる。この場合スコープ評価は `outside-write` → `outside-read` → `outside` の順で候補を評価し、最も制限的なアクションを採用する。`sed -i` / `rsync` / `wget -O` / `dd` / `tar` などにも `outside-write: deny` が正しく発火する（Critical C3）。

**ツール呼び出し（Read / Edit / Write / MCP）**: `scope:` ブロックは Bash 以外のツールにも適用される。`Read` は read、`Write` / `Edit` / `NotebookEdit` は write、MCP ツールは unknown として分類する（Critical C6）。

**シンボリックリンクの解決**: 引数パスは `filepath.EvalSymlinks` で存在する最も長い祖先を解決してから比較する。workspace 内のリンクが外を指していれば `outside` になり、外のリンクが内を指していれば `inside` になる。ファイルシステムのルートまで遡っても解決できない場合（循環リンク / 全祖先で権限エラー）は fail-closed で `outside` として扱う（Critical C7）。

**存在しないパス**: `cp foo new-file.txt` のように新規作成予定のパスは、存在する親ディレクトリを解決してから残りの部分を結合する。workspace 内の新規ファイルは正しく `inside` と判定される。

**後方互換性**: `scope:` ブロックを持たないルールは従来通りの動作（workspace 外パス引数を検出したら `allow` → `ask` にエスカレーション）。`scope: outside: allow` を明示すると、そのコマンドではエスカレーションを行わない。

**動的引数**: `$VAR` / `$(cmd)` / `` `cmd` `` を含むパス引数は判定不能。fail-closed で `outside` として扱い、未知コマンドの場合は kind を write に昇格して `outside-write` を発火させる。`cp /ws/src $(echo /elsewhere)/dst` は `outside-write: deny` が設定されていれば deny になる（Critical C4）。

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
  workspace: ~/workspace  # ワークスペーススコープ（カンマ区切りで複数指定可）
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
