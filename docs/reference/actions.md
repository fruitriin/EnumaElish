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

Blocks the command. The message reaches Claude as `permissionDecisionReason` (see [Config Reference / Hook Output](./config.md#hook-output)), enabling autonomous self-correction.

```
deny rm  "use trash instead"
deny eval  "eval is not statically analyzable; write the command directly"
```

**Hook behavior:** exit 0, `permissionDecision: "deny"` + `permissionDecisionReason` in JSON output.

**Design principle:** Deny messages should tell Claude *why* the command was blocked and *what to do instead*. This turns ccchain from a mere blocker into a teaching tool.

### `warn`

Allows the command but sends a caution message to Claude.

```
warn curl  "Consider using WebFetch instead"
```

**Hook behavior:** exit 0, `permissionDecision: "allow"` + `permissionDecisionReason` (the caution text lands in Claude's context but does not block execution).

**Note:** Whether Claude acts on the warning is model-dependent. ccchain guarantees the output format, not Claude's behavior.

### `ask`

Delegates the decision to Claude Code's built-in permission dialog. In non-interactive modes (`auto`, `dontAsk`, ...), the ask is degraded to `deny + hint` (default) or `warn` per [`ask_strategy`](./dsl.md#ask_strategy) and the ask rule's [`unattended:`](./dsl.md#unattended) direction.

```
ask rm
  message: "confirm file deletion"
  unattended: deny   # default: degrade to deny+hint in non-interactive modes
```

**Hook behavior:** exit 0, `permissionDecision: "ask"` in JSON output (interactive modes only).

### `hint`

> **Note:** `ccchain hook post` is currently a pass-through. `hint` actions used inside `postToolUse` rules and PostToolUse rule evaluation are planned for a future release.

At the PreToolUse layer, `hint` behaves like a friendlier `warn` — the call is not blocked but the message lands in Claude's context.

**Hook behavior:** exit 0, `permissionDecision: "allow"` + `permissionDecisionReason`.

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
