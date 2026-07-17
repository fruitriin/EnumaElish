package eval

import (
	"strings"
	"testing"

	"github.com/fruitriin/EnumaElish/internal/dsl"
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
// result is escalated to the strictest action in the ArgsRules block. For a
// block whose strictest action is ask, that means ask (Plan 0009 Phase 3,
// Security Info; attacker-persona finding C3).
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

// TestArgsMaxLenParentAllowDenyPreserved pins the C3 fix: when a rule's
// ArgsRules block contains a `deny` entry, over-length input must still deny
// — a plain escalation to ask would be a de-escalation of deny → ask and let
// `allow rm` + `args: -rf /: deny` be bypassed by argument padding.
func TestArgsMaxLenParentAllowDenyPreserved(t *testing.T) {
	cfg := mustParseConfig(t, `
allow rm
  args:
    ^-rf /$: deny  "rm -rf / is not allowed"
`)
	padding := strings.Repeat("A", maxArgsLen+1)
	r, err := Evaluate("rm -rf / "+padding, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "over-length action must stay deny", r.Action, dsl.ActionDeny)
}

// TestArgsMatch_PreservesUnattended is skeptic C3's regression guard: when a
// parent rule declares `ask ... unattended: deny` and an args: sub-rule
// escalates the action, the escalated Result must retain the parent's
// Unattended so ResolveAsk still degrades in the intended direction.
//
// Scenario: `ask docker unattended: deny` + `args: --privileged: ask` (still
// ask, but a different message). ResolveAsk under auto mode must land on
// deny because of the parent's `unattended: deny`, not on ask_degrade_default.
func TestArgsMatch_PreservesUnattended(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  ask_degrade_default: allow

ask docker
  message: "confirm docker op"
  unattended: deny
  args:
    --privileged: ask "docker --privileged requires review"
`)
	r, err := Evaluate("docker run --privileged image", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "action before ResolveAsk", r.Action, dsl.ActionAsk)
	assertEqual(t, "Unattended must be inherited from parent rule", r.Unattended, dsl.ActionDeny)

	// Now verify the composed pipeline: ResolveAsk under auto must deny.
	resolved := ResolveAsk(r, "auto", cfg.Settings)
	assertEqual(t, "auto mode with args match still degrades to deny", resolved.Action, dsl.ActionDeny)
}

// TestArgsMaxLenPicksStrictestOfArgsBlock: when multiple ArgsRules of
// differing strictness are present, over-length input is escalated to the
// strictest one — not merely ask.
func TestArgsMaxLenPicksStrictestOfArgsBlock(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    -X GET: allow
    -X POST: ask
    ^--data.*password: deny
`)
	padding := strings.Repeat("A", maxArgsLen+1)
	r, err := Evaluate("curl -X GET "+padding, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "over-length action picks deny", r.Action, dsl.ActionDeny)
}

// TestArgsMaxLenAllowOnlyBlockAsks: a block whose entries are all `allow`
// still escalates to ask on over-length — the floor is ask so an all-allow
// block cannot be used to silently permit unverifiable input.
func TestArgsMaxLenAllowOnlyBlockAsks(t *testing.T) {
	cfg := mustParseConfig(t, `
allow curl
  args:
    ^https://safe\.example\.com/: allow
`)
	padding := strings.Repeat("A", maxArgsLen+1)
	r, err := Evaluate("curl https://safe.example.com/"+padding, cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	assertEqual(t, "over-length all-allow block asks", r.Action, dsl.ActionAsk)
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
// A `deny` entry in the ArgsRules block means over-length input denies —
// falling back to ask would let padding bypass the deny (attacker C3).
func TestToolArgsMaxLen(t *testing.T) {
	cfg := mustParseConfig(t, `
allow WebFetch
  args:
    ^https://example\.com/: deny
`)
	longURL := "https://example.com/" + strings.Repeat("A", maxArgsLen)
	r := EvaluateTool("WebFetch", longURL, cfg)
	assertEqual(t, "tool over-length action must stay deny", r.Action, dsl.ActionDeny)
}

// TestToolArgsMaxLenAskOnlyEscalatesToAsk: for an EvaluateTool rule whose
// strictest args entry is ask, over-length input escalates to ask.
func TestToolArgsMaxLenAskOnlyEscalatesToAsk(t *testing.T) {
	cfg := mustParseConfig(t, `
allow WebFetch
  args:
    ^https://example\.com/: ask
`)
	longURL := "https://example.com/" + strings.Repeat("A", maxArgsLen)
	r := EvaluateTool("WebFetch", longURL, cfg)
	assertEqual(t, "tool over-length ask-only escalates to ask", r.Action, dsl.ActionAsk)
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
