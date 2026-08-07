<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0251 — Retune the run-tests budget regime for portability and sharding](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0251-retune-the-run-tests-budget-regime-for-portability-and-shard.md)**
<!-- docket:backlink:end -->

# Retune the run-tests budget regime for portability and sharding — design

Change: docs/changes/active/0251-retune-the-run-tests-budget-regime-for-portability-and-shard.md
Consolidates the two 0227-discovered legs (#0229 slack factor, #0230 population floor), designed
jointly because the slack retune decides what a shard's budget row means and the floor rework is
what permits the shard at all.

## Problem (verified 2026-08-07)

1. **Slack leg.** `scripts/run-tests.sh:78` compares parallel wall clock against serially-seeded
   ceilings via a fixed `5/2` calibrated on one 11-core host (measured contention 2.22x). Smaller
   hosts flake red; larger hosts make the comparison near-vacuous (`scripts/run-tests.md` "stated
   honestly": the effective trip point runs 2.75x–25x of real cost). Since the stub was filed the
   posture already moved: a breach is **advisory by default** and `--strict-budget` (exit 4) is the
   opt-in gate — the remaining defect is that the *measurement* is contention-dependent, so even the
   opt-in gate is dishonest off the calibration host.
2. **Floor leg.** `tests/test_docket_config.sh:2722` asserts the 0126 prelude-correspondence guard
   reached `>= 60` eval sites by scanning `${BASH_SOURCE[0]}`, with a same-file grep cross-check
   (`:2745-2746`) and the r9 site derivation (`:2759-2764`). Any split leaves both halves near 32
   and falsifies the floor, so the guard pins the suite's biggest file (now **2868** lines after
   0223 landed; budget 55s, measured ~50s serial — and 0223/0258 keep growing it). It is the only
   guard in the suite with this whole-file shape.

## Design

### Leg 1 — verdict by serial confirmation, not by tuned slack

Keep the advisory default and the caller contract untouched (exit map 0/1/3/4/2, run-tests.md "Why
a breach is advisory by default", ADR-0074's tri-state reading). Change what a breach *is*:

- **Parallel phase = screen only.** The existing `measured > ceiling * 5/2` comparison is demoted
  from verdict to screen. A file that trips it becomes a *candidate*, not a breach.
- **Serial confirm = verdict.** After the parallel phase (machine now idle), each candidate is
  re-run once serially in the same sandboxed way, and the verdict compares the serial re-measure
  against `ceiling * 3/2`. A serial measurement against a serially-seeded ceiling is the honest
  comparison on any host — the hardware-dependent constant no longer decides anything, it only
  spends a bounded re-run.
- **At `-j 1` there is no contention to screen out:** compare directly at `3/2`, no confirm pass.
- **Reporting.** Confirmed breaches keep today's `OVER BUDGET:` block, remedy text, and
  `--strict-budget` exit 4. A screened-but-cleared file prints one informational line
  (`screened at <t>s under -j<N>, cleared at <t'>s serial`) so contention visibility is not lost.
  The parallel run's rc/ok/notok stay the report of record — the confirm re-run exists only to
  measure time and never changes the suite verdict in either direction. A confirm re-run that
  itself exits non-zero **never clears the candidate**: its measurement is bogus (a crash yields
  a spuriously low time), so the file is reported `unconfirmed` in the `OVER BUDGET:` block with
  a named anomaly line alongside (file, parallel rc, confirm rc); under `--strict-budget` an
  unconfirmed-because-confirm-failed candidate counts as a breach (exit 4, fail-closed on the
  opt-in sharp path), while the advisory default reports it and exits 0 (see assumption 10).
- **Failure precedence unchanged:** exits 1 and 3 preempt any budget outcome; confirm runs are
  skipped entirely when the run is already red (same reason the red branch prints first today).
  A red run's candidates still appear in the existing `OVER BUDGET:` block, marked
  `unconfirmed — screened at <t>s under -j<N>; not re-measured on a red run`, under the same
  precedence note the block prints today.
- **Comments/docs**: rewrite the `:56-78` block and run-tests.md's budget sections for the new
  two-stage shape; keep the calibration history (the measurement, per learning
  `tolerance-constant-calibrated-on-one-machine`) and repoint the "change 0229" references to 0251.
- **Mutation proof (in `tests/test_run_tests.sh`):** a fixture whose measured cost is ~3x its
  `--budgets` fixture row (the row seeded at true cost, no padding — so screen 5/2 and confirm 3/2
  both trip on an idle host; a mere 2x sits below the 5/2 screen and is exactly what today's check
  also misses) is confirmed OVER BUDGET at any `-j`; a fixture slow only under parallel contention
  (screened) is cleared by the serial confirm; multiple candidates in one run are each confirmed
  once, in sequence, and all reported; `--strict-budget` exits 4 only on a confirmed breach;
  `--no-budget-check` still skips screen and confirm both.

### Leg 2 — family-corpus population, then the split

- **Population unit = the discovered family corpus.** `prelude_report` (and the raw-grep
  cross-check extractors) iterate every file matching `tests/test_docket_config*.sh` (glob at the
  guard site, `LC_ALL=C` sorted), not one `BASH_SOURCE`. Membership is *computed* from the glob —
  never an enumerated file list (ADR-0050; learning `backstop-must-compute-not-reenumerate`) — so a
  new shard self-registers into the corpus exactly as it self-registers with the runner.
- **Floors stay whole-corpus and keep today's values:** sites `>= 60`, keycount `>= 20`, `ok >=
  90%`, `viol == 0`, and the two-extractor agreement summed across the family. One new floor on
  the corpus itself: the glob matched `>= 2` files (post-split), so a renamed shard cannot quietly
  shrink the corpus to one file and go green (learning
  `marker-scoped-guard-needs-a-population-floor`).
- **SITE lines gain file attribution** (`SITE <basename>:<line> ...`); the r9 site derivation and
  the 0148 assert key on `<basename>:<line>` of whichever shard holds the r9 fixture, still
  derived by pattern, never hand-counted.
- **Guard lives once**, in the shard that keeps the file tail (markers, `T_SELF_START` block,
  section T/0148 asserts move as one unit); the self-block subtraction applies only to the file
  that carries the markers. Other shards carry no marker literals.
- **Then split** `tests/test_docket_config.sh` two ways at a measured `# ===` section boundary
  (target: each part <= ~30s serial, measured via `--timings`), named per the tests/README family
  convention (`tests/test_docket_config_<topic>.sh`). Prove assertion-count parity: identical
  summed ok/notok before vs after, from `-j 1 --timings` runs. Re-cut the two budget rows from the
  measured post-split run (seeding rule unchanged: next multiple of 5 + 5s, min 10), re-seed
  `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh`, and update tests/README.md's "two files
  were argued whole" paragraph (test_docket_config.sh leaves that list) plus run-tests.md's stale
  86-file counts where touched.

### Out of scope (unchanged from stub)

- Budget values themselves beyond the mechanical re-cut of the split file's rows.
- Any non-config-suite shard; `test_sync_agents_codex.sh` stays argued-whole.
- New default gating: the advisory posture is not revisited here.

## Assumptions

1. **Advisory-by-default stands; this change does not add a fatal default path.** Chosen: keep the
   0/1/3/4 exit contract byte-compatible; only the honesty of the measurement changes. Rejected:
   making a confirmed breach fatal by default — a live caller still reads bare non-zero as "suite
   red": finalize's `configured-bash-finalize` block is a bare eval (docket-build's gate is
   tri-state since ADR-0074 and would Halt, but finalize would dispatch repair at nothing;
   learning `exit-code-encodes-a-non-failure`), and re-plumbing callers is not this change.
   Rejected: demoting to report-only always (deleting `--strict-budget`) — it retires the only
   sharp instrument just as it becomes honest.
2. **Contention independence via screen + serial re-measure**, not via a smarter constant. Chosen
   because the verdict then compares like with like — a serial measurement against a
   serially-seeded ceiling — which removes the *contention* dependence, the defect both stubs
   name. Stated residual: the table values themselves still encode the calibration host's
   absolute speed, so a host uniformly ~2x slower could confirm a healthy near-ceiling file; that
   dependence lives in `runtime-budgets.tsv`, whose values are this change's stated out-of-scope,
   and it is confined to the opt-in `--strict-budget` path with 3/2 slack plus 5-10s seed padding
   as headroom (~1.6-1.9x of host-speed cover on the largest rows). If the strict path proves
   flaky on slower hosts, a serial canary rescale is the follow-up shape — rejected *here*
   because it adds stored per-host baseline state and a single-point estimator (one canary's
   cache/IO profile standing in for every file) to fix a path nothing runs automatically today.
   Rejected: enforce only at `-j 1` — nothing runs `-j 1` in anger, so the guard goes decorative.
   Rejected: normalize by a run-derived contention factor (median measured/ceiling) — ceilings
   are floor-padded (71 of 88 rows at 10s vs ~1-3s real cost), so the ratio population is dominated by
   padding noise and the normalizer is garbage. Rejected: scale slack from cores/jobs — still a
   machine-calibrated model, just with more inputs (the learning's point).
3. **Screen constant stays 5/2; confirm/serial constant is 3/2.** The screen keeps the measured
   worst case (2.22x) covered so healthy files rarely pay a re-run; a wrong screen now costs one
   bounded re-measure (too tight) or misses only what today's check also misses (too loose — the
   tail framing in run-tests.md stands). 3/2 serial keeps the flake headroom a loaded laptop needs
   while sitting far below the ~2.75x+ effective trip point of today. Rejected: deriving either
   from host inspection (assumption 2's rejection applies).
4. **Confirm pass is one unconditional serial re-run per candidate, no cap, no timeout.** The
   bound comes from the table's own padding arithmetic, not from one machine's zero-candidate
   run: 71 of the table's 88 rows sit at the 10s floor over ~1-3s of real cost and need >8-25x
   inflation to screen at 5/2, so even a badly contended small host realistically screens only
   among the 17 above-floor rows (budgets summing 635s; real serial cost 465-550s by the seeding
   rule). The theoretical worst case — every big row exceeding 2.5x its ceiling under contention
   — is therefore ~8-9 minutes of confirm tail, but reaching it requires contention past 2.5x on
   every large file simultaneously, at which point the run is diagnosing a genuinely
   oversubscribed host and the measurement is worth its cost; any plausible run screens a
   handful of big rows at most (seconds to ~a minute of confirm). The mutation proof covers the
   many-candidates path explicitly. No-timeout: a file that would hang the confirm had already
   hung the parallel phase. Rejected: capping candidates or time-boxing the re-run — the cap
   would bite only in the oversubscribed-host case, where dropping measurements is the wrong
   trade.
5. **Population unit is the whole-family corpus discovered by glob.** Rejected: per-file scaled
   floors — a cut point choice would move the floor arithmetic, making every re-shard a guard
   edit. Rejected: enumerated corpus — re-enumeration drifts and is the exact shape ADR-0050
   forbids for backstops. The `>= 2` files corpus floor covers the glob going quietly narrow.
6. **The guard (markers + section T + 0148 asserts) stays a single unit in the tail-holding
   shard.** Rejected: a dedicated guard-only shard file — it would need its own budget row and
   README entry for zero isolation benefit; the guard is corpus-wide either way.
7. **Two-way split at a measured section boundary, parity proven by summed assert counts.**
   Rejected: three-way or per-section shards — 0258 lands more asserts in this family and a finer
   cut multiplies row churn; two parts at ~25-30s each leave headroom for both 0258 legs.
   Coordination with #0258 (same file) is at build time: whichever lands second rebases; the glob
   corpus and family floors make the guard indifferent to where 0258's asserts land.
8. **Docs move with the code in the same change** (run-tests.sh comment block, run-tests.md budget
   sections + exit-code table wording, tests/README.md placement guidance, plus the mechanical
   nits: `tests/test_runtime_budgets.sh:14`'s "under the 5/2 factor" comment and the suite file
   count in `tests/README.md:3`, `run-tests.sh:4`, and run-tests.md — those docs still say 86 but
   the suite is 88 files today (TSV rows and EXPECTED_TOTAL 1345 = 71x10 + 635 agree), so the
   post-split count they get is **89**, not 87) —
   the 0229/0230 stub ids referenced there repoint to 0251. Rejected: deferring doc updates — the
   argued-whole paragraph in tests/README.md would actively instruct against this change's own
   split.
9. **Dependency state:** none. #0258 is `related`, not `depends_on`, in either direction — both
   are groomed against today's un-split file and the build-time rebase rule in assumption 7
   handles the collision.
10. **The confirm re-run never changes the suite pass/fail verdict, but a failed confirm never
    clears a budget candidate.** Verdict authority is single-sourced in the parallel run's
    records: a flaky pass on re-run must not launder a red file, and a parallel-green file whose
    confirm re-run exits non-zero must not flip the suite red — it prints a named anomaly line
    (file, parallel rc, confirm rc). On the *budget* axis, though, a crashed confirm produces a
    bogus low measurement, so treating it as "cleared" would let a slow-and-flaky file pass
    `--strict-budget` — the candidate stays `unconfirmed` in the `OVER BUDGET:` block, breaches
    under `--strict-budget` (fail-closed on the opt-in path), and is advisory-reported at the
    default. Rejected: letting a red confirm redden the suite (two authorities for one verdict);
    rejected: letting it clear the candidate (dulls the sharp instrument exactly where it
    matters).
