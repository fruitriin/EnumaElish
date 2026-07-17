# CLI Commands

## `ccchain check`

Validates the configuration file syntax.

```bash
ccchain check
ccchain check --config path/to/config.conf
ccchain check -v  # verbose: show parsed rules and templates
```

## `ccchain hook pre`

PreToolUse hook. Reads Claude Code's tool JSON from stdin, evaluates the command, and emits a `hookSpecificOutput` JSON response on stdout.

```bash
# Registered in .claude/settings.json — not called directly
echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"},"permission_mode":"auto","session_id":"...","cwd":"..."}' | ccchain hook pre
```

Always exits `0`. The decision is carried by the JSON body — see [Hook Output](../reference/config#hook-output) for the exact schema and [`ask_strategy`](../reference/dsl#ask_strategy) for how `ask` degrades in non-interactive `permission_mode`s.

## `ccchain hook post`

PostToolUse hook. Currently a pass-through for future use (hint actions, turn counting).

## `ccchain eval "command"`

Evaluates a command and outputs the result as JSON. Useful for debugging and scripting.

```bash
ccchain eval "find . | rm"
```

```json
{
  "action": "deny",
  "message": "don't pipe into destructive commands",
  "template": "bulkExec",
  "context": ["find", "|", "rm"]
}
```

## `ccchain diff <config-a> <config-b>`

Evaluates the same command list against two config files and reports, per command, whether the resulting action differs (`CHANGED`) or not (`same`). Useful for reviewing rule changes in PRs and for CI regression checks.

```bash
# Commands from a file (positional or --commands; specifying both is an error)
ccchain diff old.conf new.conf commands.txt
ccchain diff old.conf new.conf --commands commands.txt

# Commands from stdin
cat commands.txt | ccchain diff old.conf new.conf

# CI: fail the job when any command changes its decision
ccchain diff old.conf new.conf commands.txt --changed-only --exit-on-change
```

Example output:

```
Config A: old.conf
Config B: new.conf

find . -delete  a=[allow]  b=[deny]   CHANGED
ls -la          a=[allow]  b=[allow]  same

Summary: 2 commands — changed=1, same=1, error=0
Config A: old.conf
Config B: new.conf
```

**Scope**: only Bash command evaluation results are compared, and only the resulting action (`allow`/`ask`/`deny`/`warn`). Message differences and tool rules (Read/Edit etc.) are not compared.

Flags:

| Flag | Description |
|---|---|
| `--commands <file>` | Read commands from a file (one per line, `#` comments allowed) |
| `--changed-only` | Suppress `same` rows (summary still counts them) |
| `--exit-on-change` | Exit 2 when at least one command changed (for CI) |

Exit codes:

| Exit Code | Meaning |
|---|---|
| 0 | Completed (no changes, or changes without `--exit-on-change`) |
| 1 | Usage or config error (including empty config paths) |
| 2 | At least one `CHANGED` (only with `--exit-on-change`) |
| 3 | One or more commands failed to evaluate (takes precedence over 2) |

Notes:

- `diff` does not use the global `--config` flag; the two configs are positional arguments. Passing `--config` is an explicit error.
- Command strings are printed with control characters escaped (`\x1b` etc.) so an untrusted commands file cannot hide rows via terminal escape sequences.

## `ccchain audit`

Displays a flat expansion of all rules, showing exactly what each command+context combination resolves to.

```bash
ccchain audit
ccchain audit --config path/to/config.conf
```

Example output:
```
[allow]  ls
[allow]  ls | cat            (template: primitive)
[allow]  find
[deny]   find | rm           (template: bulkExec)  "don't pipe into destructive"
[deny]   find -exec rm       (template: bulkExec.exec)  "expand to tempfile first"
[---]    find && ...         (&&: reset → top-level rules)

Settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask

Stats:
  rules: 8
  templates: 3
```

## `ccchain suggest`

Analyzes commands and suggests rules for those that fall through to the `ask` fallback. Useful for discovering which commands need explicit rules.

```bash
# From command arguments
ccchain suggest "ls -la" "cat foo.txt" "rm -rf /"

# From a file (one command per line)
cat commands.txt | ccchain suggest
```

## `ccchain init`

Generates a default `.ccchain.conf` with sensible rules.

```bash
ccchain init                # default preset
ccchain init --sentinel     # deny-first sentinel preset (see reference/sentinel)
```

Will not overwrite an existing file. `--sentinel` selects the curated deny-first ruleset targeted at auto / dontAsk / headless environments — see [Sentinel Preset](../reference/sentinel).

## `ccchain approve`

Human-side of the [approval-token flow](../reference/approve). When ccchain degrades an `ask` to `deny + hint` in a non-interactive mode, it records the request as pending; the owner runs `ccchain approve` in their own terminal to unblock exactly that command.

```bash
ccchain approve --last                # approve the most recent pending entry
ccchain approve --list                # list pending entries
ccchain approve <hash-prefix>         # approve by hash prefix
ccchain approve --revoke-all          # invalidate every un-consumed approval
ccchain approve --last --ttl 1h       # custom lifetime (default 15m)
ccchain approve --last --global       # match any session/cwd (default: current only)
```

> **Security:** never run `ccchain approve` from an agent's Bash tool. The sentinel preset denies it; also add `Bash(ccchain approve*)` to `settings.json` `permissions.deny` for defense in depth.

## Global Flags

| Flag | Description |
|---|---|
| `--config <path>` | Explicit config file path |
| `--default-action <action>` | Override fallback action for unmatched commands (`allow`, `deny`, `ask`) |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Errors only |
| `--version` | Print version |
| `-h, --help` | Show help |
