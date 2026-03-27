package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/eval"
)

// safeCommands are commands that are generally safe to allow at top level.
var safeCommands = map[string]string{
	// File reading
	"cat":    "file reading",
	"head":   "file reading",
	"tail":   "file reading",
	"less":   "file reading",
	"more":   "file reading",
	"wc":     "file counting",
	"diff":   "file comparison",
	"file":   "file type detection",
	"stat":   "file metadata",
	"md5sum": "checksum",
	"shasum": "checksum",

	// Directory/path
	"pwd":     "current directory",
	"dirname": "path manipulation",
	"basename": "path manipulation",
	"realpath": "path resolution",

	// Text processing
	"echo":   "text output",
	"printf": "text output",
	"sort":   "text sorting",
	"uniq":   "text deduplication",
	"tr":     "text translation",
	"cut":    "text cutting",
	"tee":    "output splitting",
	"rev":    "text reversal",
	"column": "text formatting",

	// Search
	"which":   "command lookup",
	"whereis": "command lookup",
	"type":    "command type",

	// System info
	"date":     "date/time",
	"uname":    "system info",
	"hostname": "hostname",
	"whoami":   "current user",
	"id":       "user info",
	"uptime":   "system uptime",
	"env":      "environment variables",
	"printenv": "environment variables",

	// Safe file operations
	"cp":    "file copy",
	"mkdir": "directory creation",
	"touch": "file creation",
	"ln":    "link creation",

	// Version/help
	"true":  "no-op",
	"false": "no-op",
	"test":  "condition test",
}

func runSuggest(configPath string, cmdArgs []string) {
	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// Collect commands from stdin or args
	var commands []string
	if len(cmdArgs) > 0 {
		commands = cmdArgs
	} else {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				commands = append(commands, line)
			}
		}
	}

	if len(commands) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ccchain suggest <cmd1> <cmd2> ...")
		fmt.Fprintln(os.Stderr, "   or: cat commands.txt | ccchain suggest")
		os.Exit(1)
	}

	// Evaluate each command and collect ask results
	type suggestion struct {
		cmd    string
		reason string
	}
	var suggestions []suggestion
	askCmds := make(map[string]int) // command name → count

	for _, cmd := range commands {
		result, err := eval.Evaluate(cmd, cfg)
		if err != nil {
			continue
		}
		if result.Action == dsl.ActionAsk {
			// Extract the base command name
			baseName := extractBaseName(cmd)
			askCmds[baseName]++
		}
	}

	// Suggest rules for frequently-asked commands
	for name, count := range askCmds {
		if reason, ok := safeCommands[name]; ok {
			suggestions = append(suggestions, suggestion{
				cmd:    name,
				reason: fmt.Sprintf("%s (used %d times, generally safe: %s)", name, count, reason),
			})
		} else if count >= 2 {
			suggestions = append(suggestions, suggestion{
				cmd:    name,
				reason: fmt.Sprintf("%s (used %d times, consider adding a rule)", name, count),
			})
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return askCmds[suggestions[i].cmd] > askCmds[suggestions[j].cmd]
	})

	if len(suggestions) == 0 {
		fmt.Println("No suggestions — all commands are covered by existing rules.")
		return
	}

	fmt.Println("# Suggested rules for .ccchain.conf")
	fmt.Println("# Commands that currently fall through to 'ask' but appear safe:")
	fmt.Println()

	for _, s := range suggestions {
		if _, ok := safeCommands[s.cmd]; ok {
			fmt.Printf("allow %s\n", s.cmd)
		} else {
			fmt.Printf("# ask %s  # review before allowing\n", s.cmd)
		}
	}

	fmt.Println()
	fmt.Println("# ---")
	fmt.Printf("# %d commands would benefit from explicit rules\n", len(suggestions))
	fmt.Println("# Copy the 'allow' lines above into your .ccchain.conf")
}

func extractBaseName(cmd string) string {
	// Strip leading path
	cmd = strings.TrimSpace(cmd)
	// Handle chains: take the first command
	for _, sep := range []string{"&&", "||", ";"} {
		if idx := strings.Index(cmd, sep); idx > 0 {
			cmd = cmd[:idx]
		}
	}
	// Handle pipes: take the first command
	if idx := strings.Index(cmd, "|"); idx > 0 {
		cmd = cmd[:idx]
	}
	cmd = strings.TrimSpace(cmd)

	// Split into words and take first
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}

	// Strip path
	name := fields[0]
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}
