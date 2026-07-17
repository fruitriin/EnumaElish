package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/eval"
)

// toolInput represents the JSON input from Claude Code's hook system.
type toolInput struct {
	ToolName       string          `json:"tool_name"`
	Input          json.RawMessage `json:"tool_input"`
	PermissionMode string          `json:"permission_mode"`
	SessionID      string          `json:"session_id"`
	CWD            string          `json:"cwd"`
}

// hookResponse is the PreToolUse hook JSON output (exit 0 + stdout).
// https://code.claude.com/docs/en/hooks — hookSpecificOutput schema.
type hookResponse struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

func newHookResponse(decision, reason string) *hookResponse {
	return &hookResponse{hookSpecificOutput{
		HookEventName:            "PreToolUse",
		PermissionDecision:       decision,
		PermissionDecisionReason: reason,
	}}
}

// bashInput represents the input for a Bash tool call.
type bashInput struct {
	Command string `json:"command"`
}

// fileToolInput represents the input for Read/Edit/Write tool calls.
type fileToolInput struct {
	FilePath string `json:"file_path"`
}

// webFetchInput represents the input for WebFetch tool calls.
type webFetchInput struct {
	URL string `json:"url"`
}

const maxStdinBytes = 1 << 20 // 1MB

func runHookPre(configPath string, defaultAction string) {
	input, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}
	if int64(len(input)) >= maxStdinBytes {
		fmt.Fprintln(os.Stderr, "ccchain: stdin input exceeds 1MB limit (allowing)")
		os.Exit(0)
	}

	var ti toolInput
	if err := json.Unmarshal(input, &ti); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: invalid hook input JSON (allowing): %v\n", err)
		os.Exit(0)
	}

	cfg, err := dsl.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccchain config error: %v\n", err)
		if isStrictConfigError(cfg, err) {
			reason := fmt.Sprintf("ccchain: config load failed and strict_config_error is enabled: %v", err)
			emitResponse(newHookResponse("deny", reason))
		}
		os.Exit(0)
	}

	if defaultAction != "" {
		if cfg.Settings == nil {
			cfg.Settings = dsl.DefaultSettings()
		}
		cfg.Settings.Fallback = dsl.Action(defaultAction)
	}

	var result *eval.Result

	switch {
	case ti.ToolName == "Bash":
		var bi bashInput
		if err := json.Unmarshal(ti.Input, &bi); err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: invalid Bash input JSON (allowing): %v\n", err)
			os.Exit(0)
		}
		if bi.Command == "" {
			os.Exit(0)
		}
		r, err := eval.Evaluate(bi.Command, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: parse warning (allowing): %v\n", err)
			os.Exit(0)
		}
		result = r

	case ti.ToolName == "Read" || ti.ToolName == "Edit" || ti.ToolName == "Write":
		var fi fileToolInput
		if err := json.Unmarshal(ti.Input, &fi); err != nil {
			os.Exit(0)
		}
		result = eval.EvaluateTool(ti.ToolName, fi.FilePath, cfg)

	case ti.ToolName == "WebFetch":
		var wf webFetchInput
		if err := json.Unmarshal(ti.Input, &wf); err != nil {
			os.Exit(0)
		}
		result = eval.EvaluateTool(ti.ToolName, wf.URL, cfg)

	case strings.HasPrefix(ti.ToolName, "mcp__"):
		// Best-effort: try to extract file_path, path, or url from MCP input
		mcpArg := extractMCPArg(ti.Input)
		result = eval.EvaluateTool(ti.ToolName, mcpArg, cfg)

	default:
		// Unknown tool — pass through
		os.Exit(0)
	}

	// Resolve ask against the runtime permission mode (Plan 0022 Phase 2):
	// in modes where a dialog cannot reach a human, ask degrades to deny+hint
	// (default) or warn per rule `unattended:` / settings.
	result = eval.ResolveAsk(result, ti.PermissionMode, cfg.Settings)

	emitResponse(buildHookResponse(result))
}

// buildHookResponse maps an evaluation result to the hook JSON response.
// A nil return means the hook stays neutral (exit 0, no output): Claude Code
// falls through to its own permission flow, same as before ccchain existed.
//
// allow is intentionally neutral rather than permissionDecision:"allow" —
// ccchain's allow means "ccchain does not object", not "skip the user's
// remaining permission layers". deny/ask/warn/hint carry an opinion, so they
// emit JSON.
func buildHookResponse(result *eval.Result) *hookResponse {
	switch result.Action {
	case dsl.ActionDeny:
		msg := result.Message
		if msg == "" {
			msg = "blocked by ccchain"
		}
		return newHookResponse("deny", msg)

	case dsl.ActionWarn, dsl.ActionHint:
		// Let the call through but land the caution text in Claude's context.
		return newHookResponse("allow", result.Message)

	case dsl.ActionAsk:
		return newHookResponse("ask", result.Message)

	default:
		return nil
	}
}

// emitResponse writes the hook JSON to stdout and exits 0. Per the hooks
// spec, JSON output requires exit 0 (Claude Code ignores JSON on exit 2).
func emitResponse(resp *hookResponse) {
	if resp != nil {
		if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "ccchain: json encode error: %v\n", err)
		}
	}
	os.Exit(0)
}

// extractMCPArg attempts to extract a file path or URL from MCP tool input.
func extractMCPArg(input json.RawMessage) string {
	var generic map[string]json.RawMessage
	if json.Unmarshal(input, &generic) != nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "url", "filePath"} {
		if v, ok := generic[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && s != "" {
				return s
			}
		}
	}
	return ""
}

// isStrictConfigError reports whether strict-config-error mode is active,
// causing config load failures to deny (fail-closed) instead of allow (fail-open).
//
// Precedence:
//  1. The partial Config's Settings.StrictConfigError (any file that loaded
//     successfully before the failure — e.g. ~/.claude/ccchain.conf when the
//     project-local file has a parse error) — always wins.
//  2. If loadErr is nil (config loaded cleanly), env is ignored — a successful
//     load without strict_config_error means the user has taken a stance.
//  3. If loadErr is non-nil AND the partial cfg did not explicitly set
//     strict_config_error, the CCCHAIN_STRICT_CONFIG_ERROR env var ("1" or
//     "true") acts as opt-in. This is the only path when no config file could
//     be read at all. If the partial cfg explicitly set strict_config_error=false,
//     env is ignored (user opted out).
func isStrictConfigError(cfg *dsl.Config, loadErr error) bool {
	if cfg != nil && cfg.Settings != nil && cfg.Settings.StrictConfigError {
		return true
	}
	if loadErr == nil {
		return false
	}
	if cfg != nil && cfg.Settings != nil && cfg.Settings.Explicit["strict_config_error"] {
		// User explicitly set strict_config_error=false; honor that.
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CCCHAIN_STRICT_CONFIG_ERROR"))) {
	case "1", "true":
		return true
	}
	return false
}

func runHookPost(configPath string) {
	// PostToolUse hook — currently a pass-through
	// Future: hint actions, turn counting
	os.Exit(0)
}
