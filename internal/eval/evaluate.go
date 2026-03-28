// Package eval implements the ccchain rule evaluation engine.
// It matches shell command topologies against DSL rules to produce allow/deny decisions.
package eval

import (
	"path/filepath"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/shell"
)

// Result represents the evaluation result for a command.
type Result struct {
	Action      dsl.Action `json:"action"`
	Message     string     `json:"message,omitempty"`
	MatchedRule string     `json:"matched_rule,omitempty"`
	Template    string     `json:"template,omitempty"`
	Context     []string   `json:"context,omitempty"`
}

// Evaluate evaluates a shell command string against a DSL config.
// Returns the most restrictive result across all segments.
func Evaluate(cmd string, config *dsl.Config) (*Result, error) {
	topo, err := shell.BuildTopology(cmd)
	if err != nil {
		return nil, err
	}

	result, err := EvaluateTopology(topo, config)
	if err != nil {
		return nil, err
	}

	// Expand template variables in deny/warn messages
	if result != nil && result.Message != "" {
		cmdName := ""
		var cmdArgs []string
		if len(topo.Segments) > 0 && len(topo.Segments[0].Commands) > 0 {
			cmdName = topo.Segments[0].Commands[0].Name
			cmdArgs = topo.Segments[0].Commands[0].Args
		}
		result.Message = ExpandMessage(result.Message, cmdName, cmdArgs, cmd)
	}

	return result, nil
}

// EvaluateTopology evaluates a topology against a DSL config.
func EvaluateTopology(topo *shell.Topology, config *dsl.Config) (*Result, error) {
	// Collect all applicable rules (PreRules + generic Rules)
	// Use explicit copy to avoid mutating config.PreRules' backing array
	rules := make([]*dsl.Rule, 0, len(config.PreRules)+len(config.Rules))
	rules = append(rules, config.PreRules...)
	rules = append(rules, config.Rules...)

	var worstResult *Result

	for _, seg := range topo.Segments {
		result := evaluateSegment(&seg, rules, config)
		if result != nil && isMoreRestrictive(result, worstResult) {
			worstResult = result
		}
	}

	if worstResult == nil {
		// No rule matched — use fallback
		fallback := dsl.ActionAsk
		if config.Settings != nil {
			fallback = config.Settings.Fallback
		}
		return &Result{
			Action:  fallback,
			Message: "no matching rule (fallback)",
		}, nil
	}

	return worstResult, nil
}

// evaluateSegment evaluates a single segment against rules.
func evaluateSegment(seg *shell.Segment, rules []*dsl.Rule, config *dsl.Config) *Result {
	if seg.Type == shell.SegmentTypePipeline {
		return evaluatePipeline(seg.Commands, rules, config)
	}

	// Single command
	if len(seg.Commands) == 0 {
		return nil
	}
	cmd := &seg.Commands[0]

	// Check analyzability
	if !cmd.Analyzable {
		return &Result{
			Action:  dsl.ActionDeny,
			Message: "dynamic command detected: static analysis not possible",
			Context: []string{cmd.Name},
		}
	}

	result := matchCommand(cmd, nil, rules, config)

	// Check nested commands (find -exec, etc.) for single commands too
	if cmd.Nested != nil {
		nestedResult := evaluateNested(cmd, rules, config)
		if nestedResult != nil && isMoreRestrictive(nestedResult, result) {
			result = nestedResult
		}
	}

	return result
}

