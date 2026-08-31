---
id: 185
slug: test-suite-assertion-profilers
title: Test-suite profilers — per-assertion and per-command timing
status: done
priority: low
type: chore
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: [150]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: true
auto_groomable:
branch: feat/test-suite-assertion-profilers
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/146
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The suite is this repo's de-facto build gate — there is no GitHub Actions CI — and it runs 5m37s
across 73 files and 4,905 assertions. Every gate run pays that cost, and so does every build-loop
iteration that ends in a full-suite check.

Today the only timing signal available is whatever a caller wraps around each file, which answers
"which test file is slow" and nothing finer. That is the question with an easy answer; the useful
ones are "which part of that file" and "which command inside that part", and neither is reachable
without hand-instrumenting the file under suspicion — an edit that then has to be remembered and
removed.

The suite turns out to already carry the signal needed for the first question. Every test file
prints exactly one line per assertion: the `assert` helper most of them define and the `ok`/`no`/
`nok` trio the rest use all emit `ok - <name>` or `NOT OK - <name>`. That existing output is an
assertion protocol, so per-assertion timing needs no cooperation from the tests at all — only a
reader that timestamps lines as they arrive.

This is groundwork for the shell-toolchain work in #150 as much as an optimization aid: a per-file
total cannot show that a portability guard spends its time in one per-file `grep` loop, and that is
exactly the shape of cost that toolchain question is about.

## What changes

Two profilers under `scripts/`, each with its co-located contract, forming a find-then-explain
pair:

- **`profile-asserts.sh`** — runs the suite (or a named subset) and reports the wall time of each
  assertion *segment*: everything since the previous assertion line. Zero-touch and zero-overhead —
  it timestamps the tests' own output rather than instrumenting them — so it is safe to use as the
  routine way to run the suite. Emits slowest-segment, per-test-rollup and failing-assertion
  tables, plus a per-assertion TSV.
- **`profile-one-test.sh`** — profiles a single test at the command level, attributing self time to
  source lines and individual invocations. It enables xtrace through the environment rather than
  editing or sourcing the file, so `$0`, `BASH_SOURCE`, EXIT traps and the exit status all survive;
  because the traced environment is inherited, the profile reaches inside docket's own scripts
  under test.

Both resolve the repo root from their own location and run tests under the configured
`runtime.bash`, so neither carries a machine-specific path.

## Out of scope

- **No test file is edited**, and no timing hook is added to the suite. Both tools are external
  readers; that property is what makes the first one's numbers trustworthy.
- **No performance work on the tests themselves.** This change buys the measurement, not the
  optimization it might justify.
- **No CI wiring and no gate role.** Nothing in docket consumes their output and no skill invokes
  them; they exit non-zero only to mirror the suite's own result.
- **Not part of the consuming-repo convention.** These profile *this* repo's suite, which no
  consuming repo has.
- **No new test coverage for the profilers themselves.** They are read-only dev instruments whose
  output is checked by eye; guarding them would cost more than it protects. Recorded here as a
  deliberate call rather than an oversight.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
