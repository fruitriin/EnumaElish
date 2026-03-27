# Plan: ccchain — 監査出力・デフォルトルールセット・配布

## Context

ccchain の監査機能、実用的なデフォルトルールセット、配布方法の整備。ccchain を「設定して終わり」ではなく「設定を理解・検証できるツール」にする。

## 前提

- Plan 0001〜0003（DSL パーサー、AST 解析、評価エンジン）が完了していること

## 設計

### Phase 1: 監査出力（フラット展開）

1. `ccchain audit` サブコマンドとして実装:
   - DSL ルールを読み込み、全ルールの展開結果を一行ごとに出力
   - テンプレート展開を可視化

2. 出力形式:
   ```
   [allow]  ls
   [allow]  ls | cat            (template: primitive)
   [allow]  find
   [allow]  find | grep         (template: bulkExec)
   [deny]   find | rm           (template: bulkExec)       "don't pipe into destructive"
   [deny]   find -exec rm       (template: bulkExec.exec)  "expand to tempfile first"
   [---]    find && rm          (&&: reset → top-level rm rule)
   --- depth > 2: truncated ---

   Settings:
     max_context_depth: 2
     max_rules_per_cmd: 5
     fallback: ask

   Stats:
     rules: 23
     templates: 4
   ```

3. 打ち切り制御:
   - `settings.max_context_depth` 以上の深さは展開しない
   - `settings.max_rules_per_cmd` 以上のルールは省略
   - 打ち切られた部分を明示表示

### Phase 2: デフォルトルールセット

1. `ccchain init` サブコマンドで `.ccchain.conf` を生成:
   ```
   # === ccchain Default Rules ===

   settings:
     max_context_depth: 2
     max_rules_per_cmd: 5
     fallback: ask

   # --- テンプレート ---

   template primitive
     |,>>
       allow cat, echo, head, tail, wc, sort, uniq

   template safeRead
     next: primitive
     |,>>
       allow grep, awk, sed

   template bulkExec
     extends: safeRead
     |,>>
       deny rm    "don't pipe into destructive commands"
     exec:
       deny rm    "expand to tempfile first"
       allow cp, mv, touch

   # --- PreToolUse ルール ---

   preToolUse

   allow ls
     next: primitive

   allow find
     next: bulkExec

   allow xargs
     next: bulkExec

   allow grep
     next: safeRead

   deny rm -rf /    "root deletion is never allowed"
   ask rm
     message: "ファイル削除を確認してください"

   allow curl
     |
       deny bash   "curl | bash is not allowed"
       deny sh     "curl | sh is not allowed"

   deny eval       "eval は静的解析不能です。コマンドを直接記述してください"
   ```

2. ユーザーカスタマイズ:
   - `.ccchain.local.conf` でプロジェクト固有ルールを追加（gitignore 対象）
   - ローカルルールはデフォルトルールの後に評価（last-rule-wins で上書き可能）

### Phase 3: CLI サブコマンド一覧

| サブコマンド | 用途 |
|---|---|
| `ccchain hook pre` | PreToolUse Hook（stdin からツール JSON を受け取り判定） |
| `ccchain hook post` | PostToolUse Hook |
| `ccchain audit` | ルールのフラット展開・監査出力 |
| `ccchain check` | 設定ファイルの構文検証 |
| `ccchain init` | デフォルト設定ファイルを生成 |
| `ccchain reset` | ターンカウンターをリセット |
| `ccchain eval "cmd"` | 指定コマンドの評価結果を JSON で出力（デバッグ・スクリプト連携用） |

### Phase 4: 配布とインストール

1. **GitHub Releases**: `Makefile` で `GOOS`/`GOARCH` クロスコンパイルしてバイナリを配布
   - darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
   - goreleaser 等の外部ツールは使わない（依存最小化の方針に従う）
2. **`go install`**: `go install github.com/.../ccchain@latest`
3. **Homebrew**: 将来対応（ユーザー数次第）
4. セットアップガイド: `ccchain init` 実行 → `settings.json` に Hook 登録

### Phase 5: ドキュメント

1. README に使い方・DSL サンプル・インストール手順を記載
2. `docs/dsl-reference.md` — DSL リファレンス

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `cmd/ccchain/audit.go` | audit サブコマンド（新規） |
| `cmd/ccchain/check.go` | check サブコマンド（新規） |
| `cmd/ccchain/init.go` | init サブコマンド（新規） |
| `cmd/ccchain/eval.go` | eval サブコマンド（新規） |
| `internal/audit/audit.go` | 監査出力エンジン（新規） |
| `.ccchain.conf` | デフォルトルールセット（新規） |
| `Makefile` | goreleaser 設定を追加 |
| `README.md` | 使い方・インストール手順を更新 |
| `docs/dsl-reference.md` | DSL リファレンス（新規） |

## テスト戦略

### ユニットテスト (`internal/audit/*_test.go`)

- フラット展開ロジック: テンプレート展開、打ち切り制御
- 出力フォーマット: 期待する文字列との一致

### テストフィクスチャ (`testdata/audit/`)

- `testdata/audit/default_rules.conf` — デフォルトルールセット
- `testdata/audit/default_rules.golden` — 期待する監査出力
- `testdata/audit/custom_rules.conf` — カスタムルール付き
- `testdata/audit/custom_rules.golden` — 期待する監査出力

### 統合テスト

- `ccchain audit` サブコマンドの出力を golden ファイルと比較
- `ccchain init` → `ccchain check` → `ccchain audit` の一連フローテスト
- `ccchain eval "cmd"` の出力検証

### ベンチマーク (`internal/audit/bench_test.go`)

- `BenchmarkAudit` — 監査出力の生成速度（デフォルトルールセット）
- 監査は対話的に使うため、**100ms 以下** を目標

## 検証

1. 監査出力が設計メモのサンプルと一致すること
2. デフォルトルールセットで基本的なセキュリティシナリオが動作すること:
   - `curl | bash` → deny
   - `find . -exec rm` → deny
   - `rm -rf /` → deny
   - `ls | head` → allow
3. `ccchain check` で DSL の構文エラーが検出されること
4. `go test ./...` が通過すること
5. `go build ./cmd/ccchain` でシングルバイナリが生成されること
6. ベンチマークで性能目標を確認すること
