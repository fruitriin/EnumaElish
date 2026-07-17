package shell

import (
	"strings"
	"testing"
)

func TestBuildTopologySimple(t *testing.T) {
	topo, err := BuildTopology("ls -la")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	seg := topo.Segments[0]
	assertEqual(t, "type", seg.Type, SegmentTypeSingle)
	assertEqual(t, "cmd", seg.Commands[0].Name, "ls")
	assertEqual(t, "analyzable", seg.Commands[0].Analyzable, true)
}

func TestBuildTopologyPipeline(t *testing.T) {
	topo, err := BuildTopology("find . | grep foo | wc -l")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (pipeline), got %d", len(topo.Segments))
	}
	seg := topo.Segments[0]
	assertEqual(t, "type", seg.Type, SegmentTypePipeline)
	if len(seg.Commands) != 3 {
		t.Fatalf("expected 3 commands in pipeline, got %d", len(seg.Commands))
	}
	assertEqual(t, "cmd[0]", seg.Commands[0].Name, "find")
	assertEqual(t, "cmd[1]", seg.Commands[1].Name, "grep")
	assertEqual(t, "cmd[2]", seg.Commands[2].Name, "wc")
}

func TestBuildTopologyAndReset(t *testing.T) {
	topo, err := BuildTopology("find . && rm foo")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 segments (&&  reset), got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].cmd", topo.Segments[0].Commands[0].Name, "find")
	assertEqual(t, "seg[1].cmd", topo.Segments[1].Commands[0].Name, "rm")
}

func TestBuildTopologyOrReset(t *testing.T) {
	topo, err := BuildTopology("test -f foo || echo missing")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 segments (|| reset), got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].cmd", topo.Segments[0].Commands[0].Name, "test")
	assertEqual(t, "seg[1].cmd", topo.Segments[1].Commands[0].Name, "echo")
}

func TestBuildTopologySemicolon(t *testing.T) {
	topo, err := BuildTopology("echo hello; echo world")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 segments (; reset), got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].cmd", topo.Segments[0].Commands[0].Name, "echo")
	assertEqual(t, "seg[1].cmd", topo.Segments[1].Commands[0].Name, "echo")
}

func TestBuildTopologyPipeAndReset(t *testing.T) {
	// find . | rm should be 1 pipeline segment
	// find . && rm should be 2 segments
	topo1, err := BuildTopology("find . | rm")
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	if len(topo1.Segments) != 1 {
		t.Fatalf("pipe: expected 1 segment, got %d", len(topo1.Segments))
	}
	assertEqual(t, "pipe type", topo1.Segments[0].Type, SegmentTypePipeline)
	if len(topo1.Segments[0].Commands) != 2 {
		t.Fatalf("pipe: expected 2 commands, got %d", len(topo1.Segments[0].Commands))
	}

	topo2, err := BuildTopology("find . && rm")
	if err != nil {
		t.Fatalf("&& error: %v", err)
	}
	if len(topo2.Segments) != 2 {
		t.Fatalf("&&: expected 2 segments, got %d", len(topo2.Segments))
	}
}

func TestBuildTopologyComplexChain(t *testing.T) {
	// cmd1 | cmd2 && cmd3 ; cmd4 | cmd5
	topo, err := BuildTopology("cat foo | grep bar && echo done; ls | head")
	if err != nil {
		t.Fatalf("complex chain error: %v", err)
	}
	// Should be 3 segments: (cat|grep), (echo), (ls|head)
	if len(topo.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].type", topo.Segments[0].Type, SegmentTypePipeline)
	assertEqual(t, "seg[1].type", topo.Segments[1].Type, SegmentTypeSingle)
	assertEqual(t, "seg[2].type", topo.Segments[2].Type, SegmentTypePipeline)
}

