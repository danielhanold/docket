<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0427 — Verdict-path gate recovery never binds the run epoch's worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0427-verdict-path-gate-recovery-never-binds-the-run-epoch-s-workt.md)**
<!-- docket:backlink:end -->
# Verdict-path gate recovery never binds the run epoch's worktree — Results

## Outcome

`resolveGateOwnership` (`internal/app/rungate_verdict.go`) had two verdict-path claim-recovery legs — recovery from an unconfirmed reservation and adoption of a sole committed proof — that both called `ConfirmGateClaim` with an empty worktree `""`. A run epoch recovered exclusively through either leg therefore retained an empty `EpochRecord.Worktree`, so the mutation fence and `run.cancel` teardown could not locate its worktree: cancellation would report `cancelled` while the fence stayed inert, silently defeating the safety property change 0375 established.

The fix threads `PlanningDeps` into `resolveGateOwnership` and adds a private helper `gateRecoveredWorktree(ctx, deps, repoDir, changeID)` that derives the change's logical feature worktree as `filepath.Join(repo.PrimaryWorktree, ".worktrees", change.Slug)` from the selected change's authoritative metadata (pin → corpus → snapshot) and canonical primary repository identity (`Client.Discover`) — the same derivation the fresh-claim path binds. Both recovery legs now resolve that path and pass it to `ConfirmGateClaim`. When repository/change identity cannot be resolved, each leg refuses through the existing `gate-stop … gate-unavailable` / `ReasonGateProofUnavailable` path **before** confirming (and, in the sole-proof leg, before `ReserveGateClaim`), so a refusal writes nothing rather than confirming with an empty path. The feature directory need not exist at bind time; the fence canonicalizes the stored value at compare time. The confirmed-binding continuity branch is unchanged (repairing already-confirmed historical bindings is out of scope). No behavior change to receipt matching, ambiguity refusal, continuation handling, retry accounting, or epoch-less runs.

## Verification performed

- TDD per task: each recovery leg's fix was written against a failing regression that mints a **real** fresh epoch with an empty `Worktree` (`MintEpochRecord(repo, key, "")`, nothing pre-binds it), drives `RunGateVerdict`, asserts the derived logical feature path is stored (with the directory still absent), then cancels via `RunCancel` and asserts a workflow mutation from that worktree is refused specifically as `run-cancelled`.
- Per-call mutation checks (uncached, `-count=1`): restoring the empty `""` argument at each fixed `ConfirmGateClaim` reddens exactly its own regression (binding assert and post-cancel fence assert), proving both halves are load-bearing; the fixed source was restored from a scratch backup copy, never `git checkout --`.
- Unresolved-identity refusal covered for both legs: empty `PlanningDeps` yields `gate-stop gate-unavailable proof-unavailable` before any confirm, with no reservation written in the sole-proof leg.
- Existing sibling-proof and ambiguous-proof rejection tests retained unchanged.
- Full configured build suite through the Go runner (`go run ./cmd/docket development test`), driven through the native gate driver to a terminal `PASSED`: `SUITE files=47 passed=47 failed=0 asserts=403`. Build evidence recorded green at head `67e5deb07e4cafe7d244d697aad932a5b80391c4` and verified.
- Independent whole-branch review (docket-review-standard): verdict clean, no blocker or major findings.

## Findings and limitations

### Suite budget-watch lines (parallel-sensitive, not a breach)

The final green suite's budget report carried `BUDGET WATCH:` lines for several wall-clock-heavy files (`test_go_race`, `test_go_toolchain`, the `internal/app` integration suites), each at parallel-overrun streak 1/5 under `-j11`. These are screening findings on machine-dependent parallel wall-clock, not a `SERIAL CONFIRMED OVER BUDGET:` breach; no action taken in this change. Consistent with the standing note that these packages are wall-clock heavy under parallel load.

## Follow-ups

### Optional: extract a shared corpus-snapshot helper (advice only)

The review noted that the `PinContext → ReadCorpus → parseCorpus → BuildSnapshot` corpus-snapshot idiom now appears in three call sites (`resolveClaimTarget` in `change_claim.go`, `observeInProgressIDs`, and the new `gateRecoveredWorktree`), each layering slightly different error mapping. This is not a defect — the pattern is an established in-repo idiom and the per-caller error handling legitimately differs — but a small shared helper (e.g. `snapshotForRepo(ctx, deps, repoDir)`) that each caller layers its own classification onto would reduce drift risk. Deliberately left out of this change: extracting it spans `change_claim.go` and other files, which the spec's Scope excludes as unrelated refactoring. No existing change tracks this; capture deliberately if desired.
