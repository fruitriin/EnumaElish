# Roadmap

Where ccchain is today, and what is (and isn't) on the near-term list. This is a status page, not a release commitment.

## Current Status

The following capabilities are fully implemented:

- Structural context evaluation (pipes, chains, subshells, literal `for` loops)
- `allow`, `deny`, `warn`, `ask`, `hint` actions for Bash and non-Bash tools
- `args:` argument-level rules with regex patterns and length-overflow escalation
- `scope:` per-rule workspace access control (inside / outside-read / outside-write) for Bash, Read, Edit, Write, MCP
- Modern hook I/O (exit 0 + `hookSpecificOutput.permissionDecision` JSON, `permission_mode` / `session_id` / `cwd` awareness)
- [`ask_strategy`](../reference/dsl#ask_strategy) and per-rule [`unattended:`](../reference/dsl#unattended) — degrade ask in non-interactive modes
- [Approval tokens](../reference/approve) (`ccchain approve --last` / `--list` / `<hash-prefix>` / `--revoke-all`, TTL, session+cwd scope, one-shot consumption)
- [Sentinel preset](../reference/sentinel) (`ccchain init --sentinel`) — curated deny-first ruleset for auto / dontAsk / headless
- Semantics table and project auto-detection (`ccchain generate-rules`, `ccchain detect`)
- Message templates (`{command}`, `{args}`, `{id}`, `{cwd}`) with sanitisation
- CLI: `check`, `eval`, `test`, `diff`, `suggest`, `audit`, `init [--sentinel]`, `hook pre|post`, `approve`, `detect`, `generate-rules`, `version`
- Multi-file configuration with merge order (project → local → global)

---

## Completed Phases (historical)

These phases have all landed. Their features are documented under the reference pages linked below.

| Phase | Feature | Reference |
|---|---|---|
| 9 | DSL consistency (`mode:` deprecated) | [DSL Syntax](../reference/dsl) |
| 10 | Settings compat + baseline defaults | [Config Files](../reference/config) |
| 11 | Workspace scope | [DSL `scope:`](../reference/dsl#scope) |
| 12 | Message templates | [DSL Messages](../reference/dsl#messages) |
| 13 | Command semantics table | `ccchain generate-rules` |
| 14 | Multi-tool control (Read / Edit / Write / MCP) | [Config Hook Input Format](../reference/config#hook-input-format) |
| 15 | Project auto-detection | `ccchain detect` |
| 16 | Deny redirect / workspace read/write split | [DSL `scope:` outside-write](../reference/dsl#scope) |
| 22 | Ask strategy + deny-first sentinel (Plan 0022) | [`ask_strategy`](../reference/dsl#ask_strategy), [Approval Tokens](../reference/approve), [Sentinel Preset](../reference/sentinel) |
| 25 | Literal `for`-loop expansion + `unanalyzable_action` | [DSL `unanalyzable_action`](../reference/dsl#unanalyzable_action) |

---

## In Flight / Not Yet Planned

- **ADDF integration** (Plan 0023) — automatic hook wiring via `/addf-init`
- **Remote / Slack approval bridge** (Plan 0024) — approval-token delivery over the network
- **REPL / stats subcommands** — the remainder of the Plan 0019 developer experience work (`diff` is already shipped)
- **PostToolUse turn counting** — cap repeated tool invocations before requiring confirmation
- **`source` / `.` command tracking** — a fundamental limitation of static analysis; the planned deliverable is documentation, not code
