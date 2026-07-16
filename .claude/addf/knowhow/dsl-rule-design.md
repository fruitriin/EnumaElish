# DSL ルール設計パターン

## 知見

### last-rule-wins の設計活用

ccchain のルール評価は **last-rule-wins**（最後にマッチしたルールが勝つ）。これを意図的に利用する設計:

```
# 1. デフォルトで allow
allow chmod

# 2. 危険パターンを後から deny で上書き
deny chmod
  args: 777
```

読み込み順序（`.ccchain.conf` → `.ccchain.local.conf` → グローバル）で後に読まれるルールが前のルールを上書きする。

### .conf と .local.conf の役割分担

| ファイル | 用途 | git | 言語 |
|---|---|---|---|
| `.ccchain.conf` | プロジェクト共有ルール | コミット対象 | 英語 |
| `.ccchain.local.conf` | 個人上書き | .gitignore | 任意（日本語等） |

`.local.conf` で deny メッセージを日本語に上書きする例:

```
deny eval
  "eval は静的解析できません。コマンドを直接記述してください"
```

### args: 正規表現の設計注意

**罠**: 範囲パターンが意図しないコマンドにマッチする

```
# 悪い例: 7[0-7][0-7]\s+/ は 755 /tmp/dir にもマッチ
deny chmod
  args: 7[0-7][0-7]\s+/

# 良い例: 777 にピンポイント
deny chmod
  args: 777
```

chmod ルールのデバッグで発見。`-R 777 /` を deny にしたかったが、パターン `7[0-7][0-7]\s+/` が `755 /tmp/dir` にもマッチし、last-rule-wins で deny が勝ってしまった。

**原則**: args: の正規表現はピンポイントで書く。範囲パターンは予期しないマッチを生む。

### args: + next: の組み合わせ

```
allow cat
  next: primitive
  args: \.(txt|md|json|yaml|yml|toml|go|js|ts)$
```

`next: primitive` はパイプやリダイレクトがないシンプルなコマンドのみ許可。`args:` と組み合わせることで「単純な cat で安全な拡張子のみ allow」のような精密な制御ができる。

### 複合サブコマンドの正規表現

`docker system prune` のようにスペースを含むサブコマンドを regex で扱うとき、`\b` 境界がスペース前で成立して意図通りにマッチしない。

```
# 悪い例: ^(run|exec|system prune)\b
# → "system" で \b が成立し、"system prune" 全体にマッチしない

# 対策: スペースを \s+ に正規化
# → ^(run|exec|system\s+prune)\b
```

`normalizeSubcommands` 関数で自動正規化している（`internal/semantics/generate.go`）。
