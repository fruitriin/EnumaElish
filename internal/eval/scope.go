package eval

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fruitriin/EnumaElish/internal/dsl"
)

// ScopeResult represents whether a path is inside or outside the workspace.
type ScopeResult int

const (
	ScopeInside ScopeResult = iota
	ScopeOutside
)

// ClassifyPath determines if a path is inside or outside the workspace.
// Security measures (from security review):
//   - Paths containing ".." are forced to ScopeOutside (path traversal protection)
//   - filepath.Clean normalizes paths before prefix comparison
//   - Trailing slash comparison prevents ~/workspace2 matching ~/workspace
//   - Relative paths without ".." are treated as inside (CWD is typically workspace)
//   - Symbolic links are resolved via filepath.EvalSymlinks before comparison
//     (Dashboard 気になった点 #4). If a workspace-relative link points outside the
//     workspace (e.g. ~/workspace/link → /etc), the target is ScopeOutside.
//     For non-existent paths (e.g. new files to be created), the longest existing
//     ancestor is resolved and the remaining suffix is appended.
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
// Returns (resolved, true) when some ancestor (possibly the filesystem root)
// resolves. Returns ("", false) only when even the root cannot be resolved
// (circular symlink / ELOOP, or permission errors on every level). Callers
// treat (_, false) as ScopeOutside per the fail-closed policy documented in
// ClassifyPath — this is what makes `outside-write: deny` an actual sentinel
// instead of best-effort.
//
// Rationale for walking upward on any error (not only ENOENT): a permission
// error on the exact path does not tell us whether an ancestor is inside the
// workspace, so we try the ancestor. Only after exhausting every ancestor
// (including the root) do we give up and fail closed.
func resolveSymlinks(path string) (string, bool) {
	cleaned := filepath.Clean(path)

	// Fast path: the exact path exists — resolve it.
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved, true
	}

	// Walk up ancestors until one resolves. Give up (fail closed) when we
	// have tried the root and it still fails.
	suffix := ""
	dir := cleaned
	for {
		parent := filepath.Dir(dir)
		if suffix == "" {
			suffix = filepath.Base(dir)
		} else {
			suffix = filepath.Join(filepath.Base(dir), suffix)
		}
		if parent == dir {
			// Reached the filesystem root. Try it once; if even the root
			// fails to resolve, give up entirely.
			if resolved, err := filepath.EvalSymlinks(dir); err == nil {
				return filepath.Join(resolved, suffix), true
			}
			return "", false
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

// scopeViolationAction returns the action configured for workspace scope
// violations (settings: scope_violation). Defaults to ask for backward
// compatibility when unset.
func scopeViolationAction(config *dsl.Config) dsl.Action {
	if config.Settings != nil && config.Settings.ScopeViolation != "" {
		return config.Settings.ScopeViolation
	}
	return dsl.ActionAsk
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

// ExtractPathArgs extracts arguments that look like file paths from a command's
// args. Common CLI flag-glue forms (--key=/path, -k=/path, -k/path for
// well-known single-letter flags) are stripped so the DSL and semantics table
// see the actual path value — otherwise `cp --target-directory=/outside` etc
// slip past scope classification (Critical C2).
func ExtractPathArgs(args []string) []string {
	var paths []string
	for _, arg := range args {
		p := stripFlagPrefix(arg)
		if looksLikePath(p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// stripFlagPrefix normalizes a token that combines a flag and a path into
// just the path portion.
//
// Handles:
//
//	--key=/path → /path
//	-k=/path    → /path
//	-k/path     → /path  (short flag glued to a path-like value)
//
// Tokens with no flag prefix are returned unchanged. Bare flags like "-v" or
// "--verbose" are returned unchanged and then rejected by looksLikePath.
func stripFlagPrefix(arg string) string {
	if !strings.HasPrefix(arg, "-") || arg == "-" || arg == "--" {
		return arg
	}
	if eq := strings.Index(arg, "="); eq > 0 {
		// --key=value or -k=value
		return arg[eq+1:]
	}
	// -k/path (short flag glued to path). We only strip when the byte after
	// the flag letter unambiguously looks like a path start; otherwise a
	// cluster like "-tv" would be corrupted.
	if !strings.HasPrefix(arg, "--") && len(arg) > 2 {
		rest := arg[2:]
		if strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, "./") ||
			strings.HasPrefix(rest, "../") || strings.HasPrefix(rest, "~/") {
			return rest
		}
	}
	return arg
}

func looksLikePath(arg string) bool {
	return strings.HasPrefix(arg, "/") ||
		strings.HasPrefix(arg, "~/") ||
		strings.HasPrefix(arg, "./") ||
		strings.HasPrefix(arg, "../") ||
		strings.Contains(arg, "/")
}
