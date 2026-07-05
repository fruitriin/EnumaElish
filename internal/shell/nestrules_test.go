package shell

import (
	"testing"
)

func TestFindExec(t *testing.T) {
	topo, err := BuildTopology(`find . -name "*.log" -exec rm -rf {} \;`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "find")

	if cmd.Nested == nil {
		t.Fatal("expected nested topology for find -exec")
	}

	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "rm")
}

func TestXargs(t *testing.T) {
	topo, err := BuildTopology("echo foo | xargs rm")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}

	// Pipeline: echo | xargs
	xargsCmd := topo.Segments[0].Commands[1]
	assertEqual(t, "cmd.name", xargsCmd.Name, "xargs")

	if xargsCmd.Nested == nil {
		t.Fatal("expected nested topology for xargs")
	}

	nestedCmd := xargsCmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "rm")
}

func TestXargsWithFlags(t *testing.T) {
	topo, err := BuildTopology("find . | xargs -I {} cp {} /tmp/")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	xargsCmd := topo.Segments[0].Commands[1]
	assertEqual(t, "cmd.name", xargsCmd.Name, "xargs")

	if xargsCmd.Nested == nil {
		t.Fatal("expected nested topology for xargs with flags")
	}

	nestedCmd := xargsCmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "cp")
}

func TestXargsWithNumericFlag(t *testing.T) {
	// Regression test for W-3: xargs -P 4 rm was incorrectly detecting "4" as the command
	topo, err := BuildTopology("find . | xargs -P 4 rm")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	xargsCmd := topo.Segments[0].Commands[1]
	assertEqual(t, "cmd.name", xargsCmd.Name, "xargs")

	if xargsCmd.Nested == nil {
		t.Fatal("expected nested topology for xargs -P 4")
	}

	nestedCmd := xargsCmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "rm")
}

func TestBashC(t *testing.T) {
	topo, err := BuildTopology(`bash -c "echo hello | grep h"`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "bash")

	if cmd.Nested == nil {
		t.Fatal("expected nested topology for bash -c")
	}

	// The nested command should be a pipeline: echo | grep
	if len(cmd.Nested.Segments) != 1 {
		t.Fatalf("expected 1 nested segment, got %d", len(cmd.Nested.Segments))
	}

	nestedSeg := cmd.Nested.Segments[0]
	assertEqual(t, "nested.type", nestedSeg.Type, SegmentTypePipeline)
	if len(nestedSeg.Commands) != 2 {
		t.Fatalf("expected 2 nested commands, got %d", len(nestedSeg.Commands))
	}
	assertEqual(t, "nested[0]", nestedSeg.Commands[0].Name, "echo")
	assertEqual(t, "nested[1]", nestedSeg.Commands[1].Name, "grep")
}

func TestEvalStatic(t *testing.T) {
	topo, err := BuildTopology(`eval "ls -la"`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "eval")

	if cmd.Nested == nil {
		t.Fatal("expected nested topology for eval")
	}

	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "ls")
}

func TestEvalDynamic(t *testing.T) {
	topo, err := BuildTopology(`eval "$dynamic_cmd"`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "eval")

	if cmd.Nested == nil {
		t.Fatal("expected nested topology for eval")
	}

	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "(dynamic-eval)")
	assertEqual(t, "nested.analyzable", nestedCmd.Analyzable, false)
}

