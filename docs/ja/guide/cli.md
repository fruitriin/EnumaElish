# CLI コマンド

## `ccchain check`

設定ファイルの構文を検証します。

```bash
ccchain check                          # デフォルトの検索パスで設定を検証
ccchain check --config path/to/conf    # 指定ファイルを検証
ccchain check -v                       # 詳細表示（パースされたルールとテンプレート）
```

## `ccchain hook pre`

PreToolUse hook。Claude Code からツール情報 JSON を stdin で受け取り、`hookSpecificOutput` JSON を stdout に出力します。

```bash
# .claude/settings.json に登録して使用（直接呼び出しは通常しない）
echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"},"permission_mode":"auto","session_id":"...","cwd":"..."}' | ccchain hook pre
```

常に exit `0`。判定は JSON 本文で伝えます — 正確なスキーマは [Hook 出力](../reference/config#hook-出力) を、非対話 `permission_mode` での `ask` 降格は [`ask_strategy`](../reference/dsl#ask_strategy) を参照。

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
ccchain init                # デフォルトプリセット
ccchain init --sentinel     # deny-first sentinel プリセット（reference/sentinel 参照）
```

`--sentinel` は auto / dontAsk / headless 環境を狙ったキュレート済み deny-first ルールセットを出力します — 詳細は [sentinel プリセット](../reference/sentinel) 参照。

生成後の次のステップも表示されます:
1. `.ccchain.conf` を確認・カスタマイズ
2. `.claude/settings.json` に Hook を登録
3. `ccchain check` で検証
4. `ccchain audit` で展開確認

## `ccchain approve`

[承認トークンフロー](../reference/approve) の人間側コマンド。ccchain が非対話モードで `ask` を `deny + hint` に降格したとき、リクエストは pending として記録されます。オーナーが自身のターミナルで `ccchain approve` を実行すると、その具体的なコマンドだけがブロック解除されます。

```bash
ccchain approve --last                # 直近の pending を承認
ccchain approve --list                # pending 一覧
ccchain approve <hash-prefix>         # ハッシュ prefix 指定で承認
ccchain approve --revoke-all          # 未消費の承認をすべて破棄
ccchain approve --last --ttl 1h       # 有効期限を指定（デフォルト 15m）
ccchain approve --last --global       # session/cwd を問わずマッチ（デフォルト: 現在のみ）
```

> **セキュリティ:** agent の Bash ツールから `ccchain approve` を絶対に実行させないこと。sentinel プリセットは deny するが、二重防御として `settings.json` の `permissions.deny` に `Bash(ccchain approve*)` を追加すること。

## `ccchain stats`

`settings.log:` オプトイン下で hook が書き出す JSONL 評価ログを集計します。詳細は [統計](../reference/stats) を参照。

```bash
ccchain stats                                     # 直近 24h、action 別
ccchain stats --since 7d --group-by rule --top 10 # 1 週間・ルール別・Top 10
ccchain stats --json | jq '.'                     # JSON でスクリプト連携
```

`settings.log:` が未設定なら「log is not enabled」のヒントを出して exit 1 — ログ出力はオプトインです。

## 共通フラグ

| フラグ | 説明 |
|---|---|
| `--config <path>` | 設定ファイルパスを明示指定（検索をスキップ） |
| `--default-action <action>` | 未マッチコマンドのフォールバックアクションを上書き（`allow`, `deny`, `ask`） |
| `-v, --verbose` | 詳細出力 |
| `-q, --quiet` | エラーのみ出力 |
| `--version` | バージョン表示 |
| `-h, --help` | ヘルプ表示 |
