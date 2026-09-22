<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0442 — Rebase again when main advances after finalize publishes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-22-0442-rebase-again-when-main-advances-after-finalize-publishes.md)**
<!-- docket:backlink:end -->
# Rebase Again When Main Advances After Finalize Publishes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend finalize's existing base-refresh (change 0438) with exactly one additional admissible case: a completed, clean, quiescent owned rewrite whose result finalize itself already **published** — proven by the receipt's completed-gate publish checkpoint plus matching local, remote, and PR heads — may be forward-refreshed onto the newly advanced effective base, re-keying the publication lease to the proven published head, preserving prior resolutions and consumed resolver budget, and always re-running the finalize suite.

**Architecture:** All product changes live in `internal/app/finalize_rebase.go`'s `refreshOwnedRewrite` (plus one new pure admission helper). The gap: after `FinalizePublish` pushes the rebased head B, the remote feature head no longer equals the receipt's recorded lease (`OrigRemoteHead` = A), so `refreshOwnedRewrite`'s lease check refuses even though the remote holds Docket's own tested result. The fix classifies that moved remote as "published" only when the receipt's publish checkpoint (change 0408) proves it: checkpoint head == current local head == authoritative remote head == open PR head, checkpoint base == receipt's recorded base, checkpoint PR number == the open PR, and the checkpoint evidence re-verifies green for that head. Admission is a proof about a **historical tested result plus its current publication** — never permission to skip the new suite run. Under the existing workspace operation lock the receipt is reloaded and compared (existing 0438 guard) and, for the published case only, the local/remote/PR facts are re-probed before the lease is replaced. The refresh write reuses 0438's machinery, differing only in setting `OrigRemoteHead` to the proven published head. No new receipt fields, commands, config, reason constants, or stores.

**Tech Stack:** Go; real-git integration tests in `internal/app` behind the `integration` build tag; existing fakes (`fakeRebaseGitHub`, `headEvidenceGate`); `internal/workspace` receipt/rewrite services; embedded skill assets via `go generate ./internal/assets`.

**Spec:** `docs/superpowers/specs/2026-09-22-rebase-again-when-main-advances-after-finalize-publishes-design.md` (synchronized copy under `.docket/`; the spec travels on the `docket` metadata branch). Change: `docs/changes/active/0442-rebase-again-when-main-advances-after-finalize-publishes.md`.

## Global Constraints

- Bounded extension of the existing refresh: **no** new commands, receipt fields, stores, configuration, retry policies, or budget replenishment; **no** merge-race detection changes; the unpublished (0438) refresh case is preserved unchanged.
- Reuse the closed reason vocabulary: the published-case refusal keeps `ReasonRebaseRemoteHeadMismatch` (`remote-head-mismatch`); changed-under-lock facts reuse `ReasonRebaseRefreshContended` (`refresh-contended`); errored probes reuse the existing `ResultExternalFailed` reasons. Mint no new reason constants.
- Admission never weakens `checkpointDecision` and never calls it with a substituted old base to obtain a suite skip; a changed current test command cannot reuse old evidence — the refreshed gate uses the current resolved configuration and **always runs** (`forceRetest`).
- Every refusal retains local work: receipt byte-unchanged (except where a contract explicitly clears state), workspace head unchanged, remote feature head never overwritten.
- Head comparisons are full-length lowercase string equality (the caller normalizes); empty never matches — `publishCheckpointOf`'s all-or-none rule guarantees non-empty checkpoint members.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers (ADR-0054).
- Focused test runs use `-tags integration -count=1` (defeat the Go test cache on every mutation probe and re-verification).
- The whole-suite gate at build end is whatever `build.test_command` resolves to (`go run ./cmd/docket development test`), run from source; read the budget report even on green.

---

### Task 1: Published-result admission and lease replacement in `refreshOwnedRewrite`

**Files:**
- Modify: `internal/app/finalize_rebase.go` — `refreshOwnedRewrite` (the lease check that reads `rec.OrigRemoteHead`, and the `refreshed` receipt write), plus one new helper `admitPublishedRefresh` placed beside `publishCheckpointOf`/`checkpointDecision`.
- Test: `internal/app/finalize_rebase_integration_test.go` — new helper `setupPublishedRefresh` and test `TestIntegrationFinalizeRebasePublishedResultForwardRefresh`.

**Interfaces:**
- Consumes (existing, unchanged): `publishCheckpointOf(rec workspace.RebaseReceipt) (publishCheckpoint, bool)`; `evidence.Verify([]byte, string) evidence.Verdict` / `evidence.VerdictVerified`; `probeRebasePR`, `probeRebaseHeads` results already threaded into `refreshOwnedRewrite` as `pr githubcli.PullRequest`, `newBaseHead, remoteHead gitcli.ObjectID`; fixture helpers `setupRebaseFixture`, `reserveResolveContinue`, `headEvidenceGate`, `fakeRebaseGitHub`, `f.prForHead`, `f.localHead`, `f.svc.PublishRewrite`.
- Produces: `admitPublishedRefresh(rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, localHead, remoteHead gitcli.ObjectID, id int) *FinalizeRebaseResult` (nil = admitted); a `published bool` local in `refreshOwnedRewrite` that Task 3 extends with the under-lock fact re-probe; test helper `setupPublishedRefresh(t *testing.T) (*rebaseFixture, FinalizeDeps, *headEvidenceGate, *fakeRebaseGitHub, string)` (returns the published head B) that Tasks 2 and 3 reuse.

