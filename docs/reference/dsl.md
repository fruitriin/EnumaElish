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
  unattended: deny | allow  # ask rules only — degrade direction in non-interactive modes

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
  ask_strategy: degrade | passthrough | deny-all   # how ask resolves at the hook layer (default degrade)
  ask_degrade_default: deny | allow                # which side ask degrades to under degrade (default deny)
  unanalyzable_action: ask | deny                  # action for structurally unanalyzable commands (default deny)
```

**Shell-quoting note:** Both command-name and argument matching operate on strings *after* shell quote removal, just like the shell does before executing the command — `"rm"` matches a `deny rm` rule, `curl -X "POST"` matches an `args:` pattern of `-X POST`, and `rm "-rf" /` matches `-rf` in args:. Only literal single/double-quote wrappers are stripped; backslash escapes inside double quotes (`\"`, `\$`, `\\`, `` \` ``, etc.) and ANSI-C `$'...'` sequences are passed through as written, so patterns that need to defend against escaped variants must account for them explicitly.

## Actions

| Action | Meaning |
|---|---|
| `allow` | Permit the command (ccchain stays neutral — see [Hook Output](./config.md#hook-output)) |
| `deny` | Block the command. Emits `permissionDecision: "deny"` + reason JSON |
| `warn` | Allow but attach a caution message. Emits `permissionDecision: "allow"` + reason |
| `ask` | Delegate to Claude Code's permission dialog. In non-interactive modes, degrades per [`ask_strategy`](#ask_strategy) and the rule's [`unattended:`](#unattended) direction |
| `hint` | PreToolUse: friendlier `warn`. PostToolUse: suggest next action to Claude (planned) |

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

**Read/write classification** uses a built-in [semantics table](https://github.com/fruitriin/EnumaElish/blob/main/internal/semantics/table.go):

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

**One-directional (strict-only) semantics.** `scope:` clauses can only make the base rule's action **stricter**, never looser. The five actions are ordered `allow < hint < warn < ask < deny` (see [`restrictionLevel` in evaluate.go](https://github.com/fruitriin/EnumaElish/blob/main/internal/eval/evaluate.go)); a scope-derived candidate action is applied only when it is more restrictive than the base rule's action, otherwise the base action is kept. Concretely:

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
  ask_strategy: degrade        # how ask resolves at the hook layer (degrade|passthrough|deny-all)
  ask_degrade_default: deny    # which way ask degrades under degrade (deny|allow)
  unanalyzable_action: deny    # action for structurally unanalyzable commands (ask|deny)
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

### `ask_strategy`

Controls how an `ask` decision is resolved when the hook writes its response
to Claude Code. In some [permission modes](https://docs.claude.com/en/docs/claude-code/permissions) — `auto`, `dontAsk`,
`headless` (`claude -p`) — an `ask` cannot show a confirmation dialog to a
human. `ask_strategy` picks the behaviour ccchain uses in those modes:

- `degrade` (default): in interactive modes (`default` / `acceptEdits` /
  `plan` / `bypassPermissions`) `ask` passes through unchanged. In every
  other mode `ask` is **downgraded** — to `deny + hint` by default, or to
  `warn + hint` when the specific rule opts in via
  [`unattended: allow`](#unattended). The hint text explains why the block
  happened and includes the human approval procedure via
  [`ccchain approve`](./approve.md).
- `passthrough`: emit `ask` unchanged regardless of mode. This is the
  pre-Plan-0022 behaviour and is useful if you fully trust Claude Code's
  auto classifier to route asks correctly.
- `deny-all`: escalate every `ask` to `deny + hint` in every mode, even for
  rules that explicitly wrote `unattended: allow`. Intended for CI or other
  strict environments where no ask should ever leak through.

Resolution order for the direction (`deny` vs. `allow`) under `degrade`:
per-rule [`unattended:`](#unattended) → `ask_degrade_default` (global) →
built-in `deny` (safe default).

Any value other than `degrade`, `passthrough`, or `deny-all` is a parse
error.

### `ask_degrade_default`

Global default for the direction `ask_strategy: degrade` takes when a
matching rule does not specify `unattended:`. Accepted values:

- `deny` (default): degrade to `deny + hint`. Turns the block into an
  asynchronous conversation — the owner runs `ccchain approve --last` in
  their own terminal to unblock the request (see [Approval Tokens](./approve.md))
- `allow`: degrade to `warn`. The call is allowed but a caution message
  lands in Claude's context. Use this for rules where "please confirm" is a
  reminder rather than a security gate

Any value other than `deny` or `allow` is a parse error.

### `unanalyzable_action`

Controls the action for commands that ccchain cannot statically analyse:
`eval`, dynamic subshells, non-literal `for` loops, C-style loops, `select`,
positional-parameter iteration, control-flow constructs where the body
depends on runtime values, etc. Literal `for` loops (`for f in a b c; do
BODY; done`) are expanded and evaluated per iteration and are **not**
affected by this setting.

- `deny` (default): treat unanalysable commands as `deny`. Combined with
  `ask_strategy: degrade` this becomes the safety net for auto / dontAsk
  modes where an ask would silently classifier-decide
- `ask`: treat them as `ask`. Softer, useful when you want interactive
  confirmation on constructs whose intent you cannot pre-write

`allow` is deliberately **not** accepted — enabling it would let a single
setting disable the safety net that guards control-flow, subshells, and
dynamic commands. Any other value is a parse error.

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

**Warning — self-DoS failure mode.** With `strict_config_error` enabled
**and the hook wired to all tools** (or to Read/Edit/Write **in addition
to** Bash), if you save a `.ccchain.conf` that fails to parse, **every**
PreToolUse hook call exits 2 — including Read and Edit. Claude Code can no
longer open the broken config to fix it.

**Bash-only escape hatch.** If your hook registration only matches
`"Bash"`, Read/Edit are not routed through ccchain and remain usable during
a broken-config state. In that case no external terminal is needed: fix the
config file directly from Claude Code with the Edit tool, then verify with
`ccchain check --config <path>` once Bash unblocks. The numbered recovery
steps below are only needed for the harder all-tools case.

**Prevention (recommended):** Run `ccchain check --config <path>` before
saving any change to a config file. Wire it into CI on every commit that
touches `.ccchain.conf`.

**Warning — `ccchain check` requires `--config`.** A bare positional path
is **silently ignored**: `check` then validates the default search path
(see the table at the top of [config.md](./config.md)) instead of your
file, and may report `config OK` while the file you meant to test is still
broken.

```sh
ccchain check broken.conf            # WRONG: positional path is ignored;
                                     # validates the default search path and may print "config OK"
ccchain check --config broken.conf   # CORRECT: validates broken.conf
```

**Recovery steps if you are already locked out:**

0. Diagnose where strict mode comes from — the env var, a config file, or
   both. In a shell outside Claude Code:

   ```sh
   env | grep CCCHAIN_STRICT_CONFIG_ERROR
   grep -l strict_config_error .ccchain.conf .ccchain.local.conf \
     "${CLAUDE_CONFIG_DIR:-$HOME/.claude}/ccchain.conf" 2>/dev/null
   ```

   The first command tells you whether step 1 applies; the second lists the
   config file(s) that set it (handled in step 2).
1. If strict mode came from the env var: in a shell outside Claude Code,
   `unset CCCHAIN_STRICT_CONFIG_ERROR` (or set it to `false`). Unsetting
   the variable in a fresh shell does **not** propagate to an
   already-running Claude Code process — Unix environment variables are
   copied at `fork(2)` time and are not shared live between sibling
   processes. You must restart Claude Code (step 3) for the change to take
   effect — and the restart must happen from an environment where the
   variable is actually unset: if your shell profile (`~/.zshrc`,
   `~/.config/fish/config.fish`, ...) exports it, a restarted Claude Code
   inherits it again. Remove the export from the profile first. If strict
   mode came from a loaded config file only, skip to step 2.
2. Open a normal terminal outside Claude Code (so ccchain's hook is not in
   the way) and edit the broken config file directly to fix the parse
   error. (Under a Bash-only registration, editing from Claude Code works
   too — see the escape hatch above.)

   As a **last resort**, you can temporarily rename the broken file so it
   drops out of the search path. Two caveats:

   - Renaming does **not** fall back to any generic built-in ruleset. The
     search simply continues down the search-path table (see
     [config.md](./config.md)): if a global config
     (`$CLAUDE_CONFIG_DIR/ccchain.conf` or `~/.claude/ccchain.conf`)
     exists, its rules **silently take over** — a global `fallback: deny`
     will then deny even `echo hi`, while a permissive global config
     silently drops all your project-specific deny/ask/scope: rules with no
     error logged. If no config file remains anywhere, evaluation runs with
     no rules at all and every command falls back to the built-in default
     `fallback: ask`. Renaming is therefore only reasonable when the hook
     uses the default search path **and** you either have no global config
     or know exactly what it contains. Restore the original filename (and
     re-validate with `ccchain check --config <path>`) immediately after
     fixing it — do not leave it renamed.
   - If the hook is registered with an explicit `--config <path>` (e.g.
     `ccchain hook pre --config /path/to/rules.conf`), renaming recovers
     nothing: the load keeps failing, now with "file not found", and with
     strict mode still active via the env var the hook keeps denying. (If
     strict mode came from that config file itself, renaming instead drops
     you into fail-open with **no rules at all**.) In the `--config` case,
     fixing the file's content in place is the only recovery.
3. Restart Claude Code if you changed the env var in step 1 (a restart is
   the only way to pick up the new environment — see the shell-profile
   caveat there). If you only fixed the config file in step 2, no restart
   is needed — `dsl.LoadConfig` runs on every hook invocation, so the next
   tool call reloads the corrected config automatically and the workspace
   unblocks.

## `unattended:` (ask rules only)

Per-rule override for [`ask_strategy: degrade`](#ask_strategy). Declares
which side this specific `ask` should fall to when the current permission
mode is non-interactive (auto / dontAsk / headless / unknown):

```
ask docker
  message: "Container operations should be confirmed"
  unattended: allow          # in non-interactive modes, degrade to warn
                             # (allow the call, land the caution in Claude's context)

ask git-branch-delete
  message: "Take a backup ref before deleting the branch"
  unattended: deny           # same as the built-in default, spelled explicitly:
                             # degrade to deny+hint with the approve procedure
```

Rules:

- Only valid on `ask` rules. Placing `unattended:` on any other action is a
  parse error
- Values: `deny` or `allow` (parse error otherwise)
- Ignored under `ask_strategy: passthrough` (the whole ask passes through);
  overridden under `ask_strategy: deny-all` (every ask is denied)
- When omitted, the direction falls back to [`ask_degrade_default`](#ask_degrade_default)
  → built-in `deny`

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
