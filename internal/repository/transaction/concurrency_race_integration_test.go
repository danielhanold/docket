//go:build integration

package transaction

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// TestRaceIntegrationTxnConcurrencyUnrelatedWritersConverge proves two writers touching different
// records converge: the loser loses its lease, refetches the winner's base,
// replans from fresh state, and applies — its FIRST plan bytes never appear in the
// winning commit, and origin ends with both records.
// Race shard (change 0466): concurrent writers contend on one target branch.
func TestRaceIntegrationTxnConcurrencyUnrelatedWritersConverge(t *testing.T) {
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			h := newConcHarness(t, topo.build)
			before := captureCheckouts(t, h.dirA, h.dirB)

			op1 := &recordOp{id: 5, slug: "writer-x", path: "docs/changes/active/0005-writer-x.md", kind: MutationCreate}
			op2 := &recordOp{id: 6, slug: "writer-y", path: "docs/changes/active/0006-writer-y.md", kind: MutationCreate}
			res1, res2, err1, err2 := h.contendingWriters(t, nil, nil, op1, op2)
			if err1 != nil || res1.Disposition != DispositionApplied {
				t.Fatalf("winner: disposition %q err %v", res1.Disposition, err1)
			}
			if err2 != nil {
				t.Fatalf("loser Execute: %v", err2)
			}
			if res2.Disposition != DispositionApplied {
				t.Fatalf("loser disposition = %q, want applied", res2.Disposition)
			}
			if res2.Attempts != 2 {
				t.Errorf("loser attempts = %d, want 2 (one lease loss then apply)", res2.Attempts)
			}
			if op2.calls != 2 {
				t.Errorf("loser replanned %d times, want 2", op2.calls)
			}

			// Both records converged onto origin.
			names := hgitOut(t, h.r.Origin, "ls-tree", "-r", "--name-only", string(h.r.Target))
			for _, want := range []string{op1.path, op2.path} {
				if !strings.Contains(names, want) {
					t.Errorf("origin missing converged record %q:\n%s", want, names)
				}
			}

			// The loser's FIRST plan bytes (base A, "changes seen: 2") must not appear in
			// the winning commit; the committed blob is the SECOND plan (base B,
			// "changes seen: 3"). This is the proof the retry replanned from fresh state
			// rather than reusing a stale patch.
			committed := originShow(t, h.r, op2.path)
			if committed == string(op2.planBytes[0]) {
				t.Errorf("committed record equals the FIRST (stale) plan — retry reused a patch")
			}
			if committed != string(op2.planBytes[1]) {
				t.Errorf("committed record != the fresh replan bytes:\ncommitted %q\nreplan    %q", committed, op2.planBytes[1])
			}
			if !strings.Contains(committed, "changes seen: 3") {
				t.Errorf("committed record body = %q, want the fresh base's count (3)", committed)
			}

			assertCheckoutsUnchanged(t, []string{h.dirA, h.dirB}, before)
		})
	}
}

// TestRaceIntegrationTxnConcurrencySameEntityContends proves two writers expecting the same blob do
// NOT both win: the loser's retry sees the winner's new blob rather than the one
// it expected and returns contended with no commit of its own.
// Race shard (change 0466): concurrent writers contend on one target branch.
func TestRaceIntegrationTxnConcurrencySameEntityContends(t *testing.T) {
	const rec = "docs/changes/active/0001-first-change.md"
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			h := newConcHarness(t, topo.build)
			before := captureCheckouts(t, h.dirA, h.dirB)
			x1 := h.r.blobID(t, rec)

			// Both writers REPLACE record 1; both expect it at blob X1. The winner sets
			// X2; the loser's retry sees X2 != X1 and contends.
			op1 := &recordOp{id: 1, slug: "first-change", path: rec, kind: MutationReplace}
			op2 := &recordOp{id: 1, slug: "first-change", path: rec, kind: MutationReplace}
			exp := []EntityExpectation{{Path: rec, Revision: ExpectedRevision{Kind: RevisionBlob, ObjectID: x1}}}

			res1, res2, err1, err2 := h.contendingWriters(t, exp, exp, op1, op2)
			if err1 != nil || res1.Disposition != DispositionApplied {
				t.Fatalf("winner: disposition %q err %v (findings %v)", res1.Disposition, err1, res1.Findings)
			}
			if err2 != nil {
				t.Fatalf("loser Execute: %v", err2)
			}
			if res2.Disposition != DispositionContended {
				t.Fatalf("loser disposition = %q, want contended", res2.Disposition)
			}
			if len(res2.ContendedPaths) != 1 || res2.ContendedPaths[0] != gitcli.RepoPath(rec) {
				t.Errorf("loser contended paths = %v, want [%s]", res2.ContendedPaths, rec)
			}
			if res2.Attempts != 2 {
				t.Errorf("loser attempts = %d, want 2 (one lease loss then a contended re-check)", res2.Attempts)
			}

			// The loser committed nothing: origin's record 1 is the winner's body.
			committed := originShow(t, h.r, rec)
			if committed != string(op1.planBytes[len(op1.planBytes)-1]) {
				t.Errorf("origin record 1 is not the winner's bytes:\n%s", committed)
			}
			// The winner made exactly one commit past the shared base; the loser added none.
			assertCheckoutsUnchanged(t, []string{h.dirA, h.dirB}, before)
		})
	}
}

