<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0540 — A finished run left active in run.json blocks set-visibility and cannot be cancelled](../../changes/active/0540-a-finished-run-left-active-in-run-json-blocks-set-visibility.md)**
<!-- docket:backlink:end -->

# A keyed run-done verdict retires its run: design

Change #540, groomed interactively on 2026-10-07. Related: #541 (the child claims without `--run-context`, the most common cause of the symptom fixed here), #345 (the command-launched attribution gap), #532 (`set-visibility`, whose live-run scan surfaced it). ADRs: 0124 (successful-run closeout), 0128 (run.cancel ownership proofs).

## Summary

Every dispatched implement-next run leaves two files under `<state dir>/run-tracker/<key>/`: `record.json` (the run tracker's verdict and retry accounting) and `run.json` (the run's lifecycle state, which decides whether the run is live). Today only a keyed `run-done … run-complete` verdict moves `run.json` out of `active`. Every other terminal verdict marks `record.json` `terminal: true` and leaves `run.json` `active`. For a run that claimed nothing, that state can never be cleared: `docket run cancel` refuses it (`claim-unconfirmed`), and `docket repository set-visibility` refuses while it is live and names `run cancel` as the remedy.

This change makes the keyed verdict retire the run on **every** `run-done` outcome (`run-complete`, `no-attributable-claim`, `run-unclaimed`) through the existing observation-only closeout, so `run.json` ends `completed`. It also makes the live-run scan name a remedy that will work: `docket run verdict <key>` when `run cancel` would refuse.

## Evidence gathered at grooming