// evaluatePipeline evaluates a pipeline of commands.
// Commands are evaluated left to right, building context.
func evaluatePipeline(cmds []shell.Command, rules []*dsl.Rule, config *dsl.Config) *Result {
	if len(cmds) == 0 {
		return nil
	}

	var worstResult *Result

	// First command is evaluated at top level
	firstCmd := &cmds[0]
	if !firstCmd.Analyzable {
		return &Result{
			Action:  dsl.ActionDeny,
			Message: "dynamic command detected: static analysis not possible",
			Context: []string{firstCmd.Name},
		}
	}

	firstResult := matchCommand(firstCmd, nil, rules, config)
	if firstResult != nil && isMoreRestrictive(firstResult, worstResult) {
		worstResult = firstResult
	}

	// Find the rule that matches the first command (for pipe context)
	parentRule := findMatchingRule(firstCmd.Name, rules)

	// Subsequent commands are evaluated in pipe context
	for i := 1; i < len(cmds); i++ {
		cmd := &cmds[i]
		context := buildContext(cmds[:i])

		if !cmd.Analyzable {
			result := &Result{
				Action:  dsl.ActionDeny,
				Message: "dynamic command detected in pipeline: static analysis not possible",
				Context: append(context, cmd.Name),
			}
			if isMoreRestrictive(result, worstResult) {
				worstResult = result
			}
			continue
		}

		// Check pipe rules from the parent command's rule
		result := matchInPipeContext(cmd, parentRule, context, config)
		if result == nil {
			// Fall back to top-level evaluation
			result = matchCommand(cmd, context, rules, config)
		}
		if result != nil && isMoreRestrictive(result, worstResult) {
			worstResult = result
		}
	}

	// Also check nested commands (find -exec, etc.)
	for i := range cmds {
		cmd := &cmds[i]
		if cmd.Nested != nil {
			nestedResult := evaluateNested(cmd, rules, config)
			if nestedResult != nil && isMoreRestrictive(nestedResult, worstResult) {
				worstResult = nestedResult
			}
		}
	}

	return worstResult
}

// evaluateNested evaluates nested commands (find -exec, bash -c, etc.)
func evaluateNested(parent *shell.Command, rules []*dsl.Rule, config *dsl.Config) *Result {
	return evaluateNestedWithDepth(parent, rules, config, 0)
}

func evaluateNestedWithDepth(parent *shell.Command, rules []*dsl.Rule, config *dsl.Config, depth int) *Result {
	if parent.Nested == nil {
		return nil
	}

	maxDepth := 2
	if config.Settings != nil && config.Settings.MaxContextDepth > 0 {
		maxDepth = config.Settings.MaxContextDepth
	}
	if depth >= maxDepth {
		return &Result{
			Action:  dsl.ActionDeny,
			Message: "max context depth exceeded",
			Context: []string{parent.Name, "exec: (depth limit)"},
		}
	}

	parentRule := findMatchingRule(parent.Name, rules)

	var worstResult *Result

	for _, seg := range parent.Nested.Segments {
		for _, cmd := range seg.Commands {
			if !cmd.Analyzable {
				result := &Result{
					Action:  dsl.ActionDeny,
					Message: "dynamic command in " + parent.Name + " context",
					Context: []string{parent.Name, "exec:", cmd.Name},
				}
				if isMoreRestrictive(result, worstResult) {
					worstResult = result
				}
				continue
			}

			// Check exec rules from parent's rule
			context := []string{parent.Name, "exec:"}
			result := matchInExecContext(&cmd, parentRule, context, config)
			if result == nil {
				// Fall back to top-level
				result = matchCommand(&cmd, context, rules, config)
			}
			if result != nil && isMoreRestrictive(result, worstResult) {
				worstResult = result
			}

			// Recurse into nested commands with depth tracking
			if cmd.Nested != nil {
				nestedResult := evaluateNestedWithDepth(&cmd, rules, config, depth+1)
				if nestedResult != nil && isMoreRestrictive(nestedResult, worstResult) {
					worstResult = nestedResult
				}
			}
		}
	}

	return worstResult
}

// matchCommand matches a command against top-level rules (last-rule-wins).
func matchCommand(cmd *shell.Command, context []string, rules []*dsl.Rule, config *dsl.Config) *Result {
	var lastMatch *Result
	var lastMatchRule *dsl.Rule

	for _, rule := range rules {
		if matchesRule(cmd.Name, rule) {
			lastMatch = &Result{
				Action:  rule.Action,
				Message: rule.Message,
				Context: appendContext(context, cmd.Name),
			}
			lastMatchRule = rule
		}
	}

	// Apply args: rules if the matched rule has them
	if lastMatch != nil && lastMatchRule != nil {
		lastMatch = applyArgsRules(cmd, lastMatchRule, lastMatch)
	}

	// Apply workspace scope to command arguments
	if lastMatch != nil {
		lastMatch = applyScopeToCommand(cmd, config, lastMatch)
	}

	return lastMatch
}

