package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// plainService builds a real gitcli-backed Service with no repository. The
// receipt read/write/clear surface touches only the filesystem, so a bare
// service is all these round-trip proofs need.
func plainService(t *testing.T) *Service {
	t.Helper()
	requireGit(t)
	c, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

// sampleReceipt is a fully valid receipt with distinct, well-formed heads so a
// round-trip proves every field survives byte-identically.
func sampleReceipt() RebaseReceipt {
	return RebaseReceipt{
		RepoIdentity:   "/repo/common.git",
		ChangeID:       "7",
		OrigHead:       strings.Repeat("a", 40),
		OrigRemoteHead: strings.Repeat("b", 40),
		BaseRef:        "refs/heads/main",
		BaseHead:       strings.Repeat("c", 40),
		Attempt:        "attempt-01",
		CreatedUTC:     time.Now().UTC().Format(time.RFC3339),
	}
}

// TestRebaseReceiptRoundTrip proves the three-outcome receipt contract: a
// written receipt reads back equal (found), a cleared receipt reads cleanly
// absent (not found, no error), a second clear on an already-absent receipt is a
// no-op, and a corrupt file on the receipt path is an error — never mistaken for
// clean absence (learnings: probe-error-is-not-clean-absence).
func TestRebaseReceiptRoundTrip(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()
	dir := t.TempDir()
	r := sampleReceipt()

	if err := svc.WriteRebaseReceipt(ctx, dir, r); err != nil {
		t.Fatalf("WriteRebaseReceipt: %v", err)
	}

	got, found, err := svc.ReadRebaseReceipt(ctx, dir)
	if err != nil {
		t.Fatalf("ReadRebaseReceipt after write: %v", err)
	}
	if !found {
		t.Fatalf("ReadRebaseReceipt after write: found=false; want true")
	}
	if got != r {
		t.Errorf("round-trip mismatch:\n got=%+v\nwant=%+v", got, r)
	}

	if err := svc.ClearRebaseReceipt(ctx, dir); err != nil {
		t.Fatalf("ClearRebaseReceipt: %v", err)
	}
	if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
		t.Errorf("ReadRebaseReceipt after clear: found=%v err=%v; want cleanly absent (false, nil)", found, err)
	}
	// A clear on an already-absent receipt is idempotent, not an error.
	if err := svc.ClearRebaseReceipt(ctx, dir); err != nil {
		t.Errorf("ClearRebaseReceipt on absent receipt: %v; want nil", err)
	}

	// A present-but-corrupt file is an error, never clean absence.
	if err := os.WriteFile(filepath.Join(dir, "rebase-receipt.json"), []byte("{ not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err == nil || found {
		t.Errorf("ReadRebaseReceipt on corrupt file: found=%v err=%v; want (false, error)", found, err)
	}
}

// TestRebaseReceiptInvalidFieldsRefused proves the write boundary rejects a
// receipt with a malformed field rather than persisting garbage that a later
// read would have to reject as corrupt.
func TestRebaseReceiptInvalidFieldsRefused(t *testing.T) {
	svc := plainService(t)
	ctx := context.Background()
	dir := t.TempDir()

	bad := sampleReceipt()
	bad.OrigRemoteHead = "not-a-sha"
	if err := svc.WriteRebaseReceipt(ctx, dir, bad); err == nil {
		t.Errorf("WriteRebaseReceipt with malformed head = nil error; want refusal")
	}
	// Nothing was persisted by the refused write.
	if _, found, err := svc.ReadRebaseReceipt(ctx, dir); err != nil || found {
		t.Errorf("receipt present after refused write: found=%v err=%v; want cleanly absent", found, err)
	}
}
