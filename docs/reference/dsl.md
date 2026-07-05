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
  scope_violation: ask | deny         # escalate outside-workspace paths (default ask)
  strict_config_error: true | false   # deny on config load failure (default false = fail-open)
```

**Shell-quoting note:** Both command-name and argument matching operate on strings *after* shell quote removal, just like the shell does before executing the command — `"rm"` matches a `deny rm` rule, `curl -X "POST"` matches an `args:` pattern of `-X POST`, and `rm "-rf" /` matches `-rf` in args:. Only literal single/double-quote wrappers are stripped; backslash escapes inside double quotes (`\"`, `\$`, `\\`, `` \` ``, etc.) and ANSI-C `$'...'` sequences are passed through as written, so patterns that need to defend against escaped variants must account for them explicitly.

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
- Shell quoting is removed before matching, just as the shell removes it before executing the command: `curl -X "POST"` matches the pattern `-X POST`. The same quote removal is applied to command-name matching (see the shell-quoting note in the Grammar section). Only literal `'...'` / `"..."` wrappers are stripped; backslash escapes inside double quotes (`\!`, `foo\ bar`, `\"`, `\$`, `` \` ``) and ANSI-C `$'...'` sequences are passed through as-is
- If the joined argument string exceeds **4096 bytes**, args: rules are not applied and the result is escalated to the **strictest action declared in this rule's `args:` block**, with a floor of `ask` (a stricter parent action such as `deny` is kept). Concretely: an `args:` block containing a `deny` entry denies on over-length input; a block whose strictest entry is `ask` (or only `allow`) escalates to `ask`. Falling back to the parent action would let argument padding bypass an escalating args: rule (e.g. `allow rm` + `args: -rf /: deny`)

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

**One-directional (strict-only) semantics.** `scope:` clauses can only make the base rule's action **stricter**, never looser. The five actions are ordered `allow < hint < warn < ask < deny` (see [`restrictionLevel` in evaluate.go](../../internal/eval/evaluate.go)); a scope-derived candidate action is applied only when it is more restrictive than the base rule's action, otherwise the base action is kept. Concretely:

- If the base rule is `deny rm`, writing `scope: outside: allow` does **not** promote outside deletions to `allow`. The scope clause is accepted syntactically but the evaluator keeps the base `deny`.
- If the base rule is `ask cp`, writing `scope: inside: allow` does **not** demote inside copies to `allow`. It is silently ignored and the base `ask` is kept.
- If the base rule is `warn cp`, writing `scope: outside: deny` **does** escalate outside copies to `deny` (deny > warn). Writing `scope: outside: allow` on the same `warn cp`, however, does **not** demote outside copies to `allow` — the base `warn` is preserved because `allow < warn`.
- Similarly, `hint cp` + `scope: outside: warn` promotes outside copies to `warn` (warn > hint), while `hint cp` + `scope: outside: allow` keeps the base `hint`.
- The only "loosening" effect scope: has is the backward-compatibility opt-out described above: `scope: outside: allow` on an **`allow`** rule prevents the automatic `allow → ask` escalation for outside paths — but that keeps the action at the base allow, it does not promote a stricter base to allow.

**Rationale.** Workspace scope is a security feature. Making it one-directional prevents a permissive scope clause from silently widening a stricter base rule and eliminates a class of "I thought scope: could relax this" misreadings. Use `scope:` to add finer-grained restrictions; use the base action for the ceiling.

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
  max_context_depth: 2         # max depth for audit expansion
  max_rules_per_cmd: 5         # max rules per command in audit
  fallback: ask                # action for unmatched commands
  workspace: ~/workspace       # workspace scope (comma-separated for multiple paths)
  scope_violation: ask         # action when a path outside the workspace is detected (ask|deny)
  strict_config_error: true    # fail closed (deny) when config load fails; default: false
```

### `scope_violation`

Controls what happens when a command or tool call that would otherwise be
`allow`ed references a path outside the `workspace` scope:

- `ask` (default): escalate `allow` → `ask`. The user can still approve
  access outside the workspace via the permission dialog.
- `deny`: escalate `allow` → `deny`. Access outside the workspace is
  blocked outright — useful for strict setups where the permission dialog
  may not reach a human (e.g. headless / auto-approve modes).

Only `allow` results are escalated; explicit `ask` and `deny` rules are
left untouched. Any value other than `ask` or `deny` is a parse error.

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
