# ccchain stats

Aggregate the JSONL evaluation log emitted by the hook. `stats` is the
consumer side of the `settings.log:` opt-in — the hook writes one JSONL
line per PreToolUse call, `stats` reads them back and prints top-N
counts by action / matched rule / command.

The command exists to shorten the "did my config actually change what I
thought it did?" loop. Instead of grepping session transcripts you run
`ccchain stats --since 24h --group-by rule` and immediately see which
rules fired and how often.

## Opt-in

The hook does **not** write a log unless you set the `log:` setting in
your `.ccchain.conf`:

```
settings:
  log: .ccchain/log.jsonl
```

- Relative paths are resolved against the hook's `cwd` (project root);
  absolute paths are used as-is
- The log file is created `0600` and its parent directory `0700`
- Command strings are truncated to 200 UTF-8 bytes at a rune boundary

**Add the log directory to `.gitignore`.** Commands can carry secrets
(tokens on the CLI, `--password=...`, etc.):

```
# .gitignore
.ccchain/
```

Failures to write the log emit a stderr warning but never change the
allow / deny decision — the log is fail-open by design.

## Usage

```
ccchain stats [--since <duration>] [--group-by action|rule|command]
              [--top N] [--json] [--log <path>] [--config <path>]
```

Defaults: `--since 24h`, `--group-by action`, `--top 20`.

### Common invocations

```sh
# Last 24 hours, action breakdown.
ccchain stats

# Last week, rule-by-rule breakdown, top 10.
ccchain stats --since 7d --group-by rule --top 10

# JSON for scripting.
ccchain stats --group-by command --json | jq '.'

# Ad-hoc query on a log outside the config's path.
ccchain stats --log /tmp/session.jsonl --since 1h
```

### `--since` values

Go's `time.ParseDuration` syntax (`30m`, `2h30m`, `24h`, ...) plus a
convenience `Nd` form (`1d`, `7d`, `30d`). Pass `--since 0` to include
every entry.

### Output format

Table (default):

```
log:      /Users/…/.ccchain/log.jsonl
since:    24h0m0s
group-by: action

COUNT   LAST_SEEN            ACTION
42      2026-07-17 09:12:03  allow
21      2026-07-17 08:47:11  deny
5       2026-07-17 07:31:45  ask
```

JSON (`--json`):

```json
[
  {"key": "allow", "count": 42, "last_seen": 1763372823},
  {"key": "deny",  "count": 21, "last_seen": 1763370431},
  {"key": "ask",   "count": 5,  "last_seen": 1763367105}
]
```

## JSONL schema

One JSON object per line:

| Field | Type | Description |
|---|---|---|
| `timestamp` | int (unix seconds) | When the hook fired |
| `tool_name` | string | `Bash`, `Read`, `Edit`, `WebFetch`, or an `mcp__…` prefix |
| `command` | string | Raw command (Bash only), truncated to 200 UTF-8 bytes |
| `action` | string | `allow` / `deny` / `warn` / `ask` / `hint` |
| `matched_rule` | string | Rule identifier, e.g. `deny rm` |
| `message` | string | Deny/warn/hint reason |
| `permission_mode` | string | Claude Code permission mode at the call |
| `session_id` | string | Claude Code session id |
| `cwd` | string | Working directory at the call |

Malformed lines (e.g. a partial write from a crashed process) are
silently skipped by `ccchain stats` — the count is conservative rather
than wrong.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Aggregation printed |
| 1 | Usage error, config error, or `log:` is not set |

## See also

- [`log:` setting](./dsl.md#log) — how to opt in
- [Approval Tokens](./approve.md) — the `deny + hint → ccchain approve`
  flow shows up in the log as `deny` events, one per pending request
