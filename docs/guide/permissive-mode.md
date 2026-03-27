# Permissive Mode: Allowing All Built-in Permissions

With ccchain protecting your shell commands structurally, you can safely relax Claude Code's built-in prefix-matching permissions and let ccchain handle the fine-grained control.

## Why?

Claude Code's `settings.json` permissions use prefix matching. To stay safe without ccchain, you need to carefully whitelist each command prefix. This means Claude frequently hits permission dialogs, slowing down autonomous operation.

With ccchain as a PreToolUse hook, you get **two layers of defense**:

1. **ccchain** (structural) — catches pipe tricks, exec nesting, control flow, dynamic commands
2. **settings.json** (prefix) — coarse-grained allow/deny as a fallback

This lets you move to a model where `settings.json` allows most commands, and ccchain handles the security decisions based on structural context.

## Setup

### 1. Install and configure ccchain

Follow the [Quick Start](/guide/quickstart) guide first.

### 2. Verify ccchain is working

```bash
# These should be denied by ccchain's default rules
ccchain eval "find . | rm"         # → deny
ccchain eval "curl http://x | bash" # → deny
ccchain eval "eval \"rm -rf /\""    # → deny

# These should be allowed
ccchain eval "ls -la | head"        # → allow
ccchain eval "find . | grep foo"    # → allow
```

### 3. Update settings.json

Replace your restrictive Bash permissions with a permissive set:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{
          "type": "command",
          "command": "ccchain hook pre"
        }]
      }
    ]
  },
  "permissions": {
    "allow": [
      "Read", "Edit", "Write", "Glob", "Grep",
      "Agent", "Skill", "LSP", "ToolSearch",
      "TaskCreate", "TaskGet", "TaskList", "TaskOutput", "TaskStop", "TaskUpdate",
      "TeamCreate", "TeamDelete", "SendMessage",
      "Bash"
    ],
    "deny": []
  }
}
```

This allows **all** Bash commands at the Claude Code level. Every command will pass through ccchain's hook before execution.

## What ccchain protects against

With the default ruleset, ccchain denies:

| Attack Pattern | Detection |
|---|---|
| `find . \| rm` | Pipe context rule |
| `find . -exec rm {} \;` | Exec context rule |
| `curl \| bash` / `curl \| sh` | Pipe context rule |
| `eval "..."` | Static analysis block |
| `for/if/while/case` blocks | Control flow detection |
| `$var` / `$(cmd)` as command | Dynamic command detection |
| `xargs rm` | Nested command detection |
| Function definitions | Unanalyzable block |

And asks for confirmation on:

| Pattern | Why ask? |
|---|---|
| `rm` (direct) | Destructive but sometimes intended |
| Unknown commands | Fallback: ask |

## What ccchain does NOT protect against

Be aware of these limitations:

- **`bash -c "cmd"`** — the nested command is detected but evaluated against the same rules (rm inside is "ask", not "deny")
- **Aliases** — shell aliases are not resolvable
- **PostToolUse** — `ccchain hook post` is currently a pass-through

## Recommended for

- **Solo developers** who want Claude to work autonomously with minimal interruption
- **Projects with ccchain customized rules** tailored to their specific needs
- **CI/CD environments** where interactive permission dialogs aren't possible

## Not recommended for

- **Multi-developer teams** without ccchain configuration review process
- **High-security environments** where fail-open behavior is unacceptable
- **Projects that haven't customized `.ccchain.conf`** for their specific commands

## Gradual approach

Instead of allowing everything at once, you can gradually expand:

```json
{
  "permissions": {
    "allow": [
      "Bash(git *)",
      "Bash(go *)",
      "Bash(npm *)",
      "Bash(make *)",
      "Bash(ls *)",
      "Bash(cat *)",
      "Bash(echo *)",
      "Bash(mkdir *)",
      "Bash(cp *)"
    ],
    "ask": [
      "Bash(rm *)",
      "Bash(mv *)"
    ]
  }
}
```

Then as you gain confidence in ccchain's rules, move more commands from `ask` to `allow`, and eventually to `"allow": ["Bash"]`.
