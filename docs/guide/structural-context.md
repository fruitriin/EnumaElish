# Structural Context

The core innovation of ccchain is **structural context** — the ability to write rules that depend on *where* a command appears in a shell expression, not just *what* the command is.

## Pipes Build Context

When commands are connected by `|`, each command inherits the context of its parent:

```
allow find
  |,>>
    deny rm  "don't pipe find into rm"
```

| Command | Context | Result |
|---|---|---|
| `find .` | (top-level) | allow |
| `find . \| rm` | find → pipe → rm | **deny** |
| `find . \| grep foo` | find → pipe → grep | allow (no matching pipe rule) |

## Chains Reset Context

`&&`, `||`, and `;` are **reset points**. The command after a chain operator is evaluated from scratch at the top level:

```
deny rm
allow find
  |,>>
    deny rm  "don't pipe find into rm"
```

| Command | Evaluation | Result |
|---|---|---|
| `find . \| rm` | find's pipe context → deny rm | **deny** (pipe rule) |
| `find . && rm foo` | reset at `&&` → rm at top level → deny rm | **deny** (top-level rule) |
| `find . && ls` | reset at `&&` → ls at top level | allow |

This matches shell execution semantics: `&&` means "run the next command independently if the previous succeeded."

## Exec Context

Some commands run other commands as arguments. ccchain detects these patterns and evaluates the nested command in an `exec:` context:

```
allow find
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv
```

| Command | Detection | Result |
|---|---|---|
| `find . -exec rm {} \;` | `-exec` triggers exec context | **deny** |
| `find . -exec cp {} /tmp/ \;` | `-exec` triggers exec context | allow |

### Supported Nest Patterns

| Pattern | Detection |
|---|---|
| `find -exec CMD {} \;` | `-exec` / `-execdir` argument |
| `xargs CMD` | First non-flag argument |
| `bash -c "CMD"` | `-c` argument (recursively parsed) |
| `sh -c "CMD"` | `-c` argument (recursively parsed) |
| `eval "CMD"` | Argument (static strings only) |

## Unanalyzable Commands

Commands that involve dynamic expansion cannot be statically analyzed:

```bash
$cmd foo              # variable as command name
$(generate_cmd) foo   # command substitution as command name
eval "$dynamic"       # dynamic eval argument
```

These are automatically **denied** with a message explaining why:

```json
{
  "action": "deny",
  "message": "dynamic command detected: static analysis not possible"
}
```

ccchain's policy is "deny what we can't analyze," but ccchain's *own* errors
(config parse failure, template resolution failure, etc.) are fail-open by
default — see [Error Handling](../reference/config.md#error-handling-fail-open).
For environments where fail-open is unacceptable, opt into fail-closed with
[`settings.strict_config_error: true`](../reference/dsl.md#strict_config_error)
or `CCCHAIN_STRICT_CONFIG_ERROR=1`. A dynamic command is "analyzed, and safety could not be determined," not "analysis itself failed." Downstream safety mechanisms (e.g. the `args:` maximum length in `internal/eval/evaluate.go`) treat their limits the same way: exceeding a limit is "we could not verify," so the result is escalated on the safe side rather than falling back to the parent action.
