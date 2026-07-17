package dsl

import (
	"os"
	"strings"
	"testing"
)

func parseBasicRulesFixture(t *testing.T) *Config {
	t.Helper()
	f, err := os.Open("../../testdata/dsl/basic_rules.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(cfg.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(cfg.Rules))
	}
	return cfg
}

func TestParseBasicRules_AllowFind(t *testing.T) {
	cfg := parseBasicRulesFixture(t)
	r := cfg.Rules[0]
	assertEqual(t, "action", string(r.Action), "allow")
	assertEqual(t, "commands[0]", r.Commands[0], "find")
	if len(r.PipeRules) != 2 {
		t.Fatalf("expected 2 pipe rules for find, got %d", len(r.PipeRules))
	}
	if len(r.ExecRules) != 2 {
		t.Fatalf("expected 2 exec rules for find, got %d", len(r.ExecRules))
	}

	pr := r.PipeRules[1]
	assertEqual(t, "pipe.action", string(pr.Action), "deny")
	assertEqual(t, "pipe.commands[0]", pr.Commands[0], "rm")
	assertEqual(t, "pipe.message", pr.Message, "don't combine find with rm")

	er := r.ExecRules[0]
	assertEqual(t, "exec.action", string(er.Action), "deny")
	assertEqual(t, "exec.message", er.Message, "expand to tempfile first")
}

func TestParseBasicRules_AllowGrep(t *testing.T) {
	cfg := parseBasicRulesFixture(t)
	r2 := cfg.Rules[1]
	assertEqual(t, "action", string(r2.Action), "allow")
	if len(r2.PipeRules) != 1 {
		t.Fatalf("expected 1 pipe rule for grep, got %d", len(r2.PipeRules))
	}
	if len(r2.PipeRules[0].Commands) != 4 {
		t.Errorf("expected 4 commands in grep pipe rule, got %d", len(r2.PipeRules[0].Commands))
	}
}

func TestParseBasicRules_DenyRm(t *testing.T) {
	cfg := parseBasicRulesFixture(t)
	r3 := cfg.Rules[2]
	assertEqual(t, "action", string(r3.Action), "deny")
	assertEqual(t, "commands[0]", r3.Commands[0], "rm")
}

func TestParseTemplates(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/templates.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(cfg.Templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(cfg.Templates))
	}

	// primitive
	assertEqual(t, "tmpl[0].name", cfg.Templates[0].Name, "primitive")
	// "allow cat, echo, head, tail, wc" = 1 rule with 5 commands
	if len(cfg.Templates[0].PipeRules) != 1 {
		t.Errorf("expected 1 pipe rule in primitive, got %d", len(cfg.Templates[0].PipeRules))
	}
	if len(cfg.Templates[0].PipeRules) > 0 && len(cfg.Templates[0].PipeRules[0].Commands) != 5 {
		t.Errorf("expected 5 commands in primitive pipe rule, got %d", len(cfg.Templates[0].PipeRules[0].Commands))
	}

	// safeRead
	assertEqual(t, "tmpl[1].name", cfg.Templates[1].Name, "safeRead")
	assertEqual(t, "tmpl[1].next", cfg.Templates[1].Next, "primitive")

	// bulkExec
	assertEqual(t, "tmpl[2].name", cfg.Templates[2].Name, "bulkExec")
	assertEqual(t, "tmpl[2].extends", cfg.Templates[2].Extends, "safeRead")

	// Rules with next
	if len(cfg.Rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(cfg.Rules))
	}
	assertEqual(t, "rule[0].next", cfg.Rules[0].Next, "primitive")
	assertEqual(t, "rule[1].next", cfg.Rules[1].Next, "bulkExec")
}

