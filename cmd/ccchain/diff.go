package main

import (
	"fmt"
	"os"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/eval"
)

// runDiff compares two config files by evaluating a list of commands against both
// and reporting which commands change their decision (CHANGED) or stay the same (same).
//
// Usage:
//   ccchain diff <config-a> <config-b> --commands commands.txt
//   ccchain diff <config-a> <config-b>            # commands from stdin
//   ccchain diff <config-a> <config-b> commands.txt  # positional file arg
//
// Output (aligned columns):
//   find . -delete   a=[allow]  b=[deny]   CHANGED
//   ls -la           a=[allow]  b=[allow]  same
//
// Flags:
//   --commands <path>    Read commands from file (one per line, # comments allowed)
//   --changed-only       Suppress "same" rows
//   --exit-on-change     Exit 2 if any command differs (useful for CI diffing rule PRs)
func runDiff(defaultAction string, cmdArgs []string) {
	var configA, configB, commandsFile string
	var changedOnly, exitOnChange bool

	// Parse args: two positional config paths, optional third positional file,
	// plus flags --commands / --changed-only / --exit-on-change.
	positional := make([]string, 0, 3)
	for i := 0; i < len(cmdArgs); i++ {
		switch cmdArgs[i] {
		case "--commands":
			if i+1 >= len(cmdArgs) {
				fmt.Fprintln(os.Stderr, "error: --commands requires a path argument")
				os.Exit(1)
			}
			i++
			commandsFile = cmdArgs[i]
		case "--changed-only":
			changedOnly = true
		case "--exit-on-change":
			exitOnChange = true
		default:
			if len(cmdArgs[i]) > 0 && cmdArgs[i][0] == '-' {
				fmt.Fprintf(os.Stderr, "error: unknown flag for diff: %s\n", cmdArgs[i])
				os.Exit(1)
			}
			positional = append(positional, cmdArgs[i])
		}
	}

	if len(positional) < 2 {
		fmt.Fprintln(os.Stderr, "error: ccchain diff requires two config file arguments")
		fmt.Fprintln(os.Stderr, "usage: ccchain diff <config-a> <config-b> [commands.txt] [--commands FILE] [--changed-only] [--exit-on-change]")
		os.Exit(1)
	}
	configA = positional[0]
	configB = positional[1]
	if len(positional) >= 3 && commandsFile == "" {
		commandsFile = positional[2]
	}

	cfgA, err := dsl.LoadConfig(configA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error (a=%s): %v\n", configA, err)
		os.Exit(1)
	}
	cfgB, err := dsl.LoadConfig(configB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error (b=%s): %v\n", configB, err)
		os.Exit(1)
	}

	if defaultAction != "" {
		if cfgA.Settings == nil {
			cfgA.Settings = dsl.DefaultSettings()
		}
		cfgA.Settings.Fallback = dsl.Action(defaultAction)
		if cfgB.Settings == nil {
			cfgB.Settings = dsl.DefaultSettings()
		}
		cfgB.Settings.Fallback = dsl.Action(defaultAction)
	}

	// Load commands: from --commands / positional file, or stdin.
	var commands []string
	if commandsFile != "" {
		commands, err = loadCommandsFromFile(commandsFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading command list: %v\n", err)
			os.Exit(1)
		}
	} else {
		commands, err = loadCommandsFromReader(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(1)
		}
	}

	if len(commands) == 0 {
		fmt.Fprintln(os.Stderr, "no commands to diff")
		fmt.Fprintln(os.Stderr, "usage: ccchain diff <config-a> <config-b> [commands.txt]")
		os.Exit(1)
	}

	// Compute max command width for alignment (bounded to keep pathological
	// long commands from blowing out the column).
	const maxCmdWidth = 60
	cmdWidth := 0
	for _, c := range commands {
		if l := len(c); l > cmdWidth {
			cmdWidth = l
		}
	}
	if cmdWidth > maxCmdWidth {
		cmdWidth = maxCmdWidth
	}

	var changedCount, sameCount, errCount int
	for _, cmd := range commands {
		resA, errA := eval.Evaluate(cmd, cfgA)
		resB, errB := eval.Evaluate(cmd, cfgB)
		if errA != nil || errB != nil {
			errCount++
			// Report both sides so the user can localize which config failed.
			aStr := "[error]"
			bStr := "[error]"
			if errA == nil {
				aStr = fmt.Sprintf("[%s]", resA.Action)
			}
			if errB == nil {
				bStr = fmt.Sprintf("[%s]", resB.Action)
			}
			fmt.Printf("%-*s  a=%-8s b=%-8s ERROR", cmdWidth, truncateStr(cmd, cmdWidth), aStr, bStr)
			if errA != nil {
				fmt.Printf("  (a: %v)", errA)
			}
			if errB != nil {
				fmt.Printf("  (b: %v)", errB)
			}
			fmt.Println()
			continue
		}

		aAction := string(resA.Action)
		bAction := string(resB.Action)
		if aAction == bAction {
			sameCount++
			if changedOnly {
				continue
			}
			fmt.Printf("%-*s  a=%-8s b=%-8s same\n",
				cmdWidth, truncateStr(cmd, cmdWidth),
				fmt.Sprintf("[%s]", aAction),
				fmt.Sprintf("[%s]", bAction))
		} else {
			changedCount++
			fmt.Printf("%-*s  a=%-8s b=%-8s CHANGED\n",
				cmdWidth, truncateStr(cmd, cmdWidth),
				fmt.Sprintf("[%s]", aAction),
				fmt.Sprintf("[%s]", bAction))
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d commands — changed=%d, same=%d, error=%d\n",
		len(commands), changedCount, sameCount, errCount)
	fmt.Printf("Config A: %s\n", configA)
	fmt.Printf("Config B: %s\n", configB)

	if exitOnChange && (changedCount > 0 || errCount > 0) {
		os.Exit(2)
	}
}
