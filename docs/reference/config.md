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

## Recommended `.gitignore` Entries

```gitignore
.ccchain.local.conf
```
