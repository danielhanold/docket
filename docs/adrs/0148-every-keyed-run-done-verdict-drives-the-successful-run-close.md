---
id: 148
slug: 'every-keyed-run-done-verdict-drives-the-successful-run-close'
title: 'Every keyed run-done verdict drives the successful-run closeout'
status: 'Accepted'
date: '2026-10-08'
supersedes: []
reverses: []
relates_to: [124, 128]
change:
---

## Context

Only a keyed run-done `run-complete` verdict moved run.json out of `active`. `no-attributable-claim` and `run-unclaimed` wrote record.json terminal but left run.json `active`. The repository-wide live-run scan (`liveRunsUnder`, used by set-visibility) reads run.json, so it refused and named `run cancel`, which refuses these runs (`claim-unconfirmed`, by ADR-0128 design). No command could settle them; humans hand-edited run.json. Several such stranded runs existed at grooming.

## Decision

Extend ADR-0124 Rule 1 from "only a verified run-complete drives the successful-run closeout" to "every keyed run-done verdict (`run-complete`, `no-attributable-claim`, `run-unclaimed`) drives the closeout", so run.json ends `completed`.

- Report lines are unchanged on success; a blocked closeout reports `run-stop <key> run-tracker-unavailable completion-unaccounted` and is replayed by repeating the keyed verdict, consuming no retry.
- Halted, stop, retry and continue verdicts keep run.json `active` (resume relies on halted runs staying active until cancelled).
- Observe mode stays read-only.
- A cancelling, cancelled, or superseded run is never relabelled `completed`.
- `completed` means retired by its keyed verdict with full accounting; whether the implementation succeeded remains RunVerify's verdict, carried by the report line.
- The live-run scan picks its remedy using `run cancel`'s own ownership predicate (`runCancelOwner`), naming `docket run verdict <key>` when cancel would refuse.

ADR-0124 Rules 2-5 and ADR-0128's cancel proofs are unchanged.

## Consequences

- Stranded runs are cleared by re-running `docket run verdict <key>`; no migration.
- set-visibility no longer blocks on finished unattributed runs.
- The closeout's accounting now runs for more verdicts, so a live leftover obligation surfaces as `run-stop` rather than silently.

## Alternatives considered

- Let `run cancel` accept unclaimed runs: rejected, weakens ADR-0128 ownership proofs.
- Make `liveRunsUnder` read record.json terminal: rejected, a run with live obligations could be skipped without accounting.
- A new run state: rejected, no schema change needed.
- Migrate existing runs: rejected, re-running the verdict suffices.
