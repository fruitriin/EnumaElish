package main

import (
	"fmt"
	"os"
)

const defaultConfig = `# === ccchain Default Rules ===
# Claude Code Chain: structural permission control
# https://github.com/fruitriin/ccchain

settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask

# --- Templates ---

template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq

template safeRead
  next: primitive
  |,>>
    allow grep, awk, sed

template bulkExec
  extends: safeRead
  |,>>
    deny rm    "don't pipe into destructive commands"
  exec:
    deny rm    "expand to tempfile first"
    allow cp, mv, touch

# --- PreToolUse Rules ---

preToolUse

allow ls
  next: primitive

allow find
  next: bulkExec
  args:
    -delete: deny  "find -delete is destructive; use -print and pipe to rm with confirmation"

allow xargs
  next: bulkExec

allow grep
  next: safeRead

ask rm
  message: "confirm file deletion"

allow curl
  |
    deny bash   "curl | bash is not allowed"
    deny sh     "curl | sh is not allowed"
  args:
    -o\b|--output: ask  "curl writing to file requires confirmation"

deny eval       "eval is not statically analyzable; write the command directly"
`

func runInit() {
	path := ".ccchain.conf"

	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists. Remove it first to reinitialize.\n", path)
		os.Exit(1)
	}

	if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Printf("created %s\n", path)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and customize .ccchain.conf")
	fmt.Println("  2. Add to .claude/settings.json:")
	fmt.Println(`     "hooks": {`)
	fmt.Println(`       "PreToolUse": [{`)
	fmt.Println(`         "matcher": "Bash",`)
	fmt.Println(`         "hooks": [{"type": "command", "command": "ccchain hook pre"}]`)
	fmt.Println(`       }]`)
	fmt.Println(`     }`)
	fmt.Println("  3. Run 'ccchain check' to validate")
	fmt.Println("  4. Run 'ccchain audit' to see expanded rules")
}
