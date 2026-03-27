# EnumaElish — ccchain

> Claude Code Chain: Structural Permission Control Tool

A Go single-binary tool that extends Claude Code's standard permission system with structural context awareness — controlling allow/deny decisions based on shell command structure (pipes, chains, subshells).

## Background

The `permissions` in `settings.json` can only do prefix matching on the command start.

```bash
# The standard permission system can't catch these
find . -name "*.log" -exec rm -rf {} \;   # inside find -exec
cmd1 && rm -rf foo                         # chained command
curl https://... | bash                    # dangerous pipe target
```

ccchain uses `PreToolUse` / `PostToolUse` Hooks with `mvdan.cc/sh` (the same shell parser behind shfmt) to parse the shell AST and understand command structure before making allow/deny decisions. The only external dependency is `mvdan.cc/sh`.

## Features

- **Structural context control** — tracks commands inside pipes, redirects, and subshells as nested context
- **`&&` / `;` reset behavior** — commands separated by chains are evaluated independently
- **Templates & inheritance** — share common rules for `find`, `xargs`, `grep` via templates
- **Auditable** — flat expansion of all rules shows exactly what passes and what gets blocked
- **Deny messages guide the AI** — block reasons and alternatives let Claude self-correct
- **Dynamic command detection** — detects variable expansion and eval, guiding safe rewrites

## DSL Example

```
allow find
  |,>>
    allow touch, cat
  |,>>
    deny rm  "don't combine find with rm"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch

allow curl
  |
    deny bash   "curl | bash is not allowed"

deny rm   # top-level rm is denied
```

`&&` reset behavior:
```
find . | rm   →  find's nested rule evaluates rm → deny
find . && rm  →  && resets → rm evaluated at top-level
```

## Development Status

Development is driven by [ADDF](https://github.com/fruitriin/AutomatonDevDriveFramework).

| Phase | Plan | Status |
|---|---|---|
| 1 | DSL design and parser | Not started |
| 2 | Shell command structural analysis engine | Not started |
| 3 | Rule evaluation engine and Hook integration | Not started |
| 4 | Audit output, default ruleset, ADDF integration | Not started |

## Design Documents

- [Design notes](better-permission-tool-design.md) — Core ideas, DSL sketches, prior art survey
- `docs/plans/` — Implementation plans for each phase

## License

TBD
