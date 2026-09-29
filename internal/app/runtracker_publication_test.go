package app

import (
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
	rec := func(entries ...AdmittedMutation) RunRecord {
		return RunRecord{AdmittedMutations: entries}
	}
	uncertain := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &base}

	cases := []struct {
		name string
		rec  RunRecord
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

// TestSettleablePublicationIndexes: collects every settleable index, ascending, and
// nothing else.
func TestSettleablePublicationIndexes(t *testing.T) {
	base := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	r := RunRecord{AdmittedMutations: []AdmittedMutation{
		{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &base},                 // 0: settleable
		{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain},                                     // 1: legacy, pending
		{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &base}, // 2: the retry
	}}
	got := settleablePublicationIndexes(r)
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("settleable = %v, want [0]", got)
	}
}
