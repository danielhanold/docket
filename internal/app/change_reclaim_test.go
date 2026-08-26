package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
)

// This file drives `change reclaim`: the proof-gated return of a strictly-expired
// in-progress claim to proposed. The lease gate and the applied mutation run over
// the in-memory fakeTree Plan harness (blockPlanFor); the destructive branch and
// workspace gates, the atomic transaction, and the exact-version contention run
// end-to-end over real bare-remote repositories in both metadata modes with a
// fake WorkspaceService scripting the inspected state.

// --- fake WorkspaceService for the reclaim ownership/gate probe ------------

// fakeReclaimWorkspace scripts the inspected workspace state (or a probe error)
// the reclaim reads to decide whether the change's work still exists. Prepare and
// PublishHead panic — reclaim only inspects.
type fakeReclaimWorkspace struct {
	kind workspace.StateKind
	err  error
}

func (f fakeReclaimWorkspace) Prepare(context.Context, workspace.PrepareRequest) (workspace.Workspace, error) {
	panic("Prepare: reclaim must not allocate")
}
func (f fakeReclaimWorkspace) Inspect(context.Context, workspace.InspectRequest) (workspace.Inspection, error) {
	if f.err != nil {
		return workspace.Inspection{}, f.err
	}
	return workspace.Inspection{Kind: f.kind}, nil
}
func (f fakeReclaimWorkspace) PublishHead(context.Context, workspace.PublishRequest) (workspace.PublishResult, error) {
	panic("PublishHead: reclaim must not publish")
}

// reclaimClearWorkspace is the quiescent inspection (no owned live workspace) the
// clear-path fixtures inject so the workspace conjunct passes.
var reclaimClearWorkspace = WorkspaceDeps{Service: fakeReclaimWorkspace{kind: workspace.StateForeign}}

// reclaimOpFixture builds the reclaim Plan operation with proven-absent branch
// facts, matching what ChangeReclaim assembles once its destructive gate holds.
func reclaimOpFixture(id int, surfaces []string) reclaimOp {
	return reclaimOp{
		id:           id,
		ttlHours:     72,
		facts:        domain.NewBranchFacts(nil),
		proofSummary: reclaimProofSummary(72),
		eff:          planningTestConfig(surfaces),
		clock:        testClock(),
		inline:       len(surfaces) > 0 && surfaces[0] == "inline",
		link:         render.LinkContext{MetadataBranch: "main"},
		changesDir:   "docs/changes",
	}
}

// reclaimRecordWithClaim renders an in-progress record whose claim stamp is set
// to claim, so a lease-state row can be exercised deterministically.
func reclaimRecordWithClaim(id int, slug, claim string) string {
	return strings.Replace(lifecycleChange(id, slug, "in-progress"),
		"claimed_at: 2026-08-02T00:00:00Z", "claimed_at: "+claim, 1)
}

// --- lease gate (plan closure) ---------------------------------------------

