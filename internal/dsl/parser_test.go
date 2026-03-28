package dsl

import (
	"os"
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
