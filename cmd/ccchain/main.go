package main

import (
	"fmt"
	"os"

	"github.com/fruitriin/ccchain/internal/dsl"
)

var version = "dev"

func main() {
	args := os.Args[1:]

	var verbose, quiet bool
	var configPath string
	var defaultAction string
	var command string
	var cmdArgs []string

	// Parse all flags (before and after command)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--verbose", "-v":
			verbose = true
		case "--quiet", "-q":
			quiet = true
		case "--config":
			if i+1 < len(args) {
				i++
				configPath = args[i]
			} else {
				fmt.Fprintln(os.Stderr, "error: --config requires a path argument")
				os.Exit(1)
			}
		case "--default-action":
			if i+1 < len(args) {
				i++
				defaultAction = args[i]
				switch dsl.Action(defaultAction) {
				case dsl.ActionAllow, dsl.ActionDeny, dsl.ActionAsk:
					// valid
				default:
					fmt.Fprintf(os.Stderr, "error: invalid default action: %q (must be allow, deny, or ask)\n", defaultAction)
					os.Exit(1)
				}
			} else {
				fmt.Fprintln(os.Stderr, "error: --default-action requires an action (allow, deny, ask)")
				os.Exit(1)
			}
		case "--version":
			fmt.Printf("ccchain %s\n", version)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		default:
			if len(args[i]) > 0 && args[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", args[i])
				os.Exit(1)
			}
			if command == "" {
				command = args[i]
			} else {
				cmdArgs = append(cmdArgs, args[i])
			}
		}
	}

	switch command {
	case "check":
		runCheck(configPath, verbose, quiet)
	case "hook":
		if len(cmdArgs) == 0 {
			fmt.Fprintln(os.Stderr, "error: ccchain hook requires 'pre' or 'post'")
			os.Exit(1)
		}
		switch cmdArgs[0] {
		case "pre":
			runHookPre(configPath, defaultAction)
		case "post":
			runHookPost(configPath)
		default:
			fmt.Fprintf(os.Stderr, "error: unknown hook type: %s\n", cmdArgs[0])
			os.Exit(1)
		}
	case "eval":
		runEval(configPath, defaultAction, cmdArgs)
	case "audit":
		runAudit(configPath)
	case "init":
		runInit()
	case "suggest":
		runSuggest(configPath, cmdArgs)
	case "generate-rules":
		runGenerateRules()
	case "detect":
		runDetect()
	case "version":
		fmt.Printf("ccchain %s\n", version)
	case "":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command: %s\n", command)
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

Usage:
  ccchain <command> [flags]

Commands:
  check       Validate configuration file syntax
  hook pre    PreToolUse hook (reads tool JSON from stdin)
  hook post   PostToolUse hook (reads tool JSON from stdin)
  eval "cmd"  Evaluate a command and output result as JSON
  suggest     Suggest rules for unmatched commands
  audit       Display flat expansion of all rules
  init        Generate default .ccchain.conf
  version     Print version

Flags:
  --config <path>            Configuration file path
  --default-action <action>  Fallback action for unmatched commands (allow, deny, ask)
  -v, --verbose              Verbose output
  -q, --quiet                Quiet output (errors only)
  --version                  Print version
  -h, --help                 Show help`)
}
