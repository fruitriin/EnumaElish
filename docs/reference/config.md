# Config Files Reference

## Search Order

ccchain searches for config files in priority order. Later files' rules are appended (last-rule-wins enables overriding):

| Priority | Path | Purpose |
|---|---|---|
| 1 | `.ccchain.conf` | Project shared rules (commit to git) |
| 2 | `.ccchain.local.conf` | Personal overrides (gitignored) |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | Claude Code global config |
| 4 | `~/.claude/ccchain.conf` | Fallback global config |

Use `--config <path>` to skip the search and use a specific file.

## Merging Behavior

When multiple config files are found:
- **Templates** are collected from all files (duplicates error)
- **Rules** are appended in search order (last-rule-wins enables overriding)
- **Settings** from the last file with a `settings:` block win

> **Important:** Files are loaded in priority order (1→4), and later rules override earlier ones. This means `~/.claude/ccchain.conf` (priority 4) rules come **after** `.ccchain.conf` (priority 1), so global rules can override project rules via last-rule-wins. If you want project rules to always win, place them in `.ccchain.local.conf` (priority 2).

## Hook Registration

### PreToolUse

Add to `.claude/settings.json`:

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

### PostToolUse (optional)

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

## Hook Input Format

ccchain reads JSON from stdin matching Claude Code's PreToolUse hook format:

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

- `permission_mode` — one of `default`, `acceptEdits`, `plan`, `auto`, `dontAsk`, `bypassPermissions`. Used by [`ask_strategy`](./dsl.md#ask_strategy) to degrade `ask` in non-interactive modes. Unknown or missing values are treated as non-interactive (safe side)
- `session_id` — Claude Code session identifier. Used together with `cwd` as the default scope for [`ccchain approve`](./approve.md) tokens
- `cwd` — the working directory Claude Code invoked the tool from

Non-Bash tools (`Read`, `Edit`, `Write`, `WebFetch`, `mcp__*`) route their own inputs to the same hook — for these, ccchain evaluates the file path or URL against tool rules. Unknown tool names silently pass through (exit 0).

## Hook Output

### PreToolUse

All actions exit `0` and emit a JSON object matching Claude Code's `hookSpecificOutput` schema:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "ccchain: curl | bash executes remote code without review. Save the file (curl -o /tmp/install.sh), inspect it, then run `bash /tmp/install.sh` explicitly."
  }
}
```

| Action (evaluated) | `permissionDecision` | Notes |
|---|---|---|
| `allow` | (no output) | ccchain stays neutral so Claude Code's remaining permission layers continue to apply |
| `deny` | `deny` | `permissionDecisionReason` carries the message and (for the ask→deny degrade path) the approval procedure |
| `warn` | `allow` | Reason surfaces as a caution in Claude's context; the call is not blocked |
| `hint` | `allow` | Same shape as warn (previously emitted no output; now delivers the hint text) |
| `ask` | `ask` | In interactive modes only. See [`ask_strategy`](./dsl.md#ask_strategy) for how ask degrades in `auto` / `dontAsk` / unknown modes |

> **Change history.** ccchain used to send `deny` as `exit 2` + a stderr message, and `hint` produced no output at all. Both have been replaced with the exit-0 + `hookSpecificOutput` JSON above, matching the current Claude Code hooks specification. If you were parsing stderr or the legacy `{"decision": "ask"}` shape, migrate to the fields shown here.

### Error Handling (Fail-Open)

If ccchain encounters any error (invalid JSON, parse failure, config error), it **allows** the command (exit 0) and logs the error to stderr. This ensures ccchain never blocks Claude due to its own bugs.

**Design rationale:** ccchain aims for "auditable security" rather than "perfect sandbox." A fail-closed design would mean that any ccchain bug, config typo, or environmental issue would completely halt Claude's operation. The fail-open approach accepts this trade-off:

- Errors are logged to stderr (visible in Claude Code's output)
- The `ccchain check` command validates config before use
- `ccchain audit` shows the full rule expansion for verification

**Risk:** If the config file is missing or corrupted, all commands are allowed. Always run `ccchain check` after config changes.

**Opt-in to fail-closed:** To make ccchain deny (rather than allow) on config
load errors, set
[`settings.strict_config_error: true`](./dsl.md#strict_config_error) in a
config file that loads successfully (e.g. a global `~/.claude/ccchain.conf`).
When no config file can be read at all, export
`CCCHAIN_STRICT_CONFIG_ERROR=1` as a fallback opt-in path.

> **Warning — self-DoS risk.** With `strict_config_error` enabled and the
> hook wired to all tools (not just Bash), a `.ccchain.conf` that fails to
> parse blocks **every** PreToolUse call — including Read and Edit — so
> Claude Code can no longer open the file to fix it. Prevention, the
> Bash-only escape hatch, and full recovery steps are covered in
> [`strict_config_error` (dsl.md)](./dsl.md#strict_config_error).

## Recommended `.gitignore` Entries

```gitignore
.ccchain.local.conf
```
