<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0375 — `docket gate drive start` is not idempotent — a re-run spawns a second concurrent drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-14-0375-gate-drive-start-not-idempotent-spawns-concurrent-drive.md)**
<!-- docket:backlink:end -->
# `docket gate drive start` is not idempotent — a re-run spawns a second concurrent drive — Results

## Outcome

Worktree-wide gate admission and an explicit, honest human-cancellation path are now in
place. One canonical worktree carries at most one reserved-or-running gate execution: a durable
worktree execution slot is reserved *before* launch and released only on reconciled
terminal-verdict-plus-teardown evidence (a driven PASSED/FAILED terminal, or a proven raw
teardown via stop). Admission keys on the canonical worktree path (symlink aliases collapse to
one slot) and is independent per distinct worktree of the same repo. Scoped starts, scopeless
starts (finalize's local gate), automatic advance-relaunches, and raw `gate.launch` all admit
through the same slot; a second execution on the same worktree fails closed with a stable,
credential-free diagnostic (`worktree-busy` / `unresolved-execution` / `stale-run-epoch`) that
names its next action.

A run epoch, bound to the run-gate record, fences the worktree so only that epoch's own
sequential drives may re-admit, and fences new workflow mutations (the transaction engine's
admission hook, PR-publish, and workspace-publish) once the epoch is cancelling/cancelled.
Human Stop is now an explicit cataloged `run.cancel` (keyed by run-gate key + epoch) that
durably fences the epoch before shutdown and reports success only on reconciled teardown
(`cancelled` / `cancellation-pending` / `already-cancelled` / `refused`); cancelling charges no
suite attempt and resets no budget. Resume refuses to start a second run over an unstopped one
and supersedes a confirmed-cancelled epoch atomically for exactly one replacement. Codex
`agent.enter` registers the run lifecycle (participant registration + a death guardian for
uncatchable owner death), and adapters with no cancellation/death callback honestly report
`owner-lifecycle-unavailable` and name `run.cancel` as the remedy. The decision is recorded in
ADR-0118 (extends ADR-0107; preserves ADR-0115/0116 budget semantics).

Departures from the plan: none in scope. The change was completed across the planned Tasks
1–17; Task 17's ADR is ADR-0118.

## Verification performed

- Full suite (the resolved `build.test_command`, `go run ./cmd/docket development test`) driven
  to green from source via the native gate driver at the final head
  `6401e3fd4919c5a48fe936e0a3a46aff4a846d2b`: 46/46 files, 0 failed. Build evidence recorded and
  re-verified against that head.
- The budget report carried only `BUDGET WATCH` screening lines (parallel wall-clock, streak
  1/5, machine-dependent); no `SERIAL CONFIRMED OVER BUDGET` breach, so no serial confirmation
  was required.
- Deep whole-branch review (docket-review-deep) returned one blocker, two important, and two
  minor findings — all this-branch defects, all fixed in-branch (see the PR body's disposition
  table). Each fix landed under the docket-build-task contract with its own focused,
  mutation-or-RED/GREEN-verified test where behavioral:
  - Fresh-run epoch worktree binding (blocker): a test drives the real
    `RunGateBefore`(fresh)→`ReserveGateClaim`→`ConfirmGateClaim` path and asserts a post-cancel
    workflow mutation is refused `run-cancelled` (RED with the bind disabled).
  - Scope run-epoch population (important): a test drives the production `prepare-scope
    --run-epoch` → `takeover`-against-cancelled-epoch path and asserts the takeover HALTs rather
    than reviving the epoch; the paired no-epoch control proves the refusal is epoch-gated.
  - Arm surfaces the epoch id (important): a test asserts the fresh arm's result and human line
    carry the minted epoch id that `run.cancel --epoch` / `--run-epoch` consume.
  - Guardian stale-marker clear (minor) and the `releaseAdmissionIfProven` doc comment (minor).
- A separate concurrency defect surfaced by the fix-loop's own `-race` suite run (not a review
  finding) was fixed: the first-admission legacy census failed closed on a record-less in-flight
  drive directory, spuriously refusing a concurrent gate on a *different* worktree of the same
  repo. It now skips a record-less directory (a present-but-corrupt record still blocks), with a
  mutation-verified `TestLegacyRecordlessDirDoesNotBlock`.

## Findings and limitations

### Verdict-path recovery does not re-bind the epoch worktree

The two verdict-path recovery `ConfirmGateClaim` calls (`rungate_verdict.go`) pass an empty
worktree, so a run recovered *solely* via the verdict path (its local mirror lost) would not
re-bind the epoch worktree. This is the rare recovery path, not the common first-dispatch case
the blocker named; the fresh-dispatch path is fully bound and fenced. Noted as a follow-up
below.

### Native turn cancellation is best-effort where the adapter exposes none

`run.cancel`'s native task cancellation is wired only where an adapter exposes an interruption
callback; adapters without one honestly report `owner-lifecycle-unavailable` and name
`run.cancel` as the remedy rather than claiming universal UI-Stop integration. This is the
intended, spec'd limitation, not a defect.

### Always-loaded dispatch-block budget re-baselined

The always-loaded AGENTS.md dispatch block gained the human Stop/cancel and resume-after-stop
operator contract (the change's headline capability). Its word budget was re-baselined from 650
to 1137, kept strictly below the pre-0334 retired-roster ceiling of 1156 (margin 19). The
anti-regrowth invariant still holds and reddens on any further growth; the next change that
grows this always-loaded surface will have to make a conscious decision against that ceiling.

## Follow-ups

### Re-bind the epoch worktree on the verdict-path recovery claim

`rungate_verdict.go`'s two `ConfirmGateClaim` recovery calls pass an empty worktree, so a
verdict-path-only recovery leaves the epoch worktree unbound. Re-derive the feature worktree
there (the branch is known on the recovered record) so the mutation fence and `run.cancel`
teardown remain active after a mirror-loss recovery. Out of scope here because the common
first-dispatch path is fully covered; captured for a human to file with `docket change create`.
