package eval

import (
	"strings"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

func TestArgsBasicMatch(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: ask
    -X DELETE: deny  "DELETE not allowed"
`)
	// GET → no args match → allow (parent rule)
	r1, err := Evaluate("curl -X GET https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "GET action", r1.Action, dsl.ActionAllow)

	// POST → args match → ask
	r2, err := Evaluate("curl -X POST https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "POST action", r2.Action, dsl.ActionAsk)

	// DELETE → args match → deny
	r3, err := Evaluate("curl -X DELETE https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "DELETE action", r3.Action, dsl.ActionDeny)
}

func TestArgsLastRuleWins(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X: ask
    -X GET: allow
`)
	// -X GET matches both patterns, last wins → allow
	r, err := Evaluate("curl -X GET https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", r.Action, dsl.ActionAllow)
}

func TestArgsDynamicSkip(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: deny
`)
	// Dynamic argument → args: evaluation skipped → allow (parent)
	r, err := Evaluate("curl -X $METHOD https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "dynamic action", r.Action, dsl.ActionAllow)
}

func TestArgsEmptyBlock(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
`)
	// No args: block → parent action
	r, err := Evaluate("curl -X POST https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", r.Action, dsl.ActionAllow)
}

// smell-allow: ignored-test — parse error means the invalid regex was caught at parse time, which is acceptable
func TestArgsInvalidRegex(t *testing.T) {
	input := `
allow curl
  args:
    [invalid: deny
`
	_, err := dsl.Parse(strings.NewReader(input))
	if err != nil {
		t.Skip("parse error is also acceptable for invalid regex input")
	}
	// If parsing succeeds, ResolveTemplates should fail
	cfg, _ := dsl.Parse(strings.NewReader(input))
	err = dsl.ResolveTemplates(cfg)
	if err == nil {
		t.Error("expected error for invalid regex pattern, got nil")
	}
}

func TestArgsNoMatchFallsThrough(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: deny
`)
	// No match on args → parent action (allow)
	r, err := Evaluate("curl https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action", r.Action, dsl.ActionAllow)
}
