package main

import (
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

// TestIsStrictConfigError_FromSettings verifies that Settings.StrictConfigError
// activates strict mode. (Plan 0006 VULN-07)
func TestIsStrictConfigError_FromSettings(t *testing.T) {
	cfg := &dsl.Config{Settings: &dsl.Settings{StrictConfigError: true}}
	if !isStrictConfigError(cfg) {
		t.Error("expected strict mode when Settings.StrictConfigError=true")
	}
}

// TestIsStrictConfigError_DefaultOff verifies fail-open remains the default.
func TestIsStrictConfigError_DefaultOff(t *testing.T) {
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "")
	cfg := &dsl.Config{Settings: dsl.DefaultSettings()}
	if isStrictConfigError(cfg) {
		t.Error("expected fail-open by default")
	}
}

// TestIsStrictConfigError_EnvVar verifies env-var opt-in when no config could
// be loaded (partial cfg has default Settings).
func TestIsStrictConfigError_EnvVar(t *testing.T) {
	cfg := &dsl.Config{Settings: dsl.DefaultSettings()}
	cases := map[string]bool{
		"1":     true,
		"true":  true,
		"TRUE":  true,
		"True ": true,
		"":      false,
		"0":     false,
		"false": false,
		"maybe": false,
	}
	for v, want := range cases {
		t.Run("env="+v, func(t *testing.T) {
			t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", v)
			if got := isStrictConfigError(cfg); got != want {
				t.Errorf("env=%q: got %v, want %v", v, got, want)
			}
		})
	}
}

// TestIsStrictConfigError_NilCfg guards against nil-pointer paths.
func TestIsStrictConfigError_NilCfg(t *testing.T) {
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "")
	if isStrictConfigError(nil) {
		t.Error("expected nil cfg to fail-open (no env opt-in)")
	}
	if isStrictConfigError(&dsl.Config{}) {
		t.Error("expected cfg with nil Settings to fail-open (no env opt-in)")
	}
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "1")
	if !isStrictConfigError(nil) {
		t.Error("env opt-in should activate strict even when cfg is nil")
	}
}
