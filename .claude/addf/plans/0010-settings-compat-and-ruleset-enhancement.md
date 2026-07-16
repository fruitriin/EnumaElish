# Plan 0010: settings.json 互換レイヤーとデフォルトルールセット強化

## 背景

当初の設計構想（talk_about_original_idea.md）で議論されたが未実装の概念:

1. **settings.json 互換レイヤー** — 既存の `settings.json` の `permissions.allow/deny` を ccchain ルールに自動変換
2. **デフォルトルールセット強化** — 安全なコマンドのカバレッジ拡大、args: ルールの充実
3. **コマンド引数解釈の拡張** — サブコマンドレベルの意味解析（`git status` vs `git filter-branch`）

### ロードマップ上の位置づけ

- **PostToolUse ターンカウント**: ロードマップ上存在するが未計画・未実施。本 Plan のスコープ外
- **動的コマンドの一時ファイル書き出し**: コマンド引数解釈が自然にできれば deny メッセージの誘導で代替可能。実装の必要性は低い
- **source / . コマンドの追跡**: 原理的に不可能。ドキュメントに制限事項として明記のみ

## Phase 1: settings.json 互換レイヤー

### 概要

`ccchain import` サブコマンドで、Claude Code の `settings.json` の permissions を `.ccchain.conf` ルールに変換する。

### 変換ルール

```
settings.json                        → .ccchain.conf
"Bash(git log *)"                    → allow git
                                        args:
                                          ^log\b: allow
"Bash(rm *)"   (in deny)             → deny rm
"Bash(git push *)" (in ask)          → ask git
                                        args:
                                          ^push\b: ask
```

### 設計上の注意

- settings.json のプレフィックスマッチを ccchain の args: regex に変換
- `Bash(git *)` のような広いパターンは `allow git` に（サブコマンド区別なし）
- `Bash(git log *)` のようなサブコマンド付きは args: ルールに変換
- `:*` 構文（非推奨）も ` *` と同等に解釈
- 変換結果は stdout に出力（ファイルは上書きしない）。ユーザーがレビューして追記

### 計算量の考慮

引数解釈での組み合わせ爆発を防ぐ:
- 1つの settings.json エントリ → 1つの ccchain ルール（1対1変換）
- 同一コマンドに対する複数エントリ → args: ルールとして集約
- 変換は静的テキスト変換のみ（eval は行わない）

## Phase 2: デフォルトルールセット強化

### 追加候補（セキュリティレビュー済みカテゴリ）

```
# 安全なユーティリティ（副作用なし）
allow cat
  next: primitive
allow echo
allow pwd
allow diff
allow which
allow mkdir
allow wc
allow sort
allow head
allow tail

# ファイル操作（注意付き）
allow cp
allow chmod
```

### args: ルールの拡充

```
# find の破壊的オプション（既存 + 追加）
allow find
  args:
    -delete: deny  "find -delete is destructive..."
    -prune: allow  # safe option

# git のサブコマンドレベル制御
allow git
  args:
    ^(status|log|diff|show|branch|tag|stash|ls-files|remote|rev-parse)\b: allow
    ^(add|commit|checkout|merge|rebase|fetch|pull|clone|worktree)\b: allow
    ^push\b: ask  "git push requires confirmation"
    ^(filter-branch|filter-repo)\b: deny  "arbitrary code execution risk"
    ^config\b.*(editor|pager|hook): deny  "code execution via config"

# go のサブコマンドレベル制御
allow go
  args:
    ^(test|vet|build|mod|version|fmt|env|doc|tool)\b: allow
    ^(run|generate)\b: ask  "go run/generate can execute arbitrary code"
```

### 計算量の考慮（引数順序問題）

args: パターンは結合文字列への部分マッチなので、引数の順序に依存しない。
ただし以下のケースで計算量が問題になる可能性:

- 同一ルールに大量の args: パターン → 線形走査で O(n)、事前コンパイル済みなので影響軽微
- 正規表現の OR 分岐が多い → RE2 エンジンなので指数爆発しない
- **実際のリスク**: `^(subcommand1|subcommand2|...|subcommandN)\b` のパターンが肥大化する場合、1つのコンパイル済み regex で処理するため問題なし

## Phase 3: デフォルトルール適用後の統合テスト更新

既存の統合テスト（190+ケース）をデフォルトルール強化に合わせて更新:
- `cat README.md` が ask → allow に変わることを検証
- `git status` が ask → allow に変わることを検証
- `go test ./...` が ask → allow に変わることを検証
- `git push` が ask のままであることを検証
- `git filter-branch` が deny であることを検証

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `cmd/ccchain/import.go` | import サブコマンド（新規） |
| `cmd/ccchain/init_cmd.go` | デフォルトルールセット更新 |
| `cmd/ccchain/main.go` | import ルーティング追加 |
| `internal/eval/integration_test.go` | テスト期待値更新 |
| `docs/guide/quickstart.md` | import フローの追記 |

## 検証

1. `ccchain import` が settings.json を正しく変換すること
2. 強化デフォルトルールで安全コマンドが allow になること
3. 危険パターンが引き続き deny/ask であること
4. 統合テスト全パス
5. ベンチマーク回帰なし
