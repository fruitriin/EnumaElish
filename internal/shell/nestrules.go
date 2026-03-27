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
func parseFindExec(args []string) *Topology {
	for i, arg := range args {
		if arg == "-exec" || arg == "-execdir" {
			// Collect args until ; or +
			var cmdParts []string
			for j := i + 1; j < len(args); j++ {
				if args[j] == ";" || args[j] == "+" || args[j] == `\;` {
					break
				}
				if args[j] == "{}" {
					continue // skip placeholder
				}
				cmdParts = append(cmdParts, args[j])
			}
			if len(cmdParts) > 0 {
				return &Topology{
					Segments: []Segment{{
						Type: "single",
						Commands: []Command{{
							Name:       cmdParts[0],
							Args:       cmdParts[1:],
							Analyzable: true,
						}},
					}},
				}
			}
		}
	}
	return nil
}

// parseXargs extracts the command from xargs CMD [args...]
func parseXargs(args []string) *Topology {
	// Skip xargs flags (start with -)
	cmdIdx := 0
	for cmdIdx < len(args) {
		if !strings.HasPrefix(args[cmdIdx], "-") {
			break
		}
		// Some flags take a value: -I, -P, -n, -L, -d, -E, -s
		switch args[cmdIdx] {
		case "-I", "-P", "-n", "-L", "-d", "-E", "-s":
			cmdIdx += 2 // skip the flag and its value
		default:
			cmdIdx++ // skip the flag only
		}
	}

	if cmdIdx >= len(args) {
		return nil
	}

	return &Topology{
		Segments: []Segment{{
			Type: "single",
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
						Type: "single",
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
				Type: "single",
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
				Type: "single",
				Commands: []Command{{
					Name:       "(unparseable)",
					Analyzable: false,
				}},
			}},
		}
	}
	return topo
}
