package eval

import (
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

// VULN-02: Absolute path must not bypass deny rules
func TestAbsolutePathBypass(t *testing.T) {
	cfg := mustParseConfig(t, `deny rm`)

	tests := []struct {
		name string
		cmd  string
	}{
		{"bare rm", "rm foo"},
		{"absolute /bin/rm", "/bin/rm foo"},
		{"absolute /usr/bin/rm", "/usr/bin/rm foo"},
		{"relative ./rm", "./rm foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.cmd, cfg)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}
			if result.Action != dsl.ActionDeny {
				t.Errorf("expected deny for %q, got %v", tt.cmd, result.Action)
			}
		})
	}
}

// VULN-01: Control flow must be denied (not fallback to allow)
func TestControlFlowDenied(t *testing.T) {
	cfg := mustParseConfig(t, `
allow ls
settings:
  fallback: allow
`)

	tests := []struct {
		name string
		cmd  string
	}{
		{"for loop", "for f in *; do rm $f; done"},
		{"if statement", "if true; then rm -rf /; fi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.cmd, cfg)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}
			if result.Action != dsl.ActionDeny {
				t.Errorf("expected deny for control flow %q, got %v", tt.cmd, result.Action)
			}
		})
	}
}

// VULN-05: env/sudo must be evaluated against nested command rules
func TestEnvSudoEvaluation(t *testing.T) {
	cfg := mustParseConfig(t, `
deny rm
allow env
allow sudo
`)

	tests := []struct {
		name string
		cmd  string
	}{
		{"env rm", "env rm foo"},
		{"sudo rm", "sudo rm -rf /"},
		{"env VAR=val rm", "env FOO=bar rm foo"},
		{"sudo -u root rm", "sudo -u root rm -rf /"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.cmd, cfg)
			if err != nil {
				t.Fatalf("evaluate error: %v", err)
			}
			if result.Action != dsl.ActionDeny {
				t.Errorf("expected deny for %q, got %v (message: %s)", tt.cmd, result.Action, result.Message)
			}
		})
	}
}

func mustParseConfigSec(t *testing.T, input string) *dsl.Config {
	return mustParseConfig(t, input)
}
