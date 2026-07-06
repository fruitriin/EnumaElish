package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fruitriin/EnumaElish/internal/dsl"
)

// setupWorkspace creates a real workspace directory and returns the path.
// Some inside/outside tests need EvalSymlinks to succeed, which only happens
// on paths that actually exist.
func setupWorkspace(t *testing.T) (ws string, outside string) {
	t.Helper()
	tmp := t.TempDir()
	ws = filepath.Join(tmp, "ws")
	outside = filepath.Join(tmp, "outside")
	for _, d := range []string{ws, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	return ws, outside
}

// C1: shell write redirects (> >> >|) must be scope-visible.
func TestScopeRedirWriteDenied(t *testing.T) {
	ws, outside := setupWorkspace(t)

	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow cat
    scope:
      inside: allow
      outside-write: deny  "no writing outside workspace"
`)
	// cat inside > outside → outside-write deny
	r, err := Evaluate("cat "+ws+"/src.txt > "+outside+"/pwned.txt", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "redirect outside deny", r.Action, dsl.ActionDeny)
}

// C1: append (>>) also captured as write.
func TestScopeRedirAppendDenied(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow echo
    scope:
      inside: allow
      outside-write: deny
`)
	r, err := Evaluate("echo hi >> "+outside+"/log", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "append redirect outside deny", r.Action, dsl.ActionDeny)
}

// C1: read redirects (<) don't have a write target so must not falsely trigger deny.
func TestScopeRedirReadIgnored(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow cat
    scope:
      inside: allow
      outside-write: deny
`)
	r, err := Evaluate("cat < "+outside+"/src.txt", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	// Read redirect not tracked as write; only the (no) positional path
	// arg matters. cat has no path arg here → allow.
	assertEqual(t, "read redirect ignored", r.Action, dsl.ActionAllow)
}

// C2: cp -t DIR src1 src2 — DIR is the write target, not src2.
func TestScopeCpTargetDirectory(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow cp
    scope:
      inside: allow
      outside-read: allow
      outside-write: deny  "no writing outside workspace"
`)
	// Old (buggy) code: last path = write, so `cp -t /outside a b` would say
	// b is write and it's inside → allow. New code: /outside is write → deny.
	src1 := filepath.Join(ws, "a")
	src2 := filepath.Join(ws, "b")
	r, err := Evaluate("cp -t "+outside+" "+src1+" "+src2, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "cp -t DIR", r.Action, dsl.ActionDeny)

	// --target-directory=DIR variant
	r2, err := Evaluate("cp --target-directory="+outside+" "+src1+" "+src2, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "cp --target-directory=DIR", r2.Action, dsl.ActionDeny)

	// -tDIR glued form
	r3, err := Evaluate("cp -t"+outside+" "+src1+" "+src2, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "cp -tDIR glued", r3.Action, dsl.ActionDeny)

	// --target-directory DIR (space-separated long form)
	r4, err := Evaluate("cp --target-directory "+outside+" "+src1+" "+src2, cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "cp --target-directory DIR", r4.Action, dsl.ActionDeny)
}

// C2: same for mv.
func TestScopeMvTargetDirectory(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow mv
    scope:
      inside: allow
      outside-write: deny
`)
	r, err := Evaluate("mv -t "+outside+" "+ws+"/a", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "mv -t DIR", r.Action, dsl.ActionDeny)
}

// C3: an unknown command's outside path arg should trigger outside-write.
func TestScopeUnknownCommandWriteFires(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow rsync
    scope:
      inside: allow
      outside-write: deny  "rsync must not touch outside"
`)
	// rsync is not in PathArgSemantics → PathKindUnknown → outside-write fires.
	r, err := Evaluate("rsync -av "+ws+"/src/ "+outside+"/dst/", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "rsync outside", r.Action, dsl.ActionDeny)
}

// C3: unknown command with only inside args should NOT deny.
func TestScopeUnknownCommandInsideAllowed(t *testing.T) {
	ws, _ := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow rsync
    scope:
      inside: allow
      outside-write: deny
`)
	r, err := Evaluate("rsync -av "+ws+"/src/ "+ws+"/dst/", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "rsync inside only", r.Action, dsl.ActionAllow)
}

// C4: dynamic $(...) path arg must not be silently dropped.
func TestScopeDynamicPathTreatedAsOutsideForWrite(t *testing.T) {
	ws, _ := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow cp
    scope:
      inside: allow
      outside-write: deny  "cannot write outside"
`)
	// cp /ws/x $(echo /elsewhere)/y — the dst is dynamic. Old code:
	// $(echo …) skipped → only src was scope-checked → allow. New:
	// dynamic → outside → outside-write triggers → deny.
	r, err := Evaluate("cp "+ws+"/src $(echo /elsewhere)/dst", cfg)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	assertEqual(t, "cp dynamic dst", r.Action, dsl.ActionDeny)
}

// C6: ScopeRule on Write tool should be honored — outside-write: deny beats ask.
func TestScopeToolWriteOutsideDeny(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow Write
    scope:
      inside: allow
      outside-write: deny  "no external writes"
`)
	// Write inside → allow
	r1 := EvaluateTool("Write", ws+"/README.md", cfg)
	assertEqual(t, "Write inside", r1.Action, dsl.ActionAllow)

	// Write outside → deny (not just ask)
	r2 := EvaluateTool("Write", outside+"/pwned", cfg)
	assertEqual(t, "Write outside", r2.Action, dsl.ActionDeny)
}

// C6: ScopeRule on Read tool with outside-read shouldn't fire outside-write.
func TestScopeToolReadOutsideRead(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow Read
    scope:
      inside: allow
      outside-read: ask  "reading outside — confirm?"
      outside-write: deny
`)
	r := EvaluateTool("Read", outside+"/notes", cfg)
	assertEqual(t, "Read outside → ask (not deny)", r.Action, dsl.ActionAsk)
}

// C6: MCP tool (PathKindUnknown) should honor outside-write to block by default.
func TestScopeToolMCPUnknownWrite(t *testing.T) {
	ws, outside := setupWorkspace(t)
	cfg := mustParseConfig(t, `
settings:
  workspace: `+ws+`

preToolUse
  allow mcp__filesystem__write_file
    scope:
      inside: allow
      outside-write: deny
`)
	r := EvaluateTool("mcp__filesystem__write_file", outside+"/x", cfg)
	assertEqual(t, "MCP write outside", r.Action, dsl.ActionDeny)
}
