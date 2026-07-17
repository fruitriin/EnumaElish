package approve

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newStoreT returns a Store rooted at a t.TempDir() and a mutable clock.
func newStoreT(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(dir)
	c := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	s.SetNow(c.Now)
	return s, c
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func mustHash(t *testing.T, cmd string) (string, string) {
	t.Helper()
	h, canon, err := Normalize(cmd)
	if err != nil {
		t.Fatalf("Normalize(%q): %v", cmd, err)
	}
	return h, canon
}

// TestDefaultDir verifies the test-hook override and the config-dir fallback.
// Security H2: the production DefaultDir no longer honors an env-var seam;
// only SetDefaultDirForTest can redirect the store dir at test time.
func TestDefaultDir(t *testing.T) {
	SetDefaultDirForTest("/tmp/x")
	t.Cleanup(func() { SetDefaultDirForTest("") })
	got, err := DefaultDir()
	if err != nil || got != "/tmp/x" {
		t.Fatalf("test override: got %q, err %v", got, err)
	}

	SetDefaultDirForTest("")
	t.Setenv("CLAUDE_CONFIG_DIR", "/abs/claude")
	got, err = DefaultDir()
	if err != nil || got != "/abs/claude/ccchain" {
		t.Fatalf("CLAUDE_CONFIG_DIR: got %q, err %v", got, err)
	}

	t.Setenv("CLAUDE_CONFIG_DIR", "relative/path")
	got, err = DefaultDir()
	if err != nil {
		t.Fatalf("home fallback: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("home fallback should return absolute path, got %q", got)
	}
}

// TestDefaultDir_NoEnvVarSeam asserts that setting the historical
// CCCHAIN_APPROVE_STORE env var has no effect on DefaultDir. Regression
// guard for Security H2.
func TestDefaultDir_NoEnvVarSeam(t *testing.T) {
	SetDefaultDirForTest("")
	t.Setenv("CCCHAIN_APPROVE_STORE", "/tmp/hijacked")
	t.Setenv("CLAUDE_CONFIG_DIR", "/abs/claude")
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if got == "/tmp/hijacked" {
		t.Fatalf("env var CCCHAIN_APPROVE_STORE unexpectedly hijacked DefaultDir")
	}
	if got != "/abs/claude/ccchain" {
		t.Fatalf("expected CLAUDE_CONFIG_DIR path, got %q", got)
	}
}

// TestRecordAndListPending verifies pending append and read-back order.
func TestRecordAndListPending(t *testing.T) {
	s, _ := newStoreT(t)
	h1, canon1 := mustHash(t, `git push origin main`)
	h2, canon2 := mustHash(t, `rm -rf /tmp/foo`)

	if err := s.RecordPending(PendingEntry{Hash: h1, Command: canon1, CWD: "/w"}); err != nil {
		t.Fatalf("record 1: %v", err)
	}
	if err := s.RecordPending(PendingEntry{Hash: h2, Command: canon2, CWD: "/w", SessionID: "sess"}); err != nil {
		t.Fatalf("record 2: %v", err)
	}

	list, err := s.ListPending()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 entries, got %d", len(list))
	}
	if list[0].Hash != h1 || list[1].Hash != h2 {
		t.Errorf("order mismatch: %+v", list)
	}
	// Perms check.
	fi, err := os.Stat(filepath.Join(s.Dir(), "pending.jsonl"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("perms = %o, want 0600", fi.Mode().Perm())
	}
	dirfi, _ := os.Stat(s.Dir())
	if dirfi.Mode().Perm() != 0o700 {
		t.Errorf("dir perms = %o, want 0700", dirfi.Mode().Perm())
	}
}

// TestApproveLast_ConsumeAndOneShot covers the happy path: record → approve
// --last → check consumes once, then repeat check returns false.
func TestApproveLast_ConsumeAndOneShot(t *testing.T) {
	s, _ := newStoreT(t)
	cmd := `git push origin main`
	h, canon := mustHash(t, cmd)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w", SessionID: "sess-1"}); err != nil {
		t.Fatalf("record: %v", err)
	}

	appr, seed, err := s.ApproveLast(ApproveOptions{})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if seed == nil || seed.Hash != h {
		t.Fatalf("seed mismatch: %+v", seed)
	}
	if appr == nil || appr.Hash != h {
		t.Fatalf("approved mismatch: %+v", appr)
	}
	if appr.Scope != ScopeSession {
		t.Errorf("default scope should be session, got %q", appr.Scope)
	}
	if appr.CWD != "/w" || appr.SessionID != "sess-1" {
		t.Errorf("session scope should inherit seed cwd/session_id, got %+v", appr)
	}

	ok, entry, err := s.CheckApproved(cmd, "/w", "sess-1")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !ok || entry == nil {
		t.Fatalf("first check should consume, got ok=%v entry=%+v", ok, entry)
	}

	ok, _, err = s.CheckApproved(cmd, "/w", "sess-1")
	if err != nil {
		t.Fatalf("second check: %v", err)
	}
	if ok {
		t.Errorf("second check must be false (one-shot consumed)")
	}
}

// TestCheckApproved_ScopeSession rejects mismatching cwd/session_id under
// session scope.
func TestCheckApproved_ScopeSession(t *testing.T) {
	s, _ := newStoreT(t)
	cmd := `git push origin main`
	h, canon := mustHash(t, cmd)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w", SessionID: "sess-A"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, _, err := s.ApproveLast(ApproveOptions{}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	ok, _, err := s.CheckApproved(cmd, "/other", "sess-A")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("mismatched cwd must not match under session scope")
	}
	ok, _, err = s.CheckApproved(cmd, "/w", "sess-B")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("mismatched session_id must not match under session scope")
	}
	// Correct pair still matches.
	ok, _, err = s.CheckApproved(cmd, "/w", "sess-A")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("matching pair should match")
	}
}

