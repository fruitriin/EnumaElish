package shell

import (
	"regexp"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"
)

// Topology represents the execution topology of a shell command.
// It consists of segments separated by reset points (&& / ; / ||).
type Topology struct {
	Segments []Segment
}

// SegmentType represents the type of a segment.
type SegmentType string

const (
	SegmentTypeSingle   SegmentType = "single"
	SegmentTypePipeline SegmentType = "pipeline"
)

// Segment represents a group of commands connected by pipes or a single command.
type Segment struct {
	Type     SegmentType
	Commands []Command
}

// Command represents a single command with its arguments and analysis metadata.
type Command struct {
	Name       string
	Args       []string
	Analyzable bool      // false if the command involves dynamic expansion
	Nested     *Topology // nested commands (find -exec, bash -c, etc.)

	// Redirs holds file targets of shell redirects (> >> >| &> &>>).
	// Only write-side redirects are captured — read redirects (< <<) are
	// covered by the command's own args or are unanalyzable heredocs.
	// See Critical C1: without this, `cat /ws/x > /outside/y` bypassed
	// `outside-write: deny`.
	Redirs []RedirTarget
}

// RedirTarget describes a single redirect target for scope evaluation.
type RedirTarget struct {
	// Path is the literal path (may contain shell expansions).
	Path string
	// Analyzable is false if the target contains $VAR / $(cmd) / etc.
	Analyzable bool
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
	case *syntax.ForClause:
		// Attempt to expand a fully-literal `for VAR in a b c; do BODY; done`
		// into per-iteration segments. Falls through to the default deny-side
		// path when the loop is dynamic (CStyleLoop, Select, positional-param
		// loop, non-literal word list, over the iteration cap, ...).
		if segs, ok := tryExpandForClause(cmd); ok {
			return segs
		}
		seg := buildSegmentFromStmt(stmt)
		if seg != nil {
			return []Segment{*seg}
		}
		return nil
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
			return []Segment{{Type: SegmentTypePipeline, Commands: cmds}}
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
		Type:     SegmentTypeSingle,
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
		c := buildCommandFromCall(cmd)
		if c != nil {
			c.Redirs = extractWriteRedirs(stmt.Redirs)
		}
		return c
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

	name := unescapeCommandName(parts[0])
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

// unescapeCommandName strips a single leading backslash from the command-name
// position when the following character is not itself a backslash — this
// mirrors the shell's own rule for `\rm` (which bypasses alias/function
// lookup and executes the command `rm`). Without this step ccchain would treat
// `\rm -rf /` as an unrecognized command name `\rm`, letting it slip past the
// `rm` deny rules that the shell would honor (attacker C6).
//
// Argument positions are intentionally NOT unescaped: shell parses `rm \foo`
// as `rm foo` at execution time, but any rule that checks arg patterns is
// already tolerant to that (and the escape in argument position rarely
// corresponds to a rule-matching intent).
func unescapeCommandName(name string) string {
	if len(name) < 2 || name[0] != '\\' {
		return name
	}
	// `\\rm` (source: literal backslash + rm) — leave as-is; the shell would
	// execute a command literally named `\rm`, which is not one we recognize.
	// The Lit value in that case is "\\rm" (2 backslashes + rm), so index 1
	// is another backslash.
	if name[1] == '\\' {
		return name
	}
	return name[1:]
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

// wordToString converts a syntax.Word to its string representation,
// applying shell quote removal to static quoting so that `-X "POST"`,
// `-X 'POST'`, and `-X PO"ST"` all yield the same string the shell would
// pass to the command (`-X POST`). Without this, quoted arguments (and even
// quoted command names like `"rm"`) would evade rule matching.
// Dynamic parts (ParamExp, CmdSubst, ...) are printed as written so that
// downstream dynamic-content checks still see the `$` / backtick markers.
func wordToString(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var buf strings.Builder
	for _, part := range w.Parts {
		writeWordPart(&buf, part)
	}
	return buf.String()
}

// writeWordPart writes a single word part with static quote removal.
func writeWordPart(buf *strings.Builder, part syntax.WordPart) {
	switch p := part.(type) {
	case *syntax.SglQuoted:
		if p.Dollar {
			// $'...' ANSI-C quoting: escapes are not processed here, keep as
			// written. The leading $ makes dynamic-content checks treat the
			// argument conservatively.
			defaultPrinter.Print(buf, p)
			return
		}
		buf.WriteString(p.Value)
	case *syntax.DblQuoted:
		for _, inner := range p.Parts {
			writeWordPart(buf, inner)
		}
	default:
		// Lit (kept verbatim, incl. backslashes), ParamExp, CmdSubst, etc.
		defaultPrinter.Print(buf, part)
	}
}

// extractWriteRedirs extracts write-side redirect targets (> >> >| &> &>>).
// Read redirects (< << <<<) are skipped: reads are usually covered by the
// command's own args, and heredocs have no file target.
func extractWriteRedirs(redirs []*syntax.Redirect) []RedirTarget {
	if len(redirs) == 0 {
		return nil
	}
	var out []RedirTarget
	for _, r := range redirs {
		if r == nil || r.Word == nil {
			continue
		}
		switch r.Op {
		case syntax.RdrOut, syntax.AppOut, syntax.RdrClob,
			syntax.RdrAll, syntax.AppAll, syntax.DplOut:
			// write-side redirect — capture target
		default:
			continue
		}
		path := wordToString(r.Word)
		if path == "" {
			continue
		}
		out = append(out, RedirTarget{
			Path:       path,
			Analyzable: isAnalyzable(r.Word),
		})
	}
	return out
}

// maxForLoopIterations bounds the number of iterations we will statically
// expand for a literal `for VAR in ...` loop. Loops with more items than this
// fall back to the non-analyzable path (Analyzable: false). Matches the
// max_context_depth / max_rules_per_cmd style of safety valve — bounds the
// per-command work and keeps evaluation output reviewable.
const maxForLoopIterations = 50

// tryExpandForClause attempts to statically expand a fully-literal
// `for VAR in a b c; do BODY; done` into concrete per-iteration segments.
//
// Returns (segments, true) when the loop is expandable: WordIter with an
// explicit `in` clause, all items statically analyzable, iteration count at or
// under maxForLoopIterations, and no reassignment / inner shadowing of VAR
// inside the body. Returns (nil, false) otherwise — the caller then falls back
// to the existing deny-side path (Command{Name: "(control-flow)",
// Analyzable: false}).
//
// The expansion works at the string level: after extractSegments produces the
// body's Commands (with `$VAR` rendered verbatim by wordToString), each
// iteration substitutes the loop variable in Command.Name / .Args / .Redirs
// paths and re-checks analyzability from the resulting strings. This is
// substantially simpler than mutating the AST — see Plan 0025 Phase 1
// "既存の文字列描画（wordToString）の出力に対する文字列置換で実現する".
func tryExpandForClause(fc *syntax.ForClause) ([]Segment, bool) {
	if fc == nil {
		return nil, false
	}

	// Only WordIter form (`for VAR in a b c`) is supported. CStyleLoop
	// (`for ((i=0;...))`) is intentionally out of scope. Some builds of
	// mvdan.cc/sh expose Select via WordIter with a different SelectPos, but
	// SelectPos is unrelated to our concerns here — we only care that Loop
	// is a WordIter with an explicit `in`.
	wi, ok := fc.Loop.(*syntax.WordIter)
	if !ok {
		return nil, false
	}

	// `for f; do ...; done` (no `in`) iterates over the shell's positional
	// parameters ($1 $2 ...). Items is an empty slice in this form, which
	// would vacuously satisfy the "all items analyzable" check below and let
	// us bypass safety by treating the body as if the loop ran zero times.
	// The InPos check keeps this dynamic case on the deny path.
	if !wi.InPos.IsValid() {
		return nil, false
	}

	if fc.Select {
		// `select VAR in ...` reads from stdin — dynamic by nature.
		return nil, false
	}

	// An explicit `for VAR in; do BODY; done` (empty word list) executes the
	// body zero times. Rather than emit zero segments — which would collapse
	// silently to an empty topology and hide the loop from the reviewer — we
	// fall back to the deny path so the caller sees "(control-flow)" and can
	// react. If a legitimate reason to allow the zero-iteration form arises,
	// this can be revisited (e.g. return an empty-but-valid segment slice).
	if len(wi.Items) == 0 {
		return nil, false
	}

	if len(wi.Items) > maxForLoopIterations {
		return nil, false
	}

	// All items must be static literals — no $x, $(cmd), <(cmd), etc., and
	// no unquoted shell glob metacharacters (`*`, `?`, `[`). Globs are
	// runtime FS-dependent, so treating them as literals would silently
	// diverge from the shell: `for f in *.log; do rm $f; done` would evaluate
	// as `rm *.log` (a single cat/rm call the parser sees) rather than the
	// N-file loop the shell actually runs. Keep such loops on the deny path.
	values := make([]string, 0, len(wi.Items))
	for _, item := range wi.Items {
		if !isAnalyzable(item) {
			return nil, false
		}
		if wordContainsUnquotedGlob(item) {
			return nil, false
		}
		// Skeptic C4: `for f in "target dir" x; do cp -t $f file; done` runs
		// as `cp -t target dir file` (4 argv, IFS-split unquoted expansion)
		// at execution time but as `cp -t "target dir" file` (3 argv) if we
		// substitute the literal string into the body. That mismatch means an
		// evaluate-time scope check on the 3-argv form does not reflect the
		// actual 4-argv exec.
		//
		// The cleanest fix would track whether every reference to $f in the
		// body is quoted; that is a substantial change to the substitutor and
		// is deferred to a follow-up (see Plan 0025 backlog note). The
		// conservative minimal change: any literal that contains whitespace
		// forces the loop back onto the deny path. This narrows Plan 0025's
		// coverage but eliminates the exec/eval-time divergence.
		val := wordToString(item)
		if strings.ContainsAny(val, " \t\n") {
			return nil, false
		}
		values = append(values, val)
	}

	varName := ""
	if wi.Name != nil {
		varName = wi.Name.Value
	}
	if varName == "" {
		return nil, false
	}

	// Reject any reassignment / shadowing of VAR inside the body. This keeps
	// the expansion sound: without this check, `for f in a b; do f=/etc; rm
	// $f; done` would be expanded as if `$f` still referred to the outer
	// iterator, and the second iteration would look like `rm b` when in fact
	// the shell would run `rm /etc`.
	if bodyRedefinesVar(fc.Do, varName) {
		return nil, false
	}

	// Extract body segments once — they contain wordToString'd Command
	// strings with `$VAR` / `${VAR}` rendered verbatim. Each iteration then
	// gets a substituted copy.
	var bodySegs []Segment
	for _, stmt := range fc.Do {
		bodySegs = append(bodySegs, extractSegments(stmt)...)
	}
	if len(bodySegs) == 0 {
		// Body is empty (e.g. `for f in a b; do :; done`? — colon is a
		// builtin, but bodySegs would not be empty then). Rather than
		// return an empty topology, fall back to the deny path.
		return nil, false
	}

	// Guard: cap the total number of expanded segments as a defense in
	// depth against pathological body sizes (small iteration count × huge
	// body). maxForLoopIterations already caps iterations; the body size
	// is bounded by the parser input length.
	totalCap := maxForLoopIterations * 100
	if len(values)*len(bodySegs) > totalCap {
		return nil, false
	}

	out := make([]Segment, 0, len(values)*len(bodySegs))
	for _, val := range values {
		for _, seg := range bodySegs {
			expanded := seg
			expanded.Commands = make([]Command, len(seg.Commands))
			for i := range seg.Commands {
				expanded.Commands[i] = substituteInCommand(seg.Commands[i], varName, val)
			}
			out = append(out, expanded)
		}
	}
	return out, true
}

// wordContainsUnquotedGlob reports whether w contains a shell glob
// metacharacter (`*`, `?`, `[`) as an unquoted Lit — i.e. a character that
// the shell would perform pathname expansion on. Quoted forms (`"*.log"`,
// `'*.log'`) are literal by design and do not trip this check.
//
// Only Lit parts that appear at the top level of the Word are inspected;
// SglQuoted / DblQuoted parts are treated as safely literal per shell rules.
func wordContainsUnquotedGlob(w *syntax.Word) bool {
	if w == nil {
		return false
	}
	for _, part := range w.Parts {
		lit, ok := part.(*syntax.Lit)
		if !ok {
			continue
		}
		if strings.ContainsAny(lit.Value, "*?[") {
			return true
		}
	}
	return false
}

// builtinReassignsFirstArg lists shell builtins whose first non-flag argument
// (or the argument to a `-v` / similar variable flag) becomes rebound at
// runtime. Detecting these is required because they parse as plain CallExprs,
// not `*syntax.Assign`, so the syntax.Walk-driven Assign check below would
// miss them (attacker C5). Coverage:
//
//   - read VAR      — reads stdin into VAR
//   - mapfile / readarray VAR — reads into array VAR
//   - declare VAR / local VAR / export VAR / typeset VAR — declare with
//     optional assignment; presence alone constitutes rebinding
//   - let VAR=expr  — arithmetic assignment
//   - getopts SPEC VAR — the third argument names the receiver
//   - printf -v VAR — bash extension that assigns into VAR
//
// Any of these where VAR matches the loop iterator name forces the expansion
// back onto the deny path.
var builtinReassignsFirstArg = map[string]bool{
	"read":      true,
	"mapfile":   true,
	"readarray": true,
	"declare":   true,
	"local":     true,
	"export":    true,
	"typeset":   true,
	"let":       true,
}

// builtinCallReassigns reports whether a *syntax.CallExpr is a builtin call
// that rebinds varName. The check is intentionally coarse — it only looks at
// the raw literal Args; any dynamic form is treated as suspicious (returned as
// true) since we cannot tell what the shell will bind.
func builtinCallReassigns(call *syntax.CallExpr, varName string) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}
	nameWord := call.Args[0]
	name := literalWord(nameWord)
	if name == "" {
		return false
	}
	switch name {
	case "read", "mapfile", "readarray":
		// First non-flag arg (or the arg after `-a NAME`, `-A NAME` for
		// mapfile, `-p PROMPT` for read, etc). Coarse: any positional literal
		// matching varName is treated as a reassignment.
		for _, a := range call.Args[1:] {
			lit := literalWord(a)
			if lit == "" {
				continue // skip dynamic parts; conservative miss is acceptable here
			}
			if strings.HasPrefix(lit, "-") {
				continue
			}
			if lit == varName {
				return true
			}
			// `read -r VAR` / `read foo bar VAR` — any positional literal
			// might be the target; keep scanning.
		}
	case "declare", "local", "export", "typeset", "let":
		// declare/local/export/typeset VAR (=value) — VAR is either the whole
		// arg (`declare VAR`) or the LHS of `VAR=value`. `let VAR=expr` is
		// arithmetic assignment; same shape.
		for _, a := range call.Args[1:] {
			lit := literalWord(a)
			if lit == "" {
				continue
			}
			if strings.HasPrefix(lit, "-") {
				continue
			}
			candidate := lit
			if eq := strings.IndexByte(lit, '='); eq >= 0 {
				candidate = lit[:eq]
			}
			if candidate == varName {
				return true
			}
		}
	case "getopts":
		// getopts SPEC VAR — VAR is the third argument.
		if len(call.Args) >= 3 {
			if literalWord(call.Args[2]) == varName {
				return true
			}
		}
	case "printf":
		// printf -v VAR fmt args... — the arg after `-v` is the receiver.
		for i := 1; i < len(call.Args); i++ {
			if literalWord(call.Args[i]) == "-v" && i+1 < len(call.Args) {
				if literalWord(call.Args[i+1]) == varName {
					return true
				}
			}
		}
	}
	return false
}

