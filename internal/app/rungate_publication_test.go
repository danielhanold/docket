package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestPublicationDigestDomainSeparated: digests are sha256-hex, label-separated with
// an unambiguous NUL boundary so ("a","bc") can never collide with ("ab","c") or a
// different label over the same bytes.
func TestPublicationDigestDomainSeparated(t *testing.T) {
	d1 := publicationDigest("pr-title", "Add widget")
	if len(d1) != 64 {
		t.Fatalf("digest length = %d, want 64 hex chars", len(d1))
	}
	if d1 != publicationDigest("pr-title", "Add widget") {
		t.Fatal("digest must be deterministic")
	}
	if d1 == publicationDigest("pr-body", "Add widget") {
		t.Fatal("different labels over the same value must not collide")
	}
	if publicationDigest("l", "ab") == publicationDigest("la", "b") {
		t.Fatal("label/value boundary must be unambiguous")
	}
}

// TestValidPublicationPerOp: validation enforces operation/descriptor agreement —
// each op requires its own fields non-empty AND the other op's fields empty; an
// unknown op is never valid; nil is never valid.
func TestValidPublicationPerOp(t *testing.T) {
	pr := &MutationPublication{
		RepoHost: "github.com", RepoOwner: "danielhanold", RepoName: "docket",
		HeadRef: "fix/widget", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	ws := &MutationPublication{
		RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/widget", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if !validPublication(OperationPRPublish, pr) {
		t.Fatal("complete PR descriptor must validate")
	}
	if !validPublication(OperationWorkspacePublish, ws) {
		t.Fatal("complete workspace descriptor must validate")
	}
	if validPublication(OperationPRPublish, nil) || validPublication(OperationWorkspacePublish, nil) {
		t.Fatal("nil descriptor is never valid")
	}
	if validPublication("metadata.transaction", pr) {
		t.Fatal("unknown operation is never valid")
	}
	// Each required PR field, blanked, invalidates.
	for _, blank := range []func(*MutationPublication){
		func(p *MutationPublication) { p.RepoHost = "" },
		func(p *MutationPublication) { p.RepoOwner = "" },
		func(p *MutationPublication) { p.RepoName = "" },
		func(p *MutationPublication) { p.HeadRef = "" },
		func(p *MutationPublication) { p.HeadCommit = "" },
		func(p *MutationPublication) { p.BaseBranch = "" },
		func(p *MutationPublication) { p.TitleDigest = "" },
		func(p *MutationPublication) { p.BodyDigest = "" },
	} {
		c := *pr
		blank(&c)
		if validPublication(OperationPRPublish, &c) {
			t.Fatalf("PR descriptor with a blanked required field must not validate: %+v", c)
		}
	}
	// Agreement: a PR descriptor carrying workspace-only fields (and vice versa) is
	// op/descriptor disagreement and invalid.
	crossPR := *pr
	crossPR.Remote = "origin"
	if validPublication(OperationPRPublish, &crossPR) {
		t.Fatal("PR descriptor carrying a workspace-only field must not validate")
	}
	crossWS := *ws
	crossWS.TitleDigest = "x"
	if validPublication(OperationWorkspacePublish, &crossWS) {
		t.Fatal("workspace descriptor carrying a PR-only field must not validate")
	}
	for _, blank := range []func(*MutationPublication){
		func(p *MutationPublication) { p.RepoDir = "" },
		func(p *MutationPublication) { p.Remote = "" },
		func(p *MutationPublication) { p.HeadRef = "" },
		func(p *MutationPublication) { p.HeadCommit = "" },
	} {
		c := *ws
		blank(&c)
		if validPublication(OperationWorkspacePublish, &c) {
			t.Fatalf("workspace descriptor with a blanked required field must not validate: %+v", c)
		}
	}
}

// TestAdmissionJournalsPublicationDescriptorAndLegacyDecodes: an admission carrying a
// descriptor persists it verbatim in the journal entry (schema v1, additive field);
// an existing entry WITHOUT the field still decodes (legacy compatibility).
func TestAdmissionJournalsPublicationDescriptorAndLegacyDecodes(t *testing.T) {
	fx := newCancelFixture(t, false) // active epoch bound to the fixture worktree
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

	ep, _, lerr := LoadEpochRecord(fx.repo, fx.key)
	if lerr != nil {
		t.Fatalf("LoadEpochRecord: %v", lerr)
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
	if cerr := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted})
		return nil
	}); cerr != nil {
		t.Fatalf("epochCAS append legacy: %v", cerr)
	}
	ep2, _, lerr2 := LoadEpochRecord(fx.repo, fx.key)
	if lerr2 != nil {
		t.Fatalf("LoadEpochRecord after legacy append: %v", lerr2)
	}
	if ep2.AdmittedMutations[1].Publication != nil {
		t.Fatal("legacy entry must decode with a nil descriptor")
	}
}

