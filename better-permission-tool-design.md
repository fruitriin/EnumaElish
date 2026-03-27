# Better Permission Tool 設計メモ

## 背景・モチベーション

### Claude Code の標準 permission system の問題

`settings.json` の `permissions` はコマンド先頭のプレフィックスマッチしかできない。

```json
{
  "permissions": {
    "allow": ["Bash(git log:*)"],
    "deny": ["Bash(rm:*)"]
  }
}
```

**見抜けないケース：**
```bash
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身
for f in $(cat targets.txt); do rm -rf $f; done  # ループ本体
files=$(dangerous_cmd); rm $files          # 動的展開
cmd1 && rm -rf foo                         # チェーン
```

### 実装の正体

プリミティブには `PreToolUse Hook` で：
- stdin: ツール情報JSON
- exit 0: 実行許可
- exit 2 + stderr: ブロック（理由をClaudeに返す）
- exit 0 + JSON stdout: 構造化制御（deny/allow/escalate）

---

## 先行プロジェクト調査

### claude-code-auto-approve（oryband）

- **設定フォーマット**: 既存 `settings.json` そのまま（独自DSLなし）
- **アプローチ**: `shfmt` でAST化 → `jq` で全サブコマンドを再帰展開 → 各セグメントを照合
- **対応**: `$()`, `<()`, サブシェル, `if/for/while` の中身
- **限界**: ルールの意味解析なし（`sed -i` と `sed -n` を区別しない）

### Dippy（ldayton）

- **設定フォーマット**: 独自テキストDSL
- **パーサー**: Pure Python 手書き再帰降下パーサー（Parable）、外部依存なし
- **特徴**: denyにメッセージを添付してAIに代替案を誘導できる

```
allow git
deny rm -rf "Use trash instead"
deny-redirect **/.env* "Never write secrets"
allow-mcp mcp__github__get_*
deny-mcp mcp__*__delete_* "Deletions need manual approval"
set default ask
```

- **last-rule-wins** セマンティクス
- **限界**: 制御構文・パイプのコンテキストを条件にするルールがない

### 両者に共通する未実装

「このコマンドがパイプの中にいるとき」「forループの本体にいるとき」という**構造的コンテキスト条件**がない。

---

## 本ツールのコアアイデア

### 実行トポロジーとDSL構造の対応

| シェル構文 | DSL上の扱い |
|---|---|
| `cmd \| cmd` | ネスト（親子関係） |
| `cmd >> file` | ネスト（親子関係） |
| `cmd && cmd` | リセット（トップレベルから再評価） |
| `cmd ; cmd` | リセット（トップレベルから再評価） |
| `find -exec` | カスタムネストルール（引数がコマンド） |
| `xargs` | カスタムネストルール |
| `bash -c` | カスタムネストルール |

`&&` と `;` のリセットが鍵。「実行が独立している」という意味論をそのまま設計に反映している。

### DSLスケッチ

```
allow find
  |,>>
    allow touch, cat
  |,>>
    deny rm  "don't combine find with rm"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch

allow grep
  |,>>
    allow wc, sort, head, tail

deny rm   # トップレベルの rm は deny
```

### `&&` のリセット動作

```
find . | rm   →  find のネストルールで rm → deny
find . && rm  →  && でリセット → rm をトップレベルで評価
```

---

## テンプレート・継承システム

### 発想

`find`, `xargs`, `grep` からの `|` に同じルールを継承したいのが自然。
テンプレートはコマンドと同じ階層に立ち、`next:` で委譲する。

### テンプレート定義

```
template primitive
  |,>>
    allow cat, echo, head, tail, wc

template bulkExec
  |,>>
    allow grep, sort, uniq, awk, sed
    deny rm  "don't pipe into destructive commands"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch
```

### コマンドルール

```
allow ls
  next: primitive

allow find
  next: bulkExec

allow xargs
  next: bulkExec

allow grep
  next: bulkExec
  |,>>
    next: primitive   # grep の出力は primitive だけ許可
```

### テンプレートの継承

```
template safeRead
  next: primitive

template bulkExec
  extends: safeRead   # primitive の許可を継承した上で
  |,>>
    allow grep, awk   # 追加で許可
    deny rm
```

---

## 監査出力（フラット展開）

設定の意図を検証するために、展開結果を一行ごとに出力する。

```
[allow]  ls
[allow]  ls | cat            (template: primitive)
[allow]  find
[allow]  find | grep         (template: bulkExec)
[deny]   find | rm           (template: bulkExec)       "don't pipe into destructive"
[deny]   find -exec rm       (template: bulkExec.exec)  "expand to tempfile first"
[---]    find && rm          (&&: reset → top-level rm rule)
--- 以降は組み合わせ打ち切り (depth > 2) ---
```

### 打ち切り設定

```
settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask
```

打ち切られた部分は監査出力に明示される。「ここから先は見ていない」が可視化される。

---

## 動的コマンドへの対応

### 方針

静的解析不能な動的展開はブロックして終わりにせず、**展開結果を一時ファイルに書き出して再実行を誘導する**。

```
Claude: files=$(find . -name "*.log"); rm $files
        ↓
Hook: 動的展開を検出
        ↓
展開結果を一時ファイルに書き出す
  /tmp/claude_expanded_abc123.txt:
    ./logs/app.log
    ./logs/error.log
        ↓
deny + reason:
  "動的コマンド展開は禁止です。
   展開結果を /tmp/claude_expanded_abc123.txt に書き出しました。
   以下のように書き直してください：
     xargs rm < /tmp/claude_expanded_abc123.txt"
```

