//go:build integration

package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"strings"
	"testing"
)

// TestFinalizeMergeAdminGate proves --admin is honored only on an attended,
// explicitly-named run, is never inferred, and that a denial stays denied
// (never retried with admin).
func TestIntegrationFinalizeMergeAdminGate(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("admin-honored-with-explicit-id", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, true))
		if res.Result != ResultApplied || res.Merge == nil {
			t.Fatalf("admin merge with explicit id = %q (reason %q), want applied", res.Result, res.Reason)
		}
		if !gh.lastMergeAdmin {
			t.Fatalf("admin flag was not passed through to the merge effect")
		}
	})

	t.Run("admin-refused-without-explicit-id", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, false, true))
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("admin without explicit id reported success %q", res.Result)
		}
		if res.Reason != ReasonMergeAdminNotAuthorized {
			t.Fatalf("reason = %q, want %q", res.Reason, ReasonMergeAdminNotAuthorized)
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("admin-without-explicit-id issued %d merge call(s); want 0", gh.mergeCalls)
		}
	})

	t.Run("denied-stays-denied-no-admin-retry", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeDenied
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Disposition != MergeDispDenied {
			t.Fatalf("disposition = %q, want %q (result %q)", res.Disposition, MergeDispDenied, res.Result)
		}
		if res.Merge != nil {
			t.Fatalf("a denial carried a VerifiedMerge")
		}
		if gh.mergeCalls != 1 {
			t.Fatalf("a denial issued %d merge call(s); want exactly 1 (no admin retry)", gh.mergeCalls)
		}
		if gh.lastMergeAdmin {
			t.Fatalf("a denial was retried with admin; admin must never be inferred")
		}
	})
}

// TestFinalizeMergeAlreadyMergedNoop proves an already-merged exact PR is a
// verified no-op regardless of who merged it, issuing no second merge.
func TestIntegrationFinalizeMergeAlreadyMergedNoop(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupMergeFixture(t, m)
	mergeCommit := f.mergeFeatureIntoBase(t)
	gh := f.baselineFake(t)
	gh.probeOutcome = githubcli.MergeAlreadyMerged
	gh.probeFacts = mergedFactsFor(f.head, "main", mergeCommit)

	res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultNoOp || res.Disposition != MergeDispAlreadyMerged || res.Merge == nil {
		t.Fatalf("already-merged = %q disp %q merge %v (reason %q), want no-op/already-merged verified", res.Result, res.Disposition, res.Merge, res.Reason)
	}
	if res.Merge.MergeCommit != mergeCommit {
		t.Fatalf("VerifiedMerge merge commit = %q, want %q", res.Merge.MergeCommit, mergeCommit)
	}
	if gh.mergeCalls != 0 {
		t.Fatalf("an already-merged PR issued %d merge call(s); want 0 (never a second merge)", gh.mergeCalls)
	}
}

// TestFinalizeMergeAlreadyMergedOmitsMethod proves already-merged recovery
// surfaces no method (Docket did not choose the historical merge) and the
// omitempty tag keeps the "method" key out of the document entirely.
func TestIntegrationFinalizeMergeAlreadyMergedOmitsMethod(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupMergeFixture(t, m)
	mergeCommit := f.mergeFeatureIntoBase(t)
	gh := f.baselineFake(t)
	gh.probeOutcome = githubcli.MergeAlreadyMerged
	gh.probeFacts = mergedFactsFor(f.head, "main", mergeCommit)
	res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultNoOp || res.Merge == nil {
		t.Fatalf("already-merged = %q merge %v (reason %q)", res.Result, res.Merge, res.Reason)
	}
	if res.Method != "" {
		t.Fatalf("already-merged recovery carried an attempted method %q", res.Method)
	}
	doc, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(doc), "\"method\"") {
		t.Fatalf("document carries a \"method\" key despite an empty method: %s", doc)
	}
}