// applyScopeToCommand checks if any path arguments are outside the workspace.
// If so, escalates allow → ask.
func applyScopeToCommand(cmd *shell.Command, config *dsl.Config, baseResult *Result) *Result {
	if config.Settings == nil || len(config.Settings.WorkspacePaths) == 0 {
		return baseResult
	}
	if baseResult.Action != dsl.ActionAllow {
		return baseResult // only escalate allow → ask
	}

	paths := ExtractPathArgs(cmd.Args)
	if len(paths) == 0 {
		return baseResult
	}

	for _, p := range paths {
		// Skip dynamic args
		if strings.ContainsAny(p, "$`") {
			continue
		}
		scope := ClassifyPath(p, config.Settings.WorkspacePaths)
		if scope == ScopeOutside {
			return &Result{
				Action:  dsl.ActionAsk,
				Message: "workspace scope: command accesses path outside workspace",
				Context: baseResult.Context,
			}
		}
	}

	return baseResult
}

// matchInPipeContext checks pipe rules from a parent rule and its templates.
func matchInPipeContext(cmd *shell.Command, parentRule *dsl.Rule, context []string, config *dsl.Config) *Result {
	var pipeRules []*dsl.Rule

	// Collect pipe rules from the parent rule
	if parentRule != nil {
		pipeRules = append(pipeRules, parentRule.PipeRules...)

		// Also collect pipe rules from template (via next:)
		if parentRule.Next != "" {
			tmpl := dsl.LookupTemplate(config, parentRule.Next)
			if tmpl != nil {
				pipeRules = append(pipeRules, collectTemplatePipeRules(tmpl, config)...)
			}
		}
	}

	// Match against pipe rules (last-rule-wins)
	var lastMatch *Result
	var lastMatchRule *dsl.Rule
	for _, rule := range pipeRules {
		if matchesRule(cmd.Name, rule) {
			tmplName := ""
			if parentRule != nil && parentRule.Next != "" {
				tmplName = parentRule.Next
			}
			lastMatch = &Result{
				Action:   rule.Action,
				Message:  rule.Message,
				Template: tmplName,
				Context:  append(context, "|", cmd.Name),
			}
			lastMatchRule = rule
		}
	}

	// Apply args: rules
	if lastMatch != nil && lastMatchRule != nil {
		lastMatch = applyArgsRules(cmd, lastMatchRule, lastMatch)
	}

	return lastMatch
}

// matchInExecContext checks exec rules from a parent rule and its templates.
func matchInExecContext(cmd *shell.Command, parentRule *dsl.Rule, context []string, config *dsl.Config) *Result {
	var execRules []*dsl.Rule

	if parentRule != nil {
		execRules = append(execRules, parentRule.ExecRules...)

		if parentRule.Next != "" {
			tmpl := dsl.LookupTemplate(config, parentRule.Next)
			if tmpl != nil {
				execRules = append(execRules, collectTemplateExecRules(tmpl, config)...)
			}
		}
	}

	var lastMatch *Result
	var lastMatchRule *dsl.Rule
	for _, rule := range execRules {
		if matchesRule(cmd.Name, rule) {
			tmplName := ""
			if parentRule != nil && parentRule.Next != "" {
				tmplName = parentRule.Next
			}
			lastMatch = &Result{
				Action:   rule.Action,
				Message:  rule.Message,
				Template: tmplName,
				Context:  append(context, cmd.Name),
			}
			lastMatchRule = rule
		}
	}

	// Apply args: rules
	if lastMatch != nil && lastMatchRule != nil {
		lastMatch = applyArgsRules(cmd, lastMatchRule, lastMatch)
	}

	return lastMatch
}

// collectTemplatePipeRules collects all pipe rules from a template chain.
// visited prevents infinite loops from circular next: references.
func collectTemplatePipeRules(tmpl *dsl.Template, config *dsl.Config) []*dsl.Rule {
	visited := make(map[string]bool)
	return collectTemplatePipeRulesWithVisited(tmpl, config, visited)
}

