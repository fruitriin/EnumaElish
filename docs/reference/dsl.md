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
  scope:
    inside: <action> ["message"]
    outside: <action> ["message"]         # applies to both read and write
    outside-read: <action> ["message"]    # more specific than outside
    outside-write: <action> ["message"]   # more specific than outside
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
  workspace: <path>[, <path> ...]   # workspace scope roots (see `scope:`)
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

### `scope:`

Per-rule workspace scope actions. Requires `settings: workspace: <paths>`. Path arguments in the command are classified against the workspace(s) and matched against the scope clauses:

```
allow cp
  scope:
    inside: allow
    outside-read: allow          # reading outside is fine
    outside-write: deny  "cannot write outside workspace"

allow rm
  scope:
    inside: ask  "confirm deletion"
    outside-write: deny  "cannot delete outside workspace"

allow cat
  scope:
    inside: allow
    outside: ask  "please confirm outside access"   # applies to read and write
```

**Precedence** (per path argument):
- If path is inside workspace → `inside:` action (if set)
- If path is outside and used as a write argument → `outside-write:` > `outside:` (in that order)
- If path is outside and used as a read argument → `outside-read:` > `outside:` (in that order)

**Read/write classification** uses a built-in [semantics table](../../internal/semantics/table.go):

| Command family | Kind |
|---|---|
| `cat`, `head`, `tail`, `less`, `more`, `grep`, `awk`, `wc`, `file`, `stat`, `diff`, `cmp`, `md5sum`, `sha256sum`, `rg` | all path args = read |
| `rm`, `rmdir`, `shred`, `tee`, `touch`, `mkdir`, `unlink` | all path args = write |
| `cp`, `mv`, `ln` | last path arg = write, others = read |
| any other command | **unknown** — `outside-write`, `outside-read`, and `outside` clauses are all considered (most restrictive wins) |

**GNU coreutils `-t` flag.** `cp -t DIR src...`, `cp --target-directory=DIR src...`, and `cp -tDIR src...` are recognized: `DIR` is classified as a write and the remaining path arguments as reads. Same for `mv` and `ln`. (Critical C2.)

**Shell write redirects.** Redirect targets on `>`, `>>`, `>|`, `&>`, `&>>` are captured and classified as writes independently of the command's own args. `cat /ws/x > /outside/y` therefore triggers `outside-write` even though `cat` reads `/ws/x`. Read redirects (`<`, `<<`) are not tracked as writes. (Critical C1.)

**Unknown commands.** A command not listed in the semantics table produces `PathKindUnknown` for each path arg. The scope evaluator then considers the `outside-write`, `outside-read`, and `outside` clauses in that order and applies the most restrictive one — so `outside-write: deny` still fires for tools like `sed -i`, `rsync`, `wget -O`, `dd`, `tar`. (Critical C3.)

**Tool calls (Read / Edit / Write / MCP).** The `scope:` block is honored by the non-Bash tool evaluator as well. `Read` is classified as read, `Write` / `Edit` / `NotebookEdit` as write, and MCP tools as unknown. (Critical C6.)

**Symbolic link resolution.** Path classification uses `filepath.EvalSymlinks` on the longest existing ancestor of the argument. A link inside the workspace that points outside resolves to `outside`; a link outside that points inside resolves to `inside`. If resolution fails on every ancestor including the filesystem root (circular symlink / permission errors), the path is treated as `outside` (fail-closed; Critical C7).

**Non-existent paths.** For paths that don't yet exist (e.g. `cp foo new-file.txt`), the parent directory chain is resolved and the remaining suffix is appended — so a new file inside the workspace is still `inside`.

**Backward compatibility.** Rules without any `scope:` block keep the previous auto-escalation behavior: `allow` escalates to `ask` whenever any path argument is outside the workspace. Rules with `scope: outside: allow` explicitly opt out of that escalation for the whole command.

**Dynamic arguments.** A path argument containing `$VAR` / `$(cmd)` / `` `cmd` `` is undecidable — the evaluator treats it as `outside` (fail-closed) and, for unknown commands, promotes its kind to write so `outside-write` clauses still fire. `cp /ws/src $(echo /elsewhere)/dst` is therefore denied when `outside-write: deny` is set. (Critical C4.)

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
  workspace: ~/workspace  # workspace scope (comma-separated for multiple paths)
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