// TestFinalizeMergeConjunctAssembly proves the pure conjunct assembly maps each
// falsified input to exactly its closed token and holds only when every input is
// satisfied. This is the exhaustive per-field oracle; the operation-level test
// proves the recheck-before-effect wiring.
func TestIntegrationFinalizeMergeConjunctAssembly(t *testing.T) {
	const head = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	good := mergeConjunctInputs{
		status:            domain.StatusImplemented,
		canonicalPRNumber: 7, prNumber: 7,
		reqHead: head, prHead: head, remoteHead: head, localHead: head,
		prState: githubcli.StateOpen, prDraft: false,
		prBase: "main", effectiveBase: "main",
		gateOff: false, evidenceGreen: true, evidenceHead: head,
		explicitID: true, requireApproval: false,
		unretargetedOpenChildren: 0,
		versionMatches:           true, finalizeBlocked: false,
	}
	if got := mergeConjuncts(good).FirstFailure(); got != "" {
		t.Fatalf("a fully-satisfied input failed conjunct %q", got)
	}

	cases := []struct {
		name  string
		mut   func(in *mergeConjunctInputs)
		token string
	}{
		{"not-implemented", func(in *mergeConjunctInputs) { in.status = domain.StatusInProgress }, "not-implemented"},
		{"pr-identity", func(in *mergeConjunctInputs) { in.prNumber = 9 }, "pr-identity-mismatch"},
		{"head-pr", func(in *mergeConjunctInputs) { in.prHead = "deadbeef" }, "head-moved"},
		{"head-remote", func(in *mergeConjunctInputs) { in.remoteHead = "deadbeef" }, "head-moved"},
		{"head-local", func(in *mergeConjunctInputs) { in.localHead = "deadbeef" }, "head-moved"},
		{"closed", func(in *mergeConjunctInputs) { in.prState = githubcli.StateClosed }, "not-open-nondraft"},
		{"draft", func(in *mergeConjunctInputs) { in.prDraft = true }, "not-open-nondraft"},
		{"base", func(in *mergeConjunctInputs) { in.prBase = "develop" }, "base-mismatch"},
		{"gate-no-evidence", func(in *mergeConjunctInputs) { in.evidenceGreen = false }, "gate-unsatisfied"},
		{"gate-stale-evidence", func(in *mergeConjunctInputs) { in.evidenceHead = "other" }, "gate-unsatisfied"},
		{"approval", func(in *mergeConjunctInputs) { in.explicitID = false; in.requireApproval = true }, "approval-required"},
		{"open-children", func(in *mergeConjunctInputs) { in.unretargetedOpenChildren = 1 }, "open-children"},
		{"superseded-version", func(in *mergeConjunctInputs) { in.versionMatches = false }, "superseded"},
		{"superseded-blocked", func(in *mergeConjunctInputs) { in.explicitID = false; in.finalizeBlocked = true }, "superseded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := good
			tc.mut(&in)
			if got := mergeConjuncts(in).FirstFailure(); got != tc.token {
				t.Fatalf("FirstFailure = %q, want %q", got, tc.token)
			}
		})
	}

	// The overridable conjuncts: an explicit id satisfies approval and a
	// finalize-blocked marker, but never a superseding version.
	t.Run("explicit-id-overrides-approval", func(t *testing.T) {
		in := good
		in.explicitID = true
		in.requireApproval = true
		if got := mergeConjuncts(in).FirstFailure(); got != "" {
			t.Fatalf("explicit id did not satisfy approval: %q", got)
		}
	})
	t.Run("explicit-id-overrides-blocked", func(t *testing.T) {
		in := good
		in.explicitID = true
		in.finalizeBlocked = true
		if got := mergeConjuncts(in).FirstFailure(); got != "" {
			t.Fatalf("explicit id did not satisfy the finalize-blocked marker: %q", got)
		}
	})
	t.Run("explicit-id-never-overrides-version", func(t *testing.T) {
		in := good
		in.explicitID = true
		in.versionMatches = false
		if got := mergeConjuncts(in).FirstFailure(); got != "superseded" {
			t.Fatalf("explicit id wrongly overrode a superseding version: %q", got)
		}
	})
}

