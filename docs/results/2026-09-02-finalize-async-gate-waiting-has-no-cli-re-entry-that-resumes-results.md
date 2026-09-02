<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0396 — finalize async gate WAITING has no CLI re-entry that resumes the same drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0396-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md)**
<!-- docket:backlink:end -->
# Finalize gate WAITING re-entry via the owned rebase receipt — results

Change: #0396 · Branch: fix/finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes · Plan: docs/superpowers/plans/2026-09-02-finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes.md · ADRs: 105

## Verify (human)

- No genuinely manual merge-gate checks. The full suite is green and covers the behavior (seam-level WAITING/resume/terminal-clear tests, the CLI projection + attempt round-trip, and the mutation-tested prose guard). Merge on green CI.

## Findings

- **§6 attempt-token-truncation claim did NOT reproduce.** The stub asserted `finalize.rebase` JSON truncates the attempt token to an 8-char base suffix while the receipt holds 12, causing an `attempt-token-mismatch` on continue. `newRebaseAttempt` mints `<stamp>-<baseHead[:12]>` (a full 12-hex base suffix), and `TestIntegrationFinalizeRebaseAttemptRoundTrip` — which marshals the app result through the exact `json.Marshal(r)` the CLI presenter uses and asserts the document `attempt` equals the on-disk receipt `attempt` byte-for-byte — passed on its first run and never reddened. Nothing was fixed; the test stands as the standing guard. The 0364 mismatch was most likely an operator mis-copy of the token. (learnings: groomed-root-cause-is-a-hypothesis.)
- **ADR-0105** records the decision: finalize's local-gate continuation (drive id + owner generation) is persisted in the owned `rebase-receipt.json` and never carried by the caller; the WAITING re-entry is the identical `finalize.rebase` invocation; the owner generation is receipt-private (`json:"-"`, never in any CLI document). Refines ADR-0098 for finalize.
- **Two minor review findings fixed in-branch** (commit 24b96ce7): a dead `_ = want` test assertion was replaced with a real non-pair-field byte-identity cross-check of the persisted receipt; and the `deps.Gate == nil` refusal exit in `composeLocalGate` now routes through `clearGateContinuation` like every other terminal, closing the one gap in the presence-encoded-state clear thesis.
- **Build-gate environmental flake (not a defect).** A full-suite gate run reddened once on `TestRecoverLeavesUnprovableGroupForInspection` in `internal/process` — a package this change does not touch. It was caused by cross-contamination from concurrent gate-drive supervisors this run spawned under `$TMPDIR` interfering with that package's process-registry recover-scan. Serially confirmed clean: the exact test and the whole `internal/process` package both pass with `-count=1`, and a subsequent clean full-suite gate at the same head passed all 39 files green. Recorded here for transparency; the branch is genuinely green.

## Follow-ups

- **The repair path's post-repair re-gate uses raw `gate.launch`/`gate.observe` instead of the driver** (`docket-finalize-change` step 5), at odds with ADR-0098's "executable workflows compose the driver layer, never the raw gate primitives." Explicitly out of scope for 0396 (spec Non-goals) and reported for deliberate capture as its own change (`docket change create`).
- Change #0375 (`gate drive start` idempotency) is a sibling this design sidesteps for finalize (never calls `Start` while a receipt-recorded drive exists) rather than depends on; no coupling introduced.
