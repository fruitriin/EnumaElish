package main

import (
	"fmt"
	"os"

	"github.com/fruitriin/EnumaElish/internal/preset"
)

const defaultConfig = `# === ccchain Default Rules ===
# Claude Code Chain: structural permission control
# https://github.com/fruitriin/EnumaElish

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
# v0.2.1: expanded to cover common read-only utilities so that fallback: ask
# (which degrades to deny in auto/dontAsk modes) does not silently block
# everyday commands. Add or remove entries to match your workflow.
allow cat
  next: primitive
allow echo
allow pwd
allow diff
allow which
allow mkdir
allow wc
allow sort
allow uniq
allow head
allow tail
allow sed
allow awk
allow cut
allow tr
allow tee
allow basename
allow dirname
allow date
allow env
allow printf
allow seq
allow test
allow file
allow stat
allow tree
allow readlink
allow cp
allow chmod
  args:
    -R|--recursive: ask  "recursive chmod can affect many files"
    777: deny  "world-writable permissions (777) is dangerous"

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
    \.ccchain(\.local)?\.conf$: deny  "editing ccchain's own configuration would let the agent disable its safety rules. Owner: edit the file directly (attacker C7)."

allow Write
  args:
    \.env$|\.env\.: deny  ".env contains secrets. Write to .env.example instead"
    node_modules/: deny  "Don't write to node_modules. Modify package.json"
    \.ccchain(\.local)?\.conf$: deny  "writing to ccchain's own configuration would let the agent disable its safety rules. Owner: edit the file directly (attacker C7)."
`

// sentinelConfig is the deny-first curated preset delivered by
// `ccchain init --sentinel`. The canonical source lives in
// internal/preset/sentinel.conf (embedded via go:embed) so the shipped
// config and the config exercised by fixture tests are guaranteed to be
// the same bytes — avoiding drift between the two.
//
// Rule provenance and the escape-hatch guidance are covered in
// docs/knowhow/dsl-rule-design.md (args: regex pitfalls) and
// Plan 0022 Phase 4 (the collection).
var sentinelConfig = preset.Sentinel()

// runInit writes the default (non-sentinel) preset to .ccchain.conf.
// Preserved as-is for backward compatibility with existing docs and users
// who rely on the untouched behavior of ` + "`ccchain init`" + `.
func runInit() {
	writePreset(".ccchain.conf", defaultConfig, "default")
}

// runInitSentinel writes the sentinel (deny-first) preset. See
// sentinelConfig for the rule collection and rationale.
func runInitSentinel() {
	writePreset(".ccchain.conf", sentinelConfig, "sentinel")
}

func writePreset(path, content, presetName string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "%s already exists. Remove it first to reinitialize.\n", path)
		os.Exit(1)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", path, err)
		os.Exit(1)
	}

	fmt.Printf("created %s (%s preset)\n", path, presetName)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Review and customize %s\n", path)
	fmt.Println("  2. Add to .claude/settings.json:")
	fmt.Println(`     "hooks": {`)
	fmt.Println(`       "PreToolUse": [{`)
	fmt.Println(`         "matcher": "Bash",`)
	fmt.Println(`         "hooks": [{"type": "command", "command": "ccchain hook pre"}]`)
	fmt.Println(`       }]`)
	fmt.Println(`     }`)
	fmt.Println("  3. Run 'ccchain check' to validate")
	fmt.Println("  4. Run 'ccchain audit' to see expanded rules")
	fmt.Println("  5. (Optional) Persist hook decisions for `ccchain stats`:")
	fmt.Println("       settings:")
	fmt.Println("         log: .ccchain/log.jsonl")
	fmt.Println("       # Add `.ccchain/` to .gitignore — command strings can carry secrets.")
	if presetName == "sentinel" {
		fmt.Println()
		fmt.Println("The sentinel preset is deny-first: destructive patterns produce a deny")
		fmt.Println("with an explanatory message (why + how to accomplish safely). Approvals")
		fmt.Println("must come from the human owner running the command interactively.")
		fmt.Println()
		fmt.Println("Recommended defense in depth — add this to .claude/settings.json to")
		fmt.Println(`prevent the agent from ever invoking "ccchain approve" via its own Bash:`)
		fmt.Println(`     "permissions": {`)
		fmt.Println(`       "deny": ["Bash(ccchain approve*)"]`)
		fmt.Println(`     }`)
		fmt.Println("Security H4: ccchain enforces the same fence via the sentinel preset,")
		fmt.Println("but relying on ccchain to guard itself is a self-referential loop — the")
		fmt.Println("settings.json deny is the belt-and-suspenders layer.")
	}
}