// TestFinalizeMergeConjunctsRechecked proves the operation rechecks each merge
// conjunct from a FRESH reload immediately before the effect: a falsified field
// refuses with that field's closed token and issues zero merge calls.
func TestIntegrationFinalizeMergeConjunctsRechecked(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	// Cases achievable by perturbing the live fake/request over a shared baseline
	// fixture (no metadata rewrite needed).
	t.Run("pr-identity-mismatch", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug] = []githubcli.PullRequest{func() githubcli.PullRequest {
			pr := f.parentPR(f.head, greenEvidenceFor(t, f.head))
			pr.Number = 9 // not the canonical #7
			return pr
		}()}
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "pr-identity-mismatch")
	})

	t.Run("head-moved", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		other := strings.Repeat("b", 40)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, other, true, false))
		assertMergeRefusal(t, res, gh, "head-moved")
	})

	t.Run("draft", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Draft = true
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "not-open-nondraft")
	})

	t.Run("base-mismatch", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].BaseBranch = "develop"
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "base-mismatch")
	})

	t.Run("gate-unsatisfied", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Body = "" // no green evidence; gate is local
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "gate-unsatisfied")
	})

	// Metadata-shaped cases: a stale version, a durable finalize-blocked marker,
	// and a not-implemented status.
	t.Run("superseded-version", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		req := mergeReq(f, f.head, true, false)
		req.Version = "sha256:" + strings.Repeat("f", 64) // stale; explicit id never overrides a version
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, req)
		assertMergeRefusal(t, res, gh, "superseded")
	})

	t.Run("superseded-finalize-blocked", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\nBlocked pending a decision.")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, false, false))
		assertMergeRefusal(t, res, gh, "superseded")
	})

	t.Run("not-implemented", func(t *testing.T) {
		// A claimed-but-not-yet-implemented record: it carries a recorded branch (so
		// identity resolves), and the Implemented conjunct is what refuses.
		f := setupMergeFixture(t, m)
		f.patchParent(t, "in-progress", mergePRRef(), "")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "not-implemented")
	})

	t.Run("unclaimed-branch-missing", func(t *testing.T) {
		// A proposed (never-claimed) record has no recorded branch, so the merge
		// fails closed on identity before any external effect — never a second merge
		// and never a reconstruction of the branch from the slug.
		f := setupMergeFixture(t, m)
		f.patchParent(t, "proposed", mergePRRef(), "")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "branch-missing")
		if res.Result != ResultInvalidState {
			t.Fatalf("unclaimed merge result = %q, want invalid-state", res.Result)
		}
	})

	t.Run("unretargeted-open-child", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.repo.writerAdvance(t, f.branch, map[string]string{groomPath(6, "gadget"): childRecord(6, "gadget", f.id, "github.com/acme/widget#8")})
		gh := f.baselineFake(t)
		// The child's open PR still targets the parent's feature branch: unretargeted.
		gh.openByHead["feat/gadget"] = []githubcli.PullRequest{{
			Number: 8, State: githubcli.StateOpen, HeadBranch: "feat/gadget",
			HeadCommit: strings.Repeat("c", 40), BaseBranch: "feat/" + f.slug,
			Version: "sha256:" + strings.Repeat("e", 64),
		}}
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "open-children")
	})
}

// TestFinalizeMergeDeniedCarriesMethod proves an authoritative denial still
// records the attempted method (the merge command WAS issued) while keeping the
// unchanged external-failed/denied/merge-denied mapping.
func TestIntegrationFinalizeMergeDeniedCarriesMethod(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupMergeFixture(t, m)
	gh := f.baselineFake(t)
	gh.mergeOutcome = githubcli.MergeDenied
	gh.mergeMethod = githubcli.MethodSquash
	res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultExternalFailed {
		t.Fatalf("result = %q, want %q", res.Result, ResultExternalFailed)
	}
	if res.Disposition != MergeDispDenied {
		t.Fatalf("disposition = %q, want %q", res.Disposition, MergeDispDenied)
	}
	if res.Reason != "merge-denied" {
		t.Fatalf("reason = %q, want %q", res.Reason, "merge-denied")
	}
	if res.Method != "squash" {
		t.Fatalf("Method = %q, want %q", res.Method, "squash")
	}
}

