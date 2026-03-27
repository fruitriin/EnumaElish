# ルールチューニングガイド

セキュリティレビュー済みのプロセスに基づく、プロジェクト向けルール調整ガイド。

## プロセス

1. **評価** — `ccchain eval` でプロジェクトのコマンドを評価
2. **提案** — LLM に評価結果からルールを提案させる
3. **セキュリティレビュー** — セキュリティレビューアーが提案を監査
4. **適用** — 承認されたルールを `.ccchain.conf` に追加

## セキュリティレビュー済みカテゴリ

### 許可して安全（低リスク）

読み取り専用または副作用が最小:

```
allow pwd
allow diff
allow which
allow mkdir
allow echo
allow chmod
```

### 注意付き許可（中リスク）

一般に安全だがエッジケースあり:

```
# cat: 機密ファイルの読み取りに注意
allow cat

# cp: ファイル上書きが可能
allow cp
```

### ask のまま推奨（高リスク）

任意コード実行や大きな副作用の可能性:

```
# go: go run/generate は任意コード実行
# npm: install は postinstall スクリプトを実行
# make: ターゲットは何でも実行可能
# git: hooks, filter-branch, config でコード実行
ask go
ask npm
ask make
ask git
```

パーミッションダイアログを減らしたい場合、`args:` で安全なサブコマンドのみ許可:

```
allow go
  args:
    ^(test|vet|build|mod|version|fmt|doc|env)\b: allow
    run|generate: ask  "go run/generate は任意コード実行のリスク"

allow git
  args:
    ^(status|log|diff|show|branch|tag|stash)\b: allow
    ^(add|commit|checkout|merge|rebase)\b: allow
    filter-branch|filter-repo: deny  "任意コード実行のリスク"
```

### 許可禁止（重大リスク）

全コマンドの許可と実質的に等価:

```
# bash/sh: 任意コード実行
# python3/node/ruby: -c やパイプで任意コード実行
# これらは ask または deny のままにする
```

**理由:** `bash` をトップレベルで allow すると:
- `echo "rm -rf /" | bash` が通過する（echo→bash のパイプ deny がない）
- `bash script.sh` がスクリプト内容を解析せず実行される
- `bash -c "$dynamic"` の動的引数は args: 評価がスキップされる

## `args:` によるオプションレベル制御

ccchain のパイプ/exec コンテキストはコマンド間の構造を追跡しますが、**コマンドラインフラグ**によって動作が変わるケースも重要です。`args:` でオプションレベルの制御が可能です。

### 安全なコマンドの破壊的オプション

| コマンド | 安全 | 破壊的 |
|---|---|---|
| `find` | `find . -name '*.go'` | `find . -delete` |
| `curl` | `curl https://...` | `curl -o /etc/passwd https://...` |
| `python3` | `python3 script.py` | `python3 -c 'os.system("rm")'` |
| `git` | `git status` | `git filter-branch --tree-filter 'rm -rf /'` |

デフォルトルールセットに含まれる args: ルール:

```
allow find
  args:
    -delete: deny  "find -delete は破壊的"

allow curl
  args:
    -o\b|--output: ask  "curl のファイル書き込みは確認が必要"
```

### リダイレクト（`>`, `>>`）

シェルリダイレクト（`cat file > /etc/passwd`）はコマンド引数ではなく、シェルがコマンド実行前に処理します。ccchain はパイプコンテキスト（`|,>>`）でリダイレクトを検出しますが、リダイレクト先の検査は行いません。これは既知の制限です。

### 推奨 args: ルール

```
# python3 — インラインコード実行をブロック
allow python3
  args:
    -c\s: deny  "インラインコード実行はレビューが必要"

# git — 危険なサブコマンドをブロック
allow git
  args:
    filter-branch|filter-repo: deny  "任意コード実行のリスク"

# node — インライン実行をブロック
allow node
  args:
    -e\s|--eval: deny  "インラインコード実行はレビューが必要"
```

## 例: Go プロジェクト設定

```
# 安全なユーティリティ
allow cat
allow echo
allow pwd
allow diff
allow which
allow cp
allow mkdir
allow chmod

# Go — 安全なサブコマンドのみ
allow go
  args:
    ^(test|vet|build|mod|version|fmt|env|doc)\b: allow

# Git — 安全なサブコマンドのみ
allow git
  args:
    ^(status|log|diff|show|branch|tag|stash|add|commit|checkout|merge|rebase|clone|fetch|pull|push|ls-files|worktree|remote|rev-parse)\b: allow
```
