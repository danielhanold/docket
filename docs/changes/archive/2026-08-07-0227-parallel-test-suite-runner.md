---
id: 227
slug: parallel-test-suite-runner
title: Parallel test-suite runner — 4x+ wall-clock speedup
status: done
priority: medium
type: chore
created: 2026-08-06
updated: 2026-08-07
depends_on: []
related: [225]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-06-parallel-test-suite-runner-design.md
plan: docs/superpowers/plans/2026-08-07-parallel-test-suite-runner.md
results: docs/results/2026-08-07-parallel-test-suite-runner-results.md
trivial: false
auto_groomable:
branch: feat/parallel-test-suite-runner
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/165
issue:
blocked_by:
reconciled: true
---

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-06-parallel-test-suite-runner-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-06-parallel-test-suite-runner-design.md) |
| Plan | [2026-08-07-parallel-test-suite-runner.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-parallel-test-suite-runner.md) |
| Results | [2026-08-07-parallel-test-suite-runner-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-parallel-test-suite-runner-results.md) |
<!-- docket:artifacts:end -->

## Why

The suite is 79 serial bash test files taking ~10.5 minutes (629s, 5954 assertions) —
right at the harness's foreground execution ceiling (see change 0225) and a growing tax
on every build's final gate. The files are hermetic by design, so serial execution buys
nothing. Profiling shows a heavy tail: four files (`test_sync_agents.sh` 225s,
`test_harness_defaults.sh` 85s, `test_docket_config.sh` 55s, `test_sync_agents_codex.sh`
55s) are 66% of wall time, so file-level parallelism alone floors at ~2.8x — the change
must also shard the tail to reach the 4x goal.

## What changes

- New `scripts/run-tests.sh` (+ co-located `run-tests.md` contract): N-way parallel
  execution of `tests/test_*.sh` under the configured bash, per-job `HOME`/git-config/
  `TMPDIR` isolation, buffered per-file output, longest-first scheduling, aggregated
  pass/fail summary, non-zero exit iff any file failed.
- Mechanically shard the tail files (`test_sync_agents.sh` into ~4 parts;
  `test_harness_defaults.sh`, and where section boundaries allow `test_docket_config.sh`
  / `test_sync_agents_codex.sh`, into 2) so no shard exceeds ~60s. Assertion count
  before == after.
- Runtime-budget guard so the tail never regrows: a per-file wall-clock budget table,
  a guard test failing on unbudgeted new files or budget-exceeding files (~60s
  ceiling), and a "where new tests go" section in `tests/README.md`.
- One-time audit for hidden shared state (real `$HOME`, global git config, repo
  worktrees, network); offenders fixed or pinned serial.

Target: <157s suite wall time (expected ~8–10x with ~8 workers).

## Out of scope

- Optimizing per-invocation cost of `sync-agents.sh`/resolver calls.
- Any change to assertion content or coverage.
- CI wiring beyond the runner itself.

## Open questions

- ~~Whether finalize's suite auto-detect should route through the runner or
  `finalize.test_command` should name it explicitly~~ — RESOLVED 2026-08-06: set
  `finalize.test_command` explicitly to the new runner once built.

## Reconcile log

### 2026-08-07

Reconciled at claim. Scope stands unchanged — nothing in it has been built or overtaken:

- `scripts/run-tests.sh` and `scripts/run-tests.md` do not exist; there is still no top-level
  runner, and every prior results file records the suite as an ad-hoc
  `for t in tests/test_*.sh; do bash "$t"; done` loop.
- `tests/` holds exactly **79** `test_*.sh` files — the count the spec profiled against — with no
  `tests/README.md` and no `tests/lib/` shared-helper directory yet, so the shard split must
  duplicate each file's helper prologue or introduce that directory as part of the work.
- `tests/runtime-budgets.tsv` does not exist; the budget guard is entirely new. The house pattern
  it copies (`tests/test_skill_size_budgets.sh`) is present.
- `.docket.yml` has no `finalize.test_command`, so the merge gate still auto-detects. Setting it
  explicitly to the new runner (resolved 2026-08-06) remains part of this change.
- `scripts/profile-asserts.sh` / `profile-one-test.sh` (change 0185) are present and untouched by
  this scope, as the spec intends.

Related-change check: **0225** — the umbrella "suite has grown into the harness's foreground
ceiling" change — was **killed** on 2026-08-07 in favor of this concrete design, so no work is
duplicated. Its two siblings stay independent and out of scope here: **0223** (build-gate execution
posture, still `proposed`/build-ready) and **0224** (green/red exit-code keying). **0150** (pin the
resolved shell toolchain across the suite, still `proposed`) remains adjacent — the runner picks the
bash it executes under, so 0150 should be built against the runner once it exists rather than the
other way around; no sequencing constraint is added here.

No ADRs cited by this change; none recently accepted that bear on it. Scope, spec, and the
verification criteria (5954 assertions, 0 failures, <157s) all carry forward as written.
