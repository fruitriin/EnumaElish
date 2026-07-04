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
  mode: block | warn | hint  # DEPRECATED: parsed but has no effect. Use warn/hint actions directly.
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
  strict_config_error: true | false   # deny on config load failure (default false = fail-open)
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

The pattern is a Go regular expression matched against the **joined argument string** (`strings.Join(args, " ")`).

**Important notes:**
- Patterns use **partial matching** by default. Use `^` and `$` anchors for exact matching
- If arguments contain dynamic expansion (`$VAR`, `` `cmd` ``), args: evaluation is skipped and the parent rule's action is used
- Multiple args: patterns follow last-rule-wins
- Args: rules override the parent rule's action when matched

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
  max_context_depth: 2         # max depth for audit expansion
  max_rules_per_cmd: 5         # max rules per command in audit
  fallback: ask                # action for unmatched commands
  workspace: ~/workspace       # workspace scope (comma-separated for multiple paths)
  strict_config_error: true    # fail closed (deny) when config load fails; default: false
```

### `strict_config_error`

By default, ccchain is fail-open — any config load failure (missing file,
parse error, unresolved template) is logged to stderr and the command is
**allowed** (see [Error Handling](./config.md#error-handling-fail-open)).
Setting `strict_config_error: true` in any config file that loaded
successfully (e.g. a global `~/.claude/ccchain.conf`) opts into fail-closed:
if a later config file fails to load, the PreToolUse hook denies the tool
call with exit 2.

When no config file could be loaded at all, the only way to opt into strict
mode is the environment variable `CCCHAIN_STRICT_CONFIG_ERROR=1`
(or `true`).

Use strict mode when running unattended in high-security environments where
silent fail-open is unacceptable. Pair it with the `ccchain check` command
during CI to catch config errors before deployment.

**Warning — self-DoS failure mode:** With strict mode enabled AND a broken
config file, **every** PreToolUse hook call exits 2. That blocks Bash *and*
Read/Edit/Write, so Claude cannot even open the config to fix it. Recovery
requires shell-level intervention outside Claude Code:

1. Prefer prevention: run `ccchain check` before enabling strict mode, and
   again in CI on every config change.
2. If you get locked out:
   - If strict mode came from `CCCHAIN_STRICT_CONFIG_ERROR`, `unset` it in
     your shell (or restart Claude Code without the variable).
   - If strict mode came from a config file, edit the broken config directly
     in a normal terminal (bypassing ccchain), or temporarily rename the
     broken file so it fails the stat check in the search path.

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
