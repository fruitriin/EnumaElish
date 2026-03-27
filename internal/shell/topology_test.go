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

func assertEqual[T comparable](t *testing.T, name string, got, expected T) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}