// TestFinalizeMergeExplicitIDOverrides proves an explicit id satisfies the
// finalize-blocked skip but never overrides wrong PR identity, an unsafe stack,
// or the repair sign-off (gate), and never a superseding version.
func TestIntegrationFinalizeMergeExplicitIDOverrides(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("explicit-id-merges-past-finalize-blocked", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\nBlocked pending a decision.")
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied || res.Merge == nil {
			t.Fatalf("explicit id did not merge past the finalize-blocked marker: %q (reason %q)", res.Result, res.Reason)
		}
		if gh.mergeCalls != 1 {
			t.Fatalf("merge calls = %d, want 1", gh.mergeCalls)
		}
	})

	// Without an explicit id, the same finalize-blocked marker refuses.
	t.Run("auto-refuses-finalize-blocked", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\nBlocked pending a decision.")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, false, false))
		assertMergeRefusal(t, res, gh, "superseded")
	})

	t.Run("explicit-id-does-not-override-pr-identity", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Number = 9
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "pr-identity-mismatch")
	})

	t.Run("explicit-id-does-not-override-gate", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Body = ""
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "gate-unsatisfied")
	})

	t.Run("explicit-id-does-not-override-superseding-version", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		req := mergeReq(f, f.head, true, false)
		req.Version = "sha256:" + strings.Repeat("f", 64)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, req)
		assertMergeRefusal(t, res, gh, "superseded")
	})
}

// TestFinalizeMergeMethodUnavailableBlocks proves a cleanly-observed empty
// effective method set maps to a BLOCK before any effect: result blocked,
// disposition blocked, reason merge-method-unavailable (the literal token, not a
// constant equaling itself), no VerifiedMerge, no attempted method, and a
// message naming both the repository-enabled and the branch-permitted sets so a
// human can reconcile them. It is neither merge-denied (nothing was attempted)
// nor an unknown disposition (the incompatible policy WAS observed).
func TestIntegrationFinalizeMergeMethodUnavailableBlocks(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupMergeFixture(t, m)
	gh := f.baselineFake(t)
	gh.mergeOutcome = githubcli.MergeMethodUnavailable
	gh.mergeRepoMethods = []githubcli.MergeMethod{"squash"}
	gh.mergeBranchMethods = []githubcli.MergeMethod{"rebase", "merge"}
	res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultBlocked {
		t.Fatalf("result = %q, want %q", res.Result, ResultBlocked)
	}
	if res.Disposition != MergeDispBlocked {
		t.Fatalf("disposition = %q, want %q", res.Disposition, MergeDispBlocked)
	}
	if res.Reason != "merge-method-unavailable" {
		t.Fatalf("reason = %q, want literal %q", res.Reason, "merge-method-unavailable")
	}
	if res.Reason == "merge-denied" {
		t.Fatalf("reason is merge-denied; a pre-effect block was mislabeled as an attempted denial")
	}
	if res.Disposition == "unknown" {
		t.Fatalf("disposition is unknown; the incompatible policy was observed cleanly, not unobservably")
	}
	if res.Merge != nil {
		t.Fatalf("a method-unavailable block carried a VerifiedMerge")
	}
	if res.Method != "" {
		t.Fatalf("a pre-effect block carried an attempted method %q; none was issued", res.Method)
	}
	if !strings.Contains(res.Message, "squash") || !strings.Contains(res.Message, "rebase") {
		t.Fatalf("message must name both observed sets, got %q", res.Message)
	}
}

// TestFinalizeMergeReportsAttemptedMethod proves a successful merge surfaces the
// method Docket chose on the protocol document.
func TestIntegrationFinalizeMergeReportsAttemptedMethod(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupMergeFixture(t, m)
	mergeCommit := f.mergeFeatureIntoBase(t)
	gh := f.baselineFake(t)
	gh.mergeOutcome = githubcli.MergeMerged
	gh.mergeMethod = githubcli.MethodRebase
	gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
	res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultApplied || res.Merge == nil {
		t.Fatalf("verified merge = %q merge %v (reason %q)", res.Result, res.Merge, res.Reason)
	}
	if res.Method != "rebase" {
		t.Fatalf("Method = %q, want %q", res.Method, "rebase")
	}
}

