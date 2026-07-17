package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fruitriin/EnumaElish/internal/approve"
)

// runApprove implements the `ccchain approve` subcommand — the human side of
// the Plan 0022 Phase 3 approval-token flow.
//
// Security note: this command must be denied to the agent context. See the
// Phase 4 sentinel preset (Bash(ccchain approve*)). ccchain enforces its own
// deny via the hook, but relying on that alone is a self-referential loop;
// the recommended defense is to also list `Bash(ccchain approve*)` in
// settings.json `permissions.deny`.
func runApprove(args []string) {
	opts, cmd, err := parseApproveArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr, "run 'ccchain approve --help' for usage")
		os.Exit(1)
	}
	if opts.showHelp {
		printApproveUsage()
		return
	}

	dir, err := approve.DefaultDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	store := approve.NewStore(dir)

	// Best-effort purge before every command so operators see a fresh view.
	_, _, _ = store.Purge(24 * time.Hour)

	switch cmd {
	case approveCmdList:
		runApproveList(store)
	case approveCmdRevokeAll:
		runApproveRevokeAll(store)
	case approveCmdLast:
		runApproveLast(store, opts)
	case approveCmdByPrefix:
		runApproveByPrefix(store, opts)
	default:
		fmt.Fprintln(os.Stderr, "error: ccchain approve requires --last, --list, --revoke-all, or a hash prefix")
		fmt.Fprintln(os.Stderr, "run 'ccchain approve --help' for usage")
		os.Exit(1)
	}
}

type approveCmd int

const (
	approveCmdNone approveCmd = iota
	approveCmdLast
	approveCmdList
	approveCmdRevokeAll
	approveCmdByPrefix
)

type approveOpts struct {
	ttl        time.Duration
	scopeFlag  bool // --global
	prefix     string
	showHelp   bool
	explicitTTL bool
}

// parseApproveArgs is a small hand-rolled parser: the surface is intentionally
// tiny (a couple of flags + one positional) and reusing the global CLI parser
// would drag in every unrelated flag as a false positive.
func parseApproveArgs(args []string) (approveOpts, approveCmd, error) {
	var opts approveOpts
	cmd := approveCmdNone
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--help", "-h":
			opts.showHelp = true
			return opts, cmd, nil
		case "--last":
			if cmd != approveCmdNone && cmd != approveCmdLast {
				return opts, cmd, errors.New("--last conflicts with other action flags")
			}
			cmd = approveCmdLast
		case "--list":
			if cmd != approveCmdNone && cmd != approveCmdList {
				return opts, cmd, errors.New("--list conflicts with other action flags")
			}
			cmd = approveCmdList
		case "--revoke-all":
			if cmd != approveCmdNone && cmd != approveCmdRevokeAll {
				return opts, cmd, errors.New("--revoke-all conflicts with other action flags")
			}
			cmd = approveCmdRevokeAll
		case "--global":
			opts.scopeFlag = true
		case "--ttl":
			if i+1 >= len(args) {
				return opts, cmd, errors.New("--ttl requires a duration (e.g. 15m, 1h)")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return opts, cmd, fmt.Errorf("--ttl: invalid duration %q: %v", args[i], err)
			}
			if d <= 0 {
				return opts, cmd, fmt.Errorf("--ttl: must be positive, got %s", d)
			}
			opts.ttl = d
			opts.explicitTTL = true
		default:
			if len(a) > 0 && a[0] == '-' {
				return opts, cmd, fmt.Errorf("unknown flag: %s", a)
			}
			// Positional: hash prefix.
			if cmd != approveCmdNone && cmd != approveCmdByPrefix {
				return opts, cmd, errors.New("hash prefix conflicts with other action flags")
			}
			if opts.prefix != "" {
				return opts, cmd, errors.New("only one hash prefix is allowed")
			}
			opts.prefix = a
			cmd = approveCmdByPrefix
		}
	}
	return opts, cmd, nil
}

