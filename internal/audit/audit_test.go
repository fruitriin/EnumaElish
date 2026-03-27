package audit

import (
	"strings"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

func mustParseConfig(t *testing.T, input string) *dsl.Config {
	t.Helper()
	cfg, err := dsl.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if err := dsl.ResolveTemplates(cfg); err != nil {
		t.Fatalf("resolve templates: %v", err)
	}
	return cfg
}

func TestAuditBasicRules(t *testing.T) {
	cfg := mustParseConfig(t, `
allow ls
deny rm  "dangerous"

settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask
`)
	out := Audit(cfg)

	if len(out.Lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(out.Lines))
	}

	// First line: allow ls
	assertEqual(t, "line[0].action", out.Lines[0].Action, dsl.ActionAllow)
	assertEqual(t, "line[0].command", out.Lines[0].Command, "ls")

	// deny rm line
	found := false
	for _, line := range out.Lines {
		if line.Action == dsl.ActionDeny && line.Command == "rm" {
			found = true
			assertEqual(t, "rm.message", line.Message, "dangerous")
		}
	}
	if !found {
		t.Error("expected deny rm line")
	}
}

func TestAuditWithTemplates(t *testing.T) {
	cfg := mustParseConfig(t, `
template bulkExec
  |,>>
    deny rm  "don't pipe into destructive"
  exec:
    deny rm  "expand to tempfile first"

allow find
  next: bulkExec

settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask
`)
	out := Audit(cfg)

	// Should have: allow find, deny find|rm (pipe), deny find-exec rm (exec), find && ...
	foundPipe := false
	foundExec := false
	for _, line := range out.Lines {
		if line.Action == dsl.ActionDeny && line.Command == "find | rm" {
			foundPipe = true
			assertEqual(t, "pipe.template", line.Template, "bulkExec")
		}
		if line.Action == dsl.ActionDeny && line.Command == "find -exec rm" {
			foundExec = true
			assertEqual(t, "exec.template", line.Template, "bulkExec.exec")
		}
	}
	if !foundPipe {
		t.Error("expected deny find|rm line")
	}
	if !foundExec {
		t.Error("expected deny find-exec rm line")
	}
}

func TestAuditFormat(t *testing.T) {
	cfg := mustParseConfig(t, `
allow ls
deny rm  "dangerous"

settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask
`)
	out := Audit(cfg)
	formatted := Format(out)

	if !strings.Contains(formatted, "[allow]") {
		t.Error("expected [allow] in output")
	}
	if !strings.Contains(formatted, "[deny]") {
		t.Error("expected [deny] in output")
	}
	if !strings.Contains(formatted, "Settings:") {
		t.Error("expected Settings section")
	}
	if !strings.Contains(formatted, "Stats:") {
		t.Error("expected Stats section")
	}
}

func TestAuditStats(t *testing.T) {
	cfg := mustParseConfig(t, `
template foo
  |,>>
    allow cat

allow ls
deny rm

settings:
  max_context_depth: 2
  max_rules_per_cmd: 5
  fallback: ask
`)
	out := Audit(cfg)
	assertEqual(t, "rules", out.Stats.RuleCount, 2)
	assertEqual(t, "templates", out.Stats.TemplateCount, 1)
}

func assertEqual[T comparable](t *testing.T, name string, got, expected T) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}
