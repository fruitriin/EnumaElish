# ccchain stats

hook が出力した JSONL 評価ログを集計します。`stats` は `settings.log:`
オプトインの消費側 — hook が PreToolUse 呼び出しごとに 1 行の JSONL を
書き出し、`stats` がそれを読み戻して action / matched_rule / command の
Top N を出力します。

「conf を変えたら本当に狙いどおり deny が減ったか？」のループを短くする
ためのコマンドです。セッションのトランスクリプトを grep する代わりに
`ccchain stats --since 24h --group-by rule` を叩けば、どのルールが何度
発火しているかが即座に分かります。

## オプトイン

hook は `.ccchain.conf` に `log:` を書かない限りログを**出力しません**:

```
settings:
  log: .ccchain/log.jsonl
```

- 相対パスは hook の `cwd`（プロジェクトルート）基準で解決、絶対パスは
  そのまま使用
- ログファイルは `0600`、親ディレクトリは `0700` で作成
- コマンド文字列は 200 UTF-8 バイトで rune 境界を尊重して切り詰め

**ログディレクトリを `.gitignore` に追加してください。** コマンド文字列
には秘密情報（CLI トークン、`--password=...` など）が混入し得ます:

```
# .gitignore
.ccchain/
```

ログ書き込みの失敗は stderr に警告が出るのみで、allow / deny 判定には
**影響しません** — ログは fail-open が仕様です。

## 使い方

```
ccchain stats [--since <duration>] [--group-by action|rule|command]
              [--top N] [--json] [--log <path>] [--config <path>]
```

デフォルト: `--since 24h` / `--group-by action` / `--top 20`。

### よく使う呼び方

```sh
# 直近 24 時間、action 別。
ccchain stats

# 直近 1 週間、ルール別、Top 10。
ccchain stats --since 7d --group-by rule --top 10

# JSON でスクリプト連携。
ccchain stats --group-by command --json | jq '.'

# conf の設定を無視して別ログを見る。
ccchain stats --log /tmp/session.jsonl --since 1h
```

### `--since` の書式

Go の `time.ParseDuration` 書式（`30m`, `2h30m`, `24h` ...）に加え、
運用しやすい `Nd` 表記（`1d`, `7d`, `30d`）を受け付けます。全件対象に
したい場合は `--since 0`。

### 出力形式

テーブル（デフォルト）:

```
log:      /Users/…/.ccchain/log.jsonl
since:    24h0m0s
group-by: action

COUNT   LAST_SEEN            ACTION
42      2026-07-17 09:12:03  allow
21      2026-07-17 08:47:11  deny
5       2026-07-17 07:31:45  ask
```

JSON（`--json`）:

```json
[
  {"key": "allow", "count": 42, "last_seen": 1763372823},
  {"key": "deny",  "count": 21, "last_seen": 1763370431},
  {"key": "ask",   "count": 5,  "last_seen": 1763367105}
]
```

## JSONL スキーマ

1 行 1 オブジェクト:

| フィールド | 型 | 説明 |
|---|---|---|
| `timestamp` | int (unix seconds) | hook 発火時刻 |
| `tool_name` | string | `Bash` / `Read` / `Edit` / `WebFetch` / `mcp__…` |
| `command` | string | 生コマンド（Bash のみ）。200 UTF-8 バイトで切り詰め |
| `action` | string | `allow` / `deny` / `warn` / `ask` / `hint` |
| `matched_rule` | string | マッチしたルール識別子（例: `deny rm`） |
| `message` | string | deny/warn/hint の理由 |
| `permission_mode` | string | 呼び出し時の Claude Code permission mode |
| `session_id` | string | Claude Code のセッション ID |
| `cwd` | string | 呼び出し時の作業ディレクトリ |

不正な行（プロセスクラッシュに伴う中途書き込みなど）は `ccchain stats`
が静かにスキップします — 集計値は誤りではなく保守的（少なめ）に出ます。

## Exit code

| Code | 意味 |
|---|---|
| 0 | 集計を出力 |
| 1 | 使い方エラー / conf エラー / `log:` 未設定 |

## 参照

- [`log:` 設定](./dsl.md#log) — オプトイン方法
- [承認トークン](./approve.md) — `deny + hint → ccchain approve` の
  フローはログには `deny` として保留リクエストごとに 1 件現れる
