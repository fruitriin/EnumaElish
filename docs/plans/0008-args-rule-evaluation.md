# Plan: ccchain — args: ルール評価の実装

## Context

DSL の `args:` ブロックはパースされるが評価エンジンで無視されている。セキュリティレビュー（ARGS-01〜10）の指摘を踏まえて安全に実装する。

## 設計決定

### マッチング単位: 結合文字列への regex マッチ

引数配列を `strings.Join(cmd.Args, " ")` で結合し、その文字列に対して regex マッチする。

```go
argsStr := strings.Join(cmd.Args, " ")
matched := ar.Compiled.MatchString(argsStr)
```

- `-X POST` は `curl -X POST url` の結合文字列 `-X POST url` にマッチする
- `-XPOST`（結合形式）もパターン `-X ?POST` で対応可能
- 部分マッチの注意はドキュメントで明記し、アンカー使用を推奨

### 優先順位: args: は親ルールのアクションを上書き

```
allow curl
  args:
    -X POST: ask
```

1. `curl -X GET url` → `allow curl` にマッチ → args: `-X POST` にマッチしない → **allow**
2. `curl -X POST url` → `allow curl` にマッチ → args: `-X POST` にマッチ → **ask（上書き）**

args: 内で複数パターンがマッチした場合は last-rule-wins。

### 動的引数: 親ルールにフォールバック

引数に変数展開（`$VAR`, `$(cmd)`）が含まれる場合、args: 評価をスキップし、親ルールのアクションをそのまま返す。

```
allow curl
  args:
    -X POST: ask

# curl -X $METHOD url → args: 評価スキップ → allow（親ルールのアクション）
```

## 実装

### Phase 1: ArgsRule の事前コンパイル（ARGS-01, ARGS-06）

1. `ArgsRule` に `Compiled *regexp.Regexp` フィールドを追加
2. `ResolveTemplates`（または新規 `ValidateConfig`）内で全 ArgsRule.Pattern を `regexp.Compile`
3. コンパイルエラーは `ParseError` として設定ロードを失敗させる（fail-open にしない）

```go
// internal/dsl/ast.go
type ArgsRule struct {
    Pattern  string
    Action   Action
    Message  string
    Line     int
    Compiled *regexp.Regexp // 事前コンパイル済み
}
```

### Phase 2: matchCommand での args: 評価（ARGS-03）

1. `matchCommand` でルールがマッチした後、`ArgsRules` を評価
2. 引数を結合した文字列に対して各パターンをマッチ
3. マッチした場合、アクションを上書き（last-rule-wins）
4. 引数に動的展開が含まれる場合は args: 評価をスキップ（ARGS-04）

```go
func applyArgsRules(cmd *shell.Command, rule *dsl.Rule, baseResult *Result) *Result {
    if len(rule.ArgsRules) == 0 {
        return baseResult
    }
    // 動的引数チェック
    if containsDynamicArgs(cmd.Args) {
        return baseResult
    }
    argsStr := strings.Join(cmd.Args, " ")
    var lastMatch *Result
    for _, ar := range rule.ArgsRules {
        if ar.Compiled != nil && ar.Compiled.MatchString(argsStr) {
            lastMatch = &Result{Action: ar.Action, Message: ar.Message}
        }
    }
    if lastMatch != nil {
        return lastMatch
    }
    return baseResult
}
```

### Phase 3: パイプ/exec コンテキストへの適用（ARGS-07）

`matchInPipeContext` / `matchInExecContext` でマッチした後にも `applyArgsRules` を適用。

### Phase 4: テンプレートの args: 収集（ARGS-10）

`collectTemplateArgsRules` を `dsl` パッケージに追加し、テンプレート経由の args: ルールも評価対象にする。

### Phase 5: ドキュメント

- DSL リファレンスの args: セクションを「未実装」注記から正式な仕様に更新
- 部分マッチの注意、アンカー推奨を明記（ARGS-05）
- 動的引数の挙動を明記
- バイパス注意（`-XPOST` 等の結合形式）を明記（ARGS-02）

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/dsl/ast.go` | ArgsRule に Compiled フィールド追加 |
| `internal/dsl/template.go` | ValidateConfig で ArgsRule を事前コンパイル |
| `internal/eval/evaluate.go` | applyArgsRules 関数、matchCommand/matchInPipeContext/matchInExecContext に組み込み |
| `internal/eval/evaluate_test.go` | args: 評価テスト追加 |
| `internal/eval/security_test.go` | バイパスシナリオテスト |
| `docs/reference/dsl.md`, `docs/ja/reference/dsl.md` | args: セクション更新 |

## テスト戦略

### ユニットテスト

- 基本マッチ: `curl -X POST` に `-X POST: ask` がマッチ
- 結合形式: `curl -XPOST` に `-X ?POST` がマッチ
- 複数パターン: last-rule-wins で最後のマッチが勝つ
- 動的引数: `curl -X $METHOD` で args: がスキップされる
- 部分マッチの検証: アンカーなしで意図しないマッチが起きるケース
- パイプコンテキスト: パイプ内コマンドの args: が効くこと
- コンパイルエラー: 不正な regex で設定ロードが失敗すること
- 空の args: ブロック: 親ルールのアクションがそのまま返ること

### セキュリティテスト

- ARGS-02: `-XPOST`, `--request=POST`, `--request POST` のバリエーション
- ARGS-04: `$VAR` を含む引数で args: がスキップされること
- ARGS-05: `--data "-X GET is safe" -X DELETE` で誤マッチしないこと（アンカー使用時）

## 検証

1. 既存テスト全パス ✓
2. args: 評価テスト 6 件追加 ✓
3. `go test ./...` + `go vet ./...` 通過 ✓

## 実装完了: 2026-03-27
