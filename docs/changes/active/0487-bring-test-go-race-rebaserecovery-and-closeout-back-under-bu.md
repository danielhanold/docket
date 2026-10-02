---
id: 487
slug: 'bring-test-go-race-rebaserecovery-and-closeout-back-under-bu'
title: 'Bring test_go_race, rebaserecovery, and closeout back under budget, fix the repoguard concurrent-gate timeout, and gofmt comment_integration_test.go'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-01'
updated: '2026-10-02'
depends_on: []
stacked_on:
related: [466, 465, 434, 373, 333, 362, 289, 280]
discovered_from: [475, 476, 478, 484, 485]
adrs: [108, 129]
spec: 'docs/superpowers/specs/2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu-design.md'
plan: 'docs/superpowers/plans/2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-02T05:54:04Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu-design.md) |
| Plan | [2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu.md](https://github.com/danielhanold/docket/blob/fix/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu/docs/superpowers/plans/2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md), [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
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

Settled at grooming (2026-10-01). The full design and measurements are in the linked spec.

1. **Seal scan cost.** Pre-filter `TestRetiredVocabularySeal`'s scan in `internal/repoguard`. A text or Go-literal row runs its regexp only when the line contains the row's spelling. The catalog operation-reference regexp runs only on blocks that contain a bound-flag spelling. Detection is unchanged by construction, and a mutation probe proves the seal still reddens. Grooming's prototype took the seal from 65.5s to 1.4s and `internal/repoguard` from 87.6s to 25.8s under `-race`.
2. **Concurrent-gate timeout.** Add no new load-bounding machinery. Prove the seal fix with a before/after run of two gates started together on one machine. If the backstop still trips, record a follow-up.
3. **Integration shards.** Split the closeout and rebaserecovery shards into two disjoint test-name prefixes each (the 0434 precedent). Add a wrapper and a `tests/runtime-budgets.tsv` row for each half, sized from serial solo readings.
4. **Budget rows.** `tests/test_go_race.sh` keeps its 60s row and is serial-confirmed under it. Keep the narrated numbers bound to their measurements (change 0289).
5. **gofmt.** Format `internal/githubcli/comment_integration_test.go`. Formatting only.

The full suite passes at the gate, and the budget report shows no `SERIAL CONFIRMED OVER BUDGET` line for the race gate or any closeout/rebaserecovery shard file.

## Out of scope

- Raising the 8-minute `-race` backstop.
- A machine-wide or cross-gate Go test-load bound (an ADR-0108 extension). It was declined at grooming because the measured cause is the seal.
- Changing the runner's budget regime, slack factor, or report semantics, or how it measures and enforces budgets.
- Converting the shard tests to `t.Parallel()`, or speeding up the slow integration tests themselves.
- Other budget rows and other `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines, unless the investigation shows a shared cause.
- `-race` failures in packages other than `internal/repoguard`.
- Any other gofmt drift, or changing how gofmt is enforced in the suite.

## Reconcile log

### 2026-10-01

Reconciled at claim against origin/main 97cc46c3e, the same head the spec's grooming measurements were taken on. No intervening merges; `gofmt -l internal/ cmd/` still flags only `internal/githubcli/comment_integration_test.go`. Scope unchanged.

### 2026-10-02

Resume reconcile after the 2026-10-01 halt (resumed via change.resume-halted on 2026-10-02; the stranded drive's deadline had passed and no docket or go test process was live). origin/main advanced 97cc46c3e -> b15e07c43 with change 0479 only: an unfiltered-run guard in internal/testsupport + internal/app, a new probe (11) in tests/test_go_integration_contract.sh, and a tests/README.md section. None of it touches the retired-vocabulary seal, the closeout/rebaserecovery shard wrappers, their SHARD_PREFIX values, or tests/runtime-budgets.tsv, so scope is unchanged. The feature branch stays on its recorded base (plan-only commit); finalize's rebase brings 0479 in. Task 1 (gofmt) remains already satisfied under the declared toolchain.
