# Plan: ccchain — シェルコマンド構造解析エンジン

## Context

ccchain がシェルコマンドの構造（パイプ、チェーン、サブシェル等）を理解するための AST 解析エンジン。`mvdan.cc/sh` を使い、Plan 0001 の DSL パーサーと組み合わせて、Plan 0003 のルール評価エンジンの入力を生成する。

## 前提

- Plan 0001（DSL パーサー）が完了していること

## 設計

### Phase 1: `mvdan.cc/sh` によるシェル AST 取得

1. `mvdan.cc/sh/syntax` パッケージでシェルコマンド文字列をパースする
   - パースモード: **bash**（`syntax.LangBash`）。`<()` 等の bash 拡張を解析可能にする
   - 外部バイナリ（shfmt）は不要。ライブラリとして組み込み済み
   - フォールバック不要（Pure Go、ビルド時に解決済み）

2. AST から抽出する構造情報:
   | シェル構文 | `mvdan.cc/sh` の AST ノード | ccchain での扱い |
   |---|---|---|
   | `cmd \| cmd` | `BinaryExpr` (Pipe) | パイプライン（親子関係） |
   | `cmd >> file` | `Redirect` | リダイレクト（親子関係） |
   | `cmd && cmd` | `BinaryExpr` (AndStmt) | リセットポイント |
   | `cmd ; cmd` | `StmtList` | リセットポイント |
   | `$(cmd)` | `CmdSubst` | サブシェル（ネスト） |
   | `<(cmd)` | `ProcSubst` | プロセス置換（ネスト） |

### Phase 2: コマンドトポロジーの構築

1. `internal/shell/topology.go` に実装:
   - `mvdan.cc/sh` の AST を走査し「実行トポロジー」に変換する
   - トポロジーは Go の構造体で表現（JSON 中間表現は不要）

   ```go
   type Topology struct {
       Segments []Segment
   }
   type Segment struct {
       Type     string    // "pipeline", "single"
       Commands []Command
   }
   type Command struct {
       Name       string
       Args       []string
       Analyzable bool
       Nested     *Topology // find -exec, bash -c 等
   }
   ```

2. リセット動作の実装:
   - `&&` / `;` で区切られた各セグメントを独立したトップレベルとして扱う
   - `find . | rm` → find のネストルールで rm を評価
   - `find . && rm` → `&&` でリセット → rm をトップレベルルールで評価

### Phase 3: カスタムネストルール

特定コマンドの引数がコマンドとして実行されるケースを検出する:

1. **組み込みカスタムネストルール** (`internal/shell/nestrules.go`):
   - `find -exec CMD {} \;` — `-exec` 以降をコマンドとして抽出
   - `xargs CMD` — 第一引数をコマンドとして抽出
   - `bash -c "CMD"` / `sh -c "CMD"` — `-c` の引数を `mvdan.cc/sh` で再帰パース
   - `eval "CMD"` — 引数を再帰パース（静的に解析可能な場合のみ）

2. ネストルールはコードに組み込み（設定ファイルでの拡張は将来課題）

### Phase 4: 解析不能パターンの検出

1. 静的解析不能なパターンを明示的に検出・報告する:
   - 変数展開を含むコマンド: `$cmd`, `${cmd}`, `$(generate_cmd)`
   - エイリアス: 追跡不可能
   - 動的 eval: `eval "$dynamic_string"`

2. 検出結果は `Command.Analyzable = false` で後続処理に伝搬する

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/shell/parse.go` | `mvdan.cc/sh` によるシェルパース（新規） |
| `internal/shell/topology.go` | トポロジー構築（新規） |
| `internal/shell/nestrules.go` | カスタムネストルール（新規） |
| `internal/shell/*_test.go` | テスト（新規） |

## テスト戦略

### ユニットテスト (`internal/shell/*_test.go`)

- パース: シェルコマンド文字列 → `mvdan.cc/sh` AST の基本動作
- トポロジー構築: パイプライン、リセットポイント、ネスト
- カスタムネストルール: find -exec, xargs, bash -c 各パターン
- 解析不能検出: 変数展開、動的 eval

### テストフィクスチャ (`testdata/shell/`)

設計メモの全サンプルコマンドをフィクスチャとして配置:
- `testdata/shell/commands.txt` — 入力コマンド一覧（1行1コマンド）
- `testdata/shell/topologies/*.golden` — 各コマンドの期待トポロジー
- `testdata/shell/unanalyzable.txt` — 解析不能パターン一覧

### 統合テスト (`internal/shell/integration_test.go`)

- フィクスチャの全コマンドを一括パース → トポロジー変換 → golden ファイルと比較
- テーブル駆動テストで網羅

### ベンチマーク (`internal/shell/bench_test.go`)

- `BenchmarkShellParse` — `mvdan.cc/sh` によるパース速度
- `BenchmarkTopologyBuild` — トポロジー構築速度
- `BenchmarkNestedParse` — 再帰パース（bash -c のネスト）
- 典型的なコマンド（パイプ3段 + チェーン2段程度）で **1ms 以下** を目標

## 検証

1. 設計メモの全サンプルコマンドが正しくトポロジーに変換されること
2. `&&` / `;` のリセット動作が正しいこと
3. `find -exec`, `xargs`, `bash -c` のカスタムネストが検出されること
4. 解析不能パターンが `Analyzable: false` でマークされること
5. `go test ./...` が通過すること
6. ベンチマークで性能目標を確認すること
