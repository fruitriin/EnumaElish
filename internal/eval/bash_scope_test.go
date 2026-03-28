package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

func TestBashScopeInside(t *testing.T) {
	home, _ := os.UserHomeDir()
	wsPath := filepath.Join(home, "workspace")

	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace
  fallback: ask

preToolUse
  allow cat
`)
	// Inside workspace → allow
	r, err := Evaluate("cat "+wsPath+"/README.md", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "inside workspace", r.Action, dsl.ActionAllow)
}

func TestBashScopeOutside(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace
  fallback: ask

preToolUse
  allow cat
`)
	// Outside workspace → escalated to ask
	r, err := Evaluate("cat /etc/passwd", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "outside workspace", r.Action, dsl.ActionAsk)
}

func TestBashScopeTraversal(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace
  fallback: ask

preToolUse
  allow cat
`)
	// Path traversal → outside
	r, err := Evaluate("cat ../../.ssh/id_rsa", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "traversal", r.Action, dsl.ActionAsk)
}

func TestBashScopeMultipleWorkspaces(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace, /tmp/hogehoge
  fallback: ask

preToolUse
  allow cat
`)
	// /tmp/hogehoge is inside
	r1, err := Evaluate("cat /tmp/hogehoge/data.txt", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "tmp hogehoge inside", r1.Action, dsl.ActionAllow)

	// /tmp/other is outside
	r2, err := Evaluate("cat /tmp/other/file", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "tmp other outside", r2.Action, dsl.ActionAsk)
}

func TestBashScopeNoPathArgs(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace
  fallback: ask

preToolUse
  allow echo
`)
	// No path args → allow (no scope check)
	r, err := Evaluate("echo hello world", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "no path args", r.Action, dsl.ActionAllow)
}

func TestBashScopeNoWorkspaceConfig(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  fallback: ask

preToolUse
  allow cat
`)
	// No workspace configured → no scope check → allow
	r, err := Evaluate("cat /etc/passwd", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "no workspace", r.Action, dsl.ActionAllow)
}

func TestBashScopeDenyNotEscalated(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace
  fallback: ask

preToolUse
  deny rm
`)
	// deny is not escalated (already more restrictive than ask)
	r, err := Evaluate("rm /etc/passwd", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "deny not escalated", r.Action, dsl.ActionDeny)
}
