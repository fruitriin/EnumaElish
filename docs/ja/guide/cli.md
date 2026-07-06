# CLI コマンド

## `ccchain check`

設定ファイルの構文を検証します。

```bash
ccchain check                          # デフォルトの検索パスで設定を検証
ccchain check --config path/to/conf    # 指定ファイルを検証
ccchain check -v                       # 詳細表示（パースされたルールとテンプレート）
```

## `ccchain hook pre`

PreToolUse hook。Claude Code からツール情報 JSON を stdin で受け取り、評価結果に応じた exit code を返します。

```bash
# .claude/settings.json に登録して使用（直接呼び出しは通常しない）
echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}' | ccchain hook pre
```

| exit code | 意味 |
|---|---|
| 0 | 許可（または Bash 以外のツール） |
| 2 | 拒否（理由が stderr に出力） |

## `ccchain hook post`

PostToolUse hook。現在はパススルー（将来の hint アクション・ターンカウント用）。

## `ccchain eval "command"`

コマンドの評価結果を JSON で出力します。デバッグやスクリプト連携に便利です。

```bash
ccchain eval "find . | rm"
```

```json
{
  "action": "deny",
  "message": "don't pipe into destructive commands",
  "template": "bulkExec",
  "context": ["find", "|", "rm"]
}
```

```bash
ccchain eval "ls -la | head"
```

```json
{
  "action": "allow",
  "context": ["ls"]
}
```

## `ccchain diff <config-a> <config-b>`

2つの設定ファイルに対して同一のコマンドリストを評価し、コマンドごとに判定が変わるか（`CHANGED`）変わらないか（`same`）を報告します。ルール変更 PR のレビューや CI での回帰チェックに便利です。

```bash
# ファイルからコマンドを読む（positional または --commands。両方指定はエラー）
ccchain diff old.conf new.conf commands.txt
ccchain diff old.conf new.conf --commands commands.txt

# stdin からコマンドを読む
cat commands.txt | ccchain diff old.conf new.conf

# CI: 判定が1つでも変わったらジョブを失敗させる
ccchain diff old.conf new.conf commands.txt --changed-only --exit-on-change
```

出力例:

```
Config A: old.conf
Config B: new.conf

find . -delete  a=[allow]  b=[deny]   CHANGED
ls -la          a=[allow]  b=[allow]  same

Summary: 2 commands — changed=1, same=1, error=0
Config A: old.conf
Config B: new.conf
```

**比較対象**: Bash コマンドの評価結果のみを比較し、判定アクション（`allow`/`ask`/`deny`/`warn`）だけを見ます。メッセージの差分や、Read/Edit 等のツールルールの差分は比較対象外です。

フラグ:

| フラグ | 説明 |
|---|---|
| `--commands <file>` | コマンドをファイルから読む（1行1コマンド、`#` コメント可） |
| `--changed-only` | `same` の行を出力しない（Summary にはカウントされる） |
| `--exit-on-change` | 判定が1つでも変わったら exit 2 で終了（CI 用） |

終了コード:

| Exit Code | 意味 |
|---|---|
| 0 | 完走（変更なし、または `--exit-on-change` なしでの変更あり） |
| 1 | 使い方・設定エラー（空の config パスを含む） |
| 2 | 1つ以上の `CHANGED`（`--exit-on-change` 指定時のみ） |
| 3 | 1つ以上のコマンドが評価エラー（2 より優先） |

補足:

- `diff` はグローバルの `--config` フラグを使いません。2つの設定は positional 引数で渡します。`--config` を渡すと明示的にエラーになります。
- コマンド文字列は制御文字をエスケープして表示します（`\x1b` 等）。信頼できない commands ファイルがターミナルエスケープで行を隠蔽することを防ぎます。

## `ccchain audit`

全ルールのフラット展開を表示します。「何が通って何が止まるか」を一覧で確認できます。

```bash
ccchain audit
ccchain audit --config path/to/conf
```

出力例:
```
[allow]  ls
[allow]  ls | cat            (template: primitive)
[allow]  find
[deny]   find | rm           (template: bulkExec)  "don't pipe into destructive"
[deny]   find -exec rm       (template: bulkExec.exec)  "expand to tempfile first"
[---]    find && ...         (&&: reset → top-level rules)

Settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask

Stats:
  rules: 8
  templates: 3
```

## `ccchain suggest`

コマンドを分析し、`ask` フォールバックに該当するものに対してルール追加を提案します。どのコマンドに明示的なルールが必要かを発見するのに便利です。

```bash
# コマンド引数から
ccchain suggest "ls -la" "cat foo.txt" "rm -rf /"

# ファイルから（1行1コマンド）
cat commands.txt | ccchain suggest
```

## `ccchain init`

デフォルトの `.ccchain.conf` を生成します。既存ファイルがある場合は上書きしません。

```bash
ccchain init
```

生成後の次のステップも表示されます:
1. `.ccchain.conf` を確認・カスタマイズ
2. `.claude/settings.json` に Hook を登録
3. `ccchain check` で検証
4. `ccchain audit` で展開確認

## 共通フラグ

| フラグ | 説明 |
|---|---|
| `--config <path>` | 設定ファイルパスを明示指定（検索をスキップ） |
| `--default-action <action>` | 未マッチコマンドのフォールバックアクションを上書き（`allow`, `deny`, `ask`） |
| `-v, --verbose` | 詳細出力 |
| `-q, --quiet` | エラーのみ出力 |
| `--version` | バージョン表示 |
| `-h, --help` | ヘルプ表示 |
