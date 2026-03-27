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

The default ruleset covers common patterns (`find`, `ls`, `grep`, `curl`), but your project's build tools and workflows will initially fall through to `ask` (user confirmation).

Use `ccchain suggest` to automatically generate rules based on the commands your project actually uses:

```bash
# Feed your typical commands to suggest
echo "go test ./...
go build ./cmd/myapp
npm run build
git status
git push origin main
make test
python3 script.py
cat README.md
cp src dst
mkdir -p /tmp/build" | ccchain suggest
```

Output:

```
# Suggested rules for .ccchain.conf
# Commands that currently fall through to 'ask' but appear safe:

allow cat
allow cp
allow mkdir
# ask go  # review before allowing
# ask npm  # review before allowing
# ask git  # review before allowing
# ask make  # review before allowing

# ---
# 7 commands would benefit from explicit rules
```

- **`allow` lines** are commands ccchain recognizes as generally safe — copy them directly into `.ccchain.conf`
- **`# ask` lines** are project-specific tools — review and decide whether to allow them

Then add them to your config:

```
# .ccchain.conf — append after the default rules

# Safe utilities
allow cat
allow cp
allow mkdir
allow echo
allow pwd
allow diff
allow which

# Project build tools
allow go
allow npm
allow make
allow cargo

# Git (read operations are safe)
allow git
```

## 5. Advanced: Customize pipe/exec rules

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
