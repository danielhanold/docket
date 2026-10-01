---
id: 487
slug: 'bring-test-go-race-rebaserecovery-and-closeout-back-under-bu'
title: 'Bring test_go_race, rebaserecovery, and closeout back under budget, fix the repoguard concurrent-gate timeout, and gofmt comment_integration_test.go'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [466, 465, 434, 373, 333, 362, 289, 280]
discovered_from: [475, 476, 478, 484, 485]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0475, #0476, #0484, #0485, and #0478 (2026-10-01, at Daniel's request). The first four are all about suite runtime: three budget rows that the runner serial-confirmed as over budget, and one flaky gate timeout. #0478 is a one-file gofmt cleanup that rides along.

- **`tests/test_go_race.sh` over budget (#0484).** During change 0482's build gate the runner printed `SERIAL CONFIRMED OVER BUDGET: tests/test_go_race.sh — 94s solo; solo threshold 90s` against its `60 parallel` row. 0482 changed only comments and test wording, so the slowdown is already on `main`. Solo time has grown from 32s to 94s as rows were added to `TestRetiredVocabularySeal` in `internal/repoguard`.
- **`test_go_race` hits its 8-minute `-race` backstop in `internal/repoguard` under concurrent gates (#0485).** During 0482's gate a full-suite run at a results-only head went red once. The package hit the backstop while load average was 45-69, and change 0481's gate, running at the same time, hit the same backstop in the same package. A repair worker found no cause, and a re-run of the same head passed. So two concurrent gates on one machine can fail each other, and a green re-run proves nothing about the code. Changes 0465 and 0373 covered `internal/app` and integration race isolation, but nothing covers `internal/repoguard`. The seal scan from #0484 is the prime suspect.
- **`tests/test_go_integration_app_rebaserecovery.sh` over budget (#0476).** Serial-confirmed at 61s solo against a 60s threshold (row: `40 parallel`) during 0468's gate. 0468 changed only prose. This is the `TestIntegrationFinalizeRebaseRecovery` shard that change 0434 split out.
- **`tests/test_go_integration_app_closeout.sh` over budget (#0475).** Serial-confirmed at 69s solo against a 60s threshold (148s under parallel load; row: `40 parallel`) during 0470's first gate. It did not recur on 0470's second run, so it may be intermittent.
- **`internal/githubcli/comment_integration_test.go` fails `gofmt` (#0478).** Change 0471 left it alone on purpose to keep its diff about names only. Nothing else tracks it.

A serial-confirmed breach is the runner's authoritative signal, but it never fails the suite, so nothing else will act on these.

## What changes

1. **Seal scan cost (`internal/repoguard`).** Make `TestRetiredVocabularySeal`'s per-line scan cheaper, for example by pre-filtering each line before the per-row regexes run. Measure `tests/test_go_race.sh` solo time before and after on an untouched merge-base.
2. **Concurrent-gate backstop.** Reproduce the `internal/repoguard` `-race` timeout with two gates running at once, and confirm whether item 1 removes it. If it does not, fix the remaining cause by bounding total test load across concurrent gates (ADR-0108 direction), partitioning the package, or further scan work. Prove the fix with a before/after run under the same load.
3. **Integration shards.** For the rebaserecovery and closeout shards, find out whether each is a real regression (in the shard or the code it drives) or a row sized below the shard's real worst-case solo time. Fix it by speeding up or splitting the slow tests, as change 0466 did.
4. **Budget rows.** Re-size a `tests/runtime-budgets.tsv` row only when the time is genuinely needed, from measured worst solo readings (change 0466 precedent), and say why in the change. Keep the ledger narration bound to the numbers (change 0289).
5. **gofmt.** Run `gofmt -w internal/githubcli/comment_integration_test.go`. Formatting only, no behavior change.

The full suite must pass at the gate, and the budget report must show no `SERIAL CONFIRMED OVER BUDGET` line for the three affected files.

## Out of scope

- Raising the 8-minute `-race` backstop.
- Changing the runner's budget regime, slack factor, or report semantics, or how it measures and enforces budgets.
- Other budget rows and other `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines, unless the investigation shows a shared cause.
- `-race` failures in packages other than `internal/repoguard`.
- Any other gofmt drift, or changing how gofmt is enforced in the suite.
