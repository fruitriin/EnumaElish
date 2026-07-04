package dsl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadConfig_PartialOnError verifies that when a later config file in the
// search path fails to parse, LoadConfig still returns the merged partial
// Config from files that loaded successfully, alongside the error. This lets
// the hook layer inspect Settings.StrictConfigError to decide fail-open vs
// fail-closed behavior. (Plan 0006 VULN-07)
func TestLoadConfig_PartialOnError(t *testing.T) {
	dir := t.TempDir()

	// Simulate a "global" config that turns on strict mode
	globalDir := filepath.Join(dir, "claude")
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	globalPath := filepath.Join(globalDir, "ccchain.conf")
	if err := os.WriteFile(globalPath, []byte("settings:\n  strict_config_error: true\n"), 0o644); err != nil {
		t.Fatalf("write global: %v", err)
	}

	// Simulate a broken project-local .ccchain.conf
	projectPath := filepath.Join(dir, ".ccchain.conf")
	if err := os.WriteFile(projectPath, []byte("this is not valid dsl @!#\n"), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}

	// Point search paths at our tempdir via env + cwd
	t.Setenv("CLAUDE_CONFIG_DIR", globalDir)
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldwd)

	cfg, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected error from broken project config, got nil")
	}
	if cfg == nil {
		t.Fatal("expected non-nil partial config, got nil")
	}
	if cfg.Settings == nil {
		t.Fatal("expected non-nil Settings on partial config")
	}
	if !cfg.Settings.StrictConfigError {
		t.Errorf("expected StrictConfigError=true from global config; got false. err=%v", err)
	}
}

// TestLoadConfig_NoConfigStillDefaults verifies that when no config files
// exist at all, LoadConfig returns default Settings and no error.
func TestLoadConfig_NoConfigStillDefaults(t *testing.T) {
	dir := t.TempDir()
	globalDir := filepath.Join(dir, "claude") // exists but no conf inside
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", globalDir)
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldwd)

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Settings == nil {
		t.Fatal("expected default settings")
	}
	if cfg.Settings.StrictConfigError {
		t.Error("expected StrictConfigError=false by default")
	}
}

// TestLoadConfig_ExplicitPathErrorReturnsPartial verifies that when an
// explicit configPath fails to parse, LoadConfig still returns a non-nil
// Config with default Settings so callers may fall back on env-var opt-in.
func TestLoadConfig_ExplicitPathErrorReturnsPartial(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.conf")
	if err := os.WriteFile(broken, []byte("garbage @!\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadConfig(broken)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if cfg == nil || cfg.Settings == nil {
		t.Fatal("expected non-nil partial config with defaults")
	}
	if err.Error() == "" {
		t.Errorf("expected non-empty error message, got empty")
	}
}

// TestMergeConfigsFieldWise_LocalOverridesFallbackOnly verifies that a local
// config that only touches `fallback` does NOT clobber other Settings fields
// (workspace, strict_config_error, etc.) set in the base. (Plan 0006 C5)
func TestMergeConfigsFieldWise_LocalOverridesFallbackOnly(t *testing.T) {
	base, err := Parse(strings.NewReader("settings:\n  workspace: /protected\n  strict_config_error: true\n  fallback: deny\n"))
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	overlay, err := Parse(strings.NewReader("settings:\n  fallback: ask\n"))
	if err != nil {
		t.Fatalf("parse overlay: %v", err)
	}

	merged := mergeConfigs(base, overlay)

	if merged.Settings == nil {
		t.Fatal("merged.Settings is nil")
	}
	if len(merged.Settings.WorkspacePaths) != 1 || merged.Settings.WorkspacePaths[0] != "/protected" {
		t.Errorf("workspace was clobbered: got %v, want [/protected]", merged.Settings.WorkspacePaths)
	}
	if !merged.Settings.StrictConfigError {
		t.Errorf("strict_config_error was clobbered: got false, want true")
	}
	if merged.Settings.Fallback != ActionAsk {
		t.Errorf("fallback: got %v, want ask", merged.Settings.Fallback)
	}
}

// TestMergeConfigsFieldWise_LocalCanOverrideExplicitField verifies that when
// the overlay explicitly sets a field, it wins.
func TestMergeConfigsFieldWise_LocalCanOverrideExplicitField(t *testing.T) {
	base, err := Parse(strings.NewReader("settings:\n  strict_config_error: true\n"))
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	overlay, err := Parse(strings.NewReader("settings:\n  strict_config_error: false\n"))
	if err != nil {
		t.Fatalf("parse overlay: %v", err)
	}

	merged := mergeConfigs(base, overlay)

	if merged.Settings.StrictConfigError {
		t.Error("expected overlay to override strict_config_error to false")
	}
	if !merged.Settings.Explicit["strict_config_error"] {
		t.Error("merged Explicit should still record strict_config_error as explicitly set")
	}
}

// TestMergeConfigsFieldWise_DefaultUntouched verifies that fields untouched by
// either config keep their default values.
func TestMergeConfigsFieldWise_DefaultUntouched(t *testing.T) {
	base, err := Parse(strings.NewReader("settings:\n  fallback: deny\n"))
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	overlay, err := Parse(strings.NewReader("settings:\n  workspace: /x\n"))
	if err != nil {
		t.Fatalf("parse overlay: %v", err)
	}

	merged := mergeConfigs(base, overlay)

	if merged.Settings.MaxContextDepth != 2 {
		t.Errorf("MaxContextDepth: got %d, want default 2", merged.Settings.MaxContextDepth)
	}
	if merged.Settings.MaxRulesPerCmd != 5 {
		t.Errorf("MaxRulesPerCmd: got %d, want default 5", merged.Settings.MaxRulesPerCmd)
	}
	if merged.Settings.Fallback != ActionDeny {
		t.Errorf("Fallback: got %v, want deny (from base)", merged.Settings.Fallback)
	}
	if len(merged.Settings.WorkspacePaths) != 1 || merged.Settings.WorkspacePaths[0] != "/x" {
		t.Errorf("WorkspacePaths: got %v, want [/x]", merged.Settings.WorkspacePaths)
	}
}

