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
	rm := findAuditLine(t, out.Lines, dsl.ActionDeny, "rm")
	assertEqual(t, "rm.message", rm.Message, "dangerous")
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

	// Should have: deny find|rm (pipe), deny find-exec rm (exec)
	pipe := findAuditLine(t, out.Lines, dsl.ActionDeny, "find | rm")
	assertEqual(t, "pipe.template", pipe.Template, "bulkExec")

	exec := findAuditLine(t, out.Lines, dsl.ActionDeny, "find -exec rm")
	assertEqual(t, "exec.template", exec.Template, "bulkExec.exec")
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

func findAuditLine(t *testing.T, lines []AuditLine, action dsl.Action, command string) AuditLine {
	t.Helper()
	for _, line := range lines {
		if line.Action == action && line.Command == command {
			return line
		}
	}
	t.Fatalf("expected %s %s line, not found", action, command)
	return AuditLine{}
}

func assertEqual[T comparable](t *testing.T, name string, got, expected T) {
	t.Helper()
	if got != expected {
		t.Errorf("%s: expected %v, got %v", name, expected, got)
	}
}
