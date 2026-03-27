# Roadmap

This page describes planned features for ccchain. These are concepts, not release commitments — each item shows what will become possible and how it relates to current limitations.

## Current Status

The following capabilities are fully implemented:

- Structural context evaluation (pipes, chains, subshells)
- `allow`, `deny`, `ask` actions for Bash commands
- `args:` argument-level rules with regex patterns
- `permissive` mode for gradual rule adoption
- `ccchain init` / `ccchain check` / `ccchain eval` CLI commands
- Multi-file configuration with merge order

---

## Planned Features

### Phase 9 — DSL Consistency

**Status: Next up**

A documentation and DSL consistency fix. The `mode:` property is parsed by the DSL but currently has no effect at evaluation time. This phase resolves the ambiguity by clarifying the intended design.

**What changes for you:**

- The `warn` action will be documented clearly with correct syntax examples
- The `mode:` property's current behavior will be explicitly documented so you are not misled

No functional change is needed on your end; this is a correctness fix for the documentation and design intent.

---

### Phase 10 — settings.json Migration and Better Defaults

**What you can do today:**

If you are migrating from Claude Code's built-in `settings.json` permission system, you must manually translate entries like `"Bash(git log *)"` into ccchain DSL rules.

**What becomes possible:**

A `ccchain import` command that reads your existing `settings.json` and generates a `.ccchain.conf` draft:

```bash
ccchain import
# Outputs a .ccchain.conf draft based on your settings.json permissions
# Review and append to your config
```

Additionally, the built-in default ruleset expands to cover more common safe commands (`cat`, `echo`, `diff`, `wc`, etc.) and adds subcommand-level rules for `git` and `go` out of the box. Today you need to write these rules yourself; after this phase many will be included by default.

---

### Phase 11 — Workspace Scope: Path-Based Access Control

**What you can do today:**

ccchain controls commands by name and structural context. `cat ~/workspace/README.md` and `cat ~/.ssh/id_rsa` are evaluated the same way — by the `cat` rule alone.

**What becomes possible:**

A `scope:` directive lets you declare which directories are your workspace. Commands that reference paths outside the workspace are evaluated differently:

```
scope:
  workspace: ~/workspace

allow cat
  scope:
    inside: allow
    outside: ask  "This file is outside your workspace"

allow rm
  scope:
    inside: ask   "Confirm deletion"
    outside: deny "Deleting files outside workspace is not allowed"
```

Known limitations (static analysis boundaries):
- Shell variables like `$HOME/file` cannot be resolved and fall back to the command-level rule
- Relative paths are treated as inside the workspace (fail-open)
- Symlinks are not resolved

---

### Phase 12 — Dynamic deny Messages

**What you can do today:**

Deny messages are static strings. When a command is blocked, Claude sees only the fixed message you wrote.

**What becomes possible:**

Message templates can embed information about the blocked command, making denies more actionable:

```
deny rm
  message: "{command} is not allowed. Use: find . -name '...' -print > /tmp/targets_{id}.txt"
```

Available variables: `{command}`, `{cmd}`, `{args}`, `{id}` (unique per invocation), `{cwd}`.

This makes it easier to guide Claude toward the safe alternative without writing separate documentation.

---

### Phase 13 — Command Semantics Table

**What you can do today:**

Subcommand-level control requires you to write `args:` regex rules yourself. To distinguish `sed -n` (safe) from `sed -i` (in-place edit), you write the pattern manually.

**What becomes possible:**

A built-in knowledge table of 45+ CLI tools with known safe/dangerous flags and subcommands. A `ccchain generate-rules` command outputs suggested `args:` rules based on this table:

```bash
ccchain generate-rules --from-semantics
# Outputs args: rules for git, go, npm, sed, docker, kubectl, etc.
# Review and append to your .ccchain.conf
```

This is a one-time helper for bootstrapping a more precise ruleset.

---

### Phase 14 — Multi-Tool Control (Read, Edit, WebFetch, MCP)

**What you can do today:**

ccchain intercepts Bash tool calls only. Claude's `Read`, `Edit`, `Write`, `WebFetch`, and MCP tools are not subject to ccchain rules — they pass through unchecked.

**What becomes possible:**

The DSL gains a top-level section for non-Bash tools:

```
preToolUse
  Read
    scope:
      inside: allow
      outside: ask  "Reading outside workspace"
    args:
      \.env|\.ssh|credentials: deny  "Reading secrets is not allowed"

  WebFetch
    args:
      ^https://: allow
      ^http://: ask  "Unencrypted HTTP"
      \.internal\.: deny  "Internal network access is not allowed"

  mcp__*__delete_*
    action: deny  "MCP deletions require manual approval"
```

This phase depends on Phase 11 (workspace scope) being in place.

---

### Phase 15 — Project Auto-Detection

**What you can do today:**

`ccchain init` generates a generic default configuration. You then manually add rules for your specific project type.

**What becomes possible:**

`ccchain detect` inspects your project files (`go.mod`, `package.json`, `Cargo.toml`, `Makefile`, etc.) and outputs tailored rule suggestions:

```bash
ccchain detect
# Detected: Go project (go.mod), Make (Makefile)
# Outputs suggested rules for go, make, etc.
```

`ccchain init --detect` combines detection with initialization in one step.

---

### Phase 16 — Path Redirect

**What you can do today:**

You can deny access to a file with a message explaining what to do instead. The message is free text — ccchain has no structured concept of "use this path instead."

**What becomes possible:**

A `redirect` directive gives structured alternative-path guidance:

```
redirect .env
  to: .env.example
  message: "Edit .env.example instead of .env directly"

redirect node_modules/
  action: deny
  message: "Do not edit node_modules directly. Modify package.json instead"
```

This integrates with Phase 14 (multi-tool control) to apply to `Read`/`Edit`/`Write` tool calls, not just Bash commands.

---

## Unplanned Ideas

The following ideas are recorded but not yet formalized into implementation plans:

**PostToolUse turn counting** — Limit how many times Claude can invoke a tool repeatedly before requiring confirmation. Useful for preventing runaway loops.

**`source` / `.` command tracking** — Sourced scripts execute in the current shell context and cannot be statically analyzed. This is a fundamental limitation of static analysis. The planned outcome is documentation of this limitation, not a technical solution.

---

## Dependency Graph

```
Phase 9  (DSL consistency)          — no dependencies
Phase 10 (settings import)          — no dependencies
Phase 11 (workspace scope)          — no dependencies
Phase 12 (dynamic messages)         — no dependencies
Phase 13 (semantics table)          — no dependencies
Phase 14 (multi-tool)               — depends on Phase 11
Phase 15 (project detection)        — no dependencies
Phase 16 (path redirect)            — depends on Phase 14
```

Phases 9–13 and 15 can be delivered in any order. Phase 14 enables Phase 16.
