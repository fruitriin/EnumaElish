# 設定ファイル リファレンス

## 探索順序

ccchain は優先度順に設定ファイルを探します。後のファイルのルールが追加され、last-rule-wins で上書き可能です:

| 優先度 | パス | 用途 |
|---|---|---|
| 1 | `.ccchain.conf` | プロジェクト共有ルール（git にコミット） |
| 2 | `.ccchain.local.conf` | 個人用上書き（.gitignore 対象） |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | Claude Code のグローバル設定 |
| 4 | `~/.claude/ccchain.conf` | フォールバックグローバル設定 |

`--config <path>` で検索をスキップし特定ファイルを直接指定できます。

## マージ動作

複数ファイルが見つかった場合:
- **テンプレート**: 全ファイルから収集（同名テンプレートはエラー）
- **ルール**: 検索順に追加（last-rule-wins で上書き可能）
- **Settings**: 最後の `settings:` ブロックが優先

## Hook 登録

### PreToolUse

`.claude/settings.json` に追加:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{
          "type": "command",
          "command": "ccchain hook pre"
        }]
      }
    ]
  }
}
```

### PostToolUse（オプション）

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{
          "type": "command",
          "command": "ccchain hook post"
        }]
      }
    ]
  }
}
```

## Hook 入力フォーマット

ccchain は Claude Code の Hook フォーマットに従った JSON を stdin から読みます:

```json
{
  "tool_name": "Bash",
  "tool_input": {
    "command": "find . -name '*.log' | rm -rf"
  }
}
```

Bash 以外のツールはそのまま通過します（exit 0）。

## Hook 出力

### PreToolUse

| 判定 | exit code | 出力 |
|---|---|---|
| 許可 | 0 | (なし) |
| 拒否 | 2 | メッセージが stderr に |
| 警告 | 0 | `{"decision":"allow","message":"..."}` が stdout に |
| 委譲 | 0 | `{"decision":"ask"}` が stdout に |

## エラー処理（Fail-Open）

ccchain がエラーに遭遇した場合（JSON 不正、パース失敗、設定エラー）、コマンドは**許可**されます（exit 0）。エラーは stderr にログ出力されます。

これにより ccchain 自身のバグが Claude の操作をブロックすることはありません。

## 推奨 `.gitignore`

```gitignore
.ccchain.local.conf
```
