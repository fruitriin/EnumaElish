package eval

import (
	"strings"
	"testing"

	"github.com/fruitriin/ccchain/internal/dsl"
)

// Tests for deny-redirect pattern (Plan 0016)
// Uses existing args: mechanism to deny access to sensitive paths
// and suggest alternatives via deny messages.

func TestRedirectEnvFile(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow Read
    args:
      \.env$|\.env\.: deny  ".env contains secrets. Read .env.example instead"
`)
	// .env → deny with redirect message
	r1 := EvaluateTool("Read", "/workspace/.env", cfg)
	assertEqual(t, "env deny", r1.Action, dsl.ActionDeny)
	if !strings.Contains(r1.Message, ".env.example") {
		t.Errorf("expected redirect message mentioning .env.example, got: %s", r1.Message)
	}

	// .env.local → deny
	r2 := EvaluateTool("Read", "/workspace/.env.local", cfg)
	assertEqual(t, "env.local deny", r2.Action, dsl.ActionDeny)

	// Regular file → allow
	r3 := EvaluateTool("Read", "/workspace/README.md", cfg)
	assertEqual(t, "readme allow", r3.Action, dsl.ActionAllow)
}

func TestRedirectNodeModules(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow Edit
    args:
      node_modules/: deny  "Don't edit node_modules. Modify package.json"
`)
	r := EvaluateTool("Edit", "/workspace/node_modules/express/index.js", cfg)
	assertEqual(t, "node_modules deny", r.Action, dsl.ActionDeny)
	if !strings.Contains(r.Message, "package.json") {
		t.Errorf("expected redirect to package.json, got: %s", r.Message)
	}
}

func TestRedirectBuildArtifacts(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow Edit
    args:
      dist/|build/|out/: deny  "Don't edit build artifacts"
`)
	r1 := EvaluateTool("Edit", "/workspace/dist/bundle.js", cfg)
	assertEqual(t, "dist deny", r1.Action, dsl.ActionDeny)

	r2 := EvaluateTool("Edit", "/workspace/build/output.css", cfg)
	assertEqual(t, "build deny", r2.Action, dsl.ActionDeny)

	r3 := EvaluateTool("Edit", "/workspace/src/main.ts", cfg)
	assertEqual(t, "src allow", r3.Action, dsl.ActionAllow)
}

func TestRedirectSSHKeys(t *testing.T) {
	cfg := mustParseConfig(t, `
preToolUse
  allow Read
    args:
      \.ssh/|\.gnupg/: deny  "SSH/GPG keys should not be accessed"
`)
	r1 := EvaluateTool("Read", "/home/user/.ssh/id_rsa", cfg)
	assertEqual(t, "ssh key deny", r1.Action, dsl.ActionDeny)

	r2 := EvaluateTool("Read", "/home/user/.gnupg/private-keys-v1.d/key", cfg)
	assertEqual(t, "gnupg deny", r2.Action, dsl.ActionDeny)
}
