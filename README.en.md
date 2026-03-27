# EnumaElish — ccchain

> Claude Code Chain: Structural Permission Control Tool

[![Documentation](https://img.shields.io/badge/docs-VitePress-5f67ee)](https://fruitriin.github.io/EnumaElish/)

A Go single-binary tool that extends Claude Code's standard permission system with structural context awareness — controlling allow/deny decisions based on shell command structure (pipes, chains, subshells).

## Quick Start

```bash
go install github.com/fruitriin/ccchain/cmd/ccchain@latest
ccchain init
ccchain eval "find . | rm"
# → {"action":"deny","message":"don't pipe into destructive commands",...}
```

## Why ccchain?

Claude Code's `settings.json` permissions can only do prefix matching. They can't see inside `find -exec`, after `&&` chains, or through pipes like `curl | bash`.

ccchain parses the full shell AST using `mvdan.cc/sh` (the parser behind `shfmt`) and evaluates commands in their structural context.

## DSL Example

```
allow find
  |,>>
    deny rm  "don't combine find with rm"
  exec:
    deny rm  "expand to tempfile first"

allow curl
  |
    deny bash  "curl | bash is not allowed"

deny rm
```

## Features

- **Structural context** — tracks pipes, redirects, subshells as nested context
- **Reset semantics** — `&&` / `;` evaluate commands independently
- **Templates** — share rules via `extends` / `next`
- **Auditable** — `ccchain audit` shows flat expansion of all rules
- **AI-guided deny** — block messages enable Claude's self-correction
- **Single binary** — Go, only dependency is `mvdan.cc/sh`
- **~5μs latency** — zero perceptible overhead

## Documentation

**[https://fruitriin.github.io/EnumaElish/](https://fruitriin.github.io/EnumaElish/)**

## License

MIT
