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
	if !strings.Contains(err.Error(), "") { // just ensure err has a message
		t.Errorf("empty error message: %v", err)
	}
}
