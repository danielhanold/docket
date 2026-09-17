//go:build integration

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This file is the real-Git end-to-end coverage for the generated-bundle fast
// path (change 0413): a conflict stop whose only unmerged paths are Docket's own
// embedded asset bundle is cleared in-process by regenerating the bundle, with
// zero resolver reservations spent. The fixture shapes a real feature workspace
// into an ELIGIBLE Docket repo (the exact module identity, all DefaultAllowedRoots,
// and a committed bundle on a shared ancestor), then diverges the feature and the
// base so every replayed commit collides on the generated bundle alone.

// readRepoFile reads a repo-relative file's content, the read-side companion of
// writeRepoFile. It is defined here (not beside writeRepoFile) because only the
// generated-bundle integration tests need it and the commit is scoped to this
// file.
func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// regenerateBundleInto runs the REAL generator over the workspace and writes the
// bundle in place, exactly as go generate ./internal/assets/ would. It first
// ensures the internal/assets parent exists: production regenerateEmbeddedBundle
// stages beside the destination (internal/assets), which always exists in the
// real source tree but not in a fresh fixture before the first regeneration.
func regenerateBundleInto(t *testing.T, wsDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(wsDir, "internal", "assets"), 0o755); err != nil {
		t.Fatalf("prepare assets dir: %v", err)
	}
	if err := regenerateEmbeddedBundle(wsDir); err != nil {
		t.Fatalf("fixture bundle regeneration: %v", err)
	}
}

// makeBundleWorkspace turns a rebase fixture's feature workspace into an ELIGIBLE
// Docket-shaped repo: go.mod with the Docket module identity, all four
// DefaultAllowedRoots, and a committed generated bundle. It then publishes that
// docket-shaped commit onto the base branch too, so the base and the feature
// share an ancestor that already carries go.mod + the roots + the bundle — the
// invariant that makes every later divergence collide on the bundle ALONE.
// module lets the ineligible-repo test plant a foreign identity.
func makeBundleWorkspace(t *testing.T, f *rebaseFixture, module string) {
	t.Helper()
	writeRepoFile(t, f.wp, "go.mod", "module "+module+"\n\ngo 1.22\n")
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", "demo skill v1\n")
	writeRepoFile(t, f.wp, "agents/demo.md", "demo agent\n")
	writeRepoFile(t, f.wp, "cursor-rules/demo.mdc", "demo rule\n")
	writeRepoFile(t, f.wp, ".docket.example.yml", "version: 1\n")
	regenerateBundleInto(t, f.wp)
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", "docket-shaped workspace with committed bundle")
	// Establish the docket-shaped commit as the shared ancestor on the base
	// branch: both sides then regenerate over the same roots and collide only on
	// the bundle. This is a fast-forward — the commit descends the base tip.
	runGit(t, f.wp, "push", "-f", "-q", "origin", "HEAD:main")
}

// bundleFeatureCommit edits one authored skill file and regenerates the bundle,
// committing both — the shape that produces a manifest collision on every
// replayed commit once the base regenerates too.
func bundleFeatureCommit(t *testing.T, f *rebaseFixture, step int) {
	t.Helper()
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", fmt.Sprintf("demo skill feature v%d\n", step))
	regenerateBundleInto(t, f.wp)
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", fmt.Sprintf("feature bundle step %d", step))
}

// advanceBaseWithBundle lands ONE base commit that makes an authored edit to a
// DIFFERENT authored file than the feature commits touch (agents/demo.md) and
// regenerates the bundle from the base's authored roots. It mirrors
// writerAdvance's mechanics (a fresh checkout of the base branch, the edit, a
// commit, a push) but regenerates with the real generator over a base branch
// that already carries the docket-shaped roots (makeBundleWorkspace seeded them).
// The independent authored edit is what the "preserved authored edits" assertion
// checks after the rebase.
func advanceBaseWithBundle(t *testing.T, f *rebaseFixture) {
	t.Helper()
	root := testsupport.TempDir(t)
	clone := filepath.Join(root, "base-advance")
	runGit(t, root, "clone", "-q", f.repo.origin, clone)
	gitIdentity(t, clone)
	runGit(t, clone, "checkout", "-q", "main")
	writeRepoFile(t, clone, "agents/demo.md", "demo agent, base edition\n")
	regenerateBundleInto(t, clone)
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "commit", "-q", "-m", "base bundle edition")
	runGit(t, clone, "push", "-q", "origin", "main")
}

