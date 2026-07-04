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

// TestClassifyPathSymlinkEscape verifies that a symlink inside the workspace
// pointing outside the workspace resolves to ScopeOutside (Dashboard 気になった点 #4).
func TestClassifyPathSymlinkEscape(t *testing.T) {
	// Create a temporary workspace with a symlink that escapes.
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	target := filepath.Join(tmp, "outside-target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	linkInside := filepath.Join(ws, "link")
	if err := os.Symlink(target, linkInside); err != nil {
		t.Skipf("symlink not supported on this platform: %v", err)
	}

	// The link path lives literally inside workspace but points outside.
	result := ClassifyPath(linkInside, []string{ws})
	if result != ScopeOutside {
		t.Errorf("[SECURITY] symlink escaping workspace must be outside, got inside for %q", linkInside)
	}

	// A subpath under the link — must also be outside.
	subpath := filepath.Join(linkInside, "passwd")
	result2 := ClassifyPath(subpath, []string{ws})
	if result2 != ScopeOutside {
		t.Errorf("[SECURITY] path under escaping symlink must be outside, got inside for %q", subpath)
	}
}

// TestClassifyPathSymlinkInside verifies that a symlink whose target is inside
// the workspace remains ScopeInside.
func TestClassifyPathSymlinkInside(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	subdir := filepath.Join(ws, "project")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(ws, "shortcut")
	if err := os.Symlink(subdir, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	result := ClassifyPath(link, []string{ws})
	if result != ScopeInside {
		t.Errorf("symlink pointing inside workspace must be inside, got outside for %q", link)
	}
}

// TestClassifyPathNonExistent verifies that a not-yet-created file inside the
// workspace is still classified as ScopeInside via parent resolution.
func TestClassifyPathNonExistent(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	newFile := filepath.Join(ws, "not-yet-created.go")

	result := ClassifyPath(newFile, []string{ws})
	if result != ScopeInside {
		t.Errorf("non-existent path inside workspace must be inside, got outside for %q", newFile)
	}
}

// TestClassifyPathWorkspaceIsSymlink verifies that when the workspace root
// itself is a symlink, paths under it are still resolved consistently.
func TestClassifyPathWorkspaceIsSymlink(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real-workspace")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(tmp, "linked-workspace")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	subfile := filepath.Join(link, "file.go")
	// File does not need to exist; parent (link) resolves to real.

	// Configure workspace via the symlink; access via symlink → inside.
	if got := ClassifyPath(subfile, []string{link}); got != ScopeInside {
		t.Errorf("path under linked workspace should be inside, got %v", got)
	}
	// Configure workspace via the real path; access via symlink → still inside
	// (because both resolve to the same target).
	if got := ClassifyPath(subfile, []string{real}); got != ScopeInside {
		t.Errorf("path under linked workspace with real-path config should be inside, got %v", got)
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