// TestPublicationRetryMatchMatrix: an uncertain entry is settled ONLY by a later
// (higher-index) completed AND verified entry with the same operation and a
// field-for-field identical VALID descriptor. Everything else leaves it pending.
func TestPublicationRetryMatchMatrix(t *testing.T) {
	base := MutationPublication{
		RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	alter := func(f func(*MutationPublication)) *MutationPublication {
		c := base
		f(&c)
		return &c
	}
	rec := func(entries ...AdmittedMutation) EpochRecord {
		return EpochRecord{AdmittedMutations: entries}
	}
	uncertain := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &base}

	cases := []struct {
		name string
		rec  EpochRecord
		want bool
	}{
		{"identical completed retry settles", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}), true},
		{"no later entry", rec(uncertain), false},
		{"later identical but admitted", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &base}), false},
		{"later identical but uncertain", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &base}), false},
		{"identical completed at LOWER index never settles", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &base},
			uncertain), false},
		{"different operation", rec(uncertain,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}), false},
		{"missing descriptor on the retry", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true}), false},
		// Change 0444 review blocker: a completed retry that did NOT verify its
		// postcondition (contended, a local refusal, an internal error) or a legacy
		// completed entry with no verified flag is never settling evidence — missing
		// evidence never counts as success.
		{"identical completed but unverified retry", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Publication: &base}), false},
		{"verified flag on a non-completed retry never settles", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Verified: true, Publication: &base}), false},
		{"verified flag on an uncertain retry never settles", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Verified: true, Publication: &base}), false},
		{"missing descriptor on the original (legacy)", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain},
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}), false},
		{"malformed descriptor on the retry", rec(uncertain,
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
				Publication: alter(func(p *MutationPublication) { p.BaseBranch = "" })}), false},
		{"different owner", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
			Publication: alter(func(p *MutationPublication) { p.RepoOwner = "other" })}), false},
		{"different head commit", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
			Publication: alter(func(p *MutationPublication) { p.HeadCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" })}), false},
		{"different head branch", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
			Publication: alter(func(p *MutationPublication) { p.HeadRef = "fix/other" })}), false},
		{"different base", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
			Publication: alter(func(p *MutationPublication) { p.BaseBranch = "develop" })}), false},
		{"different title digest", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
			Publication: alter(func(p *MutationPublication) { p.TitleDigest = publicationDigest("pr-title", "T2") })}), false},
		{"different body digest", rec(uncertain, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true,
			Publication: alter(func(p *MutationPublication) { p.BodyDigest = publicationDigest("pr-body", "B2") })}), false},
		{"eligible only when uncertain: completed original is not a match target", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &base},
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}), false},
		{"still-admitted original is not eligible", rec(
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &base},
			AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}), false},
	}
	for _, tc := range cases {
		if got := publicationRetryMatch(tc.rec, 0); got != tc.want {
			t.Errorf("%s: match = %v, want %v", tc.name, got, tc.want)
		}
	}

	// Workspace analog: identical settles, a differing remote/ref/repo-dir does not.
	wsBase := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	wsUncertain := AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &wsBase}
	if !publicationRetryMatch(rec(wsUncertain,
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &wsBase}), 0) {
		t.Error("identical completed workspace retry must settle")
	}
	for name, f := range map[string]func(*MutationPublication){
		"remote":   func(p *MutationPublication) { p.Remote = "backup" },
		"ref":      func(p *MutationPublication) { p.HeadRef = "refs/heads/other" },
		"repo dir": func(p *MutationPublication) { p.RepoDir = "/elsewhere/.git" },
		"head":     func(p *MutationPublication) { p.HeadCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" },
	} {
		c := wsBase
		f(&c)
		if publicationRetryMatch(rec(wsUncertain,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &c}), 0) {
			t.Errorf("workspace retry differing in %s must not settle", name)
		}
	}
}

