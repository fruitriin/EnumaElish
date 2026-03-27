package eval

import (
	"strings"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
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