func TestBuildTopologyVariableExpansion(t *testing.T) {
	topo, err := BuildTopology("$cmd foo")
	if err != nil {
		t.Fatalf("variable expansion error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func TestBuildTopologyCommandSubstitution(t *testing.T) {
	topo, err := BuildTopology("$(generate_cmd) foo")
	if err != nil {
		t.Fatalf("command substitution error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyQuoteRemoval pins shell-style quote removal at word
// extraction: names and args carry the value the shell would pass to the
// command, not the source text with quotes.
func TestBuildTopologyQuoteRemoval(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		wantName string
		wantArgs []string
	}{
		{"double-quoted arg", `curl -X "POST" https://example.com`, "curl", []string{"-X", "POST", "https://example.com"}},
		{"single-quoted arg", `curl -X 'POST' https://example.com`, "curl", []string{"-X", "POST", "https://example.com"}},
		{"partially quoted arg", `curl -X PO"ST" https://example.com`, "curl", []string{"-X", "POST", "https://example.com"}},
		{"quoted command name", `"rm" -rf /tmp/x`, "rm", []string{"-rf", "/tmp/x"}},
		{"quoted path with space", `rm -rf "/tmp/some dir"`, "rm", []string{"-rf", "/tmp/some dir"}},
		{"quoted dynamic keeps marker", `curl -X "$METHOD"`, "curl", []string{"-X", "$METHOD"}},
	}
	for _, tc := range cases {
		topo, err := BuildTopology(tc.cmd)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		cmd := topo.Segments[0].Commands[0]
		assertEqual(t, tc.name+" name", cmd.Name, tc.wantName)
		if len(cmd.Args) != len(tc.wantArgs) {
			t.Fatalf("%s: expected args %q, got %q", tc.name, tc.wantArgs, cmd.Args)
		}
		for i := range cmd.Args {
			assertEqual(t, tc.name+" arg", cmd.Args[i], tc.wantArgs[i])
		}
	}
}

// TestBuildTopologyDblQuotedCmdSubst verifies that a command substitution
// nested inside double quotes (`"$(cmd)"`) is detected as non-analyzable.
// Prior to the isAnalyzable full-depth Walk, the top-level Parts inspection
// only saw *syntax.DblQuoted and returned Analyzable=true, letting an attacker
// bypass the deny-first safety net by simply quoting a command substitution.
func TestBuildTopologyDblQuotedCmdSubst(t *testing.T) {
	topo, err := BuildTopology(`"$(cmd_that_resolves_to_rm)" -rf /`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyDblQuotedParamExp verifies that a plain parameter
// expansion nested inside double quotes (`"$VAR"`) is detected as
// non-analyzable.
func TestBuildTopologyDblQuotedParamExp(t *testing.T) {
	topo, err := BuildTopology(`"$CMD" -rf /`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyDblQuotedBracedParamExp verifies that a braced parameter
// expansion inside double quotes (`"${VAR}"`) is detected as non-analyzable.
func TestBuildTopologyDblQuotedBracedParamExp(t *testing.T) {
	topo, err := BuildTopology(`"${VAR}" -rf /`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyDblQuotedStatic ensures that fully static content inside
// double quotes (`"echo static"`) remains analyzable — the full-depth Walk
// must not false-positive on quoted literals.
func TestBuildTopologyDblQuotedStatic(t *testing.T) {
	topo, err := BuildTopology(`"echo static" -rf /`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, true)
}

// TestBuildTopologyDblQuotedArithmExp verifies that an arithmetic expansion
// nested inside double quotes (`"$((1+1))"`) is detected as non-analyzable.
func TestBuildTopologyDblQuotedArithmExp(t *testing.T) {
	topo, err := BuildTopology(`"$((1+1))" foo`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func assertEqual[T comparable](t *testing.T, name string, got, expected T) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}

// --- Plan 0025 Phase 1: literal `for` loop static expansion ---

// TestBuildTopologyForLoopLiteralWordIter verifies the base case: a fully
// static `for f in a.txt b.txt; do cat "$f"; done` expands into per-iteration
// segments with the loop variable substituted.
func TestBuildTopologyForLoopLiteralWordIter(t *testing.T) {
	topo, err := BuildTopology(`for f in a.txt b.txt; do cat "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 segments (one per iteration), got %d: %+v", len(topo.Segments), topo.Segments)
	}
	for _, seg := range topo.Segments {
		if seg.Type != SegmentTypeSingle {
			t.Errorf("expected single segment type, got %v", seg.Type)
		}
		if len(seg.Commands) != 1 || seg.Commands[0].Name != "cat" {
			t.Errorf("expected cmd 'cat', got %+v", seg.Commands)
		}
		if !seg.Commands[0].Analyzable {
			t.Errorf("expected analyzable=true after substitution, got false for %+v", seg.Commands[0])
		}
	}
	assertEqual(t, "seg[0].arg", topo.Segments[0].Commands[0].Args[0], "a.txt")
	assertEqual(t, "seg[1].arg", topo.Segments[1].Commands[0].Args[0], "b.txt")
}

// TestBuildTopologyForLoopBracedVar verifies that `${VAR}` (braced form) is
// substituted correctly, distinct from `$VAR`.
func TestBuildTopologyForLoopBracedVar(t *testing.T) {
	topo, err := BuildTopology(`for f in x y; do rm "${f}"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].name", topo.Segments[0].Commands[0].Name, "rm")
	assertEqual(t, "seg[0].arg", topo.Segments[0].Commands[0].Args[0], "x")
	assertEqual(t, "seg[1].arg", topo.Segments[1].Commands[0].Args[0], "y")
	assertEqual(t, "seg[0].analyzable", topo.Segments[0].Commands[0].Analyzable, true)
}

// TestBuildTopologyForLoopDynamicWordList verifies that a for-loop whose word
// list contains a command substitution stays non-analyzable — the previous
// deny-side behavior must not regress.
func TestBuildTopologyForLoopDynamicWordList(t *testing.T) {
	topo, err := BuildTopology(`for f in $(ls); do rm "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (control-flow), got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(control-flow)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyForLoopPartiallyDynamicWordList verifies that a for-loop
// whose word list mixes literals and dynamic elements stays non-analyzable.
func TestBuildTopologyForLoopPartiallyDynamicWordList(t *testing.T) {
	topo, err := BuildTopology(`for f in a "$X" b; do rm "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (control-flow), got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyForLoopPositionalParams verifies that `for f; do ...; done`
// (in clause omitted → iterates over $1 $2 ...) stays non-analyzable even
// though Items is an empty slice. This is the critical test called out in the
// Plan: without the InPos check, `len(Items) > 0` would be false but the
// "all items analyzable" check would vacuously pass an empty slice.
func TestBuildTopologyForLoopPositionalParams(t *testing.T) {
	topo, err := BuildTopology(`for f; do echo "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (control-flow), got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(control-flow)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyForLoopEmptyWordList verifies that `for f in; do ...; done`
// (explicit `in` with no items) falls back to the deny path rather than
// silently emit zero segments.
func TestBuildTopologyForLoopEmptyWordList(t *testing.T) {
	topo, err := BuildTopology(`for f in; do echo "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyForLoopIterationCap verifies the iteration cap: loops with
// more than maxForLoopIterations items fall back to the deny path.
func TestBuildTopologyForLoopIterationCap(t *testing.T) {
	// Build a for-loop with maxForLoopIterations+1 items.
	var items []string
	for i := 0; i <= maxForLoopIterations; i++ {
		items = append(items, "item")
	}
	cmd := "for f in " + strings.Join(items, " ") + "; do echo \"$f\"; done"
	topo, err := BuildTopology(cmd)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (over-cap → deny), got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)

	// At the cap exactly, expansion should still succeed.
	items = items[:maxForLoopIterations]
	cmd = "for f in " + strings.Join(items, " ") + "; do echo \"$f\"; done"
	topo, err = BuildTopology(cmd)
	if err != nil {
		t.Fatalf("at-cap BuildTopology error: %v", err)
	}
	if len(topo.Segments) != maxForLoopIterations {
		t.Fatalf("at-cap: expected %d segments, got %d", maxForLoopIterations, len(topo.Segments))
	}
}

// TestBuildTopologyForLoopBodyReassign verifies that a body reassigning the
// loop variable falls back to the deny path (safety: static expansion is only
// sound when the binding is stable across the whole body).
func TestBuildTopologyForLoopBodyReassign(t *testing.T) {
	topo, err := BuildTopology(`for f in a b; do f=/etc; rm "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (shadowed → deny), got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyForLoopInnerForShadow verifies that a nested for-loop
// re-iterating the same name is treated as shadowing.
func TestBuildTopologyForLoopInnerForShadow(t *testing.T) {
	topo, err := BuildTopology(`for f in a b; do for f in x y; do rm "$f"; done; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (inner shadow → deny), got %d", len(topo.Segments))
	}
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

// TestBuildTopologyForLoopMultiStmtBody verifies expansion of a body with
// multiple statements — each iteration should emit all body segments.
func TestBuildTopologyForLoopMultiStmtBody(t *testing.T) {
	topo, err := BuildTopology(`for f in a b; do echo "$f"; cat "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	// 2 iterations × 2 statements = 4 segments
	if len(topo.Segments) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].name", topo.Segments[0].Commands[0].Name, "echo")
	assertEqual(t, "seg[0].arg", topo.Segments[0].Commands[0].Args[0], "a")
	assertEqual(t, "seg[1].name", topo.Segments[1].Commands[0].Name, "cat")
	assertEqual(t, "seg[1].arg", topo.Segments[1].Commands[0].Args[0], "a")
	assertEqual(t, "seg[2].name", topo.Segments[2].Commands[0].Name, "echo")
	assertEqual(t, "seg[2].arg", topo.Segments[2].Commands[0].Args[0], "b")
	assertEqual(t, "seg[3].arg", topo.Segments[3].Commands[0].Args[0], "b")
}

// TestBuildTopologyForLoopUnquotedGlob verifies that an unquoted shell glob
// in the word list keeps the loop on the deny path. `for f in *.log` at
// runtime iterates over matching files, so treating it as a literal-N=1
// would silently rewrite semantics.
func TestBuildTopologyForLoopUnquotedGlob(t *testing.T) {
	cases := []string{
		`for f in *.log; do cat "$f"; done`,
		`for f in *; do rm "$f"; done`,
		`for f in ?.txt; do rm "$f"; done`,
		`for f in [ab]; do rm "$f"; done`,
	}
	for _, c := range cases {
		topo, err := BuildTopology(c)
		if err != nil {
			t.Fatalf("%q: BuildTopology error: %v", c, err)
		}
		if len(topo.Segments) != 1 {
			t.Fatalf("%q: expected 1 segment (glob → deny), got %d", c, len(topo.Segments))
		}
		if topo.Segments[0].Commands[0].Analyzable {
			t.Errorf("%q: expected analyzable=false (glob is runtime-dynamic)", c)
		}
	}
}

// TestBuildTopologyForLoopQuotedGlobIsLiteral verifies that a quoted glob
// (`"*.log"` / `'*.log'`) is treated as a literal — the shell does not
// perform pathname expansion inside quotes, so we can safely expand.
func TestBuildTopologyForLoopQuotedGlobIsLiteral(t *testing.T) {
	topo, err := BuildTopology(`for f in "*.log" 'other.log'; do rm "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 iteration segments (quoted → literal), got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].arg", topo.Segments[0].Commands[0].Args[0], "*.log")
	assertEqual(t, "seg[1].arg", topo.Segments[1].Commands[0].Args[0], "other.log")
}

// TestBuildTopologyForLoopBoundaryPrefix ensures that `$f` inside `$file` is
// not substituted (word boundary check). The command name here is `echo`
// (static, always analyzable in this codebase's semantics — Command.Analyzable
// tracks only the first-word dynamism), but the argument must retain its
// leading `$file` unchanged so that downstream args:/scope handling still
// sees the dynamic marker.
func TestBuildTopologyForLoopBoundaryPrefix(t *testing.T) {
	topo, err := BuildTopology(`for f in x y; do echo "$file"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 iteration segments, got %d", len(topo.Segments))
	}
	for i, seg := range topo.Segments {
		if seg.Commands[0].Name != "echo" {
			t.Errorf("seg[%d] expected name 'echo', got %q", i, seg.Commands[0].Name)
		}
		if len(seg.Commands[0].Args) < 1 || seg.Commands[0].Args[0] != "$file" {
			t.Errorf("seg[%d] boundary: expected arg '$file' preserved, got %v", i, seg.Commands[0].Args)
		}
	}
}

// TestBuildTopologyForLoopMixedVars ensures that the loop variable is
// substituted while other dynamic references stay intact. The command name
// `echo` is static so Analyzable=true either way — the invariant we're
// testing is that `$f` becomes the iteration value while `$OTHER` is
// preserved verbatim for downstream dynamic-args handling.
func TestBuildTopologyForLoopMixedVars(t *testing.T) {
	topo, err := BuildTopology(`for f in a b; do echo "$f" "$OTHER"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 iteration segments, got %d", len(topo.Segments))
	}
	assertEqual(t, "seg[0].args[0]", topo.Segments[0].Commands[0].Args[0], "a")
	assertEqual(t, "seg[0].args[1]", topo.Segments[0].Commands[0].Args[1], "$OTHER")
	assertEqual(t, "seg[1].args[0]", topo.Segments[1].Commands[0].Args[0], "b")
	assertEqual(t, "seg[1].args[1]", topo.Segments[1].Commands[0].Args[1], "$OTHER")
}

// TestBuildTopologyForLoopWithinPipeline verifies that a for-loop nested in a
// pipeline stays on the deny path — pipelines aren't affected by our
// expansion, only single-statement for-loops are.
// (`x | for f in a; do ...; done` — for is on the right of a pipe.)
func TestBuildTopologyForLoopWithinPipeline(t *testing.T) {
	// The pipeline path calls buildCommandFromStmt directly, which doesn't
	// know about expansion. We just verify it doesn't crash and stays safe.
	topo, err := BuildTopology(`echo hi | for f in a b; do echo "$f"; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment (pipeline), got %d", len(topo.Segments))
	}
	if len(topo.Segments[0].Commands) != 2 {
		t.Fatalf("expected 2 cmds in pipeline, got %d", len(topo.Segments[0].Commands))
	}
	// The for-side stays on the deny path — buildCommandFromStmt is only
	// called for the pipe children, and it does not expand.
	forCmd := topo.Segments[0].Commands[1]
	assertEqual(t, "analyzable", forCmd.Analyzable, false)
	assertEqual(t, "name", forCmd.Name, "(control-flow)")
}

// TestBuildTopologyForLoopWithRedirect verifies that a body containing a
// write redirect with the loop variable in the target expands correctly.
func TestBuildTopologyForLoopWithRedirect(t *testing.T) {
	topo, err := BuildTopology(`for f in a b; do echo hi > "$f".log; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(topo.Segments))
	}
	for i, seg := range topo.Segments {
		if len(seg.Commands) < 1 {
			t.Fatalf("seg[%d] has no commands", i)
		}
		cmd := seg.Commands[0]
		if !cmd.Analyzable {
			t.Errorf("seg[%d] should be analyzable after substitution, got false", i)
		}
		if len(cmd.Redirs) != 1 {
			t.Fatalf("seg[%d] expected 1 redir, got %d", i, len(cmd.Redirs))
		}
		if !cmd.Redirs[0].Analyzable {
			t.Errorf("seg[%d].redir should be analyzable after substitution, got false", i)
		}
	}
	// Redir paths should reflect substitution.
	assertEqual(t, "seg[0].redir", topo.Segments[0].Commands[0].Redirs[0].Path, "a.log")
	assertEqual(t, "seg[1].redir", topo.Segments[1].Commands[0].Redirs[0].Path, "b.log")
}

// --- Plan 0025 Phase 1: control-flow regression tests ---
// Prior to this Plan the codebase had zero tests pinning the "control-flow
// stays deny" behavior. Add regressions so that any future change that leaks
// a while/if/case/subshell/func-decl into the analyzable path is caught.

func TestBuildTopologyWhileClauseIsDeny(t *testing.T) {
	topo, err := BuildTopology(`while true; do rm x; done`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(control-flow)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func TestBuildTopologyIfClauseIsDeny(t *testing.T) {
	topo, err := BuildTopology(`if [ -f x ]; then rm x; fi`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(control-flow)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func TestBuildTopologyCaseClauseIsDeny(t *testing.T) {
	topo, err := BuildTopology(`case $X in a) rm a;; b) rm b;; esac`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(control-flow)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func TestBuildTopologyBlockIsDeny(t *testing.T) {
	topo, err := BuildTopology(`{ rm a; rm b; }`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(control-flow)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func TestBuildTopologySubshellIsDeny(t *testing.T) {
	topo, err := BuildTopology(`(rm a; rm b)`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(subshell)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}

func TestBuildTopologyFuncDeclIsDeny(t *testing.T) {
	topo, err := BuildTopology(`myfn() { rm x; }`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}
	assertEqual(t, "name", topo.Segments[0].Commands[0].Name, "(func-decl)")
	assertEqual(t, "analyzable", topo.Segments[0].Commands[0].Analyzable, false)
}