- **Only run-complete reaches the closeout.** In `RunVerdict` (`internal/app/runtracker_verdict.go`), `case VerdictRunComplete` calls `runTrackerCompleteRun`, which drives `completeSuccessfulRun` (`runtracker_complete.go`): success fence `active→completing`, settle retry-proven publications, stop-free accounting of participants, the launch census by the run's context hash and the mutation journal, then CAS `completing→completed`. The other outcomes call `persistRunVerdict` or `runTrackerOwnershipDone`, which write only `record.json` (`Disposition`, `Terminal`, a best-effort mirror).
- **`no-attributable-claim` is decided before `RunVerify`.** `resolveRunTrackerOwnership` returns `runTrackerOwnershipDone` when (a) an unconfirmed reservation has no committed receipt, or (b) there is no binding file and zero committed claim proofs match the record's `ChildContextHash`.
- **Cancel refuses exactly these runs, on purpose.** `runCancel` (`runtracker_cancel.go`) accepts a confirmed claim binding or the resume-verified shape (`RunTrackerRecord.resumeAttributed`); anything else is `claim-unconfirmed`. ADR-0128 Decision 1 keeps "an unconfirmed reservation still refuses claim-unconfirmed".
- **The repository-wide scan reads only `run.json`.** `liveRunsUnder` (`runtracker_live.go`) treats `active`, `completing`, `cancelling`, and unknown states as live and hard-codes `runCancelCommand(key)` as the remedy for `active`/`completing`. Its only caller is `visibilityLiveRunLines` in `repository_set_visibility_probe.go`.
- **Resume depends on halted runs staying `active`.** `run start --resume` (`runtracker_start.go`) answers a prior `RunCompleted` run with "there is nothing to resume" and supersedes only a `RunCancelled` one, so the halt → cancel → resume flow needs a halted run to remain `active` until it is cancelled.
- **It is not rare.** At grooming the development machine held three `active` runs with no claim binding and no participants: two `run-done … no-attributable-claim` runs (`implement-next-20261004t222959z-96475-7c95`, started 2026-10-04 22:29Z, and `implement-next-20261007t104957z-97517-71ff`, started 2026-10-07 10:49Z) and one `gate-armed` run that never got a verdict (`implement-next-20260929t153306z-56752-95e7`, started 2026-09-29). A private test repository had a fourth, hand-edited to `completed`. Two of these runs did claim and build a change but claimed without `--run-context` (#541), so the attribution failure is common, and every one of them strands an `active` `run.json` today.

## Design

### 1. The keyed verdict retires every run-done run

`RunVerdict` routes every keyed `run-done` outcome through the closeout that `run-complete` already uses:

| Verdict | Today | After |
|---|---|---|
| `run-done <key> run-complete <id>` | closeout → `completed` | unchanged |
| `run-done <key> no-attributable-claim` | `record.json` only; `run.json` stays `active` | closeout → `completed` |
| `run-done <key> run-unclaimed <id>` | `record.json` only; `run.json` stays `active` | closeout → `completed` |
| `run-stop … run-halted`, `run-stop … run-incomplete`, every `run-stop … run-tracker-unavailable` ownership/load failure | `run.json` stays `active` | unchanged |
| `run-retry-once`, `run-continue` | nonterminal | unchanged |

- Generalize `runTrackerCompleteRun` to carry the outcome token (and the attributed id, `0` for `no-attributable-claim`) instead of hard-coding `VerdictRunComplete`. `runTrackerOwnershipDone` and the `VerdictRunUnclaimed` case call it rather than `persistRunVerdict`.
- **Report lines are unchanged on success.** A successful closeout reports exactly today's line (`run-done <key> no-attributable-claim`, `run-done <key> run-unclaimed <id>`), with `CompletionFindings` as for `run-complete`.
- **A blocked closeout fails closed through the existing channel.** If accounting finds a live, pending, or unreadable obligation, the verdict reports `run-stop <key> run-tracker-unavailable completion-unaccounted` (or the engine's other bounded reason tokens) and the run stays `completing`. Repeating the same keyed verdict once the obligation settles replays the closeout to `completed`. No retry is consumed. `run-stop` and `run-done` both forbid re-dispatch, so the parent's obligations do not change.
- **Never relabel.** A run already `cancelling`, `cancelled`, or `superseded` is never marked `completed`. The closeout's existing refusal applies (`run-cancelled` / `run-superseded` reason tokens).
- **No run beside the record** (keyless or legacy): keep today's behavior — mirror the report line, write nothing else.
- **Unattributed observe mode** (`run verdict --unattributed`) stays read-only and never reaches the closeout.
- No new run state, no schema change, no new field. `completed` means "retired by its keyed verdict with full accounting". Whether the implementation succeeded remains `RunVerify`'s verdict, which the report line carries.

### 2. The live-run remedy names a command that works

`liveRunsUnder` chooses each run's remedy with the same ownership predicate `runCancel` uses (confirmed claim binding, or `resumeAttributed`), read from the run's own key directory. Extract that predicate so the two callers cannot drift.

| `run.json` state | cancel would accept | cancel would refuse |
|---|---|---|
| `active`, `completing` | `docket run cancel --key <key> --reason <why>` (today) | `docket run verdict <key>` |
| `cancelling` | re-run the same cancel until it reports cancelled (today) | inspect or remove `<dir>` by hand |
| unknown or unreadable | by hand (today) | by hand (today) |

The `cancelling` + refuse cell covers a run that the death guardian fenced but that has no claim to cancel under. No documented command settles that state, so the honest remedy is the by-hand one.

### 3. Decision record

At build, record a new ADR (via `docket-adr`) that extends ADR-0124 Rule 1 from "only a verified `run-complete` drives the successful-run closeout" to "every keyed `run-done` verdict drives the closeout". It relates to ADR-0124 and ADR-0128. It supersedes neither: Rules 2–5 of ADR-0124 and ADR-0128's cancel proofs are unchanged.

### 4. Existing stuck runs

No migration. After install, running `docket run verdict <key>` on a stranded run re-resolves ownership, gets `no-attributable-claim`, and closes it out. The results file tells the human, as a human action, how to find stranded runs (a `run.json` still `active` beside a `record.json` with `terminal: true`) and to run the keyed verdict on each. It also names the three runs found at grooming (`implement-next-20260929t153306z-56752-95e7`, `implement-next-20261004t222959z-96475-7c95`, `implement-next-20261007t104957z-97517-71ff`) as known cases; re-check they are still `active` first.

## Tests

Each assert must go red when the code it guards is removed (mutation-test it).

1. **The incident.** Fixture: `record.json` with a child context hash and no binding file, zero matching claim proofs, `run.json` `active`, no participants. Keyed verdict → `run-done <key> no-attributable-claim`, `run.json` `completed`, and `liveRunsUnder` returns nothing (so `set-visibility` is no longer refused for it).
2. **Unconfirmed-reservation variant.** Reservation present, no committed receipt → same outcome.
3. **run-unclaimed.** Confirmed binding, change back to `proposed` → `run-done <key> run-unclaimed <id>`, `run.json` `completed`.
4. **Blocked closeout.** A live participant or launch under the run's context hash → `run-stop <key> run-tracker-unavailable completion-unaccounted`, `run.json` `completing`. Settle it, repeat the verdict → `run-done … no-attributable-claim`, `completed`.
5. **Halted stays active.** A `run-halted` verdict leaves `run.json` `active` and resume still works after cancel.
6. **Never relabel.** A `cancelling` run with no claim → the verdict does not mark it `completed`.
7. **Remedy text.** Live-run lines for an `active` run: with a confirmed binding → the cancel command; without → `docket run verdict <key>`; `cancelling` without a claim → the by-hand remedy.
8. **Observe mode.** `run verdict --unattributed` on the incident fixture leaves `run.json` `active`.

## Out of scope

- Why attribution fails: the child claiming without `--run-context` (#541) and command-launched dispatch (#345).
- `run cancel`'s ownership proofs; ADR-0128 stays as is.
- Halted and stop verdicts, which keep `run.json` `active`.
- Any new run state, run-tracker schema change, or change to the lock and generation scheme.
- Changing the resume-active-run remedy text in `run start --resume`.
