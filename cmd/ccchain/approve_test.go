package main

import (
	"testing"
	"time"
	"unicode/utf8"
)

// TestParseApproveArgs covers the small hand-rolled parser used by
// `ccchain approve`. Every action flag is exclusive, --ttl requires a
// value, and unknown flags are rejected.
func TestParseApproveArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    approveCmd
		wantTTL time.Duration
		wantErr bool
	}{
		{name: "last", args: []string{"--last"}, want: approveCmdLast},
		{name: "list", args: []string{"--list"}, want: approveCmdList},
		{name: "revoke-all", args: []string{"--revoke-all"}, want: approveCmdRevokeAll},
		{name: "prefix", args: []string{"abcd1234"}, want: approveCmdByPrefix},
		{name: "last with ttl", args: []string{"--last", "--ttl", "1h"}, want: approveCmdLast, wantTTL: time.Hour},
		{name: "global flag", args: []string{"--last", "--global"}, want: approveCmdLast},
		{name: "conflict last+list", args: []string{"--last", "--list"}, wantErr: true},
		{name: "conflict last+prefix", args: []string{"--last", "abcd1234"}, wantErr: true},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
		{name: "ttl missing value", args: []string{"--last", "--ttl"}, wantErr: true},
		{name: "ttl negative", args: []string{"--last", "--ttl", "-1s"}, wantErr: true},
		{name: "ttl invalid", args: []string{"--last", "--ttl", "abc"}, wantErr: true},
		{name: "two prefixes", args: []string{"abcd1234", "efgh5678"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, cmd, err := parseApproveArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tc.want {
				t.Errorf("cmd = %v, want %v", cmd, tc.want)
			}
			if tc.wantTTL != 0 && opts.ttl != tc.wantTTL {
				t.Errorf("ttl = %v, want %v", opts.ttl, tc.wantTTL)
			}
		})
	}
}

// TestApproveOptionsScope verifies that --global switches scope while the
// default (no flag) is session.
func TestApproveOptionsScope(t *testing.T) {
	opts, _, err := parseApproveArgs([]string{"--last"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.scopeFlag {
		t.Error("--global should default off")
	}

	opts, _, err = parseApproveArgs([]string{"--last", "--global"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.scopeFlag {
		t.Error("--global should be set")
	}
}

// TestShortHashAndPreview covers the display helpers.
func TestShortHashAndPreview(t *testing.T) {
	if shortHash("abc") != "abc" {
		t.Error("short hash unchanged when < 12 chars")
	}
	if got := shortHash("abcdef0123456789abcdef"); got != "abcdef012345" {
		t.Errorf("shortHash = %q, want abcdef012345", got)
	}
	long := "git push origin main -- " + string(make([]byte, 100))
	got := previewCommand(long)
	// Visual length is 80 (79 chars + 1 ellipsis rune); byte length is larger
	// because "…" is 3 bytes in UTF-8.
	if runes := utf8.RuneCountInString(got); runes != 80 {
		t.Errorf("preview runes = %d, want 80 (got byte length %d)", runes, len(got))
	}
}

// TestPreviewCommand_SanitizesControlChars is Security H1's regression guard:
// commands may contain ANSI escapes or other C0 control bytes (e.g.
// `echo $'\x1b[2Jhi'`). Displaying them raw in `approve --list` lets an
// attacker manipulate the terminal — clear the screen, hide characters, fake
// prompts — and mislead the human's approval decision.
func TestPreviewCommand_SanitizesControlChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"ansi escape", "echo \x1b[2Jhi"},
		{"carriage return", "echo hi\rignored"},
		{"vertical tab", "echo hi\x0bthere"},
		{"del", "echo hi\x7fthere"},
		{"nul-ish (0x01)", "echo hi\x01there"},
		{"bell", "echo hi\x07there"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := previewCommand(tc.in)
			for _, r := range got {
				if r < 0x20 || r == 0x7f {
					t.Errorf("preview retained control char %U in %q", r, got)
				}
			}
		})
	}
}

// TestSanitizeApprovalDisplay covers the standalone sanitizer used by
// printApprovalGranted for cwd / session_id.
func TestSanitizeApprovalDisplay(t *testing.T) {
	got := sanitizeApprovalDisplay("/tmp/x\x1b[2J")
	for _, r := range got {
		if r < 0x20 || r == 0x7f {
			t.Errorf("sanitize retained %U in %q", r, got)
		}
	}
	// Regular text passes through unchanged.
	if sanitizeApprovalDisplay("plain text") != "plain text" {
		t.Error("plain ASCII was modified")
	}
	// Multi-byte UTF-8 survives.
	if got := sanitizeApprovalDisplay("日本語"); got != "日本語" {
		t.Errorf("multi-byte lost: %q", got)
	}
}
