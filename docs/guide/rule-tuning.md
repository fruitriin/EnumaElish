# Rule Tuning Guide

This guide shows how to tune ccchain rules for your project, based on a security-reviewed process.

## The Process

1. **Evaluate** — Run `ccchain eval` against your project's commands
2. **Propose** — Ask an LLM to suggest rules based on the results
3. **Security Review** — Have a security reviewer audit the proposals
4. **Apply** — Add the approved rules to `.ccchain.conf`

## Security-Reviewed Command Categories

The following categorization is based on a formal security review. Use it as a starting point for your project.

### Safe to Allow (Low Risk)

These commands are read-only or have minimal side effects:

```
allow pwd
allow diff
allow which
allow mkdir
allow echo
allow chmod
```

### Allow with Caution (Medium Risk)

These are generally safe but have edge cases:

```
# cat: can read sensitive files — consider args: rules for protection
allow cat

# cp: can overwrite files — generally OK in dev context
allow cp
```

### Keep as Ask (High Risk — Requires User Confirmation)

These commands can execute arbitrary code or have significant side effects:

```
# go: go run/generate execute arbitrary code
# npm: install runs postinstall scripts
# make: targets can run anything
# git: hooks, filter-branch, config can execute code
ask go
ask npm
ask make
ask git
```

If you want to reduce permission dialogs for these, use `args:` rules to allow safe subcommands only:

```
allow go
  args:
    ^(test|vet|build|mod|version|fmt|doc|env)\b: allow
    run|generate: ask  "go run/generate can execute arbitrary code"

allow git
  args:
    ^(status|log|diff|show|branch|tag|stash)\b: allow
    ^(add|commit|checkout|merge|rebase)\b: allow
    filter-branch|filter-repo: deny  "arbitrary code execution risk"
    config.*(editor|pager|hook): deny  "code execution via config"
```

### Never Allow (Critical Risk)

These are essentially equivalent to allowing all commands:

```
# bash/sh: allows arbitrary code execution
# python3/node/ruby: arbitrary code execution via -c or pipe
# These should stay as 'ask' or 'deny'
```

**Why?** If `bash` is allowed at top level:
- `echo "rm -rf /" | bash` passes through (no pipe deny rule on echo→bash)
- `bash script.sh` executes any script without content analysis
- `bash -c "$dynamic"` with dynamic args skips args: evaluation

## Option-Level Control with `args:`

ccchain's structural context catches pipes and exec nesting, but **command-line flags** that change a command's behavior are equally important. The `args:` feature enables option-level control.

### Destructive Options on Safe Commands

Some "safe" commands have destructive options:

| Command | Safe | Destructive |
|---|---|---|
| `find` | `find . -name '*.go'` | `find . -delete` |
| `curl` | `curl https://...` | `curl -o /etc/passwd https://...` |
| `cat` | `cat file.txt` | `cat file > /etc/passwd` (redirect, not an arg) |
| `python3` | `python3 script.py` | `python3 -c 'os.system("rm -rf /")'` |
| `git` | `git status` | `git filter-branch --tree-filter 'rm -rf /'` |

The default ruleset includes:

```
allow find
  args:
    -delete: deny  "find -delete is destructive"

allow curl
  args:
    -o\b|--output: ask  "curl writing to file requires confirmation"
```

### Redirects (`>`, `>>`)

Note that shell redirects (`cat file > /etc/passwd`) are **not command arguments** — they are handled by the shell before the command runs. ccchain currently detects redirects in pipe context (`|,>>`) but does not inspect redirect targets. This is a known limitation.

### Recommendations for Common Commands

```
# python3 — block inline code execution
allow python3
  args:
    -c\s: deny  "inline code execution requires review"

# git — block dangerous subcommands
allow git
  args:
    filter-branch|filter-repo: deny  "arbitrary code execution risk"
    config.*(editor|pager): deny  "code execution via config"

# node — block inline execution
allow node
  args:
    -e\s|--eval: deny  "inline code execution requires review"
```

## Example: Go Project Configuration

```
# .ccchain.conf additions for a Go project

# Safe utilities
allow cat
allow echo
allow pwd
allow diff
allow which
allow cp
allow mkdir
allow chmod

# Go — safe subcommands only
allow go
  args:
    ^(test|vet|build|mod|version|fmt|env|doc)\b: allow

# Git — safe subcommands only
allow git
  args:
    ^(status|log|diff|show|branch|tag|stash|add|commit|checkout|merge|rebase|clone|fetch|pull|push|ls-files|worktree|remote|rev-parse)\b: allow
```

## Example: Node.js Project Configuration

```
# Safe utilities (same as above)
allow cat
allow echo
allow pwd
allow diff
allow which
allow cp
allow mkdir

# npm — safe subcommands
allow npm
  args:
    ^(test|run|version|ls|outdated|audit)\b: allow

# Node.js — not recommended to allow
# ask node  # node -e can execute arbitrary code
```
