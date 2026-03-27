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
	assertEqual(t, "nested.type", nestedSeg.Type, "pipeline")
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

func TestCurlPipeBash(t *testing.T) {
	topo, err := BuildTopology("curl https://example.com | bash")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	if len(topo.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(topo.Segments))
	}

	seg := topo.Segments[0]
	assertEqual(t, "type", seg.Type, "pipeline")
	assertEqual(t, "cmd[0]", seg.Commands[0].Name, "curl")
	assertEqual(t, "cmd[1]", seg.Commands[1].Name, "bash")
}
