//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_runcompletion.sh (prefix ^TestIntegrationRunCompletion);
// the race-classified settlement test runs in tests/test_go_integration_app_concurrency.sh
// (prefix ^TestRaceIntegrationAppConcurrency).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestIntegrationRunCompletionAdmissionJournalsPublicationDescriptorAndLegacyDecodes: an admission carrying a
// descriptor persists it verbatim in the journal entry (schema v1, additive field);
// an existing entry WITHOUT the field still decodes (legacy compatibility).
func TestIntegrationRunCompletionAdmissionJournalsPublicationDescriptorAndLegacyDecodes(t *testing.T) {
	fx := newCancelFixture(t, false) // active run bound to the fixture worktree
	pub := &MutationPublication{
		RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	done, err := admitWorkflowMutation(fx.worktree, OperationPRPublish, pub)
	if err != nil {
		t.Fatalf("admitWorkflowMutation: %v", err)
	}
	done(mutationStatusUncertain, false)

	ep, _, lerr := LoadRunRecord(fx.repo, fx.key)
	if lerr != nil {
		t.Fatalf("LoadRunRecord: %v", lerr)
	}
	if len(ep.AdmittedMutations) != 1 {
		t.Fatalf("journal length = %d, want 1", len(ep.AdmittedMutations))
	}
	got := ep.AdmittedMutations[0]
	if got.Status != mutationStatusUncertain || got.OpKey != OperationPRPublish {
		t.Fatalf("entry = %+v, want uncertain pr.publish", got)
	}
	if got.Publication == nil || *got.Publication != *pub {
		t.Fatalf("persisted descriptor = %+v, want %+v", got.Publication, pub)
	}

	// Legacy shape: an entry with no publication field decodes and stays usable.
	if cerr := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted})
		return nil
	}); cerr != nil {
		t.Fatalf("runRecordCAS append legacy: %v", cerr)
	}
	ep2, _, lerr2 := LoadRunRecord(fx.repo, fx.key)
	if lerr2 != nil {
		t.Fatalf("LoadRunRecord after legacy append: %v", lerr2)
	}
	if ep2.AdmittedMutations[1].Publication != nil {
		t.Fatal("legacy entry must decode with a nil descriptor")
	}
}

// TestIntegrationRunCompletionJournaledRetryOutcomeGatesSettlement (change 0444 review blocker): through the
// REAL admission + completion callback, an identical retry settles the uncertain
// original ONLY when its final Result verified the postcondition. A retry resolved
// contended, invalid-state, invalid-input, or internal-error persists completed
// but unverified and leaves the original pending; a verified flag handed to an
// uncertain completion is never persisted; and a legacy completed entry decoded
// without the field is unverified.
func TestIntegrationRunCompletionJournaledRetryOutcomeGatesSettlement(t *testing.T) {
	fx := newCancelFixture(t, false)
	cases := []struct {
		r      Result
		settle bool
	}{
		{ResultApplied, true},
		{ResultNoOp, true},
		{ResultContended, false},
		{ResultInvalidState, false},
		{ResultInvalidInput, false},
		{ResultInternalError, false},
	}
	for n, tc := range cases {
		desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
			HeadRef: "refs/heads/fix/outcome", HeadCommit: fmt.Sprintf("%040x", n+1)}
		od, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &desc)
		if err != nil {
			t.Fatalf("%s: admit original: %v", tc.r, err)
		}
		od(mutationJournalOutcome(ResultExternalFailed))
		rd, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &desc)
		if err != nil {
			t.Fatalf("%s: admit retry: %v", tc.r, err)
		}
		rd(mutationJournalOutcome(tc.r))
		ep, _, err := LoadRunRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("%s: LoadRunRecord: %v", tc.r, err)
		}
		orig, retry := len(ep.AdmittedMutations)-2, len(ep.AdmittedMutations)-1
		if ep.AdmittedMutations[orig].Status != mutationStatusUncertain || ep.AdmittedMutations[orig].Verified {
			t.Fatalf("%s: original = %+v, want uncertain and unverified", tc.r, ep.AdmittedMutations[orig])
		}
		if ep.AdmittedMutations[retry].Status != mutationStatusCompleted || ep.AdmittedMutations[retry].Verified != tc.settle {
			t.Fatalf("%s: retry = %+v, want completed with verified=%v", tc.r, ep.AdmittedMutations[retry], tc.settle)
		}
		if got := publicationRetryMatch(ep, orig); got != tc.settle {
			t.Fatalf("%s: publicationRetryMatch = %v, want %v", tc.r, got, tc.settle)
		}
	}

	// A verified flag handed alongside an uncertain completion is never persisted.
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/uncertain-verified", HeadCommit: fmt.Sprintf("%040x", 99)}
	ud, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	ud(mutationStatusUncertain, true)
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if last := ep.AdmittedMutations[len(ep.AdmittedMutations)-1]; last.Verified {
		t.Fatalf("uncertain entry persisted verified: %+v", last)
	}

	// Legacy decode: a completed entry written before the field existed is
	// unverified and never settles an identical uncertain original.
	var legacy AdmittedMutation
	if err := json.Unmarshal([]byte(`{"op_key":"workspace.publish","status":"completed",`+
		`"publication":{"repo_dir":"/repo/.git","remote":"origin","head_ref":"refs/heads/fix/legacy",`+
		`"head_commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`), &legacy); err != nil {
		t.Fatalf("decode legacy entry: %v", err)
	}
	if legacy.Verified || !validPublication(OperationWorkspacePublish, legacy.Publication) {
		t.Fatalf("legacy entry = %+v, want a valid descriptor and verified=false", legacy)
	}
	orig := AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: legacy.Publication}
	if publicationRetryMatch(RunRecord{AdmittedMutations: []AdmittedMutation{orig, legacy}}, 0) {
		t.Fatal("a legacy completed entry with no verified flag must never settle")
	}
}

