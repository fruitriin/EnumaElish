# Plan: ccchain — コード品質リファクタリング

## Context

コードレビューの Warning/Suggestion とセキュリティレビューの Medium 指摘を統合したリファクタリング。

## 修正項目

### Warning (必ず修正)

#### W-1: `Segment.Type` の型定数化

文字列リテラル `"pipeline"` / `"single"` を型付き定数に変更。typo の実行時エラーをコンパイルエラーに変える。

```go
type SegmentType string
const (
    SegmentTypeSingle   SegmentType = "single"
    SegmentTypePipeline SegmentType = "pipeline"
)
```

#### W-2: `EvaluateTopology` の nil Settings panic

公開 API として `config.Settings == nil` に対応。

#### W-3: `_ = parent` 死コード除去

`template.go:70` の未使用変数を削除。

#### W-4: Audit の exec ルール truncation 欠落

`maxRulesPerCmd` を exec ルールにも適用。

#### W-5: PostRules 未評価の明示

`runHookPost` のコメントと README/ドキュメントに「PostRules 評価は未実装」を明記。

### Suggestion (改善)

#### S-1: テンプレート収集ロジックの重複排除

`eval` と `audit` の `collectTemplatePipeRules` / `collectTemplateExecRules` を `dsl` パッケージに移動。

```go
// internal/dsl/lookup.go
func CollectTemplatePipeRules(tmpl *Template, config *Config) []*Rule
func CollectTemplateExecRules(tmpl *Template, config *Config) []*Rule
```

#### S-2: `LookupTemplate` の O(1) 化

`Config` に `templateIndex map[string]*Template` を追加。`ResolveTemplates` 後に設定。

#### S-3: `main.go` の blank identifier 残置削除

`_ = verbose / _ = quiet / _ = cmdArgs` を削除。

#### S-4: `parseArgsBlock` の無音スキップをエラーに

パターンや Action が空の行を `ParseError` として報告。

#### S-6: `stripQuotes` のエスケープ未対応

`\"` を含むクォート文字列の処理改善。

#### S-7: xargs の GNU 長オプション未対応

`--flag=value` 形式の認識（Plan 0005 の VULN-04 と統合）。

### Medium (セキュリティ)

#### VULN-07: 設定エラー時の strict モードオプション

`settings: strict_config_error: true` で設定読み込み失敗時に deny にするオプション。

#### VULN-08: `args:` ルール評価の実装

`matchCommand` 内で `ArgsRule` のパターンをコマンド引数に対して regex マッチ。

#### VULN-09: `next:` 循環の静的検出

`ResolveTemplates` で `next:` チェーンの循環もチェック。

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/shell/topology.go` | W-1: SegmentType 型定数化 |
| `internal/eval/evaluate.go` | W-2, S-1, VULN-08: nil check, ロジック移動, args 評価 |
| `internal/dsl/template.go` | W-3, S-2, VULN-09: 死コード削除, map 化, next 循環検出 |
| `internal/dsl/lookup.go` | S-1: テンプレート収集ロジック（新規） |
| `internal/dsl/parser.go` | S-4: parseArgsBlock エラー |
| `internal/audit/audit.go` | W-4, S-1: truncation 修正, ロジック移動 |
| `cmd/ccchain/main.go` | S-3: blank identifier 削除 |
| `internal/shell/nestrules.go` | S-6: stripQuotes 改善 |

## 検証

1. 既存テスト全パス
2. `args:` ルール評価のテスト追加
3. `go test ./...` + `go vet ./...` 通過
4. ベンチマーク回帰なし
