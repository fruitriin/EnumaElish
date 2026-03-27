# Quick Start

## 1. Generate default config

```bash
ccchain init
```

This creates `.ccchain.conf` with sensible defaults.

## 2. Register the hook

Add to `.claude/settings.json`:

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
  }
}
```

## 3. Verify

Check that your config is valid:

```bash
ccchain check
```

See the expanded rules:

```bash
ccchain audit
```

Test a specific command:

```bash
ccchain eval "find . | rm"
# → {"action":"deny","message":"don't pipe into destructive commands",...}

ccchain eval "ls -la | head"
# → {"action":"allow",...}
```

## 4. Tune rules for your project

The default ruleset covers common dangerous patterns (`find | rm`, `curl | bash`, `eval`, etc.), but most project-specific commands will fall through to `ask` (user confirmation).

### Step 1: Collect your project's commands

List the commands your project typically uses:

```bash
# Evaluate each command against current rules
ccchain eval "go test ./..."
ccchain eval "npm run build"
ccchain eval "git status"
ccchain eval "make test"
# ... all commands that Claude runs in your project
```

Or use `ccchain suggest` as a starting point:

```bash
echo "go test ./...
npm run build
git status
make test
cat README.md" | ccchain suggest
```

### Step 2: Ask Claude to propose rules

Give Claude (or any LLM) the evaluation results and ask it to propose `.ccchain.conf` additions. The LLM can assess which commands are safe to allow based on your project context.

### Step 3: Security review

**This step is mandatory.** Before applying any suggested rules, run a security review:

> Have a security review agent audit the proposed rules. The reviewer should check:
> - Whether any `allow` rule could be exploited in pipe/exec context
> - Whether allowing a command at top level creates bypass paths for destructive operations
> - Whether the suggestion adequately considers the command's side effects

The security reviewer may reject or modify suggestions. Revise the rules based on their feedback before adding them to `.ccchain.conf`.

### Step 4: Apply and verify

Add the reviewed rules to `.ccchain.conf`:

```
# Project build tools (reviewed and approved)
allow go
allow npm
allow make
allow cargo

# Safe utilities
allow cat
allow cp
allow mkdir
allow echo
allow pwd
allow diff
allow which

# Git
allow git
```

Then verify:

```bash
ccchain check     # Syntax OK
ccchain audit     # Review expanded rules
```

## 5. Advanced: Pipe/exec rules

For project-specific tools, add structural context rules:

```
allow npm
  |
    deny rm  "don't pipe npm output into rm"

allow docker
  exec:
    deny rm  "don't exec rm inside docker"
```

## 6. Personal overrides

Create `.ccchain.local.conf` for personal preferences (add to `.gitignore`):

```
# I'm comfortable with rm
allow rm
```
