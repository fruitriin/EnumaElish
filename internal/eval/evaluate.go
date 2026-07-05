// Package eval implements the ccchain rule evaluation engine.
// It matches shell command topologies against DSL rules to produce allow/deny decisions.
package eval

import (
	"path/filepath"
	"strings"

	"github.com/fruitriin/ccchain/internal/dsl"
	"github.com/fruitriin/ccchain/internal/semantics"
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
		fallbackResult := &Result{
			Action:  fallback,
			Message: "no matching rule (fallback)",
		}
		// Even without a matching rule, workspace scope must still apply.
		// Otherwise `fallback: allow` (or fallback: warn/hint, which behave
		// like allow at the hook layer) would let paths outside workspace
		// through, bypassing `scope_violation: deny`.
		if restrictionLevel(fallbackResult.Action) < restrictionLevel(dsl.ActionAsk) {
			for i := range topo.Segments {
				seg := &topo.Segments[i]
				for j := range seg.Commands {
					fallbackResult = applyScopeToCommand(&seg.Commands[j], nil, config, fallbackResult)
				}
			}
		}
		return fallbackResult, nil
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
		lastMatch = applyScopeToCommand(cmd, lastMatchRule, config, lastMatch)
	}

	return lastMatch
}

// applyScopeToCommand checks path arguments against the workspace scope and,
// if the rule has a `scope:` block (Plan 0011 v2), applies per-scope actions
// with read/write awareness (semantics table). Without a `scope:` block it
// falls back to the pre-v2 behavior: escalate any result less restrictive
// than ask when any path is outside the workspace.
//
// Sources of paths (each is scope-classified independently):
//   - cmd.Args via ExtractPathArgs, kinded by semantics.ClassifyPathArgs
//     (flag-aware for cp/mv/ln -t DIR — Critical C2)
//   - cmd.Redirs — always PathKindWrite (Critical C1: `> /outside/y` was
//     invisible to the scope engine)
//
// Handling of dynamic args ($VAR / $(cmd)): rather than silently dropping
// them (Critical C4), we treat them as ScopeOutside with the "safe kind"
// — PathKindWrite if the command is unknown or a known write, PathKindRead
// otherwise. This way, `cp /ws/x $(echo /outside)/y` still triggers
// outside-write.
func applyScopeToCommand(cmd *shell.Command, rule *dsl.Rule, config *dsl.Config, baseResult *Result) *Result {
	if config.Settings == nil || len(config.Settings.WorkspacePaths) == 0 {
		return baseResult
	}

	pathInfo := collectScopePathInfo(cmd, config.Settings.WorkspacePaths)
	if len(pathInfo) == 0 {
		return baseResult
	}

	// v2: rule has `scope:` — evaluate each path against ScopeRule and return
	// the most restrictive result.
	if rule != nil && rule.ScopeRule != nil {
		var worst *Result
		for _, info := range pathInfo {
			act := selectScopeAction(rule.ScopeRule, info.scope, info.kind)
			if act == nil {
				continue // this rule doesn't say anything about this scope combo
			}
			candidate := &Result{
				Action:  act.Action,
				Message: firstNonEmpty(act.Message, "workspace scope: "+scopeDescription(info.scope, info.kind)),
				Context: baseResult.Context,
			}
			if isMoreRestrictive(candidate, worst) {
				worst = candidate
			}
		}
		if worst != nil && isMoreRestrictive(worst, baseResult) {
			return worst
		}
		// If the ScopeRule doesn't escalate, keep the base (rule's action or
		// args: override). This lets `scope: outside: allow` explicitly opt
		// out of the automatic escalation below.
		if rule.ScopeRule.Outside != nil || rule.ScopeRule.OutsideRead != nil || rule.ScopeRule.OutsideWrite != nil {
			return baseResult
		}
		// scope: block exists but says nothing about outside — fall through
		// to legacy escalation.
	}

	// Legacy behavior (pre-v2 or scope: block without outside*): escalate any
	// result less restrictive than ask when any path is outside.
	// Rationale: warn/hint return {"decision":"allow"} at the hook layer, so
	// they must not be treated as safer than allow for scope-violation
	// purposes. This mirrors the step-comparison used by argsTooLongResult
	// (from speculative/scope-violation-deny, now on main).
	if restrictionLevel(baseResult.Action) >= restrictionLevel(dsl.ActionAsk) {
		return baseResult
	}
	for _, info := range pathInfo {
		if info.scope == ScopeOutside {
			return &Result{
				Action:  scopeViolationAction(config),
				Message: "workspace scope: command accesses path outside workspace",
				Context: baseResult.Context,
			}
		}
	}
	return baseResult
}

