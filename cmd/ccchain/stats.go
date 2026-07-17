package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/evallog"
)

// runStats implements the `ccchain stats` subcommand — aggregation over the
// JSONL log emitted by the hook when `settings.log:` is set (Plan 0029).
//
// Usage:
//
//	ccchain stats [--since <duration>] [--group-by action|rule|command] [--json] [--top N]
//
// Defaults: --since 24h, --group-by action, --top 20.
//
// Exit codes:
//
//	0  aggregation printed
//	1  usage error, config error, or "log is not enabled"
func runStats(configPath string, cmdArgs []string) {
	os.Exit(statsMain(configPath, cmdArgs, os.Stdout, os.Stderr))
}

type statsOpts struct {
	since   time.Duration
	groupBy evallog.GroupBy
	asJSON  bool
	top     int
	logPath string // resolved log path (overrides settings.log if set)
	help    bool
}

func statsMain(configPath string, cmdArgs []string, stdout, stderr io.Writer) int {
	opts, err := parseStatsArgs(cmdArgs)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		fmt.Fprintln(stderr, "run 'ccchain stats --help' for usage")
		return 1
	}
	if opts.help {
		fprintStatsUsage(stdout)
		return 0
	}

	logPath := opts.logPath
	if logPath == "" {
		cfg, err := dsl.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintf(stderr, "config error: %v\n", err)
			return 1
		}
		if cfg.Settings == nil || cfg.Settings.LogPath == "" {
			fmt.Fprintln(stderr, "log is not enabled; add `settings: log: <path>` to your .ccchain.conf")
			fmt.Fprintln(stderr, "or pass --log <path> to override.")
			return 1
		}
		logPath = cfg.Settings.LogPath
	}
	if !filepath.IsAbs(logPath) {
		wd, err := os.Getwd()
		if err == nil {
			logPath = filepath.Join(wd, logPath)
		}
	}

	agg, err := evallog.Stats(logPath, opts.since, opts.groupBy)
	if err != nil {
		fmt.Fprintf(stderr, "error reading log: %v\n", err)
		return 1
	}

	if opts.top > 0 && len(agg) > opts.top {
		agg = agg[:opts.top]
	}

	if opts.asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(agg); err != nil {
			fmt.Fprintf(stderr, "error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	printStatsTable(stdout, agg, opts.groupBy, opts.since, logPath)
	return 0
}

func parseStatsArgs(args []string) (statsOpts, error) {
	opts := statsOpts{
		since:   24 * time.Hour,
		groupBy: evallog.GroupByAction,
		top:     20,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--help", "-h":
			opts.help = true
			return opts, nil
		case "--since":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--since requires a duration (e.g. 24h, 7d)")
			}
			i++
			d, err := parseSinceDuration(args[i])
			if err != nil {
				return opts, fmt.Errorf("--since: %v", err)
			}
			opts.since = d
		case "--group-by":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--group-by requires action|rule|command")
			}
			i++
			switch args[i] {
			case "action":
				opts.groupBy = evallog.GroupByAction
			case "rule":
				opts.groupBy = evallog.GroupByRule
			case "command":
				opts.groupBy = evallog.GroupByCommand
			default:
				return opts, fmt.Errorf("--group-by: unknown value %q (must be action|rule|command)", args[i])
			}
		case "--json":
			opts.asJSON = true
		case "--top":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--top requires an integer")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return opts, fmt.Errorf("--top: invalid count %q", args[i])
			}
			opts.top = n
		case "--log":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--log requires a path")
			}
			i++
			opts.logPath = args[i]
		default:
			if len(a) > 0 && a[0] == '-' {
				return opts, fmt.Errorf("unknown flag: %s", a)
			}
			return opts, fmt.Errorf("unexpected positional argument: %s", a)
		}
	}
	return opts, nil
}

// parseSinceDuration accepts Go's time.ParseDuration syntax plus the `Nd`
// convenience form. Go's parser only reaches `h`; larger scales are
// operator-friendly (Plan 0029 explicitly cites `7d`).
func parseSinceDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		trimmed := strings.TrimSuffix(s, "d")
		n, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid days: %q", s)
		}
		if n < 0 {
			return 0, fmt.Errorf("negative duration: %q", s)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("negative duration: %q", s)
	}
	return d, nil
}

func printStatsTable(w io.Writer, agg []evallog.Aggregate, by evallog.GroupBy, since time.Duration, logPath string) {
	fmt.Fprintf(w, "log:      %s\n", logPath)
	if since > 0 {
		fmt.Fprintf(w, "since:    %s\n", since)
	} else {
		fmt.Fprintf(w, "since:    (all)\n")
	}
	fmt.Fprintf(w, "group-by: %s\n\n", by)

	if len(agg) == 0 {
		fmt.Fprintln(w, "no events in window")
		return
	}

	// Column widths.
	keyWidth := len(string(by))
	for _, a := range agg {
		if l := len(a.Key); l > keyWidth {
			keyWidth = l
		}
	}
	// Cap key column so a rogue long command does not blow out the layout.
	const maxKeyWidth = 80
	if keyWidth > maxKeyWidth {
		keyWidth = maxKeyWidth
	}

	fmt.Fprintf(w, "%-6s  %-19s  %-*s\n", "COUNT", "LAST_SEEN", keyWidth, strings.ToUpper(string(by)))
	for _, a := range agg {
		key := a.Key
		if len(key) > keyWidth {
			key = key[:keyWidth-1] + "…"
		}
		fmt.Fprintf(w, "%-6d  %-19s  %-*s\n",
			a.Count,
			time.Unix(a.LastSeen, 0).Format("2006-01-02 15:04:05"),
			keyWidth, key)
	}
}

func printStatsUsage() {
	fprintStatsUsage(os.Stdout)
}

func fprintStatsUsage(w io.Writer) {
	fmt.Fprintln(w, `usage: ccchain stats [flags]

Aggregate the hook evaluation log (settings.log:) by action, rule, or
command. The log is populated only when settings.log: is set in the
active config; otherwise this command exits 1 with a hint.

Flags:
  --since <duration>       Time window (default 24h; e.g. 30m, 24h, 7d)
  --group-by <dim>         action | rule | command (default action)
  --top N                  Show top N rows (default 20, 0 = all)
  --json                   Emit JSON array instead of a table
  --log <path>             Override settings.log: (absolute or relative)
  --config <path>          Config file used to resolve settings.log:
  -h, --help               Show this help

Exit codes:
  0  aggregation printed
  1  usage/config error, or log is not enabled`)
}
