# Plan 0015: プロジェクト自動検出によるルール推定

## 背景

当初構想: プロジェクトの `package.json`, `Cargo.toml`, `Makefile`, `go.mod` 等からビルド・テストコマンドを推定し、初期ルールセットを自動生成する。

## 設計

### `ccchain detect` サブコマンド

```bash
ccchain detect
# → プロジェクトファイルを検出して推奨ルールを出力
```

### 検出ロジック

```go
type ProjectType struct {
    Name       string
    Indicators []string          // 存在判定ファイル
    Rules      []SuggestedRule   // 推奨ルール
}

var projectTypes = []ProjectType{
    {
        Name:       "Go",
        Indicators: []string{"go.mod", "go.sum"},
        Rules: []SuggestedRule{
            {Rule: "allow go", Args: map[string]string{
                `^(test|vet|build|mod|version|fmt|env|doc)\b`: "allow",
                `^(run|generate)\b`: "ask",
            }},
        },
    },
    {
        Name:       "Node.js",
        Indicators: []string{"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml"},
        Rules: []SuggestedRule{
            {Rule: "allow npm", Args: ...},
            {Rule: "allow node", Note: "# ask node -e/--eval"},
            {Rule: "allow npx"},
        },
    },
    {
        Name:       "Rust",
        Indicators: []string{"Cargo.toml", "Cargo.lock"},
        Rules: []SuggestedRule{
            {Rule: "allow cargo"},
            {Rule: "allow rustc"},
        },
    },
    {
        Name:       "Python",
        Indicators: []string{"pyproject.toml", "setup.py", "requirements.txt", "Pipfile"},
        Rules: []SuggestedRule{
            {Rule: "allow pip"},
            {Rule: "allow uv"},
            {Rule: "# ask python3  # -c can execute arbitrary code"},
        },
    },
    {
        Name:       "Ruby",
        Indicators: []string{"Gemfile", "Gemfile.lock"},
        Rules: []SuggestedRule{
            {Rule: "allow bundle"},
            {Rule: "allow rake"},
        },
    },
    {
        Name:       "Docker",
        Indicators: []string{"Dockerfile", "docker-compose.yml", "compose.yml"},
        Rules: []SuggestedRule{
            {Rule: "ask docker"},
            {Rule: "ask docker-compose"},
        },
    },
    {
        Name:       "Make",
        Indicators: []string{"Makefile", "GNUmakefile"},
        Rules: []SuggestedRule{
            {Rule: "ask make  # Makefile targets can run anything"},
        },
    },
}
```

### 出力例

```bash
$ ccchain detect

# Detected: Go project (go.mod)
# Detected: Make (Makefile)

# Suggested rules for .ccchain.conf:

allow go
  args:
    ^(test|vet|build|mod|version|fmt|env|doc)\b: allow
    ^(run|generate)\b: ask  "go run/generate can execute arbitrary code"

ask make  # Makefile targets can run anything

# Review these suggestions, then append to .ccchain.conf
```

### Makefile ターゲットの解析（オプション）

```bash
# Makefile のターゲット名を抽出
make -qp 2>/dev/null | grep -E '^[a-zA-Z0-9_-]+:' | cut -d: -f1
```

安全そうなターゲット（`build`, `test`, `lint`, `clean`）を推定:

```
allow make
  args:
    ^(build|test|lint|check|fmt|vet)\b: allow
    ^(clean|install|deploy|release)\b: ask
```

### `ccchain init --detect` との統合

```bash
ccchain init --detect
# → デフォルトルール + 検出されたプロジェクトルールを含む .ccchain.conf を生成
```

## 実装量: 小〜中（ファイル存在チェック + テンプレート出力）
