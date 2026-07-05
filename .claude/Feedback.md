# Process Feedback

開発プロセスの振り返りと改善を記録する。

## 記録方法

タスク完了時や問題発生時に、以下のいずれかのセクションに追記する。

## オーナーフィードバック

## 問題の記録

- `go get mvdan.cc/sh/v3` が Go 1.25.0 を要求し、.tool-versions で設定した 1.24.10 からの自動アップグレードが発生した。Go バージョン管理に注意

## 改善アクション

## 完了済み

### 2026-07-05 — CLAUDE.repo.md のテストセクションに Go テスト明記

- **元の改善アクション**: ADDF テストランナー (`bash .claude/tests/run-all.sh`) と Go テスト (`go test ./...`) が共存する構成。CLAUDE.repo.md のテストセクションに Go テストも明記すべき
- **対応済み**: CLAUDE.repo.md の「テスト」セクションに Go テスト（`go test ./...` / `go vet ./...` / `go build ./cmd/ccchain`）と ADDF テスト（`bash .claude/tests/run-all.sh`）の両方を明記済み

### 2026-07-05 — savanna-smell-detector Go 相性評価（v0.3.0）— 分析完了・upstream で対応進行中

**状態**: 分析済み。改善提案は upstream の [fruitriin/savanna-smell-detector#15](https://github.com/fruitriin/savanna-smell-detector/issues/15) として起票済み、対応が進行中。

112件検出のうち、内訳と Go イディオムとの相性は以下のように分析されている:

| スメル | 件数 | 評価 | 理由 |
|---|---|---|---|
| Conditional Test Logic | 87 | **誤検出** | `if err != nil { t.Fatalf(...) }` は Go の標準イディオム。全テストで使うため大量検出されていた。table-driven test の `for range` 内の `if` も含まれる |
| Missing Assertion | 13 | **誤検出** | カスタムヘルパー `assertEqual(t, ...)` をアサーションとして認識しない。Go では `t.Fatal`/`t.Error` が実質アサーション |
| Giant Test | 8 | **一部有用** | 統合テスト（TestIntegrationSafeCommands 等）は構造体スライスのテーブルドリブンで意図的に長い。ただし本当に分割すべきものもある可能性 |
| Redundant Print | 3 | **一部有用** | fixture_test.go の比較レポート出力は意図的。ただし不要な `t.Log` が残っている可能性は確認価値あり |
| Silent Skip | 1 | **有用** | `TestArgsInvalidRegex` での条件スキップ。`t.Skip` に変更するのが望ましいと判定された |

**upstream に持ち込んだ改善提案（[#15](https://github.com/fruitriin/savanna-smell-detector/issues/15) で対応進行中）:**
1. Go の `if err != nil { t.Fatalf }` パターンを Conditional Test Logic から除外する（Go イディオム）
2. `t.Fatal` / `t.Fatalf` / `t.Error` / `t.Errorf` をアサーションとして認識する
3. カスタムアサーションヘルパー（`assertEqual` 等）を設定で登録可能にする
4. JSON 出力で severity が全て 0 になるバグ（コンソール出力では正しく表示される）を修正する
