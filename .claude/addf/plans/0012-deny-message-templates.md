# Plan 0012: deny メッセージテンプレートと rewrite 誘導

## 背景

当初構想の核心: deny 時にただブロックするのではなく、**具体的な書き直し例を動的に生成**して Claude に渡す。現在の deny メッセージは静的文字列のみ。

## 設計

### メッセージテンプレート変数

```
deny rm
  message: "{command} は禁止。代わりに: find ... -print > /tmp/targets_{id}.txt && xargs rm < /tmp/targets_{id}.txt"

allow find
  args:
    -delete: deny  "find -delete は禁止。代わりに: {command_without '-delete'} -print > /tmp/targets_{id}.txt を実行してください"
```

利用可能な変数:

| 変数 | 展開結果 |
|---|---|
| `{command}` | 元のコマンド文字列全体 |
| `{cmd}` | コマンド名（`rm`, `find` 等） |
| `{args}` | 引数部分 |
| `{id}` | ユニーク ID（一時ファイル名に使用） |
| `{timestamp}` | タイムスタンプ |
| `{cwd}` | `$CLAUDE_PROJECT_DIR` |

### expand_and_pause 戦略（将来拡張）

当初構想の最もユニークなアイデア: 動的コマンドを実際に安全に展開し、結果を一時ファイルに書き出してからリトライを誘導する。

```
# 将来構文のスケッチ
policy expand-and-rewrite
  trigger: dynamic_command  # $VAR, $(cmd) を含むコマンド
  action:
    expand_to: /tmp/ccchain_{id}.txt
    deny_message: "動的コマンドを検出。展開結果を {expand_to} に書き出しました。確認して書き直してください"
```

これは ccchain 自身がコマンドを**実行する**必要があるため、PreToolUse hook の責務を超える。PostToolUse hook との連携、またはラッパースクリプトとして実装する方が適切。Plan のスコープ外だがアイデアとして記録。

## 変更対象

| ファイル | 変更 |
|---|---|
| `internal/eval/evaluate.go` | Result.Message のテンプレート展開 |
| `internal/eval/template.go` | テンプレートエンジン（新規） |
| デフォルトルールセット | テンプレート変数を使った deny メッセージに更新 |

## 実装量: 小（テンプレート展開は strings.Replace 程度）