func TestParseHookSections(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/hook_sections.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(cfg.PreRules) != 2 {
		t.Fatalf("expected 2 pre rules, got %d", len(cfg.PreRules))
	}
	if len(cfg.PostRules) != 1 {
		t.Fatalf("expected 1 post rule, got %d", len(cfg.PostRules))
	}

	// PreToolUse: deny rm with mode and message
	r := cfg.PreRules[1]
	assertEqual(t, "pre[1].action", string(r.Action), "deny")
	assertEqual(t, "pre[1].mode", r.Mode, "block")
	assertEqual(t, "pre[1].message", r.Message, "Use trash instead")

	// PostToolUse: allow WebFetch
	pr := cfg.PostRules[0]
	assertEqual(t, "post[0].action", string(pr.Action), "allow")
	assertEqual(t, "post[0].mode", pr.Mode, "hint")
}

func TestParseSettings(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/settings.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if cfg.Settings == nil {
		t.Fatal("settings is nil")
	}

	assertEqual(t, "max_context_depth", cfg.Settings.MaxContextDepth, 3)
	assertEqual(t, "max_rules_per_cmd", cfg.Settings.MaxRulesPerCmd, 10)
	assertEqual(t, "fallback", string(cfg.Settings.Fallback), "deny")
}

func TestParseScopeViolationDeny(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
settings:
  workspace: ~/workspace
  scope_violation: deny
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "scope_violation", string(cfg.Settings.ScopeViolation), "deny")
}

func TestParseScopeViolationAsk(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
settings:
  workspace: ~/workspace
  scope_violation: ask
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "scope_violation", string(cfg.Settings.ScopeViolation), "ask")
}

func TestParseScopeViolationDefault(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
settings:
  workspace: ~/workspace
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Unset → defaults to ask (backward compatible)
	assertEqual(t, "scope_violation default", string(cfg.Settings.ScopeViolation), "ask")
}

func TestParseScopeViolationInvalid(t *testing.T) {
	for _, val := range []string{"warn", "allow", "hint", "block"} {
		_, err := Parse(strings.NewReader("settings:\n  scope_violation: " + val + "\n"))
		if err == nil {
			t.Fatalf("expected parse error for scope_violation: %s", val)
		}
		if !strings.Contains(err.Error(), "scope_violation") {
			t.Fatalf("error should mention scope_violation, got: %v", err)
		}
	}
}

// Plan 0025 Phase 2: unanalyzable_action

func TestParseUnanalyzableActionAsk(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
settings:
  unanalyzable_action: ask
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "unanalyzable_action", string(cfg.Settings.UnanalyzableAction), "ask")
	assertEqual(t, "explicit", cfg.Settings.Explicit["unanalyzable_action"], true)
}

func TestParseUnanalyzableActionDeny(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
settings:
  unanalyzable_action: deny
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "unanalyzable_action", string(cfg.Settings.UnanalyzableAction), "deny")
}

