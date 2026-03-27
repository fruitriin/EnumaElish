package shell

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Topology represents the execution topology of a shell command.
// It consists of segments separated by reset points (&& / ; / ||).
type Topology struct {
	Segments []Segment
}

// Segment represents a group of commands connected by pipes or a single command.
type Segment struct {
	Type     string    // "pipeline" or "single"
	Commands []Command
}

// Command represents a single command with its arguments and analysis metadata.
type Command struct {
	Name       string
	Args       []string
	Analyzable bool      // false if the command involves dynamic expansion
	Nested     *Topology // nested commands (find -exec, bash -c, etc.)
}

// BuildTopology converts a shell command string into an execution topology.
func BuildTopology(cmd string) (*Topology, error) {
	file, err := ParseCommand(cmd)
	if err != nil {
		return nil, err
	}

	topo := &Topology{}
	for _, stmt := range file.Stmts {
		segments := extractSegments(stmt)
		topo.Segments = append(topo.Segments, segments...)
	}

	return topo, nil
}

// extractSegments recursively extracts segments from a statement.
// && / || / ; are reset points that create new segments.
func extractSegments(stmt *syntax.Stmt) []Segment {
	if stmt == nil {
		return nil
	}

	switch cmd := stmt.Cmd.(type) {
	case *syntax.BinaryCmd:
		return extractBinarySegments(cmd)
	default:
		seg := buildSegmentFromStmt(stmt)
		if seg != nil {
			return []Segment{*seg}
		}
		return nil
	}
}

// extractBinarySegments handles BinaryCmd nodes (&&, ||, |).
func extractBinarySegments(bin *syntax.BinaryCmd) []Segment {
	switch bin.Op {
	case syntax.AndStmt, syntax.OrStmt:
		// && and || are reset points — each side is an independent segment
		left := extractSegments(bin.X)
		right := extractSegments(bin.Y)
		return append(left, right...)

	case syntax.Pipe, syntax.PipeAll:
		// | is a pipeline — commands are in parent-child relationship
		cmds := flattenPipeline(bin)
		if len(cmds) > 0 {
			return []Segment{{Type: "pipeline", Commands: cmds}}
		}
		return nil

	default:
		return nil
	}
}

// flattenPipeline flattens a pipe chain into a list of commands.
func flattenPipeline(bin *syntax.BinaryCmd) []Command {
	if bin.Op != syntax.Pipe && bin.Op != syntax.PipeAll {
		return nil
	}

	var cmds []Command

	// Left side
	if leftBin, ok := bin.X.Cmd.(*syntax.BinaryCmd); ok && (leftBin.Op == syntax.Pipe || leftBin.Op == syntax.PipeAll) {
		cmds = append(cmds, flattenPipeline(leftBin)...)
	} else {
		cmd := buildCommandFromStmt(bin.X)
		if cmd != nil {
			cmds = append(cmds, *cmd)
		}
	}

	// Right side
	if rightBin, ok := bin.Y.Cmd.(*syntax.BinaryCmd); ok && (rightBin.Op == syntax.Pipe || rightBin.Op == syntax.PipeAll) {
		cmds = append(cmds, flattenPipeline(rightBin)...)
	} else {
		cmd := buildCommandFromStmt(bin.Y)
		if cmd != nil {
			cmds = append(cmds, *cmd)
		}
	}

	return cmds
}

// buildSegmentFromStmt builds a Segment from a single statement.
func buildSegmentFromStmt(stmt *syntax.Stmt) *Segment {
	cmd := buildCommandFromStmt(stmt)
	if cmd == nil {
		return nil
	}
	return &Segment{
		Type:     "single",
		Commands: []Command{*cmd},
	}
}

// buildCommandFromStmt extracts a Command from a statement node.
func buildCommandFromStmt(stmt *syntax.Stmt) *Command {
	if stmt == nil {
		return nil
	}

	switch cmd := stmt.Cmd.(type) {
	case *syntax.CallExpr:
		return buildCommandFromCall(cmd)
	case *syntax.Subshell:
		// (cmd1; cmd2) — treat as a single unanalyzable command
		return &Command{Name: "(subshell)", Analyzable: false}
	case *syntax.BinaryCmd:
		// This shouldn't happen at this level (handled by extractBinarySegments)
		// but handle gracefully
		return &Command{Name: "(binary)", Analyzable: false}
	case *syntax.ForClause, *syntax.WhileClause, *syntax.IfClause,
		*syntax.CaseClause, *syntax.Block:
		// Control flow structures contain commands that can't be statically
		// evaluated in isolation — deny for safety
		return &Command{Name: "(control-flow)", Analyzable: false}
	case *syntax.FuncDecl:
		// Function definitions hide their body from evaluation
		return &Command{Name: "(func-decl)", Analyzable: false}
	default:
		// Unknown AST node types — deny for safety (never return nil)
		return &Command{Name: "(unknown-stmt)", Analyzable: false}
	}
}

// buildCommandFromCall extracts command name, args, and analyzability from a CallExpr.
func buildCommandFromCall(call *syntax.CallExpr) *Command {
	if len(call.Args) == 0 {
		return nil
	}

	parts := wordParts(call.Args)
	if len(parts) == 0 {
		return &Command{Name: "(empty)", Analyzable: false}
	}

	name := parts[0]
	args := parts[1:]
	analyzable := isAnalyzable(call.Args[0])

	cmd := &Command{
		Name:       name,
		Args:       args,
		Analyzable: analyzable,
	}

	// Apply custom nest rules
	nested := ApplyNestRules(cmd, call)
	if nested != nil {
		cmd.Nested = nested
	}

	return cmd
}

// wordParts extracts string representations of word arguments.
func wordParts(args []*syntax.Word) []string {
	var parts []string
	for _, w := range args {
		parts = append(parts, wordToString(w))
	}
	return parts
}

// defaultPrinter is reused across calls (Printer is stateless).
var defaultPrinter = syntax.NewPrinter()

// wordToString converts a syntax.Word to its string representation.
func wordToString(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var buf strings.Builder
	defaultPrinter.Print(&buf, w)
	return buf.String()
}

// isAnalyzable checks if a word can be statically analyzed.
// Returns false for variable expansions, command substitutions, etc.
func isAnalyzable(w *syntax.Word) bool {
	for _, part := range w.Parts {
		switch part.(type) {
		case *syntax.ParamExp:
			return false // $var, ${var}
		case *syntax.CmdSubst:
			return false // $(cmd)
		case *syntax.ProcSubst:
			return false // <(cmd)
		case *syntax.ArithmExp:
			return false // $((expr))
		}
	}
	return true
}
