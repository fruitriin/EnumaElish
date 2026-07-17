package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fruitriin/EnumaElish/internal/approve"
	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/eval"
	"github.com/fruitriin/EnumaElish/internal/evallog"
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
	// bashCommand carries the raw Bash command through the evaluate → resolve
	// flow so that the approval-token bookkeeping (Phase 3) can be run against
	// exactly what the agent submitted, not a partially-normalized string.
	var bashCommand string

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
		bashCommand = bi.Command
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

	// Phase 3 approval-token: for Bash commands whose ask degrades to deny,
	// consult the approve store first. A live matching approval turns the
	// result into an allow-with-warn (one-shot consumed).
	//
	// Skeptic C1: the gate is "preAskAction == ask AND ResolveAsk turned it
	// into deny", NOT "mode is non-interactive". `ask_strategy: deny-all`
	// degrades ask to deny in every mode (including interactive), and the
	// deny message promises `ccchain approve --last` as the human path — so
	// the gate must fire whenever that promise appears in the outgoing
	// message, i.e. whenever ResolveAsk actually converted ask → deny.
	preAskAction := result.Action

	// Resolve ask against the runtime permission mode (Plan 0022 Phase 2):
	// in modes where a dialog cannot reach a human, ask degrades to deny+hint
	// (default) or warn per rule `unattended:` / settings. `deny-all` strategy
	// degrades regardless of mode.
	result = eval.ResolveAsk(result, ti.PermissionMode, cfg.Settings)

	if bashCommand != "" && preAskAction == dsl.ActionAsk && result.Action == dsl.ActionDeny {
		if consumed, entry := lookupApproval(bashCommand, ti.CWD, ti.SessionID); consumed {
			approved := approvalAllowResult(entry)
			// Plan 0029: record the approved-allow decision so the operator's
			// `ccchain stats` sees consumption alongside allow/deny counts.
			writeEvalLog(cfg, &ti, bashCommand, approved)
			emitResponse(buildHookResponse(approved))
			return
		}
	}

	// If the ask degraded to deny (i.e. a block that a human could still
	// approve), record the request so `ccchain approve --last` has something
	// to grant. Interactive ask that did NOT degrade stays ask — no pending
	// entry.
	if bashCommand != "" && preAskAction == dsl.ActionAsk && result.Action == dsl.ActionDeny {
		recordPendingApproval(bashCommand, ti.CWD, ti.SessionID, result)
	}

	// Plan 0029: persist the evaluation outcome for later `ccchain stats`.
	// Errors are logged to stderr only; the log is best-effort and must
	// never change the allow/deny decision.
	writeEvalLog(cfg, &ti, bashCommand, result)
	emitResponse(buildHookResponse(result))
}

// writeEvalLog appends the evaluation result to the configured JSONL log.
// No-op when settings.log is not set. Errors are surfaced to stderr but
// swallowed so the hook decision is never affected (Plan 0029: fail-open).
//
// Relative log paths are resolved against ti.CWD (the hook's cwd), matching
// how `.ccchain.conf` itself is discovered. Absolute paths are used as-is.
func writeEvalLog(cfg *dsl.Config, ti *toolInput, bashCommand string, result *eval.Result) {
	if cfg == nil || cfg.Settings == nil || cfg.Settings.LogPath == "" {
		return
	}
	if result == nil {
		return
	}
	logPath := cfg.Settings.LogPath
	if !filepath.IsAbs(logPath) {
		base := ti.CWD
		if base == "" {
			// Fall back to process cwd — the hook may lack a cwd field on old
			// Claude Code versions. This mirrors what `.ccchain.conf` search
			// does implicitly.
			if wd, err := os.Getwd(); err == nil {
				base = wd
			}
		}
		if base != "" {
			logPath = filepath.Join(base, logPath)
		}
	}

	// Prefer the raw bash command (when available) so operators can grep the
	// log for the exact string they saw at the prompt. Non-Bash tools log
	// the tool argument via ExtractedTarget wrapped in Result.Message if we
	// have one, otherwise empty.
	command := bashCommand

	entry := evallog.Entry{
		ToolName:       ti.ToolName,
		Command:        command,
		Action:         string(result.Action),
		MatchedRule:    result.MatchedRule,
		Message:        result.Message,
		PermissionMode: ti.PermissionMode,
		SessionID:      ti.SessionID,
		CWD:            ti.CWD,
	}
	if err := evallog.Log(logPath, entry); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: eval log write failed (allowing): %v\n", err)
	}
}

