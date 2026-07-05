package dsl

import "testing"

// buildLookupTestConfig constructs a config with a template chain:
// base -> child (via next:), each holding one pipe rule and one exec rule.
func buildLookupTestConfig() *Config {
	child := &Template{
		Name:      "child",
		PipeRules: []*Rule{{Action: ActionDeny, Commands: []string{"rm"}}},
		ExecRules: []*Rule{{Action: ActionDeny, Commands: []string{"curl"}}},
	}
	base := &Template{
		Name:      "base",
		Next:      "child",
		PipeRules: []*Rule{{Action: ActionAllow, Commands: []string{"grep"}}},
		ExecRules: []*Rule{{Action: ActionAllow, Commands: []string{"echo"}}},
	}
	return &Config{
		Templates: []*Template{base, child},
		TemplateIndex: map[string]*Template{
			"base":  base,
			"child": child,
		},
	}
}

func TestCollectTemplatePipeRulesFollowsChain(t *testing.T) {
	config := buildLookupTestConfig()
	base := LookupTemplate(config, "base")

	rules := CollectTemplatePipeRules(base, config)

	if len(rules) != 2 {
		t.Fatalf("expected 2 pipe rules, got %d", len(rules))
	}
	if rules[0].Commands[0] != "grep" {
		t.Errorf("expected first pipe rule for grep, got %v", rules[0].Commands)
	}
	if rules[1].Commands[0] != "rm" {
		t.Errorf("expected second pipe rule for rm (from next: chain), got %v", rules[1].Commands)
	}
}

func TestCollectTemplateExecRulesFollowsChain(t *testing.T) {
	config := buildLookupTestConfig()
	base := LookupTemplate(config, "base")

	rules := CollectTemplateExecRules(base, config)

	if len(rules) != 2 {
		t.Fatalf("expected 2 exec rules, got %d", len(rules))
	}
	if rules[0].Commands[0] != "echo" {
		t.Errorf("expected first exec rule for echo, got %v", rules[0].Commands)
	}
	if rules[1].Commands[0] != "curl" {
		t.Errorf("expected second exec rule for curl (from next: chain), got %v", rules[1].Commands)
	}
}

func TestCollectTemplateRulesCircularNext(t *testing.T) {
	// a -> b -> a: the visited set must break the cycle without infinite recursion.
	a := &Template{
		Name:      "a",
		Next:      "b",
		PipeRules: []*Rule{{Action: ActionAllow, Commands: []string{"cat"}}},
	}
	b := &Template{
		Name:      "b",
		Next:      "a",
		PipeRules: []*Rule{{Action: ActionDeny, Commands: []string{"rm"}}},
	}
	config := &Config{
		Templates:     []*Template{a, b},
		TemplateIndex: map[string]*Template{"a": a, "b": b},
	}

	rules := CollectTemplatePipeRules(a, config)

	if len(rules) != 2 {
		t.Fatalf("expected 2 pipe rules (each template visited once), got %d", len(rules))
	}
}

func TestCollectTemplateRulesMissingNext(t *testing.T) {
	// next: pointing to a nonexistent template must be silently skipped.
	orphan := &Template{
		Name:      "orphan",
		Next:      "ghost",
		ExecRules: []*Rule{{Action: ActionAsk, Commands: []string{"ssh"}}},
	}
	config := &Config{
		Templates:     []*Template{orphan},
		TemplateIndex: map[string]*Template{"orphan": orphan},
	}

	rules := CollectTemplateExecRules(orphan, config)

	if len(rules) != 1 {
		t.Fatalf("expected 1 exec rule, got %d", len(rules))
	}
}
