# カスタマイズ

## 設定ファイル探索順

| 優先度 | パス | 用途 |
|---|---|---|
| 1 | `.ccchain.conf` | プロジェクト共有（git 管理） |
| 2 | `.ccchain.local.conf` | 個人用上書き（gitignore） |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | グローバル設定 |
| 4 | `~/.claude/ccchain.conf` | フォールバック |

## プロジェクトルール

`.ccchain.conf` にチームで共有するルールを記述:

```
allow npm
  |
    deny rm

deny rm  "trash コマンドを使ってください"
allow trash
```

## 個人用オーバーライド

`.ccchain.local.conf`（.gitignore 対象）:

```
allow rm
```

詳細は[英語版](/guide/customization)を参照してください。