// scopePathInfo is the pre-classified info used by applyScopeToCommand and
// the tool-level scope check (see EvaluateTool → applyScopeToToolCall).
type scopePathInfo struct {
	path  string
	scope ScopeResult
	kind  semantics.PathKind
}

// collectScopePathInfo collects and classifies all path targets of a Bash
// command: positional args (through the flag-aware semantics table) plus
// write-side shell redirects.
func collectScopePathInfo(cmd *shell.Command, workspacePaths []string) []scopePathInfo {
	paths := ExtractPathArgs(cmd.Args)
	kinds := semantics.ClassifyPathArgs(cmd.Name, cmd.Args, paths)

	var out []scopePathInfo
	// Positional path args.
	for i, p := range paths {
		info := scopePathInfo{path: p, kind: kinds[i]}
		if isDynamicPath(p) {
			// Dynamic — we cannot know where it points. Fail closed:
			// treat as outside, and upgrade Unknown to Write so that
			// outside-write rules match (Critical C4).
			info.scope = ScopeOutside
			if info.kind == semantics.PathKindUnknown || info.kind == semantics.PathKindRead {
				// For known-read tools (cat, grep), don't lie about direction
				// — but Unknown must be considered write for deny to fire.
				if info.kind == semantics.PathKindUnknown {
					info.kind = semantics.PathKindWrite
				}
			}
		} else {
			info.scope = ClassifyPath(p, workspacePaths)
		}
		out = append(out, info)
	}
	// Write-side redirects (Critical C1): all are PathKindWrite.
	for _, r := range cmd.Redirs {
		info := scopePathInfo{
			path: r.Path,
			kind: semantics.PathKindWrite,
		}
		if !r.Analyzable || isDynamicPath(r.Path) {
			info.scope = ScopeOutside
		} else {
			info.scope = ClassifyPath(r.Path, workspacePaths)
		}
		out = append(out, info)
	}
	return out
}

// isDynamicPath reports whether a path contains shell expansions that we
// cannot statically resolve.
func isDynamicPath(p string) bool {
	return strings.ContainsAny(p, "$`")
}

// selectScopeAction picks the ScopeAction that applies to a given scope+kind
// combination.
//
// Precedence:
//   - inside → sr.Inside
//   - outside + read → outside-read > outside
//   - outside + write → outside-write > outside
//   - outside + unknown (Critical C3) → most restrictive of outside-write /
//     outside-read / outside, so that unknown tools like `sed -i` cannot
//     silently bypass `outside-write: deny`.
func selectScopeAction(sr *dsl.ScopeRule, scope ScopeResult, kind semantics.PathKind) *dsl.ScopeAction {
	if scope == ScopeInside {
		return sr.Inside
	}
	// scope == ScopeOutside
	switch kind {
	case semantics.PathKindRead:
		if sr.OutsideRead != nil {
			return sr.OutsideRead
		}
	case semantics.PathKindWrite:
		if sr.OutsideWrite != nil {
			return sr.OutsideWrite
		}
	case semantics.PathKindUnknown:
		// Consider all outside-* clauses and pick the most restrictive.
		var chosen *dsl.ScopeAction
		for _, cand := range []*dsl.ScopeAction{sr.OutsideWrite, sr.OutsideRead, sr.Outside} {
			if cand == nil {
				continue
			}
			if chosen == nil || restrictionLevel(cand.Action) > restrictionLevel(chosen.Action) {
				chosen = cand
			}
		}
		return chosen
	}
	return sr.Outside
}

