<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0446 — Orphaned halted gate drive blocks every new worktree's first gate admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0446-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md)**
<!-- docket:backlink:end -->
# Orphaned halted gate drive blocks every new worktree's first gate admission — Results

**Human action:** After this PR is merged and the `docket` binary is reinstalled, one Important check is needed: run change 444's existing recovery workflow while keeping its uncommitted work, to confirm the real-world blocker is gone. Nothing else needs a human before merge beyond the usual review of the diff.

## Outcome

Before this change, a single leftover gate record could stop unrelated work. Docket keeps a record of every test-gate run ("drive") and, for each worktree, a "slot" saying who is currently running tests there. A fresh worktree's first gate start scanned every drive record in the repository. If any old record could not be matched or read, for example change 368's HALTED drive whose worktree had been deleted, the start was refused. Cancellation, resume and successful closeout had the same kind of problem: they treated history they could not explain as work the current run still owed.

Docket now treats historical records as diagnostics unless they are positively tied to the worktree or run being acted on. That tie has to come from the current slot, scope, epoch, reservation token or participant link. Unrelated, unreadable or obsolete records no longer block anything, and `gate history cleanup` still reports them. A missing or corrupt record that the target's own current bookkeeping names still refuses, with an exact locator.

Other behavior changes:

- **Finished incumbent settled at admission.** A new start can now clear a slot left behind by a run that is proven finished: a PASSED or FAILED drive, or a raw run with positive teardown proof. It uses the existing exact-token compare-and-swap. HALTED alone is never accepted as proof.
- **Settled epoch released.** A released slot whose recorded run epoch is settled (completed, cancelled or superseded) is retired at admission instead of refusing with `stale-run-epoch`.
- **Removed worktrees stay addressable.** Slots are looked up by their stored identity after the worktree directory is deleted, so cancel and retire still reach them.
- **Obligations attributed to the run.** The epoch launch census counts only obligations tied to that epoch through current references. It also now detects a missing drive that a scope still names, unless existing records prove the reservation was withdrawn.
- **Owner selection.** The worktree owner is chosen deterministically: an active owner outranks fenced predecessors, two active owners give a typed refusal, and a slot-named epoch that cannot be resolved (or an unreadable slot) fails closed.
- **Durable completion facts.** Closeout reads durable facts (a released slot, or a PASSED/FAILED drive record) when the scratch run directory has already been cleaned up.
- **Release errors reported.** Release failures now appear as `release_finding`. A run root is kept, with a reason, whenever its release is unsettled.
- **Torn resume converges.** A resume that was interrupted between superseding the old epoch and minting or binding the new one no longer dead-ends. It falls back to the stored worktree identity.

Decision record: ADR-0125 (historical gate discovery has no global veto). It replaces only the conflicting clauses of ADR-0118 (worktree-wide inventory scope, raw release only on explicit stop) and ADR-0120. Those ADRs stay Accepted for their other guarantees.

Departures from the plan: several internal interfaces differ from the plan's sketches. For example, reconciliation takes the run epoch, the app layer uses exported reconcile methods, and the seam that decides whether an epoch is settled treats superseded as settled. The ADR was recorded through `docket-adr` on the metadata branch, not as a build task.

## Human actions and testing

### Important — verify change 444's recovery with the installed build

Change 444's cancellation failed on exactly this defect. Automated tests reproduce its conditions only with fixtures, including a frozen copy of the quarantined change-368 record. The real repository's history is the one environment they do not cover. If you skip this, a still-unknown shape of real historical record could block 444.

Prerequisites: this PR merged to `main`, and the binary reinstalled from `main` following the repository's "Rebuild the binary after a merge" rule.

1. Run `docket run verify --id 444`.
   Expected: a closed verdict (for example `run-halted` or `run-incomplete`), not a crash.
2. Follow 444's existing recovery workflow: arm with `docket run gate-before implement-next --resume 444`, and cancel any live epoch it names first if the arm tells you to.
   Expected: no refusal that names an unrelated drive (such as the change-368 record) or `inventory-legacy-drives`.
3. Check 444's worktree with `git -C <444 worktree> status`.
   Expected: its uncommitted work is still present.

### Optional — watch an unrelated broken record stay harmless

1. In a scratch clone, start and finish a gate in worktree A, then delete worktree A's directory.
2. Start a gate in a fresh worktree B with `docket gate drive start --owner build …`.
   Expected: B is admitted.
3. Run `docket gate history cleanup`.
   Expected: A's record is still listed as retained.

Cleanup: delete the scratch clone.

## Verification performed

- Every plan task and review fix was built test-first through the native gate driver, with focused RED and GREEN runs and mutation checks that were each seen to fail.
- The whole-suite build gate went red once. The cause was a CLI test that assumed a finished first run still holds the slot, which is no longer true by design. That test was repaired (fb7744eb), and the suite then passed at that head: 53 of 53 files, with BUDGET WATCH screening lines only and no serial over-budget breach.
- A deep whole-branch review returned 7 findings: 0 blocker, 4 important, 3 minor. All 7 were fixed in-branch (53910162, 0cbd0d9c, 7bc62a27, d7d7c1bf, b30c40b7).
- The final certification suite run on the head that contains this file is recorded in the PR's build-evidence block.

## Known issues and follow-ups

### Registry read failure still refuses first admission

If the drive registry directory itself cannot be listed (a permission or IO error, not "does not exist"), a first admission still refuses with `inventory-legacy-drives`. The spec leans toward treating this as diagnostic. It is kept fail-closed on purpose and should be decided in a follow-up.

### Epoch records pruned while a slot still names them

If an epoch record is pruned after its retention window while a slot still names it (for example after a failed closeout), mutations and gate starts on that worktree now fail closed with `epoch-owner-unresolved`. The remedy message tells a human to inspect the epoch record under the rungate store. There is no automated repair.

### Unreadable replacement chain on resume

A resume whose replacement epoch record is corrupt, or whose supersession chain loops, still refuses (`replacement-epoch-unreadable`, `replacement-chain-cycle`). The record has to be made readable again by hand.

### Stale comment in finalize e2e test

`finalize_e2e_test` still says a raw launch holds its slot until an explicit stop. After this change that is no longer strictly true. The comment is harmless, and cleaning it up is a small follow-up.
