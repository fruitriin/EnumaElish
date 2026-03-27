# Plan 0013: コマンドセマンティクステーブル

## 背景

Dippy は 45+CLI ツールの意味解析（`sed -i` vs `sed -n` の区別等）を持つ。ccchain の `args:` は regex パターンマッチだが、コマンドごとの「このオプションは破壊的」「このサブコマンドは安全」という知識を組み込みテーブルとして持てば、デフォルトルールの品質が飛躍的に向上する。

## 設計

### 組み込みセマンティクステーブル

```go
// internal/semantics/table.go
type CommandSemantics struct {
    DestructiveArgs []string  // これらの引数があれば破壊的
    SafeSubcommands []string  // これらのサブコマンドは安全
    DangerousSubcommands []string
    WritesFiles     bool      // ファイル書き込みを行うか
    ExecutesCode    bool      // コード実行が可能か
}

var Table = map[string]CommandSemantics{
    "sed": {
        DestructiveArgs: []string{"-i", "--in-place"},
    },
    "chmod": {
        DestructiveArgs: []string{"-R", "--recursive"},
    },
    "git": {
        SafeSubcommands: []string{"status", "log", "diff", "show", "branch", "tag", "stash", "ls-files"},
        DangerousSubcommands: []string{"filter-branch", "filter-repo", "config"},
    },
    "go": {
        SafeSubcommands: []string{"test", "vet", "build", "mod", "version", "fmt", "doc", "env"},
        DangerousSubcommands: []string{"run", "generate"},
        ExecutesCode: true,
    },
    "npm": {
        SafeSubcommands: []string{"test", "run", "version", "ls", "outdated", "audit"},
        DangerousSubcommands: []string{"install", "publish"},
        ExecutesCode: true,
    },
    "docker": {
        DangerousSubcommands: []string{"run", "exec", "build"},
        ExecutesCode: true,
    },
    "kubectl": {
        SafeSubcommands: []string{"get", "describe", "logs"},
        DangerousSubcommands: []string{"delete", "exec", "apply"},
    },
}
```

### DSL との統合

セマンティクステーブルは `args:` ルールを**自動生成**するのに使える:

```
# ccchain generate-rules --from-semantics
# → 以下を .ccchain.conf に出力

allow sed
  args:
    -i|--in-place: ask  "sed -i modifies files in place"

allow git
  args:
    ^(status|log|diff|show|branch|tag|stash|ls-files)\b: allow
    ^(filter-branch|filter-repo)\b: deny  "arbitrary code execution"
    ^config\b: ask  "git config can execute code"
```

### `ccchain generate-rules` サブコマンド

```bash
ccchain generate-rules --from-semantics
# → セマンティクステーブルから args: ルールを生成して stdout に出力
# ユーザーが .ccchain.conf にコピーして使う
```

## 実装量: 中（テーブル定義 + generate-rules サブコマンド）