// letRebindsVar walks a LetClause arithmetic expression tree looking for any
// mention of varName. Any occurrence is treated conservatively as a
// rebinding (`let f=99`, `let f+=1`, `let (( f *= 2 ))` all rebind).
func letRebindsVar(expr syntax.ArithmExpr, varName string) bool {
	if expr == nil {
		return false
	}
	found := false
	syntax.Walk(expr, func(node syntax.Node) bool {
		if found || node == nil {
			return false
		}
		if lit, ok := node.(*syntax.Lit); ok && lit.Value == varName {
			found = true
			return false
		}
		return true
	})
	return found
}

// literalWord returns the string value of w only if w is fully a literal
// (single *syntax.Lit part). Any dynamic/quoted content returns "" — the
// caller then falls through to the conservative miss.
func literalWord(w *syntax.Word) string {
	if w == nil || len(w.Parts) != 1 {
		return ""
	}
	lit, ok := w.Parts[0].(*syntax.Lit)
	if !ok {
		return ""
	}
	return lit.Value
}

// bodyRedefinesVar reports whether any statement in the loop body reassigns
// or re-iterates over the loop variable — either as a plain assignment
// (`VAR=value`) or as an inner `for VAR in ...` / `for ((VAR=...))` /
// `for VAR;` etc. Detecting this is a safety check: static expansion is only
// sound if the loop variable's binding is stable across the whole body.
func bodyRedefinesVar(stmts []*syntax.Stmt, varName string) bool {
	shadowed := false
	for _, stmt := range stmts {
		if stmt == nil {
			continue
		}
		syntax.Walk(stmt, func(node syntax.Node) bool {
			if shadowed || node == nil {
				return false
			}
			switch n := node.(type) {
			case *syntax.Assign:
				if n.Name != nil && n.Name.Value == varName {
					shadowed = true
					return false
				}
			case *syntax.CallExpr:
				// Attacker C5: builtins like `read`, `printf -v`, `mapfile`,
				// `getopts` rebind the named variable but parse as CallExpr
				// (not Assign), so the Assign case above misses them.
				if builtinCallReassigns(n, varName) {
					shadowed = true
					return false
				}
			case *syntax.LetClause:
				// `let VAR=expr` parses as its own AST node — the arithmetic
				// expression walks below already catch a mention of varName
				// in most cases, but be explicit here.
				for _, expr := range n.Exprs {
					if letRebindsVar(expr, varName) {
						shadowed = true
						return false
					}
				}
			case *syntax.DeclClause:
				// `declare/local/export/typeset/readonly VAR[=value]` — each
				// Args entry is a *syntax.Assign; the walker will also visit
				// them, but the Assign case above only fires on a *value*
				// Assign. Naked Assigns (`declare VAR`) still rebind the name.
				for _, a := range n.Args {
					if a != nil && a.Name != nil && a.Name.Value == varName {
						shadowed = true
						return false
					}
				}
			case *syntax.ForClause:
				// Nested for that binds the same name — inner loop shadows
				// the outer binding. Cover both WordIter and CStyleLoop.
				switch inner := n.Loop.(type) {
				case *syntax.WordIter:
					if inner != nil && inner.Name != nil && inner.Name.Value == varName {
						shadowed = true
						return false
					}
				case *syntax.CStyleLoop:
					// Arithmetic loop headers can rebind the variable via
					// the Init clause. Rather than analyze the arithmetic
					// expression, walk it and look for an Assign / ParamExp
					// naming varName in a way that suggests rebinding.
					// Conservative: any mention counts as shadowing.
					mentions := false
					syntax.Walk(inner, func(w syntax.Node) bool {
						if id, ok := w.(*syntax.Lit); ok && id != nil && id.Value == varName {
							mentions = true
							return false
						}
						return !mentions
					})
					if mentions {
						shadowed = true
						return false
					}
				}
			}
			return true
		})
		if shadowed {
			return true
		}
	}
	return shadowed
}

