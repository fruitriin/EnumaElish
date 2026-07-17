package main

import (
	"fmt"
	"io"
	"os"

	"github.com/fruitriin/EnumaElish/internal/dsl"
)

var version = "dev"

// repoURL はバイナリ名（ccchain）からリポジトリ（EnumaElish）を辿れるようにする出自表記。
const repoURL = "https://github.com/fruitriin/EnumaElish"

// flagPassthroughCommands lists subcommands that run their own flag parser.
// Only these receive unknown flags via cmdArgs. For every other command an
// unknown flag is a hard error, so typos like `hook pre --defualt-action deny`
// fail loudly instead of being silently ignored (a mistyped security flag
// must never degrade into a false safety signal).
var flagPassthroughCommands = map[string]bool{
	"diff":    true,
	"approve": true,
	"init":    true,
}

// cliArgs holds the parsed global CLI state.
type cliArgs struct {
	verbose       bool
	quiet         bool
	configPath    string
	defaultAction string
	command       string
	cmdArgs       []string
	showVersion   bool
	showHelp      bool
}

// parseCLIArgs parses global flags and the subcommand from args.
// --help / --version short-circuit parsing immediately (preserving the
// historical "help wins" behavior even if later args are malformed).
func parseCLIArgs(args []string) (*cliArgs, error) {
	c := &cliArgs{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--verbose", "-v":
			c.verbose = true
		case "--quiet", "-q":
			c.quiet = true
		case "--config":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--config requires a path argument")
			}
			i++
			c.configPath = args[i]
		case "--default-action":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--default-action requires an action (allow, deny, ask)")
			}
			i++
			c.defaultAction = args[i]
			switch dsl.Action(c.defaultAction) {
			case dsl.ActionAllow, dsl.ActionDeny, dsl.ActionAsk:
				// valid
			default:
				return nil, fmt.Errorf("invalid default action: %q (must be allow, deny, or ask)", c.defaultAction)
			}
		case "--version":
			c.showVersion = true
			return c, nil
		case "--help", "-h":
			c.showHelp = true
			return c, nil
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				// Unknown flags pass through only to whitelisted subcommands
				// that parse their own flags; everything else errors.
				if c.command != "" && flagPassthroughCommands[c.command] {
					c.cmdArgs = append(c.cmdArgs, args[i])
					continue
				}
				return nil, fmt.Errorf("unknown flag: %s", args[i])
			}
			if c.command == "" {
				c.command = args[i]
			} else {
				c.cmdArgs = append(c.cmdArgs, args[i])
			}
		}
	}

	// diff takes its two configs as positional arguments; a global --config
	// would be silently swallowed and lead to confusing "missing positional"
	// behavior, so reject it explicitly.
	if c.command == "diff" && c.configPath != "" {
		return nil, fmt.Errorf("diff does not use --config; pass the two config files as positional arguments: ccchain diff <config-a> <config-b>")
	}

	return c, nil
}

func main() {
	c, err := parseCLIArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if c.showVersion {
		fmt.Printf("ccchain %s — Claude Code Chain (%s)\n", version, repoURL)
		os.Exit(0)
	}
	if c.showHelp {
		// After a subcommand, --help shows the subcommand-specific usage
		// (diff and approve have their own).
		switch c.command {
		case "diff":
			printDiffUsage()
		case "approve":
			printApproveUsage()
		default:
			printUsage()
		}
		os.Exit(0)
	}

	switch c.command {
	case "check":
		runCheck(c.configPath, c.verbose, c.quiet)
	case "hook":
		if len(c.cmdArgs) == 0 {
			fmt.Fprintln(os.Stderr, "error: ccchain hook requires 'pre' or 'post'")
			os.Exit(1)
		}
		switch c.cmdArgs[0] {
		case "pre":
			runHookPre(c.configPath, c.defaultAction)
		case "post":
			runHookPost(c.configPath)
		default:
			fmt.Fprintf(os.Stderr, "error: unknown hook type: %s\n", c.cmdArgs[0])
			os.Exit(1)
		}
	case "eval":
		runEval(c.configPath, c.defaultAction, c.cmdArgs)
	case "audit":
		runAudit(c.configPath)
	case "init":
		runInitDispatch(c.cmdArgs)
	case "suggest":
		runSuggest(c.configPath, c.cmdArgs)
	case "generate-rules":
		runGenerateRules()
	case "detect":
		runDetect()
	case "test":
		runTest(c.configPath, c.defaultAction, c.cmdArgs)
	case "diff":
		runDiff(c.defaultAction, c.cmdArgs)
	case "approve":
		runApprove(c.cmdArgs)
	case "version":
		fmt.Printf("ccchain %s — Claude Code Chain (%s)\n", version, repoURL)
	case "":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", c.command)
		fmt.Fprintln(os.Stderr, "run 'ccchain --help' for usage")
		os.Exit(1)
	}
}

