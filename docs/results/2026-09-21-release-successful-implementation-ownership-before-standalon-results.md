<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0441 — Release successful implementation ownership before standalone finalize](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-22-0441-release-successful-implementation-ownership-before-standalon.md)**
<!-- docket:backlink:end -->
# Release successful implementation ownership before standalone finalize — Results

**Human action:** Optional. The change is covered by the automated suite (unit, mutation, and end-to-end integration tests) and needs no human action to be correct. One real-world walkthrough is worth doing before relying on it in anger, and one out-of-band artifact (a new ADR on the metadata branch) is noted below for the reviewer.

## Outcome

A docket implementation run is bracketed by a "run epoch" that owns the feature worktree so nothing else touches it while the build is in flight. Before this change, when an implementation run finished successfully, that epoch stayed **active** and its released worktree-execution slot kept pointing at the finished epoch. A later *standalone* finalize gate — one started without that epoch — then tried to reserve the same worktree and was refused with `stale-run-epoch`, so finalize could not proceed on a completed change without a human explicitly cancelling the run first.

This change gives a successful run a real close-out. The keyed, attributed run-verdict path (`RunGateVerdict` on a verified `run-complete`) now drives an ownership close-out through two new durable epoch states:

- **`completing`** — a success fence. The run verified complete, but ownership accounting is not yet finished, so the epoch still owns its worktree and admits no new registration, launch, mutation, takeover, or relaunch.
- **`completed`** — close-out finished. The epoch is terminal and is excluded from the ambient worktree-owner lookup, so a standalone finalize gate (and later supported mutations) can reserve the worktree normally. Explicit references to the completed epoch stay revoked.

Key properties, all preserved by design:

