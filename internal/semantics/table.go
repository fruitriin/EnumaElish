// Package semantics provides a built-in knowledge table of CLI command behaviors.
// It maps command names to their semantic properties (safe/dangerous subcommands,
// destructive options, etc.) and can generate ccchain DSL rules from this knowledge.
package semantics

import (
	"fmt"
	"strings"
)

// PathKind classifies a path argument as read or write access.
// Used by the workspace scope evaluator (Plan 0011 v2) to distinguish
// e.g. `cp src dst` where src is read and dst is write.
type PathKind int

const (
	// PathKindRead means the command reads from this path.
	PathKindRead PathKind = iota
	// PathKindWrite means the command writes to (creates/modifies/deletes) this path.
	PathKindWrite
	// PathKindUnknown means the command is not in the semantics table so the
	// evaluator cannot say which args are read vs write. The scope evaluator
	// MUST consider outside-write actions for these paths (Critical C3) —
	// otherwise tools like `sed -i` / `rsync` / `dd` / `wget -O` silently
	// bypass write-side rules. selectScopeAction chooses the most restrictive
	// of outside-write / outside-read / outside when it sees this kind.
	PathKindUnknown
)

// ArgSemanticsKind describes how a command's path arguments are classified.
type ArgSemanticsKind int

const (
	// KindAllRead: every path argument is a read (cat, head, tail, grep).
	KindAllRead ArgSemanticsKind = iota
	// KindAllWrite: every path argument is a write (rm, shred, tee).
	KindAllWrite
	// KindLastWrite: the last path argument is a write, all prior paths are reads
	// (cp src... dst, mv src... dst, ln src dst).
	KindLastWrite
)

// PathArgSemantics maps a command name to how its path arguments read/write.
// Missing commands default to KindAllRead (backward-compatible, since the
// existing scope check treated everything as an access without direction).
var PathArgSemantics = map[string]ArgSemanticsKind{
	// Read-only paths
	"cat":       KindAllRead,
	"head":      KindAllRead,
	"tail":      KindAllRead,
	"less":      KindAllRead,
	"more":      KindAllRead,
	"grep":      KindAllRead,
	"egrep":     KindAllRead,
	"fgrep":     KindAllRead,
	"rg":        KindAllRead,
	"awk":       KindAllRead,
	"wc":        KindAllRead,
	"file":      KindAllRead,
	"stat":      KindAllRead,
	"diff":      KindAllRead,
	"cmp":       KindAllRead,
	"md5sum":    KindAllRead,
	"sha256sum": KindAllRead,

	// Write-only paths (destination arguments only)
	"rm":     KindAllWrite,
	"rmdir":  KindAllWrite,
	"shred":  KindAllWrite,
	"tee":    KindAllWrite,
	"touch":  KindAllWrite,
	"mkdir":  KindAllWrite,
	"unlink": KindAllWrite,

	// Last argument is write, others are read
	"cp": KindLastWrite,
	"mv": KindLastWrite,
	"ln": KindLastWrite,
}

// ClassifyPathArg returns whether the path at position i (in the extracted
// path list of length numPaths) is a read or write for the given command.
//
// This shim exists for backward compatibility; new callers should prefer
// ClassifyPathArgs which is flag-aware (Critical C2: `cp -t DIR src...`
// is not the same as `cp src... DIR`).
//
// Unknown commands return PathKindUnknown (Critical C3): the scope evaluator
// treats unknown paths as potentially-write for outside-write rule matching.
func ClassifyPathArg(cmdName string, i, numPaths int) PathKind {
	kind, ok := PathArgSemantics[cmdName]
	if !ok {
		return PathKindUnknown
	}
	switch kind {
	case KindAllRead:
		return PathKindRead
	case KindAllWrite:
		return PathKindWrite
	case KindLastWrite:
		if numPaths > 0 && i == numPaths-1 {
			return PathKindWrite
		}
		return PathKindRead
	}
	return PathKindRead
}

// ClassifyPathArgs returns the PathKind for each element of paths, given the
// command's full argv (rawArgs, without the command name). This lets us honor
// coreutils flags like `-t DIR` / `--target-directory=DIR` where the
// destination is not the last positional argument.
//
// Contract:
//   - len(result) == len(paths)
//   - When cp/mv/ln is invoked with -t DIR or --target-directory=DIR, the DIR
//     path is PathKindWrite and all other path arguments are PathKindRead.
//   - Unknown commands: every entry is PathKindUnknown.
//   - rawArgs is the argv AFTER the command name (i.e. what shell.Command.Args
//     already contains).
//
// This function assumes callers extracted `paths` from `rawArgs` in the same
// left-to-right order via a "looks like path" heuristic. The mapping between
// them is by identity: each path in `paths` must appear (by string equality)
// in `rawArgs`. The first matching argv position wins.
func ClassifyPathArgs(cmdName string, rawArgs []string, paths []string) []PathKind {
	out := make([]PathKind, len(paths))
	kind, ok := PathArgSemantics[cmdName]
	if !ok {
		for i := range out {
			out[i] = PathKindUnknown
		}
		return out
	}

	// Flag-aware overrides (Critical C2).
	// For cp/mv/ln, detect -t DIR / --target-directory=DIR / -T (no dir dest).
	if cmdName == "cp" || cmdName == "mv" || cmdName == "ln" {
		if targetDir, hasTargetDir := findTargetDirectory(rawArgs); hasTargetDir {
			// -t DIR: DIR is the sole write, all other paths are reads.
			for i, p := range paths {
				if p == targetDir {
					out[i] = PathKindWrite
				} else {
					out[i] = PathKindRead
				}
			}
			return out
		}
		// -T (no target directory): treat last path as write, as usual.
		// Fall through to KindLastWrite.
	}

	switch kind {
	case KindAllRead:
		for i := range out {
			out[i] = PathKindRead
		}
	case KindAllWrite:
		for i := range out {
			out[i] = PathKindWrite
		}
	case KindLastWrite:
		for i := range out {
			if i == len(out)-1 {
				out[i] = PathKindWrite
			} else {
				out[i] = PathKindRead
			}
		}
	}
	return out
}

// findTargetDirectory scans argv for GNU coreutils -t / --target-directory=
// flags and returns the DIR value. Handles the four forms:
//
//	-t DIR       (separate arg)
//	-tDIR        (glued)
//	--target-directory DIR
//	--target-directory=DIR
func findTargetDirectory(argv []string) (string, bool) {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-t" || a == "--target-directory":
			if i+1 < len(argv) {
				return argv[i+1], true
			}
		case strings.HasPrefix(a, "--target-directory="):
			return strings.TrimPrefix(a, "--target-directory="), true
		case strings.HasPrefix(a, "-t") && len(a) > 2:
			// -tDIR (glued short form). Guard against -t being part of a
			// larger cluster like `-tv`: we accept only if the char after -t
			// looks like a path (starts with / or . or contains /).
			rest := a[2:]
			if strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, "~/") ||
				strings.HasPrefix(rest, "./") || strings.HasPrefix(rest, "../") ||
				strings.Contains(rest, "/") {
				return rest, true
			}
		}
	}
	return "", false
}

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