// TestIntegrationRunCompletionSettleUncertainPublicationsDurable: settlement re-derives matches under the
// run lock, flips ONLY matched originals uncertain→completed, is idempotent, and
// never touches unmatched entries, participants, or run state.
func TestIntegrationRunCompletionSettleUncertainPublicationsDurable(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	participant := RunParticipant{Kind: "task", NativeHandle: "handle-1", RegisteredAt: "2026-09-23T00:00:00Z"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.State = RunCancelling // settlement is observation of fact; it works on a fenced run
		r.Participants = []RunParticipant{participant}
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationPRPublish, Status: mutationStatusUncertain}, // legacy: stays pending
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	before, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord (before): %v", err)
	}

	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if len(settled) != 1 || settled[0] != "mutation-settled:"+OperationWorkspacePublish {
		t.Fatalf("settled = %v, want [mutation-settled:workspace.publish]", settled)
	}

	ep, gen, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 3 {
		t.Fatalf("journal length = %d, want 3 (settlement never appends or drops entries)", len(ep.AdmittedMutations))
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("matched original must be durably completed — assert the RECORD changed, not a result string")
	}
	if ep.AdmittedMutations[0].OpKey != OperationWorkspacePublish ||
		ep.AdmittedMutations[0].Publication == nil || *ep.AdmittedMutations[0].Publication != desc {
		t.Fatal("settlement must preserve the entry's identity")
	}
	if ep.AdmittedMutations[1].Status != mutationStatusUncertain || ep.AdmittedMutations[1].Publication != nil {
		t.Fatal("legacy descriptor-less entry must remain pending and untouched")
	}
	if ep.AdmittedMutations[2].Status != mutationStatusCompleted ||
		ep.AdmittedMutations[2].Publication == nil || *ep.AdmittedMutations[2].Publication != desc {
		t.Fatal("the settling retry entry must be untouched")
	}
	if ep.State != RunCancelling {
		t.Fatalf("run state = %q; settlement must never transition the run", ep.State)
	}
	if ep.RunID != before.RunID || ep.ChangeID != before.ChangeID || ep.Worktree != before.Worktree {
		t.Fatal("settlement must never touch run identity fields")
	}
	if len(ep.Participants) != 1 || ep.Participants[0] != participant {
		t.Fatalf("participants = %+v; settlement must never touch participants", ep.Participants)
	}

	// Idempotent replay: nothing left to settle, no findings, and NO write — the
	// physical generation does not rotate.
	settled2, findings2 := settleUncertainPublications(fx.repo, fx.key)
	if len(settled2) != 0 || len(findings2) != 0 {
		t.Fatalf("replay settled=%v findings=%v, want none", settled2, findings2)
	}
	_, gen2, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord (replay): %v", err)
	}
	if gen2 != gen {
		t.Fatalf("replay rotated generation %q -> %q; a no-match pass must write nothing", gen, gen2)
	}
}

