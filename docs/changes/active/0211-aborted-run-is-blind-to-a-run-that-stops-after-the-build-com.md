---
id: 211
slug: aborted-run-is-blind-to-a-run-that-stops-after-the-build-com
title: aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent
status: proposed
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [113]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
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

Add a third, time-free leg to `aborted-run` that keys on **built-but-not-delivered** rather than on
field incoherence. The candidate predicate, scoped like the others to `status: in-progress`:

the feature branch named in `branch:` exists and carries commits ahead of the integration branch,
AND either the branch has no remote tracking ref, or `pr:` is empty.

On the 0206 instance that fires the moment the run stops, with no time horizon at all.

Design questions grooming must settle:

- **The live-run window.** Every healthy run passes through this state for the whole build span —
  commits landing on an unpushed branch with `pr:` empty is what a working build looks like. So the
  leg needs either a branch-idle floor (newest commit older than N minutes) or an explicit acceptance
  that the finding is advisory and self-clearing, the same posture leg A takes for its
  commit-to-field-write race. Note the floor cannot key on `claimed_at` — that is leg B, and the
  heartbeat makes it unusable here.
- **Whether the two disjuncts are one leg or two.** Unpushed-branch and empty-`pr:` are different
  evidence: the first says Step 7 never started, the second says it started and did not finish
  (0194's second stop). They may deserve separate messages.
- **Cost.** The leg needs a remote-tracking-ref probe per in-progress change; `board-checks.sh` is a
  pure reader run on every `docket-status` and every Board pass, and change 0176 established that
  per-invocation cost in this path is a real constraint.

Every predicate must be mutation-tested, per 0113's own rule: a completion check that cannot fail is
this defect wearing a badge.

## Out of scope

- Retuning leg B's 12h horizon or the heartbeat rider — both are correct for what they cover.
- Anything that flips status or releases a claim. `board-checks.sh` is a pure reader by contract and
  `aborted-run` is advisory by design; the originating incident left real committed work that a naive
  claim release would have stranded.
- The prose half of the failure — an inlined role skill's terminal "you stop" ending the whole run —
  which is a separate change.
