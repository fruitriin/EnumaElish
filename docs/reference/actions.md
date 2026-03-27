# Actions Reference

## Action Types

### `allow`

Permits the command to execute. No output.

```
allow ls
allow find
  next: bulkExec
```

**Hook behavior:** exit 0, no output.

### `deny`

Blocks the command. The message is sent to Claude via stderr, enabling autonomous self-correction.

```
deny rm  "use trash instead"
deny eval  "eval is not statically analyzable; write the command directly"
```

**Hook behavior:** exit 2, message on stderr.

**Design principle:** Deny messages should tell Claude *why* the command was blocked and *what to do instead*. This turns ccchain from a mere blocker into a teaching tool.

### `warn`

Allows the command but sends a warning to Claude via stdout JSON.

```
allow curl
  mode: warn
  message: "Consider using WebFetch instead"
```

**Hook behavior:** exit 0, `{"decision":"allow","message":"..."}` on stdout.

**Note:** Whether Claude acts on the warning is model-dependent. ccchain guarantees the exit code and output format, not Claude's behavior.

### `ask`

Delegates the decision to Claude Code's built-in permission dialog, prompting the user.

```
ask rm
  message: "confirm file deletion"
```

**Hook behavior:** exit 0, `{"decision":"ask"}` on stdout.

### `hint`

> **Note:** `ccchain hook post` is currently a pass-through. `hint` actions and PostToolUse rule evaluation are planned for a future release.

PostToolUse action for suggesting next steps after a command runs.

```
postToolUse
  allow WebFetch
    mode: hint
    message: "Save the result to a file"
```

**Hook behavior:** exit 0, message on stdout (PostToolUse only).

## Evaluation Order

### Last-Rule-Wins

When multiple rules match a command, the **last** matching rule takes precedence:

```
allow rm      # first match
deny rm       # second match — this wins
```

### Restriction Levels

When evaluating a pipeline or complex command, the **most restrictive** result across all segments is returned:

| Level | Action |
|---|---|
| 0 | allow |
| 1 | hint |
| 2 | warn |
| 3 | ask |
| 4 | deny |

### Fallback

Commands that don't match any rule use the `fallback` setting (default: `ask`).

## Dynamic Commands

Commands with variable expansion or command substitution are automatically denied:

```bash
$cmd foo              # → deny (variable as command)
$(generate_cmd) foo   # → deny (command substitution)
eval "$dynamic"       # → deny (dynamic eval)
```

Message: `"dynamic command detected: static analysis not possible"`
