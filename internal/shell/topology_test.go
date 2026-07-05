package shell

import (
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