func runCheck(configPath string, verbose, quiet bool) {
	if err := runCheckWithWriters(os.Stdout, os.Stderr, configPath, verbose, quiet); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// runCheckWithWriters is the testable core of runCheck: it accepts explicit
// writers for stdout/stderr so unit tests can capture output without touching
// process-global state, and returns errors instead of calling os.Exit.
func runCheckWithWriters(stdout, stderr io.Writer, configPath string, verbose, quiet bool) error {
	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		return err
	}

	if quiet {
		return nil
	}

	ruleCount := len(cfg.Rules) + len(cfg.PreRules) + len(cfg.PostRules)
	fmt.Fprintf(stdout, "config OK: %d templates, %d rules\n", len(cfg.Templates), ruleCount)

	// v0.2.1: warn about the fallback:ask + ask_strategy:degrade combo that
	// silently turns unlisted commands into deny in auto/dontAsk modes
	// (Issue #15). This is a legitimate behavior of the ask degrade
	// framework, but it needs to be surfaced at check time so users can
	// choose to opt out (ask_strategy: passthrough) or list more allow
	// rules instead of hitting it at runtime.
	if cfg.Settings != nil &&
		cfg.Settings.Fallback == dsl.ActionAsk &&
		(cfg.Settings.AskStrategy == "" || cfg.Settings.AskStrategy == dsl.AskStrategyDegrade) {
		fmt.Fprintln(stderr, "warning: settings.fallback: ask + ask_strategy: degrade (default)")
		fmt.Fprintln(stderr, "  In auto / dontAsk permission modes, every command not explicitly")
		fmt.Fprintln(stderr, "  covered by a rule will degrade to deny (Plan 0022 Phase 2).")
		fmt.Fprintln(stderr, "  To opt out: set settings.ask_strategy: passthrough (v0.1 behavior)")
		fmt.Fprintln(stderr, "  or add allow rules for the utilities you rely on (sed, awk, cut, ...).")
		fmt.Fprintln(stderr, "  Docs: https://github.com/fruitriin/EnumaElish/blob/main/docs/reference/dsl.md")
	}

	if verbose {
		// Plan 0028: show the currently-effective settings block so operators
		// can see at a glance which knobs are set (fallback / ask_strategy /
		// ask_degrade_default / unanalyzable_action / scope_violation / ...).
		// Written to stdout — verbose is an explicit opt-in, and stdout keeps
		// the digest greppable alongside the "config OK" line above.
		writeSettingsDigest(stdout, cfg)

		for _, t := range cfg.Templates {
			fmt.Fprintf(stdout, "  template: %s", t.Name)
			if t.Extends != "" {
				fmt.Fprintf(stdout, " (extends %s)", t.Extends)
			}
			if t.Next != "" {
				fmt.Fprintf(stdout, " (next: %s)", t.Next)
			}
			fmt.Fprintln(stdout)
		}
		for _, r := range cfg.PreRules {
			fmt.Fprintf(stdout, "  [pre]  %s %v\n", r.Action, r.Commands)
		}
		for _, r := range cfg.PostRules {
			fmt.Fprintf(stdout, "  [post] %s %v\n", r.Action, r.Commands)
		}
		for _, r := range cfg.Rules {
			fmt.Fprintf(stdout, "  [rule] %s %v\n", r.Action, r.Commands)
		}
	}
	return nil
}

