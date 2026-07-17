package eval

import (
	"strings"
	"testing"

	"github.com/fruitriin/EnumaElish/internal/dsl"
)

// helper to parse DSL config from string
func mustParseConfig(t *testing.T, input string) *dsl.Config {
	t.Helper()
	cfg, err := dsl.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if err := dsl.ResolveTemplates(cfg); err != nil {
		t.Fatalf("resolve templates: %v", err)
	}
	return cfg
}

func TestEvaluateSimpleAllow(t *testing.T) {
	cfg := mustParseConfig(t, `allow ls`)
	result, err := Evaluate("ls -la", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAllow)
}

func TestEvaluateSimpleDeny(t *testing.T) {
	cfg := mustParseConfig(t, `deny rm`)
	result, err := Evaluate("rm -rf foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

func TestEvaluateLastRuleWins(t *testing.T) {
	cfg := mustParseConfig(t, `
allow rm
deny rm
`)
	result, err := Evaluate("rm foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

func TestEvaluateFallback(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  fallback: ask
allow ls
`)
	result, err := Evaluate("cat foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAsk)
}

func TestEvaluatePipeContext(t *testing.T) {
	cfg := mustParseConfig(t, `
allow find
  |,>>
    allow cat
    deny rm  "don't combine find with rm"
deny rm
`)
	// find | rm should be deny (pipe context rule)
	result, err := Evaluate("find . | rm", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	if !strings.Contains(result.Message, "don't combine") {
		t.Errorf("expected message about combining, got: %s", result.Message)
	}
}

func TestEvaluateAndReset(t *testing.T) {
	cfg := mustParseConfig(t, `
allow find
  |,>>
    deny rm  "don't pipe find into rm"
deny rm
`)
	// find && rm → reset → rm evaluated at top level → deny (top-level rule)
	result, err := Evaluate("find . && rm foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	// The deny should come from top-level "deny rm", not from pipe context
}

func TestEvaluateCurlPipeBash(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  |
    deny bash  "curl | bash is not allowed"
    deny sh    "curl | sh is not allowed"
`)
	result, err := Evaluate("curl https://example.com | bash", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	if !strings.Contains(result.Message, "curl | bash") {
		t.Errorf("expected curl|bash message, got: %s", result.Message)
	}
}

func TestEvaluateTemplateNext(t *testing.T) {
	cfg := mustParseConfig(t, `
template bulkExec
  |,>>
    deny rm  "don't pipe into destructive"
  exec:
    deny rm  "expand to tempfile first"

allow find
  next: bulkExec
`)
	// find | rm → deny via template
	result, err := Evaluate("find . | rm", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	assertEqual(t, "template", result.Template, "bulkExec")
}

func TestEvaluateFindExec(t *testing.T) {
	cfg := mustParseConfig(t, `
template bulkExec
  exec:
    deny rm  "expand to tempfile first"
    allow cp

allow find
  next: bulkExec
`)
	result, err := Evaluate(`find . -exec rm {} \;`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	if !strings.Contains(result.Message, "tempfile") {
		t.Errorf("expected tempfile message, got: %s", result.Message)
	}
}

func TestEvaluateDynamicCommand(t *testing.T) {
	cfg := mustParseConfig(t, `allow ls`)
	result, err := Evaluate("$cmd foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	if !strings.Contains(result.Message, "dynamic") {
		t.Errorf("expected dynamic command message, got: %s", result.Message)
	}
}

// Plan 0025 Phase 2: unanalyzable_action affects the deny-fallback path.

// TestEvaluateUnanalyzableActionAskDynamicCommand verifies that with
// `unanalyzable_action: ask`, a dynamic command name that would previously
// have denied now asks instead.
func TestEvaluateUnanalyzableActionAskDynamicCommand(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  unanalyzable_action: ask
allow ls
`)
	result, err := Evaluate("$cmd foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAsk)
	if !strings.Contains(result.Message, "dynamic") {
		t.Errorf("expected dynamic command message, got: %s", result.Message)
	}
}

// TestEvaluateUnanalyzableActionAskSubshell verifies the same setting affects
// subshells / other control-flow structures — not just dynamic command names.
func TestEvaluateUnanalyzableActionAskSubshell(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  unanalyzable_action: ask
`)
	result, err := Evaluate("(rm x; rm y)", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAsk)
}

// TestEvaluateUnanalyzableActionDefaultDeny is a regression test for the
// unchanged existing behavior when unanalyzable_action is not set: dynamic
// commands still return deny (Plan 0025 Phase 2 backward compat).
func TestEvaluateUnanalyzableActionDefaultDeny(t *testing.T) {
	cfg := mustParseConfig(t, `allow ls`)
	if cfg.Settings.UnanalyzableAction != dsl.ActionDeny {
		t.Errorf("expected default UnanalyzableAction=deny, got %v", cfg.Settings.UnanalyzableAction)
	}
	result, err := Evaluate("$cmd foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateUnanalyzableActionExplicitDeny verifies explicit
// `unanalyzable_action: deny` behaves the same as the default.
func TestEvaluateUnanalyzableActionExplicitDeny(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  unanalyzable_action: deny
`)
	result, err := Evaluate("$cmd foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateUnanalyzableActionDoesNotAffectAnalyzable verifies that when
// the command IS analyzable and matches an allow rule, the setting does not
// spuriously downgrade it to ask.
func TestEvaluateUnanalyzableActionDoesNotAffectAnalyzable(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  unanalyzable_action: ask
allow ls
`)
	result, err := Evaluate("ls -la", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAllow)
}

// Plan 0025 Phase 1: for-loop expansion at the eval layer — expanded body
// commands are matched against rules, so a deny rule for the body command
// fires for the whole loop.

// TestEvaluateForLoopExpandedBodyMatchesDenyRule verifies that
// `for f in a b; do rm -rf "$f"; done` denies when rm has a deny rule.
func TestEvaluateForLoopExpandedBodyMatchesDenyRule(t *testing.T) {
	cfg := mustParseConfig(t, `deny rm`)
	result, err := Evaluate(`for f in a b; do rm -rf "$f"; done`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateForLoopExpandedBodyAllows verifies that a for-loop over a safe
// body (allow rule) evaluates to allow after expansion — previously this
// would have been the hardcoded "dynamic command detected" deny.
func TestEvaluateForLoopExpandedBodyAllows(t *testing.T) {
	cfg := mustParseConfig(t, `allow cat`)
	result, err := Evaluate(`for f in a.txt b.txt; do cat "$f"; done`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAllow)
}

// TestEvaluateForLoopDynamicWordListStillDenies confirms the safety
// invariant: a for-loop with a non-literal word list keeps the pre-existing
// deny behavior.
func TestEvaluateForLoopDynamicWordListStillDenies(t *testing.T) {
	cfg := mustParseConfig(t, `allow rm`)
	result, err := Evaluate(`for f in $(ls); do rm "$f"; done`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
	if !strings.Contains(result.Message, "dynamic") {
		t.Errorf("expected dynamic message for dynamic word list, got: %s", result.Message)
	}
}

// TestEvaluateForLoopMostRestrictiveAcrossIterations verifies that the
// most-restrictive-wins rule applies across expanded iterations.
func TestEvaluateForLoopMostRestrictiveAcrossIterations(t *testing.T) {
	// `rm -rf a` might allow, `rm -rf b` might deny — but here we simply
	// deny rm entirely, so both iterations deny and the result is deny.
	cfg := mustParseConfig(t, `
allow ls
deny rm
`)
	result, err := Evaluate(`ls; for f in a b; do rm -rf "$f"; done`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateForLoopControlFlowRegressionWhile locks in that a `while` loop
// still returns deny (the Plan explicitly wants Phase 1 to leave non-for
// control-flow untouched).
func TestEvaluateForLoopControlFlowRegressionWhile(t *testing.T) {
	cfg := mustParseConfig(t, `allow rm`)
	result, err := Evaluate(`while true; do rm x; done`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateForLoopControlFlowRegressionIf locks in that `if` remains deny.
func TestEvaluateForLoopControlFlowRegressionIf(t *testing.T) {
	cfg := mustParseConfig(t, `allow rm`)
	result, err := Evaluate(`if [ -f x ]; then rm x; fi`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateForLoopControlFlowRegressionCase locks in that `case` remains
// deny.
func TestEvaluateForLoopControlFlowRegressionCase(t *testing.T) {
	cfg := mustParseConfig(t, `allow rm`)
	result, err := Evaluate(`case $X in a) rm a;; b) rm b;; esac`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateForLoopControlFlowRegressionFuncDecl locks in that function
// declarations remain deny.
func TestEvaluateForLoopControlFlowRegressionFuncDecl(t *testing.T) {
	cfg := mustParseConfig(t, `allow rm`)
	result, err := Evaluate(`myfn() { rm x; }`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

// TestEvaluateForLoopUnanalyzableActionAsk verifies that a dynamic-word-list
// for-loop with `unanalyzable_action: ask` returns ask (Phase 1 × Phase 2
// interaction).
func TestEvaluateForLoopUnanalyzableActionAsk(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  unanalyzable_action: ask
allow rm
`)
	result, err := Evaluate(`for f in $(ls); do rm "$f"; done`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionAsk)
}

func TestEvaluatePreToolUseSection(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow ls
  deny rm
`)
	result, err := Evaluate("rm foo", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

func TestEvaluateTemplateInheritance(t *testing.T) {
	cfg := mustParseConfig(t, `
template primitive
  |,>>
    allow cat, echo, head, tail, wc

template safeRead
  next: primitive

template bulkExec
  extends: safeRead
  |,>>
    deny rm  "don't pipe into destructive"

allow find
  next: bulkExec
`)
	// find | cat → allow (from primitive via safeRead via bulkExec)
	result1, err := Evaluate("find . | cat", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "find|cat action", result1.Action, dsl.ActionAllow)

	// find | rm → deny (from bulkExec)
	result2, err := Evaluate("find . | rm", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "find|rm action", result2.Action, dsl.ActionDeny)
}

func assertEqual[T comparable](t *testing.T, name string, got, expected T) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}