// TestRaceIntegrationTxnConcurrencyDerivedOverlapReplansView proves two writers that both regenerate
// one derived index file converge WITHOUT a text merge: the loser replans the
// derived bytes from fresh state, so the final index reflects both primary
// changes, byte-for-byte the deterministic rendering.
// Race shard (change 0466): concurrent writers contend on one target branch.
func TestRaceIntegrationTxnConcurrencyDerivedOverlapReplansView(t *testing.T) {
	const indexPath = "records-index.txt"
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			h := newConcHarness(t, topo.build)
			before := captureCheckouts(t, h.dirA, h.dirB)

			op1 := &recordOp{id: 5, slug: "writer-x", path: "docs/changes/active/0005-writer-x.md", kind: MutationCreate, indexPath: indexPath}
			op2 := &recordOp{id: 6, slug: "writer-y", path: "docs/changes/active/0006-writer-y.md", kind: MutationCreate, indexPath: indexPath}
			res1, res2, err1, err2 := h.contendingWriters(t, nil, nil, op1, op2)
			if err1 != nil || res1.Disposition != DispositionApplied {
				t.Fatalf("winner: disposition %q err %v", res1.Disposition, err1)
			}
			if err2 != nil || res2.Disposition != DispositionApplied {
				t.Fatalf("loser: disposition %q err %v", res2.Disposition, err2)
			}
			if res2.Attempts != 2 {
				t.Errorf("loser attempts = %d, want 2", res2.Attempts)
			}

			// The final derived view reflects BOTH primary changes, rendered fresh: the
			// two base records (1,2) plus the winner's 5 and the loser's 6, sorted. A
			// text merge would leave conflict markers or drop one side; the exact
			// deterministic rendering proves neither happened.
			want := renderIndex([]int{1, 2, 5, 6})
			got := originShow(t, h.r, indexPath)
			if got != want {
				t.Errorf("derived index = %q, want %q (both primary changes, no merge artifact)", got, want)
			}
			// The loser's FIRST derived bytes (base A: 1,2,6) never reached origin.
			first := string(op2.idxBytes[0])
			if got == first {
				t.Errorf("derived index equals the loser's stale first render %q", first)
			}

			assertCheckoutsUnchanged(t, []string{h.dirA, h.dirB}, before)
		})
	}
}

// TestRaceIntegrationTxnConcurrencyFourLeaseLossesContend proves the attempt cap: a writer whose
// origin is advanced before every one of its pushes makes exactly four attempts,
// returns contended, and leaves its transactions root empty — every candidate was
// cleaned.
// Race shard (change 0466): concurrent writers contend on one target branch.
func TestRaceIntegrationTxnConcurrencyFourLeaseLossesContend(t *testing.T) {
	for _, topo := range concTopologies() {
		t.Run(topo.name, func(t *testing.T) {
			requireGit(t)
			r := topo.build(t)
			client, err := gitcli.NewClient()
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			eng := newEngine(t, client)
			repo, dir := freshClone(t, client, r, "loser")
			before := captureCheckouts(t, dir)

			op := &recordOp{id: 7, slug: "loser", path: "docs/changes/active/0007-loser.md", kind: MutationCreate}
			// On every attempt, advance origin (via the independent writer clone) AFTER
			// the engine has fetched its base but BEFORE it pushes, so the lease always
			// loses. Execute runs in THIS goroutine, so advanceOrigin's t.Fatalf is safe.
			op.barrier = func(call int) {
				adv := call + 10
				r.advanceOrigin(t, fmt.Sprintf("docs/changes/active/00%02d-adv%d.md", adv, call),
					corpusChange(adv, fmt.Sprintf("adv%d", call), "proposed"))
			}

			res, err := eng.Execute(context.Background(), Request{
				Repository: repo, Remote: "origin", TargetRef: r.Target,
				Loader: testLoader{}, Operation: op,
			})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Disposition != DispositionContended {
				t.Fatalf("disposition = %q, want contended", res.Disposition)
			}
			if res.Attempts != maxAttempts {
				t.Errorf("attempts = %d, want %d", res.Attempts, maxAttempts)
			}
			if op.calls != maxAttempts {
				t.Errorf("operation planned %d times, want %d", op.calls, maxAttempts)
			}
			if !transactionsEmpty(t, repo) {
				t.Error("transactions root not empty after four cleaned lease losses")
			}
			// The loser's own record never reached origin.
			names := hgitOut(t, r.Origin, "ls-tree", "-r", "--name-only", string(r.Target))
			if strings.Contains(names, op.path) {
				t.Errorf("loser's record reached origin despite contention:\n%s", names)
			}
			assertCheckoutsUnchanged(t, []string{dir}, before)
		})
	}
}
