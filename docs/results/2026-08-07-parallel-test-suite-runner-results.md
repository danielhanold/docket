<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0227 — Parallel test-suite runner — 4x+ wall-clock speedup](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0227-parallel-test-suite-runner.md)**
<!-- docket:backlink:end -->

# Parallel test-suite runner — results

Change: #0227 · Branch: feat/parallel-test-suite-runner · PR: (see below) · Plan: docs/superpowers/plans/2026-08-07-parallel-test-suite-runner.md · ADRs: none

Suite wall clock: **592s → ~120s** (4.9x), assertions unchanged through every shard.

## Verify (human)

- [ ] **The budget breach is now advisory, not a gate.** A review blocker found that `run-tests.sh`'s
      exit 4 (green suite, one file over its wall-clock budget) reads as a *red suite* at both
      consumers — `docket-finalize-change`'s marker block and `docket-build`'s gate are bare
      non-zero checks — so a healthy-but-slow suite would have blocked the merge *and* dispatched
      `docket-integration-repair` to root-cause zero failures. The fix makes a breach advisory by
      default (loud report, exit 0), with `--strict-budget` opting into the failing exit. **This is
      a deliberate softening of the change's third pillar and it deserves your judgement**: the
      regrowth defence is now the loud report plus the structural guard, not a hard gate. Change
      0229 tracks making the check contention-independent so it can be hard again.
- [ ] **The rebase reconciliation.** Change 0220 merged into `main` *during* this build and added
      +229 lines to `tests/test_sync_agents.sh` — the file this branch splits into five. The branch
      was rebased and 0220's 34 added assertions were transplanted into
      `tests/test_sync_agents_runners.sh`. Byte-identity of the transplanted 269-line span against
      `origin/main` was verified, but this is the one place in the change where an assertion could
      have been silently lost, so it is worth your eye on the diff.

## Findings

Ten review findings (1 blocker, 3 important, 6 minor) — all fixed in-branch; see the PR body's
disposition table. Three deserve recording beyond that table:

- **The plan was wrong twice, and workers caught both by executing it.** Task 4 originally split
  `tests/test_docket_config.sh`; that file carries the change-0126 prelude guard, which scans its
  own `$BASH_SOURCE` and asserts a whole-file floor of ≥60 `eval` sites against a 64-site corpus,
  so *every* two-way split falsifies it. The split was dropped under the spec's own "otherwise
  leave and accept the ~60s floor" branch (the file is 50s, already inside the ceiling). The plan's
  other error: the named `test_harness_defaults.sh` boundary produced 4s/76s shards, buying 4s and
  leaving one over the ceiling — re-cut by measured `hd_validate` call distribution instead.
- **The budget slack constant is calibrated to one machine.** It shipped at 3/2, which rejected 11
  healthy files; measured contention inflation reached 2.22x, so it is now 5/2. That number is
  hardware-dependent in both directions — too tight and the gate flakes, too loose and enforcement
  goes vacuous for the 69 rows sitting at the 10s floor.
- **A guard's own claim was false.** The budget guard's two "relief counters" were documented as
  making it impossible to launder a number silently. They were not: counter A only fired above 60s,
  so raising any row to ≤60 evaded all five assertions, and a fourth relief path — adding
  `--no-budget-check` at the wiring site — turned the whole table into decoration with everything
  green. Closed by pinning the table's total and asserting the resolved `finalize.test_command`
  does not suppress the report.

No ADRs: every decision here is recorded at its own call site (the slack constant's comment,
`run-tests.md`'s "Why a breach is advisory by default", `tests/README.md`'s unsplit-file arguments)
rather than being a cross-cutting architectural rule.

## Follow-ups

Three stubs auto-captured during this run:

- **#0228** (`fix`) — `docket-finalize-change`'s auto-detect suite loop has no failure accumulator,
  so its exit status is the *last* test's. A mid-suite red merges green. Found while running this
  change's own build gate, which needed the accumulator added by hand to be trustworthy.
- **#0229** (`refactor`) — the budget slack factor is a hardware-dependent constant. Directly gates
  whether the advisory-by-default posture above can ever become a hard gate again.
- **#0230** (`refactor`) — a self-scanning population floor pins `test_docket_config.sh`'s size and
  blocks sharding it. Not urgent (50s, inside the ceiling), but the budget guard will redden when
  the file grows, and at that moment the cheapest path back to green is raising a number — the
  exact evasion the guard exists to catch.

Deliberately **not** split, each argued in `tests/README.md`: `test_sync_agents_codex.sh` (no
internal section banners to cut on) and `test_docket_config.sh` (see #0230).
