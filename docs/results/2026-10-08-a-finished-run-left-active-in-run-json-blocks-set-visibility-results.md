<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0540 — A finished run left active in run.json blocks set-visibility and cannot be cancelled](../changes/active/0540-a-finished-run-left-active-in-run-json-blocks-set-visibility.md)**
<!-- docket:backlink:end -->

# A finished run left active in run.json blocks set-visibility and cannot be cancelled — Results

**Human action:** After this merges and the binary is reinstalled, clear the runs already stranded on this machine by re-running their keyed verdict (Important item below). Nothing else is required.

## Outcome

Before this change, a dispatched implement-next run that ended with any verdict other than `run-complete` left its `run.json` at `active`. `docket repository set-visibility` then refused with "a run or gate is still live" and told you to run `docket run cancel`, which refuses a run that never claimed anything. The only way out was editing `run.json` by hand.

Now:

- Every keyed `run-done` verdict (`run-complete`, `no-attributable-claim`, `run-unclaimed`) retires the run through the existing closeout, so `run.json` ends `completed`. The report line you see is unchanged. If the closeout finds something still live, the verdict reports `run-stop <key> run-tracker-unavailable completion-unaccounted`; repeat the same verdict once it settles.
- Halted, stop, retry and continue verdicts behave as before. `run verdict --unattributed` stays read-only. A cancelling, cancelled or superseded run is never relabelled.
- `set-visibility`'s live-run list names a command that works: the cancel command when `run cancel` would accept the run, "once its dispatch has returned, run `docket run verdict <key>`" when it would refuse, and the by-hand note for a `cancelling` run with no claim.
- `run start --resume <id>` no longer treats a run retired by a non-`run-complete` verdict as the change's finished run, so a change whose earlier run ended `run-unclaimed` can still be resumed later.
- ADR-0148 records the decision (extends ADR-0124 Rule 1).

## Human actions and testing

### Important — clear runs stranded before this fix

Runs that finished before this fix still have `run.json` at `active` and will keep blocking `set-visibility` until their verdict is re-run with the new binary.

Prerequisites: the change is merged and the `docket` binary reinstalled from `main`.

1. List candidate runs: `grep -l '"state": *"active"' <state dir>/run-tracker/*/run.json` (the state dir is the one `docket diagnostic runtime` reports). For each hit, check that `record.json` beside it has `"terminal": true`. Known cases at grooming: `implement-next-20260929t153306z-56752-95e7`, `implement-next-20261004t222959z-96475-7c95`, `implement-next-20261007t104957z-97517-71ff`; check they are still `active` first.
   Expected: a short list of keys whose dispatches are long finished.
2. For each key, run `docket run verdict <key>`.
   Expected: `run-done <key> no-attributable-claim` (or `run-unclaimed`), and `run.json` now reads `completed`. A run with no verdict yet (such as the `gate-armed` one from 2026-09-29) gets its first verdict the same way.
3. Run `docket repository set-visibility` dry/preview or re-run the command that was refused before.
   Expected: no "a run or gate is still live" refusal naming those keys.

## Verification performed

- Full suite green at the final head 75102ddf6 (build gate, run twice: after the build and after the review fixes). The budget report had only BUDGET WATCH / PARALLEL-SENSITIVE screening lines, no SERIAL CONFIRMED OVER BUDGET line.
- Each build and fix task wrote a failing test first and mutation-tested its guard (closeout routing for each run-done outcome, remedy selection, the resume lookup).
- Whole-branch review (deep tier): 4 findings (2 important, 2 minor), all fixed in-branch; the important resume dead-end and the remedy-could-end-a-live-run finding each got a code fix and tests, one minor was a comment fix, and the missing ADR was recorded as ADR-0148. Full table in the PR body.

## Known issues and follow-ups

- **A halted change can still bounce between two refusing commands.** When a run has a confirmed claim but lacks cancel authority (no parent capability or a claim mismatch) and its change is halted or incomplete, `set-visibility` suggests the keyed verdict, the verdict returns `run-stop` and leaves the run `active`, and `run cancel` refuses it. Production `run start` always sets the capability, so this is suspected to occur only with hand-built or legacy records. Workaround: settle such a run by hand. This is an in-scope limitation (the spec kept halted and stop verdicts unchanged); the code comment on `liveRunsUnder` states it.
