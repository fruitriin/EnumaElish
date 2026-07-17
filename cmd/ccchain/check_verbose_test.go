package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCheckFixture writes a .ccchain.conf into t.TempDir() and returns the
// path. t.TempDir() cleans itself up so no scratch survives the run.
func writeCheckFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".ccchain.conf")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestRunCheckVerboseIncludesSettingsDigest guards Plan 0028 Phase 2:
// `ccchain check -v` must surface the effective settings block so an operator
// can see fallback / ask_strategy / ask_degrade_default / unanalyzable_action
// / scope_violation / strict_config_error in one place. Non-verbose runs must
// stay quiet (no digest) — verbose is opt-in.
func TestRunCheckVerboseIncludesSettingsDigest(t *testing.T) {
	// A conf that flexes several settings so we can assert each one landed
	// with the value the parser saw (not defaults).
	conf := `settings:
  fallback: deny
  ask_strategy: passthrough
  ask_degrade_default: allow
  unanalyzable_action: ask
  scope_violation: deny
  strict_config_error: true
  max_context_depth: 3
  max_rules_per_cmd: 7
  workspace: /tmp/ws, /tmp/ws2

allow echo
`
	path := writeCheckFixture(t, conf)

	var stdout, stderr bytes.Buffer
	if err := runCheckWithWriters(&stdout, &stderr, path, true, false); err != nil {
		t.Fatalf("runCheckWithWriters: %v", err)
	}

	out := stdout.String()
	// Structural markers.
	if !strings.Contains(out, "config OK:") {
		t.Errorf("missing config OK header in stdout: %q", out)
	}
	if !strings.Contains(out, "  settings:") {
		t.Errorf("verbose stdout should include settings digest header: %q", out)
	}

	// Every knob's value must appear in the digest.
	wants := []string{
		"fallback:            deny",
		"ask_strategy:        passthrough",
		"ask_degrade_default: allow",
		"unanalyzable_action: ask",
		"scope_violation:     deny",
		"strict_config_error: true",
		"max_context_depth:   3",
		"max_rules_per_cmd:   7",
		"workspace:           [/tmp/ws /tmp/ws2]",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("settings digest missing %q\nfull stdout:\n%s", w, out)
		}
	}
}

// TestRunCheckDefaultVerboseShowsDefaults asserts that when the operator writes
// no `settings:` block, the digest still reports the effective defaults (from
// DefaultSettings via mergeConfigs) rather than blanks. This is what makes the
// digest useful — the operator can see *why* their ask degrades to deny even
// though they never spelled out `ask_degrade_default`.
func TestRunCheckDefaultVerboseShowsDefaults(t *testing.T) {
	conf := "allow echo\n"
	path := writeCheckFixture(t, conf)

	var stdout, stderr bytes.Buffer
	if err := runCheckWithWriters(&stdout, &stderr, path, true, false); err != nil {
		t.Fatalf("runCheckWithWriters: %v", err)
	}
	out := stdout.String()
	// Defaults per dsl.DefaultSettings().
	wants := []string{
		"fallback:            ask",
		"ask_strategy:        degrade",
		"ask_degrade_default: deny",
		"unanalyzable_action: deny",
		"scope_violation:     ask",
		"strict_config_error: false",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("default digest missing %q\nfull stdout:\n%s", w, out)
		}
	}
}

// TestRunCheckNonVerboseHidesSettingsDigest guards that the digest is verbose-
// only. In quiet-ish runs the historical single-line output stays intact so
// existing scripts that grep for "config OK" don't accidentally match settings
// lines.
func TestRunCheckNonVerboseHidesSettingsDigest(t *testing.T) {
	conf := "allow echo\n"
	path := writeCheckFixture(t, conf)

	var stdout, stderr bytes.Buffer
	if err := runCheckWithWriters(&stdout, &stderr, path, false, false); err != nil {
		t.Fatalf("runCheckWithWriters: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "config OK:") {
		t.Errorf("non-verbose still needs the OK header: %q", out)
	}
	if strings.Contains(out, "  settings:") {
		t.Errorf("non-verbose stdout must not include settings digest: %q", out)
	}
}

// TestRunCheckQuietSuppressesAll asserts that --quiet suppresses both the OK
// header and the settings digest. The Issue #15 warning going to stderr is
// preserved for the fallback:ask + degrade combo, but --quiet on a benign
// config should print nothing at all on stdout.
func TestRunCheckQuietSuppressesAll(t *testing.T) {
	// A conf that would NOT trigger the Issue #15 stderr warning
	// (fallback: allow keeps stderr silent).
	conf := "settings:\n  fallback: allow\n\nallow echo\n"
	path := writeCheckFixture(t, conf)

	var stdout, stderr bytes.Buffer
	if err := runCheckWithWriters(&stdout, &stderr, path, true, true); err != nil {
		t.Fatalf("runCheckWithWriters: %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("quiet mode must produce no stdout, got %q", stdout.String())
	}
}
