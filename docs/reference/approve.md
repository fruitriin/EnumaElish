# Approval Tokens (`ccchain approve`)

The approval-token flow lets a human owner unblock a specific command that
ccchain denied through the [ask → deny degrade path](./dsl.md#ask_strategy) — turning
"the agent got blocked in `auto` mode" into an out-of-band conversation
instead of a dead end.

## Threat Model (why this design)

The hint text that ccchain returns as `permissionDecisionReason` reaches
the agent's context. **A token embedded in that hint would let the agent
self-approve.** The flow therefore:

- **Never puts a token in the hint** — only the procedure (`ccchain
  approve --last`) is written, and only the owner can act on it in a shell
  outside Claude Code
- **Records pending requests server-side** (`~/.claude/ccchain/pending.jsonl`)
  so the owner sees exactly what the agent asked for
- **Fences off `ccchain approve` from the agent's Bash tool** via the
  [sentinel preset](./sentinel.md) (`allow ccchain / args: ^approve\b:
  deny`). For defense in depth, also add `Bash(ccchain approve*)` to
  `settings.json` `permissions.deny`

## Flow

```
┌───────────────────────────┐
│ agent runs a Bash command │
└──────────────┬────────────┘
               │
               ▼
      ┌────────────────┐    interactive mode         ┌────────────────┐
      │ evaluate → ask │ ──────────────────────────► │ ask dialog     │
      └───────┬────────┘                             │ (human clicks) │
              │ non-interactive                      └────────────────┘
              │ (auto / dontAsk / …)
              ▼
   ┌──────────────────────────────┐
   │ ask degrades to deny + hint  │
   │ + record pending.jsonl entry │
   └──────────────┬───────────────┘
                  │
                  ▼
   agent sees: "run `ccchain approve --last`"
                  │
                  ▼
   owner (shell outside Claude Code):
     $ ccchain approve --last
   → writes approved.jsonl (TTL, scope)
                  │
                  ▼
   agent retries the same command
                  │
                  ▼
   ┌──────────────────────────────┐
   │ hook matches approval → allow │
   │ (one-shot: entry consumed)   │
   └──────────────────────────────┘
```

## CLI

```bash
ccchain approve --last              # approve the most recent pending entry
ccchain approve --list              # show pending entries (# / hash / age / command)
ccchain approve <hash-prefix>       # approve by hash prefix (min 4 chars)
ccchain approve --revoke-all        # mark every un-consumed approval as spent
```

Flags:

| Flag | Description |
|---|---|
| `--ttl <duration>` | Approval lifetime. Default `15m`. Accepts Go durations: `30s`, `1h`, ... |
| `--global` | Match any session / cwd. Default is session+cwd only |
| `-h, --help` | Show usage |

Example session:

```bash
# 1. Agent tries a command; ccchain records pending
$ ccchain approve --list
#    HASH              AGE       COMMAND
1    a1b2c3d4e5f6      3s        git push origin main
```

```bash
# 2. Owner approves in their own terminal
$ ccchain approve --last --ttl 1h
approved: a1b2c3d4e5f6
  command: git push origin main
  scope:   session
  cwd:     /home/user/project
  session: 01HXYZ...
  ttl:     1h0m0s
```

The next time the agent runs the identical command from the same session
+ cwd within the TTL, ccchain returns `permissionDecision: "allow"` and
the pending entry is consumed (one-shot).

## Matching Rules

- **Normalization**: commands are normalized by re-emitting the parsed
  shell AST (`mvdan.cc/sh` printer) before hashing (SHA-256). Whitespace
  and quoting differences (`ls -la` vs `ls  -la` vs `ls "-la"`) collapse
  to the same hash
- **Scope (default: session)**: `session_id` + `cwd` must match the entry
  recorded when the deny happened. `--global` widens this to any session
  and any directory
- **TTL (default: 15m)**: the approval expires TTL after `ccchain
  approve` was run. Expired approvals are never consumed
- **One-shot**: consumption is atomic per hook invocation; a granted
  approval covers exactly one retry

## Dynamic Commands are Ineligible

Commands containing `$VAR`, `$(...)`, or backticks are **not** eligible for
approval. Their expansion depends on runtime state, so the hash cannot
guarantee "the same command." When ccchain sees a dynamic command in the
degrade path, the deny message is appended with the reason and the human
is asked to rewrite the command with literal arguments (or run it in an
interactive session).

## Storage

- Directory: `$CLAUDE_CONFIG_DIR/ccchain/` if `CLAUDE_CONFIG_DIR` is set,
  otherwise `~/.claude/ccchain/`
- Files: `pending.jsonl` and `approved.jsonl` (append-only)
- Permissions: `0600` (owner read/write only)
- Locking: `O_EXCL` lock file for concurrent access safety — no external
  dependencies required
- Audit: approvals and consumptions are also written to the ccchain audit
  log (see [`ccchain audit`](../guide/cli.md#ccchain-audit))

## Security Notes

- **Do not run `ccchain approve` from an agent shell.** The sentinel
  preset denies it, but only if you use the sentinel preset. As defense in
  depth, add `Bash(ccchain approve*)` to `settings.json`'s
  `permissions.deny`
- Approvals are per-machine; there is no network path
- `--revoke-all` is the emergency stop button — it invalidates every
  un-consumed approval and forces the flow to start over
