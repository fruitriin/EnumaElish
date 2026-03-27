# Customization

## Config File Search Order

ccchain searches for config files in this order (later files override earlier ones):

1. `.ccchain.conf` — project root (shared, committed)
2. `.ccchain.local.conf` — local override (personal, gitignored)
3. `$CLAUDE_CONFIG_DIR/ccchain.conf` — Claude Code global config
4. `~/.claude/ccchain.conf` — fallback global config

## Project Rules (`.ccchain.conf`)

Shared rules that all team members should use:

```
# Project-specific build tools
allow npm
  |
    deny rm

allow cargo
  next: bulkExec

# Project convention: always use trash instead of rm
deny rm  "use 'trash' command instead"
allow trash
```

## Personal Overrides (`.ccchain.local.conf`)

Add to `.gitignore`, then customize:

```
# I'm a senior dev, let me rm
allow rm

# My local tools
allow brew
```

## Writing Custom Templates

```
template myProjectSafe
  |,>>
    allow jq, yq, csvkit
    deny curl  "don't pipe into curl"
  exec:
    allow node, python3

allow my-cli
  next: myProjectSafe
```

## Combining with Claude Code Permissions

ccchain operates as a **PreToolUse hook**, which runs before Claude Code's built-in permission check. The two systems complement each other:

- **Claude Code permissions** (`settings.json`): coarse-grained allow/deny by command prefix
- **ccchain**: fine-grained structural context rules

A command must pass **both** checks to execute.

Example workflow:
1. `settings.json` allows `Bash(find *)`
2. ccchain's hook evaluates `find . | rm` against structural rules
3. ccchain denies because `rm` is in find's pipe context
4. Claude sees the deny reason and rewrites the command
