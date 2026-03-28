# Process Feedback

開発プロセスの振り返りと改善を記録する。

## 記録方法

タスク完了時や問題発生時に、以下のいずれかのセクションに追記する。

## オーナーフィードバック

## 問題の記録

- `go get mvdan.cc/sh/v3` が Go 1.25.0 を要求し、.tool-versions で設定した 1.24.10 からの自動アップグレードが発生した。Go バージョン管理に注意

## 改善アクション

- ADDF テストランナー (`bash .claude/tests/run-all.sh`) と Go テスト (`go test ./...`) が共存する構成。CLAUDE.repo.md のテストセクションに Go テストも明記すべき

### savanna-smell-detector Go 相性評価（v0.3.0）

112件検出。内訳と Go イディオムとの相性:

| スメル | 件数 | 評価 | 理由 |
|---|---|---|---|
| Conditional Test Logic | 87 | **誤検出** | `if err != nil { t.Fatalf(...) }` は Go の標準イディオム。全テストで使うため大量検出される。table-driven test の `for range` 内の `if` も含まれる |
| Missing Assertion | 13 | **誤検出** | カスタムヘルパー `assertEqual(t, ...)` をアサーションとして認識しない。Go では `t.Fatal`/`t.Error` が実質アサーション |
| Giant Test | 8 | **一部有用** | 統合テスト（TestIntegrationSafeCommands 等）は構造体スライスのテーブルドリブンで意図的に長い。ただし本当に分割すべきものもある可能性 |
| Redundant Print | 3 | **一部有用** | fixture_test.go の比較レポート出力は意図的。ただし不要な `t.Log` が残っている可能性は確認価値あり |
| Silent Skip | 1 | **有用** | `TestArgsInvalidRegex` での条件スキップ。`t.Skip` に変更すべき |

**savanna-smell-detector への改善提案（issue 候補）:**
1. Go の `if err != nil { t.Fatalf }` パターンを Conditional Test Logic から除外すべき（Go イディオム）
2. `t.Fatal` / `t.Fatalf` / `t.Error` / `t.Errorf` をアサーションとして認識すべき
3. カスタムアサーションヘルパー（`assertEqual` 等）を設定で登録できるとよい
4. JSON 出力で severity が全て 0 になるバグ（コンソール出力では正しく表示される）

## 完了済み
