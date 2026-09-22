<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0442 — Rebase again when main advances after finalize publishes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-22-0442-rebase-again-when-main-advances-after-finalize-publishes.md)**
<!-- docket:backlink:end -->

# Rebase again after finalize publishes and the base advances

## Intent and scope

Fix the gap immediately after change 0438: main advances **after** finalize pushes its rebased head but **before** the PR merges. Re-entering finalize must be able to rebase from that proven published result, preserve prior resolutions, and retest. Use the existing effective-base resolver; main is the ordinary example.

This is a bounded extension of the existing refresh, not a new finalize workflow. No dependency or stacked parent. Type fix, medium priority. The user requested a very tight YAGNI design grounded in current implementation and prior decisions.

## Implementation trace and prior decisions

Inspected main at `3ab594194d4ba7887a9384d5f6a0855c7ba0c31c`. The external report described a successful reproduction; this grooming inspected source and did not independently replay it.

- `FinalizeRebase` probes the PR against the supplied head, fetches the base and remote feature head, and routes an existing receipt to `recoverFromReceipt`.
- For a completed rewrite with a changed base, `refreshOwnedRewrite` requires the remote feature head to equal `OrigRemoteHead`. A successful push from A to B makes that condition false without any third-party feature-branch edit.
- `FinalizePublish` calls `workspace.PublishRewrite`. That operation already recognizes the exact intended remote head as published/no-op, otherwise pushing under the old exact lease. Publication leaves the rebase receipt's old lease intact.
- `publishCheckpointOf` exposes the existing tested head/base/command/policy/PR/evidence group. `checkpointDecision` governs test skipping against the live base; it must not be weakened to accept an old base.
- #0438 already refreshes the receipt under the workspace operation lock, preserves resolver accounting, clears old evidence, persists before `BeginRebase`, and forces retesting. Its spec explicitly excluded already-published rewrites.
- #0316 established exact-lease publication and observable-state replay. #0396 / ADR-0105 own live gate continuation; #0408 / ADR-0112 own the completed checkpoint; #0349 / ADR-0113 own durable resolver accounting; #0411 preserves reconciliation of completed resolver work. ADR-0010's resolver/repair boundary and ADR-0118's execution admission remain authoritative.
- Reviewed the ADR and learnings indexes and relevant findings: `groomed-root-cause-is-a-hypothesis`, `moving-base`, `decide-and-act-on-the-same-copy`, and `probe-error-is-not-clean-absence`. Adjacent #0291 is reference-loading work, not a dependency.

The proposed “fresh owned attempt” is refined to **refresh the existing receipt with a fresh rewrite token and a proven new lease**, preserving the consumed budget. The checkpoint plus live equality provides the necessary proof without another persistent publication marker.

## Design

Extend the moved-base classification in `recoverFromReceipt` / `refreshOwnedRewrite` with one additional admissible case. Preserve the unpublished case unchanged.

For the published case require:

1. Existing repo/change/version/manifest/feature/base/PR identity checks pass; the registered workspace is clean, no rebase is in progress, and no gate continuation or unresolved resolver work is superseded.
2. The receipt has a complete checkpoint whose head equals the current local head, authoritative remote feature head, and matching open PR head. Its base equals the receipt's recorded base, its PR number matches, and its evidence re-verifies green for that head with consistent recorded command/policy. Missing or inconsistent proof retains the refusal.
3. The new effective base descends from the recorded base. Preserve carried-descendant proofs before and after the rewrite.

This checks a **historical tested result plus its current publication**, not permission to skip a new test run. A changed current test command cannot reuse old evidence; the new gate uses the current resolved configuration. Do not call `checkpointDecision` with a substituted old base to obtain a skip.

Under the existing workspace operation lock, reload and compare the exact receipt; re-probe the local, remote, and PR facts used to admit this additional case before replacing its lease. Refuse changed or unknown facts without mutation. Remote changes after admission remain protected by the later exact-lease push.

Reuse the existing refresh write: set `OrigHead` and `OrigRemoteHead` to the proven published B, advance `BaseHead`, mint the fresh `Attempt`, and clear the old checkpoint. Preserve resolver limit/usage and existing settled-work requirements. Persist before rebasing from B; do not delete the receipt, reset to A, or replenish budgets.

Use the established conflict, continuation, gate, publication, and merge paths. Require the full configured finalize suite on the refreshed result, including a mechanically unchanged rebase; old evidence/token cannot authorize it. Existing durable-before-Git and completed-checkpoint replay handles interruption after the refresh without repeating the transition.

Document that post-publication re-entry obtains the current version and **published head B** from `context.finalize`; replaying the original head A would correctly fail the PR-head check. Once the new attempt is waiting, repeat its identical invocation as usual. Reuse existing dispositions; do not add an automatic retry loop or change merge admission.

## Acceptance and validation

Extend existing real-Git finalize fixtures with controlled GitHub/gate seams:

- Complete a conflict-resolved rebase A→B, pass the gate, publish B through the existing publication seam, advance main, and re-enter with B. Assert preserved resolution, new base ancestry, fresh token, lease B, unchanged budget, and a fresh suite.
- Publish the next result under exactly B; an intervening remote edit must refuse without overwriting it. Old attempt/evidence must not authorize the new result.
- Cover response loss after publication, including before PR evidence update: the owned checkpoint and live PR head suffice; stale PR-body evidence never skips retesting.
- Reject missing/mismatched checkpoint, local/remote/PR disagreement, foreign or dirty state, divergent base, unresolved work, and failed probes. Assert retained receipt/work/remote.
- Cover concurrent admission and interruption after refreshed receipt persistence, then waiting/completed-gate replay. Assert one refreshed attempt and no duplicate gate on valid replay.
- Keep #0438's unpublished recovery and unchanged-base publication replay green.

Mutation-test new admission guards. Run the whole source-entered suite resolved from `build.test_command` at build time and inspect budget findings. Update the maintained finalize guidance and regenerate embedded assets. Grooming writes no product code and runs no suite.

## Alternatives and exclusions

Deleting the receipt and taking the fresh path discards accounting and interruption authority. Blindly accepting any moved remote weakens overwrite protection. A new publication field/store is unnecessary for this proven checkpoint-backed case.

Exclude checkpoint-less or ambiguous published heads, adoption of later arbitrary commits, force-rewritten bases, rollback, new commands/schema/configuration, retry or budget policy, merge-race detection changes, permission changes, unrelated cleanup, and implementation planning. Existing ADR decisions remain intact; no new ADR is needed.