// TestIntegrationRunCompletionSettleUncertainPublicationsFailureIsBoundedFinding: an unreadable run is a
// bounded finding, never a panic and never a fabricated settlement.
func TestIntegrationRunCompletionSettleUncertainPublicationsFailureIsBoundedFinding(t *testing.T) {
	fx := newCancelFixture(t, false)
	// Corrupt the record so the CAS read fails closed.
	dir := filepath.Join(fx.common, "docket", runTrackerDirName, fx.key)
	if err := os.WriteFile(filepath.Join(dir, runRecordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt record: %v", err)
	}
	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(settled) != 0 {
		t.Fatalf("settled = %v, want none on failure", settled)
	}
	if len(findings) != 1 || findings[0] != "mutation-settle-failed" {
		t.Fatalf("findings = %v, want [mutation-settle-failed]", findings)
	}
}

// TestIntegrationRunCompletionSettleUncertainPublicationsWriteFailureReportsNoSettlement: when matches are
// found under the lock but the atomic write cannot land, the writer reports the
// bounded finding and NO settled tokens (the closure's accumulated tokens are
// discarded), and the durable entry stays uncertain — exclusion is retained.
func TestIntegrationRunCompletionSettleUncertainPublicationsWriteFailureReportsNoSettlement(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	// The lock file already exists (the seed CAS created it); a read-only key dir
	// still lets the CAS lock and read, but the same-directory temp file cannot be
	// created, so the write fails AFTER the match closure ran.
	dir := filepath.Join(fx.common, "docket", runTrackerDirName, fx.key)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod key dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(settled) != 0 {
		t.Fatalf("settled = %v, want none when the write never landed", settled)
	}
	if len(findings) != 1 || findings[0] != "mutation-settle-failed" {
		t.Fatalf("findings = %v, want [mutation-settle-failed]", findings)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("restore key dir: %v", err)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusUncertain {
		t.Fatal("a failed settlement write must leave the original entry uncertain")
	}
}

// TestRaceIntegrationAppConcurrencySettlementNeverDowngradesUnderRacingCallback (change 0444 acceptance 6): a
// completion callback racing the settlement (both under the run CAS) can never
// regress completed→uncertain or lose its own completed write, and an unrelated
// entry appended between match and write is never cleared — the settlement
// re-derives its matches from the fresh record under the lock.
// Race shard (change 0465): eight settlements race the retry completion callback and four fresh admissions per round.
func TestRaceIntegrationAppConcurrencySettlementNeverDowngradesUnderRacingCallback(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	other := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/other", HeadCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// An unrelated admission lands right before settlement runs.
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &other})
		return nil
	}); err != nil {
		t.Fatalf("append racer: %v", err)
	}
	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(findings) != 0 || len(settled) != 1 {
		t.Fatalf("settled=%v findings=%v, want one settlement and no finding", settled, findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.AdmittedMutations[2].Status != mutationStatusAdmitted {
		t.Fatal("the racing unrelated admission must be untouched")
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted || ep.AdmittedMutations[1].Status != mutationStatusCompleted {
		t.Fatal("the matched original must be settled and a completed entry must never be downgraded")
	}

	// Deterministic ordering: settlement BEFORE the retry's completion callback
	// lands settles nothing (the retry is still admitted, so it is no evidence);
	// the callback then lands completed and a repeat settlement converges.
	d2 := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w2", HeadCommit: "cccccccccccccccccccccccccccccccccccccccc"}
	origDone, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &d2)
	if err != nil {
		t.Fatalf("admit original: %v", err)
	}
	origDone(mutationStatusUncertain, false)
	retryDone, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &d2)
	if err != nil {
		t.Fatalf("admit retry: %v", err)
	}
	if s, f := settleUncertainPublications(fx.repo, fx.key); len(s) != 0 || len(f) != 0 {
		t.Fatalf("settled=%v findings=%v before the retry completed, want none", s, f)
	}
	retryDone(mutationStatusCompleted, true)
	if s, f := settleUncertainPublications(fx.repo, fx.key); len(s) != 1 || len(f) != 0 {
		t.Fatalf("settled=%v findings=%v after the retry completed, want one settlement", s, f)
	}
	ep, _, err = LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord (ordering): %v", err)
	}
	for i := 3; i <= 4; i++ {
		if ep.AdmittedMutations[i].Status != mutationStatusCompleted {
			t.Fatalf("entry %d = %q after ordered callback + settlement, want completed", i, ep.AdmittedMutations[i].Status)
		}
	}

	// Real parallelism: each round journals an uncertain original, an in-flight
	// identical retry, and an unrelated in-flight admission through the REAL
	// admission gate, then races eight settlements against the retry's completion
	// callback and four fresh unrelated admissions. The flock-serialized CAS must
	// keep every invariant in every round: the callback's completed write is never
	// lost or downgraded, no racing admission is dropped by a settlement write,
	// the unrelated admissions and every earlier entry are untouched, and the
	// original is only ever uncertain or completed.
	for round := range 12 {
		rd := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
			HeadRef: "refs/heads/fix/round", HeadCommit: fmt.Sprintf("%040x", round+1)}
		ru := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
			HeadRef: "refs/heads/fix/unrelated", HeadCommit: fmt.Sprintf("%040x", round+1)}
		before, _, lerr := LoadRunRecord(fx.repo, fx.key)
		if lerr != nil {
			t.Fatalf("round %d: load: %v", round, lerr)
		}
		base := len(before.AdmittedMutations)
		od, aerr := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &rd)
		if aerr != nil {
			t.Fatalf("round %d: admit original: %v", round, aerr)
		}
		od(mutationStatusUncertain, false)
		rdone, aerr := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &rd)
		if aerr != nil {
			t.Fatalf("round %d: admit retry: %v", round, aerr)
		}
		if _, aerr := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &ru); aerr != nil {
			t.Fatalf("round %d: admit unrelated: %v", round, aerr)
		}

		start := make(chan struct{})
		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			raceFind []string
		)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, f := settleUncertainPublications(fx.repo, fx.key)
				mu.Lock()
				raceFind = append(raceFind, f...)
				mu.Unlock()
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rdone(mutationStatusCompleted, true)
		}()
		// Unrelated admissions APPENDED during the race: a settlement that wrote a
		// record matched outside the lock (a stale snapshot) would silently drop them.
		const racers = 4
		var admitErrs []error
		for k := range racers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				cp := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
					HeadRef: fmt.Sprintf("refs/heads/fix/racer-%d", k), HeadCommit: rd.HeadCommit}
				if _, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &cp); err != nil {
					mu.Lock()
					admitErrs = append(admitErrs, err)
					mu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()
		if len(raceFind) != 0 {
			t.Fatalf("round %d: concurrent settlement findings = %v, want none (the lock serializes, never fails)", round, raceFind)
		}
		if len(admitErrs) != 0 {
			t.Fatalf("round %d: racing admissions failed: %v", round, admitErrs)
		}

		got, _, lerr := LoadRunRecord(fx.repo, fx.key)
		if lerr != nil {
			t.Fatalf("round %d: load after race: %v", round, lerr)
		}
		if len(got.AdmittedMutations) != base+3+racers {
			t.Fatalf("round %d: journal length = %d, want %d (a racing admission was lost)", round, len(got.AdmittedMutations), base+3+racers)
		}
		seen := map[string]bool{}
		for _, m := range got.AdmittedMutations[base+3:] {
			if m.Status != mutationStatusAdmitted || m.Publication == nil {
				t.Fatalf("round %d: racing admission = %+v, want an untouched admitted entry", round, m)
			}
			seen[m.Publication.HeadRef] = true
		}
		if len(seen) != racers {
			t.Fatalf("round %d: racing admissions present = %v, want %d distinct", round, seen, racers)
		}
		for i := range base {
			if got.AdmittedMutations[i].Status != before.AdmittedMutations[i].Status {
				t.Fatalf("round %d: earlier entry %d changed %q -> %q", round, i,
					before.AdmittedMutations[i].Status, got.AdmittedMutations[i].Status)
			}
		}
		if s := got.AdmittedMutations[base].Status; s != mutationStatusUncertain && s != mutationStatusCompleted {
			t.Fatalf("round %d: original = %q, want uncertain or completed", round, s)
		}
		if s := got.AdmittedMutations[base+1].Status; s != mutationStatusCompleted {
			t.Fatalf("round %d: retry = %q after its completion callback, want completed (a lost or downgraded callback write)", round, s)
		}
		if s := got.AdmittedMutations[base+2].Status; s != mutationStatusAdmitted {
			t.Fatalf("round %d: unrelated admission = %q, want admitted (untouched)", round, s)
		}

		// Convergence: whatever the interleaving, one more settlement settles it.
		if _, f := settleUncertainPublications(fx.repo, fx.key); len(f) != 0 {
			t.Fatalf("round %d: convergence findings = %v", round, f)
		}
		conv, _, lerr := LoadRunRecord(fx.repo, fx.key)
		if lerr != nil {
			t.Fatalf("round %d: load after convergence: %v", round, lerr)
		}
		if s := conv.AdmittedMutations[base].Status; s != mutationStatusCompleted {
			t.Fatalf("round %d: original = %q after convergence, want completed", round, s)
		}
	}
}