// TestReclaimRequiresStrictExpiry proves the lease gate: only a strictly-expired
// in-progress lease applies; a future or exactly-at-boundary stamp is a retained
// refusal, and a non-in-progress source is an illegal-source-status refusal (the
// result mapper folds it onto contended). The missing/empty/malformed rows are
// corpus-invalid states the engine's load-before validation refuses end-to-end
// (TestReclaimMalformedLeaseSkips) and the domain's own EvaluateLease coverage
// pins; they cannot be represented as a clean corpus here.
func TestReclaimRequiresStrictExpiry(t *testing.T) {
	recPath := groomPath(3, "widget")

	// Strictly expired (claimed 2026-08-02, evaluated 2026-08-16, TTL 72h): applies.
	t.Run("expired-applies", func(t *testing.T) {
		files := map[string]string{
			recPath:                 lifecycleChange(3, "widget", "in-progress"),
			"docs/changes/BOARD.md": "# Backlog\n\nold\n",
		}
		plan, opRes := blockPlanFor(t, files, planningTestConfig([]string{"inline"}), reclaimOpFixture(3, []string{"inline"}))
		if opRes.Refused {
			t.Fatalf("strict expiry refused: %v", opRes.Findings)
		}
		rec := lifecycleRecordBytes(t, plan, recPath)
		for _, want := range []string{"status: 'proposed'", "reconciled: false", "## Reclaim log", "2026-08-02T00:00:00Z", "lease strictly expired"} {
			if !strings.Contains(rec, want) {
				t.Errorf("reclaimed record missing %q:\n%s", want, rec)
			}
		}
		if strings.Contains(rec, "branch: feat/widget") {
			t.Errorf("branch not cleared:\n%s", rec)
		}
		if strings.Contains(rec, "claimed_at: 2026-08-02") {
			t.Errorf("claim stamp not cleared:\n%s", rec)
		}
	})

	// Future and exactly-at-boundary stamps are not expiry: retained refusal.
	for _, tc := range []struct{ name, claim string }{
		{"future", "2026-08-20T00:00:00Z"},
		{"exactly-at-boundary", "2026-08-13T12:00:00Z"}, // 72h before the clock
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{recPath: reclaimRecordWithClaim(3, "widget", tc.claim)}
			_, opRes := blockPlanFor(t, files, planningTestConfig(nil), reclaimOpFixture(3, nil))
			if !opRes.Refused || firstFindingCode(opRes.Findings) != "lease-not-expired" {
				t.Fatalf("%s: refused=%v code=%q, want lease-not-expired", tc.name, opRes.Refused, firstFindingCode(opRes.Findings))
			}
		})
	}

	// A non-in-progress source has no lease to expire: illegal-source-status.
	t.Run("non-in-progress", func(t *testing.T) {
		files := map[string]string{recPath: lifecycleChange(3, "widget", "proposed")}
		_, opRes := blockPlanFor(t, files, planningTestConfig(nil), reclaimOpFixture(3, nil))
		if !opRes.Refused || firstFindingCode(opRes.Findings) != reclaimReasonIllegalSource {
			t.Fatalf("non-in-progress: refused=%v code=%q, want %q", opRes.Refused, firstFindingCode(opRes.Findings), reclaimReasonIllegalSource)
		}
	})
}

// --- destructive gate: proven absence (real git) ---------------------------

// TestReclaimRequiresProvenAbsence proves the destructive gate refuses and
// mutates NOTHING when a recorded/conventional branch is still present (locally
// or remotely), when an owned workspace is still live, or when any probe cannot
// be answered — unknown never shares the absent branch. Every refusal leaves the
// origin record byte-identical.
func TestReclaimRequiresProvenAbsence(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			// branch present locally.
			t.Run("local-branch-present", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				runGit(t, repo.invocation, "branch", "feat/widget")
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimBranchPresent)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})

			// the fresh-mint candidate is <type>/<slug>, not the fixed feat/
			// prefix: a type-fix change probes fix/<slug> alongside its recorded
			// name. Here the recorded feat/widget is absent, so the block can only
			// come from the minted fix/widget candidate.
			t.Run("mint-candidate-probes-type", func(t *testing.T) {
				rec := strings.Replace(lifecycleChange(3, "widget", "in-progress"), "type: feat", "type: fix", 1)
				repo := m.build(t, map[string]string{recPath: rec})
				node := planningDepsFor(t, repo.invocation)
				runGit(t, repo.invocation, "branch", "fix/widget")
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimBranchPresent)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})

			// branch present remotely (local absent).
			t.Run("remote-branch-present", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				runGit(t, repo.invocation, "push", "-q", "origin", "HEAD:refs/heads/feat/widget")
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimBranchPresent)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})

			// an owned workspace still holds live work.
			t.Run("workspace-active", func(t *testing.T) {
				for _, kind := range []workspace.StateKind{workspace.StateReady, workspace.StateDirty, workspace.StateResumable, workspace.StateMismatch} {
					repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
					node := planningDepsFor(t, repo.invocation)
					ver := blobVersionAt(t, repo.origin, m.branch, recPath)
					before, _ := originFile(t, repo.origin, m.branch, recPath)
					res := ChangeReclaim(context.Background(), node.deps,
						WorkspaceDeps{Service: fakeReclaimWorkspace{kind: kind}}, node.dir,
						ChangeReclaimRequest{ID: 3, Version: ver})
					assertReclaimSkipped(t, res, ReasonReclaimWorkspaceActive)
					assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
				}
			})

			// a workspace probe that cannot be answered fails closed.
			t.Run("workspace-probe-error", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps,
					WorkspaceDeps{Service: fakeReclaimWorkspace{err: errors.New("inspect boom")}}, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				assertReclaimSkipped(t, res, ReasonReclaimWorkspaceProbe)
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})
		})
	}
}

