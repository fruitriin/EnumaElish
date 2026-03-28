// Package semantics provides a built-in knowledge table of CLI command behaviors.
// It maps command names to their semantic properties (safe/dangerous subcommands,
// destructive options, etc.) and can generate ccchain DSL rules from this knowledge.
package semantics

import (
	"fmt"
	"strings"
)

// CommandSemantics describes the semantic properties of a CLI command.
type CommandSemantics struct {
	SafeSubcommands      []string // subcommands that are read-only or safe
	DangerousSubcommands []string // subcommands that execute code or have destructive effects
	DestructiveArgs      []string // flags/options that make the command destructive
	ExecutesCode         bool     // whether the command can execute arbitrary code
	DefaultAction        string   // "allow", "ask", or "" (inherit)
	UnknownAction        string   // action for unknown subcommands ("ask" recommended)
}

// Table is the built-in semantics table for common CLI tools.
var Table = map[string]CommandSemantics{
	// Version control
	"git": {
		SafeSubcommands:      []string{"status", "log", "diff", "show", "branch", "tag", "stash", "ls-files", "remote", "rev-parse", "worktree", "shortlog", "describe"},
		DangerousSubcommands: []string{"filter-branch", "filter-repo"},
		DefaultAction:        "allow",
		UnknownAction:        "allow",
	},

	// Go
	"go": {
		SafeSubcommands:      []string{"test", "vet", "build", "mod", "version", "fmt", "env", "doc", "tool", "clean"},
		DangerousSubcommands: []string{"run", "generate"},
		ExecutesCode:         true,
		DefaultAction:        "allow",
		UnknownAction:        "ask",
	},

	// Node.js ecosystem
	"npm": {
		SafeSubcommands:      []string{"test", "run", "version", "ls", "outdated", "audit", "ci", "view", "info", "explain", "pack"},
		DangerousSubcommands: []string{"install", "publish", "unpublish", "exec"},
		ExecutesCode:         true,
		DefaultAction:        "allow",
		UnknownAction:        "ask",
	},
	"npx": {
		ExecutesCode:  true,
		DefaultAction: "ask",
		UnknownAction: "ask",
	},
	"yarn": {
		SafeSubcommands:      []string{"test", "run", "version", "info", "list", "outdated", "audit", "why"},
		DangerousSubcommands: []string{"add", "install", "publish"},
		ExecutesCode:         true,
		DefaultAction:        "allow",
		UnknownAction:        "ask",
	},
	"pnpm": {
		SafeSubcommands:      []string{"test", "run", "list", "outdated", "audit", "why"},
		DangerousSubcommands: []string{"add", "install", "publish"},
		ExecutesCode:         true,
		DefaultAction:        "allow",
		UnknownAction:        "ask",
	},

	// Rust
	"cargo": {
		SafeSubcommands:      []string{"build", "test", "check", "clippy", "fmt", "doc", "metadata", "tree", "version"},
		DangerousSubcommands: []string{"install", "publish", "run"},
		ExecutesCode:         true,
		DefaultAction:        "allow",
		UnknownAction:        "ask",
	},

	// Python
	"pip": {
		DangerousSubcommands: []string{"install", "uninstall"},
		ExecutesCode:         true,
		DefaultAction:        "ask",
		UnknownAction:        "ask",
	},
	"uv": {
		SafeSubcommands:      []string{"pip", "venv", "tool", "version"},
		DangerousSubcommands: []string{"run"},
		ExecutesCode:         true,
		DefaultAction:        "allow",
		UnknownAction:        "ask",
	},

	// Containers & orchestration
	"docker": {
		SafeSubcommands:      []string{"ps", "images", "inspect", "logs", "stats", "version", "info"},
		DangerousSubcommands: []string{"run", "exec", "build", "rm", "rmi", "system prune"},
		ExecutesCode:         true,
		DefaultAction:        "ask",
		UnknownAction:        "ask",
	},
	"kubectl": {
		SafeSubcommands:      []string{"get", "describe", "logs", "diff", "version", "config view", "api-resources"},
		DangerousSubcommands: []string{"delete", "exec", "apply", "patch", "replace", "create", "run", "edit"},
		DefaultAction:        "ask",
		UnknownAction:        "ask",
	},

	// Text processing (destructive options)
	"sed": {
		DestructiveArgs: []string{"-i", "--in-place"},
		DefaultAction:   "allow",
	},
	"chmod": {
		DestructiveArgs: []string{"-R", "--recursive"},
		DefaultAction:   "allow",
	},
	"chown": {
		DestructiveArgs: []string{"-R", "--recursive"},
		DefaultAction:   "ask",
	},

	// Cloud CLIs
	"aws": {
		SafeSubcommands:      []string{"sts get-caller-identity", "s3 ls", "ec2 describe-instances"},
		DangerousSubcommands: []string{"s3 rm", "ec2 terminate-instances", "iam delete-user"},
		DefaultAction:        "ask",
		UnknownAction:        "ask",
	},
	"gcloud": {
		DangerousSubcommands: []string{"compute instances delete", "projects delete"},
		DefaultAction:        "ask",
		UnknownAction:        "ask",
	},
	"terraform": {
		SafeSubcommands:      []string{"plan", "show", "state list", "validate", "fmt", "version"},
		DangerousSubcommands: []string{"destroy", "apply"},
		DefaultAction:        "ask",
		UnknownAction:        "ask",
	},

	// Dangerous standalone commands
	"dd": {
		DefaultAction: "deny",
	},
	"mkfs": {
		DefaultAction: "deny",
	},
	"nc": {
		DestructiveArgs: []string{"-e", "-c"},
		DefaultAction:   "ask",
	},
}

