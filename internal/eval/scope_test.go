package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

func TestClassifyPathInside(t *testing.T) {
	home, _ := os.UserHomeDir()
	ws := []string{"~/workspace"}

	tests := []struct {
		name string
		path string
	}{
		{"workspace root", "~/workspace"},
		{"workspace subdir", "~/workspace/project/file.go"},
		{"relative no slash", "file.txt"},
		{"relative with dir", "src/main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyPath(tt.path, ws)
			if result != ScopeInside {
				t.Errorf("expected inside for %q (home=%s), got outside", tt.path, home)
			}
		})
	}
}

func TestClassifyPathOutside(t *testing.T) {
	ws := []string{"~/workspace"}

	tests := []struct {
		name string
		path string
	}{
		{"home dir", "~/"},
		{"ssh dir", "~/.ssh/id_rsa"},
		{"etc", "/etc/passwd"},
		{"other home dir", "~/Documents/secret.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyPath(tt.path, ws)
			if result != ScopeOutside {
				t.Errorf("expected outside for %q, got inside", tt.path)
			}
		})
	}
}

func TestClassifyPathTraversal(t *testing.T) {
	ws := []string{"~/workspace"}

	// [Critical] Path traversal must be ScopeOutside
	tests := []struct {
		name string
		path string
	}{
		{"parent traversal", "../../etc/passwd"},
		{"workspace escape", "~/workspace/../.ssh/id_rsa"},
		{"dot-dot in middle", "some/../../other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyPath(tt.path, ws)
			if result != ScopeOutside {
				t.Errorf("[SECURITY] path traversal %q must be outside, got inside", tt.path)
			}
		})
	}
}

func TestClassifyPathTildeBypass(t *testing.T) {
	ws := []string{"~/workspace"}

	// [High] ~/workspace2 must NOT match ~/workspace
	tests := []struct {
		name   string
		path   string
		expect ScopeResult
	}{
		{"workspace2 no match", "~/workspace2/file", ScopeOutside},
		{"workspace-other", "~/workspace-other/file", ScopeOutside},
		{"workspaces", "~/workspaces/file", ScopeOutside},
		{"workspace exact", "~/workspace/file", ScopeInside},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyPath(tt.path, ws)
			if result != tt.expect {
				t.Errorf("%q: expected %v, got %v", tt.path, tt.expect, result)
			}
		})
	}
}

func TestClassifyPathMultipleWorkspaces(t *testing.T) {
	ws := []string{"~/workspace", "~/projects"}

	tests := []struct {
		name   string
		path   string
		expect ScopeResult
	}{
		{"in workspace", "~/workspace/a.go", ScopeInside},
		{"in projects", "~/projects/b.go", ScopeInside},
		{"in neither", "~/Documents/c.txt", ScopeOutside},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyPath(tt.path, ws)
			if result != tt.expect {
				t.Errorf("%q: expected %v, got %v", tt.path, tt.expect, result)
			}
		})
	}
}

func TestClassifyPathNoScope(t *testing.T) {
	// No workspace configured → everything is inside
	result := ClassifyPath("/etc/passwd", nil)
	if result != ScopeInside {
		t.Error("no workspace configured should return inside")
	}
}

func TestScopeWithToolEvaluation(t *testing.T) {
	home, _ := os.UserHomeDir()
	wsPath := filepath.Join(home, "workspace")

	cfg := mustParseConfig(t, `
settings:
  workspace: ~/workspace
  fallback: ask

preToolUse
  allow Read
`)
	// Inside workspace → allow
	r1 := EvaluateTool("Read", wsPath+"/README.md", cfg)
	assertEqual(t, "inside workspace", r1.Action, dsl.ActionAllow)

	// Outside workspace → escalated to ask
	r2 := EvaluateTool("Read", home+"/.ssh/id_rsa", cfg)
	assertEqual(t, "outside workspace", r2.Action, dsl.ActionAsk)
}
