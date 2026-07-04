package eval

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
)

// ScopeResult represents whether a path is inside or outside the workspace.
type ScopeResult int

const (
	ScopeInside  ScopeResult = iota
	ScopeOutside
)

// ClassifyPath determines if a path is inside or outside the workspace.
// Security measures (from security review):
// - Paths containing ".." are forced to ScopeOutside (path traversal protection)
// - filepath.Clean normalizes paths before prefix comparison
// - Trailing slash comparison prevents ~/workspace2 matching ~/workspace
// - Relative paths without ".." are treated as inside (CWD is typically workspace)
// - Symbolic links are resolved via filepath.EvalSymlinks before comparison
//   (Dashboard 気になった点 #4). If a workspace-relative link points outside the
//   workspace (e.g. ~/workspace/link → /etc), the target is ScopeOutside.
//   For non-existent paths (e.g. new files to be created), the longest existing
//   ancestor is resolved and the remaining suffix is appended.
func ClassifyPath(path string, workspacePaths []string) ScopeResult {
	if len(workspacePaths) == 0 {
		return ScopeInside // no scope configured → everything is inside
	}

	// Path traversal: any path containing ".." is forced outside
	if strings.Contains(path, "..") {
		return ScopeOutside
	}

	expanded := expandTilde(path)
	expanded = filepath.Clean(expanded)

	// Absolute path → check against workspace paths (with symlink resolution)
	if filepath.IsAbs(expanded) {
		resolved, ok := resolveSymlinks(expanded)
		if !ok {
			// Resolution failed entirely (e.g. permission error on all ancestors)
			// → safe side: outside
			return ScopeOutside
		}
		for _, ws := range workspacePaths {
			wsExpanded := filepath.Clean(expandTilde(ws))
			wsResolved, wsOk := resolveSymlinks(wsExpanded)
			if !wsOk {
				// If we cannot resolve the workspace root itself, fall back to
				// the non-resolved path for this comparison (best-effort).
				wsResolved = wsExpanded
			}
			// Trailing slash comparison to prevent ~/workspace2 matching ~/workspace
			if resolved == wsResolved || strings.HasPrefix(resolved+"/", wsResolved+"/") {
				return ScopeInside
			}
		}
		return ScopeOutside
	}

	// Tilde path that didn't resolve to absolute (shouldn't happen after expandTilde)
	if strings.HasPrefix(path, "~/") {
		return ScopeOutside
	}

	// Pure relative path — resolve against CLAUDE_PROJECT_DIR if available
	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		abs := filepath.Clean(filepath.Join(projectDir, path))
		absResolved, ok := resolveSymlinks(abs)
		if !ok {
			return ScopeOutside
		}
		for _, ws := range workspacePaths {
			wsExpanded := filepath.Clean(expandTilde(ws))
			wsResolved, wsOk := resolveSymlinks(wsExpanded)
			if !wsOk {
				wsResolved = wsExpanded
			}
			if absResolved == wsResolved || strings.HasPrefix(absResolved+"/", wsResolved+"/") {
				return ScopeInside
			}
		}
		return ScopeOutside
	}

	// No CLAUDE_PROJECT_DIR — assume inside (CWD is typically workspace)
	return ScopeInside
}

// resolveSymlinks resolves symbolic links on the longest existing ancestor of
// path, then appends the remaining (non-existent) suffix. This handles the
// common case of a not-yet-created file inside an existing directory tree.
//
// Returns (resolved, true) on success. Returns ("", false) only when the path
// has no existing ancestor that can be resolved (i.e. no meaningful comparison
// is possible) — callers should treat that as ScopeOutside per fail-closed policy.
//
// os.IsNotExist errors are expected (new-file paths) and cause upward walking.
// Any other error (e.g. permission) also causes upward walking; if the root is
// reached without resolving, we return the cleaned input as best-effort — the
// caller then compares against workspace roots as-is.
func resolveSymlinks(path string) (string, bool) {
	cleaned := filepath.Clean(path)

	// Fast path: the exact path exists — resolve it.
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved, true
	}

	// Walk up ancestors until one resolves.
	suffix := ""
	dir := cleaned
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding an existing ancestor.
			// Best-effort: use the cleaned original path so at least prefix
			// comparison against workspace roots is possible.
			return cleaned, true
		}
		if suffix == "" {
			suffix = filepath.Base(dir)
		} else {
			suffix = filepath.Join(filepath.Base(dir), suffix)
		}
		dir = parent
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, suffix), true
		}
	}
}

// EvaluatePathScope checks a file path against workspace scope and returns
// the most restrictive result if the path is outside the workspace.
func EvaluatePathScope(filePath string, config *dsl.Config) *ScopeResult {
	if config.Settings == nil || len(config.Settings.WorkspacePaths) == 0 {
		return nil // no scope configured
	}

	result := ClassifyPath(filePath, config.Settings.WorkspacePaths)
	return &result
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

// ExtractPathArgs extracts arguments that look like file paths from a command's args.
func ExtractPathArgs(args []string) []string {
	var paths []string
	for _, arg := range args {
		if looksLikePath(arg) {
			paths = append(paths, arg)
		}
	}
	return paths
}

func looksLikePath(arg string) bool {
	return strings.HasPrefix(arg, "/") ||
		strings.HasPrefix(arg, "~/") ||
		strings.HasPrefix(arg, "./") ||
		strings.HasPrefix(arg, "../") ||
		strings.Contains(arg, "/")
}
