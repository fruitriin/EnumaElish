package eval

import (
	"path/filepath"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
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

	// Apply workspace scope for file-based tools
	if toolArg != "" && config.Settings != nil && len(config.Settings.WorkspacePaths) > 0 {
		scope := ClassifyPath(toolArg, config.Settings.WorkspacePaths)
		if scope == ScopeOutside {
			// Outside workspace → escalate to at least "ask" if currently "allow"
			if lastMatch != nil && lastMatch.Action == dsl.ActionAllow {
				return &Result{
					Action:  dsl.ActionAsk,
					Message: "workspace scope: accessing path outside workspace",
					Context: []string{toolName, toolArg},
				}
			}
		}
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

// applyToolArgsRules evaluates args: rules against a tool argument string.
func applyToolArgsRules(toolArg string, rule *dsl.Rule, baseResult *Result) *Result {
	if len(rule.ArgsRules) == 0 {
		return baseResult
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
		return lastMatch
	}
	return baseResult
}