// normalizeSubcommands converts space-separated subcommands to regex-safe patterns.
// "system prune" → "system\\s+prune"
func normalizeSubcommands(subs []string) string {
	normalized := make([]string, len(subs))
	for i, sub := range subs {
		normalized[i] = strings.ReplaceAll(sub, " ", "\\s+")
	}
	return strings.Join(normalized, "|")
}

// GenerateRules generates ccchain DSL rules from the semantics table.
func GenerateRules() string {
	var sb strings.Builder

	sb.WriteString("# Generated by ccchain generate-rules --from-semantics\n")
	sb.WriteString("# Review and customize before adding to .ccchain.conf\n\n")

	for name, sem := range Table {
		action := sem.DefaultAction
		if action == "" {
			action = "ask"
		}

		sb.WriteString(fmt.Sprintf("%s %s\n", action, name))

		hasArgs := len(sem.SafeSubcommands) > 0 || len(sem.DangerousSubcommands) > 0 || len(sem.DestructiveArgs) > 0
		if hasArgs {
			sb.WriteString("  args:\n")
		}

		if len(sem.SafeSubcommands) > 0 {
			pattern := "^(" + normalizeSubcommands(sem.SafeSubcommands) + ")\\b"
			sb.WriteString(fmt.Sprintf("    %s: allow\n", pattern))
		}

		if len(sem.DangerousSubcommands) > 0 {
			dangerAction := "ask"
			if sem.ExecutesCode {
				dangerAction = "ask"
			}
			pattern := "^(" + normalizeSubcommands(sem.DangerousSubcommands) + ")\\b"
			msg := name + " subcommand can have significant effects"
			sb.WriteString(fmt.Sprintf("    %s: %s  \"%s\"\n", pattern, dangerAction, msg))
		}

		if len(sem.DestructiveArgs) > 0 {
			pattern := strings.Join(sem.DestructiveArgs, "|")
			msg := name + " with these flags can be destructive"
			sb.WriteString(fmt.Sprintf("    %s: ask  \"%s\"\n", pattern, msg))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