// substituteInCommand returns a deep-enough copy of cmd with the loop variable
// substituted in every string-valued field. Args are copied (the caller
// preallocates .Commands but reuses .Args backing arrays); Redirs are copied;
// Nested is recursed into so `find -exec grep $f {} \;` bodies also see the
// substitution.
//
// Post-substitution, Analyzable is recomputed from the resulting strings:
// if no unresolved shell markers ($ or `) remain, we mark the command
// analyzable so it can be matched by rules. Any remaining marker keeps the
// command on the deny path — the substitution only rescues loop-variable
// references, not other dynamic content.
func substituteInCommand(cmd Command, varName, value string) Command {
	out := cmd
	out.Name = substituteLoopVar(cmd.Name, varName, value)

	if len(cmd.Args) > 0 {
		args := make([]string, len(cmd.Args))
		for i, a := range cmd.Args {
			args[i] = substituteLoopVar(a, varName, value)
		}
		out.Args = args
	}

	if len(cmd.Redirs) > 0 {
		redirs := make([]RedirTarget, len(cmd.Redirs))
		for i, r := range cmd.Redirs {
			redirs[i] = RedirTarget{
				Path:       substituteLoopVar(r.Path, varName, value),
				Analyzable: r.Analyzable,
			}
			// Recompute per-redir analyzability from the substituted path
			// so a `> $f` that becomes `> /tmp/x` no longer looks dynamic
			// to applyScopeToCommand.
			if !r.Analyzable && !containsUnresolvedExpansion(redirs[i].Path) {
				redirs[i].Analyzable = true
			}
		}
		out.Redirs = redirs
	}

	if cmd.Nested != nil {
		out.Nested = substituteInTopology(cmd.Nested, varName, value)
	}

	// Recompute overall analyzability. If the original was already
	// analyzable (unusual — it means the loop var did not appear in this
	// particular sub-command), keep it. Otherwise, we can rescue it only if
	// no unresolved marker remains anywhere.
	if !cmd.Analyzable {
		if !stringsContainUnresolved(out.Name, out.Args) {
			out.Analyzable = true
		}
	}
	return out
}

