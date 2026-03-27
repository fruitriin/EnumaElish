# Plan: ccchain — DSL 設計とパーサー実装

## Context

`better-permission-tool-design.md` に記載された ccchain (Claude Code Chain) の基盤となる DSL の仕様策定とパーサー実装。他の全プランがこのパーサーに依存する。

## 技術スタック

- **言語**: Go
- **シェルパーサー**: `mvdan.cc/sh`（shfmt と同じライブラリ、Pure Go、外部依存なし）
- **DSL パーサー**: 自作（Go の手書き再帰降下）
- **ビルド成果物**: シングルバイナリ `ccchain`
- **外部依存**: `mvdan.cc/sh` のみ（サプライチェーンリスク最小化）

## 背景

Claude Code の標準 permission system はコマンド先頭のプレフィックスマッチしかできない。パイプ・チェーン・サブシェル内のコマンドを構造的に制御するために、独自 DSL を設計する。

## 設計

### Phase 1: Go プロジェクト初期化

1. プロジェクト構造:
   ```
   /
   ├── cmd/ccchain/         # エントリポイント
   │   └── main.go
   ├── internal/
   │   ├── dsl/             # DSL パーサー
   │   │   ├── lexer.go
   │   │   ├── parser.go
   │   │   ├── ast.go       # DSL の AST 定義
   │   │   └── template.go  # テンプレート解決
   │   ├── shell/           # シェル AST 解析（Plan 0002 で実装）
   │   └── eval/            # ルール評価（Plan 0003 で実装）
   ├── go.mod
   ├── go.sum
   └── Makefile
   ```

2. `go mod init` + `mvdan.cc/sh` を依存に追加

### Phase 2: DSL 文法仕様の策定

1. フォーマット: 独自テキスト DSL（インデントベース）
   - 理由: 監査出力と設定を同じ構文にできる、ネスト構造との親和性が高い
   - TOML/YAML は深いネスト表現が不自然なため不採用

2. 文法要素の定義:
   ```
   # トップレベルルール
   <action> <command> [message]
     # コンテキスト修飾子
     |,>>
       <action> <command> [message]
     exec:
       <action> <command> [message]
     args:
       <pattern>: <action>
     # プロパティ
     mode: block | warn | hint
     message: "..."
     # テンプレート委譲
     next: <template_name>

   # テンプレート定義
   template <name>
     extends: <parent_template>
     # （ルールと同じ構造）

   # Hook Type セクション
   preToolUse
     # ルール群
   postToolUse
     # ルール群

   # 設定
   settings:
     max_context_depth: 2
     max_rules_per_cmd: 5
     fallback: ask
   ```

3. アクション種別:
   | action | 意味 |
   |---|---|
   | `allow` | 許可 |
   | `deny` | ブロック + 理由メッセージ |
   | `warn` | 実行許可 + 警告メッセージ（PreToolUse） |
   | `ask` | ユーザーへ委譲 |
   | `hint` | 次アクション誘導（PostToolUse） |

4. セマンティクス:
   - **last-rule-wins**: 複数ルールがマッチした場合、最後のルールが勝つ
   - **`&&` / `;` はリセット**: チェーン演算子で区切られたコマンドはトップレベルから再評価
   - **`|` / `>>` はネスト**: パイプ・リダイレクトは親子関係として扱う

### Phase 3: パーサー実装

1. レキサー (`internal/dsl/lexer.go`):
   - インデントレベル解析（スペース/タブ）
   - トークン種別: action, command, message(文字列リテラル), keyword, indent, dedent

2. パーサー (`internal/dsl/parser.go`):
   - 手書き再帰降下パーサー
   - DSL AST を構築

3. テンプレート解決 (`internal/dsl/template.go`):
   - `extends:` による継承チェーンを解決
   - `next:` による委譲先の参照を解決
   - 循環参照の検出

### Phase 4: 設定ファイルの配置と読み込み

1. `ccchain` のデフォルト設定ファイル検索パス（優先度順）:
   - `.ccchain.conf`（プロジェクトルート）
   - `.ccchain.local.conf`（ローカル上書き、gitignore 対象）
   - `$CLAUDE_CONFIG_DIR/ccchain.conf`（Claude Code のグローバル設定ディレクトリ）
   - `$CLAUDE_CONFIG_DIR` が未設定の場合は `~/.claude/ccchain.conf`
2. CLI サブコマンド: `ccchain check` で設定ファイルのパース検証
3. CLI 共通フラグ:
   - `--verbose` / `-v`: デバッグログ出力（パース過程、ルールマッチング詳細）
   - `--quiet` / `-q`: エラーのみ出力
   - `--config <path>`: 設定ファイルパスを明示指定

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `cmd/ccchain/main.go` | エントリポイント（新規） |
| `internal/dsl/lexer.go` | レキサー（新規） |
| `internal/dsl/parser.go` | パーサー（新規） |
| `internal/dsl/ast.go` | DSL AST 定義（新規） |
| `internal/dsl/template.go` | テンプレート解決（新規） |
| `internal/dsl/*_test.go` | パーサーテスト（新規） |
| `go.mod`, `go.sum` | モジュール定義（新規） |
| `Makefile` | ビルド定義（新規） |
| `.ccchain.conf` | デフォルトルールセット（新規） |

## 決定事項

- `args:` パターンのマッチング: **regex**（Go の `regexp` 標準パッケージ）
- コメント構文: `#` で統一
- エラーメッセージの国際化は不要（開発者向けツール）
- バージョニング: セマンティックバージョニング

## テスト戦略

### ユニットテスト (`internal/dsl/*_test.go`)

- レキサー: トークン列の期待値テスト
- パーサー: DSL サンプルごとに AST の構造を検証
- テンプレート解決: 継承チェーン、循環参照検出

### テストフィクスチャ (`testdata/dsl/`)

設計メモの全 DSL サンプルをフィクスチャファイルとして配置:
- `testdata/dsl/basic_rules.conf` — 基本ルール
- `testdata/dsl/templates.conf` — テンプレート定義・継承
- `testdata/dsl/hook_sections.conf` — preToolUse / postToolUse セクション
- `testdata/dsl/settings.conf` — settings ブロック
- `testdata/dsl/error_*.conf` — 不正な DSL（エラーケース）
- 各フィクスチャに対応する `.golden` ファイル（期待される AST の JSON 表現）

### ベンチマーク (`internal/dsl/bench_test.go`)

- `BenchmarkLexer` — レキサーのスループット
- `BenchmarkParser` — パーサーのスループット
- `BenchmarkTemplateResolve` — テンプレート解決（深い継承チェーン）
- Hook の応答速度に直結するため、パース処理は **1ms 以下** を目標とする

## 検証

1. パーサーが設計メモの全 DSL サンプルを正しくパースできること ✓
2. テンプレート継承・循環参照検出が動作すること ✓
3. `go test ./...` が通過すること ✓
4. ベンチマークで性能目標を確認すること ✓ (Lexer: 3.6μs, Parser: 4.7μs, Resolve: 5.0μs — 目標1ms大幅クリア)

## 実装完了: 2026-03-27

### レビュー指摘の残課題 (Suggestion、次フェーズで対応可)

- S-1: `copyRules` の深いコピー（Plan 0003 でルールに状態を持たせる際に対応）
- S-2: `LookupTemplate` を O(1) にする（Config に map を持たせる）
- S-3: `main.go` のフラグパーサーを `flag.FlagSet` に置き換え
- S-4: `|:` 構文の仕様明確化
- S-6: `parseTemplate` の extends パース分岐の簡素化
