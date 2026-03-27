package shell

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ApplyNestRules checks if a command has nested commands (find -exec, xargs, bash -c, etc.)
// and returns a Topology for those nested commands if detected.
func ApplyNestRules(cmd *Command, call *syntax.CallExpr) *Topology {
	switch cmd.Name {
	case "find":
		return parseFindExec(cmd.Args)
	case "xargs":
		return parseXargs(cmd.Args)
	case "bash", "sh":
		return parseBashC(cmd.Args)
	case "eval":
		return parseEval(cmd.Args)
	case "env":
		return parseEnvCmd(cmd.Args)
	case "sudo", "doas", "su":
		return parseSudoCmd(cmd.Args)
	default:
		return nil
	}
}

// stripQuotes removes matching outer quotes from a string.
func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseFindExec extracts commands from find -exec CMD {} \;
// parseFindExec extracts ALL commands from find -exec/-execdir patterns.
func parseFindExec(args []string) *Topology {
	topo := &Topology{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-exec" || args[i] == "-execdir" {
			// Collect args until ; or +
			var cmdParts []string
			j := i + 1
			for ; j < len(args); j++ {
				if args[j] == ";" || args[j] == "+" || args[j] == `\;` {
					break
				}
				if args[j] == "{}" {
					continue // skip placeholder
				}
				cmdParts = append(cmdParts, args[j])
			}
			if len(cmdParts) > 0 {
				topo.Segments = append(topo.Segments, Segment{
					Type: SegmentTypeSingle,
					Commands: []Command{{
						Name:       cmdParts[0],
						Args:       cmdParts[1:],
						Analyzable: true,
					}},
				})
			}
			i = j // skip past the terminator
		}
	}
	if len(topo.Segments) == 0 {
		return nil
	}
	return topo
}

// xargsValueFlags lists all xargs flags that take a value argument.
var xargsValueFlags = map[string]bool{
	"-I": true, "-P": true, "-n": true, "-L": true,
	"-d": true, "-E": true, "-s": true, "-a": true,
	"--arg-file": true, "--max-procs": true, "--max-args": true,
	"--max-lines": true, "--delimiter": true, "--eof": true,
	"--replace": true, "--max-chars": true,
}

// parseXargs extracts the command from xargs CMD [args...]
func parseXargs(args []string) *Topology {
	cmdIdx := 0
	for cmdIdx < len(args) {
		arg := args[cmdIdx]
		if !strings.HasPrefix(arg, "-") {
			break
		}
		// Handle --flag=value form
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			cmdIdx++
			continue
		}
		if xargsValueFlags[arg] {
			cmdIdx += 2 // skip the flag and its value
		} else {
			cmdIdx++
		}
	}

	if cmdIdx >= len(args) {
		return nil
	}

	return &Topology{
		Segments: []Segment{{
			Type: SegmentTypeSingle,
			Commands: []Command{{
				Name:       args[cmdIdx],
				Args:       args[cmdIdx+1:],
				Analyzable: true,
			}},
		}},
	}
}

// parseBashC extracts and re-parses commands from bash -c "CMD" / sh -c "CMD"
func parseBashC(args []string) *Topology {
	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) {
			cmdStr := stripQuotes(args[i+1])
			if cmdStr == "" {
				return nil
			}

			// Re-parse the command string
			topo, err := BuildTopology(cmdStr)
			if err != nil {
				// If we can't parse it, mark as unanalyzable
				return &Topology{
					Segments: []Segment{{
						Type: SegmentTypeSingle,
						Commands: []Command{{
							Name:       "(unparseable)",
							Analyzable: false,
						}},
					}},
				}
			}
			return topo
		}
	}
	return nil
}

// parseEval handles eval "CMD" — only analyzable if the argument is a static string.
func parseEval(args []string) *Topology {
	if len(args) == 0 {
		return nil
	}

	// Join all args as the eval string
	evalStr := stripQuotes(strings.Join(args, " "))

	// Check if the eval string contains variable references (making it dynamic)
	if strings.ContainsAny(evalStr, "$`") {
		return &Topology{
			Segments: []Segment{{
				Type: SegmentTypeSingle,
				Commands: []Command{{
					Name:       "(dynamic-eval)",
					Analyzable: false,
				}},
			}},
		}
	}

	// Static eval string — re-parse it
	topo, err := BuildTopology(evalStr)
	if err != nil {
		return &Topology{
			Segments: []Segment{{
				Type: SegmentTypeSingle,
				Commands: []Command{{
					Name:       "(unparseable)",
					Analyzable: false,
				}},
			}},
		}
	}
	return topo
}

// parseEnvCmd extracts the actual command from env [VAR=val...] CMD [args...]
func parseEnvCmd(args []string) *Topology {
	cmdIdx := 0
	for cmdIdx < len(args) {
		arg := args[cmdIdx]
		if strings.HasPrefix(arg, "-") {
			cmdIdx++
			continue
		}
		if strings.Contains(arg, "=") {
			cmdIdx++
			continue
		}
		break
	}

	if cmdIdx >= len(args) {
		return nil
	}

	return &Topology{
		Segments: []Segment{{
			Type: SegmentTypeSingle,
			Commands: []Command{{
				Name:       args[cmdIdx],
				Args:       args[cmdIdx+1:],
				Analyzable: true,
			}},
		}},
	}
}

// parseSudoCmd extracts the actual command from sudo/doas [flags] CMD [args...]
func parseSudoCmd(args []string) *Topology {
	cmdIdx := 0
	sudoValueFlags := map[string]bool{
		"-u": true, "-g": true, "-C": true, "-D": true,
		"-R": true, "-T": true,
		"--user": true, "--group": true,
	}

	for cmdIdx < len(args) {
		arg := args[cmdIdx]
		if !strings.HasPrefix(arg, "-") {
			break
		}
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			cmdIdx++
			continue
		}
		if sudoValueFlags[arg] {
			cmdIdx += 2
		} else {
			cmdIdx++
		}
	}

	if cmdIdx >= len(args) {
		return nil
	}

	return &Topology{
		Segments: []Segment{{
			Type: SegmentTypeSingle,
			Commands: []Command{{
				Name:       args[cmdIdx],
				Args:       args[cmdIdx+1:],
				Analyzable: true,
			}},
		}},
	}
}
