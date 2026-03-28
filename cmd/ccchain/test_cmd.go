package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/eval"
)

// runTest evaluates a list of commands against a config and reports results.
//
// Usage:
//   ccchain test                                  # stdin commands, default config search
//   ccchain test commands.txt                     # file commands, default config search
//   ccchain test --config rules.conf commands.txt # file commands, explicit config
//   cat commands.txt | ccchain test --config rules.conf  # stdin + explicit config
func runTest(configPath string, defaultAction string, cmdArgs []string) {
	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	if defaultAction != "" {
		if cfg.Settings == nil {
			cfg.Settings = dsl.DefaultSettings()
		}
		cfg.Settings.Fallback = dsl.Action(defaultAction)
	}

	// Determine command source: file arg or stdin
	var commands []string
	if len(cmdArgs) > 0 {
		commands, err = loadCommandsFromFile(cmdArgs[0])
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
		fmt.Fprintln(os.Stderr, "no commands to test")
		fmt.Fprintln(os.Stderr, "usage: ccchain test [commands.txt]")
		fmt.Fprintln(os.Stderr, "   or: cat commands.txt | ccchain test")
		os.Exit(1)
	}

	// Evaluate and report
	var allowCount, askCount, denyCount, warnCount, errorCount int

	for _, cmd := range commands {
		result, err := eval.Evaluate(cmd, cfg)
		if err != nil {
			errorCount++
			fmt.Printf("[error]  %s  (%v)\n", cmd, err)
			continue
		}

		actionStr := fmt.Sprintf("[%s]", result.Action)
		msgStr := ""
		if result.Message != "" && result.Action != dsl.ActionAllow {
			msgStr = fmt.Sprintf("  %q", truncateStr(result.Message, 60))
		}

		fmt.Printf("%-8s %s%s\n", actionStr, cmd, msgStr)

		switch result.Action {
		case dsl.ActionAllow:
			allowCount++
		case dsl.ActionAsk:
			askCount++
		case dsl.ActionDeny:
			denyCount++
		case dsl.ActionWarn:
			warnCount++
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d commands — allow=%d, ask=%d, deny=%d, warn=%d, error=%d\n",
		len(commands), allowCount, askCount, denyCount, warnCount, errorCount)

	if configPath != "" {
		fmt.Printf("Config: %s\n", configPath)
	} else {
		fmt.Println("Config: (default search path)")
	}
}

func loadCommandsFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return loadCommandsFromReader(f)
}

func loadCommandsFromReader(r *os.File) ([]string, error) {
	var cmds []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmds = append(cmds, line)
	}
	return cmds, scanner.Err()
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
