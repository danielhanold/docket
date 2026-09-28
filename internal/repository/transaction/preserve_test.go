package transaction

import (
	"strings"
	"testing"
)

// This file is the byte-identical preservation proof for the transaction engine.
// The spec's acceptance boundary requires that a transaction move ONLY the remote
// refs/objects and the private <common>/docket/transactions state — every user
// checkout and index must be byte-for-byte untouched. The two helpers here
// (captureCheckouts / assertCheckoutsUnchanged) snapshot every preserved property
// of a working tree using read-only Git plumbing, and are shared by the
// concurrency and interruption matrices so every scenario also carries a
// preservation assertion. A dedicated dirty-checkout case proves the guarantee
// holds even when the invocation clone carries staged, unstaged, AND untracked
// local work across an applied transaction.

// checkoutState records every preserved property of one working tree, captured
// with read-only plumbing only (rev-parse, symbolic-ref -q, ls-files -s -z,
// status --porcelain=v2 -z, and a whole-tree content hash) so snapshotting can
// never itself perturb what it measures. The working-tree hash (hashTreeExcept,
// which skips .git) catches any file-content change the porcelain status would
// summarize but not fingerprint.
type checkoutState struct {
	symbolic string // symbolic-ref -q HEAD (empty on a detached HEAD)
	head     string // rev-parse HEAD
	index    string // ls-files --stage -z (the full index)
	status   string // status --porcelain=v2 -z --untracked-files=all
	workHash string // content hash of the whole working tree minus .git
}

// captureCheckout snapshots one working tree. symbolic-ref -q returns non-zero on
// a detached HEAD, recorded as the empty string.
func captureCheckout(t *testing.T, dir string) checkoutState {
	t.Helper()
	sym := ""
	if out, err := hgitTry(dir, "symbolic-ref", "-q", "HEAD"); err == nil {
		sym = strings.TrimSpace(out)
	}
	return checkoutState{
		symbolic: sym,
		head:     hgitOut(t, dir, "rev-parse", "HEAD"),
		index:    hgitOutRaw(t, dir, "ls-files", "--stage", "-z"),
		status:   hgitOutRaw(t, dir, "status", "--porcelain=v2", "-z", "--untracked-files=all"),
		workHash: hashTreeExcept(t, dir, nil),
	}
}

// captureCheckouts snapshots several working trees in order, returning one state
// per directory.
func captureCheckouts(t *testing.T, dirs ...string) []checkoutState {
	t.Helper()
	snaps := make([]checkoutState, len(dirs))
	for i, d := range dirs {
		snaps[i] = captureCheckout(t, d)
	}
	return snaps
}

// assertCheckoutsUnchanged re-snapshots each directory and fails, field by field,
// on any drift from its captured state — the proof that a transaction left every
// user checkout byte-identical.
func assertCheckoutsUnchanged(t *testing.T, dirs []string, before []checkoutState) {
	t.Helper()
	if len(dirs) != len(before) {
		t.Fatalf("assertCheckoutsUnchanged: %d dirs vs %d snapshots", len(dirs), len(before))
	}
	for i, dir := range dirs {
		after := captureCheckout(t, dir)
		b := before[i]
		if b.symbolic != after.symbolic {
			t.Errorf("%s: HEAD symbolic ref changed: %q -> %q", dir, b.symbolic, after.symbolic)
		}
		if b.head != after.head {
			t.Errorf("%s: HEAD commit changed: %q -> %q", dir, b.head, after.head)
		}
		if b.index != after.index {
			t.Errorf("%s: index changed", dir)
		}
		if b.status != after.status {
			t.Errorf("%s: working-tree status changed:\nbefore %q\nafter  %q", dir, b.status, after.status)
		}
		if b.workHash != after.workHash {
			t.Errorf("%s: working-tree content changed: %s -> %s", dir, b.workHash, after.workHash)
		}
	}
}
