package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/eval"
)

// runDiff compares two config files by evaluating a list of commands against both
// and reporting which commands change their decision (CHANGED) or stay the same (same).
//
// Usage:
//
//	ccchain diff <config-a> <config-b> --commands commands.txt
//	ccchain diff <config-a> <config-b>            # commands from stdin
//	ccchain diff <config-a> <config-b> commands.txt  # positional file arg
//
// Output (aligned columns):
//
//	Config A: old.conf
//	Config B: new.conf
//
//	find . -delete   a=[allow]  b=[deny]   CHANGED
//	ls -la           a=[allow]  b=[allow]  same
//
// Only Bash command evaluation is compared, and only the resulting action
// (allow/ask/deny/warn); message differences and tool rules (Read/Edit etc.)
// are not compared.
//
// Exit codes:
//
//	0  completed (no changes, or changes without --exit-on-change)
//	1  usage or config error
//	2  at least one CHANGED (only with --exit-on-change)
//	3  one or more commands failed to evaluate (takes precedence over 2)
func runDiff(defaultAction string, cmdArgs []string) {
	os.Exit(diffMain(defaultAction, cmdArgs, os.Stdin, os.Stdout, os.Stderr))
}

// diffMain is the testable core of runDiff. It returns the process exit code.
func diffMain(defaultAction string, cmdArgs []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var commandsFile string
	var changedOnly, exitOnChange bool

	// Parse args: two positional config paths, optional third positional
	// commands file, plus flags --commands / --changed-only / --exit-on-change.
	positional := make([]string, 0, 4)
	for i := 0; i < len(cmdArgs); i++ {
		switch cmdArgs[i] {
		case "--commands":
			if i+1 >= len(cmdArgs) {
				fmt.Fprintln(stderr, "error: --commands requires a path argument")
				return 1
			}
			i++
			commandsFile = cmdArgs[i]
		case "--changed-only":
			changedOnly = true
		case "--exit-on-change":
			exitOnChange = true
		case "--help", "-h":
			// Normally intercepted by the global parser; kept for robustness.
			fprintDiffUsage(stdout)
			return 0
		default:
			if len(cmdArgs[i]) > 0 && cmdArgs[i][0] == '-' {
				fmt.Fprintf(stderr, "error: unknown flag for diff: %s\n", cmdArgs[i])
				return 1
			}
			positional = append(positional, cmdArgs[i])
		}
	}

	if len(positional) < 2 {
		fmt.Fprintln(stderr, "error: ccchain diff requires two config file arguments")
		fprintDiffUsage(stderr)
		return 1
	}
	if len(positional) > 3 {
		fmt.Fprintf(stderr, "error: too many positional arguments for diff: %q\n", positional[3:])
		fprintDiffUsage(stderr)
		return 1
	}
	configA := positional[0]
	configB := positional[1]
	// Empty or whitespace-only config paths must be rejected: dsl.LoadConfig("")
	// falls back to the default search paths, which would silently compare a
	// config against itself (false "no changes" in CI when a variable is empty).
	if strings.TrimSpace(configA) == "" || strings.TrimSpace(configB) == "" {
		fmt.Fprintln(stderr, "error: config file paths must be non-empty (empty path would fall back to the default config search)")
		return 1
	}
	if len(positional) >= 3 {
		if commandsFile != "" {
			fmt.Fprintln(stderr, "error: both --commands and a positional commands file were given; specify only one")
			return 1
		}
		if strings.TrimSpace(positional[2]) == "" {
			fmt.Fprintln(stderr, "error: commands file path must be non-empty")
			return 1
		}
		commandsFile = positional[2]
	}

	cfgA, err := dsl.LoadConfig(configA)
	if err != nil {
		fmt.Fprintf(stderr, "config error (a=%s): %v\n", configA, err)
		return 1
	}
	cfgB, err := dsl.LoadConfig(configB)
	if err != nil {
		fmt.Fprintf(stderr, "config error (b=%s): %v\n", configB, err)
		return 1
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
			fmt.Fprintf(stderr, "error reading command list: %v\n", err)
			return 1
		}
	} else {
		commands, err = loadCommandsFromReader(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "error reading stdin: %v\n", err)
			return 1
		}
	}

	if len(commands) == 0 {
		fmt.Fprintln(stderr, "no commands to diff")
		fmt.Fprintln(stderr, "usage: ccchain diff <config-a> <config-b> [commands.txt]")
		return 1
	}

	// Escape control characters up front: commands.txt content is untrusted
	// and raw \r or ANSI CSI sequences could hide CHANGED rows from review.
	displays := make([]string, len(commands))
	for i, c := range commands {
		displays[i] = sanitizeControlChars(c)
	}

	// Compute max command width for alignment (bounded to keep pathological
	// long commands from blowing out the column).
	const maxCmdWidth = 60
	cmdWidth := 0
	for _, d := range displays {
		if l := len(d); l > cmdWidth {
			cmdWidth = l
		}
	}
	if cmdWidth > maxCmdWidth {
		cmdWidth = maxCmdWidth
	}

	// Header: say which file is which before any rows, so long outputs are
	// readable without scrolling to the bottom.
	fmt.Fprintf(stdout, "Config A: %s\n", sanitizeControlChars(configA))
	fmt.Fprintf(stdout, "Config B: %s\n\n", sanitizeControlChars(configB))

	var changedCount, sameCount, errCount int
	for i, cmd := range commands {
		display := truncateStr(displays[i], cmdWidth)
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
			fmt.Fprintf(stdout, "%-*s  a=%-8s b=%-8s ERROR", cmdWidth, display, aStr, bStr)
			if errA != nil {
				fmt.Fprintf(stdout, "  (a: %s)", sanitizeControlChars(errA.Error()))
			}
			if errB != nil {
				fmt.Fprintf(stdout, "  (b: %s)", sanitizeControlChars(errB.Error()))
			}
			fmt.Fprintln(stdout)
			continue
		}

		aAction := string(resA.Action)
		bAction := string(resB.Action)
		if aAction == bAction {
			sameCount++
			if changedOnly {
				continue
			}
			fmt.Fprintf(stdout, "%-*s  a=%-8s b=%-8s same\n",
				cmdWidth, display,
				fmt.Sprintf("[%s]", aAction),
				fmt.Sprintf("[%s]", bAction))
		} else {
			changedCount++
			fmt.Fprintf(stdout, "%-*s  a=%-8s b=%-8s CHANGED\n",
				cmdWidth, display,
				fmt.Sprintf("[%s]", aAction),
				fmt.Sprintf("[%s]", bAction))
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "Summary: %d commands — changed=%d, same=%d, error=%d\n",
		len(commands), changedCount, sameCount, errCount)
	fmt.Fprintf(stdout, "Config A: %s\n", sanitizeControlChars(configA))
	fmt.Fprintf(stdout, "Config B: %s\n", sanitizeControlChars(configB))

	// Errors take precedence over "changed" so CI can distinguish a genuine
	// rule change (2) from a broken command list / config (3).
	if errCount > 0 {
		return 3
	}
	if exitOnChange && changedCount > 0 {
		return 2
	}
	return 0
}