// prepareBundleConflictsWithoutBegin builds an eligible (or, with a foreign
// module, ineligible) bundle-collision fixture WITHOUT beginning the rebase: it
// shapes the workspace, publishes featureCommits bundle-regenerating feature
// commits, advances the base with an independent authored+bundle commit, sets the
// resolver limit, and returns the fixture, deps, and the authorized head.
func prepareBundleConflictsWithoutBegin(t *testing.T, limit, featureCommits int, module string) (*rebaseFixture, FinalizeDeps, string) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	makeBundleWorkspace(t, f, module)
	for i := 1; i <= featureCommits; i++ {
		bundleFeatureCommit(t, f, i)
	}
	head := runGit(t, f.wp, "rev-parse", "HEAD")
	runGit(t, f.wp, "push", "-f", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
	advanceBaseWithBundle(t, f)
	if limit > 0 {
		setResolverConfig(t, f, limit)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	return f, deps, head
}

// beginBundleConflicts is prepareBundleConflictsWithoutBegin plus the initial
// FinalizeRebase call: the rebase begins and stops (or, on the fast path,
// completes straight through). It returns the fixture, deps, the authorized head,
// and the begin result.
func beginBundleConflicts(t *testing.T, limit, featureCommits int, module string) (*rebaseFixture, FinalizeDeps, string, FinalizeRebaseResult) {
	t.Helper()
	f, deps, head := prepareBundleConflictsWithoutBegin(t, limit, featureCommits, module)
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	return f, deps, head, begin
}

// TestIntegrationGeneratedOnlyStopsBypassResolverBudget is change 0413's core
// regression: more generated-only stops than the configured resolver limit
// complete WITHOUT any reservation, authored edits from both sides survive, and
// the committed bundle matches the authored roots exactly. Reverting the fast
// path must make this test fail (the second stop would exhaust limit 2).
func TestIntegrationGeneratedOnlyStopsBypassResolverBudget(t *testing.T) {
	requireRealGit(t)
	// limit 2, four feature commits -> four successive generated-only stops.
	f, deps, _, begin := beginBundleConflicts(t, 2, 4, docketModulePath)
	if begin.Result != ResultApplied || begin.Disposition != RebaseDispRebased {
		t.Fatalf("begin = (%q, %q) reason %q msg %q paths %v, want applied/rebased straight through",
			begin.Result, begin.Disposition, begin.Reason, begin.Message, begin.UnmergedPaths)
	}
	if begin.Gate == nil || begin.Gate.Evidence == "" {
		t.Errorf("the completed rebase did not compose the gate: %+v", begin.Gate)
	}
	// Zero reservations: the budget group is untouched.
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("fast path spent resolver state: used %q token %q cont %q, want 0 and empty", rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	// Both sides' authored edits survive at the rebased head.
	if got := readRepoFile(t, f.wp, "skills/demo/SKILL.md"); got != "demo skill feature v4\n" {
		t.Errorf("feature authored edit lost: %q", got)
	}
	if got := readRepoFile(t, f.wp, "agents/demo.md"); got != "demo agent, base edition\n" {
		t.Errorf("base authored edit lost: %q", got)
	}
	// Clean bundle drift check at the final head.
	m, payload, err := assets.Generate(f.wp, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("post-rebase generate: %v", err)
	}
	diffs, err := assets.DiffTree(filepath.Join(f.wp, "internal", "assets", "embedded"), m, payload)
	if err != nil || len(diffs) != 0 {
		t.Fatalf("post-rebase bundle drift: %v %v", diffs, err)
	}
	if st, _ := f.deps.Client.RebaseState(context.Background(), f.wp); st.Disposition != gitcli.RebaseUnchanged {
		t.Errorf("a rebase is still in progress: %q", st.Disposition)
	}
	_ = deps
}

// TestIntegrationGeneratedOnlyIneligibleRepoStaysOnNormalPath proves a repo that
// is NOT Docket (foreign module identity) never enters the fast path: the same
// bundle-shaped collision surfaces as an ordinary conflicted stop.
func TestIntegrationGeneratedOnlyIneligibleRepoStaysOnNormalPath(t *testing.T) {
	requireRealGit(t)
	_, _, _, begin := beginBundleConflicts(t, 2, 1, "github.com/someone/else")
	if begin.Disposition != RebaseDispConflicted || begin.Reason != ReasonRebaseConflicted {
		t.Fatalf("begin = disp %q reason %q, want a plain conflicted stop", begin.Disposition, begin.Reason)
	}
	if !pathsGeneratedOnly(begin.UnmergedPaths) {
		t.Fatalf("fixture defect: the stop was not bundle-only: %v", begin.UnmergedPaths)
	}
}

// TestIntegrationGeneratedOnlyGenerationFailureBlocks proves a failing
// regeneration blocks through the existing failure path, leaves the rebase
// stopped (abort available), and never spends resolver state.
func TestIntegrationGeneratedOnlyGenerationFailureBlocks(t *testing.T) {
	requireRealGit(t)
	f, deps, head := prepareBundleConflictsWithoutBegin(t, 2, 1, docketModulePath)
	deps.RegenerateBundle = func(string) error { return fmt.Errorf("synthetic generation failure") }
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if begin.Result != ResultBlocked || begin.Reason != ReasonRebaseGitFailed {
		t.Fatalf("begin = (%q, %q), want blocked/%q", begin.Result, begin.Reason, ReasonRebaseGitFailed)
	}
	if st, _ := f.deps.Client.RebaseState(context.Background(), f.wp); st.Disposition != gitcli.RebaseConflicted {
		t.Errorf("the failed fast path did not retain the stopped rebase: %q", st.Disposition)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" || rec.ResolverReservationToken != "" {
		t.Errorf("generation failure spent resolver state: used %q token %q", rec.ResolverUsed, rec.ResolverReservationToken)
	}
	// Abort restores the recorded original head.
	abort := FinalizeRebaseAbort(context.Background(), deps, f.repo.invocation, f.id, rec.Attempt,
		ResolverReport{ChangeID: f.id, Attempt: rec.Attempt, Disposition: ResolverStuck})
	if abort.Result != ResultApplied {
		t.Fatalf("abort after generation failure = %q reason %q", abort.Result, abort.Reason)
	}
}

// advanceBaseWithMixedConflict lands ONE base commit that conflictingly edits the
// SAME authored file a feature commit touches (skills/demo/SKILL.md) and
// regenerates the bundle from the base's authored roots. Replaying a feature
// commit onto it produces a MIXED stop: the authored file AND the bundle outputs
// are both unmerged. It mirrors advanceBaseWithBundle's mechanics (a fresh
// checkout of the base branch, the edit, a commit, a push) but collides on the
// authored skill instead of leaving an independent edit.
func advanceBaseWithMixedConflict(t *testing.T, f *rebaseFixture) {
	t.Helper()
	root := testsupport.TempDir(t)
	clone := filepath.Join(root, "base-advance-mixed")
	runGit(t, root, "clone", "-q", f.repo.origin, clone)
	gitIdentity(t, clone)
	runGit(t, clone, "checkout", "-q", "main")
	writeRepoFile(t, clone, "skills/demo/SKILL.md", "conflicting base skill\n")
	regenerateBundleInto(t, clone)
	runGit(t, clone, "add", "-A")
	runGit(t, clone, "commit", "-q", "-m", "base conflicting skill edition")
	runGit(t, clone, "push", "-q", "origin", "main")
}

// beginMixedBundleConflict builds an eligible bundle fixture whose single base
// commit conflictingly edits the SAME authored skill file the single feature
// commit touches, so the begin stops on a MIXED unmerged set — the authored path
// (skills/demo/SKILL.md) plus the bundle outputs. It sets the resolver limit and
// begins the rebase, returning the fixture, deps, the authorized head, and the
// begin result.
func beginMixedBundleConflict(t *testing.T, limit int) (*rebaseFixture, FinalizeDeps, string, FinalizeRebaseResult) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	makeBundleWorkspace(t, f, docketModulePath)
	bundleFeatureCommit(t, f, 1)
	head := runGit(t, f.wp, "rev-parse", "HEAD")
	runGit(t, f.wp, "push", "-f", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
	advanceBaseWithMixedConflict(t, f)
	if limit > 0 {
		setResolverConfig(t, f, limit)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	return f, deps, head, begin
}

// TestIntegrationMixedConflictChargesOnlyAuthoredDispatch proves a mixed
// authored+generated stop charges exactly ONE reservation for the authored
// decision: the resolver resolves the authored path only, the controller
// regenerates and stages the bundle in the same continue, and the rebase
// completes with used=1.
func TestIntegrationMixedConflictChargesOnlyAuthoredDispatch(t *testing.T) {
	requireRealGit(t)
	f, deps, _, begin := beginMixedBundleConflict(t, 2)
	if begin.Disposition != RebaseDispConflicted {
		t.Fatalf("begin = disp %q (paths %v), want a mixed conflicted stop", begin.Disposition, begin.UnmergedPaths)
	}
	if pathsGeneratedOnly(begin.UnmergedPaths) {
		t.Fatalf("fixture defect: the stop is generated-only, not mixed: %v", begin.UnmergedPaths)
	}
	attempt := begin.Attempt
	ctx := context.Background()

	reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
	if reserve.Disposition != ReserveReserved {
		t.Fatalf("reserve = %q reason %q", reserve.Disposition, reserve.Reason)
	}
	// Native entry must admit exactly the authored conflict set, without
	// granting the resolver ownership of controller-generated bundle outputs.
	for _, tc := range []struct {
		name  string
		paths []string
		valid bool
	}{
		{"authored only", []string{"skills/demo/SKILL.md"}, true},
		{"includes generated", begin.UnmergedPaths, false},
		{"unrelated authored", []string{"skills/other/SKILL.md"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := checkFinalizeEntry(t, f, deps, "docket-rebase-resolver", "resolver", "resolver", attempt, reserve.Reservation, tc.paths)
			if (entry.Result == ResultApplied) != tc.valid {
				t.Errorf("entry result=%s reason=%s, want accepted=%t", entry.Result, entry.Reason, tc.valid)
			}
		})
	}
	// A matching directory name in a different module conveys no controller
	// ownership: all its conflicts still belong to the resolver.
	modulePath := filepath.Join(f.wp, "go.mod")
	moduleBody, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, f.wp, "go.mod", "module example.invalid/other\n")
	for _, tc := range []struct {
		paths []string
		valid bool
	}{
		{[]string{"skills/demo/SKILL.md"}, false},
		{begin.UnmergedPaths, true},
	} {
		entry := checkFinalizeEntry(t, f, deps, "docket-rebase-resolver", "resolver", "resolver", attempt, reserve.Reservation, tc.paths)
		if (entry.Result == ResultApplied) != tc.valid {
			t.Errorf("foreign-module entry result=%s reason=%s, want accepted=%t", entry.Result, entry.Reason, tc.valid)
		}
	}
	if err := os.WriteFile(modulePath, moduleBody, 0o644); err != nil {
		t.Fatal(err)
	}
	// The resolver resolves ONLY the authored path and reports only it — leaving
	// every bundle output for the controller.
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", "reconciled skill content\n")
	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"skills/demo/SKILL.md"}, ResolverReservation: reserve.Reservation}
	cont := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if cont.Result != ResultApplied || cont.Disposition != RebaseDispRebased {
		t.Fatalf("mixed continue = (%q, %q) reason %q msg %q, want applied/rebased", cont.Result, cont.Disposition, cont.Reason, cont.Message)
	}
	// A completed (rebased) continue carries no resolver counts on its result
	// document — the protocol omits them on non-conflict dispositions
	// (FinalizeRebaseResult: "Zero ... on non-conflict dispositions"), so the
	// authoritative "charged exactly ONE dispatch" check is the receipt assertion
	// below (used=="1"), matching the completed-continue convention in
	// finalize_rebase_integration_test.go's post-restart continue.
	// The regenerated bundle at the final head is drift-clean and reflects the
	// RESOLVED authored content (never generated from unresolved inputs).
	m, payload, err := assets.Generate(f.wp, assets.DefaultAllowedRoots())
	if err != nil {
		t.Fatalf("post-rebase generate: %v", err)
	}
	diffs, err := assets.DiffTree(filepath.Join(f.wp, "internal", "assets", "embedded"), m, payload)
	if err != nil || len(diffs) != 0 {
		t.Fatalf("post-rebase bundle drift: %v %v", diffs, err)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "1" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("receipt after mixed continue = used %q token %q cont %q, want 1/empty/empty", rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
}

// TestIntegrationMixedConflictGenerationFailureNoContinuation proves a
// regeneration failure during a mixed continue refuses BEFORE the
// continuation-started marker is written: the reservation stays outstanding
// (retriable), Git is untouched, and no continuation is recorded.
func TestIntegrationMixedConflictGenerationFailureNoContinuation(t *testing.T) {
	requireRealGit(t)
	f, deps, _, begin := beginMixedBundleConflict(t, 2)
	attempt := begin.Attempt
	ctx := context.Background()
	reserve := FinalizeResolverReserve(ctx, deps, f.repo.invocation, f.id, attempt)
	if reserve.Disposition != ReserveReserved {
		t.Fatalf("reserve = %q", reserve.Disposition)
	}
	writeRepoFile(t, f.wp, "skills/demo/SKILL.md", "reconciled skill content\n")
	deps.RegenerateBundle = func(string) error { return fmt.Errorf("synthetic generation failure") }
	report := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved,
		ConflictedPaths: []string{"skills/demo/SKILL.md"}, ResolverReservation: reserve.Reservation}
	cont := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if cont.Result != ResultBlocked || cont.Reason != ReasonRebaseGitFailed {
		t.Fatalf("continue = (%q, %q), want blocked/%q", cont.Result, cont.Reason, ReasonRebaseGitFailed)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverContinuationStarted != "" {
		t.Errorf("a failed regeneration wrote the continuation-started marker: %q", rec.ResolverContinuationStarted)
	}
	if rec.ResolverReservationToken == "" {
		t.Errorf("the outstanding reservation was lost; a retried continue can no longer verify it")
	}
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseConflicted {
		t.Errorf("Git was mutated by the failed regeneration path: %q", st.Disposition)
	}
	// The SAME reservation retries successfully once regeneration works again.
	deps.RegenerateBundle = nil
	retry := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, report)
	if retry.Result != ResultApplied || retry.Disposition != RebaseDispRebased {
		t.Fatalf("retried continue = (%q, %q) reason %q, want applied/rebased", retry.Result, retry.Disposition, retry.Reason)
	}
}

// TestIntegrationGeneratedOnlyInterruptedReentry proves a fast-path run
// interrupted between stops resumes idempotently on re-entry: the SAME
// finalize.rebase request recovers through the owned receipt, clears the
// remaining generated-only stops without any reservation, and never replays a
// completed continuation (the stopped commit advanced, so re-entry works on the
// NEXT stop, not the cleared one).
func TestIntegrationGeneratedOnlyInterruptedReentry(t *testing.T) {
	requireRealGit(t)
	f, deps, head := prepareBundleConflictsWithoutBegin(t, 2, 3, docketModulePath)
	ctx := context.Background()

	// First entry: regeneration succeeds once, then fails — the run clears stop 1
	// and blocks at stop 2, simulating an interruption mid-loop.
	calls := 0
	deps.RegenerateBundle = func(ws string) error {
		calls++
		if calls > 1 {
			return fmt.Errorf("synthetic interruption")
		}
		return regenerateEmbeddedBundle(ws)
	}
	first := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if first.Result != ResultBlocked || first.Reason != ReasonRebaseGitFailed {
		t.Fatalf("interrupted entry = (%q, %q), want blocked/%q", first.Result, first.Reason, ReasonRebaseGitFailed)
	}
	stoppedAfterFirst, err := f.deps.Client.StoppedRebaseCommit(ctx, f.wp)
	if err != nil {
		t.Fatalf("probe stopped commit: %v", err)
	}

	// Re-entry with the IDENTICAL request: recoverFromReceipt adopts the owned
	// attempt and the fast path resumes from the live stop.
	deps.RegenerateBundle = nil
	second := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
		t.Fatalf("re-entry = (%q, %q) reason %q msg %q, want applied/rebased", second.Result, second.Disposition, second.Reason, second.Message)
	}
	rec := reloadReceipt(t, f)
	if rec.ResolverUsed != "0" || rec.ResolverReservationToken != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("re-entry spent resolver state: used %q token %q cont %q", rec.ResolverUsed, rec.ResolverReservationToken, rec.ResolverContinuationStarted)
	}
	_ = stoppedAfterFirst // documents that re-entry resumed from the live stop
}
