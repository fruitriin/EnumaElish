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

ccchain reads JSON from stdin matching Claude Code's hook format:

```json
{
  "tool_name": "Bash",
  "tool_input": {
    "command": "find . -name '*.log' | rm -rf"
  }
}
```

Non-Bash tools are silently passed through (exit 0).

## Hook Output

### PreToolUse

| Decision | Exit Code | Output |
|---|---|---|
| Allow | 0 | (none) |
| Deny | 2 | Message on stderr |
| Warn | 0 | `{"decision":"allow","message":"..."}` on stdout |
| Ask | 0 | `{"decision":"ask"}` on stdout |

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

> **Warning — self-DoS recovery.** This section assumes the `PreToolUse`
> hook is registered to match every tool (or Read/Edit/Write **in addition to**
> Bash). If your registration only matches `"Bash"` (as in the sample above),
> Read/Edit are not routed through ccchain and remain usable during a broken-
> config state — you can fix the file directly from Claude Code and step 2
> below is the only step that applies. The paragraph below covers the harder
> case where the hook is wired to all tools.
>
> With `strict_config_error` enabled **and the hook wired to all tools**, if
> you save a `.ccchain.conf` that fails to parse, **every** PreToolUse hook
> call exits 2 — including Read and Edit. Claude Code can no longer open the
> broken config to fix it. See also the deeper explanation in
> [`strict_config_error` (dsl.md)](./dsl.md#strict_config_error).
>
> **Prevention (recommended):** Run `ccchain check --config <path>` before
> saving any change to a config file. Wire it into CI on every commit that
> touches `.ccchain.conf`.
>
> Note: `check` requires `--config` to validate a specific file. A bare
> positional path (e.g. `ccchain check broken.conf`) is silently ignored and
> `check` falls back to the default search path (see the table at the top of
> this file), so it may report "config OK" while the file you meant to test
> is still broken.
>
> **Recovery steps if you are already locked out:**
>
> 1. In a shell outside Claude Code, `unset CCCHAIN_STRICT_CONFIG_ERROR` (or
>    set it to `false`) if strict mode came from the env var. Note that
>    unsetting the variable in a fresh shell does **not** propagate to an
>    already-running Claude Code process — Unix environment variables are
>    copied at `fork(2)` time and are not shared live between sibling
>    processes. You must restart Claude Code (step 3) for the change to
>    take effect. If strict mode came from a loaded config file, skip to
>    step 2.
> 2. Open a normal terminal outside Claude Code (so ccchain's hook is not in
>    the way) and edit `.ccchain.conf` directly to fix the parse error — or
>    temporarily rename the broken file so it drops out of the search path.
>
>    Warning: renaming the file away does not merely disable strict-mode — it
>    silently drops **all** your project-specific rules (deny/ask/scope:) and
>    falls back to ccchain's generic built-in semantics table, with no error
>    logged. Restore the original filename (and re-validate with
>    `ccchain check --config <path>`) immediately after fixing it — do not
>    leave it renamed.
> 3. Restart Claude Code if you changed the env var in step 1 (a restart is
>    the only way to pick up the new environment). If you only fixed the
>    config file in step 2, no restart is needed — `dsl.LoadConfig` runs on
>    every hook invocation, so the next tool call reloads the corrected
>    config automatically and the workspace unblocks.

## Recommended `.gitignore` Entries

```gitignore
.ccchain.local.conf
```