// TestIntegrationRunCompletionSettlementInterruptionConverges (change 0444 acceptance 6): a settlement
// whose durable write cannot land never lets cancellation claim `cancelled` — the
// entry stays uncertain, exclusion is retained, and the bounded finding names the
// failure — and once the record is writable again, repeating the SAME cancel
// converges. (An interruption AFTER a successful write is a harmless idempotent
// replay, proven by TestIntegrationRunCompletionSettleUncertainPublicationsDurable's replay assert.)
func TestIntegrationRunCompletionSettlementInterruptionConverges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	fx := newCancelFixture(t, true)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A read-only key dir still lets the CAS lock and read, but the same-directory
	// temp file cannot be created, so every run write fails.
	dir := filepath.Join(fx.common, "docket", runTrackerDirName, fx.key)
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	originalStatus := func(when string) string {
		t.Helper()
		ep, _, err := LoadRunRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("LoadRunRecord (%s): %v", when, err)
		}
		return ep.AdmittedMutations[0].Status
	}

	// (a) Unwritable before the cancel: the fence itself cannot land, so the
	// cancel refuses — never cancelled — and the entry and run are untouched.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	pre := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
	if pre.Disposition == CancelDispositionCancelled {
		t.Fatalf("disposition = cancelled with an unwritable run; findings=%v", pre.Findings)
	}
	if s := originalStatus("unwritable fence"); s != mutationStatusUncertain {
		t.Fatalf("original = %q after a failed fence, want uncertain", s)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunActive {
		t.Fatalf("run state = %q after a failed fence, want active (nothing landed)", st)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod back: %v", err)
	}

	// (b) Interrupted between the fence and the settlement: the fence lands, then
	// the store turns unwritable during teardown (the process stop), so ONLY the
	// settlement write fails. Cancellation must stay pending with the bounded
	// finding, report no settlement, and leave the entry uncertain.
	stopper.onStop = func(string) {
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Errorf("chmod mid-teardown: %v", err)
		}
	}
	res := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q with an unpersistable settlement (findings %v), want cancellation-pending", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "mutation-settle-failed") {
		t.Fatalf("findings = %v, want mutation-settle-failed", res.Findings)
	}
	if !hasFinding(res.Findings, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("findings = %v, want mutation-pending:workspace.publish (exclusion retained)", res.Findings)
	}
	if hasFinding(res.Findings, "mutation-settled") {
		t.Fatalf("findings = %v; a settlement that never landed must not be reported", res.Findings)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod back: %v", err)
	}
	if s := originalStatus("interrupted settlement"); s != mutationStatusUncertain {
		t.Fatalf("original = %q after a failed settlement write, want uncertain", s)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling (the fence is durably held)", st)
	}

	// (c) Writable again: the SAME repeat cancel converges.
	stopper.onStop = nil
	res2 := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
	if res2.Disposition != CancelDispositionCancelled {
		t.Fatalf("repeat disposition = %q (findings %v), want cancelled", res2.Disposition, res2.Findings)
	}
	if s := originalStatus("repeat cancel"); s != mutationStatusCompleted {
		t.Fatalf("original = %q after the converged repeat cancel, want completed", s)
	}
}
