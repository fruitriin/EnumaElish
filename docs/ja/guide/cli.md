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
| `-v, --verbose` | 詳細出力 |
| `-q, --quiet` | エラーのみ出力 |
| `--version` | バージョン表示 |
| `-h, --help` | ヘルプ表示 |