// writeSettingsDigest renders the effective settings for `ccchain check -v`.
// The digest is intentionally exhaustive — every knob that changes hook
// behaviour (fallback / ask_strategy / ask_degrade_default /
// unanalyzable_action / scope_violation / strict_config_error /
// max_context_depth / max_rules_per_cmd / workspace) is listed so a operator
// diagnosing why an ask degraded (or why a config is stricter than they
// expected) can find the answer in one screen instead of hunting through
// multiple .conf files.
//
// The digest sources values from cfg.Settings (which the parser has already
// merged with DefaultSettings via mergeConfigs); we never fall back to
// DefaultSettings() here because the parser is authoritative and a nil
// Settings block on a loaded config would be a bug worth surfacing loudly.
func writeSettingsDigest(w io.Writer, cfg *dsl.Config) {
	fmt.Fprintln(w, "  settings:")
	if cfg == nil || cfg.Settings == nil {
		fmt.Fprintln(w, "    (no settings block; effective defaults not resolved)")
		return
	}
	s := cfg.Settings
	fmt.Fprintf(w, "    fallback:            %s\n", displayAction(s.Fallback))
	fmt.Fprintf(w, "    ask_strategy:        %s\n", displayString(s.AskStrategy, dsl.AskStrategyDegrade))
	fmt.Fprintf(w, "    ask_degrade_default: %s\n", displayAction(s.AskDegradeDefault))
	fmt.Fprintf(w, "    unanalyzable_action: %s\n", displayAction(s.UnanalyzableAction))
	fmt.Fprintf(w, "    scope_violation:     %s\n", displayAction(s.ScopeViolation))
	fmt.Fprintf(w, "    strict_config_error: %t\n", s.StrictConfigError)
	fmt.Fprintf(w, "    max_context_depth:   %d\n", s.MaxContextDepth)
	fmt.Fprintf(w, "    max_rules_per_cmd:   %d\n", s.MaxRulesPerCmd)
	if len(s.WorkspacePaths) == 0 {
		fmt.Fprintln(w, "    workspace:           (unset)")
	} else {
		fmt.Fprintf(w, "    workspace:           %v\n", s.WorkspacePaths)
	}
}

func displayAction(a dsl.Action) string {
	if a == "" {
		return "(unset)"
	}
	return string(a)
}

func displayString(v, fallback string) string {
	if v == "" {
		if fallback == "" {
			return "(unset)"
		}
		return fallback + " (default)"
	}
	return v
}

func printUsage() {
	fmt.Println(`ccchain - Claude Code Chain: structural permission control
https://github.com/fruitriin/EnumaElish

Usage:
  ccchain <command> [flags]

Commands:
  check            Validate configuration file syntax
  hook pre         PreToolUse hook (reads tool JSON from stdin)
  hook post        PostToolUse hook (reads tool JSON from stdin)
  eval "cmd"       Evaluate a command and output result as JSON
  test [file]      Evaluate a list of commands (file or stdin)
  diff a b [file]  Compare two configs on a list of commands (CHANGED/same)
  suggest          Suggest rules for unmatched commands
  detect           Auto-detect project type and suggest rules
  generate-rules   Generate rules from built-in semantics table
  audit            Display flat expansion of all rules
  approve          Approve a deny that degraded from an ask (human-side)
  init [--sentinel] Generate .ccchain.conf (default preset; --sentinel: deny-first)
  version          Print version

Flags:
  --config <path>            Configuration file path
  --default-action <action>  Fallback action for unmatched commands (allow, deny, ask)
  -v, --verbose              Verbose output
  -q, --quiet                Quiet output (errors only)
  --version                  Print version
  -h, --help                 Show help

init subcommand flags:
  --sentinel                 Emit the deny-first sentinel preset instead of
                             the default one (curated for auto/dontAsk/headless
                             modes where ask is not routed to a human).`)
}

// runInitDispatch parses ` + "`" + `ccchain init [--sentinel]` + "`" + ` arguments and delegates
// to the matching preset writer. Unknown flags/positional args are rejected
// so a typo in a security-sensitive flag never falls back to a lax default.
func runInitDispatch(cmdArgs []string) {
	sentinel := false
	for _, a := range cmdArgs {
		switch a {
		case "--sentinel":
			sentinel = true
		default:
			fmt.Fprintf(os.Stderr, "error: unknown argument for init: %q (only --sentinel is accepted)\n", a)
			os.Exit(1)
		}
	}
	if sentinel {
		runInitSentinel()
		return
	}
	runInit()
}
