package app

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
)

// This is the deterministic two-gate ownership interleaving matrix (change 0407,
// spec acceptance items 1, 2, 6, 7). It exercises the acceptance criteria the unit
// tests in rungate_verdict_test.go prove one branch at a time as WHOLE
// interleavings: two gates A and B, each armed before either claim, each bound to
// its own change through the store binding + committed-proof seams Task 3 produces,
// verify only their own change and never each other's — under every ordering of
// completion and verdict, under unrelated corpus churn, under a replacement claim,
// and under a later overwrite attempt. No sleeps: every interleaving is a literal
// call sequence over one temp repo with injected proofs (the app/store boundary).
//
// The RunVerify dispositions are driven by the run-verify fixtures (newRunVerifyFixture
// / rvRecord shapes), generalized here to any (id, slug) that reuses the fixture's
// single pushed feature branch — so two distinct change ids can both report
// run-complete against the one branch the fixture published.

// mxImplementedRecord renders an implemented change (id, slug) whose linkage
// verifies against the shared rv fixture: plan/results at the fixture head, the
// recorded PR, and branch feat/widget (the branch newRunVerifyFixture pushed), so
// RunVerify reports run-complete for any id that reuses that one feature branch.
func mxImplementedRecord(id int, slug string) []byte {
	src := lifecycleChange(id, slug, "in-progress")
	src = strings.Replace(src, "status: in-progress", "status: implemented", 1)
	src = strings.Replace(src, "plan:\n", "plan: '"+rvPlanPath+"'\n", 1)
	src = strings.Replace(src, "results:\n", "results: '"+rvResultsPath+"'\n", 1)
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+rvRecordedPR()+"'\nblocked_by:\n", 1)
	src = strings.Replace(src, "branch: feat/"+slug, "branch: feat/"+rvSlug, 1)
	return []byte(src)
}

// mxIncompleteRecord renders an in-progress (claimed, not-yet-implemented) change
// (id, slug) whose only unmet postcondition is not-implemented — gateIncompleteRecord's
// shape generalized to any id, reusing the fixture's pushed feature branch.
func mxIncompleteRecord(id int, slug string) []byte {
	src := lifecycleChange(id, slug, "in-progress")
	src = strings.Replace(src, "plan:\n", "plan: '"+rvPlanPath+"'\n", 1)
	src = strings.Replace(src, "results:\n", "results: '"+rvResultsPath+"'\n", 1)
	src = strings.Replace(src, "blocked_by:\n", "pr: '"+rvRecordedPR()+"'\nblocked_by:\n", 1)
	src = strings.Replace(src, "branch: feat/"+slug, "branch: feat/"+rvSlug, 1)
	return []byte(src)
}

// mxDispRecord picks the implemented or in-progress record for a disposition word.
func mxDispRecord(id int, slug, disp string) []byte {
	if disp == "complete" {
		return mxImplementedRecord(id, slug)
	}
	return mxIncompleteRecord(id, slug)
}

// mxBlob wraps a change record in the corpus StatusBlob shape at its groom path.
func mxBlob(id int, slug string, record []byte) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  miVersion,
		Data:     record,
	}
}

// mxDeps assembles run-verify deps over the rv fixture for a single change (id,
// slug), mirroring rvFixture.deps but generalized off change 3: the reader supplies
// one record, the fake workspace reports the fixture head, the fake GitHub reports
// the fixture PR, and the real client performs the remote/blob probes. The caller
// attaches wdeps.ClaimProofs.
func mxDeps(t *testing.T, f *rvFixture, id int, slug string, record []byte) (PlanningDeps, WorkspaceDeps, GitHubDeps) {
	t.Helper()
	reader := &fakeReader{
		pin:    f.pin,
		corpus: []StatusBlob{mxBlob(id, slug, record)},
		facts:  domain.NewBranchFacts(nil),
	}
	deps := PlanningDeps{Client: f.client, Reader: reader, Clock: testClock()}
	wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(f.head)}}}
	gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{rvPR(f.head, string(prEvidenceBytes(t, f.head)))}}}
	return deps, wdeps, gdeps
}

