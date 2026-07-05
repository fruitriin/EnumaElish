package shell

import (
	"strings"

	"mvdan.cc/sh/v3/expand"
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

// unquoteWords parses s as a command line consisting only of static shell
// words and returns their literal values joined by single spaces. Quotes are
// removed and escape sequences (\", \', "a \"b\"", 'it'\''s', $'..', etc.)
// are resolved by the shell parser itself, replacing the previous naive
// outer-quote stripping (Plan 0006 S-6).
//
// ok is false when s cannot be statically resolved: dynamic expansion at any
// nesting depth ($var, $(cmd), `cmd`, <(cmd), $((expr))), unclosed quotes,
// redirects, assignments, or multiple statements. Callers decide how to
// handle unresolvable input (treat as dynamic, or fall back to raw).
func unquoteWords(s string) (string, bool) {
	// Fast path: no quoting or escaping characters — nothing to resolve.
	if !strings.ContainsAny(s, `"'\`) {
		return s, true
	}

	file, err := ParseCommand(s)
	if err != nil || len(file.Stmts) != 1 {
		return "", false
	}
	stmt := file.Stmts[0]
	if len(stmt.Redirs) > 0 || stmt.Negated || stmt.Background || stmt.Coprocess {
		return "", false
	}
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Assigns) > 0 || len(call.Args) == 0 {
		return "", false
	}

	parts := make([]string, 0, len(call.Args))
	for _, w := range call.Args {
		if !isStaticWord(w) {
			return "", false
		}
		// expand.Fields (not expand.Literal) matches execution semantics:
		// it also removes unquoted backslash escapes (echo \"hi\" → echo "hi",
		// 'it'\''s' → it's), which Literal keeps verbatim.
		// Fresh Config per call: expand mutates the config in place, so
		// sharing one (or passing nil, which shares a package-level
		// zeroConfig) is not concurrency-safe.
		fields, err := expand.Fields(&expand.Config{}, w)
		if err != nil || len(fields) != 1 {
			// A static word never splits into multiple fields here (the
			// parser already split words); != 1 means brace expansion or
			// similar — bail out rather than guess.
			return "", false
		}
		parts = append(parts, fields[0])
	}
	return strings.Join(parts, " "), true
}

// hasDynamicNode reports whether the AST subtree contains any dynamic
// expansion node (parameter expansion, command substitution, process
// substitution, arithmetic expansion) at any nesting depth. Unlike
// isAnalyzable, it recurses into double-quoted parts, so `"echo $x"` is
// correctly detected as dynamic. Escaped forms (`\$x`) parse as literals
// and are correctly treated as static.
func hasDynamicNode(node syntax.Node) bool {
	dynamic := false
	syntax.Walk(node, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.ParamExp, *syntax.CmdSubst, *syntax.ProcSubst, *syntax.ArithmExp:
			dynamic = true
		}
		return !dynamic
	})
	return dynamic
}

// isStaticWord reports whether w contains no dynamic expansion at any depth.
func isStaticWord(w *syntax.Word) bool {
	return !hasDynamicNode(w)
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

// parseBashC extracts and re-parses commands from bash -c "CMD" / sh -c "CMD".
//
// The `raw` string here is already post-`wordToString`: upstream quote removal
// has run, so `raw` is exactly what the outer shell would hand bash -c. That
// means we can re-parse it directly — no `unquoteWords` normalization step,
// which would otherwise consume backslash escapes (e.g. turn `echo \$x` into
// `echo $x`) and false-flag the script as dynamic when it is actually static.
func parseBashC(args []string) *Topology {
	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) {
			cmdStr := args[i+1]
			if cmdStr == "" {
				return nil
			}

			// Detect unresolved dynamic expansion anywhere in the script,
			// mirroring parseEval's (dynamic-eval). An unresolved $x can
			// rewrite the script structure (x='; rm -rf /'), so the nested
			// command cannot be known statically. Escaped forms (`\$x`)
			// parse as literals and are correctly treated as static.
			if file, err := ParseCommand(cmdStr); err == nil && hasDynamicNode(file) {
				return &Topology{
					Segments: []Segment{{
						Type: SegmentTypeSingle,
						Commands: []Command{{
							Name:       "(dynamic-shell)",
							Analyzable: false,
						}},
					}},
				}
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
//
// Each arg here has already been through `wordToString` (static outer quotes
// removed), so joining reproduces what eval would concatenate at runtime. We
// skip `unquoteWords` because it would consume backslash escapes and
// false-flag `eval 'echo \$x'` (literal `$x`) as dynamic.
func parseEval(args []string) *Topology {
	if len(args) == 0 {
		return nil
	}

	// eval concatenates its arguments with spaces before re-parsing.
	evalStr := strings.Join(args, " ")

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
