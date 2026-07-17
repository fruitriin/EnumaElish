package main

import (
	"strings"
	"testing"

	"github.com/fruitriin/EnumaElish/internal/approve"
	"github.com/fruitriin/EnumaElish/internal/dsl"
	"github.com/fruitriin/EnumaElish/internal/eval"
)

// TestLookupAndRecord_RoundTrip exercises the two hook-side helpers together:
// record a pending entry, approve it via the store, and confirm that
// lookupApproval consumes it.
func TestLookupAndRecord_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CCCHAIN_APPROVE_STORE", dir)

	// Simulate the hook recording a pending downgrade deny.
	deny := &eval.Result{Action: dsl.ActionDeny, Message: "please confirm push"}
	recordPendingApproval("git push origin main", "/w", "sess-1", deny)

	// Human side: approve --last.
	store := approve.NewStore(dir)
	if _, _, err := store.ApproveLast(approve.ApproveOptions{}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// Second hook call: lookup should consume the approval.
	ok, entry := lookupApproval("git push origin main", "/w", "sess-1")
	if !ok || entry == nil {
		t.Fatalf("lookup should consume approval, got ok=%v entry=%v", ok, entry)
	}

	// Third call: already consumed.
	ok, _ = lookupApproval("git push origin main", "/w", "sess-1")
	if ok {
		t.Errorf("second lookup must return false (one-shot)")
	}
}

// TestRecordPending_DynamicAnnotatesReason verifies that a dynamic command
// does not go into pending but its deny reason gets a "not eligible" note so
// the agent understands why `ccchain approve` is not an option.
func TestRecordPending_DynamicAnnotatesReason(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CCCHAIN_APPROVE_STORE", dir)

	deny := &eval.Result{Action: dsl.ActionDeny, Message: "please confirm push"}
	recordPendingApproval("git push origin $BRANCH", "/w", "sess-1", deny)

	if !strings.Contains(deny.Message, "not eligible for `ccchain approve`") {
		t.Errorf("deny reason should include ineligibility notice; got %q", deny.Message)
	}

	store := approve.NewStore(dir)
	list, err := store.ListPending()
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("dynamic command must not create a pending entry; got %d", len(list))
	}
}

// TestApprovalAllowResult confirms the shape of the allow result the hook
// emits after consuming an approval — the reason includes a short hash so
// operators can correlate audit entries.
func TestApprovalAllowResult(t *testing.T) {
	entry := &approve.ApprovedEntry{Hash: "16f880284c51ff513ff5465f0082c75d9c7ebb186e65e98b4fa362534044846a"}
	r := approvalAllowResult(entry)
	if r.Action != dsl.ActionWarn {
		t.Errorf("action = %q, want warn (drives permissionDecision=allow with reason)", r.Action)
	}
	if !strings.Contains(r.Message, "16f88028") {
		t.Errorf("reason should include short hash prefix; got %q", r.Message)
	}
	if !strings.Contains(r.Message, "one-shot consumed") {
		t.Errorf("reason should explain one-shot semantics; got %q", r.Message)
	}
}
