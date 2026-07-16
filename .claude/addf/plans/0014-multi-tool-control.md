# Plan 0014: Bash 以外のツール制御（Read/Edit/WebFetch/MCP）

## 背景

ccchain は現在 Bash ツールのみを対象にしているが、Claude Code の PreToolUse hook は**全ツール**に対して発火する。Read/Edit/Write/WebFetch/MCP ツールにもルールを適用できれば、より包括的な制御が可能。

## ユースケース

### Read/Edit のパス制御

```
preToolUse
  Read
    scope:
      inside: allow
      outside: ask  "workspace 外の読み取り"
    args:
      \.env|\.ssh|credentials: deny  "機密ファイルの読み取りは禁止"

  Edit
    scope:
      inside: allow
      outside: deny  "workspace 外の編集は禁止"
    args:
      \.claude/settings: deny  "settings の直接編集は禁止"

  Write
    scope:
      inside: allow
      outside: deny  "workspace 外への書き込みは禁止"
```

### WebFetch の URL 制御

```
preToolUse
  WebFetch
    args:
      ^https://: allow
      ^http://: ask  "HTTP (非暗号化) アクセスです"
      localhost|127\.0\.0\.1|0\.0\.0\.0: ask  "ローカルサービスへのアクセス"
      \.internal\.|\.corp\.: deny  "内部ネットワークへのアクセスは禁止"
```

### MCP ツールの制御

```
preToolUse
  mcp__github__create_issue
    action: ask  "GitHub issue の作成は確認が必要"

  mcp__slack__post_message
    action: ask  "Slack メッセージの送信は確認が必要"

  mcp__*__delete_*
    action: deny  "MCP 経由の削除操作は禁止"
```

## 実装

### Phase 1: hook 入力のツール種別判定

現在の `runHookPre` は `tool_name == "Bash"` のみ処理。これを拡張:

```go
switch ti.ToolName {
case "Bash":
    evaluateBashCommand(bi.Command, cfg)
case "Read", "Edit", "Write":
    evaluateFileAccess(ti.ToolName, ti.Input, cfg)
case "WebFetch":
    evaluateWebFetch(ti.Input, cfg)
default:
    if strings.HasPrefix(ti.ToolName, "mcp__") {
        evaluateMCPTool(ti.ToolName, ti.Input, cfg)
    }
}
```

### Phase 2: DSL のツール種別セクション

```
preToolUse
  Bash
    # 既存のルール
  Read
    # Read 用ルール
  WebFetch
    # WebFetch 用ルール
```

### 設計上の注意

- Read/Edit/Write の input にはファイルパスが含まれる → scope 判定（Plan 0011）と自然に統合
- WebFetch の input には URL が含まれる → args: の regex で制御可能
- MCP ツールは名前のワイルドカードマッチが必要（`mcp__*__delete_*`）

## 実装量: 中（hook 分岐 + DSL パーサー拡張 + ツール別 evaluator）

## 実装完了: 2026-03-28 (Phase 1)

Phase 1 完了:
- hook.go: Bash/Read/Edit/Write/WebFetch/MCP ツールの分岐処理
- eval/tool.go: EvaluateTool（ツール名マッチ + args: 評価 + MCP ワイルドカード）
- テスト 7 件

Phase 2（DSL ツール種別セクション構文）は将来課題。