- [ ] **Step 1: Write the failing integration test**

Add to `internal/app/finalize_rebase_integration_test.go` (it already has `//go:build integration` and the needed imports):

```go
// setupPublishedRefresh drives the exact pre-conditions of change 0442's gap: a
// conflict-resolved rebase A→B whose local gate PASSED (recording the publish
// checkpoint for B), B published through the real receipt-scoped publication
// seam under the exact lease A (so the receipt's recorded lease is now stale),
// the open PR renamed to B, and main advanced AGAIN after the publication. It
// returns the fixture, the deps (headEvidenceGate so checkpoint evidence
// certifies the head the gate actually saw), the two fakes, and the published
// head B.
func setupPublishedRefresh(t *testing.T) (*rebaseFixture, FinalizeDeps, *headEvidenceGate, *fakeRebaseGitHub, string) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	// The base conflictingly rewrites the feature's file so the rebase stops and
	// a resolver-authored resolution is carried into B.
	f.repo.writerAdvance(t, "main", map[string]string{"feature.txt": "conflicting base content\n"})
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &headEvidenceGate{t: t}
	deps := f.finalizeDeps(gh, gate)
	begin := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if begin.Disposition != RebaseDispConflicted {
		t.Fatalf("begin = %q (reason %q msg %q), want conflicted", begin.Disposition, begin.Reason, begin.Message)
	}
	_, cont := reserveResolveContinue(t, f, deps, begin.Attempt, begin.UnmergedPaths, 1)
	if cont.Disposition != RebaseDispRebased || gate.calls != 1 {
		t.Fatalf("continue = %q (reason %q) gate calls %d, want rebased with one gate run", cont.Disposition, cont.Reason, gate.calls)
	}
	published := f.localHead()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt before publication: present=%v err=%v", present, err)
	}
	if _, ok := publishCheckpointOf(rec); !ok {
		t.Fatalf("no publish checkpoint recorded after the PASSED gate: %+v", rec)
	}
	// Publish B through the REAL publication seam: exact-lease push A→B. The
	// receipt's recorded lease (OrigRemoteHead = A) is deliberately left stale —
	// that is the gap under test.
	outcome, perr := f.svc.PublishRewrite(context.Background(),
		workspace.RewriteRequest{Dir: f.metaDir, Receipt: rec, NewHead: published})
	if perr != nil || outcome != workspace.RewritePublished {
		t.Fatalf("PublishRewrite = %q err %v, want published", outcome, perr)
	}
	// The open PR now names the published head B (as GitHub would after the push).
	gh.prs = []githubcli.PullRequest{f.prForHead(published, "")}
	// Main advances AFTER the publication and BEFORE the merge (non-conflicting).
	f.repo.writerAdvance(t, "main", map[string]string{"later.txt": "post-publish base work\n"})
	return f, deps, gate, gh, published
}

// TestIntegrationFinalizeRebasePublishedResultForwardRefresh covers acceptance
// item 1 (and the publish half of item 2) of the 0442 spec: re-entering
// finalize.rebase with the PUBLISHED head B forward-refreshes the owned attempt
// onto the advanced base — preserved resolution, new base ancestry, fresh
// attempt token, lease re-keyed to B, unchanged resolver budget, and a fresh
// suite run — and the next result publishes under exactly lease B.
func TestIntegrationFinalizeRebasePublishedResultForwardRefresh(t *testing.T) {
	requireRealGit(t)
	f, deps, gate, _, published := setupPublishedRefresh(t)
	recBefore, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)

	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: published})
	if out.Result != ResultApplied || out.Disposition != RebaseDispRebased {
		t.Fatalf("published refresh = %q/%q (reason %q msg %q), want applied/rebased",
			out.Result, out.Disposition, out.Reason, out.Message)
	}
	// A published refresh ALWAYS retests: old evidence/checkpoint never
	// authorizes the refreshed head (spec: proof of publication, not a skip).
	if out.Gate == nil || out.Gate.Compose != gateComposeRan || gate.calls != 2 {
		t.Fatalf("gate = %+v calls %d, want compose ran with a second suite run", out.Gate, gate.calls)
	}
	// The conflict resolution carried into B survived the second rewrite.
	if got := readRepoFile(t, f.wp, "feature.txt"); got != "reconciled content for cycle 1\n" {
		t.Fatalf("resolution lost: feature.txt = %q", got)
	}
	next := f.localHead()
	if next == published {
		t.Fatalf("the refresh did not rewrite the head onto the advanced base")
	}
	rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if rec.OrigHead != published {
		t.Errorf("refreshed OrigHead = %q, want the published head %q", rec.OrigHead, published)
	}
	if rec.OrigRemoteHead != published {
		t.Errorf("refreshed lease = %q, want re-keyed to the proven published head %q (was %q)",
			rec.OrigRemoteHead, published, recBefore.OrigRemoteHead)
	}
	if rec.Attempt == recBefore.Attempt {
		t.Errorf("refresh kept the superseded attempt token %q", rec.Attempt)
	}
	if rec.BaseHead == recBefore.BaseHead {
		t.Errorf("refreshed BaseHead unchanged %q; want the advanced base", rec.BaseHead)
	}
	if rec.ResolverLimit != recBefore.ResolverLimit || rec.ResolverUsed != recBefore.ResolverUsed {
		t.Errorf("budget changed: %s/%s -> %s/%s (base movement never replenishes)",
			recBefore.ResolverUsed, recBefore.ResolverLimit, rec.ResolverUsed, rec.ResolverLimit)
	}
	// The superseded checkpoint (old base) is gone; the refresh's own PASSED gate
	// recorded a fresh one for the NEW head against the NEW base.
	if rec.PublishCheckpointHead != next {
		t.Errorf("checkpoint head = %q, want the refreshed head %q (old-base checkpoint must not survive)",
			rec.PublishCheckpointHead, next)
	}
	if rec.PublishCheckpointBaseHead != rec.BaseHead {
		t.Errorf("checkpoint base = %q, want the refreshed base %q", rec.PublishCheckpointBaseHead, rec.BaseHead)
	}
	// Acceptance item 2 (publish half): the next result publishes under exactly
	// lease B — the re-keyed OrigRemoteHead is what the exact-lease push consumes.
	outcome, perr := f.svc.PublishRewrite(context.Background(),
		workspace.RewriteRequest{Dir: f.metaDir, Receipt: rec, NewHead: next})
	if perr != nil || outcome != workspace.RewritePublished {
		t.Fatalf("post-refresh PublishRewrite = %q err %v, want published under lease %q", outcome, perr, published)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails on the current gap**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublishedResultForwardRefresh' -count=1 -v`

