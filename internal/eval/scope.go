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

	// Absolute path → check against workspace paths
	if filepath.IsAbs(expanded) {
		for _, ws := range workspacePaths {
			wsExpanded := filepath.Clean(expandTilde(ws))
			// Trailing slash comparison to prevent ~/workspace2 matching ~/workspace
			if expanded == wsExpanded || strings.HasPrefix(expanded+"/", wsExpanded+"/") {
				return ScopeInside
			}
		}
		return ScopeOutside
	}

	// Tilde path that didn't resolve to absolute (shouldn't happen after expandTilde)
	if strings.HasPrefix(path, "~/") {
		return ScopeOutside
	}

	// Pure relative path (no .., no /) → assume inside (CWD is typically workspace)
	return ScopeInside
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
