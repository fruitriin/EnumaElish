# Plan 0021: テストスメル修正

## Context

savanna-smell-detector v0.3.0 (`5caf079`) で検出された残り8件のテストスメルを修正する。
112件から8件まで絞った結果、全て検討価値のあるスメルが残っている。

## 対象スメル

| # | ファイル | スメル | 対応方針 |
|---|---|---|---|
| 1 | `audit_test.go:22` TestAuditBasicRules | Conditional Test Logic | テーブルドリブン内の `if ... { continue }` を整理 |
| 2 | `audit_test.go:55` TestAuditWithTemplates | Conditional Test Logic | 同上 |
| 3 | `lexer_test.go:83` TestLexMultipleCommands | Conditional Test Logic | 条件分岐の内容を確認して対応 |
| 4 | `parser_test.go:8` TestParseBasicRules | Giant Test | テストケースの分割を検討 |
| 5 | `args_test.go:80` TestArgsInvalidRegex | Silent Skip | `if err != nil { return }` → `t.Skip("reason")` に変更 |
| 6 | `fixture_test.go:157` TestFixtureCompareRulesets | Missing Assertion | アサーション追加 or テスト目的の明確化 |
| 7 | `fixture_test.go:157` TestFixtureCompareRulesets | Conditional Test Logic | 上記と合わせて整理 |
| 8 | `fixture_test.go:61` TestFixtureCombination | Giant Test | ヘルパー抽出で行数削減を検討 |

## 実装ステップ

1. 各テストファイルを読み、スメルの実態を確認
2. 修正が安全なもの（Silent Skip, Missing Assertion）から着手
3. Conditional Test Logic は条件分岐の除去 or `smell-allow` コメントで意図的な許容
4. Giant Test はヘルパー抽出 or テスト分割
5. `make smell` で 0件 or 意図的な `smell-allow` のみ残ることを確認
6. `go test ./...` で全テスト通過を確認

## 検証

```bash
make smell          # 0件 or smell-allow のみ
go test ./...       # 全テスト通過
make check          # 品質ゲート通過
```
