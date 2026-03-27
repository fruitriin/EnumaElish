package eval

import (
	"strings"
	"testing"
)

func TestExpandMessageBasic(t *testing.T) {
	msg := ExpandMessage("blocked: {cmd} {args}", "rm", []string{"-rf", "/"}, "rm -rf /")
	if !strings.Contains(msg, "rm") {
		t.Errorf("expected cmd in message, got: %s", msg)
	}
	if !strings.Contains(msg, "-rf /") {
		t.Errorf("expected args in message, got: %s", msg)
	}
}

func TestExpandMessageID(t *testing.T) {
	msg1 := ExpandMessage("file_{id}.txt", "rm", nil, "rm")
	msg2 := ExpandMessage("file_{id}.txt", "rm", nil, "rm")
	// IDs should be different
	if msg1 == msg2 {
		t.Error("expected unique IDs, got same")
	}
	if strings.Contains(msg1, "{id}") {
		t.Error("template variable not expanded")
	}
}

func TestExpandMessageNoTemplate(t *testing.T) {
	msg := ExpandMessage("simple message", "rm", nil, "rm")
	if msg != "simple message" {
		t.Errorf("expected unchanged message, got: %s", msg)
	}
}

func TestExpandMessageSanitize(t *testing.T) {
	// Control characters should be replaced with spaces
	msg := ExpandMessage("{command}", "rm", nil, "rm\x00\x01\x02test")
	if strings.ContainsAny(msg, "\x00\x01\x02") {
		t.Error("control characters not sanitized")
	}
}

func TestExpandMessageTruncate(t *testing.T) {
	longCmd := strings.Repeat("a", 300)
	msg := ExpandMessage("{command}", "cmd", nil, longCmd)
	if len(msg) > 210 { // 200 + "..."
		t.Errorf("message not truncated: len=%d", len(msg))
	}
	if !strings.HasSuffix(msg, "...") {
		t.Error("expected truncated message to end with ...")
	}
}

func TestExpandMessageTimestamp(t *testing.T) {
	msg := ExpandMessage("time: {timestamp}", "cmd", nil, "cmd")
	if strings.Contains(msg, "{timestamp}") {
		t.Error("timestamp not expanded")
	}
	// Should contain date-like pattern
	if len(msg) < 20 {
		t.Errorf("timestamp too short: %s", msg)
	}
}

func TestExpandMessageIntegration(t *testing.T) {
	cfg := mustParseConfig(t, `
deny rm
  message: "{cmd} is blocked. Use trash {args} instead."
`)
	result, err := Evaluate("rm -rf /tmp/build", cfg)
	if err != nil {
		t.Fatalf("evaluate error: %v", err)
	}
	if !strings.Contains(result.Message, "rm is blocked") {
		t.Errorf("expected expanded {cmd}, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "-rf /tmp/build") {
		t.Errorf("expected expanded {args}, got: %s", result.Message)
	}
}
