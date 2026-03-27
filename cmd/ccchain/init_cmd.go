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
    deny rm    "Don't pipe into rm. Instead: redirect to /tmp/targets.txt, review, then xargs rm < /tmp/targets.txt"
  exec:
    deny rm    "Don't rm inside -exec. Instead: find ... -print > /tmp/targets.txt, review, then xargs rm < /tmp/targets.txt"
    allow cp, mv, touch

# --- PreToolUse Rules ---

preToolUse

# --- Safe Utilities (no side effects) ---
allow cat
  next: primitive
allow echo
allow pwd
allow diff
allow which
allow mkdir
allow wc
allow sort
allow head
allow tail
allow cp
allow chmod

# --- Search & Processing ---
allow ls
  next: primitive

allow find
  next: bulkExec
  args:
    -delete: deny  "find -delete is destructive. Instead: find ... -print > /tmp/targets.txt, review the list, then xargs rm < /tmp/targets.txt"

allow xargs
  next: bulkExec

allow grep
  next: safeRead

# --- Version Control ---
allow git
  args:
    ^(status|log|diff|show|branch|tag|stash|ls-files|remote|rev-parse|worktree)\b: allow
    ^(add|commit|checkout|merge|rebase|fetch|pull|clone)\b: allow
    ^push\b: ask  "git push requires confirmation"
    ^(filter-branch|filter-repo)\b: deny  "arbitrary code execution risk"
    ^config\b.*(editor|pager|hook): deny  "code execution via config"

# --- Build Tools ---
allow go
  args:
    ^(test|vet|build|mod|version|fmt|env|doc|tool)\b: allow
    ^(run|generate)\b: ask  "go run/generate can execute arbitrary code"

allow make
allow npm
  args:
    ^(test|run|version|ls|outdated|audit|ci)\b: allow
    ^install\b: ask  "npm install runs postinstall scripts"
    ^(publish|unpublish)\b: ask  "npm publish affects the registry"

allow cargo

# --- Destructive Commands ---
ask rm
  message: "confirm file deletion"

# --- Network ---
allow curl
  |
    deny bash   "curl | bash is not allowed"
    deny sh     "curl | sh is not allowed"
  args:
    -o\b|--output: ask  "curl writing to file requires confirmation"

# --- Dangerous ---
deny eval       "eval is not statically analyzable; write the command directly"

# --- Path Protection (deny-redirect pattern) ---
# Protect sensitive files by denying access and suggesting alternatives

allow Read
  args:
    \.env$|\.env\.: deny  ".env contains secrets. Read .env.example instead"
    \.ssh/|\.gnupg/: deny  "SSH/GPG keys should not be accessed by AI"
    node_modules/: deny  "Don't read node_modules directly. Check package.json instead"

allow Edit
  args:
    \.env$|\.env\.: deny  ".env contains secrets. Edit .env.example instead"
    node_modules/: deny  "Don't edit node_modules. Modify package.json and run npm install"
    dist/|build/|out/: deny  "Don't edit build artifacts. Modify source code instead"

allow Write
  args:
    \.env$|\.env\.: deny  ".env contains secrets. Write to .env.example instead"
    node_modules/: deny  "Don't write to node_modules. Modify package.json"
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