func TestParseUnanalyzableActionDefault(t *testing.T) {
	// No settings block: default is deny (existing behavior preserved).
	cfg, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "unanalyzable_action default", string(cfg.Settings.UnanalyzableAction), "deny")

	// Explicit settings: block that doesn't set unanalyzable_action still
	// defaults to deny.
	cfg2, err := Parse(strings.NewReader(`
settings:
  fallback: ask
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "unanalyzable_action default (with other settings)", string(cfg2.Settings.UnanalyzableAction), "deny")
}

func TestParseUnanalyzableActionInvalid(t *testing.T) {
	for _, val := range []string{"warn", "allow", "hint", "block"} {
		_, err := Parse(strings.NewReader("settings:\n  unanalyzable_action: " + val + "\n"))
		if err == nil {
			t.Fatalf("expected parse error for unanalyzable_action: %s", val)
		}
		if !strings.Contains(err.Error(), "unanalyzable_action") {
			t.Fatalf("error should mention unanalyzable_action, got: %v", err)
		}
	}
}

func TestParseArgsRules(t *testing.T) {
	f, err := os.Open("../../testdata/dsl/args_rules.conf")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	cfg, err := Parse(f)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(cfg.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cfg.Rules))
	}

	r := cfg.Rules[0]
	assertEqual(t, "rule.action", string(r.Action), "allow")
	assertEqual(t, "rule.commands[0]", r.Commands[0], "curl")

	if len(r.ArgsRules) != 2 {
		t.Fatalf("expected 2 args rules, got %d", len(r.ArgsRules))
	}

	assertEqual(t, "args[0].pattern", r.ArgsRules[0].Pattern, "-X GET")
	assertEqual(t, "args[0].action", string(r.ArgsRules[0].Action), "allow")
	assertEqual(t, "args[1].pattern", r.ArgsRules[1].Pattern, "-X POST")
	assertEqual(t, "args[1].action", string(r.ArgsRules[1].Action), "ask")
}

func TestParseSettings_StrictConfigError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantFlag bool
	}{
		{
			name:     "true enables strict mode",
			input:    "settings:\n  strict_config_error: true\n",
			wantFlag: true,
		},
		{
			name:     "false keeps default fail-open",
			input:    "settings:\n  strict_config_error: false\n",
			wantFlag: false,
		},
		{
			name:    "invalid bool errors",
			input:   "settings:\n  strict_config_error: maybe\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Parse(strings.NewReader(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if cfg.Settings == nil {
				t.Fatal("settings is nil")
			}
			assertEqual(t, "strict_config_error", cfg.Settings.StrictConfigError, tc.wantFlag)
		})
	}
}

func TestParseUnattended(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
preToolUse:
  ask docker "container op"
    unattended: allow
  ask git "branch delete"
    unattended: deny
  ask kubectl "cluster op"
`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(cfg.PreRules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(cfg.PreRules))
	}
	assertEqual(t, "docker unattended", string(cfg.PreRules[0].Unattended), "allow")
	assertEqual(t, "git unattended", string(cfg.PreRules[1].Unattended), "deny")
	assertEqual(t, "kubectl unattended (unset)", string(cfg.PreRules[2].Unattended), "")
}

func TestParseUnattendedInvalidValue(t *testing.T) {
	_, err := Parse(strings.NewReader(`
preToolUse:
  ask docker
    unattended: warn
`))
	if err == nil {
		t.Fatal("expected error for unattended: warn")
	}
}

func TestParseUnattendedOnNonAskRule(t *testing.T) {
	_, err := Parse(strings.NewReader(`
preToolUse:
  deny curl
    unattended: allow
`))
	if err == nil {
		t.Fatal("expected error: unattended: is only valid on ask rules")
	}
}

func TestParseAskStrategy(t *testing.T) {
	for _, v := range []string{"degrade", "passthrough", "deny-all"} {
		cfg, err := Parse(strings.NewReader("settings:\n  ask_strategy: " + v + "\n"))
		if err != nil {
			t.Fatalf("ask_strategy %s: parse error: %v", v, err)
		}
		assertEqual(t, "ask_strategy "+v, cfg.Settings.AskStrategy, v)
		if !cfg.Settings.Explicit["ask_strategy"] {
			t.Errorf("ask_strategy %s: Explicit flag not set", v)
		}
	}
}

func TestParseAskStrategyInvalid(t *testing.T) {
	_, err := Parse(strings.NewReader("settings:\n  ask_strategy: yolo\n"))
	if err == nil {
		t.Fatal("expected error for invalid ask_strategy")
	}
}

func TestParseAskDegradeDefault(t *testing.T) {
	for _, v := range []string{"deny", "allow"} {
		cfg, err := Parse(strings.NewReader("settings:\n  ask_degrade_default: " + v + "\n"))
		if err != nil {
			t.Fatalf("ask_degrade_default %s: parse error: %v", v, err)
		}
		assertEqual(t, "ask_degrade_default "+v, string(cfg.Settings.AskDegradeDefault), v)
	}
}

func TestParseAskDegradeDefaultInvalid(t *testing.T) {
	_, err := Parse(strings.NewReader("settings:\n  ask_degrade_default: warn\n"))
	if err == nil {
		t.Fatal("expected error for invalid ask_degrade_default")
	}
}

func TestParseSettingsDefaultAskStrategy(t *testing.T) {
	cfg, err := Parse(strings.NewReader("allow ls\n"))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	assertEqual(t, "default ask_strategy", cfg.Settings.AskStrategy, AskStrategyDegrade)
	assertEqual(t, "default ask_degrade_default", string(cfg.Settings.AskDegradeDefault), "deny")
}
