package eval

import (
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

func TestEvaluateToolExactMatch(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  deny Read
    message: "Read is denied"
`)
	result := EvaluateTool("Read", "/etc/passwd", cfg)
	assertEqual(t, "action", result.Action, dsl.ActionDeny)
}

func TestEvaluateToolArgsMatch(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow Read
    args:
      \.env|\.ssh|credentials: deny  "sensitive file"
`)
	// Safe file
	r1 := EvaluateTool("Read", "/workspace/README.md", cfg)
	assertEqual(t, "safe file", r1.Action, dsl.ActionAllow)

	// Sensitive file
	r2 := EvaluateTool("Read", "/home/user/.ssh/id_rsa", cfg)
	assertEqual(t, "ssh key", r2.Action, dsl.ActionDeny)

	// .env file
	r3 := EvaluateTool("Read", "/workspace/.env", cfg)
	assertEqual(t, "env file", r3.Action, dsl.ActionDeny)
}

func TestEvaluateToolMCPWildcard(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  deny mcp__*__delete_*
    message: "MCP delete operations are denied"
`)
	// Delete operation → deny
	r1 := EvaluateTool("mcp__github__delete_issue", "", cfg)
	assertEqual(t, "mcp delete", r1.Action, dsl.ActionDeny)

	// Non-delete → fallback (ask)
	r2 := EvaluateTool("mcp__github__create_issue", "", cfg)
	assertEqual(t, "mcp create", r2.Action, dsl.ActionAsk)
}

func TestEvaluateToolWebFetch(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow WebFetch
    args:
      localhost|127\.0\.0\.1: ask  "local service access"
      169\.254\.169\.254: deny  "cloud metadata endpoint"
`)
	// Normal URL
	r1 := EvaluateTool("WebFetch", "https://example.com", cfg)
	assertEqual(t, "normal url", r1.Action, dsl.ActionAllow)

	// Localhost
	r2 := EvaluateTool("WebFetch", "http://localhost:3000/api", cfg)
	assertEqual(t, "localhost", r2.Action, dsl.ActionAsk)

	// Cloud metadata
	r3 := EvaluateTool("WebFetch", "http://169.254.169.254/latest/meta-data/", cfg)
	assertEqual(t, "metadata", r3.Action, dsl.ActionDeny)
}

func TestEvaluateToolFallback(t *testing.T) {
	cfg := mustParseConfig(t, `
settings:
  fallback: ask
`)
	// No matching rule → fallback
	result := EvaluateTool("Read", "/some/file", cfg)
	assertEqual(t, "fallback", result.Action, dsl.ActionAsk)
}

func TestEvaluateToolEditWrite(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow Edit
  deny Write
    args:
      \.claude/settings: deny  "settings file protection"
`)
	// Edit → allow
	r1 := EvaluateTool("Edit", "/workspace/main.go", cfg)
	assertEqual(t, "edit", r1.Action, dsl.ActionAllow)

	// Write to settings → deny
	r2 := EvaluateTool("Write", "/workspace/.claude/settings.json", cfg)
	assertEqual(t, "write settings", r2.Action, dsl.ActionDeny)
}
