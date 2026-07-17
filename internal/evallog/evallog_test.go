package evallog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogAppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	if err := Log(path, Entry{
		ToolName: "Bash",
		Command:  "ls -la",
		Action:   "allow",
	}); err != nil {
		t.Fatalf("Log first: %v", err)
	}
	if err := Log(path, Entry{
		ToolName: "Bash",
		Command:  "rm -rf /",
		Action:   "deny",
	}); err != nil {
		t.Fatalf("Log second: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var got []Entry
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		got = append(got, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].Action != "allow" || got[1].Action != "deny" {
		t.Fatalf("unexpected actions: %+v", got)
	}
	if got[0].Timestamp == 0 || got[1].Timestamp == 0 {
		t.Fatalf("timestamps not set: %+v", got)
	}
}

func TestLogFilePermsAreOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	// Log into a nested dir so ensureDir runs.
	path := filepath.Join(dir, "sub", "log.jsonl")

	if err := Log(path, Entry{ToolName: "Bash", Action: "allow"}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("file perm: want 0600, got %o", got)
	}

	di, err := os.Stat(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := di.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir perm: want 0700, got %o", got)
	}
}

func TestLogEmptyPathReturnsError(t *testing.T) {
	if err := Log("", Entry{Action: "allow"}); err == nil {
		t.Fatal("want error on empty path, got nil")
	}
}

func TestLogUnwritablePathFailsOpen(t *testing.T) {
	// A file where the parent is a regular file (not a dir) is unwritable —
	// verify Log returns an error instead of panicking.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("create blocker: %v", err)
	}
	// Try to log at blocker/log.jsonl — mkdir will fail.
	err := Log(filepath.Join(blocker, "log.jsonl"), Entry{Action: "allow"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestLogConcurrentAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	const goroutines = 10
	const perGoroutine = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				if err := Log(path, Entry{
					ToolName: "Bash",
					Command:  "cmd",
					Action:   "allow",
					// Encode source for correlation.
					MatchedRule: rulePayload(g, i),
				}); err != nil {
					t.Errorf("goroutine %d iter %d: %v", g, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()

	// Every line must be a valid JSON object; count them.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	count := 0
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("line %d not valid JSON: %v (line=%q)", count+1, err, scanner.Text())
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := goroutines * perGoroutine
	if count != want {
		t.Fatalf("want %d entries, got %d", want, count)
	}
}

func rulePayload(g, i int) string {
	return "g" + itoa(g) + "-i" + itoa(i)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestTruncateCommandDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	long := strings.Repeat("a", CommandLengthLimit+50)
	if err := Log(path, Entry{ToolName: "Bash", Command: long, Action: "allow"}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var e Entry
	if err := json.Unmarshal(data[:len(data)-1], &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(e.Command) != CommandLengthLimit {
		t.Fatalf("want command length %d, got %d", CommandLengthLimit, len(e.Command))
	}
}

func TestTruncateCommandUTF8Boundary(t *testing.T) {
	// The Japanese ellipsis is 3 bytes; pack the string so the naive byte-cut
	// would land in the middle of a rune, then verify we get valid UTF-8.
	ellipsis := "…" // 3 bytes: e2 80 a6
	prefix := strings.Repeat("a", CommandLengthLimit-2)
	cmd := prefix + ellipsis // length = CommandLengthLimit + 1, cut lands mid-rune

	got := truncateCommand(cmd, CommandLengthLimit)
	if len(got) > CommandLengthLimit {
		t.Fatalf("truncated len %d > limit %d", len(got), CommandLengthLimit)
	}
	// Byte position CommandLengthLimit-2 starts the ellipsis; the whole rune
	// must be dropped, leaving prefix intact.
	if got != prefix {
		t.Fatalf("want prefix without partial rune, got %q", got)
	}
}

func TestStatsMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.jsonl")

	got, err := Stats(path, 0, GroupByAction)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestStatsGroupByAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	now := time.Now().Unix()

	entries := []Entry{
		{Timestamp: now, ToolName: "Bash", Action: "allow", MatchedRule: "allow ls"},
		{Timestamp: now, ToolName: "Bash", Action: "allow", MatchedRule: "allow cat"},
		{Timestamp: now, ToolName: "Bash", Action: "deny", MatchedRule: "deny rm"},
	}
	for _, e := range entries {
		if err := Log(path, e); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}

	got, err := Stats(path, 0, GroupByAction)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(got), got)
	}
	if got[0].Key != "allow" || got[0].Count != 2 {
		t.Fatalf("want allow=2 first, got %+v", got[0])
	}
	if got[1].Key != "deny" || got[1].Count != 1 {
		t.Fatalf("want deny=1 second, got %+v", got[1])
	}
}

func TestStatsGroupByRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	now := time.Now().Unix()

	for i := 0; i < 5; i++ {
		if err := Log(path, Entry{Timestamp: now, Action: "allow", MatchedRule: "allow ls"}); err != nil {
			t.Fatalf("Log: %v", err)
		}
	}
	if err := Log(path, Entry{Timestamp: now, Action: "deny", MatchedRule: "deny rm"}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if err := Log(path, Entry{Timestamp: now, Action: "allow"}); err != nil {
		// no rule → (none) bucket
		t.Fatalf("Log: %v", err)
	}

	got, err := Stats(path, 0, GroupByRule)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 groups, got %d: %+v", len(got), got)
	}
	if got[0].Key != "allow ls" || got[0].Count != 5 {
		t.Fatalf("want allow ls=5 first, got %+v", got[0])
	}
	// The other two share Count=1; the tie-breaker is Key asc.
	if got[1].Key == "(none)" && got[2].Key != "deny rm" {
		t.Fatalf("tie-break broken: %+v", got)
	}
}

func TestStatsSinceFilter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	now := time.Now().Unix()

	// One entry in the past (2h ago), one recent.
	if err := Log(path, Entry{Timestamp: now - 2*3600, Action: "deny"}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if err := Log(path, Entry{Timestamp: now, Action: "allow"}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	got, err := Stats(path, 1*time.Hour, GroupByAction)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(got) != 1 || got[0].Key != "allow" {
		t.Fatalf("since filter dropped wrong entry: %+v", got)
	}
}

func TestStatsSkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	// Write a mix of valid and garbage lines directly.
	valid, _ := json.Marshal(Entry{Timestamp: time.Now().Unix(), Action: "allow"})
	content := append(valid, '\n')
	content = append(content, []byte("not json\n")...)
	content = append(content, valid...)
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := Stats(path, 0, GroupByAction)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(got) != 1 || got[0].Count != 2 {
		t.Fatalf("want allow=2 (garbage skipped), got %+v", got)
	}
}
