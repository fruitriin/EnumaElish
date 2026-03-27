# CLI コマンド

| コマンド | 説明 |
|---|---|
| `ccchain check` | 設定ファイルの構文検証 |
| `ccchain hook pre` | PreToolUse Hook（stdin から JSON を受け取り判定） |
| `ccchain hook post` | PostToolUse Hook |
| `ccchain eval "cmd"` | コマンドの評価結果を JSON で出力 |
| `ccchain audit` | 全ルールのフラット展開表示 |
| `ccchain init` | デフォルト .ccchain.conf を生成 |

## 共通フラグ

| フラグ | 説明 |
|---|---|
| `--config <path>` | 設定ファイルパスを明示指定 |
| `-v, --verbose` | 詳細出力 |
| `-q, --quiet` | エラーのみ出力 |
| `--version` | バージョン表示 |

詳細は[英語版](/guide/cli)を参照してください。