// TestCheckApproved_ScopeGlobal matches on hash alone.
func TestCheckApproved_ScopeGlobal(t *testing.T) {
	s, _ := newStoreT(t)
	cmd := `git push origin main`
	h, canon := mustHash(t, cmd)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w-A"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, _, err := s.ApproveLast(ApproveOptions{Scope: ScopeGlobal}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	ok, _, err := s.CheckApproved(cmd, "/w-B", "any-session")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("global scope must match across cwd/session")
	}
}

// TestCheckApproved_TTLExpired ensures expired approvals are not consumed.
func TestCheckApproved_TTLExpired(t *testing.T) {
	s, clock := newStoreT(t)
	cmd := `git push origin main`
	h, canon := mustHash(t, cmd)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w", SessionID: "s"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, _, err := s.ApproveLast(ApproveOptions{TTL: 1 * time.Minute}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	clock.advance(2 * time.Minute)

	ok, _, err := s.CheckApproved(cmd, "/w", "s")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("expired approval must not match")
	}
}

// TestCheckApproved_DynamicRejects verifies that dynamic commands return
// false safely instead of erroring (the caller records these as denies but
// there is nothing to approve).
func TestCheckApproved_DynamicRejects(t *testing.T) {
	s, _ := newStoreT(t)
	ok, entry, err := s.CheckApproved(`echo $HOME`, "/w", "s")
	if err != nil {
		t.Fatalf("dynamic check: %v", err)
	}
	if ok || entry != nil {
		t.Errorf("dynamic command must yield (false, nil), got (%v, %+v)", ok, entry)
	}
}

// TestApproveByHashPrefix_Unique matches a distinctive prefix.
func TestApproveByHashPrefix_Unique(t *testing.T) {
	s, _ := newStoreT(t)
	h, canon := mustHash(t, `git push origin main`)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w", SessionID: "s"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	appr, _, err := s.ApproveByHashPrefix(h[:8], ApproveOptions{})
	if err != nil {
		t.Fatalf("approve by prefix: %v", err)
	}
	if appr.Hash != h {
		t.Errorf("hash mismatch: %s vs %s", appr.Hash, h)
	}
}

// TestApproveByHashPrefix_TooShort rejects prefixes below the safety minimum.
func TestApproveByHashPrefix_TooShort(t *testing.T) {
	s, _ := newStoreT(t)
	_, _, err := s.ApproveByHashPrefix("ab", ApproveOptions{})
	if err == nil {
		t.Errorf("expected error for short prefix")
	}
}

// TestRevokeAll marks all live approvals consumed.
func TestRevokeAll(t *testing.T) {
	s, _ := newStoreT(t)
	for _, cmd := range []string{`git push origin main`, `rm -rf /tmp/foo`} {
		h, canon := mustHash(t, cmd)
		if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w"}); err != nil {
			t.Fatalf("record: %v", err)
		}
		if _, _, err := s.ApproveLast(ApproveOptions{}); err != nil {
			t.Fatalf("approve: %v", err)
		}
	}
	n, err := s.RevokeAll()
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if n != 2 {
		t.Errorf("want 2 revoked, got %d", n)
	}
	ok, _, err := s.CheckApproved(`git push origin main`, "/w", "")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("revoked approval must not match")
	}
}

// TestConcurrentCheckApproved_NoDoubleConsume verifies that only one of many
// concurrent CheckApproved calls consumes an approval. This is the store
// lock's core guarantee — without it an approval could be spent twice.
func TestConcurrentCheckApproved_NoDoubleConsume(t *testing.T) {
	s, _ := newStoreT(t)
	cmd := `git push origin main`
	h, canon := mustHash(t, cmd)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w", SessionID: "s"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, _, err := s.ApproveLast(ApproveOptions{}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	const goroutines = 12
	var wg sync.WaitGroup
	var consumed int32
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, _, err := s.CheckApproved(cmd, "/w", "s")
			if err != nil {
				t.Errorf("check err: %v", err)
				return
			}
			if ok {
				atomic.AddInt32(&consumed, 1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if consumed != 1 {
		t.Errorf("expected exactly 1 consumer, got %d", consumed)
	}
}

// TestPurge removes expired and consumed entries.
func TestPurge(t *testing.T) {
	s, clock := newStoreT(t)
	h, canon := mustHash(t, `rm -rf /tmp/purge`)
	if err := s.RecordPending(PendingEntry{Hash: h, Command: canon, CWD: "/w"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if _, _, err := s.ApproveLast(ApproveOptions{TTL: 1 * time.Minute}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	clock.advance(2 * time.Minute)
	approvedGone, pendingGone, err := s.Purge(30 * time.Second)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if approvedGone != 1 {
		t.Errorf("want 1 approved removed, got %d", approvedGone)
	}
	if pendingGone != 1 {
		t.Errorf("want 1 pending removed (age > cutoff), got %d", pendingGone)
	}
}