// TestReclaimUnreachableRemoteFailsClosed proves the destructive leg fails closed
// when the remote is unreachable: nothing is reclaimed and the origin record is
// untouched. An unreachable origin is caught while pinning authoritative context
// (which fetches origin before any probe), so a reclaim can never proceed on
// state it could not authoritatively read — unknown never shares the clean-read
// branch on a destructive operation.
func TestReclaimUnreachableRemoteFailsClosed(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newMainModeRepo(t, map[string]string{
		".docket.yml": "metadata_branch: main\n",
		recPath:       lifecycleChange(3, "widget", "in-progress"),
	})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "main", recPath)
	before, _ := originFile(t, repo.origin, "main", recPath)
	// Break the remote so no origin state can be authoritatively read or probed.
	runGit(t, repo.invocation, "remote", "set-url", "origin", t.TempDir()+"/nonexistent.git")
	res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
		ChangeReclaimRequest{ID: 3, Version: ver})
	if res.Result == ResultApplied {
		t.Fatalf("an unreachable remote was reclaimed: %q", res.Result)
	}
	if res.Disposition != ReclaimDispSkipped {
		t.Errorf("disposition = %q, want skipped", res.Disposition)
	}
	assertOriginRecordUnchanged(t, repo.origin, "main", recPath, before)
}

// TestReclaimMalformedLeaseSkips proves a record whose claim stamp is malformed
// (an unevaluable lease, hence a corpus error) is refused end-to-end with no
// mutation — the destructive leg fails closed on an unreadable lease.
func TestReclaimMalformedLeaseSkips(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newMainModeRepo(t, map[string]string{
		".docket.yml": "metadata_branch: main\n",
		recPath:       reclaimRecordWithClaim(3, "widget", "not-a-timestamp"),
	})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "main", recPath)
	before, _ := originFile(t, repo.origin, "main", recPath)
	res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
		ChangeReclaimRequest{ID: 3, Version: ver})
	if res.Result == ResultApplied {
		t.Fatalf("a malformed lease was reclaimed: %q", res.Result)
	}
	if res.Disposition != ReclaimDispSkipped {
		t.Errorf("disposition = %q, want skipped", res.Disposition)
	}
	assertOriginRecordUnchanged(t, repo.origin, "main", recPath, before)
}

// --- transaction (real git, both modes) ------------------------------------

// TestReclaimTransaction proves the applied path end-to-end: the landed Reclaim
// action returns the record to proposed, clears branch/claim, sets
// reconciled:false, appends one dated ## Reclaim log entry, and rerenders the
// board — all in one atomic commit; and that an exact-version contention refuses.
func TestReclaimTransaction(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")

	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			t.Run("reclaims", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				ver := blobVersionAt(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: ver})
				if res.Result != ResultApplied || res.Disposition != ReclaimDispReclaimed {
					t.Fatalf("reclaim = (%q, %q) reason=%q findings=%v", res.Result, res.Disposition, res.Reason, res.Findings)
				}
				if res.Revision == "" {
					t.Fatalf("applied reclaim carried no committed revision")
				}
				rec, ok := originFile(t, repo.origin, m.branch, recPath)
				if !ok {
					t.Fatalf("record vanished after reclaim")
				}
				for _, want := range []string{"status: 'proposed'", "reconciled: false", "## Reclaim log", "2026-08-02T00:00:00Z", "lease strictly expired"} {
					if !strings.Contains(rec, want) {
						t.Errorf("reclaimed origin record missing %q:\n%s", want, rec)
					}
				}
				if strings.Contains(rec, "branch: feat/widget") {
					t.Errorf("branch not cleared on origin:\n%s", rec)
				}
				if strings.Contains(rec, "claimed_at: 2026-08-02") {
					t.Errorf("claim stamp not cleared on origin:\n%s", rec)
				}
				// The board is refreshed to a fresh render of the committed corpus.
				assertBoardMatchesCommitted(t, repo.origin, m.branch, repo.invocation)
			})

			t.Run("version-drift-contends", func(t *testing.T) {
				repo := m.build(t, map[string]string{recPath: lifecycleChange(3, "widget", "in-progress")})
				node := planningDepsFor(t, repo.invocation)
				before, _ := originFile(t, repo.origin, m.branch, recPath)
				res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
					ChangeReclaimRequest{ID: 3, Version: strings.Repeat("b", 40)})
				if res.Result != ResultContended {
					t.Fatalf("version drift = %q, want contended", res.Result)
				}
				assertOriginRecordUnchanged(t, repo.origin, m.branch, recPath, before)
			})
		})
	}
}

