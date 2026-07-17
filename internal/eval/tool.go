package eval

import (
	"path/filepath"
	"strings"

	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/semantics"
)

// EvaluateTool evaluates a non-Bash tool call against config rules.
// toolName is the Claude Code tool name (e.g., "Read", "Edit", "WebFetch", "mcp__github__create_issue").
// toolArg is the primary argument to match against args: rules (e.g., file path, URL).
func EvaluateTool(toolName string, toolArg string, config *dsl.Config) *Result {
	// Collect all applicable rules
	rules := make([]*dsl.Rule, 0, len(config.PreRules)+len(config.Rules))
	rules = append(rules, config.PreRules...)
	rules = append(rules, config.Rules...)

	var lastMatch *Result
	var lastMatchRule *dsl.Rule

	for _, rule := range rules {
		if matchesToolRule(toolName, rule) {
			lastMatch = &Result{
				Action:  rule.Action,
				Message: rule.Message,
				Context: []string{toolName},
			}
			lastMatchRule = rule
		}
	}

	// Apply args: rules against the tool argument
	if lastMatch != nil && lastMatchRule != nil && toolArg != "" {
		lastMatch = applyToolArgsRules(toolArg, lastMatchRule, lastMatch)
	}

	// Apply workspace scope for file-based tools.
	// Critical C6 (from workspace-scope-hardening): scope: block on
	// Read/Edit/Write/MCP rules was not honored — routed through
	// applyScopeToToolCall which handles both the `scope:` block and the
	// legacy escalation.
	// Critical (from scope-violation-deny): even without a matching rule,
	// workspace scope must still apply so that `fallback: allow` (or
	// fallback: warn/hint, which behave like allow at the hook layer) does
	// not bypass `scope_violation: deny`. We synthesize the fallback result
	// so scope escalation can act on it uniformly.
	if toolArg != "" && config.Settings != nil && len(config.Settings.WorkspacePaths) > 0 {
		effective := lastMatch
		if effective == nil {
			fallback := dsl.ActionAsk
			if config.Settings != nil {
				fallback = config.Settings.Fallback
			}
			effective = &Result{
				Action:  fallback,
				Message: "no matching rule (fallback)",
				Context: []string{toolName},
			}
		}
		lastMatch = applyScopeToToolCall(toolName, toolArg, lastMatchRule, config, effective)
	}

	if lastMatch != nil {
		return lastMatch
	}

	// Fallback
	fallback := dsl.ActionAsk
	if config.Settings != nil {
		fallback = config.Settings.Fallback
	}
	return &Result{
		Action:  fallback,
		Message: "no matching rule (fallback)",
		Context: []string{toolName},
	}
}

// matchesToolRule checks if a tool name matches a rule's command list.
// Supports exact match, base name match, and glob-like wildcards for MCP tools.
func matchesToolRule(toolName string, rule *dsl.Rule) bool {
	for _, c := range rule.Commands {
		// Exact match
		if c == toolName {
			return true
		}
		// Glob match for MCP tools (e.g., "mcp__*__delete_*")
		if strings.Contains(c, "*") {
			if matched, _ := filepath.Match(c, toolName); matched {
				return true
			}
		}
	}
	return false
}

// toolPathKind maps a Claude Code tool name to the direction of its primary
// path argument. Read tools are PathKindRead; Write/Edit are PathKindWrite;
// MCP tools and anything unfamiliar are PathKindUnknown so outside-write
// clauses still fire (Critical C3 applied to tools).
func toolPathKind(toolName string) semantics.PathKind {
	switch toolName {
	case "Read", "Grep", "Glob":
		return semantics.PathKindRead
	case "Write", "Edit", "NotebookEdit":
		return semantics.PathKindWrite
	case "WebFetch", "WebSearch":
		// URL — treat as read.
		return semantics.PathKindRead
	}
	// MCP tools (mcp__*) and unknown tools: unknown direction.
	return semantics.PathKindUnknown
}

// applyScopeToToolCall applies the workspace scope decision to a tool call.
// When the rule has a `scope:` block, per-scope actions are used (with tool-
// name-derived PathKind). When it doesn't, we fall back to the legacy
// allow → ask escalation.
func applyScopeToToolCall(toolName, toolArg string, rule *dsl.Rule, config *dsl.Config, baseResult *Result) *Result {
	kind := toolPathKind(toolName)

	var scope ScopeResult
	if isDynamicPath(toolArg) {
		// Dynamic tool argument (rare for Claude Code tools, but be honest):
		// treat as outside so outside-write / outside can catch it.
		scope = ScopeOutside
		if kind == semantics.PathKindUnknown {
			kind = semantics.PathKindWrite
		}
	} else {
		scope = ClassifyPath(toolArg, config.Settings.WorkspacePaths)
	}

	// v2: rule has `scope:` — pick the per-scope action.
	if rule != nil && rule.ScopeRule != nil {
		if act := selectScopeAction(rule.ScopeRule, scope, kind); act != nil {
			candidate := &Result{
				Action:  act.Action,
				Message: firstNonEmpty(act.Message, "workspace scope: "+scopeDescription(scope, kind)),
				Context: []string{toolName, toolArg},
				// Skeptic C3: preserve parent's Unattended (see the Bash
				// path for details).
				Unattended: baseResult.Unattended,
			}
			if isMoreRestrictive(candidate, baseResult) {
				return candidate
			}
		}
		// scope: block present but doesn't cover outside* → keep baseResult
		// unless we should fall through to legacy escalation.
		if rule.ScopeRule.Outside != nil || rule.ScopeRule.OutsideRead != nil || rule.ScopeRule.OutsideWrite != nil {
			return baseResult
		}
	}

	// Legacy escalation. Use step-comparison (from scope-violation-deny) so
	// warn/hint and `fallback: allow` are treated as escalation candidates,
	// and honor settings.scope_violation for the target action (ask/deny).
	if scope == ScopeOutside && restrictionLevel(baseResult.Action) < restrictionLevel(dsl.ActionAsk) {
		return &Result{
			Action:  scopeViolationAction(config),
			Message: "workspace scope: accessing path outside workspace",
			Context: []string{toolName, toolArg},
			// Skeptic C3: preserve parent's Unattended.
			Unattended: baseResult.Unattended,
		}
	}
	return baseResult
}

// applyToolArgsRules evaluates args: rules against a tool argument string.
// Over-length arguments escalate to the strictest action in the rule's
// ArgsRules block (see maxArgsLen and argsTooLongResult in evaluate.go).
func applyToolArgsRules(toolArg string, rule *dsl.Rule, baseResult *Result) *Result {
	if len(rule.ArgsRules) == 0 {
		return baseResult
	}

	if len(toolArg) > maxArgsLen {
		return argsTooLongResult(rule, baseResult)
	}

	// Skip if arg contains dynamic expansion
	if strings.ContainsAny(toolArg, "$`") {
		return baseResult
	}

	var lastMatch *Result
	for _, ar := range rule.ArgsRules {
		if ar.Compiled != nil && ar.Compiled.MatchString(toolArg) {
			lastMatch = &Result{
				Action:  ar.Action,
				Message: ar.Message,
				Context: baseResult.Context,
			}
		}
	}

	if lastMatch != nil {
		// Skeptic C3: propagate parent's Unattended (tool-side mirror of
		// applyArgsRules).
		lastMatch.Unattended = baseResult.Unattended
		return lastMatch
	}
	return baseResult
}
