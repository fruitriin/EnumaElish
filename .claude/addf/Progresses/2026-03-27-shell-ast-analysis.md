# 進捗表

## 運用ルール

### タスク開始時
1. `.claude/Feedback.md` を読み、前回の改善アクションで未対応のものがあれば考慮する
2. 以下の手順で Markdown チェックリストを作成する
   1. 1ショットで作業できる範囲にサブタスクを分割する
   2. 並行作業できる粒度でさらに分割する
   3. 各サブタスクにテスト作成・統合テスト・Lint・ビルドが必要か検討し、必要なら追加する
   4. 必要に応じて 2.1〜2.3 を再帰的に適用する

### 作業中
3. サブタスク着手時に `- [x]` でチェックしていく。並列可能なタスクはコンテナオーケストレーションを利用する
4. 実装フェーズの最終サブタスク完了時、実装で得た知見を `/addf-knowhow` で記録する（既存 knowhow の更新も含む）

### エージェント起動時の共通ルール
- エージェントチーム（TeamCreate）やサブエージェント（Agent）を作成するとき、各エージェントへのプロンプトに **最初に `/addf-knowhow-index` を実行する** よう指示を含めること
- これにより各エージェントがプロジェクトの知見ベースを把握した状態で作業を開始できる

### タスク完了時 — 品質検証

4. プロジェクトのビルド・Lint・テストコマンドを実行する
   - ADD フレームワークテスト: `bash .claude/tests/run-all.sh`
   - **失敗した場合 → 実装に差し戻す**。原因分析 → 修正 → 再実行
5. `addf-code-review-agent` でコードレビューを実施する
6. `addf-contribution-agent` で ADD フレームワークへのコントリビューション候補を検出する
7. レビュー指摘への対応:
   - **Critical/High**: 必ずこのフェーズ内で修正する（先送り禁止）
   - **Medium**: 原則修正。先送りする場合は独立計画を起こす
   - **Low/Info**: Plan に記録し、必要に応じて独立計画で対応
   - **バグ分離**: 発見されたバグが現在のプランと関心事が異なる場合は、修正せずに新しいプラン（`docs/plans/`）を書き起こし、`TODO.md` に追加するのみで現在のプランを完了させる
   - 修正後、ビルド・Lint・テストを再実行して通過を確認する

#### 完了処理

8. 投入されたタスクのPlanに実装完了状況を反映する
9. `.claude/Feedback.md` にPlan, TODO, Progress推進エンジンの問題の記録・改善アクションを追記する。反映済みの項目は削除する
10. `.claude/Feedback.md` にプロジェクト進行上の問題の記録・改善アクションを追記する。反映済みの項目は削除する
11. `.claude/Progresses/YYYY-MM-DD-プラン名.md` にリネームして移動し、`.claude/templates/ProgressTemplate.addf.md` から新規の Progress.md を作成する
12. Progress 推進エンジン自体に関するフィードバック・ノウハウがあれば、テンプレート（`.claude/templates/ProgressTemplate.addf.md`）の改善案を `.claude/Feedback.md` に記録する

13. コミットする

---

## タスク: Plan 0002 — ccchain シェルコマンド構造解析エンジン

### Phase 1: シェルパース基盤
- [x] `internal/shell/parse.go` — `mvdan.cc/sh` によるシェルコマンドパース（bash モード）

### Phase 2: トポロジー型定義と構築
- [x] `internal/shell/topology.go` — Topology/Segment/Command 型定義
- [x] トポロジー構築ロジック（パイプライン、リセットポイント、サブシェル）

### Phase 3: カスタムネストルール
- [x] `internal/shell/nestrules.go` — find -exec, xargs, bash -c, eval の検出

### Phase 4: 解析不能パターン検出
- [x] 変数展開・動的 eval の Analyzable フラグ処理

### Phase 5: テスト
- [x] `internal/shell/topology_test.go` — トポロジー構築テスト（テーブル駆動）
- [x] `internal/shell/nestrules_test.go` — カスタムネストルールテスト
- [x] `internal/shell/bench_test.go` — ベンチマーク

### Phase 6: 品質ゲート
- [x] `go test ./...` 全パス
- [x] `go vet ./...` パス
- [x] `go build ./cmd/ccchain` でバイナリ生成確認
- [x] ベンチマーク実行・結果確認（Parse: 1.7μs, Topology: 9.2μs, Nested: 6.5μs — 目標1ms大幅クリア）
