# 設定ファイル リファレンス

## 探索順序

| 優先度 | パス | 用途 |
|---|---|---|
| 1 | `.ccchain.conf` | プロジェクト共有 |
| 2 | `.ccchain.local.conf` | 個人用上書き |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | グローバル |
| 4 | `~/.claude/ccchain.conf` | フォールバック |

## Hook 登録

`.claude/settings.json` に追加:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{"type": "command", "command": "ccchain hook pre"}]
    }]
  }
}
```

## エラー処理（Fail-Open）

ccchain がエラーを起こした場合（JSON 不正、パース失敗、設定エラー）、コマンドは**許可**されます（exit 0）。ccchain 自身のバグでコマンドをブロックしません。

詳細は[英語版](/reference/config)を参照してください。
