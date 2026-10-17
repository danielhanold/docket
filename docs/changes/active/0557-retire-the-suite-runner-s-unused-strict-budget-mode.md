---
id: 557
slug: 'retire-the-suite-runner-s-unused-strict-budget-mode'
title: 'Retire the suite runner''s unused strict budget mode'
status: 'in-progress'
priority: 'low'
type: 'refactor'
created: '2026-10-16'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [528, 273]
discovered_from: [528]
adrs: [74]
spec: 'docs/superpowers/specs/2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode-design.md'
plan: 'docs/superpowers/plans/2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/retire-the-suite-runner-s-unused-strict-budget-mode'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-17T22:10:25Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode-design.md](../../superpowers/specs/2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode-design.md) |
| Plan | [2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode.md](../../superpowers/plans/2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode.md) |
| ADRs | [ADR-0074](../../adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) |
<!-- docket:artifacts:end -->

## Why

The suite runner's strict budget mode (`DOCKET_RUNTESTS_STRICT=1`) re-checks every slow test immediately and fails the run with exit 4 on a confirmed or doubtful breach. Nothing turns it on: no gate, no `.docket.yml` key, no skill, no CI. The release-candidate workflow gets the same effect its own way, by failing a release when the log carries an `OVER BUDGET:` or `SERIAL CONFIRMED OVER BUDGET:` line. So strict mode is code, tests, an exit code, and docs that every budget change still has to design around (#528 had to decide its behavior under contention) for a mode with no users.

Its user-facing text has also gone stale. When a single-job run goes over budget, the runner tells you to set `DOCKET_RUNTESTS_STRICT=1` "but see `scripts/run-tests.md` first", and that file no longer exists. The strict arm's message says "`--strict-budget` was given", a flag the Go runner does not have.

Surfaced while grooming #528.

## What changes

Remove strict budget mode from the suite runner: the `DOCKET_RUNTESTS_STRICT` environment seam, `Config.Strict`, the strict re-check path (`StrictConfirmCandidates`), exit code 4, the strict note and the stale "set `DOCKET_RUNTESTS_STRICT=1` … see `scripts/run-tests.md`" hint in the budget report, the strict tests, and the strict sentences in `tests/README.md`. After this, no budget finding changes a run's exit code.

Keep the direct reading that a single-job run or a serial-mode budget row gets (`OVER BUDGET:`, exit 0). Screening and the solo re-check only run on a parallel run, so without it those two paths would carry ceilings nothing checks. A new test pins it, and `tests/README.md` is corrected: it wrongly says a single-job breach prints `SERIAL CONFIRMED OVER BUDGET:`.

Drop wording that implied a non-default mode: "by default" in `AGENTS.md`'s budget rule and "default" in the glossary's budget entry. Reword two stale comments in `.github/workflows/release-candidate.yml`: the claim that its greps are "the runner's complete budget vocabulary" (the `SERIAL CONFIRMATION NOT COUNTED:` line is left out on purpose, because it is meant to fail nothing) and the "(no --strict-budget)" aside. The greps themselves don't change.

## Out of scope

- The scheduled solo re-check and its contention handling (#528).
- Host-relative budgets (#273).
- Detecting another running suite during a direct single-job reading.
- The release-candidate workflow's greps; only its comments change.
- Point-in-time records that mention strict mode (plans, specs, results files, ADRs) and the frozen `testdata/repositories/status-corpus/` fixture.

## Reconcile log

### 2026-10-17

Reconciled against origin/main a52c2d9c9. The strict budget mode is still present exactly as the spec describes (DOCKET_RUNTESTS_STRICT read in internal/cli/development_test_cmd.go, the stale scripts/run-tests.md hint and ExitCode strictArmed in internal/suiterunner/aggregate.go). No related change touched it since grooming; #528 is done. Scope unchanged.