// TestMutationJournalOutcomeVerifiesOnlyObservedPostcondition (change 0444 review
// blocker): only applied and no-op verify the postcondition; contended, every local
// refusal, and an internal error are completed-for-accounting but UNVERIFIED; an
// external failure or interruption is uncertain and unverified. An unknown future
// Result is unverified (fail-safe).
func TestMutationJournalOutcomeVerifiesOnlyObservedPostcondition(t *testing.T) {
	cases := []struct {
		r        Result
		status   string
		verified bool
	}{
		{ResultApplied, mutationStatusCompleted, true},
		{ResultNoOp, mutationStatusCompleted, true},
		{ResultContended, mutationStatusCompleted, false},
		{ResultInvalidInput, mutationStatusCompleted, false},
		{ResultInvalidState, mutationStatusCompleted, false},
		{ResultBlocked, mutationStatusCompleted, false},
		{ResultUnsupportedConfig, mutationStatusCompleted, false},
		{ResultGateFailed, mutationStatusCompleted, false},
		{ResultInternalError, mutationStatusCompleted, false},
		{ResultExternalFailed, mutationStatusUncertain, false},
		{ResultInterrupted, mutationStatusUncertain, false},
		{Result("some-future-result"), mutationStatusCompleted, false},
	}
	for _, tc := range cases {
		status, verified := mutationJournalOutcome(tc.r)
		if status != tc.status || verified != tc.verified {
			t.Errorf("mutationJournalOutcome(%q) = (%q, %v), want (%q, %v)", tc.r, status, verified, tc.status, tc.verified)
		}
	}
}

// TestJournaledRetryOutcomeGatesSettlement (change 0444 review blocker): through the
// REAL admission + completion callback, an identical retry settles the uncertain
// original ONLY when its final Result verified the postcondition. A retry resolved
// contended, invalid-state, invalid-input, or internal-error persists completed
// but unverified and leaves the original pending; a verified flag handed to an
// uncertain completion is never persisted; and a legacy completed entry decoded
// without the field is unverified.
func TestJournaledRetryOutcomeGatesSettlement(t *testing.T) {
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
		ep, _, err := LoadEpochRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("%s: LoadEpochRecord: %v", tc.r, err)
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
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
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
	if publicationRetryMatch(EpochRecord{AdmittedMutations: []AdmittedMutation{orig, legacy}}, 0) {
		t.Fatal("a legacy completed entry with no verified flag must never settle")
	}
}

// TestSettleablePublicationIndexes: collects every settleable index, ascending, and
// nothing else.
func TestSettleablePublicationIndexes(t *testing.T) {
	base := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	r := EpochRecord{AdmittedMutations: []AdmittedMutation{
		{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &base},                 // 0: settleable
		{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain},                                     // 1: legacy, pending
		{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}, // 2: the retry
	}}
	got := settleablePublicationIndexes(r)
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("settleable = %v, want [0]", got)
	}
}

// TestSettleUncertainPublicationsDurable: settlement re-derives matches under the
// epoch lock, flips ONLY matched originals uncertain→completed, is idempotent, and
// never touches unmatched entries, participants, or epoch state.
func TestSettleUncertainPublicationsDurable(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	participant := EpochParticipant{Kind: "task", NativeHandle: "handle-1", RegisteredAt: "2026-09-23T00:00:00Z"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.State = EpochCancelling // settlement is observation of fact; it works on a fenced epoch
		r.Participants = []EpochParticipant{participant}
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationPRPublish, Status: mutationStatusUncertain}, // legacy: stays pending
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	before, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord (before): %v", err)
	}

	settled, findings := settleUncertainPublications(fx.repo, fx.key)
	if len(findings) != 0 {
		t.Fatalf("findings = %v, want none", findings)
	}
	if len(settled) != 1 || settled[0] != "mutation-settled:"+OperationWorkspacePublish {
		t.Fatalf("settled = %v, want [mutation-settled:workspace.publish]", settled)
	}

	ep, gen, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
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
	if ep.State != EpochCancelling {
		t.Fatalf("epoch state = %q; settlement must never transition the epoch", ep.State)
	}
	if ep.EpochID != before.EpochID || ep.ChangeID != before.ChangeID || ep.Worktree != before.Worktree {
		t.Fatal("settlement must never touch epoch identity fields")
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
	_, gen2, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord (replay): %v", err)
	}
	if gen2 != gen {
		t.Fatalf("replay rotated generation %q -> %q; a no-match pass must write nothing", gen, gen2)
	}
}

