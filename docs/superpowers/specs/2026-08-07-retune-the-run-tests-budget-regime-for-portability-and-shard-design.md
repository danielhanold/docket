<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0251 — Retune the run-tests budget regime for portability and sharding** — `docs/changes/archive/2026-08-23-0251-retune-the-run-tests-budget-regime-for-portability-and-shard.md`
<!-- docket:backlink:end -->

# Retune the run-tests budget regime for portability and sharding — design

Change: docs/changes/active/0251-retune-the-run-tests-budget-regime-for-portability-and-shard.md
Consolidates the two 0227-discovered legs (#0229 slack factor, #0230 population floor), designed
jointly because the slack retune decides what a shard's budget row means and the floor rework is
what permits the shard at all.

**Amended 2026-08-11 (human review):** the budget leg is revised from "one unconditional serial
re-run per candidate per run" to a stateful confirmation schedule — candidates are tracked across
executions and confirmed individually on a bounded cadence. Assumption 4's no-cap/no-timeout
design is withdrawn. The floor leg (Leg 2) is unchanged.

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

### Leg 1 — stateful budget confirmation (amended 2026-08-11)

Keep the advisory default and the caller contract untouched (exit map 0/1/3/4/2, run-tests.md "Why
a breach is advisory by default", ADR-0074's tri-state reading). Change what a breach *is*: only a
solo (serial) measurement or a `-j 1` measurement can establish a confirmed breach. Parallel
measurements are screening observations tracked in persistent state; confirmation runs are
scheduled from that state on a bounded cadence rather than appended unconditionally to every run.

**Thresholds.** The serially seeded ceilings in `tests/runtime-budgets.tsv` stay the reference.

- Parallel screening threshold: `parallel_time > serial_ceiling * 5/2`. Crossing it creates a
  *candidate observation* — never, by itself, an authoritative breach.
- Solo confirmation threshold: `solo_time > serial_ceiling * 3/2`.

**Qualifying parallel overrun.** A measurement advances persistent state only when ALL hold:
budget checking is enabled; the invocation is a normal full-suite execution over the default
discovered corpus (not a targeted subset); `JOBS > 1`; the test is configured for parallel
execution; the test exited 0; every requested test produced a result; the overall suite is green;
and the measured parallel time exceeds `serial_ceiling * 5/2`. Red, incomplete, interrupted,
targeted, and `--no-budget-check` runs neither advance nor reset persistent counters — their
timings still appear in the current run's normal report.

**Per-test state model.** One persistent record per test and execution context, in one of four
states:

- `unobserved` — no qualifying history.
- `watching` — fewer than five consecutive qualifying overruns.
- `parallel-sensitive` — slow in parallel, healthy at its most recent solo confirmation.
- `confirmed-breach` — over budget at its most recent solo confirmation.

Record fields: `schema_version`, `test_path`, `context_key`, `state`, `initial_overrun_streak`,
`overruns_since_confirmation`, `last_parallel_seconds`, `last_solo_seconds`, `budget_seconds`,
`last_confirmation_result`, `due_sequence`. State is evidence about previous measurements; it
never replaces the result of the current parallel execution.

**Initial confirmation trigger.** For `unobserved`/`watching`: a qualifying overrun increments
`initial_overrun_streak`; a qualifying full-suite measurement below the parallel threshold resets
it to zero; the fifth *consecutive* qualifying overrun makes the test due for its first solo
confirmation.

**Classification at solo confirmation.** On a solo confirmation that completes successfully:
`solo_time <= ceiling * 3/2` → `parallel-sensitive`, `last_confirmation_result = cleared`;
`solo_time > ceiling * 3/2` → `confirmed-breach`, `last_confirmation_result = breached`. Either
way `overruns_since_confirmation` resets to 0 and `last_solo_seconds` records the measurement.
The confirmation run exists only to establish the budget classification — it never replaces or
alters the parallel run's test result, assertion counts, log, or suite pass/fail verdict.

**Periodic revalidation.** After a successful solo confirmation, each later qualifying overrun
increments `overruns_since_confirmation`; parallel executions below the screening threshold
neither increment nor reset it. At ten, the test becomes due for another solo confirmation. The
counter resets to zero only on a successful solo confirmation. The interval therefore means every
tenth later overrun *for that test*, not every tenth suite execution.

**Bounded confirmation tail.** A normal full-suite execution performs at most ONE scheduled solo
confirmation. When several tests are due: (1) confirm the largest overdue count — the amount by
which a record's counter exceeds its trigger threshold (5 initial / 10 recheck); (2) tie → the
test that became due first (`due_sequence`, assigned from a monotonic per-state-file counter when
a record becomes due); (3) still tied → `LC_ALL=C` test-path order. Deferred tests stay due —
their counters do not reset until their own solo confirmation completes successfully. The report
marks them deferred (see reporting).

**`--strict-budget` bypasses the schedule.** On a strict run: every *current* parallel candidate
is rerun individually, immediately — the five-overrun initial threshold, ten-overrun recheck
interval, and one-confirmation-per-run bound all do not apply. A healthy solo result clears the
candidate; a solo result over `ceiling * 3/2` is a strict breach (exit 4); a failed solo
confirmation cannot clear the candidate and fails closed on the strict budget axis (exit 4).
Successful strict confirmations update persistent state. Exit precedence unchanged: 1 (test
failure) > 3 (missing result) > 4 (strict budget breach or failed strict confirmation) > 0.
Strict stays authoritative without stored history: it confirms current candidates on the spot, so
a corrupt or missing state file never dulls the sharp instrument.

**Confirmation failure.** A solo confirmation exiting non-zero must not replace the parallel
result, turn the suite red in advisory mode, clear the candidate, reset either counter, or record
a healthy solo duration. It records `last_confirmation_result = failed`. Advisory mode reports
the anomaly and leaves the test due; strict mode treats it as an unconfirmed breach (exit 4
unless exits 1/3 take precedence). Rationale unchanged from the original design: a crashed
confirm yields a spuriously low time, so "cleared" would let a slow-and-flaky file pass the gate.

**`-j 1`.** The measurement is already uncontended: compare directly against `ceiling * 3/2`,
update no parallel-overrun counters, schedule no second execution. Advisory report on a normal
run; exit 4 under `--strict-budget`.

**Persistent state store.** `$GIT_DIR/docket/run-tests-budget-state.tsv`, with `$GIT_DIR`
resolved through Git (`git rev-parse --git-dir`), never by assuming `.git` is a directory — this
gives linked worktrees separate histories. The file is untracked, survives normal runs, carries a
versioned schema, is created with restrictive permissions, and is rewritten via temp file +
atomic rename beside its destination (same-filesystem; the AGENTS.md mktemp template rule) —
never left partially written.

**Execution-context isolation.** The state key includes `test_path`, `job_count`,
`logical_cpu_count`, `operating_system`, `architecture`, `budget_ceiling`, `execution_mode`, and
`schema_version` — a test observed under `-j16` neither advances nor consumes the `-j4` history.
Records are NOT keyed to the current commit: resetting per commit would prevent ever accumulating
five observations during normal development.

**State invalidation.** Reset a test's state when its repo-relative path, budget ceiling,
execution mode, execution-context key, or the schema version changes. Do NOT reset an established
`parallel-sensitive`/`confirmed-breach` record for a branch change, a new commit, or a parallel
execution below the screening threshold. (A `watching` test's five-streak DOES reset on a
qualifying below-threshold measurement — the asymmetry is deliberate: the initial streak demands
consecutive evidence; revalidation counts accumulated overruns.)

**Concurrent updates.** Guard the state with a portable atomic lock directory: acquire, reload
state under the lock, apply this run's observations, write the full replacement to a temp file,
atomically rename, release. If the lock is not acquired within a short bounded period: continue
the run, do not modify state, print ONE warning naming the lock path (so a stale lock left by a
killed run is discoverable and removable by hand), and do not claim counters advanced. A lock
failure never fails the suite.

**Corrupt or unavailable state.** State persistence is advisory infrastructure and must never
prevent test execution. Unknown schema version → old state ignored and rebuilt. Malformed record
→ ignored, reported once. Unreadable/unwritable file → run operates without history and the
report says so. Strict mode remains authoritative regardless (it needs no stored history).

**Reporting.** A parallel screen crossing is never labeled `OVER BUDGET`. Classifications:

- `BUDGET WATCH: <path>` — e.g. `75s under -j8; consecutive parallel-overrun streak 3/5`
- `PARALLEL-SENSITIVE: <path>` — e.g. `75s under -j8; last solo measurement 20s; recheck progress 4/10`
- `SERIAL CONFIRMATION DUE: <path>`
- `SERIAL CONFIRMATION DEFERRED: <path>` — e.g. `Recheck is due; another test consumed this run's confirmation slot`
- `SERIAL CONFIRMED OVER BUDGET: <path>` — e.g. `75s under -j8; 40s solo; solo threshold 37.5s`
- `SERIAL CONFIRMATION FAILED: <path>`

Only a solo-confirmed result or a direct `-j 1` measurement may be described as over budget. The
report includes the latest parallel and solo evidence needed to explain each classification.

**Flag interactions.** `--no-budget-check` skips the screening comparison and all solo
confirmations, neither reads nor writes budget history, and leaves every counter unchanged; its
contradiction with `--strict-budget` remains a usage error. The `--timings` five-column format
(`path seconds rc passes failures`) stays byte-compatible — no budget-state fields are appended;
the state file carries the cross-run classification data.

**Required tests (deterministic — no real multi-second sleeps; counter transitions and
scheduling are exercised through controlled fixture measurements, which requires a test-only
seam for injecting measured durations rather than wall-clock sleeping):**

1. The first four consecutive qualifying overruns do not trigger confirmation.
2. The fifth consecutive qualifying overrun triggers exactly one solo confirmation.
3. A qualifying clean result before the fifth overrun resets the initial streak.
4. A healthy solo result records `parallel-sensitive`.
5. The next nine qualifying overruns do not trigger another confirmation.
6. The tenth later qualifying overrun triggers exactly one recheck.
7. Clean parallel results do not advance the ten-overrun counter.
8. Clean parallel results do not reset the ten-overrun counter.
9. Targeted test runs do not mutate persistent history.
10. Red suite runs do not mutate persistent history.
11. Runs with missing results do not mutate persistent history.
12. Interrupted runs do not mutate persistent history.
13. `--no-budget-check` neither reads nor writes history.
14. `--strict-budget` confirms every current candidate immediately.
15. Strict confirmations update stored state.
16. `-j 1` performs a direct comparison without a second execution.
17. Different `-j` values maintain independent histories.
18. A budget-ceiling change invalidates the old record.
19. An execution-mode change invalidates the old record.
20. Only one scheduled confirmation runs during a normal suite execution.
21. Multiple due tests are confirmed across later suite executions in deterministic order.
22. A deferred confirmation remains due.
23. A failed confirmation cannot clear a candidate.
24. A failed advisory confirmation does not change the suite verdict.
25. A failed strict confirmation produces exit 4 when exits 1 and 3 do not apply.
26. Corrupt state records are safely ignored and reported.
27. An unavailable state lock does not lose or overwrite another runner's update.
28. Existing `--timings` output remains byte-compatible.
29. The parallel run remains the sole authority for test results, assertion counts, and logs.
30. The persistent report includes the latest parallel and solo evidence needed to explain a
    test's classification.

**Docs move with the code:** `scripts/run-tests.sh` comment block, `scripts/run-tests.md`, and
this design state: budget rows remain serially seeded; parallel measurements are screening
observations only; five consecutive qualifying overruns trigger the first solo confirmation; a
parallel-sensitive test is rechecked after every ten later qualifying overruns; a normal run
performs at most one scheduled confirmation; `--strict-budget` confirms all current candidates
immediately; only solo or `-j 1` measurements establish authoritative breaches; budget history is
local to the worktree and execution context; red/incomplete/interrupted/targeted/budget-disabled
runs do not mutate history; the default posture stays advisory; exit codes and precedence are
unchanged. AGENTS.md's Guards bullet ("A trailing `OVER BUDGET:` line is a finding to act on")
is updated to the new vocabulary in the same change. The prior design language requiring one
unconditional serial rerun per candidate with no cap and no timeout is removed.

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
3. **Screen constant stays 5/2; confirm/solo constant is 3/2.** The screen keeps the measured
   worst case (2.22x) covered so healthy files rarely produce candidate observations; a wrong
   screen now costs counter noise (too tight) or misses only what today's check also misses (too
   loose — the tail framing in run-tests.md stands). 3/2 solo keeps the flake headroom a loaded
   laptop needs while sitting far below the ~2.75x+ effective trip point of today. Rejected:
   deriving either from host inspection (assumption 2's rejection applies).
4. **Confirmation is scheduled from persistent state, bounded at one scheduled confirmation per
   normal run.** *(Amended 2026-08-11 — this replaces the original "one unconditional serial
   re-run per candidate, no cap, no timeout", withdrawn on human review.)* The original design
   attached an unbounded serial tail to every contended run and could not distinguish persistent
   parallel contention from a genuine solo regression — every candidate paid a confirm on every
   run forever. The stateful schedule preserves parallel execution as the normal posture, bounds
   the added serial cost per run at one test, classifies contention-sensitive tests once and
   revalidates them on a ten-overrun cadence, and operates automatically without users requesting
   diagnostic runs. The sharp instrument does not depend on the schedule: `--strict-budget`
   confirms all current candidates immediately, so stored history is never load-bearing for the
   gate. Rejected: keeping the unconditional per-run confirm — the tail it buys is spent
   re-measuring the same contention-sensitive files on every run.
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
   sections + exit-code table wording, tests/README.md placement guidance, AGENTS.md's
   `OVER BUDGET:` Guards bullet, plus the mechanical
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
10. **The confirmation run never changes the suite pass/fail verdict, but a failed confirmation
    never clears a budget candidate.** Verdict authority is single-sourced in the parallel run's
    records: a flaky pass on re-run must not launder a red file, and a parallel-green file whose
    confirmation exits non-zero must not flip the suite red — it is reported as
    `SERIAL CONFIRMATION FAILED` with the evidence (file, parallel rc, confirm rc). On the
    *budget* axis a crashed confirmation produces a bogus low measurement, so treating it as
    "cleared" would let a slow-and-flaky file pass `--strict-budget` — the candidate stays due
    (`last_confirmation_result = failed`, counters unreset), breaches under `--strict-budget`
    (fail-closed on the opt-in path), and is advisory-reported at the default. Rejected: letting
    a red confirmation redden the suite (two authorities for one verdict); rejected: letting it
    clear the candidate (dulls the sharp instrument exactly where it matters).
11. **The state store is advisory infrastructure — fail-open for the run, fail-closed only via
    strict.** A missing, corrupt, locked, or unwritable state file never blocks or fails a test
    run; the run proceeds without history and says so. This is safe because nothing
    authoritative reads the stored state: advisory classifications inform, and `--strict-budget`
    re-measures current candidates directly. Rejected: hard-failing on state errors — it would
    convert bookkeeping breakage into suite outages for a mechanism whose default posture is
    advisory.
12. **Context-keyed histories, never commit-keyed.** The key (`test_path`, `job_count`,
    `logical_cpu_count`, OS, arch, `budget_ceiling`, `execution_mode`, `schema_version`) isolates
    materially different execution regimes while letting observations accumulate across normal
    development (five consecutive overruns must be reachable). Ceiling/mode/schema changes
    invalidate via the key itself. Rejected: commit-keyed records — reset on every commit, the
    five-observation trigger is unreachable. Rejected: one shared history across `-j` values — a
    `-j16` contention profile says nothing about `-j4`.
13. **Deterministic state-machine tests need a measured-duration injection seam.** The 30-item
    test list must run without real multi-second sleeps, so the runner needs a test-only override
    for a file's measured seconds (fixture-controlled), with the production path unaffected.
    Rejected: real sleeps scaled down — sub-second wall-clock measurement is noise-dominated and
    the suite's own budget discipline (this change's subject) forbids multi-second fixtures.
