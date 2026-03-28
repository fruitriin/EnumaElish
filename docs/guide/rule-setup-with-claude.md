# Setting Up Rules with Claude Code

This guide explains how to use Claude Code session logs to build a well-tuned `.ccchain.conf` for your project.

## Overview

```
1. Collect  — Extract real commands from Claude Code session logs
2. Detect   — Auto-detect project type for initial rules
3. Test     — Evaluate commands against your rules
4. Tune     — Adjust rules based on results
5. Verify   — Security review the final ruleset
```

## Step 1: Collect Commands from Session Logs

Claude Code stores session logs in `~/.claude/projects/`. Each project directory contains `.jsonl` files with every command Claude executed.

### Extract commands from your project

```bash
# Find your project's log directory
ls ~/.claude/projects/ | grep your-project-name

# Extract all Bash commands from session logs
for f in ~/.claude/projects/-Users-you-workspace-your-project/*.jsonl; do
  grep -o '"command":"[^"]*"' "$f" 2>/dev/null
done | sed 's/"command":"//; s/"$//' | sort -u > /tmp/my-commands.txt

# Review the list
wc -l /tmp/my-commands.txt
head -20 /tmp/my-commands.txt
```

### Filter out noise

Session logs contain many internal commands. Filter to keep meaningful ones:

```bash
# Remove git internals, path-specific commands, and framework noise
cat /tmp/my-commands.txt \
  | grep -vE '^\s*$|^\\' \
  | grep -vE 'claude-501|Progresses|Progress\.md' \
  | grep -vE '^(ls|cat|cp|echo|mkdir) /' \
  | grep -vE 'tail -c|head -[0-9]' \
  | sort -u > /tmp/my-commands-filtered.txt

wc -l /tmp/my-commands-filtered.txt
```

### Collect from multiple projects

```bash
# Combine logs from several projects
for d in ~/.claude/projects/-Users-you-workspace-*; do
  for f in "$d"/*.jsonl; do
    grep -o '"command":"[^"]*"' "$f" 2>/dev/null
  done
done | sed 's/"command":"//; s/"$//' | sort -u > /tmp/all-commands.txt
```

## Step 2: Generate Initial Rules

### Auto-detect project type

```bash
ccchain detect
```

This checks for `go.mod`, `package.json`, `Cargo.toml`, etc. and outputs suggested rules based on the built-in semantics table.

### Start with defaults + detected rules

```bash
ccchain init
# Then append detected rules:
ccchain detect >> .ccchain.conf
```

## Step 3: Test Commands Against Your Rules

### Evaluate all collected commands

```bash
ccchain test /tmp/my-commands-filtered.txt
```

Output:

```
[allow]  go test ./...
[allow]  git status
[ask]    npm install
[deny]   find . | rm
[ask]    docker run ubuntu ls
...

Summary: 85 commands — allow=42, ask=30, deny=13, warn=0, error=0
```

### Compare with a stricter ruleset

```bash
ccchain test --config testdata/eval/rules-strict.conf /tmp/my-commands-filtered.txt
```

### Identify problems

Look for:
- **allow on dangerous commands** — needs a deny or ask rule
- **ask on safe commands you use frequently** — add an allow rule to reduce friction
- **deny on commands you need** — adjust args: patterns to be more specific

## Step 4: Iterate on Rules

### The tuning loop

```
Edit .ccchain.conf
    ↓
ccchain test /tmp/my-commands-filtered.txt
    ↓
Review results (any unexpected allow/deny?)
    ↓
Adjust rules
    ↓
Repeat until satisfied
```

### Example: Too many `ask` results

```bash
# See what's falling through to ask
ccchain test /tmp/my-commands-filtered.txt | grep '^\[ask\]'
```

```
[ask]    docker run ubuntu ls
[ask]    kubectl get pods
[ask]    terraform plan
```

Add rules:

```
# .ccchain.conf additions
ask docker
  args:
    ^(ps|images|inspect|logs|stats|version|info)\b: allow
    ^(run|exec|build)\b: ask  "container execution"

allow kubectl
  args:
    ^(get|describe|logs|diff|version)\b: allow
    ^(delete|exec|apply)\b: ask  "cluster modification"

allow terraform
  args:
    ^(plan|show|validate|fmt|version)\b: allow
    ^(apply|destroy)\b: ask  "infrastructure changes"
```

Test again:

```bash
ccchain test /tmp/my-commands-filtered.txt
# → docker ps → allow, docker run → ask, kubectl get → allow
```

### Example: A command is wrongly denied

```bash
ccchain test /tmp/my-commands-filtered.txt | grep '^\[deny\]'
```

```
[deny]   find . -name '*.pyc' -delete
```

If you want `find -delete` to be `ask` instead of `deny` in your project:

```
# Override in .ccchain.local.conf (personal, gitignored)
allow find
  args:
    -delete: ask  "find -delete requires confirmation (project override)"
```

## Step 5: Security Review

Before committing your rules, have Claude review them:

> Review my .ccchain.conf for security issues. Check if any `allow` rule
> could be exploited in pipe/exec context, and whether the `args:` patterns
> are specific enough to prevent bypass.

Or use the security review agent:

> Launch a security review agent to audit the proposed ccchain rules.

### Automated check

```bash
# Verify dangerous commands are never allowed
ccchain test /tmp/dangerous-commands.txt
# All should be deny or ask, never allow
```

## Tips

### Keep your command list updated

As your project evolves, re-collect commands periodically:

```bash
# Add to your project's Makefile
collect-commands:
	@for f in ~/.claude/projects/-Users-*-$(notdir $(CURDIR))/*.jsonl; do \
	  grep -o '"command":"[^"]*"' "$$f" 2>/dev/null; \
	done | sed 's/"command":"//; s/"$$//' | sort -u > testdata/eval/project-commands.txt
	@echo "Collected $$(wc -l < testdata/eval/project-commands.txt) commands"
```

### Share command lists with your team

Commit your command list to the repo so the team can test against the same set:

```
testdata/eval/
  commands.txt           # shared command fixtures
  project-commands.txt   # project-specific commands (from logs)
  rules-default.conf     # shared rules
```

### Use `ccchain suggest` as a starting point

```bash
cat /tmp/my-commands-filtered.txt | ccchain suggest
```

This identifies commands that fall through to `ask` and suggests safe ones to `allow`.
