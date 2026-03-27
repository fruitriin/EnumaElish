# DSL 構文リファレンス

ccchain はインデントベースのテキスト DSL を使用します。

```
# コメント

# トップレベルルール
<action> <command>[, command2, ...] ["message"]
  |,>>
    <action> <command> ["message"]
  exec:
    <action> <command> ["message"]
  args:
    <pattern>: <action>
  next: <template_name>

# テンプレート
template <name>
  extends: <parent>

# Hook セクション
preToolUse
  # ルール群
postToolUse
  # ルール群

# 設定
settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask
```

完全なリファレンスは[英語版](/reference/dsl)を参照してください。
