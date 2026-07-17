# EnumaElish — ccchain

> **the Chain of Heaven** — the chain that once bound even divine beasts now manifests in the terminal.
>
> Enuma Elish is the Chain of Heaven, forged by mortals to bind the gods.
> It parses command-line strings, reads the structure of the shell,
> and returns a `permissionDecision` with a Usable Hint — driving a wedge
> into the omnipotent AI's actions.
>
> *— A permission that knows not the structure is no permission at all.*

[日本語](README.md)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-VitePress-5f67ee)](https://fruitriin.github.io/EnumaElish/)

ccchain extends Claude Code's permission system with **structural context awareness** — using an AST to understand pipes, chains, subshells, `-exec`, and literal `for` loops before deciding allow / deny. It ships as a single Go binary.

## Positioning — a deny-first safety net for the auto / bypass era

Claude Code's permission system has evolved rapidly, and non-interactive modes — `auto`, `dontAsk`, `bypassPermissions`, headless (`claude -p`) — have become the norm. Human confirmation dialogs no longer reach a person in those modes. That is where ccchain earns its keep:

- **`deny` is honoured in every mode** — including `bypassPermissions` and `dontAsk`. That is ccchain's headline value
- **Deterministic decisions from AST analysis** — `auto` mode's classifier is probabilistic, but ccchain always blocks the same structural patterns (`curl | bash`, `find -exec rm`, `git push --force main`, ...)
- **Ask is used where it can actually reach a human**. Everywhere else it becomes `deny + hint` plus an [approval token](https://fruitriin.github.io/EnumaElish/reference/approve) (`ccchain approve`), turning the block into an asynchronous conversation — see the [Mode × Decision table](#mode-decision-table-plan-0022) and the [`ask_strategy` reference](https://fruitriin.github.io/EnumaElish/reference/dsl#ask_strategy)

**It doesn't just block — it teaches.** Every `deny` in ccchain can carry a hint message. Write `deny rm -rf / "rm -rf ~/ destroys all user files"` and Claude understands *why* it was blocked and *what to do instead*, rewriting the command autonomously. Blocking becomes a conversation — that's the design philosophy of ccchain.

## Difference from prefix matching

Claude Code's `settings.json` permissions match only the first command in a line:

```bash
find . -name "*.log" -exec rm -rf {} \;   # -exec content is invisible
cmd1 && rm -rf foo                        # chained commands are invisible
curl https://... | bash                   # pipe targets are invisible
```

ccchain uses [`mvdan.cc/sh`](https://github.com/mvdan/sh) (the same parser behind shfmt) to parse the shell AST and evaluate commands with full structural understanding.

## Quick Start

### 1. Install

```bash
go install github.com/fruitriin/EnumaElish/cmd/ccchain@latest
```

### 2. Generate config

**Recommended: the sentinel preset** (curated deny-first rules for the patterns Claude Code's classifier cannot reliably catch in auto / dontAsk / headless):

```bash
ccchain init --sentinel
# → Writes a .ccchain.conf denying "curl | bash", "find -exec rm",
#   "git push --force main", and more.
```

Or start from a minimal skeleton:

```bash
ccchain init
# → Writes a minimal .ccchain.conf
```

Neither variant overwrites an existing file.

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
ccchain eval "curl https://example.com/install.sh | bash"
# → deny: "sentinel: `curl ... | bash` executes remote code without review..."

ccchain eval "find . | grep foo"
# → allow
```

## Mode × Decision Table (Plan 0022)

How each ccchain action resolves for Claude Code's [permission_mode](https://docs.claude.com/en/docs/claude-code/permissions) values:

| permission_mode | `deny` | `ask` | `warn` / `hint` | `allow` |
|---|---|---|---|---|
| `default` / `acceptEdits` / `plan` | Block | Show dialog | Run + caution | Run |
| `bypassPermissions` | **Block** (deny is honoured in every mode) | Show dialog (explicit ask rules still prompt) | Run + caution | Run |
| `auto` | Block | **Degrade to `deny + hint`** (default) | Run + caution | Run |
| `dontAsk` | Block | **Degrade to `deny + hint`** (default) | Run + caution | Run |
| `headless` (`claude -p`) | Block | **Degrade to `deny + hint`** (default) | Run + caution | Run |
| Unknown / missing | Block | **Degrade to `deny + hint`** (default; safe side) | Run + caution | Run |

The degrade direction is controlled by the global `settings: ask_strategy: degrade` (default) and `ask_degrade_default: deny` (default), plus per-rule `unattended: deny|allow`. The hint that ships with a degraded deny includes the [`ccchain approve`](https://fruitriin.github.io/EnumaElish/reference/approve) procedure so the owner can unblock the exact command from their own terminal (one-shot consumption).

All actions emit exit 0 + `hookSpecificOutput.permissionDecision` JSON ([Hook Output details](https://fruitriin.github.io/EnumaElish/reference/config#hook-output)).

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
| `for f in a b; do rm $f; done` | **deny** | literal `for` loop is expanded, `rm` fires |

## Features

- **Structural context** — Tracks pipes (`|`), redirects (`>>`), subshells (`$()`), `-exec`, and literal `for` loops as nested structures
- **Deny with hints** — Deny messages arrive at Claude as `permissionDecisionReason`, letting the model rewrite the command safely on its own
- **Ask degradation in auto / dontAsk** — The Plan 0022 flow: undeliverable ask dialogs turn into `deny + hint + approval token`
- **Sentinel preset** — Curated deny-first rules for curl|bash, find -exec rm, git force-push, chmod -R 777, and friends (`ccchain init --sentinel`)
- **Reset semantics** — Commands separated by `&&` / `;` are evaluated independently
- **Workspace scope** — Path-based access control via `scope: inside / outside-read / outside-write` for Bash, Read, Edit, Write, MCP
- **Templates & inheritance** — `extends` and `next` for shared pipe/exec rules
- **Auditable** — `ccchain audit` flattens all rules; `ccchain diff a.conf b.conf` shows how rule changes shift decisions
- **Single binary** — Go, only `mvdan.cc/sh` as a runtime dependency
- **~4μs** — End-to-end evaluation in ~3.8μs. Virtually zero hook overhead

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
| `ccchain init [--sentinel]` | Generate `.ccchain.conf` (`--sentinel` for the deny-first preset) |
| `ccchain check` | Validate config file syntax |
| `ccchain eval "cmd"` | Evaluate a command and output result as JSON |
| `ccchain test [file]` | Batch-evaluate commands from a file or stdin |
| `ccchain diff a b [file]` | Compare two configs on a shared command set (CI regression checks) |
| `ccchain suggest` | Suggest rules for unmatched commands |
| `ccchain detect` | Auto-detect project type and suggest rules |
| `ccchain generate-rules` | Emit rules from the built-in semantics table |
| `ccchain audit` | Flatten all rules after template expansion |
| `ccchain approve` | Approve a degraded deny from the owner side (`--last` / `--list` / `<hash-prefix>` / `--revoke-all`) |
| `ccchain hook pre` / `hook post` | PreToolUse / PostToolUse hook (reads JSON from stdin) |
| `ccchain version` | Print version |

## Documentation

**[https://fruitriin.github.io/EnumaElish/](https://fruitriin.github.io/EnumaElish/)**

| Guide | Description |
|---|---|
| [What is ccchain?](https://fruitriin.github.io/EnumaElish/guide/) | Overview and design philosophy |
| [Installation](https://fruitriin.github.io/EnumaElish/guide/installation) | How to install |
| [Quick Start](https://fruitriin.github.io/EnumaElish/guide/quickstart) | Setup walkthrough |
| [How It Works](https://fruitriin.github.io/EnumaElish/guide/how-it-works) | Architecture and flow |
| [DSL Reference](https://fruitriin.github.io/EnumaElish/reference/dsl) | DSL syntax (including ask_strategy / unattended / unanalyzable_action) |
| [Approval Tokens](https://fruitriin.github.io/EnumaElish/reference/approve) | `ccchain approve` design, CLI, and threat model |
| [Sentinel Preset](https://fruitriin.github.io/EnumaElish/reference/sentinel) | The curated deny-first ruleset in detail |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
