---
id: 477
slug: 'rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex'
title: 'Finish the run-tracker rename (--gate-context, DOCKET_AGENT_GUARDIAN_GATE_KEY, dispatch_context)'
status: 'implemented'
priority: 'high'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: []
stacked_on:
related: [471, 472, 473, 474]
discovered_from: [471]
adrs: [129]
spec: 'docs/superpowers/specs/2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-design.md'
plan: 'docs/superpowers/plans/2026-09-29-0477-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex.md'
results: 'docs/results/2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex'
pr: 'https://github.com/danielhanold/docket/pull/353'
blocked_by:
reconciled: true
claimed_at: '2026-09-29T21:39:42Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-design.md) |
| Plan | [2026-09-29-0477-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex.md](https://github.com/danielhanold/docket/blob/refactor/rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex/docs/superpowers/plans/2026-09-29-0477-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex.md) |
| Results | [2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-results.md](https://github.com/danielhanold/docket/blob/refactor/rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex/docs/results/2026-09-29-rename-the-gate-drive-s-leftover-gate-vocabulary-gate-contex-results.md) |
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0471 renamed the run gate to the run tracker as a hard cut, but kept three spellings that ADR-0129 had no row for: the gate drive's own `--gate-context` flag, the guardian environment variable `DOCKET_AGENT_GUARDIAN_GATE_KEY`, and the `run.start` result key `dispatch_context`. So callers still pass one run-context token under two flag names (`--run-context` to `change claim`, `--gate-context` to the gate drive), and the retired-vocabulary seal needs about 100 lines of special-case code to tell the gate drive's kept `--gate-context` apart from the retired `change claim` one.

## What changes

Apply ADR-0129 rows 38e-38h (recorded at this change's grooming, 2026-09-29) as one hard cut with no aliases:

- `gate drive start` and `gate drive prepare-scope`: `--gate-context` becomes `--run-context`, the same flag `change claim` takes.
- `DOCKET_AGENT_GUARDIAN_GATE_KEY` becomes `DOCKET_AGENT_GUARDIAN_RUN_KEY`.
- The `run.start` result JSON key `dispatch_context` becomes `run_context`.
- "Outer gate" and "gate context" wording in maintained Go comments and help text moves to run-tracker words.
- Skills, their embedded copies, the generated dispatch block and AGENTS.md, and golden output move with them.
- Seal: row 12 becomes a plain row that also covers 38e, and its kept-namesake special case is deleted. Rows 38f and 38g are added. Each changed or new row is mutation-tested. Row 38h (`rungate store`, already renamed by 0471) is recorded only, because row 38's token already seals it.

**Landing (human procedure):** merge with no dispatched run in flight, rebuild the binary immediately, restart coordinator sessions, and re-run `docket install` in consumer repos. There is no storage reset, because nothing persisted carries the old names.

## Out of scope

Aliases or a deprecation window; the committed claim-receipt key `gate_context_hash`; the gate drive's own name and every checkpoint sense of "gate"; point-in-time records (archived changes, specs, plans, results); rows owned by changes 0472-0474; the unrelated `gofmt` failure in internal/githubcli/comment_integration_test.go (change 0478).

## Reconcile log

### 2026-09-29

2026-09-29: Re-traced on main 32fd8adea. Every site the spec names still carries the old spelling (internal/cli/gate.go flag registrations and reads, agent_guardian.go env constant, runtracker_start.go dispatch_context tag, runtracker_run_id_refusal.go hint, skills docket-build / docket-build-task / docket-implement-next and embedded copies, harness/dispatch.go + AGENTS.md managed block, capability_production_test.go goldens, repoguard seal row 12 Kept machinery). No work landed elsewhere; 0472-0474 still proposed. Scope unchanged.
