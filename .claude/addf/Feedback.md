# Process Feedback

開発プロセスの振り返りと改善を記録する。

## 記録方法

タスク完了時や問題発生時に、以下のいずれかのセクションに追記する。

## オーナーフィードバック

## 問題の記録

- `go get mvdan.cc/sh/v3` が Go 1.25.0 を要求し、.tool-versions で設定した 1.24.10 からの自動アップグレードが発生した。Go バージョン管理に注意

## 改善アクション

### 2026-07-17 — Plan 0022/0025 完了サイクルからのプロセス知見

- **worktree 並列委譲時の base 経過差問題**: 3体を並列 worktree で立ち上げた後、Phase 0/1/2 が main に反映されるタイミングと、Phase 3/4/0025 の worktree base（v0.1.0 直後）の間で時差が発生し、統合時に手動衝突解消が3件必要になった。Plan 実装エージェントの委譲プロンプトに「実装済み前提は grep で確認する」を明記すると安全（実際 Phase 4 エージェントは grep で気づき、unattended: の直接記述を deny に切り替えて回避した）
- **レビュー scratch ファイルの残置**: skeptic/attacker のレビューエージェントが実 E2E 再現のため一時テストファイル (`zz_*_test.go`, `attackerscratch/`) をリポジトリ内に作成したが、リポジトリ自身の ccchain hook が `rm` を拒否するため掃除できず未追跡ファイルとして残った。委譲プロンプトに「scratch は `t.TempDir()` を使い、リポジトリファイルは作らない」を明記する
- **修正エージェント集約運用**: Stage 2 レビュー 4 体を並列起動した後、Critical 8 + High 4 + attacker 追加 3 を 1 体の修正エージェントにまとめて委譲した。SendMessage で追加指示（attacker 分）を送る運用は機能したが、初回プロンプトが長大化した。次回は「初回プロンプトで既知の Critical 群 → 別レビュー到着時に SendMessage で追加」のプロンプトテンプレを作ると再現性が上がる
- **自己ホスト環境の hook が rm を deny する副作用**: リポジトリ自身が dogfooding で ccchain hook を有効化しており、レビュー・修正エージェントが自分の scratch を掃除できないケースが頻発した。auto モード下の rm は sentinel の unattended: deny で降格されるのが仕様通り = 保護機構が動いている証拠だが、開発者体験としてはノイズ。エージェント委譲時に「.claude/settings.local.json で hook を一時無効にした worktree で作業する」オプションを検討する

## 完了済み

### 2026-07-05 — CLAUDE.repo.md のテストセクションに Go テスト明記

- **元の改善アクション**: ADDF テストランナー (`bash .claude/addf/tests/run-all.sh`) と Go テスト (`go test ./...`) が共存する構成。CLAUDE.repo.md のテストセクションに Go テストも明記すべき
- **対応済み**: CLAUDE.repo.md の「テスト」セクションに Go テスト（`go test ./...` / `go vet ./...` / `go build ./cmd/ccchain`）と ADDF テスト（`bash .claude/addf/tests/run-all.sh`）の両方を明記済み

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