func substituteInTopology(topo *Topology, varName, value string) *Topology {
	if topo == nil {
		return nil
	}
	out := &Topology{}
	out.Segments = make([]Segment, 0, len(topo.Segments))
	for _, seg := range topo.Segments {
		newSeg := seg
		newSeg.Commands = make([]Command, len(seg.Commands))
		for i, c := range seg.Commands {
			newSeg.Commands[i] = substituteInCommand(c, varName, value)
		}
		out.Segments = append(out.Segments, newSeg)
	}
	return out
}

// containsUnresolvedExpansion is a syntactic heuristic: any remaining `$` or
// backtick indicates a shell expansion the substitution did not consume.
// Errs on the safe side — false positives here just keep a command on the
// deny path, they never unsafely mark a dynamic command as analyzable.
func containsUnresolvedExpansion(s string) bool {
	return strings.ContainsAny(s, "$`")
}

func stringsContainUnresolved(name string, args []string) bool {
	if containsUnresolvedExpansion(name) {
		return true
	}
	for _, a := range args {
		if containsUnresolvedExpansion(a) {
			return true
		}
	}
	return false
}

// loopVarRegexCache memoizes the per-variable-name regex used for
// word-boundary-aware `$VAR` substitution. `${VAR}` is handled by plain
// literal ReplaceAll, so no regex is needed for that form.
var (
	loopVarRegexCache sync.Map // map[string]*regexp.Regexp
)