// mxBind reserves and confirms a store binding for a dispatched claim — the durable
// (change, request, revision) proof Task 3's ChangeClaim writes on the applied path.
func mxBind(t *testing.T, repoDir, key string, id int, requestID, revision string) {
	t.Helper()
	if err := ReserveGateClaim(repoDir, key, id, requestID); err != nil {
		t.Fatalf("ReserveGateClaim(%d): %v", id, err)
	}
	if err := ConfirmGateClaim(repoDir, key, id, requestID, revision); err != nil {
		t.Fatalf("ConfirmGateClaim(%d): %v", id, err)
	}
}

// mxAssertOwnIDOnly asserts the verdict resolved ownID and that siblingID appears
// in NO field of the report line. The gate key is stripped first: a random gate key
// can itself contain the sibling's digit, so a raw strings.Contains over the whole
// line would false-positive — the only numeric field after the key is the resolved id.
func mxAssertOwnIDOnly(t *testing.T, res RunGateVerdictResult, key string, ownID, siblingID int) {
	t.Helper()
	if res.AttributedID != ownID {
		t.Fatalf("AttributedID = %d, want %d (a gate must verify only its own change)", res.AttributedID, ownID)
	}
	sib := strconv.Itoa(siblingID)
	for _, field := range strings.Fields(strings.ReplaceAll(res.HumanText(), key, "")) {
		if field == sib {
			t.Fatalf("sibling id %d leaked into report line %q", siblingID, res.HumanText())
		}
	}
}

// mxAssertDisposition asserts the verdict's outcome matches the intended
// disposition (a completed bound change reports run-complete; an in-progress one
// reports run-incomplete regardless of the retry decision word).
func mxAssertDisposition(t *testing.T, res RunGateVerdictResult, disp string) {
	t.Helper()
	switch disp {
	case "complete":
		if res.Decision != GateDecisionDone || res.Outcome != VerdictRunComplete {
			t.Fatalf("disposition = %q/%q, want gate-done/run-complete", res.Decision, res.Outcome)
		}
	case "incomplete":
		if res.Outcome != VerdictRunIncomplete {
			t.Fatalf("outcome = %q, want run-incomplete", res.Outcome)
		}
	default:
		t.Fatalf("unknown disposition %q", disp)
	}
}

// TestTwoGatesEachVerifyOnlyTheirOwn — spec acceptance item 1, all four orderings:
// (complete A, verdict A, verdict B), (verdict B, complete A, verdict A), (both
// in-progress), (both complete). Gate A must always report on A's id and gate B on
// B's id; neither line may carry the sibling id.
func TestTwoGatesEachVerifyOnlyTheirOwn(t *testing.T) {
	const (
		idA   = 3
		slugA = "widget"
		idB   = 4
		slugB = "gadget"
	)
	// The shared committed history both gates read: each change's own claim proof,
	// newest-first. Continuity keys each gate on the NEWEST proof for its bound id.
	proofs := []ClaimProof{
		{RequestID: "claim-4-v", ChangeID: idB, GateContextHash: "hb", Revision: "rB"},
		{RequestID: "claim-3-v", ChangeID: idA, GateContextHash: "ha", Revision: "rA"},
	}

	cases := []struct {
		name         string
		dispA, dispB string
		bFirst       bool
	}{
		{"A-completes-first", "complete", "incomplete", false},
		{"B-verdict-first", "complete", "incomplete", true},
		{"both-in-progress", "incomplete", "incomplete", false},
		{"both-complete", "complete", "complete", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRunVerifyFixture(t, true)
			// Both gates armed BEFORE either claim, each bound to its own change.
			keyA := gateMintArmed(t, f.repo.invocation, nil, 1, "ha")
			mxBind(t, f.repo.invocation, keyA, idA, "claim-3-v", "rA")
			keyB := gateMintArmed(t, f.repo.invocation, nil, 1, "hb")
			mxBind(t, f.repo.invocation, keyB, idB, "claim-4-v", "rB")

			runA := func() RunGateVerdictResult {
				deps, wdeps, gdeps := mxDeps(t, f, idA, slugA, mxDispRecord(idA, slugA, tc.dispA))
				wdeps.ClaimProofs = &fakeProofScanner{proofs: proofs}
				return RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, keyA)
			}
			runB := func() RunGateVerdictResult {
				deps, wdeps, gdeps := mxDeps(t, f, idB, slugB, mxDispRecord(idB, slugB, tc.dispB))
				wdeps.ClaimProofs = &fakeProofScanner{proofs: proofs}
				return RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, keyB)
			}

			var resA, resB RunGateVerdictResult
			if tc.bFirst {
				resB = runB()
				resA = runA()
			} else {
				resA = runA()
				resB = runB()
			}

			mxAssertOwnIDOnly(t, resA, keyA, idA, idB)
			mxAssertOwnIDOnly(t, resB, keyB, idB, idA)
			mxAssertDisposition(t, resA, tc.dispA)
			mxAssertDisposition(t, resB, tc.dispB)
		})
	}
}

