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
  scope_violation: ask | deny         # ワークスペース外パスを検出したときの動作（デフォルト ask）
  strict_config_error: true | false   # 設定ロード失敗時に deny (デフォルト false = fail-open)
```

**シェルクォートに関する注記:** コマンド名・引数どちらのマッチングも、シェルがコマンド実行前に行うのと同じ **クォート除去後の文字列** に対して行われる。`"rm"` は `deny rm` にマッチし、`curl -X "POST"` は args: パターン `-X POST` にマッチし、`rm "-rf" /` は args: の `-rf` にマッチする。除去されるのは静的な `'...'` / `"..."` の外側ラップのみで、ダブルクォート内のバックスラッシュエスケープ（`\"`, `\$`, `\\`, `` \` `` 等）や ANSI-C `$'...'` はソース表記のまま渡される。エスケープ亜種に対しても防御したいパターンは明示的にカバーすること。

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
- シェルのクォートは、シェルがコマンド実行前に除去するのと同様、マッチ前に除去される: `curl -X "POST"` はパターン `-X POST` にマッチする。同じクォート除去はコマンド名マッチングにも適用される（文法セクションのシェルクォート注記を参照）。除去されるのは静的な `'...'` / `"..."` のラップのみで、ダブルクォート内のバックスラッシュエスケープ（`\!`, `foo\ bar`, `\"`, `\$`, `` \` ``）や ANSI-C `$'...'` はソース表記のまま渡される
- 結合した引数文字列が **4096 バイト**を超える場合、args: ルールは適用されず、結果は **その args: ブロックに現れるうち最も厳格なアクション**（下限は `ask`）にエスカレーションされる（親アクションが `deny` 等より厳格な場合はそちらを維持）。具体的には: `deny` を含むブロックは超過時 deny、`ask` 止まり／`allow` のみのブロックは `ask` になる。親アクションへのフォールバックにすると、引数のパディングでエスカレーション系 args: ルール（例: `allow rm` + `args: -rf /: deny`）をバイパスできてしまうため

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
  max_context_depth: 2         # audit 展開の最大深度
  max_rules_per_cmd: 5         # audit でのコマンドあたりルール数上限
  fallback: ask                # マッチしないコマンドのデフォルト動作
  workspace: ~/workspace       # ワークスペーススコープ（カンマ区切りで複数指定可）
  scope_violation: ask         # ワークスペース外パス検出時のアクション（ask|deny）
  strict_config_error: true    # 設定ロード失敗時に fail-closed（deny）にする。デフォルト: false
```

### `scope_violation`

本来 `allow` になるコマンド・ツール呼び出しが `workspace` スコープ外のパスを
参照していた場合の挙動を制御します:

- `ask`（デフォルト）: `allow` → `ask` に降格。パーミッションダイアログで
  承認すればワークスペース外へのアクセスも可能
- `deny`: `allow` → `deny` に降格。ワークスペース外へのアクセスを完全に
  ブロックする。パーミッションダイアログが人間に届かない構成
  （headless / 自動承認モード等）での厳格運用向け

降格の対象は `allow` 結果のみで、明示的な `ask` / `deny` ルールは変更されません。
`ask` / `deny` 以外の値はパースエラーになります。

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

**警告 — self-DoS 障害モード。** `strict_config_error` が有効で、**かつ hook が
全ツール（または Bash に加えて Read/Edit/Write）にマッチする登録**になっている
場合、パースに失敗する `.ccchain.conf` を保存すると **すべての** PreToolUse
フック呼び出しが exit 2 になります。Read や Edit も塞がれるため、Claude Code
から壊れた設定ファイルを開いて直すことができなくなります。

**Bash-only の脱出ハッチ。** hook の登録（matcher）が `"Bash"` のみの場合、
Read/Edit は ccchain を経由しないため、設定が壊れた状態でもそのまま使えます。
この場合、外部ターミナルは**不要**です — Claude Code の Edit ツールで設定
ファイルを直接修正し、Bash が通るようになったら `ccchain check --config <path>`
で検証してください。下の番号付き復旧手順が必要になるのは、hook が全ツールに
配線されている難しいケースだけです。

**予防（推奨）:** 設定ファイルを変更して保存する前に必ず
`ccchain check --config <path>` を実行してください。`.ccchain.conf` に触れる
コミットには CI でも同コマンドを回すこと。

