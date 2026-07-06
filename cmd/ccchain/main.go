package main

import (
	"fmt"
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
	"diff": true,
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
		// (currently only diff has one).
		if c.command == "diff" {
			printDiffUsage()
		} else {
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
		runInit()
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
	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !quiet {
		ruleCount := len(cfg.Rules) + len(cfg.PreRules) + len(cfg.PostRules)
		fmt.Printf("config OK: %d templates, %d rules\n", len(cfg.Templates), ruleCount)

		if verbose {
			for _, t := range cfg.Templates {
				fmt.Printf("  template: %s", t.Name)
				if t.Extends != "" {
					fmt.Printf(" (extends %s)", t.Extends)
				}
				if t.Next != "" {
					fmt.Printf(" (next: %s)", t.Next)
				}
				fmt.Println()
			}
			for _, r := range cfg.PreRules {
				fmt.Printf("  [pre]  %s %v\n", r.Action, r.Commands)
			}
			for _, r := range cfg.PostRules {
				fmt.Printf("  [post] %s %v\n", r.Action, r.Commands)
			}
			for _, r := range cfg.Rules {
				fmt.Printf("  [rule] %s %v\n", r.Action, r.Commands)
			}
		}
	}
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
  init             Generate default .ccchain.conf
  version          Print version

Flags:
  --config <path>            Configuration file path
  --default-action <action>  Fallback action for unmatched commands (allow, deny, ask)
  -v, --verbose              Verbose output
  -q, --quiet                Quiet output (errors only)
  --version                  Print version
  -h, --help                 Show help`)
}