// TestFinalizeMergeVerification proves the authoritative post-merge verification:
// a reachable merge commit yields a verified merge; a divergent head/base is
// contended; an unobservable reprobe is unknown; none but the reachable proof
// permits closeout.
func TestIntegrationFinalizeMergeVerification(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("reachable-merge-commit-verifies", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied || res.Disposition != MergeDispMerged || res.Merge == nil {
			t.Fatalf("verified merge = %q disp %q merge %v (reason %q)", res.Result, res.Disposition, res.Merge, res.Reason)
		}
		vm := res.Merge
		if vm.PRNumber != mergeCanonicalPRNumber || vm.HeadOID != f.head || vm.BaseRef != "main" || vm.MergeCommit != mergeCommit {
			t.Fatalf("VerifiedMerge facts = %+v, want number 7 head %s base main mergeCommit %s", vm, f.head, mergeCommit)
		}
		if vm.MergedAtUTC == "" {
			t.Fatalf("VerifiedMerge carried no mergedAt")
		}
	})

	t.Run("head-divergence-is-contended", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(strings.Repeat("b", 40), "main", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("head divergence = %q merge %v, want contended and no VerifiedMerge", res.Result, res.Merge)
		}
	})

	t.Run("base-divergence-is-contended", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "develop", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("base divergence = %q merge %v, want contended and no VerifiedMerge", res.Result, res.Merge)
		}
	})

	t.Run("unobservable-reprobe-is-unknown", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeUnknown
		gh.mergeErr = errors.New("gh merge transport boom")
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Disposition != MergeDispUnknown || res.Merge != nil {
			t.Fatalf("unobservable merge = disp %q merge %v (result %q), want unknown and no VerifiedMerge", res.Disposition, res.Merge, res.Result)
		}
	})

	t.Run("present-but-unreachable-merge-commit-is-contended", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		// The feature head is a real object in the shared store but is NOT merged
		// into the destination, so it is present yet not reachable from the base
		// tip — a clean unreachable answer, distinct from an absent object.
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", f.head)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("unreachable merge commit = %q merge %v (reason %q), want contended and no VerifiedMerge", res.Result, res.Merge, res.Reason)
		}
		if res.Reason != ReasonMergeUnreachable {
			t.Fatalf("reason = %q, want %q", res.Reason, ReasonMergeUnreachable)
		}
	})

	t.Run("reported-merge-commit-absent-is-unknown", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		// A well-formed object id that is not in the destination's object graph:
		// the reachability probe cannot observe it, so the result is unknown.
		gh.mergeFacts = mergedFactsFor(f.head, "main", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Merge != nil {
			t.Fatalf("an unverifiable merge commit produced a VerifiedMerge")
		}
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("an unverifiable merge commit reported success %q", res.Result)
		}
	})
}

// --- carried-descendant preservation gate (Task 8) ------------------------

// fakeMergeCarryGitHub serves the GitHub reads a carry-gated merge makes: the
// parent's open PR and open-child probe (FindOpenPullRequestsByHead), each
// stack descendant's merged reprobe AND the parent's already-merged short
// circuit (ProbeMerged, keyed by number), and the parent MergePullRequest whose
// call count is the witness a refused merge issued ZERO external merges. Every
// other finalize-half method panics so an accidental call is loud. It keys
// ProbeMerged by number (unlike fakeMergeGitHub's single scripted value) so the
// parent #7 can stay unmerged while a stacked child #8 reprobes already-merged.
type fakeMergeCarryGitHub struct {
	repo       githubcli.Repository
	openByHead map[string][]githubcli.PullRequest
	merged     map[int]closeoutProbe

	mergeOutcome githubcli.MergeOutcome
	mergeMethod  githubcli.MergeMethod
	mergeFacts   githubcli.MergedFacts
	mergeCalls   int
	probes       int
}

func (f *fakeMergeCarryGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}
func (f *fakeMergeCarryGitHub) ProbeMerged(_ context.Context, _ githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	f.probes++
	p, ok := f.merged[number]
	if !ok {
		return githubcli.MergeNotMergeable, githubcli.MergedFacts{}, nil
	}
	return p.outcome, p.facts, nil
}
func (f *fakeMergeCarryGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, head string) ([]githubcli.PullRequest, error) {
	return f.openByHead[head], nil
}
func (f *fakeMergeCarryGitHub) MergePullRequest(_ context.Context, _ githubcli.Repository, _ int, _ githubcli.ObjectRef, _ bool) (githubcli.MergeResult, error) {
	f.mergeCalls++
	return githubcli.MergeResult{Outcome: f.mergeOutcome, Method: f.mergeMethod, Facts: f.mergeFacts}, nil
}
func (f *fakeMergeCarryGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: carry-gated merge must not call this")
}
func (f *fakeMergeCarryGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: carry-gated merge must not call this")
}
func (f *fakeMergeCarryGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: carry-gated merge must not call this")
}
func (f *fakeMergeCarryGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: carry-gated merge must not call this")
}

