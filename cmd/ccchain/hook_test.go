package main

import (
	"errors"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

var errLoadFailure = errors.New("simulated load failure")

// TestIsStrictConfigError_FromSettings verifies that Settings.StrictConfigError
// activates strict mode regardless of loadErr. (Plan 0006 VULN-07)
func TestIsStrictConfigError_FromSettings(t *testing.T) {
	cfg := &dsl.Config{Settings: &dsl.Settings{StrictConfigError: true}}
	if !isStrictConfigError(cfg, errLoadFailure) {
		t.Error("expected strict mode when Settings.StrictConfigError=true (with loadErr)")
	}
	if !isStrictConfigError(cfg, nil) {
		t.Error("expected strict mode when Settings.StrictConfigError=true (no loadErr)")
	}
}

// TestIsStrictConfigError_DefaultOff verifies fail-open remains the default
// when the config loaded successfully.
func TestIsStrictConfigError_DefaultOff(t *testing.T) {
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "")
	cfg := &dsl.Config{Settings: dsl.DefaultSettings()}
	if isStrictConfigError(cfg, nil) {
		t.Error("expected fail-open by default")
	}
}

// TestIsStrictConfigError_EnvVarOnlyWhenLoadErr verifies that the env-var
// opt-in ONLY activates when loadErr is non-nil (documented contract:
// "the only opt-in path when no config file could be read at all").
func TestIsStrictConfigError_EnvVarOnlyWhenLoadErr(t *testing.T) {
	cfg := &dsl.Config{Settings: dsl.DefaultSettings()}
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "1")
	if isStrictConfigError(cfg, nil) {
		t.Error("env-var must NOT activate strict mode when config loaded cleanly")
	}
	if !isStrictConfigError(cfg, errLoadFailure) {
		t.Error("env-var should activate strict mode when loadErr is non-nil")
	}
}

// TestIsStrictConfigError_EnvVar verifies env-var value parsing (only when
// loadErr is non-nil per the documented opt-in path).
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
			if got := isStrictConfigError(cfg, errLoadFailure); got != want {
				t.Errorf("env=%q with loadErr: got %v, want %v", v, got, want)
			}
		})
	}
}

// TestIsStrictConfigError_ExplicitFalseVetoesEnv verifies that when the loaded
// partial config explicitly sets strict_config_error=false, env is ignored
// (the user has taken a stance).
func TestIsStrictConfigError_ExplicitFalseVetoesEnv(t *testing.T) {
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "1")
	s := dsl.DefaultSettings()
	s.StrictConfigError = false
	s.Explicit["strict_config_error"] = true
	cfg := &dsl.Config{Settings: s}
	if isStrictConfigError(cfg, errLoadFailure) {
		t.Error("explicit strict_config_error=false should veto env-var opt-in")
	}
}

// TestIsStrictConfigError_NilCfg guards against nil-pointer paths.
func TestIsStrictConfigError_NilCfg(t *testing.T) {
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "")
	if isStrictConfigError(nil, errLoadFailure) {
		t.Error("expected nil cfg to fail-open (no env opt-in)")
	}
	if isStrictConfigError(&dsl.Config{}, errLoadFailure) {
		t.Error("expected cfg with nil Settings to fail-open (no env opt-in)")
	}
	t.Setenv("CCCHAIN_STRICT_CONFIG_ERROR", "1")
	if !isStrictConfigError(nil, errLoadFailure) {
		t.Error("env opt-in should activate strict when cfg is nil and loadErr set")
	}
	if isStrictConfigError(nil, nil) {
		t.Error("env opt-in must NOT activate when loadErr is nil (no load failure)")
	}
}