// TestSettleUncertainPublicationsFailureIsBoundedFinding: an unreadable epoch is a
// bounded finding, never a panic and never a fabricated settlement.
func TestSettleUncertainPublicationsFailureIsBoundedFinding(t *testing.T) {
	fx := newCancelFixture(t, false)
	// Corrupt the record so the CAS read fails closed.
	dir := filepath.Join(fx.common, "docket", "rungate", fx.key)
	if err := os.WriteFile(filepath.Join(dir, epochRecordFileName), []byte("{not json"), 0o600); err != nil {
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

// TestSettleUncertainPublicationsWriteFailureReportsNoSettlement: when matches are
// found under the lock but the atomic write cannot land, the writer reports the
// bounded finding and NO settled tokens (the closure's accumulated tokens are
// discarded), and the durable entry stays uncertain — exclusion is retained.
func TestSettleUncertainPublicationsWriteFailureReportsNoSettlement(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
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
	dir := filepath.Join(fx.common, "docket", "rungate", fx.key)
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
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusUncertain {
		t.Fatal("a failed settlement write must leave the original entry uncertain")
	}
}

// TestSettlementNeverDowngradesUnderRacingCallback (change 0444 acceptance 6): a
// completion callback racing the settlement (both under the epoch CAS) can never
// regress completed→uncertain or lose its own completed write, and an unrelated
// entry appended between match and write is never cleared — the settlement
// re-derives its matches from the fresh record under the lock.
func TestSettlementNeverDowngradesUnderRacingCallback(t *testing.T) {
	fx := newCancelFixture(t, false)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	other := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/other", HeadCommit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// An unrelated admission lands right before settlement runs.
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
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
	ep, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
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
	ep, _, err = LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord (ordering): %v", err)
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
		before, _, lerr := LoadEpochRecord(fx.repo, fx.key)
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

		got, _, lerr := LoadEpochRecord(fx.repo, fx.key)
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
		conv, _, lerr := LoadEpochRecord(fx.repo, fx.key)
		if lerr != nil {
			t.Fatalf("round %d: load after convergence: %v", round, lerr)
		}
		if s := conv.AdmittedMutations[base].Status; s != mutationStatusCompleted {
			t.Fatalf("round %d: original = %q after convergence, want completed", round, s)
		}
	}
}

// TestSettlementInterruptionConverges (change 0444 acceptance 6): a settlement
// whose durable write cannot land never lets cancellation claim `cancelled` — the
// entry stays uncertain, exclusion is retained, and the bounded finding names the
// failure — and once the record is writable again, repeating the SAME cancel
// converges. (An interruption AFTER a successful write is a harmless idempotent
// replay, proven by TestSettleUncertainPublicationsDurable's replay assert.)
func TestSettlementInterruptionConverges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission")
	}
	fx := newCancelFixture(t, true)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A read-only key dir still lets the CAS lock and read, but the same-directory
	// temp file cannot be created, so every epoch write fails.
	dir := filepath.Join(fx.common, "docket", "rungate", fx.key)
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	originalStatus := func(when string) string {
		t.Helper()
		ep, _, err := LoadEpochRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("LoadEpochRecord (%s): %v", when, err)
		}
		return ep.AdmittedMutations[0].Status
	}

	// (a) Unwritable before the cancel: the fence itself cannot land, so the
	// cancel refuses — never cancelled — and the entry and epoch are untouched.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	pre := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if pre.Disposition == CancelDispositionCancelled {
		t.Fatalf("disposition = cancelled with an unwritable epoch; findings=%v", pre.Findings)
	}
	if s := originalStatus("unwritable fence"); s != mutationStatusUncertain {
		t.Fatalf("original = %q after a failed fence, want uncertain", s)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochActive {
		t.Fatalf("epoch state = %q after a failed fence, want active (nothing landed)", st)
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
	res := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
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
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCancelling {
		t.Fatalf("epoch state = %q, want cancelling (the fence is durably held)", st)
	}

	// (c) Writable again: the SAME repeat cancel converges.
	stopper.onStop = nil
	res2 := runCancel(seams, fx.repo, fx.key, fx.epochID, "human stop")
	if res2.Disposition != CancelDispositionCancelled {
		t.Fatalf("repeat disposition = %q (findings %v), want cancelled", res2.Disposition, res2.Findings)
	}
	if s := originalStatus("repeat cancel"); s != mutationStatusCompleted {
		t.Fatalf("original = %q after the converged repeat cancel, want completed", s)
	}
}