// TestUnquoteWords covers the escape-aware quote resolution introduced for
// Plan 0006 S-6 (replacing the naive outer-quote stripping).
func TestUnquoteWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"no quotes", `echo hello`, `echo hello`, true},
		{"double quoted", `"echo hello"`, `echo hello`, true},
		{"single quoted", `'echo hello'`, `echo hello`, true},
		{"escaped double quote inside", `"echo \"hi there\""`, `echo "hi there"`, true},
		{"escaped backslash inside", `"a\\b"`, `a\b`, true},
		{"unquoted escaped quote", `echo \"hi\"`, `echo "hi"`, true},
		{"unquoted escaped space", `echo hi\ there`, `echo hi there`, true},
		{"single quote via concat", `'it'\''s'`, `it's`, true},
		{"mixed quote concat", `"a"'b'c`, `abc`, true},
		{"double quotes inside single", `'echo "a b"'`, `echo "a b"`, true},
		{"multiple static words", `rm '-rf' "/tmp/x y"`, `rm -rf /tmp/x y`, true},
		{"empty single quotes", `''`, ``, true},
		{"empty double quotes", `""`, ``, true},
		{"unclosed double quote", `"echo hi`, ``, false},
		{"unclosed single quote", `'echo hi`, ``, false},
		{"dynamic var in double quotes", `"echo $x"`, ``, false},
		{"dynamic cmdsubst in double quotes", `"echo $(id)"`, ``, false},
		{"dynamic backquote in double quotes", "\"echo `id`\"", ``, false},
		{"bare dynamic var", `$CMD "x"`, ``, false},
		{"redirect", `'ls' > /tmp/out`, ``, false},
		{"assignment prefix", `x='a b' ls`, ``, false},
		{"dollar single quote ansi-c", `$'a\tb'`, "a\tb", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unquoteWords(tt.in)
			assertEqual(t, "ok", ok, tt.ok)
			if tt.ok {
				assertEqual(t, "value", got, tt.want)
			}
		})
	}
}

// TestBashCEscapedQuotes verifies that escape sequences in the -c argument are
// resolved before re-parsing, so command boundaries are detected correctly.
func TestBashCEscapedQuotes(t *testing.T) {
	topo, err := BuildTopology(`bash -c "echo \"hi there\" && rm -rf /"`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "bash")
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for bash -c")
	}

	// echo "hi there" && rm -rf / — two segments split at &&
	if len(cmd.Nested.Segments) != 2 {
		t.Fatalf("expected 2 nested segments, got %d", len(cmd.Nested.Segments))
	}
	assertEqual(t, "nested[0].name", cmd.Nested.Segments[0].Commands[0].Name, "echo")
	rmCmd := cmd.Nested.Segments[1].Commands[0]
	assertEqual(t, "nested[1].name", rmCmd.Name, "rm")
	assertEqual(t, "nested[1].analyzable", rmCmd.Analyzable, true)
}

// TestBashCMixedQuotes verifies concatenated/mixed quoting on the -c argument.
func TestBashCMixedQuotes(t *testing.T) {
	topo, err := BuildTopology(`bash -c 'rm '"-rf"' /tmp/x'`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for bash -c")
	}
	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "rm")
	assertEqual(t, "nested.args[0]", nestedCmd.Args[0], "-rf")
}

// TestBashCDynamicInDoubleQuotes verifies that dynamic expansion inside a
// double-quoted -c argument is detected as unanalyzable instead of being
// partially resolved (previously it slipped through as a static command).
func TestBashCDynamicInDoubleQuotes(t *testing.T) {
	topo, err := BuildTopology(`bash -c "echo $x && rm -rf /"`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "bash")
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for bash -c")
	}
	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "(dynamic-shell)")
	assertEqual(t, "nested.analyzable", nestedCmd.Analyzable, false)
}

// TestBashCDynamicQuoteStripped verifies (dynamic-shell) detection when the
// -c argument arrives WITHOUT quotes — e.g. when upstream word extraction has
// already removed static quotes (integration with args-hardening). Detection
// must not depend on quote characters surviving in the argument string.
func TestBashCDynamicQuoteStripped(t *testing.T) {
	tests := []struct {
		name string
		arg  string
	}{
		{"param expansion", `echo $x && rm -rf /`},
		{"cmd substitution", `echo $(id)`},
		{"backquote substitution", "echo `id`"},
		{"arithmetic expansion", `echo $((1+1))`},
		{"dynamic deep in pipeline", `cat /etc/passwd | grep $user`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topo := parseBashC([]string{"-c", tt.arg})
			if topo == nil {
				t.Fatal("expected topology from parseBashC")
			}
			cmd := topo.Segments[0].Commands[0]
			assertEqual(t, "nested.name", cmd.Name, "(dynamic-shell)")
			assertEqual(t, "nested.analyzable", cmd.Analyzable, false)
		})
	}
}