// TestReclaimIndependentOfAutoPolicy proves explicit reclaim applies even when
// reclaim.auto is false — the auto policy governs only maintenance sweep.
func TestReclaimIndependentOfAutoPolicy(t *testing.T) {
	requireRealGit(t)
	recPath := groomPath(3, "widget")
	repo := newMainModeRepo(t, map[string]string{
		".docket.yml": "metadata_branch: main\nreclaim:\n  auto: false\n",
		recPath:       lifecycleChange(3, "widget", "in-progress"),
	})
	node := planningDepsFor(t, repo.invocation)
	ver := blobVersionAt(t, repo.origin, "main", recPath)
	res := ChangeReclaim(context.Background(), node.deps, reclaimClearWorkspace, node.dir,
		ChangeReclaimRequest{ID: 3, Version: ver})
	if res.Result != ResultApplied || res.Disposition != ReclaimDispReclaimed {
		t.Fatalf("explicit reclaim under reclaim.auto:false = (%q, %q) findings=%v", res.Result, res.Disposition, res.Findings)
	}
}

// --- helpers ---------------------------------------------------------------

func assertReclaimSkipped(t *testing.T, res ChangeReclaimResult, wantReason string) {
	t.Helper()
	if res.Result == ResultApplied {
		t.Fatalf("expected a skip, got applied")
	}
	if res.Disposition != ReclaimDispSkipped {
		t.Errorf("disposition = %q, want skipped", res.Disposition)
	}
	if res.Reason != wantReason {
		t.Errorf("reason = %q, want %q", res.Reason, wantReason)
	}
}

func assertOriginRecordUnchanged(t *testing.T, origin, branch, path, before string) {
	t.Helper()
	after, ok := originFile(t, origin, branch, path)
	if !ok {
		t.Fatalf("record %q vanished on a refusal", path)
	}
	if after != before {
		t.Errorf("a refused reclaim mutated the origin record:\n--before--\n%s\n--after--\n%s", before, after)
	}
}

// TestReclaimResultFromOutcomeFailedCarriesCause proves a reclaim transaction
// that fails mid-flight is dispositioned `failed` (not mislabeled as a reasonless
// `skipped`) and carries its typed cause in the envelope's failure diagnosis.
func TestReclaimResultFromOutcomeFailedCarriesCause(t *testing.T) {
	execErr := &transaction.Failure{
		Stage:  transaction.StageVerifyDelta,
		Kind:   transaction.KindInvalidState,
		Detail: "an undeclared path changed in the worktree",
	}
	out := reclaimResultFromOutcome(
		transaction.Result{Disposition: transaction.DispositionFailed}, execErr)

	if out.Disposition != ReclaimDispFailed {
		t.Errorf("disposition = %q, want %q — a failed reclaim is not a reasonless skip", out.Disposition, ReclaimDispFailed)
	}
	if out.Failure == nil || out.Failure.Detail == "" {
		t.Fatalf("failure diagnosis missing or empty: %+v", out.Failure)
	}
}
