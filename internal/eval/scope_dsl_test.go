package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fruitriin/EnumaElish/internal/dsl"
)

// TestScopeDSLOutsideReadWrite verifies that a rule's `scope:` block with
// `outside-read` and `outside-write` correctly distinguishes cp's src (read)
// from dst (write) via the semantics table (Plan 0011 v2).
func TestScopeDSLOutsideReadWrite(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outsideDir := filepath.Join(tmp, "outside")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	insideFile := filepath.Join(ws, "src.txt")
	outsideFile := filepath.Join(outsideDir, "src.txt")
	insideDst := filepath.Join(ws, "dst.txt")
	outsideDst := filepath.Join(outsideDir, "dst.txt")

	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`
  fallback: ask

preToolUse
  allow cp
    scope:
      inside: allow
      outside-read: allow  "read outside is fine"
      outside-write: deny  "cannot write outside workspace"
`)

	// cp inside → inside: both allow
	r1, err := Evaluate("cp "+insideFile+" "+insideDst, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "inside→inside", r1.Action, dsl.ActionAllow)

	// cp outside_read → inside: outside-read allow, inside allow → allow
	r2, err := Evaluate("cp "+outsideFile+" "+insideDst, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "outside-read→inside", r2.Action, dsl.ActionAllow)

	// cp inside → outside: outside-write deny → deny
	r3, err := Evaluate("cp "+insideFile+" "+outsideDst, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "inside→outside-write", r3.Action, dsl.ActionDeny)

	// cp outside → outside: outside-write deny (most restrictive) → deny
	r4, err := Evaluate("cp "+outsideFile+" "+outsideDst, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "outside→outside", r4.Action, dsl.ActionDeny)
}

// TestScopeDSLOutsideOnly verifies the legacy shorthand: writing only
// `outside:` applies to both read and write (backward compatibility).
func TestScopeDSLOutsideOnly(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outsideFile := filepath.Join(tmp, "outside.txt")

	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`
  fallback: ask

preToolUse
  allow cat
    scope:
      inside: allow
      outside: ask  "please confirm outside access"
`)
	// cat outside → outside: ask
	r, err := Evaluate("cat "+outsideFile, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "cat outside via outside:", r.Action, dsl.ActionAsk)
	if r.Message != "please confirm outside access" {
		t.Errorf("expected custom message, got %q", r.Message)
	}
}

// TestScopeDSLOutsideAllowOptOut verifies that `outside: allow` explicitly
// opts out of the automatic escalation (e.g. `ls` is safe anywhere).
func TestScopeDSLOutsideAllowOptOut(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(tmp, "elsewhere")

	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow ls
    scope:
      inside: allow
      outside: allow
`)
	r, err := Evaluate("ls "+outside, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "ls outside with outside:allow", r.Action, dsl.ActionAllow)
}

// TestScopeDSLRmAllWrite verifies that rm's arguments are all classified as
// writes so that `outside-write: deny` blocks removing outside files.
func TestScopeDSLRmAllWrite(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outsideFile := filepath.Join(tmp, "victim.txt")

	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow rm
    scope:
      inside: ask  "confirm deletion"
      outside-write: deny  "cannot delete outside workspace"
`)
	r, err := Evaluate("rm "+outsideFile, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "rm outside", r.Action, dsl.ActionDeny)
}

// TestScopeDSLLegacyBehaviorNoScope verifies that rules without `scope:` blocks
// keep the pre-v2 auto-escalation behavior (allow → ask outside).
func TestScopeDSLLegacyBehaviorNoScope(t *testing.T) {
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(tmp, "outside.txt")

	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow cat
`)
	r, err := Evaluate("cat "+outside, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "legacy escalation", r.Action, dsl.ActionAsk)
}
