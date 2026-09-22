<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0442 — Rebase again when main advances after finalize publishes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-22-0442-rebase-again-when-main-advances-after-finalize-publishes.md)**
<!-- docket:backlink:end -->
# Rebase again when main advances after finalize publishes — Results

**Human action:** None required to merge beyond the normal review of the diff. One decision is worth a glance at merge time: this change raised the word-count ceiling for `skills/docket-finalize-change/SKILL.md` (5421 → 5520) to fit substantive new operator guidance — see Known issues and follow-ups.

## Outcome

Finalize's merge gate rebases a change's branch onto the latest integration base, re-runs the suite, and merges. Change 0438 already handled the base (`main`) advancing *before* finalize pushes its rebased head. This change closes the adjacent gap: when `main` advances *after* finalize has already **published** the rebased head but *before* the PR merges, re-entering finalize used to refuse — it compared the remote feature branch against the receipt's pre-push lease and saw a mismatch, even though the remote already held Docket's own tested result.

Now, re-entering `finalize.rebase` recognises that published result and forward-refreshes the same owned attempt onto the newly advanced base instead of refusing. It admits the moved remote only when it is provably Docket's own published result: the receipt's completed-gate publish checkpoint (change 0408) plus the tested head matching the current local head, the authoritative remote feature head, and the open PR head, with the checkpoint's base and PR number matching and its evidence re-verifying green for that head. On admission it re-keys the publication lease to that proven head, preserves prior conflict resolutions and the already-consumed resolver budget (base movement never replenishes it), and **always** re-runs the full finalize suite against the new base — the admission is proof of a past tested result plus its current publication, never permission to skip the new run. Any moved remote that is not provably Docket's published result — missing or mismatched checkpoint, evidence that no longer verifies, a different PR, or a foreign edit on the branch — stays retained (`remote-head-mismatch`, `blocked`), never overwritten or adopted. Under the workspace operation lock the local/remote/PR facts that admitted the refresh are re-proven before the lease is replaced; a changed fact is `contended` and an errored probe is `external-failed`, each leaving the receipt byte-unchanged.

The change is confined to `refreshOwnedRewrite` and a new `admitPublishedRefresh` helper in `internal/app/finalize_rebase.go`; it reuses the existing conflict, continuation, gate, publication, and merge machinery and adds no new commands, receipt fields, configuration, reason constants, or budget policy. The change-0438 unpublished path is preserved unchanged.

## Human actions and testing

### Optional — Exercise the published-result re-entry end to end

Automated integration tests already cover this behavior; this walkthrough is only for a reader who wants to see the flow against real Git. It reproduces the scenario the change fixes.

Prerequisites: the repository checked out on the merged change, Go toolchain on PATH.

1. Run the focused integration set:
   `go test -tags integration ./internal/app/ -run 'TestIntegrationFinalizeRebasePublished' -count=1`
   Expected: `ok` — the forward-refresh, the refusal matrix, the under-lock re-probe, the contended, and the interruption-replay tests all pass.
2. Read `skills/docket-finalize-change/SKILL.md`, the paragraph beginning "When the base advances **after** finalize has already published the rebased result".
   Expected: it directs an operator to re-enter with the **current** `--version`/`--head` from `context finalize` (the published head), not the pre-rebase head, which now correctly fails the PR-head check.

## Verification performed

- The full configured build suite (`go run ./cmd/docket development test`) was driven to green through the native gate at the final head 69fb48b6, then re-established green after the review fix at head 842b5198 (see the PR's build-evidence block). Suite result: 53 shards, all passing.
- TDD: each admission conjunct and the under-lock re-probe were built test-first; five new integration tests exercise the forward-refresh, the nine-case refusal matrix, the under-lock fact re-probe, concurrency contention, and interruption replay.
- Mutation probes were executed against the admission guards (one conjunct removed at a time, focused detector re-run, restored between): removing the checkpoint-head, remote-head, PR-number, evidence, published-lease-re-key, or under-lock-re-probe guards each reddened its detector. Removing the `!ok` checkpoint-completeness early-return stayed green and was proven redundant — `publishCheckpointOf` returns a zeroed checkpoint on incompleteness, so the head conjunct necessarily refuses it. The `checkpoint-head-mismatch` subtest was strengthened during probing to isolate the `cp.Head` conjunct (its original form also broke the evidence, masking the head guard).
- Independent whole-branch review (deep tier) confirmed the admission logic, the preserved unpublished path, the non-widening lease re-key, and the absence of out-of-scope additions; its one minor finding (an unused parameter) was fixed in-branch.

## Known issues and follow-ups

### Raised word-count budget for the finalize skill

To fit the new post-publication guidance, the ceiling for `skills/docket-finalize-change/SKILL.md` in `internal/repoguard/budgets_test.go` was raised from 5421 to 5520 words. The paragraph was first slimmed by de-duplicating the forward-rebase mechanic shared with the adjacent change-0438 paragraph; the remaining overage is irreducible distinct operator guidance. Impact: the finalize skill is now pinned at exactly 5520 words — a future edit that adds words will fail `TestSkillSizeBudgets` until it slims or re-ratchets. This is a deliberate, confirmed decision recorded here for merge-time awareness, not a defect.

### Reproduction was source-inspected, not replayed

The original gap was confirmed by source inspection and by a first-RED integration test reproducing the exact refusal (`remote-head-mismatch`); the separate session's live reproduction was not independently replayed. The behavioral coverage now added exercises the fixed path directly, so this is a documentation note about provenance rather than an open risk.
