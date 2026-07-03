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

// TestArgsQuotedMatch pins the guarantee that shell quoting is removed before
// args: pattern matching, exactly as the shell removes it before exec.
// `curl -X "POST"`, `curl -X 'POST'`, and `curl -X PO"ST"` must all match the
// pattern `-X POST` (Plan 0009 Phase 3, Security Low).
func TestArgsQuotedMatch(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: ask
`)
	cases := []struct {
		name string
		cmd  string
		want dsl.Action
	}{
		{"double-quoted value", `curl -X "POST" https://example.com`, dsl.ActionAsk},
		{"single-quoted value", `curl -X 'POST' https://example.com`, dsl.ActionAsk},
		{"partially quoted value", `curl -X PO"ST" https://example.com`, dsl.ActionAsk},
		{"fully quoted flag and value", `curl "-X" "POST" https://example.com`, dsl.ActionAsk},
		{"unquoted control", `curl -X POST https://example.com`, dsl.ActionAsk},
		{"non-matching method", `curl -X "GET" https://example.com`, dsl.ActionAllow},
	}
	for _, tc := range cases {
		r, err := Evaluate(tc.cmd, cfg)
		if err != nil {
			t.Fatalf("%s: evaluate error: %v", tc.name, err)
		}
		assertEqual(t, tc.name, r.Action, tc.want)
	}
}

// TestArgsQuotedDynamicStillSkipped: quote removal must not hide dynamic
// expansion — `curl -X "$METHOD"` keeps its $ marker and skips args: rules.
func TestArgsQuotedDynamicStillSkipped(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: deny
`)
	r, err := Evaluate(`curl -X "$METHOD" https://example.com`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "quoted dynamic action", r.Action, dsl.ActionAllow)
}

// TestQuotedCommandNameMatchesRule: quote removal also applies to the command
// name itself, so `"rm"` cannot evade a rule written for `rm`.
func TestQuotedCommandNameMatchesRule(t *testing.T) {
	cfg := mustParseConfig(t, `
deny rm  "rm is not allowed"
`)
	r, err := Evaluate(`"rm" -rf /tmp/x`, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "quoted name action", r.Action, dsl.ActionDeny)
}

// TestArgsMaxLenEscalatesToAsk: when the joined argument string exceeds
// maxArgsLen, args: rules cannot be applied safely. Falling back to the
// parent action would let padding bypass an escalating args: rule, so the
// result is escalated to ask instead (Plan 0009 Phase 3, Security Info).
func TestArgsMaxLenEscalatesToAsk(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: ask
`)
	padding := strings.Repeat("A", maxArgsLen+1)
	r, err := Evaluate("curl -X POST -H "+padding+" https://example.com", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "over-length action", r.Action, dsl.ActionAsk)
}

// TestArgsMaxLenKeepsStricterParent: an already stricter parent action (deny)
// is kept when the argument string is over-length — the limit never
// de-escalates.
func TestArgsMaxLenKeepsStricterParent(t *testing.T) {
	cfg := mustParseConfig(t, `
deny rm
  args:
    ^-i$: allow
`)
	padding := strings.Repeat("A", maxArgsLen+1)
	r, err := Evaluate("rm "+padding, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "over-length deny kept", r.Action, dsl.ActionDeny)
}

// TestArgsMaxLenBoundary: exactly maxArgsLen bytes is still evaluated
// normally; the limit only triggers strictly above it.
func TestArgsMaxLenBoundary(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X POST: deny
`)
	// "-X GET " is 7 bytes; pad to exactly maxArgsLen total
	padding := strings.Repeat("A", maxArgsLen-7)
	r, err := Evaluate("curl -X GET "+padding, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "at-limit action", r.Action, dsl.ActionAllow)
}

// TestArgsMaxLenWithoutArgsRules: rules without an args: block are unaffected
// by the length limit — the parent action passes through unchanged.
func TestArgsMaxLenWithoutArgsRules(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
`)
	padding := strings.Repeat("A", maxArgsLen+1)
	r, err := Evaluate("curl "+padding, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "no-args-rules action", r.Action, dsl.ActionAllow)
}

// TestToolArgsMaxLen: the same limit protects EvaluateTool's args: matching.
func TestToolArgsMaxLen(t *testing.T) {
	cfg := mustParseConfig(t, `
allow WebFetch
  args:
    ^https://example\.com/: deny
`)
	longURL := "https://example.com/" + strings.Repeat("A", maxArgsLen)
	r := EvaluateTool("WebFetch", longURL, cfg)
	assertEqual(t, "tool over-length action", r.Action, dsl.ActionAsk)
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