// lookupApproval consults the store for a live, matching approval and
// consumes it. Store failures are logged and treated as "no approval" — the
// approve store is best-effort; a broken store must never open a hole in the
// deny path (the caller falls through to normal degradation).
func lookupApproval(command, cwd, sessionID string) (bool, *approve.ApprovedEntry) {
	dir, err := approve.DefaultDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: approve store dir error (skipping): %v\n", err)
		return false, nil
	}
	store := approve.NewStore(dir)
	ok, entry, err := store.CheckApproved(command, cwd, sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: approve check error (skipping): %v\n", err)
		return false, nil
	}
	return ok, entry
}

// recordPendingApproval appends a pending entry for a downgrade-deny. Any
// error is logged; the deny itself has already been decided so a store
// failure must not change the outcome for the agent.
//
// When the command is not eligible for approval (dynamic expansion), the
// deny message is amended to explain why the human path is unavailable.
func recordPendingApproval(command, cwd, sessionID string, result *eval.Result) {
	hash, canonical, err := approve.Normalize(command)
	if err != nil {
		// Normalize() has three deliberate error taxa. Silently swallowing any
		// of them would produce a broken-promise deny message ("run `ccchain
		// approve --last`") because no pending record actually exists — the
		// human path would misfire against an unrelated pending entry.
		// Skeptic C1: for-loops (and other control-flow AST that Plan 0025
		// expands at the evaluate layer) hit ErrUnsupported here and used to
		// slip through silently.
		switch err {
		case approve.ErrDynamicCommand:
			result.Message = joinDenyMessage(result.Message,
				"ccchain: this command contains dynamic expansion ($VAR, $(...), backticks) and is not eligible for `ccchain approve`. Rewrite it with literal arguments, or run it in an interactive session.")
		case approve.ErrUnsupported:
			result.Message = joinDenyMessage(result.Message,
				"ccchain: this command uses a control structure (for/while/if/case/function) that the current approval mechanism cannot represent. Run each inner command individually, or execute this interactively.")
		}
		return
	}
	dir, derr := approve.DefaultDir()
	if derr != nil {
		fmt.Fprintf(os.Stderr, "ccchain: approve store dir error (skipping pending record): %v\n", derr)
		return
	}
	store := approve.NewStore(dir)
	if err := store.RecordPending(approve.PendingEntry{
		Hash:      hash,
		Command:   canonical,
		CWD:       cwd,
		SessionID: sessionID,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: approve pending record error: %v\n", err)
	}
}

// approvalAllowResult builds the eval result used when an approval was
// consumed. It surfaces as `permissionDecision: "allow"` with a reason,
// matching the way warn/hint land caution text in the agent's context.
func approvalAllowResult(entry *approve.ApprovedEntry) *eval.Result {
	reason := "ccchain: approved via `ccchain approve` (one-shot consumed)"
	if entry != nil && entry.Hash != "" {
		reason = fmt.Sprintf("ccchain: approved via `ccchain approve` (one-shot consumed, hash %s)", entry.Hash[:min(8, len(entry.Hash))])
	}
	return &eval.Result{Action: dsl.ActionWarn, Message: reason}
}

func joinDenyMessage(orig, extra string) string {
	if orig == "" {
		return extra
	}
	return orig + "\n" + extra
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

// emitResponse writes the hook JSON to stdout and exits.
//
// For allow/ask/warn, we exit 0 — modern Claude Code honors the JSON
// permissionDecision. For deny, we ALSO write the reason to stderr and exit 2
// (Security C2): older Claude Code versions and future regressions ignore the
// JSON body and only look at the exit code, so a hook that always exits 0
// would silently fail-open on a `deny`. Modern Claude Code prioritizes the
// JSON on exit 0 but also handles exit 2 as "block with stderr as reason";
// combining both is a belt-and-suspenders that keeps the deny path effective
// across versions.
func emitResponse(resp *hookResponse) {
	if resp == nil {
		os.Exit(0)
	}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "ccchain: json encode error: %v\n", err)
	}
	if resp.HookSpecificOutput.PermissionDecision == "deny" {
		reason := resp.HookSpecificOutput.PermissionDecisionReason
		if reason == "" {
			reason = "blocked by ccchain"
		}
		fmt.Fprintln(os.Stderr, reason)
		os.Exit(2)
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
