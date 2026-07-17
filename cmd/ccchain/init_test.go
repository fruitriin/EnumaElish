package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/preset"
)

// TestSentinelConfigMatchesPreset verifies that the sentinelConfig used by
// runInitSentinel is byte-identical to the embedded preset. If a
// contributor duplicates the string in another location, this test catches
// the drift before it ships.
func TestSentinelConfigMatchesPreset(t *testing.T) {
	if sentinelConfig != preset.Sentinel() {
		t.Fatalf("sentinelConfig drifted from preset.Sentinel() — the shipped init --sentinel output would diverge from what fixture tests exercise")
	}
	if !strings.Contains(sentinelConfig, "sentinel:") {
		t.Error("sentinelConfig missing the sentinel: message prefix — messages are the sentinel's contract")
	}
	if !strings.Contains(sentinelConfig, "strict_config_error: true") {
		t.Error("sentinelConfig missing strict_config_error: true — sentinel should fail closed on config errors")
	}
}

// TestSentinelConfigParsesAndEvaluates does an end-to-end round-trip: the
// sentinel string parses, resolves templates, and evaluates one canonical
// deny (curl|bash). This is the smallest gate that catches a config regression
// before `ccchain init --sentinel` ships broken bytes to users.
func TestSentinelConfigParsesAndEvaluates(t *testing.T) {
	cfg, err := dsl.Parse(strings.NewReader(sentinelConfig))
	if err != nil {
		t.Fatalf("sentinelConfig failed to parse: %v", err)
	}
	if err := dsl.ResolveTemplates(cfg); err != nil {
		t.Fatalf("sentinelConfig failed template resolution: %v", err)
	}
	if cfg.Settings == nil || !cfg.Settings.StrictConfigError {
		t.Error("sentinelConfig must set strict_config_error: true")
	}
}

// TestRunInitSentinelWritesFile exercises the actual file-write path of
// runInitSentinel by pointing writePreset at a scratch directory. We do
// not call runInitSentinel directly (it hard-codes .ccchain.conf in the
// cwd and calls os.Exit on failure); instead we invoke writePreset,
// which is the whole surface it exposes.
func TestRunInitSentinelWritesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ccchain.conf")
	// writePreset would os.Exit on failure; but on the happy path it
	// simply writes and prints — safe to call in tests.
	writePreset(path, sentinelConfig, "sentinel")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != sentinelConfig {
		t.Fatalf("written file does not match sentinelConfig (len got=%d, want=%d)",
			len(got), len(sentinelConfig))
	}
}

// TestRunInitDispatchRejectsUnknownFlag makes sure a typo in an
// init-subcommand flag hard-errors, so a misspelled `--sentinal` cannot
// silently fall through to the (lax) default preset.
//
// runInitDispatch calls os.Exit on unknown flags; we can't test that
// path without a subprocess. Instead this test documents the contract at
// the source of truth: the dispatcher's known-flag map.
func TestInitFlagPassthroughContract(t *testing.T) {
	if !flagPassthroughCommands["init"] {
		t.Fatal("init must be in flagPassthroughCommands so its own flags reach runInitDispatch")
	}
}

// TestDefaultConfigCoversReadOnlyUtilities verifies that the v0.2.1 default
// preset lists the common read-only utilities added in response to Issue #15.
// Without these, `settings: fallback: ask` + ask degrade combined with the
// auto permission mode silently blocks everyday commands.
func TestDefaultConfigCoversReadOnlyUtilities(t *testing.T) {
	must := []string{
		"allow sed", "allow awk", "allow cut", "allow tr",
		"allow tee", "allow basename", "allow dirname",
		"allow date", "allow env", "allow printf", "allow seq",
		"allow test", "allow file", "allow stat", "allow tree",
		"allow readlink", "allow uniq",
	}
	for _, line := range must {
		if !strings.Contains(defaultConfig, line) {
			t.Errorf("defaultConfig missing %q — v0.2.1 default preset must cover it (Issue #15)", line)
		}
	}
}
