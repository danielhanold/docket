---
id: 211
slug: aborted-run-is-blind-to-a-run-that-stops-after-the-build-com
title: 'aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent'
status: done
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: [202]
related: [113, 212]
discovered_from: [113]
adrs: []
spec: docs/superpowers/specs/2026-08-05-aborted-run-built-but-not-delivered-leg-design.md
plan: docs/superpowers/plans/2026-08-05-aborted-run-built-but-not-delivered-leg-plan.md
results: docs/results/2026-08-05-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com-results.md
trivial: false
auto_groomable: true
branch: feat/aborted-run-is-blind-to-a-run-that-stops-after-the-build-com
pr: https://github.com/danielhanold/docket/pull/160
blocked_by:
claimed_at: 
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-aborted-run-built-but-not-delivered-leg-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-aborted-run-built-but-not-delivered-leg-design.md) |
| Plan | [2026-08-05-aborted-run-built-but-not-delivered-leg-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-05-aborted-run-built-but-not-delivered-leg-plan.md) |
| Results | [2026-08-05-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-05-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com-results.md) |
<!-- docket:artifacts:end -->

## Why

Change 0113 shipped the `aborted-run` check as the external, mechanical oracle for an autonomous run
that stops mid-step. On 2026-08-05 a `docket-implement-next` fork building change 0206 stopped at the
**Step 5/6 boundary**, and both of `aborted-run`'s legs were silent — by construction, not by
accident.

Verified on-disk state when the fork returned:

- `feat/delegated-runner-runs-are-anchored-at-the-main-worktree-not` at `7c7d7d02` with **four real
  build commits**, clean tree, the 78-file suite green
- the branch had **no remote tracking ref** — never pushed
- manifest: `status: in-progress`, `plan:` SET, `pr:` empty, `results:` empty
- no review had run; Steps 6, 6.5 and 7 never started

Why each leg missed:

- **Leg A (manifest/git incoherence)** fires on a committed plan with `plan:` unset, or a committed
  results file with `results:` unset. Here `plan:` was correctly recorded at `5e5750e6` and no results
  file existed yet. **Every field the leg inspects was coherent.** The run dropped no bookkeeping
  write — it dropped two whole steps.
- **Leg B (12h run-scale stale claim)** needs `claimed_at` older than `ABORTED_RUN_STALE_SECS`. The
  claim had been re-stamped at the `plan:` write 34 minutes earlier, by 0113's own heartbeat rider.
  The rider is right on its merits, but it means a run that dies immediately after a metadata commit
  starts its countdown from the freshest possible stamp, so leg B is at its blindest exactly when a
  run has just finished a step.

Leg B would have fired ~12h later, advisory, and only on the next `docket-status`. The human caught
it in minutes by reading `git log` and the manifest — which is the detection 0113 exists to
mechanize.

This is the fourth observed stop and the third distinct signature (0109 and 0194's first stop at the
Step 4/5 seam with nothing built; 0194's second stop at the end of Step 7 with the PR open and the
manifest unwritten; this one at Step 5/6 with the build complete and unpushed). 0113 was designed
against the first three, where a step's work and its bookkeeping were **separated**. This one
separated nothing, so an incoherence oracle has nothing to see.

## What changes

Add a third leg to `aborted-run` that keys on **built-but-not-delivered** rather than on field
incoherence. Scoped like the others to `status: in-progress`: the feature branch named in `branch:`
exists and carries commits ahead of the integration branch, `pr:` is empty, and the branch has gone
quiet — with the message naming whether the branch was never pushed, or was pushed with no PR
recorded.

On the 0206 instance that fires roughly 2h after the run stops, against leg B's 12h.

Settled by the linked spec (auto-groom, 2026-08-05):

- **The live-run window** gets a **branch-idle floor** — the branch's newest commit older than a
  hardcoded 2h — not the advisory/self-clearing posture leg A takes. Leg A's false-positive window is
  seconds; leg C's would be the entire build span, on a check that runs on every Board pass including
  those inside the run being built. The floor keys on the commit timestamp, never on `claimed_at`.
- **One leg, two messages.** The expensive probe is shared and the two disjuncts are mutually
  exclusive once ordered, so a single emit site branches the message on whether the branch was ever
  pushed. No new check-id.
- **Cost** is gated by the free frontmatter read first: a non-empty `pr:` skips leg C with zero git
  calls; a non-firing path costs at most three, the firing path five.
- The ahead-of-integration comparison excludes **both** the local integration ref and its
  remote-tracking twin — feature branches are cut from `origin/<integration>`, and a lagging local
  ref would otherwise make a nothing-built branch look arbitrarily far ahead with arbitrarily old
  commits, firing leg C on the 0109 signature.

Every predicate is mutation-tested, per 0113's own rule: a completion check that cannot fail is this
defect wearing a badge. The existing ARM fixtures are silent for leg C only because the suite's
`NOW_EPOCH` predates their wall-clock commit dates — the build must neutralize that skew rather than
inherit it.

## Out of scope

- Retuning leg B's 12h horizon or the heartbeat rider — both are correct for what they cover.
- Anything that flips status or releases a claim. `board-checks.sh` is a pure reader by contract and
  `aborted-run` is advisory by design; the originating incident left real committed work that a naive
  claim release would have stranded.
- The prose half of the failure — an inlined role skill's terminal "you stop" ending the whole run —
  which is a separate change (0212).
- A sixth signature: the PR opened and `pr:` written, then the run dies before `status: implemented`.
  Leg C's `pr:`-empty gate makes it invisible; leg B catches it at 12h. Its evidence is a
  manifest/GitHub comparison, and `board-checks.sh` is git-only by contract.

## Reconcile log

### 2026-08-05

Re-read against `origin/docket`, `origin/main`, `related: [113, 212]`, and current code. The change
stands as written; scope unchanged.

- **`depends_on: [202]` is satisfied.** 0202 reached `done` and is archived; `branch_only_artifact`
  now carries its NUL-delimited `ls-tree` rewrite, and leg C is therefore being added beside hardened
  predicates as assumption 9 intended.
- **Fixture numbering confirmed.** `tests/test_board_checks.sh` ends its ARM series at **226** (0202
  landed 224, 225, 226), so the spec's assumption 8 holds verbatim: leg-C fixtures start at **227**.
- **The target block is unchanged since the spec was written.** `scripts/board-checks.sh` still has
  legs A and B inside one `if [ "$status" = "in-progress" ]`, `branch_ref` still probes local-then-
  `origin`, `ABORTED_RUN_STALE_SECS` still sits at line ~172 beside `FINALIZE_BLOCKED_STALE_SECS`.
  Every code claim in the spec's *Predicate* and *Cost* sections re-verified against the file.
- **No collision with the two other in-progress changes.** 0212 (the prose half) edits skill bodies,
  and 0190 edits the build-evidence path; neither touches `board-checks.sh` or its test file.
- The spec's `## Assumptions` remain the audit trail; nothing in them was invalidated.
