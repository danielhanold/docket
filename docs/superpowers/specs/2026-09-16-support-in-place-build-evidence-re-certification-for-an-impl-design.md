<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0415 — Support in-place build-evidence re-certification for an implemented change](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0415-support-in-place-build-evidence-re-certification-for-an-impl.md)**
<!-- docket:backlink:end -->

# In-place build-evidence recertification

## Goal and boundary

For an implemented change whose open PR received a follow-up commit, provide one supported command that reruns its build gate and refreshes the PR's build-evidence block at that head. The change remains implemented throughout.

## Design

Add `evidence.recertify`, exposed as `docket evidence recertify --id <id> [--repo-dir <dir>]`, with normal human/JSON output and capability/schema registration. This is a small app-layer composition of existing workspace, gate-drive, evidence, and GitHub services. It does not re-enter implement-next or invoke finalize.

1. Resolve the change, its existing feature worktree, and its recorded open PR. Require implemented status, a clean worktree on the recorded branch, and agreement between local HEAD, the remote feature head, and the PR head. Refuse missing, ambiguous, or mismatched state before starting; an unpushed follow-up must be published first through the existing workflow.
2. Run the configured `build.test_command` in that worktree using the existing build-owned gate driver. Advance its WAITING slices within this operation until terminal; WAITING never means success or starts a second suite. Preserve existing admission, ownership, cancellation, deadlines, and diagnostic retention. Run one suite attempt; report failure or halt without an automatic code-repair loop.
3. For PASSED, obtain canonical evidence through `EvidenceRecord` using the actual terminal run directory and tested head, then reparse and verify it. Honor `build.gate: off` with the existing skipped evidence behavior, explicitly reported as skipped. An enabled gate with no configured command refuses; there is no finalize-command fallback.
4. Recheck the tested head, clean worktree, build configuration, implemented status, and PR identity before publishing. A changed head or command cannot inherit the earlier pass. Read the current PR body and replace only its evidence block with `evidence.Upsert`, using the same expected-head/version edit behavior as `FinalizePublish`. Preserve all other body bytes, title, base, and PR number.
5. Report completion only after the remote PR update is confirmed and its stored evidence verifies against its current head. Include the change, head, PR, and green/skipped outcome. Existing unrelated `run.verify` findings remain visible.

A failed gate, interruption, malformed evidence, identity drift, or failed/uncertain PR edit reports the appropriate existing failure category, never completed recertification. Do not overwrite the PR block on a gate failure. An explicit retry may rerun the gate; it still edits the same PR. Reuse existing gate ownership protections if prior work remains live.

## Implementation limits

Keep new code to the entry point and necessary composition glue; extract a small shared helper only where required for reuse. In particular, reuse finalize's evidence-block edit behavior without its rebase receipt, branch push, or merge path. Document this command in the existing evidence/review-follow-up guidance.

No new subsystem, agent, lifecycle status, configuration setting, evidence store, cache, recovery journal, or retry framework. No branch creation, commits, pushes, rebases, merges, results rewriting, or automatic fixes. Preserve evidence staleness rules, evidence rendering, finalize behavior, and the deferred results-only-delta optimization.

## Acceptance

- An implemented change with a published follow-up goes from stale evidence to verified evidence for that exact head; the same PR stays open and the change stays implemented.
- Differing build/finalize commands prove only the build command runs. Gate-off records skipped; missing build configuration refuses.
- Gate failure/halt, dirty or moved HEAD, changed configuration, non-implemented status, and closed/mismatched PR cannot publish successful evidence.
- A PR edit failure is not completion; a later retry updates the same PR and preserves authored content.
- Exercise these cases through the operation with existing test seams. Run the full configured build suite at implementation's build gate and review its budget report.
