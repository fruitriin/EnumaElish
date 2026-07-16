# Plan 0011: ワークスペーススコープによるパスベースアクセス制御

## 背景

プログラミング用に `~/workspace` のようなディレクトリを割り当て、`~/` にはあまりアクセスさせたくないユースケース。

現状の ccchain はコマンド名とパイプ構造で判定するが、コマンドの**引数に含まれるパス**を見ていない。
`cat ~/workspace/README.md` と `cat ~/.ssh/id_rsa` が同じ判定になる。

## 設計

### DSL 拡張: `scope` ディレクティブ

```
scope:
  workspace: ~/workspace
  # 複数指定可能
  # workspace: ~/projects, ~/workspace

# スコープ外パスへのアクセスを制御
allow cat
  scope:
    inside: allow
    outside: ask  "workspace 外のファイル読み取りです"

allow cp
  scope:
    inside: allow            # workspace 内の cp は自由
    outside-read: allow      # workspace 外からの読み取りは OK
    outside-write: ask       # workspace 外への書き込みは確認

allow rm
  scope:
    inside: ask  "confirm file deletion"
    outside: deny  "workspace 外のファイル削除は禁止"

allow find
  scope:
    inside: allow
    outside: ask  "workspace 外の検索です"

allow ls
  scope:
    inside: allow
    outside: allow  # ls はどこでも安全
```

### 実装アプローチ

#### Phase 1: パス引数の抽出

`shell.Command.Args` からパス引数を抽出する。パス判定のヒューリスティック:

```go
func looksLikePath(arg string) bool {
    return strings.HasPrefix(arg, "/") ||
           strings.HasPrefix(arg, "~/") ||
           strings.HasPrefix(arg, "./") ||
           strings.HasPrefix(arg, "../") ||
           strings.Contains(arg, "/")
}
```

**限界（明示すべき）:**
- `$HOME/file` — 変数展開は解決不能（既存の動的引数スキップで対応）
- 相対パス `file.txt` — CWD が不明なので scope 判定不能（inside 扱い）
- シンボリックリンク — 解決しない（静的解析の限界）

#### Phase 2: scope 判定ロジック

```go
type ScopeResult int
const (
    ScopeInside  ScopeResult = iota // workspace 内
    ScopeOutside                     // workspace 外
    ScopeUnknown                     // 判定不能（相対パス等）
)

func classifyPath(path string, workspacePaths []string) ScopeResult {
    // ~ を展開
    expanded := expandTilde(path)
    for _, ws := range workspacePaths {
        if strings.HasPrefix(expanded, ws) {
            return ScopeInside
        }
    }
    if strings.HasPrefix(expanded, "/") || strings.HasPrefix(path, "~/") {
        return ScopeOutside
    }
    return ScopeUnknown // 相対パスは判定不能
}
```

#### Phase 3: 評価エンジンへの組み込み

`applyArgsRules` の後、`applyScopeRules` で scope 判定を適用:

```go
func applyScopeRules(cmd *shell.Command, rule *dsl.Rule, config *dsl.Config, baseResult *Result) *Result {
    if rule.ScopeRules == nil || config.Settings.WorkspacePaths == nil {
        return baseResult
    }

    paths := extractPathArgs(cmd.Args)
    if len(paths) == 0 {
        return baseResult // パス引数なし → scope 判定しない
    }

    for _, path := range paths {
        scope := classifyPath(path, config.Settings.WorkspacePaths)
        switch scope {
        case ScopeOutside:
            if rule.ScopeRules.Outside != nil {
                return &Result{Action: rule.ScopeRules.Outside.Action, ...}
            }
        case ScopeInside:
            if rule.ScopeRules.Inside != nil {
                return &Result{Action: rule.ScopeRules.Inside.Action, ...}
            }
        }
    }
    return baseResult
}
```

#### Phase 4: 読み取り/書き込みの区別

一部のコマンドは引数の位置で読み取り/書き込みを区別できる:

| コマンド | 読み取り引数 | 書き込み引数 |
|---|---|---|
| `cp src dst` | src (最後以外) | dst (最後) |
| `mv src dst` | src (最後以外) | dst (最後) |
| `cat file` | file (全て) | — |
| `rm file` | — | file (全て = 削除) |
| `tee file` | — | file (全て) |

これはコマンドごとのセマンティクス定義が必要で、組み込みテーブルで対応:

```go
var commandArgSemantics = map[string]ArgSemantics{
    "cat":  {AllRead: true},
    "head": {AllRead: true},
    "tail": {AllRead: true},
    "rm":   {AllWrite: true},
    "cp":   {LastWrite: true},  // cp src... dst
    "mv":   {LastWrite: true},  // mv src... dst
    "tee":  {AllWrite: true},
}
```

ただし**計算量の懸念**: コマンドセマンティクステーブルの肥大化。最初は read/write 区別なしで `inside`/`outside` のみ実装し、需要に応じて拡張。

### デフォルトルール例