// TestUnrelatedChurnDoesNotMoveOwnership — spec acceptance item 2 / item 6 first
// half: after binding A→3, mutating the corpus (sibling in-progress claims, a
// refreshed claimed_at on 3, a priority edit) and adding sibling proofs under other
// context hashes leaves the verdict unchanged — same id, same run-complete outcome.
// Ownership rests on the confirmed binding + the exact committed proof, never on the
// current claim set.
func TestUnrelatedChurnDoesNotMoveOwnership(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	key := gateMintArmed(t, f.repo.invocation, nil, 1, "ha")
	mxBind(t, f.repo.invocation, key, 3, "claim-3-v", "rA")

	verdict := func(proofs []ClaimProof, corpus []StatusBlob) RunGateVerdictResult {
		reader := &fakeReader{pin: f.pin, corpus: corpus, facts: domain.NewBranchFacts(nil)}
		deps := PlanningDeps{Client: f.client, Reader: reader, Clock: testClock()}
		wdeps := WorkspaceDeps{
			Service:     &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(f.head)}},
			ClaimProofs: &fakeProofScanner{proofs: proofs},
		}
		gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{rvPR(f.head, string(prEvidenceBytes(t, f.head)))}}}
		return RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	}

	baseProof := ClaimProof{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "rA"}
	baseCorpus := []StatusBlob{mxBlob(3, "widget", mxImplementedRecord(3, "widget"))}

	res1 := verdict([]ClaimProof{baseProof}, baseCorpus)
	if got, want := res1.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("baseline verdict = %q, want %q", got, want)
	}

	// Refresh id 3's claimed_at and priority (a real refresh-claim / groom edit
	// leaves the change complete), and add two sibling in-progress claims — exactly
	// the churn the pre-0407 before-set/epoch inference keyed on.
	refreshed := string(mxImplementedRecord(3, "widget"))
	refreshed = strings.Replace(refreshed, "claimed_at: 2026-08-02T00:00:00Z", "claimed_at: 2026-09-05T00:00:00Z", 1)
	refreshed = strings.Replace(refreshed, "priority: medium", "priority: high", 1)
	churnedCorpus := []StatusBlob{
		mxBlob(3, "widget", []byte(refreshed)),
		gateInProgressBlob(9, "sibling", "2026-09-01T00:00:00Z"),
		gateInProgressBlob(10, "sibling2", "2026-09-02T00:00:00Z"),
	}
	churnedProofs := []ClaimProof{
		{RequestID: "sib-9", ChangeID: 9, GateContextHash: "OTHER"},
		{RequestID: "sib-10", ChangeID: 10, GateContextHash: "OTHER2"},
		baseProof,
	}

	res2 := verdict(churnedProofs, churnedCorpus)
	if got, want := res2.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("post-churn verdict = %q, want %q (ownership must survive corpus churn)", got, want)
	}
	mxAssertOwnIDOnly(t, res2, key, 3, 9)
	mxAssertOwnIDOnly(t, res2, key, 3, 10)
}

