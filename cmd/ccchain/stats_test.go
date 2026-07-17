package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fruitriin/EnumaElish/internal/evallog"
)

func TestParseStatsArgsDefaults(t *testing.T) {
	opts, err := parseStatsArgs(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.since != 24*time.Hour {
		t.Errorf("default since: want 24h, got %s", opts.since)
	}
	if opts.groupBy != evallog.GroupByAction {
		t.Errorf("default group-by: want action, got %s", opts.groupBy)
	}
	if opts.top != 20 {
		t.Errorf("default top: want 20, got %d", opts.top)
	}
}

func TestParseStatsArgsAll(t *testing.T) {
	opts, err := parseStatsArgs([]string{
		"--since", "7d",
		"--group-by", "rule",
		"--top", "5",
		"--json",
		"--log", "/tmp/log.jsonl",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.since != 7*24*time.Hour {
		t.Errorf("since 7d: want 168h, got %s", opts.since)
	}
	if opts.groupBy != evallog.GroupByRule {
		t.Errorf("group-by rule: got %s", opts.groupBy)
	}
	if opts.top != 5 {
		t.Errorf("top 5: got %d", opts.top)
	}
	if !opts.asJSON {
		t.Error("json: not set")
	}
	if opts.logPath != "/tmp/log.jsonl" {
		t.Errorf("logPath: got %q", opts.logPath)
	}
}

func TestParseStatsArgsInvalidGroupBy(t *testing.T) {
	_, err := parseStatsArgs([]string{"--group-by", "yolo"})
	if err == nil {
		t.Fatal("want error for invalid group-by")
	}
}

func TestParseStatsArgsPositionalRejected(t *testing.T) {
	_, err := parseStatsArgs([]string{"extra"})
	if err == nil {
		t.Fatal("want error for unexpected positional")
	}
}

func TestParseSinceDurationDays(t *testing.T) {
	d, err := parseSinceDuration("7d")
	if err != nil {
		t.Fatalf("parse 7d: %v", err)
	}
	if d != 7*24*time.Hour {
		t.Errorf("7d: got %s", d)
	}
}

func TestParseSinceDurationHours(t *testing.T) {
	d, err := parseSinceDuration("3h")
	if err != nil {
		t.Fatalf("parse 3h: %v", err)
	}
	if d != 3*time.Hour {
		t.Errorf("3h: got %s", d)
	}
}

func TestStatsMainMissingLog(t *testing.T) {
	// Config without log setting → exit 1 with hint.
	dir := t.TempDir()
	conf := filepath.Join(dir, ".ccchain.conf")
	if err := os.WriteFile(conf, []byte("allow ls\n"), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := statsMain(conf, nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("want exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "log is not enabled") {
		t.Errorf("stderr should mention log-not-enabled: %q", stderr.String())
	}
}

func TestStatsMainTableOutput(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log.jsonl")
	now := time.Now().Unix()
	for _, e := range []evallog.Entry{
		{Timestamp: now, Action: "allow"},
		{Timestamp: now, Action: "allow"},
		{Timestamp: now, Action: "deny"},
	} {
		if err := evallog.Log(log, e); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := statsMain("", []string{"--log", log}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stats: exit %d, stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "COUNT") || !strings.Contains(out, "allow") || !strings.Contains(out, "deny") {
		t.Errorf("table missing expected content:\n%s", out)
	}
}

func TestStatsMainJSONOutput(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log.jsonl")
	now := time.Now().Unix()
	if err := evallog.Log(log, evallog.Entry{Timestamp: now, Action: "deny"}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := statsMain("", []string{"--log", log, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stats: exit %d, stderr=%q", code, stderr.String())
	}

	var got []evallog.Aggregate
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (out=%q)", err, stdout.String())
	}
	if len(got) != 1 || got[0].Key != "deny" || got[0].Count != 1 {
		t.Errorf("unexpected aggregate: %+v", got)
	}
}

func TestStatsMainWithConfigLogSetting(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log.jsonl")

	conf := filepath.Join(dir, ".ccchain.conf")
	confBody := "settings:\n  log: " + log + "\n\nallow ls\n"
	if err := os.WriteFile(conf, []byte(confBody), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	// Seed an entry.
	if err := evallog.Log(log, evallog.Entry{Timestamp: time.Now().Unix(), Action: "allow"}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := statsMain(conf, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("stats: exit %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "allow") {
		t.Errorf("stdout missing allow row: %q", stdout.String())
	}
}