Claudeは理由を読んで自律的に修正できる。「透明性を保ちながら作業継続」が目的。

### eval系の扱い

```
policy eval-to-file
  eval, bash -c, sh -c
    → write_tempfile + deny "rewrite as: bash {tempfile}"
```

### 解析不能パターン

```bash
eval "rm -rf foo"           # eval の中身は静的解析不能
bash -c "rm -rf foo"        # bash -c も同様
alias del='rm -rf'; del foo # エイリアス展開後の追跡
```

これらは「分からない → deny + 理由を出す」に徹する。

---

## ユーザーツールとしての位置づけ

### Anthropicが公式にできないこと

- `eval` を一時ファイル強制 → 余計なお世話になるユースケースがある
- `curl` を全ブロック → 正当な使い方が無数にある
- 特定ワークフローへの最適化 → 他の人には合わない

### ユーザーツールなら

```
allow curl
  args:
    -X GET:   allow      # 読み取りは通す
    -X POST:  ask        # 書き込みは確認
    | bash:   deny       # curl | bash は絶対駄目
    > *:      ask        # ファイル書き出しは確認
```

自分のワークフローに特化した「意見のある設定」が書ける。

### 二重の役割

このツールは「AIへの制限ツール」であると同時に「AIへの文脈説明ツール」でもある。denyのメッセージでClaudeに「なぜダメか・代わりに何をすべきか」を伝えられる。Anthropicのガードレールは汎用的すぎてこれができない。

---

## 設計の価値

「完璧なサンドボックス」ではなく「監査可能なセキュリティ」。

1. **誤検知（false positive）が減る** → 作業が止まらない
2. **ブロック時にClaudeが自律修正できる理由を渡す** → リトライが賢くなる
3. **静的解析の限界を明示する** → ユーザーが信頼できる

---

## Hook Type による構造化

### PreToolUse と PostToolUse の役割分担

| | PreToolUse | PostToolUse |
|---|---|---|
| タイミング | 実行前 | 実行後 |
| blockの意味 | ツールを止める | 次のアクションを誘導 |
| exit 2の効果 | ツール停止 | フィードバックのみ |

hookタイプをDSLの構造として持つことで、「このルールがどのタイミングで何をするか」が設定を読むだけで分かる。`--profile` で切り替えるより構造が明確。

### DSL構造

```
preToolUse
  find
    next: bulkExec
  rm
    action: deny
    message: "Use trash instead"

postToolUse
  WebFetch
    action: hint
    message: "結果をファイルに書き出してください"

  Skill(Hearing)
    action: hint
    message: "もう一度Hearingを実行してください"
    max_repeat: 3
    on_exceed:
      action: block
      message: "3回試行しました。別のアプローチを検討してください"
```

---

## アクション種別

exit codeとの対応：

| action | exit code | 効果 |
|---|---|---|
| `deny` | exit 2 + stderr | block + hint（ツール停止、Claudeに理由が届く） |
| `warn` | exit 0 + stdout | non-block hint（実行継続、Claudeに届く） |
| `allow` | exit 0 | そのまま通す |
| `ask` | （ユーザーへ委譲） | 標準のpermissionダイアログ |
| `hint` | exit 0 + stdout | PostToolUse用、次のアクションを誘導 |

`warn` と `hint` の違い：`warn` はPreToolUseで実行前に注意を促す、`hint` はPostToolUseで実行後に次の行動を誘導する。

### 記法の方向性

一行記法（`#`/`##`）は直感的だが拡張余地が詰まる。コマンドのプロパティとして表現する方が拡張しやすい：

```
deny rm
  mode: block       # or: hint
  message: "Use trash instead"

allow curl
  mode: warn
  message: "Check if WebFetch is sufficient"
```

---

## ターンカウント処理

`max_repeat` のような繰り返し制御は一時ファイルで実装できる。

**実装方針：**

1. PostToolUseフックで一時ファイルにカウントアップ
2. 次回実行時にカウンターを読んで閾値と比較
3. 閾値以上ならレスポンスを出す/出さないを制御

```bash
# 実装イメージ（Skill(Hearing) の場合）
COUNTER_FILE="/tmp/bpt_counter_Hearing"
COUNT=$(cat "$COUNTER_FILE" 2>/dev/null || echo 0)
COUNT=$((COUNT + 1))
echo $COUNT > "$COUNTER_FILE"

if [ $COUNT -lt 3 ]; then
  echo "もう一度Hearingを実行してください"
  exit 0  # hint
else
  echo "3回試行しました。別のアプローチを検討してください"
  exit 2  # block
fi
```

カウンターファイルはセッション開始時（SessionStart hook）にクリアするのが自然。

---

## 未解決事項

- `|>`, `<()` などあまり使われない構文の扱い
- カスタムネストルールの宣言を誰が持つか（組み込み vs ユーザー定義）
- フォーマット（YAML / TOML / 独自DSL）
  - TOML: フラット設定には強いがネストが深くなるとつらい
  - 独自テキストDSL: 監査出力と設定を同じ構文にできる可能性
- テンプレート継承とネスト構造の組み合わせの詳細
- `warn` のメッセージはstdoutに出すのかstderrに出すのか（Claudeへの見え方が変わる）
- `warn` を受け取ったClaudeが「無視してよい」と判断するかどうかはモデル次第
- ターンカウンターのスコープ（ツール単位 / セッション単位 / プロジェクト単位）