// TestReplacementClaimBlocksOldGate — spec acceptance item 6 second half: after A
// is confirmed-bound to change 3 at claim-3-v1, a NEWER committed proof for change 3
// under a different request id means the change was reclaimed and re-claimed by
// another run. A's verdict stops gate-unavailable claim-replaced, never spends the
// retry, and a subsequent verdict still refuses — the old gate never takes over the
// replacement run.
func TestReplacementClaimBlocksOldGate(t *testing.T) {
	repo := newGateRepo(t)
	key := gateMintArmed(t, repo, nil, 1, "ha")
	mxBind(t, repo, key, 3, "claim-3-v1", "r1")

	proofs := []ClaimProof{
		{RequestID: "claim-3-v2", ChangeID: 3, GateContextHash: "hb"},
		{RequestID: "claim-3-v1", ChangeID: 3, GateContextHash: "ha", Revision: "r1"},
	}
	verdict := func() RunGateVerdictResult {
		return RunGateVerdict(context.Background(), PlanningDeps{},
			WorkspaceDeps{ClaimProofs: &fakeProofScanner{proofs: proofs}}, GitHubDeps{}, repo, key)
	}

	res := verdict()
	if res.Decision != GateDecisionStop || res.Outcome != GateOutcomeUnavailable || res.Reason != ReasonGateClaimReplaced {
		t.Fatalf("got %q/%q/%q, want gate-stop/gate-unavailable/%s", res.Decision, res.Outcome, res.Reason, ReasonGateClaimReplaced)
	}
	if !res.Terminal {
		t.Errorf("claim-replaced stop must be terminal")
	}
	if gateRetryMarkerExists(t, repo, key) {
		t.Errorf("a replaced claim must never spend the retry")
	}

	// A subsequent verdict is not a fresh chance to take over the replacement.
	res2 := verdict()
	if res2.Reason != ReasonGateClaimReplaced {
		t.Fatalf("second verdict reason = %q, want %s (still refused)", res2.Reason, ReasonGateClaimReplaced)
	}
	if gateRetryMarkerExists(t, repo, key) {
		t.Errorf("the second refusal must still not spend the retry")
	}
}

// TestLaterVerdictCannotOverwriteBinding — spec acceptance item 7: after A's verdict
// bound and reported change 3 at revision r1, a later confirm carrying a DIFFERENT
// revision is refused binding-conflict, and re-running the verdict resolves the same
// bound id with the binding intact.
func TestLaterVerdictCannotOverwriteBinding(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	key := gateMintArmed(t, f.repo.invocation, nil, 1, "ha")
	mxBind(t, f.repo.invocation, key, 3, "claim-3-v", "r1")

	proofs := []ClaimProof{{RequestID: "claim-3-v", ChangeID: 3, GateContextHash: "ha", Revision: "r1"}}
	verdict := func() RunGateVerdictResult {
		deps, wdeps, gdeps := mxDeps(t, f, 3, "widget", mxImplementedRecord(3, "widget"))
		wdeps.ClaimProofs = &fakeProofScanner{proofs: proofs}
		return RunGateVerdict(context.Background(), deps, wdeps, gdeps, f.repo.invocation, key)
	}

	res1 := verdict()
	if got, want := res1.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("first verdict = %q, want %q", got, want)
	}

	// A later verdict or replay can never overwrite the confirmed binding with a
	// different revision — the store refuses binding-conflict.
	err := ConfirmGateClaim(f.repo.invocation, key, 3, "claim-3-v", "DIFFERENT")
	gse, ok := AsGateStoreError(err)
	if !ok || gse.Kind != ErrGateBindingConflict {
		t.Fatalf("want binding-conflict on a differing-revision confirm, got %v", err)
	}

	res2 := verdict()
	if got, want := res2.HumanText(), "gate-done "+key+" run-complete 3"; got != want {
		t.Fatalf("re-run verdict = %q, want %q (same bound id, binding intact)", got, want)
	}
	b, present, berr := LoadGateClaimBinding(f.repo.invocation, key)
	if berr != nil || !present || b.ChangeID != 3 || b.Revision != "r1" || !b.Confirmed {
		t.Fatalf("binding = %+v present=%v err=%v, want confirmed change 3 @ r1 unchanged", b, present, berr)
	}
}
