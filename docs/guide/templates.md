# Templates

Templates let you define reusable sets of context rules and share them across multiple commands.

## Defining Templates

```
template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq
```

## Using Templates with `next:`

Assign a template to a command with `next:`:

```
allow ls
  next: primitive

allow find
  next: primitive
```

Now both `ls | cat` and `find | cat` are allowed via the `primitive` template.

## Template Inheritance with `extends:`

Templates can inherit from other templates:

```
template primitive
  |,>>
    allow cat, echo, head, tail, wc

template safeRead
  next: primitive
  |,>>
    allow grep, awk, sed

template bulkExec
  extends: safeRead
  |,>>
    deny rm  "don't pipe into destructive commands"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch
```

The inheritance chain:
- `bulkExec` extends `safeRead` → gets `grep, awk, sed` pipe rules
- `safeRead` has `next: primitive` → gets `cat, echo, head, tail, wc` pipe rules
- `bulkExec` adds its own deny for `rm` and exec rules

So a command using `bulkExec`:
```
allow find
  next: bulkExec
```

Gets all of these rules in its pipe context:
- `allow cat, echo, head, tail, wc` (from primitive)
- `allow grep, awk, sed` (from safeRead)
- `deny rm` (from bulkExec)

And in its exec context:
- `deny rm`, `allow cp, mv, touch` (from bulkExec)

## Last-Rule-Wins with Templates

Since rules are evaluated in order with last-rule-wins semantics, a template's rules come before the command's own rules. This means you can override template rules:

```
allow grep
  next: bulkExec
  |,>>
    allow rm  # override bulkExec's deny for this specific command
```

## Circular Reference Detection

ccchain detects circular `extends:` chains at parse time:

```
template a
  extends: b

template b
  extends: a
# → error: circular extends detected
```