func printApproveUsage() {
	fmt.Println(`ccchain approve — human-side approval of a deferred command

Usage:
  ccchain approve --last              Approve the most recent pending entry
  ccchain approve --list              List pending entries (hash prefix + command)
  ccchain approve <hash-prefix>       Approve a specific pending entry (min 4 chars)
  ccchain approve --revoke-all        Mark every un-consumed approval as spent

Flags:
  --ttl <duration>   Approval lifetime (default 15m; e.g. 30s, 1h)
  --global           Match any session/cwd (default: session+cwd only)
  -h, --help         Show this help

Security: 'ccchain approve' must not run from the agent's Bash tool. Add
Bash(ccchain approve*) to settings.json permissions.deny (and use the
--sentinel preset for defense in depth).`)
}

func runApproveList(store *approve.Store) {
	list, err := store.ListPending()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(list) == 0 {
		fmt.Println("no pending approvals")
		return
	}
	fmt.Printf("%-4s  %-16s  %-8s  %s\n", "#", "HASH", "AGE", "COMMAND")
	now := time.Now()
	for i, e := range list {
		age := now.Sub(time.Unix(e.Timestamp, 0)).Round(time.Second)
		fmt.Printf("%-4d  %-16s  %-8s  %s\n", i+1, shortHash(e.Hash), age, previewCommand(e.Command))
	}
}

func runApproveRevokeAll(store *approve.Store) {
	n, err := store.RevokeAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("revoked %d approval(s)\n", n)
}

func runApproveLast(store *approve.Store, opts approveOpts) {
	entry, seed, err := store.ApproveLast(buildApproveOptions(opts))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printApprovalGranted(entry, seed)
}

func runApproveByPrefix(store *approve.Store, opts approveOpts) {
	entry, seed, err := store.ApproveByHashPrefix(opts.prefix, buildApproveOptions(opts))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	printApprovalGranted(entry, seed)
}

func buildApproveOptions(opts approveOpts) approve.ApproveOptions {
	scope := approve.ScopeSession
	if opts.scopeFlag {
		scope = approve.ScopeGlobal
	}
	out := approve.ApproveOptions{Scope: scope}
	if opts.explicitTTL {
		out.TTL = opts.ttl
	}
	return out
}

func printApprovalGranted(entry *approve.ApprovedEntry, seed *approve.PendingEntry) {
	ttl := time.Duration(entry.TTLSeconds) * time.Second
	// Security H1: cwd / session_id also flow from untrusted input (hook
	// tool_input) and can carry control sequences; sanitize before print.
	fmt.Printf("approved: %s\n", shortHash(entry.Hash))
	fmt.Printf("  command: %s\n", previewCommand(entry.Command))
	fmt.Printf("  scope:   %s\n", sanitizeApprovalDisplay(string(entry.Scope)))
	if entry.Scope == approve.ScopeSession {
		fmt.Printf("  cwd:     %s\n", sanitizeApprovalDisplay(entry.CWD))
		if entry.SessionID != "" {
			fmt.Printf("  session: %s\n", sanitizeApprovalDisplay(entry.SessionID))
		}
	}
	fmt.Printf("  ttl:     %s\n", ttl)
	_ = seed
}

func shortHash(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

// previewCommand renders a command for `approve --list` / audit output.
// Control characters (including ANSI escapes) are replaced with spaces before
// the length cap is applied — an approval command that embeds `\x1b[…m` could
// otherwise re-color the terminal, hide characters via `\r`, or fake the
// prompt with `\x1b[2J`, all of which mislead the human's approval judgment
// (Security H1). Sanitization runs before length capping so the visible
// output length is bounded on displayed characters, not source bytes.
func previewCommand(cmd string) string {
	const max = 80
	sanitized := sanitizeApprovalDisplay(cmd)
	if len(sanitized) <= max {
		return sanitized
	}
	cut := max - 1
	// Cut on a rune boundary so multi-byte characters do not produce broken UTF-8.
	for cut > 0 && !utf8.RuneStart(sanitized[cut]) {
		cut--
	}
	return sanitized[:cut] + "…"
}

// sanitizeApprovalDisplay replaces control characters with a space and drops
// bytes that would break UTF-8. Mirrors sanitizeForMessage in internal/eval
// but is kept local to the approve command to avoid a package dependency.
func sanitizeApprovalDisplay(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == utf8.RuneError:
			b.WriteByte(' ')
		case r < 0x20:
			// Tab, newline, CR, ANSI ESC, all other C0 controls become space
			// so any ANSI escape sequence loses its introducer.
			b.WriteByte(' ')
		case r == 0x7f:
			// DEL — treat as control.
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
