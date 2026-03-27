# How It Works

## Architecture

ccchain operates as a Claude Code [PreToolUse hook](https://docs.anthropic.com/en/docs/build-with-claude/claude-code/hooks). When Claude attempts to run a Bash command, the hook fires before execution:

```
Claude wants to run: find . -name "*.log" | rm -rf
                         │
                         ▼
              ┌──────────────────────┐
              │  ccchain hook pre    │
              │                      │
              │  1. Parse shell AST  │  ← mvdan.cc/sh (bash mode)
              │  2. Build topology   │  ← pipes, chains, subshells
              │  3. Evaluate rules   │  ← .ccchain.conf
              │  4. Return decision  │
              └──────────────────────┘
                         │
              ┌──────────┴──────────┐
              │                     │
         exit 0 (allow)       exit 2 (deny)
              │                     │
         Command runs        Claude sees:
                             "don't pipe find into rm"
                             and rewrites the command
```

## Evaluation Flow

### 1. Shell AST Parsing

ccchain uses `mvdan.cc/sh` (the parser behind `shfmt`) in bash mode to parse the command into a full AST. This handles:

- Pipes: `cmd1 | cmd2`
- Chains: `cmd1 && cmd2`, `cmd1 ; cmd2`
- Subshells: `$(cmd)`, `(cmd1; cmd2)`
- Process substitution: `<(cmd)`
- Redirects: `cmd > file`, `cmd >> file`

### 2. Topology Construction

The AST is converted into an **execution topology** — a simplified representation focused on command relationships:

- **Segments** separated by reset points (`&&`, `||`, `;`)
- Each segment is either a **pipeline** (commands connected by `|`) or a **single** command
- **Nested commands** from `find -exec`, `xargs`, `bash -c`, `eval` are recursively parsed

### 3. Rule Evaluation

Each segment is evaluated independently against the DSL rules:

1. The first command in a pipeline is matched against top-level rules
2. Subsequent piped commands are matched against the parent command's `|,>>` context rules
3. Nested commands (from `-exec`, `xargs`, etc.) are matched against `exec:` context rules
4. **Last-rule-wins**: if multiple rules match, the last one takes precedence
5. Template chains (`next:`, `extends:`) are expanded during matching

### 4. Decision Output

The evaluation produces one of five actions:

| Action | Exit Code | Effect |
|---|---|---|
| `allow` | 0 | Command proceeds |
| `deny` | 2 + stderr message | Command blocked, Claude sees the reason |
| `warn` | 0 + stdout JSON | Command proceeds, Claude sees a warning |
| `ask` | 0 + `{"decision":"ask"}` | Delegates to Claude Code's permission dialog |

## Performance

All processing happens in-memory in a single binary. Typical latency:

| Operation | Time |
|---|---|
| Shell AST parse | ~2 μs |
| Topology build | ~9 μs |
| Rule evaluation | ~3 μs |
| **Total (end to end)** | **~5 μs** |

This is fast enough that the hook adds no perceptible delay to Claude Code's operation.
