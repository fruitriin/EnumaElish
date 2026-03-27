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

デフォルトルールセットは危険パターン（`find | rm`, `curl | bash`, `eval` 等）をカバーしますが、プロジェクト固有のコマンドは `ask`（確認ダイアログ）になります。

### ステップ 1: プロジェクトのコマンドを収集

プロジェクトで使う典型的なコマンドを列挙し、現在のルールで評価します:

```bash
ccchain eval "go test ./..."
ccchain eval "npm run build"
ccchain eval "git status"
ccchain eval "make test"
```

または `ccchain suggest` を起点にします:

```bash
echo "go test ./...
npm run build
git status
make test
cat README.md" | ccchain suggest
```

### ステップ 2: ルールを提案

評価結果を Claude（または LLM）に渡し、`.ccchain.conf` の追加ルールを提案してもらいます。LLM はプロジェクトの文脈を踏まえてコマンドの安全性を判断できます。

### ステップ 3: セキュリティレビュー（必須）

**このステップは省略不可。** 提案されたルールを適用する前に、セキュリティレビューを実施します:

> セキュリティレビューエージェントに提案ルールの監査を依頼。以下をチェック:
> - `allow` ルールがパイプ/exec コンテキストで悪用されないか
> - コマンドをトップレベルで許可することで破壊的操作のバイパスパスが生まれないか
> - コマンドの副作用が十分に考慮されているか

セキュリティレビューアーが指摘した場合は提案を修正し、承認後に `.ccchain.conf` に追加します。

### ステップ 4: 適用して検証

レビュー済みルールを `.ccchain.conf` に追記:

```
# プロジェクトビルドツール（レビュー済み）
allow go
allow npm
allow make

# 安全なユーティリティ
allow cat
allow cp
allow mkdir
allow echo
allow pwd

# Git
allow git
```

検証:

```bash
ccchain check     # 構文 OK
ccchain audit     # 展開ルールを確認
```

## 5. 上級: パイプ/exec ルール

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