Expected: FAIL — the refresh call returns `blocked` with reason `remote-head-mismatch` (the remote feature head B is not the receipt's recorded lease A). If it fails for any *other* reason, stop and fix the fixture first — the failure must reproduce the spec's gap, not a setup defect.

- [ ] **Step 3: Implement the admission helper and the lease-check extension**

In `internal/app/finalize_rebase.go`, add beside `checkpointDecision`:

```go
// admitPublishedRefresh decides whether a remote feature head that no longer
// equals the receipt's recorded publication lease is docket's OWN published
// result (change 0442) — the one additional admissible moved-remote case.
// Admission requires the receipt's complete publish checkpoint (change 0408)
// whose tested head equals the current local head, the authoritative remote
// feature head, AND the open PR's head; the checkpoint's recorded base must be
// the receipt's recorded base, its PR number the open PR's, and its evidence
// must re-verify green for that exact head. This proves a historical tested
// result plus its current publication — it is NEVER permission to skip the new
// suite run (the refresh always retests under the currently resolved
// configuration; checkpointDecision is deliberately not called here, so no old
// base can be substituted to obtain a skip). Any missing or inconsistent proof
// returns the retained lease refusal; nil means admitted.
func admitPublishedRefresh(rc *rebaseContext, pr githubcli.PullRequest, rec workspace.RebaseReceipt, localHead, remoteHead gitcli.ObjectID, id int) *FinalizeRebaseResult {
	refuse := func() *FinalizeRebaseResult {
		r := rebaseRefusal(OperationFinalizeRebase, ResultBlocked, RebaseDispBlocked, ReasonRebaseRemoteHeadMismatch,
			"the remote feature head is not the receipt's recorded publication lease, and the owned checkpoint does not prove it is this rewrite's published result — retained, not refreshed", id)
		return &r
	}
	cp, ok := publishCheckpointOf(rec)
	if !ok {
		return refuse()
	}
	currentHead := strings.ToLower(string(localHead))
	if currentHead == "" ||
		cp.Head != currentHead ||
		strings.ToLower(string(remoteHead)) != currentHead ||
		strings.ToLower(pr.HeadCommit) != currentHead ||
		cp.BaseHead != rec.BaseHead ||
		cp.PRNumber != strconv.Itoa(pr.Number) {
		return refuse()
	}
	if evidence.Verify([]byte(cp.Evidence), currentHead) != evidence.VerdictVerified {
		return refuse()
	}
	return nil
}
```

In `refreshOwnedRewrite`, replace the lease check (the `if string(remoteHead) != rec.OrigRemoteHead { … ReasonRebaseRemoteHeadMismatch … }` block whose comment begins "The publication lease must be intact") with:

```go
	// Publication-lease classification (change 0442 extends change 0438): the
	// unpublished case requires the remote feature head to still equal the
	// recorded lease. The ONE additional admissible case is a moved remote that
	// is provably this rewrite's own published result — the owned completed-gate
	// checkpoint plus matching local, remote, and PR heads (admitPublishedRefresh).
	// Anything else — checkpoint-less, ambiguous, or foreign — is retained.
	published := string(remoteHead) != rec.OrigRemoteHead
	if published {
		if r := admitPublishedRefresh(rc, pr, rec, localHead, remoteHead, id); r != nil {
			return r
		}
	}
```

(Note the helper returns `*FinalizeRebaseResult`; keep `return *r` / `return r` consistent with how you declare it — the surrounding function returns a value, so dereference: `return *r`.)

Then, in the `refreshed := disk` write block further down, re-key the lease for the published case only (0438's unpublished case must keep the lease untouched):

```go
	refreshed := disk
	refreshed.OrigHead = string(localHead)
	if published {
		// The proven published head becomes the new exact publication lease: the
		// next PublishRewrite pushes over exactly this observed remote value.
		refreshed.OrigRemoteHead = string(remoteHead)
	}
```

Also extend `refreshOwnedRewrite`'s doc comment ("It preserves … the exact remote publication lease …") to name the published case: the lease is preserved for an unpublished rewrite and re-keyed to the proven published head for a published one.

- [ ] **Step 4: Run the new test and the neighboring 0438 refresh tests**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublishedResultForwardRefresh|TestIntegrationFinalizeRebaseRecoveryForwardRefresh' -count=1 -v`

Expected: ALL PASS — the new published-case test and every existing `RecoveryForwardRefresh*` test (the unpublished case must be byte-for-byte preserved: same refusals, same untouched lease).

- [ ] **Step 5: Run the package's unit tests too**

Run: `go test ./internal/app/ -count=1`

Expected: PASS (no unit-level regression from the edited function).

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go
git commit -m "feat(app): admit a proven published result into the finalize base refresh (change 0442)"
```

---

### Task 2: Refusal matrix — every unproven moved remote stays retained

**Files:**
- Modify: `internal/app/finalize_rebase_integration_test.go` — new test `TestIntegrationFinalizeRebasePublishedRefreshRefusals` (subtests), reusing `setupPublishedRefresh` from Task 1.
- Modify (only if a subtest exposes a real gap): `internal/app/finalize_rebase.go`.

**Interfaces:**
- Consumes: `setupPublishedRefresh` (Task 1), `f.svc.ReadRebaseReceipt` / `WriteRebaseReceipt`, `runGit`, `writeRepoFile`, `greenEvidenceFor`, reason constants `ReasonRebaseRemoteHeadMismatch`, `ReasonRebaseMovedBase`, `ReasonRebaseWorkspaceDirty`.
- Produces: test helper `assertPublishedRefusalRetained(t, f, want workspace.RebaseReceipt, wantLocal, wantRemote string)` used by every subtest (and reusable by Task 3).

These subtests are the **mutation detectors** for `admitPublishedRefresh`'s conjuncts: each one exists so that deleting one admission conjunct turns at least one subtest red (Task 4 proves this by execution). Assert the mechanism (the exact reason), not just "it failed".

- [ ] **Step 1: Write the failing/probing subtests**

```go
// assertPublishedRefusalRetained proves a refusal retained everything: the
// on-disk receipt is byte-unchanged, the workspace head did not move, and the
// remote feature head was not overwritten.
func assertPublishedRefusalRetained(t *testing.T, f *rebaseFixture, want workspace.RebaseReceipt, wantLocal, wantRemote string) {
	t.Helper()
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after refusal: present=%v err=%v", present, err)
	}
	if rec != want {
		t.Errorf("a refusal mutated the receipt:\n got %+v\nwant %+v", rec, want)
	}
	if got := f.localHead(); got != wantLocal {
		t.Errorf("local head moved on a refusal: %q -> %q", wantLocal, got)
	}
	if got := runGit(t, f.repo.origin, "rev-parse", "refs/heads/feat/"+f.slug); got != wantRemote {
		t.Errorf("remote feature head moved on a refusal: %q -> %q", wantRemote, got)
	}
}

// TestIntegrationFinalizeRebasePublishedRefreshRefusals covers acceptance item 4
// (and the remote-edit half of item 2): every moved remote that is NOT provably
// this rewrite's published result is retained — no refresh, no overwrite, no
// budget movement. Each subtest is the mutation detector for one admission
// conjunct of admitPublishedRefresh.
func TestIntegrationFinalizeRebasePublishedRefreshRefusals(t *testing.T) {
	requireRealGit(t)
	reenter := func(f *rebaseFixture, deps FinalizeDeps, head string) FinalizeRebaseResult {
		return FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: head})
	}

	t.Run("missing-checkpoint-refuses", func(t *testing.T) {
		f, deps, gate, _, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		rec.PublishCheckpointHead, rec.PublishCheckpointBaseHead = "", ""
		rec.PublishCheckpointCommand, rec.PublishCheckpointGate = "", ""
		rec.PublishCheckpointPRNumber, rec.PublishCheckpointEvidence = "", ""
		if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
			t.Fatal(err)
		}
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseRemoteHeadMismatch {
			t.Fatalf("= %q/%q, want blocked/remote-head-mismatch (no checkpoint proof)", out.Result, out.Reason)
		}
		if gate.calls != 1 {
			t.Errorf("a refusal ran the gate (%d calls)", gate.calls)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("stale-evidence-refuses", func(t *testing.T) {
		// Checkpoint evidence certifying a DIFFERENT head (the pre-rebase head A)
		// must not admit: stale PR-body-shaped evidence never skips or admits.
		f, deps, _, _, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		rec.PublishCheckpointEvidence = greenEvidenceFor(t, f.head)
		if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
			t.Fatal(err)
		}
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseRemoteHeadMismatch {
			t.Fatalf("= %q/%q, want blocked/remote-head-mismatch (evidence does not verify for B)", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("checkpoint-head-mismatch-refuses", func(t *testing.T) {
		// A checkpoint recorded for some OTHER head (here: the pre-rebase head A,
		// with matching evidence so only the head conjunct differs) must not admit.
		f, deps, _, _, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		rec.PublishCheckpointHead = strings.ToLower(f.head)
		rec.PublishCheckpointEvidence = greenEvidenceFor(t, f.head)
		if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
			t.Fatal(err)
		}
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseRemoteHeadMismatch {
			t.Fatalf("= %q/%q, want blocked/remote-head-mismatch (checkpoint names another head)", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("pr-number-mismatch-refuses", func(t *testing.T) {
		f, deps, _, gh, published := setupPublishedRefresh(t)
		pr := f.prForHead(published, "")
		pr.Number = 2 // checkpoint recorded PR 1
		gh.prs = []githubcli.PullRequest{pr}
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseRemoteHeadMismatch {
			t.Fatalf("= %q/%q, want blocked/remote-head-mismatch (open PR is not the recorded one)", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("foreign-remote-edit-refuses-without-overwrite", func(t *testing.T) {
		// A third party pushed on top of B: remote != local, so the proof fails.
		// The refusal must leave the foreign remote head in place (item 2's
		// intervening-remote-edit requirement at the admission layer; the exact-
		// lease push protects the publish layer).
		f, deps, _, _, published := setupPublishedRefresh(t)
		foreign := runGit(t, f.wp, "commit-tree", "HEAD^{tree}", "-p", "HEAD", "-m", "foreign edit")
		runGit(t, f.repo.origin, "update-ref", "refs/heads/feat/"+f.slug, foreign)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseRemoteHeadMismatch {
			t.Fatalf("= %q/%q, want blocked/remote-head-mismatch (remote is not the tested head)", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, foreign)
	})

	t.Run("divergent-base-refuses", func(t *testing.T) {
		// The base was force-rewritten (moved to a non-descendant of the recorded
		// base): the published proof holds but forward-only ancestry fails.
		f, deps, _, _, published := setupPublishedRefresh(t)
		runGit(t, f.repo.origin, "update-ref", "refs/heads/main", f.baseTip)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseMovedBase {
			t.Fatalf("= %q/%q, want blocked/base-moved-under-receipt (base does not descend the recorded base)", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("unsettled-resolver-work-refuses", func(t *testing.T) {
		f, deps, _, _, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		rec.ResolverReservationToken = "res-outstanding"
		rec.ResolverReservationStopped = strings.Repeat("a", 40)
		if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
			t.Fatal(err)
		}
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseMovedBase {
			t.Fatalf("= %q/%q, want blocked/base-moved-under-receipt (unsettled resolver work)", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("dirty-workspace-refuses", func(t *testing.T) {
		f, deps, _, _, published := setupPublishedRefresh(t)
		writeRepoFile(t, f.wp, "scratch.txt", "uncommitted\n")
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		out := reenter(f, deps, published)
		if out.Result != ResultBlocked || out.Reason != ReasonRebaseWorkspaceDirty {
			t.Fatalf("= %q/%q, want blocked/workspace-dirty", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})

	t.Run("replaying-the-old-head-fails-the-pr-check", func(t *testing.T) {
		// Documented contract: post-publication re-entry must supply the CURRENT
		// published head from context.finalize; replaying the pre-rebase head A
		// correctly fails the PR-head check before any refresh classification.
		f, deps, _, _, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		out := reenter(f, deps, f.head)
		if out.Result != ResultBlocked || out.Reason != ReasonRebasePRHeadMismatch {
			t.Fatalf("= %q/%q, want blocked/pr-head-mismatch", out.Result, out.Reason)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})
}
```

Note on `f.repo.origin`: it is the fixture's origin repository path (the same value `blobVersionAt(t, repo.origin, …)` reads). If the field is spelled differently in `gitRepo`, use the actual field — verify by reading the `gitRepo` type — and keep the `update-ref` mechanism.

- [ ] **Step 2: Run the subtests**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublishedRefreshRefusals' -count=1 -v`

Expected: ALL PASS against Task 1's implementation. Any red subtest is a real admission gap — fix it in `admitPublishedRefresh` (or the surrounding ordering in `refreshOwnedRewrite`), never by weakening the subtest. In particular `unsettled-resolver-work` and `dirty-workspace` must be refused by the checks that run BEFORE the lease classification (existing 0438 order: unsettled resolver → workspace state → lease); do not reorder them.

- [ ] **Step 3: Commit**

```bash
git add internal/app/finalize_rebase_integration_test.go internal/app/finalize_rebase.go
git commit -m "test(app): retain every unproven moved remote at the published-refresh gate (change 0442)"
```

---

### Task 3: Under-lock fact re-probe, concurrency, and interruption replay

**Files:**
- Modify: `internal/app/finalize_rebase.go` — `refreshOwnedRewrite`, inside the operation-lock section (after the `disk != rec` decide-and-act guard, before `refreshed := disk`).
- Test: `internal/app/finalize_rebase_integration_test.go` — `TestIntegrationFinalizeRebasePublishedRefreshUnderLockReprobe`, `TestIntegrationFinalizeRebasePublishedRefreshContended`, `TestIntegrationFinalizeRebasePublishedRefreshInterruption`.

**Interfaces:**
- Consumes: `published bool` and `admitPublishedRefresh` (Task 1); `setupPublishedRefresh`, `assertPublishedRefusalRetained` (Tasks 1–2); existing `divergeOnLockWorkspace` pattern (defined above `TestIntegrationFinalizeRebaseRecoveryForwardRefreshContended`); `deps.Workspace.Inspect`, `deps.Planning.Client.ProbeRemoteBranch`, `probeRebasePR`, `originRemote`, `ReasonRebaseRefreshContended`.
- Produces: no new exported surface; a `hookOnLockWorkspace` test wrapper (an `AcquireOperationLock` hook running an arbitrary `func()` once) reusable for fact-perturbation tests.

- [ ] **Step 1: Write the failing tests**

```go
// hookOnLockWorkspace runs hook exactly once, at the moment the refresh acquires
// the workspace operation lock — i.e. AFTER the lock-free classification admitted
// the published case and BEFORE the under-lock fact re-probe. Every other method
// delegates to the embedded real workspace.
type hookOnLockWorkspace struct {
	FinalizeWorkspace
	hook  func()
	fired bool
}

func (w *hookOnLockWorkspace) AcquireOperationLock(dir string) (func(), error) {
	release, err := w.FinalizeWorkspace.AcquireOperationLock(dir)
	if err != nil {
		return release, err
	}
	if !w.fired {
		w.fired = true
		w.hook()
	}
	return release, err
}

// TestIntegrationFinalizeRebasePublishedRefreshUnderLockReprobe covers the spec's
// decide-and-act requirement for the published case: the local/remote/PR facts
// that admitted the lease replacement are re-proven under the operation lock;
// changed facts refuse without mutation.
func TestIntegrationFinalizeRebasePublishedRefreshUnderLockReprobe(t *testing.T) {
	requireRealGit(t)

	t.Run("remote-moved-between-admission-and-lock", func(t *testing.T) {
		f, deps, _, _, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		// At lock time, move the remote feature ref back to the pre-rebase head A:
		// the lock-free admission saw B, the under-lock re-probe must see A and refuse.
		deps.Workspace = &hookOnLockWorkspace{FinalizeWorkspace: deps.Workspace, hook: func() {
			runGit(t, f.repo.origin, "update-ref", "refs/heads/feat/"+f.slug, f.head)
		}}
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: published})
		if out.Result != ResultContended || out.Reason != ReasonRebaseRefreshContended {
			t.Fatalf("= %q/%q (msg %q), want contended/refresh-contended", out.Result, out.Reason, out.Message)
		}
		assertPublishedRefusalRetained(t, f, rec, published, f.head)
	})

	t.Run("pr-changed-between-admission-and-lock", func(t *testing.T) {
		f, deps, _, gh, published := setupPublishedRefresh(t)
		rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		deps.Workspace = &hookOnLockWorkspace{FinalizeWorkspace: deps.Workspace, hook: func() {
			gh.prs = []githubcli.PullRequest{f.prForHead(f.head, "")} // PR now names A
		}}
		out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: published})
		// The re-probe reuses probeRebasePR against the current local head, so a
		// changed PR surfaces as its typed refusal; the receipt must be untouched.
		if out.Result == ResultApplied {
			t.Fatalf("a changed PR was refreshed over: %+v", out)
		}
		assertPublishedRefusalRetained(t, f, rec, published, published)
	})
}

// TestIntegrationFinalizeRebasePublishedRefreshContended extends 0438's
// divergeOnLockWorkspace coverage to the published case: a concurrent
// refresh/continue rewrote the receipt between classification and lock; the
// loser refuses refresh-contended and retains everything.
func TestIntegrationFinalizeRebasePublishedRefreshContended(t *testing.T) {
	requireRealGit(t)
	f, deps, _, _, published := setupPublishedRefresh(t)
	rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	diverged := rec
	diverged.Attempt = "20260922T000000Z-winner"
	deps.Workspace = &divergeOnLockWorkspace{FinalizeWorkspace: deps.Workspace, diverged: diverged}
	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: published})
	if out.Result != ResultContended || out.Reason != ReasonRebaseRefreshContended {
		t.Fatalf("= %q/%q, want contended/refresh-contended", out.Result, out.Reason)
	}
	assertPublishedRefusalRetained(t, f, diverged, published, published)
}

// TestIntegrationFinalizeRebasePublishedRefreshInterruption covers acceptance
// item 5's interruption leg: a crash after the refreshed receipt persisted but
// before Git started resumes the SAME refreshed attempt (pre-start resume), and
// a valid replay after completion reuses the new checkpoint without a duplicate
// gate run.
func TestIntegrationFinalizeRebasePublishedRefreshInterruption(t *testing.T) {
	requireRealGit(t)
	f, deps, gate, _, published := setupPublishedRefresh(t)
	recBefore, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	newBaseHead := runGit(t, f.repo.origin, "rev-parse", "refs/heads/main")
	// Hand-write the receipt the published refresh WOULD persist, Git untouched
	// (the crash window between WriteRebaseReceipt and BeginRebase).
	crashed := recBefore
	crashed.OrigHead = published
	crashed.OrigRemoteHead = published
	crashed.BaseHead = newBaseHead
	crashed.Attempt = "20260922T101010Z-published-refresh-crash"
	crashed.GateDriveID, crashed.GateOwnerGeneration = "", ""
	crashed.PublishCheckpointHead, crashed.PublishCheckpointBaseHead = "", ""
	crashed.PublishCheckpointCommand, crashed.PublishCheckpointGate = "", ""
	crashed.PublishCheckpointPRNumber, crashed.PublishCheckpointEvidence = "", ""
	if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, crashed); err != nil {
		t.Fatal(err)
	}
	out := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: published})
	if out.Disposition != RebaseDispRebased || out.Attempt != crashed.Attempt {
		t.Fatalf("pre-start resume = %q attempt %q (reason %q), want rebased with the recorded attempt %q",
			out.Disposition, out.Attempt, out.Reason, crashed.Attempt)
	}
	if out.Gate == nil || out.Gate.Compose != gateComposeRan {
		t.Fatalf("gate = %+v, want compose ran (recovery never skips on PR evidence)", out.Gate)
	}
	callsAfter := gate.calls
	// Valid replay of the identical invocation: the completed rewrite's fresh
	// checkpoint is reused — one refreshed attempt, no duplicate gate.
	replay := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: published})
	if replay.Disposition != RebaseDispRebased || replay.Gate == nil || replay.Gate.Compose != gateComposeSkipped {
		t.Fatalf("replay = %q gate %+v, want rebased with the checkpoint skip", replay.Disposition, replay.Gate)
	}
	if gate.calls != callsAfter {
		t.Errorf("replay re-ran the gate: %d -> %d calls", callsAfter, gate.calls)
	}
	if replay.Attempt != crashed.Attempt {
		t.Errorf("replay attempt = %q, want the one refreshed attempt %q", replay.Attempt, crashed.Attempt)
	}
}
```

Note for the interruption replay subtest: the replay's PR probe runs against `Head: published` while the local head has moved to the refreshed rewrite — if `probeRebasePR` refuses on the replay (`pr-head-mismatch` because the PR still names B and the request supplies B, which is fine; the PR head must equal the request head), keep the request head `published` and, if the checkpoint-skip path instead requires the PR to name the NEW head, update `gh.prs` to the new local head before the replay and pass that head — mirror whatever the existing `setupPassedRebaseCheckpoint` reuse tests (change 0408) do for their replay invocation. The load-bearing asserts are: one refreshed attempt, `gateComposeSkipped`, and no additional gate call.

- [ ] **Step 2: Run to verify the re-probe tests fail**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublishedRefresh(UnderLockReprobe|Contended|Interruption)' -count=1 -v`

Expected: `UnderLockReprobe` FAILS (no re-probe exists yet — the refresh proceeds on stale facts). `Contended` should already PASS (0438's `disk != rec` guard). `Interruption` should already PASS (0438's pre-start resume + 0408's checkpoint replay). Investigate any different outcome before implementing.

- [ ] **Step 3: Implement the under-lock re-probe**

In `refreshOwnedRewrite`, immediately after the `if !present || disk != rec { … refresh-contended … }` guard and before `refreshed := disk`, add:

```go
	// Published-case fact re-probe (change 0442): the write below adopts the
	// moved remote head as this rewrite's own published result, so the exact
	// facts that admitted it — the clean local head, the authoritative remote
	// feature head, and the open PR — are re-proven under the lock, against the
	// copies the lease replacement will act on. A changed fact is contended
	// (re-read and re-admit); an unknown (errored) fact refuses without mutation.
	if published {
		insp, ierr := deps.Workspace.Inspect(ctx, workspace.InspectRequest{Repository: rc.repo, Target: rc.target})
		if ierr != nil {
			return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseWorkspaceProbe, ierr.Error(), id)
		}
		rref, perr := deps.Planning.Client.ProbeRemoteBranch(ctx, rc.repo, originRemote, rc.target.FeatureRef)
		if perr != nil {
			return rebaseRefusal(op, ResultExternalFailed, RebaseDispBlocked, ReasonRebaseRemoteFeatureProbe, perr.Error(), id)
		}
		pr2, prRefusal := probeRebasePR(ctx, deps, repoDir, rc, string(localHead))
		if prRefusal != nil {
			return *prRefusal
		}
		if insp.Kind != workspace.StateReady || insp.HeadCommit != localHead ||
			rref.State != gitcli.RemoteRefFound || rref.Commit != remoteHead ||
			pr2.Number != pr.Number {
			return rebaseRefusal(op, ResultContended, RebaseDispContended, ReasonRebaseRefreshContended,
				"the local, remote, or PR facts that admitted the published-result refresh changed while it was being admitted; re-read context finalize", id)
		}
	}
```

(`refreshOwnedRewrite` already receives `repoDir`; if its current signature omits it, add it and update the single call site in `recoverFromReceipt`, which already holds `repoDir`.)

- [ ] **Step 4: Run all Task 1–3 tests plus the 0438 neighbors**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublished|TestIntegrationFinalizeRebaseRecoveryForwardRefresh' -count=1 -v`

Expected: ALL PASS. Then run the package units: `go test ./internal/app/ -count=1` — PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/finalize_rebase_integration_test.go
git commit -m "feat(app): re-prove published-refresh admission facts under the operation lock (change 0442)"
```

---

### Task 4: Guidance, embedded assets, doc sweep, and mutation probes

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` — the base-advance paragraph (the one beginning "When the effective base merely advances under a **completed, clean, unpublished** owned rebase…").
- Modify: `internal/app/finalize_rebase.go` — the file-header "Load-bearing properties" comment (the bullet asserting "a changed remote feature head … is retained and blocked, never reset or adopted" must now carve out the proven-published case).
- Regenerate: `internal/assets/embedded` via `go generate ./internal/assets` (runs `cmd/genassets`).

**Interfaces:**
- Consumes: everything landed in Tasks 1–3.
- Produces: nothing new — documentation and verification only.

- [ ] **Step 1: Update the finalize guidance**

In `skills/docket-finalize-change/SKILL.md`, directly after the existing 0438 paragraph, add (adjusting only to match the surrounding voice):

```markdown
When the base advances **after** finalize has already published the rebased result — the remote feature head and the open PR both name the tested head — re-entering `finalize.rebase` refreshes the **same owned attempt** onto the new base rather than refusing: the receipt's completed-gate checkpoint plus the matching local, remote, and PR heads prove the moved remote is docket's own published result, the publication lease is re-keyed to that head, prior conflict resolutions and the consumed resolver budget carry forward, and the full local suite always re-runs before the next exact-lease publish. Re-enter with the **current** `--version --head` from `context finalize` — the published head, never the original pre-rebase head, which now correctly fails the PR-head check; once the refreshed attempt is waiting, repeat the identical invocation as usual. A moved remote that is **not** provably docket's published result — a missing or mismatched checkpoint, evidence that no longer verifies, or foreign edits on the branch — stays retained (`remote-head-mismatch`, `blocked`): never overwritten, never adopted.
```

Also update `internal/app/finalize_rebase.go`'s header comment: in the "Load-bearing properties" list, amend the "changed remote feature head … retained and blocked" clause to note the one exception — a remote head proven to be the rewrite's own published result by the owned publish checkpoint and matching local/remote/PR heads is forward-refreshed (change 0442), never adopted blindly.

- [ ] **Step 2: Regenerate the embedded assets**

Run: `go generate ./internal/assets`
Then: `git status --short internal/assets` — expect the embedded tree/manifest to reflect the SKILL.md edit and nothing else.

- [ ] **Step 3: Mutation-test the admission guards**

For each probe: copy the file aside first (`cp internal/app/finalize_rebase.go "${TMPDIR:-/tmp}/finalize_rebase.go.bak."$$` — never rely on `git checkout --`, which restores HEAD and would destroy Tasks 1–3's uncommitted state if you probe before committing; probe AFTER Task 3's commit and restore with `git checkout -- internal/app/finalize_rebase.go`). Run the detector suite with the cache defeated:

`go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublished' -count=1`

| Mutation (in `admitPublishedRefresh` / `refreshOwnedRewrite`) | Expected red detector |
| --- | --- |
| Delete the `cp.Head != currentHead` conjunct | `checkpoint-head-mismatch-refuses` |
| Delete the `strings.ToLower(string(remoteHead)) != currentHead` conjunct | `foreign-remote-edit-refuses-without-overwrite` |
| Delete the `cp.PRNumber != strconv.Itoa(pr.Number)` conjunct | `pr-number-mismatch-refuses` |
| Replace the `evidence.Verify` check with `true` | `stale-evidence-refuses` |
| Make `publishCheckpointOf` failure admit (drop the `!ok` refusal) | `missing-checkpoint-refuses` |
| Drop the `if published { … OrigRemoteHead … }` lease re-key | `TestIntegrationFinalizeRebasePublishedResultForwardRefresh` (post-refresh publish under lease B) |
| Delete the under-lock re-probe block | `remote-moved-between-admission-and-lock` |

Each mutation must produce at least one FAIL naming the expected test; a green run under any mutation is a defect — add the missing assert before proceeding. Restore the original file and re-run the detector suite green after the last probe. Record the probe outcomes in the task's commit message body (one line per mutation).

- [ ] **Step 4: Run the focused set one final time, clean**

Run: `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublished|TestIntegrationFinalizeRebaseRecoveryForwardRefresh|TestIntegrationFinalizeRebaseGatePassedRecordsPublishCheckpoint' -count=1`
And: `go test ./internal/app/ -count=1`

Expected: PASS, PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md internal/assets internal/app/finalize_rebase.go
git commit -m "docs(app): published-result refresh guidance + mutation probes (change 0442)"
```

---

## Suite gate (owned by the build role, not a task)

After the last task, the build's single full-suite gate runs whatever `build.test_command` resolves to (`go run ./cmd/docket development test`), entered from source. Read the budget report even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines are findings). The embedded-assets drift guard and the repo-wide comment/anchor guards run inside that suite — a red there means Task 4's regeneration or comment edits are incomplete.

## Self-review notes (spec coverage)

- Spec "Design" admission conjuncts 1–3 → Task 1 (checkpoint + head/PR/evidence proof, forward-only ancestry via the existing `IsAncestor` gate, existing identity/clean/quiescent/unsettled checks preserved in order) and Task 3 (under-lock reload + fact re-probe).
- Spec "Reuse the existing refresh write … set `OrigHead` and `OrigRemoteHead` to the proven published B, advance `BaseHead`, mint the fresh `Attempt`, clear the old checkpoint, preserve resolver limit/usage" → Task 1 Step 3 (only the `OrigRemoteHead` line is new; the rest is 0438's write, asserted in Task 1's test).
- Spec "always require the full configured finalize suite on the refreshed result, including a mechanically unchanged rebase" → 0438's `forceRetest` already covers the refresh path (`TestIntegrationFinalizeRebaseRecoveryForwardRefreshUnchangedRetests` stays green); Task 1 asserts the fresh run on the published path.
- Acceptance item 1 → Task 1. Item 2 → Task 1 (publish under exactly B) + Task 2 (`foreign-remote-edit`) + Task 4 mutation row on the lease re-key. Item 3 → Task 2 (`stale-evidence`, checkpoint-proof-only admission; no PR-body read is added). Item 4 → Task 2. Item 5 → Task 3. Item 6 → the 0438 tests run in every task's verification step.
- Spec "Document that post-publication re-entry obtains the current version and published head B from `context.finalize`" → Task 4 Step 1 and Task 2's `replaying-the-old-head-fails-the-pr-check` subtest.
- Mutation-testing of new admission guards → Task 4 Step 3. Grooming-time exclusions (no new fields/commands/config/ADR) → Global Constraints.