- Only the attributed keyed verdict drives close-out; `RunVerify` stays read-only and unattributed/observe-mode verdicts never change ownership.
- Close-out is **observation-only** — it stops no process, signals nothing, and settles no never-launched reservation. It **fails closed**: any live, busy, pending, uncertain, or unreadable obligation (an unobserved native participant, a live execution process, an unreleased/foreign worktree slot, an unaccounted launch, or a pending mutation) blocks close-out with a bounded `gate-unavailable` reason and no success reported. The remedy is to settle the named evidence and repeat the same keyed verdict.
- Success is never encoded as cancellation. Explicit human cancellation still **wins** from `completing` (completion then loses without reporting success); cancelling a `completed` run is a no-op refusal, never a state regression.
- Exact native-participant terminal observation (the coordinator's thread/turn) is now persisted at the adapter boundary and required as quiescence evidence; a bare owner-complete marker or a yielded return no longer counts.
- Ordinary between-drive release still retains the epoch and still refuses foreign/epoch-less starts between drives — full exclusion during outstanding work is unchanged.

The design decision is recorded as **ADR-0124** ("Successful-run ownership closeout extends the run-epoch lifecycle with completing and completed"), which relates to and extends ADR-0118 without rewriting it.

## Human actions and testing

### Important — Real-world standalone-finalize-after-completion walkthrough

The close-out path is verified by the hermetic Go suite, including an end-to-end integration test that arms a real epoch, runs a root coordinator, closes out, and then admits a standalone finalize reservation. What the hermetic suite cannot exercise is a genuine multi-process run on a real machine (a real implement-next run to `implemented`, then a separately-invoked `docket-finalize-change` on the same worktree). This matters because the original problem was observed in production ownership state, not in tests; a real walkthrough confirms the released-slot and participant-observation bookkeeping behaves as modelled outside the fixtures. If skipped, the residual uncertainty is only whether a real run leaves exactly the released-slot/terminal-participant state the fixtures arrange — the logic and its fail-closed guards are otherwise tested.

Prerequisites: a scratch repo migrated to docket mode, the freshly built `docket` binary, and the ability to run an implement-next run to an open PR (`implemented`).

1. Drive a normal implement-next run for a small change to `implemented` (PR open).
   Expected: the run reaches `implemented`; `docket run verify --id <id>` reports `run-complete`.
2. Confirm the epoch closed out: the same keyed `docket run gate-verdict <key>` returns `gate-done <key> run-complete <id>` (not `gate-stop … gate-unavailable`).
   Expected: `gate-done run-complete`.
3. Start a standalone finalize gate on that change (`docket-finalize-change` for `<id>`) on the same worktree.
   Expected: finalize's local gate admits and advances using its own continuation receipt — no `stale-run-epoch` refusal and no request for a human cancellation.

No cleanup beyond the scratch repo is required.

### Optional — Confirm cancellation still wins mid-close-out

A reader wanting to see the "cancellation wins from completing" guarantee can inspect `TestRunCancelWinsFromCompletingEpoch` and `TestRunCancelRefusesCompletedEpoch` in `internal/app/rungate_cancel_test.go`, and `TestCompleteSuccessfulRunSendsNoStops` in `internal/app/rungate_complete_test.go` (proves the success path issues no stop/cancel signals). Run: `go test ./internal/app/ -run 'TestRunCancel|TestCompleteSuccessfulRun' -count=1`. Expected: PASS.

## Verification performed

- Full source-resolved suite through the configured runner (`go run ./cmd/docket development test`): **green** — 52 files, 0 failed, 423 asserts. The build-evidence record at head `aa45a3eb` reflects this run. (An earlier full-suite run at head `d8ffc18a` was also green; the fix loop below re-certified after two minor fixes.)
- The initial full-suite gate went **red** on one integration test (`TestIntegrationWorkflowLifecycleRootEntryGateAttribution/context-preserved`): the end-to-end fixture drove a raw evidence gate but never released its raw worktree slot and never wired the participant/terminal recorder a real run establishes, so close-out correctly failed closed with `slot-ownership-unresolved`. Root-caused as a fixture gap (not an engine defect); the fixture was corrected to arrange the real-run evidence, and the closeout accounting was left unchanged. This is the reassuring direction: the fail-closed guard fired on incomplete evidence exactly as intended.
- Each production guard was mutation-tested during the build (fence transitions, re-enumeration before retirement, non-released-slot short-circuit, `CompleteEpoch` state gate, ordinary-release epoch retention, report-persistence check, successor protection) — each guard was stripped and the naming test confirmed to redden, then restored.
- Deep whole-branch review returned 0 blocker / 0 major findings and 2 minor findings, both fixed in-branch (see the PR disposition table); no re-review round is performed after fixes.
- Budget report on the green run: `BUDGET WATCH` screening lines on several parallel test files (streak 1/5) and no `SERIAL CONFIRMED OVER BUDGET` line — machine-dependent parallel wall-clock numbers, not an authoritative breach. Noted for awareness only.

## Known issues and follow-ups

### ADR-0124 lives on the metadata branch, invisible to the hermetic suite

ADR-0124 (the success-lifecycle decision record) was committed to the `docket` metadata branch, not to this feature branch, so it does not appear in this PR's diff and the hermetic suite cannot see it. This is expected for docket ADRs. The reviewer/merger can confirm it with `git -C .docket log --oneline -3` on the metadata worktree (commit `4cc61d4e`); change 0441's `adrs:` relation now lists 124. No action needed beyond awareness.

### Refusal-message prose for the new `run-completed` reason (fixed)

The publish/workspace mutation-refusal helpers hardcoded a "cancelled or superseded" human message; the new `run-completed` reason (returned when a `completing` epoch refuses a mutation) made that prose inaccurate for a successful mid-close-out run. This was a pre-existing message-accuracy gap that this change first made reachable, and it is fixed in-branch (the message is now reason-aware; the machine-readable reason token was unchanged). Recorded here only because it documents a consequence of introducing the `run-completed` reason — no residual risk.
