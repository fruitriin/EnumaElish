# Plan: ccchain — ドキュメント整備・仕上げ

## Context

セキュリティ強化・リファクタリング後の仕上げ。ドキュメントの正確性確認、未実装機能の明示、リリース準備。

## 修正項目

### ドキュメント

#### args: ルールの状態明示

README、DSL リファレンス、ドキュメントサイトに `args:` ルールの実装状況を明記:
- Plan 0006 で実装された場合: 使い方を説明
- 未実装のまま残る場合: 「パースされるが評価には未対応」と注記

#### PostToolUse の状態明示

`ccchain hook post` が現在パススルーであることをドキュメントに明記。

#### 設定ファイルのマージ順序の明確化

`~/.claude/ccchain.conf` がプロジェクト設定を上書きする点（INFO-01）をドキュメントに追記。

#### Fail-Open 設計の明文化

VULN-07 の指摘を踏まえ、Fail-Open 設計の意図・リスク・strict_config_error オプションの使い方をドキュメントに追記。

### テストカバレッジ

#### S-5: テストヘルパーの整理

`assertEqual` / `mustParseConfig` の重複を各パッケージ内で `testhelper_test.go` に分離。

### リリース準備

#### LICENSE ファイル作成

MIT ライセンスファイルを追加。

#### VULN-10: DSL Scanner バッファ制限

`bufio.Scanner` のバッファ上限を 1MB に拡張。

#### VULN-11: ANSI-C quoting の安全なフォールバック確認

テストを追加して `$'...'` 形式が `(unparseable)` として安全にフォールバックされることを確認。

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `docs/guide/`, `docs/ja/guide/` | args:, PostToolUse, Fail-Open の記述更新 |
| `docs/reference/`, `docs/ja/reference/` | DSL リファレンスの args: セクション更新 |
| `README.md`, `README.en.md` | 制限事項セクション追加 |
| `internal/dsl/lexer.go` | VULN-10: Scanner バッファ拡張 |
| `LICENSE` | MIT ライセンスファイル（新規） |
| `internal/shell/*_test.go` | VULN-11: ANSI-C quoting フォールバックテスト |

## 検証

1. ドキュメントサイトのビルド成功 ✓
2. 全テストパス ✓
3. リリース可能な状態の確認 ✓

## 実装完了: 2026-03-27