**警告 — `ccchain check` には `--config` が必須。** 位置引数で渡したパスは
**黙って無視**されます。その場合 `check` はデフォルト探索パス
（[config.md](./config.md) 冒頭の表を参照）の方を検証してしまい、テストしたい
ファイルが壊れたままでも `config OK` と報告することがあります。

```sh
ccchain check broken.conf            # 誤り: 位置引数は無視される。
                                     # デフォルト探索パスを検証し「config OK」が出うる
ccchain check --config broken.conf   # 正しい: broken.conf を検証する
```

**すでにロックアウトされた場合の復旧手順:**

0. strict モードの由来（環境変数か、設定ファイルか、その両方か）を診断する。
   Claude Code の外のシェルで:

   ```sh
   env | grep CCCHAIN_STRICT_CONFIG_ERROR
   grep -l strict_config_error .ccchain.conf .ccchain.local.conf \
     "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/ccchain.conf" 2>/dev/null
   ```

   1つ目のコマンドで手順 1 が必要かどうかが分かり、2つ目で
   `strict_config_error` を設定しているファイル（手順 2 の対象）が分かります。
1. 環境変数由来の場合: Claude Code の外のシェルで
   `unset CCCHAIN_STRICT_CONFIG_ERROR`（または `false` を設定）する。新しい
   シェルで unset しても、起動済みの Claude Code プロセスには伝播しません —
   Unix の環境変数は `fork(2)` 時にコピーされ、兄弟プロセス間でライブ共有され
   ないためです。変更を反映するには Claude Code の再起動（手順 3）が必要です。
   さらに、再起動は**変数が実際に unset された環境から**行うこと: シェル
   プロファイル（`~/.zshrc` や `~/.config/fish/config.fish` 等）で export
   している場合、再起動後の Claude Code も変数を引き継ぎます。先に
   プロファイルから export を外してください。strict モードが設定ファイル
   由来のみの場合は手順 2 へ。
2. Claude Code の外の通常のターミナル（ccchain フックを経由しない）を開き、
   壊れた設定ファイルを直接編集してパースエラーを修正する（Bash-only 登録の
   場合は Claude Code から編集してもよい — 上記の脱出ハッチ参照）。

   **最終手段として**、壊れたファイルを一時的にリネームして探索パスから外す
   こともできますが、以下の 2 点に注意:

   - リネームしても汎用の組み込みルールセットにフォールバックする訳では
     **ありません**。探索は探索パスの表（[config.md](./config.md) 参照）の
     続きに進むだけで、グローバル設定（`$CLAUDE_CONFIG_DIR/ccchain.conf`
     または `~/.claude/ccchain.conf`）が存在すれば、そのルールに**黙って
     切り替わり**ます — グローバル側に `fallback: deny` があれば `echo hi`
     すら deny になり、逆に緩いグローバル設定ならプロジェクト固有の
     deny/ask/scope: ルールがエラーログもなしに全て消えます。どこにも設定
     ファイルが残っていなければ、ルールゼロで評価され、全コマンドが組み込み
     デフォルトの `fallback: ask` に落ちます。したがってリネームが妥当なのは、
     hook がデフォルト探索パスで起動されていて、**かつ**グローバル設定が
     無いか内容を正確に把握している場合だけです。修正が終わったら直ちに元の
     ファイル名に戻し、`ccchain check --config <path>` で再検証してください —
     リネームしたまま放置しないこと。
   - hook が明示的な `--config <path>`（例: `ccchain hook pre --config
     /path/to/rules.conf`）で登録されている場合、リネームは復旧になりません:
     ロードは "file not found" という別のエラーで失敗し続け、strict モードが
     環境変数由来なら deny も継続します（strict がそのファイル自身由来なら、
     今度はルールゼロの fail-open に黙って落ちます）。`--config` 起動時は、
     その場でファイル内容を直すことが唯一の復旧手段です。
3. 手順 1 で環境変数を変更した場合は Claude Code を再起動する（新しい環境を
   反映する唯一の方法。手順 1 のプロファイルの注意も参照）。手順 2 でファイル
   を直しただけなら再起動は不要 — `dsl.LoadConfig` はフック呼び出しごとに
   実行されるため、次のツール呼び出しで修正済み設定が自動的に再ロードされ、
   ワークスペースのブロックが解除されます。

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
