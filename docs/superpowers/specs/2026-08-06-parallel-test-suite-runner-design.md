<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0227 — Parallel test-suite runner — 4x+ wall-clock speedup](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0227-parallel-test-suite-runner.md)**
<!-- docket:backlink:end -->

# Parallel test-suite runner — design

## Problem

The suite is 79 standalone bash test files run serially in ~10.5 minutes (629s, 5954
assertions). Profiling shows a heavy tail: `test_sync_agents.sh` (225s),
`test_harness_defaults.sh` (85s), `test_docket_config.sh` (55s), and
`test_sync_agents_codex.sh` (55s) are 66% of total wall time; the slow assertions are
dominated by real `sync-agents.sh` / resolver invocations costing 3–5s each.

Goal: **≥4x wall-clock speedup** (<157s) with zero change to what is asserted.

## Why parallelize (and the caveats)

Every test file is hermetic in design — `set -uo pipefail`, own tmpdir fixtures, no
ordering dependencies, runnable as `bash tests/test_X.sh`. There is no principled
objection to running them concurrently. The two real caveats:

1. **Hidden shared state** — any test touching the real `$HOME`, global git config, the
   repo's own worktrees, or the network would race. The runner isolates per-job
   (`HOME`, `GIT_CONFIG_GLOBAL`, `TMPDIR` per shard) and the change includes a one-time
   audit for offenders; any found are fixed or pinned to a serial group.
2. **Output interleaving** — solved by buffering each file's output to a per-shard log
   and emitting it atomically on completion, serial-identical in shape.

## Why file-level parallelism alone is not enough

Wall time under file parallelism floors at the slowest file: 225s ⇒ best case ~2.8x.
Hitting 4x requires breaking up the tail as well.

## Design

Two parts, both pure orchestration — no assertion changes:

### 1. `scripts/run-tests.sh` — the parallel runner

- Runs `tests/test_*.sh` (or explicit args) under `$DOCKET_BASH_PATH`/configured bash
  with N workers (default: CPU count, `-j N` to override; `-j 1` = today's serial
  behavior, byte-comparable output modulo ordering).
- Per-job isolation: fresh `TMPDIR`, `HOME` shim, `GIT_CONFIG_GLOBAL=/dev/null`-style
  guards so no test can see another's state or the developer's.
- Buffered output per file, flushed atomically on completion; longest-first scheduling
  (static order from the known tail) so the big files start immediately.
- Aggregated tail: per-file pass/fail line, failing-file list, total counts, non-zero
  exit iff any file failed. Same PASS/NOT-OK vocabulary the files already emit.
- Co-located contract `scripts/run-tests.md` per house convention.

### 2. Shard the tail files

Mechanically split the 3–4 slowest files into independent parts so no single shard
exceeds ~60s:

- `test_sync_agents.sh` (225s, 613 asserts) → ~4 parts along its existing per-change
  section boundaries (each section already builds its own fixture).
- `test_harness_defaults.sh` (85s) → 2 parts.
- `test_docket_config.sh` (55s) and `test_sync_agents_codex.sh` (55s) → 2 parts each
  if section boundaries allow; otherwise leave and accept the ~60s floor.
- Split = move assertion blocks + duplicate the shared helper prologue (or extract it
  to a sourced `tests/lib/` helper if the files already share one). Assertion count
  before == after, verified in the change.

Expected result: floor ≈ max shard (~60s) with ~8 workers ⇒ **~8–10x** on suite wall
time; comfortably past 4x even with contention.

### 3. Keep the tail from regrowing — a runtime-budget guard

A one-time split decays as new tests land. Ship the discipline as a guard (house
pattern: `test_skill_size_budgets.sh`):

- A budget table (e.g. `tests/runtime-budgets.tsv`) assigning every test file a
  wall-clock ceiling; default ceiling ~60s.
- A guard test asserting (a) every `tests/test_*.sh` has a budget row — a new file
  without one fails loudly, forcing a conscious placement decision, and (b) no file's
  measured runtime (from the runner's own timing output) exceeds its ceiling —
  the failure message says "shard this file or extend an existing shard."
- A short "where new tests go" section in `tests/README.md` (create if absent):
  extend the topical shard the assertion belongs to; a new file only for a new
  subsystem; never grow a file past its budget.

### Integration

- `finalize.test_command` is set explicitly to `scripts/run-tests.sh` in `.docket.yml`
  as part of this change (decided 2026-08-06) — no reliance on auto-detect, so the
  merge gate deterministically runs the parallel suite.
- `profile-asserts.sh` is untouched (profiling stays serial by design).

## Out of scope

- Optimizing the per-invocation cost of `sync-agents.sh`/resolver calls (a follow-up
  if the ~60s floor ever needs to drop further).
- Any change to assertion content or coverage.
- CI wiring beyond the runner itself.

## Verification

- Full suite via the runner: same 5954 assertions, 0 failures, wall time <157s.
- `-j 1` run matches serial semantics.
- Shard-count audit: sum of asserts across split parts equals the original file's count.