func collectTemplatePipeRulesWithVisited(tmpl *dsl.Template, config *dsl.Config, visited map[string]bool) []*dsl.Rule {
	if visited[tmpl.Name] {
		return nil
	}
	visited[tmpl.Name] = true

	var rules []*dsl.Rule
	rules = append(rules, tmpl.PipeRules...)

	if tmpl.Next != "" {
		nextTmpl := dsl.LookupTemplate(config, tmpl.Next)
		if nextTmpl != nil {
			rules = append(rules, collectTemplatePipeRulesWithVisited(nextTmpl, config, visited)...)
		}
	}

	return rules
}

// collectTemplateExecRules collects all exec rules from a template chain.
func collectTemplateExecRules(tmpl *dsl.Template, config *dsl.Config) []*dsl.Rule {
	visited := make(map[string]bool)
	return collectTemplateExecRulesWithVisited(tmpl, config, visited)
}

func collectTemplateExecRulesWithVisited(tmpl *dsl.Template, config *dsl.Config, visited map[string]bool) []*dsl.Rule {
	if visited[tmpl.Name] {
		return nil
	}
	visited[tmpl.Name] = true

	var rules []*dsl.Rule
	rules = append(rules, tmpl.ExecRules...)

	if tmpl.Next != "" {
		nextTmpl := dsl.LookupTemplate(config, tmpl.Next)
		if nextTmpl != nil {
			rules = append(rules, collectTemplateExecRulesWithVisited(nextTmpl, config, visited)...)
		}
	}

	return rules
}

// findMatchingRule finds the last matching top-level rule for a command name.
func findMatchingRule(cmdName string, rules []*dsl.Rule) *dsl.Rule {
	var lastMatch *dsl.Rule
	for _, rule := range rules {
		if matchesRule(cmdName, rule) {
			lastMatch = rule
		}
	}
	return lastMatch
}

// matchesRule checks if a command name matches a rule's command list.
func matchesRule(cmdName string, rule *dsl.Rule) bool {
	baseName := filepath.Base(cmdName)
	for _, c := range rule.Commands {
		if c == cmdName || c == baseName {
			return true
		}
	}
	return false
}

// isMoreRestrictive returns true if a is more restrictive than b.
func isMoreRestrictive(a, b *Result) bool {
	if b == nil {
		return true
	}
	return restrictionLevel(a.Action) > restrictionLevel(b.Action)
}

func restrictionLevel(action dsl.Action) int {
	switch action {
	case dsl.ActionAllow:
		return 0
	case dsl.ActionHint:
		return 1
	case dsl.ActionWarn:
		return 2
	case dsl.ActionAsk:
		return 3
	case dsl.ActionDeny:
		return 4
	default:
		return 0
	}
}

func buildContext(cmds []shell.Command) []string {
	var ctx []string
	for _, c := range cmds {
		ctx = append(ctx, c.Name)
	}
	return ctx
}

func appendContext(base []string, items ...string) []string {
	out := make([]string, len(base), len(base)+len(items))
	copy(out, base)
	return append(out, items...)
}

// applyArgsRules evaluates args: rules against a command's arguments.
// If a pattern matches, the action overrides the parent rule's action (last-rule-wins).
// If arguments contain dynamic expansion ($VAR, $(cmd)), args: evaluation is skipped
// and the base result is returned unchanged.
func applyArgsRules(cmd *shell.Command, rule *dsl.Rule, baseResult *Result) *Result {
	if len(rule.ArgsRules) == 0 {
		return baseResult
	}

	// Skip args: evaluation if arguments contain dynamic expansion
	if containsDynamicArgs(cmd.Args) {
		return baseResult
	}

	argsStr := strings.Join(cmd.Args, " ")
	var lastMatch *Result

	for _, ar := range rule.ArgsRules {
		if ar.Compiled != nil && ar.Compiled.MatchString(argsStr) {
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

// containsDynamicArgs checks if any argument contains shell variable expansion.
func containsDynamicArgs(args []string) bool {
	for _, arg := range args {
		if strings.ContainsAny(arg, "$`") {
			return true
		}
	}
	return false
}
