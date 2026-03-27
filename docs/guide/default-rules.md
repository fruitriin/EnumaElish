# Default Rules

Running `ccchain init` generates `.ccchain.conf` with the following default ruleset.

## Templates

### `primitive`
Basic safe output commands for pipe context:
```
template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq
```

### `safeRead`
Read-oriented processing commands, extends primitive:
```
template safeRead
  next: primitive
  |,>>
    allow grep, awk, sed
```

### `bulkExec`
Bulk processing commands with destructive command protection:
```
template bulkExec
  extends: safeRead
  |,>>
    deny rm    "don't pipe into destructive commands"
  exec:
    deny rm    "expand to tempfile first"
    allow cp, mv, touch
```

## Command Rules

| Command | Action | Template | Notes |
|---|---|---|---|
| `ls` | allow | primitive | Safe directory listing |
| `find` | allow | bulkExec | Protected against pipe-to-rm and exec-rm |
| `xargs` | allow | bulkExec | Same protection as find |
| `grep` | allow | safeRead | Read-only processing |
| `rm` | **ask** | — | Prompts user for confirmation |
| `curl \| bash` | **deny** | — | Prevents remote code execution |
| `curl \| sh` | **deny** | — | Prevents remote code execution |
| `eval` | **deny** | — | Not statically analyzable |

## Settings

```
settings:
  max_context_depth: 2    # audit expansion depth limit
  max_rules_per_cmd: 5    # audit rules-per-command limit
  fallback: ask           # action for unmatched commands
```

## Customizing

Override any default rule by adding rules after the defaults (last-rule-wins):

```
# In .ccchain.conf or .ccchain.local.conf

# Allow rm for this project (you know what you're doing)
allow rm

# Add project-specific tools
allow npm
  |
    deny rm  "don't pipe npm into rm"
```
