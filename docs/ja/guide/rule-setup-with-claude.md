# Claude Code を使ったルールセットアップ

Claude Code のセッションログから実際のコマンドを収集し、`.ccchain.conf` を最適化するガイド。

## フロー

```
1. 収集   — セッションログから実コマンドを抽出
2. 検出   — ccchain detect でプロジェクト種別を自動判定
3. テスト — ccchain test でルールを検証
4. 調整   — 結果を見てルールを修正
5. 検証   — セキュリティレビュー
```

## ステップ 1: セッションログからコマンドを収集

Claude Code のセッションログは `~/.claude/projects/` に保存されています。

### プロジェクトのコマンドを抽出

```bash
# プロジェクトのログディレクトリを探す
ls ~/.claude/projects/ | grep your-project

# 全 Bash コマンドを抽出
for f in ~/.claude/projects/-Users-you-workspace-your-project/*.jsonl; do
  grep -o '"command":"[^"]*"' "$f" 2>/dev/null
done | sed 's/"command":"//; s/"$//' | sort -u > /tmp/my-commands.txt

# 確認
wc -l /tmp/my-commands.txt
head -20 /tmp/my-commands.txt
```

### ノイズを除去

```bash
cat /tmp/my-commands.txt \
  | grep -vE '^\s*$|^\\' \
  | grep -vE 'claude-501|Progresses' \
  | sort -u > /tmp/my-commands-filtered.txt
```

### 複数プロジェクトから収集

```bash
for d in ~/.claude/projects/-Users-you-workspace-*; do
  for f in "$d"/*.jsonl; do
    grep -o '"command":"[^"]*"' "$f" 2>/dev/null
  done
done | sed 's/"command":"//; s/"$//' | sort -u > /tmp/all-commands.txt
```

## ステップ 2: 初期ルールを生成

```bash
# プロジェクト種別を自動検出
ccchain detect

# デフォルトルール + 検出結果で初期化
ccchain init
ccchain detect >> .ccchain.conf
```

## ステップ 3: コマンドをテスト

```bash
# 収集したコマンドをルールで評価
ccchain test /tmp/my-commands-filtered.txt
```

出力:

```
[allow]  go test ./...
[allow]  git status
[ask]    npm install
[deny]   find . | rm
...

Summary: 85 commands — allow=42, ask=30, deny=13
```

### 別のルールセットと比較

```bash
ccchain test --config testdata/eval/rules-strict.conf /tmp/my-commands-filtered.txt
```

## ステップ 4: ルールを調整

### チューニングループ

```
.ccchain.conf を編集
    ↓
ccchain test /tmp/my-commands-filtered.txt
    ↓
結果を確認（予期しない allow/deny はないか？）
    ↓
ルールを修正
    ↓
繰り返し
```

### 例: ask が多すぎる

```bash
ccchain test /tmp/my-commands-filtered.txt | grep '^\[ask\]'
```

```
[ask]    docker run ubuntu ls
[ask]    kubectl get pods
[ask]    terraform plan
```

ルールを追加:

```
allow kubectl
  args:
    ^(get|describe|logs|diff|version)\b: allow
    ^(delete|exec|apply)\b: ask  "クラスタ変更は確認が必要"

allow terraform
  args:
    ^(plan|show|validate|fmt|version)\b: allow
    ^(apply|destroy)\b: ask  "インフラ変更は確認が必要"
```

再テスト:

```bash
ccchain test /tmp/my-commands-filtered.txt
# → kubectl get → allow, terraform plan → allow
```

### 例: 誤って deny されるコマンド

```bash
ccchain test /tmp/my-commands-filtered.txt | grep '^\[deny\]'
```

プロジェクト固有の上書きは `.ccchain.local.conf` に:

```
allow find
  args:
    -delete: ask  "このプロジェクトでは find -delete を確認付きで許可"
```

## ステップ 5: セキュリティレビュー

ルールを確定する前に Claude にレビューを依頼:

> .ccchain.conf のセキュリティをレビューしてください。
> allow ルールがパイプ/exec コンテキストで悪用されないか確認してください。

### 自動チェック

```bash
# 危険コマンドが allow にならないことを確認
ccchain test /tmp/dangerous-commands.txt
# 全て deny か ask であること
```

## Tips

### コマンドリストを定期的に更新

```makefile
# Makefile に追加
collect-commands:
	@for f in ~/.claude/projects/-Users-*-$(notdir $(CURDIR))/*.jsonl; do \
	  grep -o '"command":"[^"]*"' "$$f" 2>/dev/null; \
	done | sed 's/"command":"//; s/"$$//' | sort -u > testdata/eval/project-commands.txt
	@echo "Collected $$(wc -l < testdata/eval/project-commands.txt) commands"
```

### チームでコマンドリストを共有

```
testdata/eval/
  commands.txt           # 共有コマンドフィクスチャ
  project-commands.txt   # プロジェクト固有（ログから収集）
  rules-default.conf     # 共有ルール
```

### `ccchain suggest` を起点にする

```bash
cat /tmp/my-commands-filtered.txt | ccchain suggest
```

ask に落ちるコマンドを分析し、安全なものに allow を提案します。
