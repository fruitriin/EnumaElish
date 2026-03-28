# Plan 0017: Bash コマンド引数へのワークスペーススコープ適用

## 背景

Plan 0011 でワークスペーススコープを実装したが、Read/Edit/Write ツールにのみ適用されている。Bash コマンドの引数に含まれるファイルパスにはスコープが効かない。

```
settings:
  workspace: ~/workspace, /tmp/hogehoge

# 現状:
# Read /etc/shadow → ask (スコープで保護される)
# cat /etc/shadow  → allow (Bash eval ではスコープ未適用)
```

## 設計

### 評価フロー

```
Bash コマンド文字列
  → shell.BuildTopology → Topology
  → eval.Evaluate → ルールマッチ → Result
  → 【新規】applyScopeToCommand → パス引数を抽出してスコープ判定
  → 最終 Result
```

### 実装

`evaluate.go` の `matchCommand` 結果に対して、コマンド引数のパスをスコープ判定する。

```go
func applyScopeToCommand(cmd *shell.Command, config *dsl.Config, baseResult *Result) *Result {
    if config.Settings == nil || len(config.Settings.WorkspacePaths) == 0 {
        return baseResult
    }

    // パス引数を抽出
    paths := ExtractPathArgs(cmd.Args)
    if len(paths) == 0 {
        return baseResult
    }

    // 全パスを評価し、最も制限的なスコープ結果を採用
    worstScope := ScopeInside
    for _, path := range paths {
        scope := ClassifyPath(path, config.Settings.WorkspacePaths)
        if scope == ScopeOutside {
            worstScope = ScopeOutside
        }
    }

    // outside + allow → ask にエスカレーション
    if worstScope == ScopeOutside && baseResult != nil && baseResult.Action == dsl.ActionAllow {
        return &Result{
            Action:  dsl.ActionAsk,
            Message: "workspace scope: command accesses path outside workspace",
            Context: baseResult.Context,
        }
    }

    return baseResult
}
```

### 適用箇所

1. `evaluateSegment` の single command 評価後
2. `evaluatePipeline` の各コマンド評価後
3. ネストコマンド（find -exec 等）の評価後

### セキュリティレビュー反映

- `..` を含むパスは ScopeOutside 強制（Plan 0011 で対策済み）
- 複数パス引数は全評価して最も制限的な結果を返す
- 動的引数（`$VAR`）はパス抽出時にスキップ（`looksLikePath` で `$` 含むものを除外）
- outside の扱い: 現在は ask エスカレーション。将来的に `scope_violation: deny` オプション対応可能

### 例

```
settings:
  workspace: ~/workspace, /tmp/hogehoge

allow cat
allow cp

# 動作:
# cat ~/workspace/file     → allow (inside)
# cat /etc/passwd          → ask   (outside → エスカレーション)
# cat /tmp/hogehoge/data   → allow (inside)
# cp file /etc/cron.d/     → ask   (dst が outside)
# cat ../../.ssh/id_rsa    → ask   (.. → ScopeOutside)
```

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/eval/evaluate.go` | `applyScopeToCommand` を matchCommand 後に適用 |
| `internal/eval/scope.go` | `ExtractPathArgs` に `$` 含むパスの除外を追加 |
| `internal/eval/evaluate_test.go` | Bash スコープテスト追加 |
| `testdata/eval/commands.txt` | スコープ対象コマンドを追加 |

## 検証

1. `cat ~/workspace/file` → allow (inside)
2. `cat /etc/passwd` → ask (outside)
3. `cat /tmp/hogehoge/data` → allow (inside, 複数 workspace)
4. `cp file /etc/cron.d/` → ask (dst が outside)
5. `cat ../../.ssh/id_rsa` → ask (.. トラバーサル)
6. `cat $HOME/file` → allow (動的パスはスキップ → 親ルールのまま)
7. 既存統合テスト全パス
