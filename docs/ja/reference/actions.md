# アクション リファレンス

| アクション | 意味 | exit code |
|---|---|---|
| `allow` | 許可 | 0 |
| `deny` | ブロック（理由を Claude に通知） | 2 |
| `warn` | 許可 + 警告を Claude に通知 | 0 |
| `ask` | Claude Code のパーミッションダイアログに委譲 | 0 |
| `hint` | PostToolUse: 次のアクションを誘導 | 0 |

詳細は[英語版](/reference/actions)を参照してください。
