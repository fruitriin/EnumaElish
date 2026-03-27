# CLI Commands

## `ccchain check`

Validates the configuration file syntax.

```bash
ccchain check
ccchain check --config path/to/config.conf
ccchain check -v  # verbose: show parsed rules and templates
```

## `ccchain hook pre`

PreToolUse hook. Reads Claude Code's tool JSON from stdin, evaluates the command, and returns the appropriate exit code.

```bash
# Registered in .claude/settings.json — not called directly
echo '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}' | ccchain hook pre
```

| Exit Code | Meaning |
|---|---|
| 0 | Allow (or non-Bash tool) |
| 2 | Deny (reason on stderr) |

## `ccchain hook post`

PostToolUse hook. Currently a pass-through for future use (hint actions, turn counting).

## `ccchain eval "command"`

Evaluates a command and outputs the result as JSON. Useful for debugging and scripting.

```bash
ccchain eval "find . | rm"
```

```json
{
  "action": "deny",
  "message": "don't pipe into destructive commands",
  "template": "bulkExec",
  "context": ["find", "|", "rm"]
}
```

## `ccchain audit`

Displays a flat expansion of all rules, showing exactly what each command+context combination resolves to.

```bash
ccchain audit
ccchain audit --config path/to/config.conf
```

Example output:
```
[allow]  ls
[allow]  ls | cat            (template: primitive)
[allow]  find
[deny]   find | rm           (template: bulkExec)  "don't pipe into destructive"
[deny]   find -exec rm       (template: bulkExec.exec)  "expand to tempfile first"
[---]    find && ...         (&&: reset → top-level rules)

Settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask

Stats:
  rules: 8
  templates: 3
```

## `ccchain suggest`

Analyzes commands and suggests rules for those that fall through to the `ask` fallback. Useful for discovering which commands need explicit rules.

```bash
# From command arguments
ccchain suggest "ls -la" "cat foo.txt" "rm -rf /"

# From a file (one command per line)
cat commands.txt | ccchain suggest
```

## `ccchain init`

Generates a default `.ccchain.conf` with sensible rules.

```bash
ccchain init
```

Will not overwrite an existing file.

## Global Flags

| Flag | Description |
|---|---|
| `--config <path>` | Explicit config file path |
| `--default-action <action>` | Override fallback action for unmatched commands (`allow`, `deny`, `ask`) |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Errors only |
| `--version` | Print version |
| `-h, --help` | Show help |
