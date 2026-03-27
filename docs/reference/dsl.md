# DSL Syntax Reference

ccchain uses an indent-based text DSL for rule configuration.

## Grammar

```
# Comments start with #

# Top-level rule
<action> <command>[, command2, ...] ["message"]
  # Context modifiers (indented)
  |,>>
    <action> <command>[, command2, ...] ["message"]
  exec:
    <action> <command>[, command2, ...] ["message"]
  args:
    <pattern>: <action>
  # Properties
  mode: block | warn | hint
  message: "..."
  next: <template_name>

# Template definition
template <name>
  extends: <parent_template>
  # Same structure as rules (|,>>, exec:, args:, next:)

# Hook sections
preToolUse
  # Rules for PreToolUse hook
postToolUse
  # Rules for PostToolUse hook

# Settings
settings:
  max_context_depth: <int>
  max_rules_per_cmd: <int>
  fallback: <action>
```

## Actions

| Action | Meaning |
|---|---|
| `allow` | Permit the command |
| `deny` | Block the command (exit 2 + message to Claude) |
| `warn` | Allow but send a warning to Claude |
| `ask` | Delegate to Claude Code's permission dialog |
| `hint` | PostToolUse: suggest next action to Claude |

## Context Modifiers

### `|,>>`

Rules that apply when the command appears as a pipe destination or redirect target:

```
allow find
  |,>>
    allow grep, sort
    deny rm  "don't pipe find into rm"
```

You can also use `|` alone (pipe only) or `>>` alone (redirect only).

### `exec:`

Rules that apply to commands nested via `-exec`, `xargs`, `bash -c`, etc.:

```
allow find
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv
```

### `args:`

Pattern-based rules on command arguments (regex):

```
allow curl
  args:
    -X GET: allow
    -X POST: ask
```

> **Note:** `args:` rules are currently parsed but **not yet evaluated** by the rule engine. They are reserved for a future release. Do not rely on `args:` rules for security decisions.

## Templates

### Definition

```
template <name>
  |,>>
    <rules>
  exec:
    <rules>
```

### Inheritance

```
template child
  extends: parent    # inherit all rules from parent
  |,>>
    allow extra-cmd  # add more rules
```

### Delegation

```
allow find
  next: bulkExec    # use bulkExec's pipe and exec rules
```

## Settings

```
settings:
  max_context_depth: 2    # max depth for audit expansion
  max_rules_per_cmd: 5    # max rules per command in audit
  fallback: ask           # action for unmatched commands
```

## Multiple Commands Per Rule

Comma-separated commands share the same rule:

```
allow cat, echo, head, tail, wc
```

## Messages

Quoted strings after commands are deny/warn messages:

```
deny rm  "use trash instead"
deny eval  "eval is not statically analyzable"
```

## Indentation

- Use spaces (2 or 4) or tabs
- Tabs are treated as 4 spaces
- Consistent indentation within a block is required
- Deeper indentation = child of the line above