// sanitizeControlChars makes control characters visible as \xNN escapes so
// untrusted command strings cannot inject terminal escape sequences
// (ANSI CSI, carriage-return overwrites) into diff output.
func sanitizeControlChars(s string) string {
	if strings.IndexFunc(s, isControlChar) < 0 {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if isControlChar(r) {
			fmt.Fprintf(&b, "\\x%02x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isControlChar(r rune) bool {
	return r < 0x20 || r == 0x7f
}

func printDiffUsage() {
	fprintDiffUsage(os.Stdout)
}

func fprintDiffUsage(w io.Writer) {
	fmt.Fprintln(w, `usage: ccchain diff <config-a> <config-b> [commands.txt] [flags]

Evaluate the same command list against two config files and report, per
command, whether the resulting action differs (CHANGED) or not (same).

Scope: only Bash command evaluation is compared, and only the resulting
action (allow/ask/deny/warn). Message differences and tool rules
(Read/Edit etc.) are not compared.

Command sources (mutually exclusive; stdin is the fallback):
  --commands FILE   read commands from FILE (one per line, # comments allowed)
  commands.txt      third positional argument
  stdin             used when neither of the above is given

Flags:
  --commands FILE   Read commands from FILE
  --changed-only    Suppress "same" rows
  --exit-on-change  Exit 2 when at least one command changed (for CI)
  -h, --help        Show this help

Exit codes:
  0  completed (no changes, or changes without --exit-on-change)
  1  usage or config error
  2  at least one CHANGED (only with --exit-on-change)
  3  one or more commands failed to evaluate (takes precedence over 2)`)
}