func scopeDescription(scope ScopeResult, kind semantics.PathKind) string {
	if scope == ScopeInside {
		return "inside workspace"
	}
	if kind == semantics.PathKindWrite {
		return "write outside workspace"
	}
	return "read outside workspace"
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
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
				pipeRules = append(pipeRules, dsl.CollectTemplatePipeRules(tmpl, config)...)
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
				execRules = append(execRules, dsl.CollectTemplateExecRules(tmpl, config)...)
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

// maxArgsLen bounds the length (in bytes) of the argument string that args:
// patterns are matched against. Go's regexp is RE2-based and runs in linear
// time, so catastrophic backtracking (classic ReDoS) is not possible, but
// matching many patterns against megabyte-scale argument strings still costs
// CPU and memory proportional to input size. 4096 bytes comfortably covers
// legitimate CLI invocations (typical args: patterns target short flag
// sequences) while bounding the per-rule work.
//
// Exceeding the limit does NOT fall back to the parent rule's action: padding
// arguments past the limit would otherwise trivially bypass an escalating
// args: rule (e.g. `allow curl` + `args: -X POST: ask`, or worse,
// `allow rm` + `args: -rf /: deny`). Consistent with the project's fail-open
// policy — which applies only to ccchain's own errors, not to "analyzed but
// safety could not be determined" cases (see docs/guide/structural-context.md
// / docs/ja/guide/structural-context.md) — the result is escalated to the
// strictest action declared in that rule's ArgsRules block, floored at ask.
// If the block contains a `deny` entry, deny wins; otherwise ask. An already
// stricter parent action (ask/deny) is always kept — the limit never
// de-escalates.
const maxArgsLen = 4096

// argsTooLongResult returns the safe-side result for an over-length argument
// string. The strictest action declared anywhere in the rule's ArgsRules
// block is used as the ceiling, so `allow rm` + `args: -rf /: deny` still
// denies when the argument string is over-length. The floor is ask — even
// an ArgsRules block that only allows escalates over-length inputs to ask,
// since we could not verify the pattern.
func argsTooLongResult(rule *dsl.Rule, baseResult *Result) *Result {
	// Ceiling: the strictest action in this rule's ArgsRules block, capped
	// below by ask so an all-allow block still asks.
	strictest := dsl.ActionAsk
	strictestLevel := restrictionLevel(strictest)
	for _, ar := range rule.ArgsRules {
		if lvl := restrictionLevel(ar.Action); lvl > strictestLevel {
			strictest = ar.Action
			strictestLevel = lvl
		}
	}
	// Never de-escalate: if the parent action is already stricter, keep it.
	if restrictionLevel(baseResult.Action) >= strictestLevel {
		return baseResult
	}
	return &Result{
		Action:  strictest,
		Message: "args: rules skipped: argument string exceeds max length, escalating to strictest args: action",
		Context: baseResult.Context,
	}
}

// applyArgsRules evaluates args: rules against a command's arguments.
// If a pattern matches, the action overrides the parent rule's action (last-rule-wins).
// If arguments contain dynamic expansion ($VAR, $(cmd)), args: evaluation is skipped
// and the base result is returned unchanged.
// If the joined argument string exceeds maxArgsLen, args: rules cannot be
// applied safely and the result is escalated to ask (see maxArgsLen).
func applyArgsRules(cmd *shell.Command, rule *dsl.Rule, baseResult *Result) *Result {
	if len(rule.ArgsRules) == 0 {
		return baseResult
	}

	argsStr := strings.Join(cmd.Args, " ")
	if len(argsStr) > maxArgsLen {
		return argsTooLongResult(rule, baseResult)
	}

	// Skip args: evaluation if arguments contain dynamic expansion
	if containsDynamicArgs(cmd.Args) {
		return baseResult
	}

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
