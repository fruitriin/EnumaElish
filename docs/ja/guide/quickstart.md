# クイックスタート

## 1. デフォルト設定を生成

```bash
ccchain init
```

`.ccchain.conf` が生成されます。

## 2. Hook を登録

`.claude/settings.json` に追加:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "ccchain hook pre"}]
      }
    ]
  }
}
```

## 3. 動作確認

```bash
ccchain check          # 設定ファイルの検証
ccchain audit          # ルールのフラット展開表示
ccchain eval "find . | rm"   # 特定コマンドの評価
```

## 4. カスタマイズ

`.ccchain.conf` にプロジェクト固有ルールを追加:

```
allow npm
  |
    deny rm  "npm の出力を rm にパイプしないでください"
```

個人用の上書きは `.ccchain.local.conf`（.gitignore 対象）に:

```
allow rm
```