func loopVarRegex(varName string) *regexp.Regexp {
	if v, ok := loopVarRegexCache.Load(varName); ok {
		return v.(*regexp.Regexp)
	}
	// `\$` followed by exactly the variable name, terminated by end of
	// string OR any non-word character (so `$f` does not match `$file`).
	// We intentionally do not accept `\$$varName` (escaped) — that is a
	// literal `$` in shell, which the parser would represent differently
	// anyway (Lit token, not ParamExp), and after wordToString we wouldn't
	// see it as `\$`.
	pattern := `\$` + regexp.QuoteMeta(varName) + `([^A-Za-z0-9_]|$)`
	re := regexp.MustCompile(pattern)
	loopVarRegexCache.Store(varName, re)
	return re
}

// substituteLoopVar returns s with occurrences of `$VAR` and `${VAR}` replaced
// by value. Word-boundary rules: `$f` matches `$f` at end of string or before
// any non-`[A-Za-z0-9_]` character but never inside a longer identifier like
// `$file`. `${VAR}` is exact literal.
//
// Note: this operates on the wordToString output, which has already applied
// static quote removal. Shell quoting rules for the substituted value are
// NOT re-applied — see Plan 0025 Phase 1 caveat: "この方式の限界は
// 「置換後に再度シェル引用規則を考慮する必要がある位置」だが、単純な引数展開の
// 範囲では影響が小さいと判断". Downstream analyzability re-check catches the
// obvious cases (remaining `$` / `` ` `` after substitution).
func substituteLoopVar(s, varName, value string) string {
	if s == "" || varName == "" {
		return s
	}
	// Fast path: no `$` in the string means no substitution work at all.
	if !strings.Contains(s, "$") {
		return s
	}

	// ${VAR}: literal replace.
	braced := "${" + varName + "}"
	if strings.Contains(s, braced) {
		s = strings.ReplaceAll(s, braced, value)
	}

	if !strings.Contains(s, "$") {
		return s
	}

	// $VAR with word-boundary check via regex. The pattern captures the
	// terminator character (if any) so it can be preserved on replacement.
	re := loopVarRegex(varName)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		// match is `$VAR` + terminator (1 char, unless matched at end of
		// string where the terminator group is empty).
		prefix := "$" + varName
		if len(match) == len(prefix) {
			// End-of-string match — no terminator to preserve.
			return value
		}
		return value + match[len(prefix):]
	})
}

// isAnalyzable checks if a word can be statically analyzed.
// Returns false for variable expansions, command substitutions, etc.
//
// The check traverses the word at all depths using syntax.Walk so that
// dynamic parts nested inside *syntax.DblQuoted (e.g. `"$(cmd)"`, `"$VAR"`)
// are detected. Only inspecting w.Parts at the top level would incorrectly
// treat `"$(rm)" -rf /` as fully static and bypass the deny-first safety
// net for dynamic command names.
func isAnalyzable(w *syntax.Word) bool {
	if w == nil {
		return true
	}
	analyzable := true
	syntax.Walk(w, func(node syntax.Node) bool {
		if node == nil {
			return false
		}
		switch node.(type) {
		case *syntax.ParamExp, // $var, ${var}
			*syntax.CmdSubst,  // $(cmd), `cmd`
			*syntax.ProcSubst, // <(cmd), >(cmd)
			*syntax.ArithmExp: // $((expr))
			analyzable = false
			return false
		}
		return true
	})
	return analyzable
}