```
scope:
  workspace: ~/workspace

# 読み取り系 — workspace 外は ask
allow cat
  scope:
    inside: allow
    outside: ask  "workspace 外のファイルです"

allow head
  scope:
    inside: allow
    outside: ask

# 書き込み系 — workspace 外は deny
allow cp
  scope:
    inside: allow
    outside: deny  "workspace 外へのコピーは禁止"

allow rm
  scope:
    inside: ask  "confirm file deletion"
    outside: deny  "workspace 外のファイル削除は禁止"

# 検索系 — workspace 外は ask
allow find
  scope:
    inside: allow
    outside: ask  "workspace 外の検索です"

# ls はどこでも安全
allow ls
  scope:
    inside: allow
    outside: allow

# 書き出し系の deny
allow curl
  scope:
    outside: deny  "workspace 外へのダウンロードは禁止"
  args:
    -o\b|--output: ask  "ファイル書き出しは確認が必要"
```

### セキュリティ上の考慮（セキュリティレビュー反映済み）

1. **パストラバーサル対策 [Critical]**: `..` を含む相対パスは強制的に `ScopeOutside` として扱う。`../../etc/passwd` が ScopeUnknown → inside → allow になるバイパスを防止
2. **ScopeUnknown ポリシー**: デフォルトは `outside`（fail-closed）に変更。`..` を含まない純粋な相対パス（`file.txt`）のみ inside 扱い
3. **Tilde 展開バイパス防止 [High]**: `~/workspace2/../.ssh/id_rsa` が `~/workspace` のプレフィックスに誤マッチしないよう、`filepath.Clean` で正規化 + トレイリングスラッシュ付き比較
4. **複数パス引数 [Medium]**: 全パスを評価し最も制限的な結果を返す（最初の一致で返さない）
5. **環境変数パス**: `$HOME/file` は動的引数として args: 評価をスキップ → 親ルールのアクション
6. **scope なしのルール**: scope ディレクティブがない場合は従来通りの動作（後方互換）
7. **Read/Edit/Write ツールの限界**: 本 Plan は **Bash コマンドの引数のみ** が対象。`Read ~/.ssh/id_rsa` のような直接的なツール呼び出しには Plan 0014（マルチツール制御）が必要。この限界をドキュメントに明記すること

### CWD の取得

Claude Code の hook JSON にはツール情報のみ含まれ、CWD は直接取得できない。
ただし `$CLAUDE_PROJECT_DIR` 環境変数が利用可能。これを workspace 判定のヒントにできる:

```go
// CWD が不明な場合、CLAUDE_PROJECT_DIR を workspace の一部と仮定
projectDir := os.Getenv("CLAUDE_PROJECT_DIR")
```

## 変更対象ファイル

| ファイル | 変更 |
|---|---|
| `internal/dsl/ast.go` | ScopeRule, WorkspacePaths 型追加 |
| `internal/dsl/parser.go` | `scope:` ディレクティブのパース |
| `internal/eval/evaluate.go` | `applyScopeRules` 関数 |
| `internal/eval/scope.go` | パス抽出、scope 判定ロジック（新規） |
| `internal/eval/scope_test.go` | パス判定テスト |
| `cmd/ccchain/init_cmd.go` | デフォルトルールに scope 例を追加 |
| `docs/reference/dsl.md` | scope 構文リファレンス |

## 段階的実装

### v1（最小）: inside/outside の2値判定のみ
- `scope: workspace: ~/workspace` の宣言
- パス引数の抽出（ヒューリスティック）
- inside/outside 判定
- read/write 区別なし

### v2（拡張）: read/write 区別
- コマンドセマンティクステーブル
- `outside-read` / `outside-write` の分離

### v3（将来）: 複数スコープ
- `scope: trusted: ~/workspace, ~/projects`
- `scope: sensitive: ~/.ssh, ~/.gnupg`
- `sensitive: deny` のような明示的保護

## コラム: CWD のワークスペース逸脱防止

ccchain のスコープ外だが、関連テクニックとして記録。

Claude Code は `cd` でカレントディレクトリを変更できる。`cd ..` を繰り返してワークスペース外に出た状態で相対パスコマンドを実行されると、ccchain の scope 判定は「CWD 不明 → inside 扱い（fail-open）」になる。

シェル環境側の対策として、fish の `--on-variable PWD` や bash の `PROMPT_COMMAND` で CWD を監視し、`$CLAUDE_PROJECT_DIR` より上に出たら自動で戻す function を設定する方法がある:

```fish
# fish の例
function __enforce_workspace --on-variable PWD
    if not string match -q "$CLAUDE_PROJECT_DIR*" "$PWD"
        cd $CLAUDE_PROJECT_DIR
        echo "workspace 外への cd を検出。$CLAUDE_PROJECT_DIR に戻しました。" >&2
    end
end
```

これは ccchain ではなくシェル環境の設定で対応すべきもの。ccchain は渡されたコマンド文字列の静的解析に徹する。

## 検証

1. `cat ~/workspace/file` → allow, `cat ~/.ssh/id_rsa` → ask/deny
2. `find ~/workspace` → allow, `find ~` → ask
3. 相対パス `cat file.txt` → inside（fail-open）
4. `$HOME/file` → 動的引数スキップ
5. scope なしのルール → 従来動作
6. 統合テスト全パス
7. ベンチマーク回帰なし
