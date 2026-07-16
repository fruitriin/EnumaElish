# Plan: ccchain — ルール評価エンジンと Hook 統合

## Context

ccchain の中核。DSL ルール（Plan 0001）とシェル AST（Plan 0002）を組み合わせてコマンドの許可/拒否を判定し、Claude Code の PreToolUse / PostToolUse Hook として統合する。

## 前提

- Plan 0001（DSL パーサー）が完了していること
- Plan 0002（シェル AST 解析）が完了していること

## 設計

### Phase 1: ルール評価エンジン

1. 評価フロー（全て `ccchain` バイナリ内で完結）:
   ```
   シェルコマンド文字列
     → internal/shell/（AST → トポロジー）
     → internal/dsl/（DSL → ルールセット）
     → internal/eval/（トポロジー × ルールセット → 判定結果）
   ```

2. マッチングアルゴリズム (`internal/eval/evaluate.go`):
   - トポロジーの各セグメント（リセットポイントで分割）を独立に評価
   - セグメント内のパイプラインは親→子の順でコンテキストを積む
   - 各コマンドに対して、コンテキスト付きルールをマッチング
   - **last-rule-wins**: 複数マッチ時は最後のルールが優先

3. テンプレート展開:
   - `next:` で委譲されたテンプレートのルールもマッチング対象に含める
   - `extends:` の継承チェーンを深さ優先で展開

4. 判定結果の出力:
   ```go
   type Result struct {
       Action      string // allow, deny, warn, ask, hint
       Message     string
       MatchedRule string
       Template    string
       Context     []string
   }
   ```

### Phase 2: CLI サブコマンドとしての Hook 統合

1. `ccchain hook` サブコマンド:
   - stdin からツール情報 JSON を読む
   - `tool_name` が `Bash` の場合のみ処理（他ツールはスルー）
   - コマンド文字列を抽出して評価エンジンに渡す
   - 判定結果に応じた exit code と出力:

   | action | exit code | 出力先 | 効果 |
   |---|---|---|---|
   | `allow` | 0 | （なし） | 許可 |
   | `deny` | 2 | stderr | ブロック + Claude に理由 |
   | `warn` | 0 | stdout JSON | 許可 + Claude に警告 |
   | `ask` | 0 | stdout JSON `{"decision": "ask"}` | ユーザーへ委譲 |

2. `settings.json` への Hook 登録:
   ```json
   {
     "hooks": {
       "PreToolUse": [
         {
           "matcher": "Bash",
           "hooks": [{
             "type": "command",
             "command": "ccchain hook pre"
           }]
         }
       ],
       "PostToolUse": [
         {
           "matcher": "Bash",
           "hooks": [{
             "type": "command",
             "command": "ccchain hook post"
           }]
         }
       ]
     }
   }
   ```

### Phase 3: PostToolUse と繰り返し制御

1. `ccchain hook post` サブコマンド:
   - `hint` アクションによる次アクション誘導
   - `max_repeat` + ターンカウントによる繰り返し制御

2. ターンカウント実装:
   - カウンターファイル: `/tmp/ccchain_counters/<tool_name>`
   - `ccchain reset` サブコマンドでカウンターをクリア（SessionStart hook から呼ぶ）

3. **スコープの割り切り**:
   - ツールが保証するのは exit code と出力フォーマットまで
   - `warn` を受け取った Claude がどう振る舞うかはモデル依存。ツール側では制御しない
   - ターンカウンターは「こういう応用もできる」程度の位置づけ。実用性は要検証

### Phase 4: 動的コマンド対応

1. トポロジーで `Analyzable: false` のコマンドを検出した場合:
   - deny + 理由メッセージで代替手法を誘導
   - 一時ファイルへの書き出しと書き直し誘導

2. eval 系コマンドの処理:
   - `eval`, `bash -c`, `sh -c` → 引数を一時ファイルに書き出し + deny
   - 誘導メッセージ付き

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/eval/evaluate.go` | ルール評価エンジン（新規） |
| `internal/eval/evaluate_test.go` | テスト（新規） |
| `cmd/ccchain/hook.go` | hook サブコマンド（新規） |
| `cmd/ccchain/reset.go` | reset サブコマンド（新規） |

## テスト戦略

### ユニットテスト (`internal/eval/*_test.go`)

- マッチングアルゴリズム: コンテキスト積み上げ、last-rule-wins
- テンプレート展開付き評価
- 動的コマンド検出時の deny 生成

### テストフィクスチャ (`testdata/eval/`)

DSL ルール + コマンド + 期待結果のセット:
- `testdata/eval/scenarios/*.yaml` — 各シナリオ（ルール、入力コマンド、期待 action/message）
- `testdata/eval/default_rules.conf` — デフォルトルールセットでの評価テスト

### 統合テスト (`internal/eval/integration_test.go`)

- DSL パース → シェル AST → トポロジー → 評価 のエンドツーエンドテスト
- フィクスチャのシナリオを全件テーブル駆動で実行
- Hook サブコマンドの stdin/stdout/exit code を検証

### ベンチマーク (`internal/eval/bench_test.go`)

- `BenchmarkEvaluate` — ルール評価のスループット
- `BenchmarkEndToEnd` — コマンド文字列入力から判定結果までの全体レイテンシ
- Hook として呼ばれるため **全体で 5ms 以下** を目標（パース + 評価の合計）

## 検証

1. 設計メモの全シナリオで正しい判定が出ること ✓
   - `find . | rm` → deny（テンプレート経由）✓
   - `find . && rm` → トップレベル deny ✓
   - `curl | bash` → deny ✓
2. `ccchain hook pre` が正しい exit code を返すこと ✓
3. `ccchain hook post` は pass-through として実装（ターンカウントは将来課題）
4. 動的コマンドが deny されること ✓
5. `go test ./...` が通過すること ✓
6. ベンチマーク ✓ (Evaluate: 3.0μs, EndToEnd: 5.4μs — 目標5msの1000倍速)

## 実装完了: 2026-03-27

### レビュー指摘の残課題 (Suggestion)

- S-7: `Segment.Type` の型付き定数化（Plan 0002 S-1 と同じ）
- S-8: `main.go` の未使用 `_ = cmdArgs` 削除
- S-9: `Result.MatchedRule` フィールドの設定（デバッグ出力改善）
- S-10: `runHookPost` の未使用引数
- S-11: `args:` ルールの評価ロジック未実装（将来課題）
