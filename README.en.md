# EnumaElish — ccchain

> **the Chain of Heaven** — the chain that once bound even divine beasts now manifests in the terminal.
>
> Enuma Elish is the Chain of Heaven, forged by mortals to bind the gods.
> It parses command-line strings, reads the structure of the shell,
> and returns exit codes and Usable Hints — driving a wedge into the omnipotent AI's actions.
>
> *— A permission that knows not the structure is no permission at all.*

[日本語](README.md)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-VitePress-5f67ee)](https://fruitriin.github.io/EnumaElish/)

A Go single-binary tool that extends Claude Code's standard permission system with **structural context awareness** — understanding pipes, chains, subshells, and `-exec` to make informed allow/deny decisions.

**It doesn't just block — it teaches.** Every `deny` in ccchain can carry a hint message. Write `deny rm -rf / "rm -rf ~/ destroys all user files"` and Claude understands *why* it was blocked and *what to do instead*, rewriting the command autonomously without human intervention. Blocking becomes a conversation — that's the design philosophy of ccchain.

## Why ccchain?

Claude Code's `settings.json` permissions only match the first command in a line:

```bash
find . -name "*.log" -exec rm -rf {} \;   # -exec content is invisible
cmd1 && rm -rf foo                          # chained commands are invisible
curl https://... | bash                     # pipe targets are invisible
```

ccchain uses [`mvdan.cc/sh`](https://github.com/mvdan/sh) (the same parser behind shfmt) to parse shell AST and evaluate commands with full structural understanding.

## Quick Start

### 1. Install

```bash
go install github.com/fruitriin/ccchain/cmd/ccchain@latest
```

### 2. Generate config

```bash
ccchain init
# → Creates .ccchain.conf
```

### 3. Register as a Claude Code Hook

Add to `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{"type": "command", "command": "ccchain hook pre"}]
    }]
  }
}
```

### 4. Verify

```bash
ccchain eval "find . | rm"
# → deny: "don't pipe into destructive commands"

ccchain eval "find . | grep foo"
# → allow
```

## DSL Example

```
allow find
  |,>>
    allow touch, cat
    deny rm  "don't combine find with rm"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch

allow curl
  |
    deny bash  "curl | bash is not allowed"

deny rm
```

### Evaluation Results

| Command | Result | Reason |
|---|---|---|
| `find . \| grep foo` | allow | grep is allowed in pipe context |
| `find . \| rm` | **deny** | rm is denied in pipe context |
| `find . && rm foo` | **deny** | `&&` resets context → top-level `deny rm` |
| `curl ... \| bash` | **deny** | bash is denied in curl's pipe context |
| `find . -exec rm {} \;` | **deny** | rm is denied in exec context |

## Features

- **Deny with hints** — Write `deny rm -rf / "rm -rf ~/ destroys all user files"` and Claude reads the hint to autonomously rewrite the command safely. Not just a guardrail — a conversation channel with AI
- **Structural context** — Tracks commands inside pipes (`|`), redirects (`>>`), subshells (`$()`), and `-exec` as nested structures
- **Reset semantics** — Commands separated by `&&` / `;` are evaluated independently
- **Templates & inheritance** — `extends` to build on existing templates, `next` to share pipe-target rules. Define common rules for `find`, `xargs`, `grep` once
- **4 actions** — `allow` / `deny` / `ask` / `warn` for flexible permission control
- **Auditable** — `ccchain audit` shows all rules after template expansion
- **Config merging** — Project, local, and global configs merged in priority order
- **Single binary** — Built with Go, only dependency is `mvdan.cc/sh`
- **~4μs** — End-to-end evaluation in ~3.8μs. Virtually zero Hook overhead (verify with `go test -bench=.`)

## Config Search Paths

Files are loaded in priority order; later files override earlier ones:

| Priority | Path | Purpose |
|---|---|---|
| 1 | `.ccchain.conf` | Project-level shared config |
| 2 | `.ccchain.local.conf` | Local override (gitignore recommended) |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | Environment variable path (absolute only) |
| 4 | `~/.claude/ccchain.conf` | Global fallback |

> **Note:** Priorities 3 and 4 are mutually exclusive — if `CLAUDE_CONFIG_DIR` is set, only 3 is loaded; otherwise only 4.

## Subcommands

| Command | Description |
|---|---|
| `ccchain init` | Generate default `.ccchain.conf` |
| `ccchain check` | Validate config file syntax |
| `ccchain eval "cmd"` | Evaluate a command and output result as JSON |
| `ccchain suggest` | Suggest rules for unmatched commands |
| `ccchain hook pre` | PreToolUse Hook (reads JSON from stdin) |
| `ccchain audit` | Display all rules after template expansion |

## Documentation

**[https://fruitriin.github.io/EnumaElish/](https://fruitriin.github.io/EnumaElish/)**

| Guide | Description |
|---|---|
| [What is ccchain?](https://fruitriin.github.io/EnumaElish/guide/) | Overview and design philosophy |
| [Installation](https://fruitriin.github.io/EnumaElish/guide/installation) | How to install |
| [Quick Start](https://fruitriin.github.io/EnumaElish/guide/quickstart) | Setup walkthrough |
| [How It Works](https://fruitriin.github.io/EnumaElish/guide/how-it-works) | Architecture and flow |
| [DSL Reference](https://fruitriin.github.io/EnumaElish/reference/dsl) | DSL syntax reference |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
