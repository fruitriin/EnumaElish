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
ccchain eval "find . | rm"   # → deny
ccchain eval "ls -la | head" # → allow
```

## 4. プロジェクトに合わせてルールを調整

デフォルトルールセットは一般的なパターン（`find`, `ls`, `grep`, `curl`）をカバーしますが、プロジェクト固有のビルドツールは最初 `ask`（確認ダイアログ）になります。

`ccchain suggest` でプロジェクトで実際に使うコマンドからルールを自動提案できます:

```bash
# プロジェクトで使う典型的なコマンドを渡す
echo "go test ./...
go build ./cmd/myapp
npm run build
git status
make test
cat README.md
cp src dst
mkdir -p /tmp/build" | ccchain suggest
```

出力:

```
# Suggested rules for .ccchain.conf
# Commands that currently fall through to 'ask' but appear safe:

allow cat
allow cp
allow mkdir
# ask go  # review before allowing
# ask npm  # review before allowing
```

- **`allow` 行**: ccchain が安全と認識したコマンド — そのまま `.ccchain.conf` にコピー
- **`# ask` 行**: プロジェクト固有ツール — レビューして許可するか判断

`.ccchain.conf` に追記:

```
# 安全なユーティリティ
allow cat
allow cp
allow mkdir
allow echo
allow pwd
allow diff
allow which

# プロジェクトのビルドツール
allow go
allow npm
allow make

# Git
allow git
```

## 5. 上級: パイプ/exec ルールのカスタマイズ

```
allow npm
  |
    deny rm  "npm の出力を rm にパイプしないでください"
```

## 6. 個人用オーバーライド

`.ccchain.local.conf`（.gitignore 対象）:

```
allow rm
```
