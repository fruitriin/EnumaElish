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

## 4. Customize

Edit `.ccchain.conf` to add project-specific rules:

```
# Allow your build tool
allow npm
  |
    deny rm  "don't pipe npm output into rm"

# Allow your test runner
allow pytest
```

Create `.ccchain.local.conf` for personal overrides (add to `.gitignore`):

```
# I know what I'm doing
allow rm
```
