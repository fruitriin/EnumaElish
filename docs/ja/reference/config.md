# 設定ファイル リファレンス

## 探索順序

ccchain は優先度順に設定ファイルを探します。後のファイルのルールが追加され、last-rule-wins で上書き可能です:

| 優先度 | パス | 用途 |
|---|---|---|
| 1 | `.ccchain.conf` | プロジェクト共有ルール（git にコミット） |
| 2 | `.ccchain.local.conf` | 個人用上書き（.gitignore 対象） |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | Claude Code のグローバル設定 |
| 4 | `~/.claude/ccchain.conf` | フォールバックグローバル設定 |

`--config <path>` で検索をスキップし特定ファイルを直接指定できます。

## マージ動作

複数ファイルが見つかった場合:
- **テンプレート**: 全ファイルから収集（同名テンプレートはエラー）
- **ルール**: 検索順に追加（last-rule-wins で上書き可能）
- **Settings**: 最後の `settings:` ブロックが優先

## Hook 登録

### PreToolUse

`.claude/settings.json` に追加:

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
    ]
  }
}
```

### PostToolUse（オプション）

```json
{
  "hooks": {
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

## Hook 入力フォーマット

ccchain は Claude Code の PreToolUse Hook フォーマットに従った JSON を stdin から読みます:

```json
{
  "tool_name": "Bash",
  "tool_input": {
    "command": "find . -name '*.log' | rm -rf"
  },
  "permission_mode": "auto",
  "session_id": "01HXYZ...",
  "cwd": "/home/user/project"
}
```

- `permission_mode` — `default` / `acceptEdits` / `plan` / `auto` / `dontAsk` / `bypassPermissions` のいずれか。非対話モードで `ask` を降格させる [`ask_strategy`](./dsl.md#ask_strategy) の分岐入力に使う。未知の値・欠落は非対話扱い（安全側）
- `session_id` — Claude Code のセッション ID。`cwd` と併用して [`ccchain approve`](./approve.md) トークンの既定スコープを決める
- `cwd` — Claude Code がツールを呼び出したときのカレントディレクトリ

Bash 以外のツール（`Read` / `Edit` / `Write` / `WebFetch` / `mcp__*`）も同じ hook 経由でルーティングされ、ccchain はファイルパス・URL を tool ルールで評価する。未知のツール名はそのまま通過する（exit 0）。

## Hook 出力

### PreToolUse

すべてのアクションは exit code `0` を返し、Claude Code の `hookSpecificOutput` スキーマに沿った JSON を出力します:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "ccchain: curl | bash はレビュー無しでリモートコードを実行します。まず `curl -o /tmp/install.sh` で保存 → 中身を確認 → 明示的に `bash /tmp/install.sh` を実行してください。"
  }
}
```

| 評価結果 | `permissionDecision` | 備考 |
|---|---|---|
| `allow` | (出力なし) | ccchain は中立を保ち、Claude Code の後続パーミッションレイヤの評価を妨げない |
| `deny` | `deny` | `permissionDecisionReason` にメッセージ、ask→deny 降格経路では承認手順も含まれる |
| `warn` | `allow` | reason は Claude のコンテキストに注意文として残る。実行はブロックされない |
| `hint` | `allow` | warn と同形式（従来は出力なしで無言だったが、hint テキストを届けるようになった） |
| `ask` | `ask` | 対話モードでのみ。`auto` / `dontAsk` / 未知モードでの降格は [`ask_strategy`](./dsl.md#ask_strategy) を参照 |

> **変更履歴。** ccchain は以前、`deny` を `exit 2` + stderr メッセージで返し、`hint` は無出力でした。両者とも上表の exit-0 + `hookSpecificOutput` JSON に置き換わり、現在の Claude Code hooks 仕様に準拠しています。stderr パースや旧 `{"decision": "ask"}` 形式を扱っていた場合は本表の各フィールドへ移行してください。

## エラー処理（Fail-Open）

ccchain がエラーに遭遇した場合（JSON 不正、パース失敗、設定エラー）、コマンドは**許可**されます（exit 0）。エラーは stderr にログ出力されます。

これにより ccchain 自身のバグが Claude の操作をブロックすることはありません。

**設計上の意図:** ccchain は「完全なサンドボックス」ではなく「監査可能なセキュリティ」を目指しています。fail-closed 設計では、ccchain のバグ・設定の typo・環境要因のいずれでも Claude の操作が完全に停止してしまいます。fail-open はこのトレードオフを受け入れる選択です:

- エラーは stderr にログ出力される（Claude Code の出力で確認可能）
- `ccchain check` コマンドで使用前に設定を検証できる
- `ccchain audit` でルールの全展開を確認できる

**リスク:** 設定ファイルが欠損・破損していると、すべてのコマンドが許可されます。設定変更後は必ず `ccchain check` を実行してください。

**fail-closed へのオプトイン:** 設定ロードエラー時に deny させたい場合は、
先に読み込めた設定ファイル（例: グローバル `~/.claude/ccchain.conf`）で
[`settings.strict_config_error: true`](./dsl.md#strict_config_error) を
指定します。設定ファイルが 1 つも読めない場合のフォールバックとして
`CCCHAIN_STRICT_CONFIG_ERROR=1` を export する opt-in パスも用意しています。

> **警告 — self-DoS リスク。** `strict_config_error` が有効で、hook が
> （Bash だけでなく）全ツールに配線されている場合、パースに失敗する
> `.ccchain.conf` は **すべての** PreToolUse 呼び出しをブロックします —
> Read や Edit も含まれるため、Claude Code からファイルを開いて直せなく
> なります。予防策・Bash-only の脱出ハッチ・復旧手順の全文は
> [`strict_config_error`（dsl.md）](./dsl.md#strict_config_error) を参照。

## 推奨 `.gitignore`

```gitignore
.ccchain.local.conf
```