// TestBashCStaticQuoteStripped verifies that quote-stripped STATIC arguments
// are still fully analyzed (no false (dynamic-shell) positives).
func TestBashCStaticQuoteStripped(t *testing.T) {
	topo := parseBashC([]string{"-c", `echo hello | grep h`})
	if topo == nil {
		t.Fatal("expected topology from parseBashC")
	}
	seg := topo.Segments[0]
	assertEqual(t, "type", seg.Type, SegmentTypePipeline)
	assertEqual(t, "cmd[0]", seg.Commands[0].Name, "echo")
	assertEqual(t, "cmd[1]", seg.Commands[1].Name, "grep")
}

// TestBashCEscapedDollarIsStatic verifies that an escaped dollar in the
// nested script (`bash -c 'echo \$x'` — single quotes preserve the
// backslash, so the inner script is `echo \$x` with a literal dollar) is not
// flagged as dynamic. The AST-based check parses `\$` as a literal; a
// plain-text "$" scan would false-positive here.
func TestBashCEscapedDollarIsStatic(t *testing.T) {
	topo, err := BuildTopology(`bash -c 'echo \$x'`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "bash")
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for bash -c")
	}
	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "echo")
	assertEqual(t, "nested.analyzable", nestedCmd.Analyzable, true)
}

// TestEvalDynamicQuoteStripped verifies (dynamic-eval) detection when eval's
// argument arrives without quotes (same integration scenario as
// TestBashCDynamicQuoteStripped).
func TestEvalDynamicQuoteStripped(t *testing.T) {
	topo := parseEval([]string{`echo $x`})
	if topo == nil {
		t.Fatal("expected topology from parseEval")
	}
	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "nested.name", cmd.Name, "(dynamic-eval)")
	assertEqual(t, "nested.analyzable", cmd.Analyzable, false)
}

// TestEvalMultipleQuotedArgs verifies eval's argument concatenation: quotes on
// each argument are resolved before the joined string is re-parsed. The old
// implementation corrupted `'a' b 'c'` into `a' b 'c`.
func TestEvalMultipleQuotedArgs(t *testing.T) {
	topo, err := BuildTopology(`eval rm '-rf' '/tmp/x'`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "cmd.name", cmd.Name, "eval")
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for eval")
	}
	nestedCmd := cmd.Nested.Segments[0].Commands[0]
	assertEqual(t, "nested.name", nestedCmd.Name, "rm")
	assertEqual(t, "nested.args[0]", nestedCmd.Args[0], "-rf")
	assertEqual(t, "nested.args[1]", nestedCmd.Args[1], "/tmp/x")
}

// TestEvalHiddenCommandSeparator verifies that a quoted `;` argument to eval
// is resolved so the second command is not missed. The old implementation
// treated `ls ';' rm` as a single ls invocation.
func TestEvalHiddenCommandSeparator(t *testing.T) {
	topo, err := BuildTopology(`eval ls ';' 'rm -rf /'`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for eval")
	}
	if len(cmd.Nested.Segments) != 2 {
		t.Fatalf("expected 2 nested segments, got %d", len(cmd.Nested.Segments))
	}
	assertEqual(t, "nested[0].name", cmd.Nested.Segments[0].Commands[0].Name, "ls")
	assertEqual(t, "nested[1].name", cmd.Nested.Segments[1].Commands[0].Name, "rm")
}

func TestCurlPipeBash(t *testing.T) {
	topo, err := BuildTopology("curl https://example.com | bash")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}

	seg := topo.Segments[0]
	assertEqual(t, "type", seg.Type, SegmentTypePipeline)
	assertEqual(t, "cmd[0]", seg.Commands[0].Name, "curl")
	assertEqual(t, "cmd[1]", seg.Commands[1].Name, "bash")
}