// carryFake builds a carry-gated fake whose parent PR (#7) carries prBody (green
// evidence, or "" when the gate is off) and passes every non-carry conjunct;
// tests seed the child #8 reprobe and the merge outcome onto it.
func (f *mergeFixture) carryFake(prBody string) *fakeMergeCarryGitHub {
	return &fakeMergeCarryGitHub{
		repo:       retargetRepo(),
		openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {f.parentPR(f.head, prBody)}},
		merged:     map[int]closeoutProbe{},
	}
}

// TestIntegrationFinalizeMergeCarryPreservation proves FinalizeMerge proves every
// carried descendant's merged work is preserved at the verified PR head BEFORE
// the external merge, and that the new gate COMPLEMENTS the existing conjuncts
// (it never runs before the head/lease conjunct, and never displaces the
// already-merged short circuit closeout owns the post-merge proof for).
func TestIntegrationFinalizeMergeCarryPreservation(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	ctx := context.Background()

	t.Run("all-conjuncts-hold-but-carried-child-lost-refuses-zero-merge-calls", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		seedRebaseCarryChild(t, f.rebaseFixture)
		dropped := carryDroppedCommit(t, f.rebaseFixture, map[string]string{"catalog.yaml": "Y\n"})
		// The child merge object EXISTS (a drop, not a missing object): pins the
		// refusal is an observed non-preservation, never an observation error.
		if _, err := tryGit(f.repo.invocation, "cat-file", "-e", dropped); err != nil {
			t.Fatalf("the child merge object must exist to pin a drop: %v", err)
		}
		gh := f.carryFake(greenEvidenceFor(t, f.head))
		gh.merged[8] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}
		// A real merge outcome would otherwise land: the witness is that the gate
		// refuses BEFORE any MergePullRequest, leaving zero merge calls.
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", dropped)

		res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Reason != ReasonCarryUnproven || res.Result != ResultBlocked || res.Disposition != MergeDispBlocked {
			t.Fatalf("carry-lost merge = result %q disp %q reason %q, want blocked/blocked/%s", res.Result, res.Disposition, res.Reason, ReasonCarryUnproven)
		}
		if res.Merge != nil {
			t.Fatalf("a carry refusal carried a VerifiedMerge")
		}
		if len(res.Findings) == 0 {
			t.Fatalf("a carry refusal carried no findings naming the unproven descendant")
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("a carry refusal issued %d merge call(s); want 0 (witness: no external merge)", gh.mergeCalls)
		}
	})

	t.Run("ordinary-change-merges-witness-fires-on-happy-path", func(t *testing.T) {
		// No stack descendants: the proof is vacuously satisfied with zero probes and
		// the merge lands, so the MergePullRequest witness fires exactly once — the
		// companion that proves the zero-call assertion above is not vacuous.
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.carryFake(greenEvidenceFor(t, f.head))
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied || res.Disposition != MergeDispMerged || res.Merge == nil {
			t.Fatalf("ordinary merge = %q disp %q merge %v (reason %q), want applied/merged verified", res.Result, res.Disposition, res.Merge, res.Reason)
		}
		if gh.mergeCalls != 1 {
			t.Fatalf("the happy-path witness issued %d merge call(s); want exactly 1", gh.mergeCalls)
		}
	})

	t.Run("gate-off-enforces-the-same-proof", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		seedRebaseCarryChild(t, f.rebaseFixture)
		dropped := carryDroppedCommit(t, f.rebaseFixture, map[string]string{"catalog.yaml": "G\n"})
		// finalize.gate: off — the gate conjunct is satisfied by config, not by
		// green PR evidence, so the merge reaches the proof through the gate-off
		// route. The proof still runs and refuses: gate mode cannot disable it.
		f.repo.writerAdvance(t, "main", map[string]string{
			".docket.yml": "integration_branch: main\nbuild:\n  test_command: 'go test ./...'\nfinalize:\n  gate: \"off\"\n  test_command: 'go test ./...'\n",
		})
		gh := f.carryFake("") // NO green evidence: only gate-off lets the gate conjunct pass
		gh.merged[8] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}
		gh.mergeOutcome = githubcli.MergeMerged
		res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Reason != ReasonCarryUnproven {
			t.Fatalf("gate-off carry-lost merge = reason %q (result %q msg %q), want %s (gate mode cannot disable the proof)", res.Reason, res.Result, res.Message, ReasonCarryUnproven)
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("gate-off carry refusal issued %d merge call(s); want 0", gh.mergeCalls)
		}
	})

	t.Run("direct-merge-explicit-id-enforces-the-proof", func(t *testing.T) {
		// An explicit-id request reaches FinalizeMerge directly, with no prior rebase
		// receipt: the proof still gates the external merge.
		f := setupMergeFixture(t, m)
		seedRebaseCarryChild(t, f.rebaseFixture)
		dropped := carryDroppedCommit(t, f.rebaseFixture, map[string]string{"catalog.yaml": "D\n"})
		gh := f.carryFake(greenEvidenceFor(t, f.head))
		gh.merged[8] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}
		gh.mergeOutcome = githubcli.MergeMerged
		res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Reason != ReasonCarryUnproven || res.Result != ResultBlocked {
			t.Fatalf("direct explicit-id carry-lost merge = result %q reason %q, want blocked/%s", res.Result, res.Reason, ReasonCarryUnproven)
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("direct-merge carry refusal issued %d merge call(s); want 0", gh.mergeCalls)
		}
	})

	t.Run("concurrent-head-move-fails-the-existing-conjunct-first", func(t *testing.T) {
		// The PR head no longer matches the requested head: the EXISTING
		// exact-head/lease conjunct rejects the movement BEFORE the carry proof, so
		// the reason is the existing head-moved token, not the carry reason — proving
		// the new gate complements, not replaces, the head/lease authorization.
		f := setupMergeFixture(t, m)
		seedRebaseCarryChild(t, f.rebaseFixture)
		dropped := carryDroppedCommit(t, f.rebaseFixture, map[string]string{"catalog.yaml": "H\n"})
		gh := f.carryFake(greenEvidenceFor(t, f.head))
		gh.merged[8] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}
		gh.mergeOutcome = githubcli.MergeMerged
		other := strings.Repeat("b", 40)
		res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, other, true, false))
		if res.Reason != "head-moved" {
			t.Fatalf("concurrent head move = reason %q, want the existing head-moved conjunct (carry gate must not preempt it)", res.Reason)
		}
		if res.Reason == ReasonCarryUnproven {
			t.Fatalf("the carry gate preempted the existing head/lease conjunct")
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("a head-moved refusal issued %d merge call(s); want 0", gh.mergeCalls)
		}
	})

	t.Run("already-merged-short-circuit-unchanged-no-new-proof", func(t *testing.T) {
		// An already-merged exact PR is a verified no-op probed BEFORE the conjunct
		// recheck and the carry proof; closeout owns the post-merge proof, so a lost
		// carried child does not turn the short circuit into a carry refusal.
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		seedRebaseCarryChild(t, f.rebaseFixture)
		dropped := carryDroppedCommit(t, f.rebaseFixture, map[string]string{"catalog.yaml": "A\n"})
		gh := f.carryFake(greenEvidenceFor(t, f.head))
		gh.merged[7] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)}
		gh.merged[8] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/widget", dropped)}
		res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultNoOp || res.Disposition != MergeDispAlreadyMerged || res.Merge == nil {
			t.Fatalf("already-merged short circuit = %q disp %q merge %v (reason %q), want no-op/already-merged verified", res.Result, res.Disposition, res.Merge, res.Reason)
		}
		if res.Reason == ReasonCarryUnproven {
			t.Fatalf("the already-merged short circuit ran the carry proof; closeout owns the post-merge proof")
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("an already-merged PR issued %d merge call(s); want 0", gh.mergeCalls)
		}
	})
}
