<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0273 — Put runtime budgets on a host-relative basis and re-seed the table](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0273-put-runtime-budgets-on-a-host-relative-basis-and-re-seed-the.md)**
<!-- docket:backlink:end -->

# Put runtime budgets on a host-relative basis and re-seed the table — design

Change: docs/changes/active/0273-put-runtime-budgets-on-a-host-relative-basis-and-re-seed-the.md
Depends on #0251 (screen-then-serial-confirm regime); this change lands ON TOP of that regime and
is the "serial-canary rescale" follow-up 0251's spec names in its assumption 2. Designed against
the post-0251 shape: parallel 5/2 comparison = screen only, serial re-measure vs `ceiling * 3/2` =
verdict.

## Problem (verified 2026-08-08, method in the change's Why)

`tests/runtime-budgets.tsv` rows encode the calibration host's **absolute speed**. Eight files
breach on an untouched merge-base with serial ratios clustering 1.05–1.25 — the signature of
uniform host drift, not of any file re-shaping. The table's own remedy prose forbids bumping rows
for host slowdown, so the breaches can be neither fixed nor ignored: the noise devalues every
`OVER BUDGET:` trailer and the AGENTS.md posture ("a finding to act on") stops being followable.

## Design

### 1 — Run-time serial canary; ratio-scaled ceilings at the verdict

- **Canary workload.** A dedicated, deliberately boring script `tests/lib/budget-canary.sh` whose
  cost profile matches what the suite actually spends time on — process forks, `git init/commit`
  cycles in a scratch dir, and file I/O — sized to run in roughly 3–5 seconds on the current host.
  It is NOT a test file: its cost changes only when it is deliberately edited, never as a side
  effect of coverage growth.
- **Calibration value lives in the TSV header** as a structured comment line the runner parses
  (e.g. `# canary: <seconds-in-milliseconds-resolution> seeded <UTC date> host <model/cores>`),
  alongside the existing seeding prose. Seeded in this change from the same quiet run that
  re-seeds the rows, so rows and calibration share one basis by construction.
- **The canary times itself, sub-second** (assumption 10): it prints its own measurement in
  milliseconds (direct `EPOCHREALTIME` under Bash 5+, else a ≥10s multi-pass loop reporting
  ms-per-pass), and the same standalone invocation is how the calibration is captured at seeding
  time (assumption 12). A budgets table with no calibration line runs unscaled with one loud
  informational line (assumption 11).
- **Ratio.** At verdict time the runner runs the canary once serially and computes
  `ratio = canary_measured / canary_calibrated` (integer fixed-point at ms resolution,
  truncating — fail-closed, assumption 13). Every ceiling is scaled by the ratio **at comparison time only** — the table's stored
  values never change per-host: the confirm verdict becomes
  `serial_measured > ceiling * ratio * 3/2` (and the direct `-j 1` comparison likewise). The
  property the table exists to catch — a file that doubles *its own* cost — still reddens,
  because the canary did not double with it; the property it accidentally encodes — the 0227
  host being faster than today's — cancels.
- **When it runs: lazily, at most once per run — triggered at the clamp floor.** The canary need
  only run when a scaled comparison could possibly trip, and the clamp band bounds where that is:
  a measurement is a *possible* candidate iff it exceeds its threshold scaled by the clamp floor
  (`ceiling * 0.5 * 5/2` in the parallel phase; `ceiling * 0.5 * 3/2` at `-j 1`). When any
  measurement crosses that floor-scaled pre-screen, the canary runs once, serially, before the
  confirm phase (machine idle by 0251's construction; at `-j 1`, before the comparison loop), and
  the **measured ratio then scales both the 5/2 screen and the 3/2 confirm verdict**
  (`measured > ceiling * ratio * 5/2` screens; `serial_measured > ceiling * ratio * 3/2`
  confirms). Scaling the screen is what makes the down direction reachable on a fast host — with
  an unscaled screen, ratio < 1 could never screen anything and the tightened verdict would be
  dead code at `-j > 1`; a too-tight screen costs only one bounded re-run (0251's own argument).
  A comfortably-green run — every measurement under half its ceiling's screen threshold — never
  pays the canary.
- **Clamp band with a named anomaly.** The ratio is honored inside `[0.5, 3.0]`. Outside the band
  the measurement is presumed bogus (a canary that hit a swap storm, a broken edit to the canary
  itself): the runner falls back to `ratio = 1`, prints one named `CANARY ANOMALY` line (measured,
  calibrated, band), and — mirroring 0251's unconfirmed-candidate posture — every candidate
  decided under an anomalous canary is reported `unconfirmed` rather than cleared: fail-closed
  (breach) under `--strict-budget`, advisory-reported at the default. **The anomaly-time
  candidate set is the clamp-floor pre-screen set** (every measurement above
  `ceiling * 0.5 * slack`), not the ratio-1 set: a measurement that could only breach under a
  ratio < 1 must not be silently cleared by the ratio-1 fallback — fail-closed on the whole set
  the band could reach (critic-identified; adopted per its stated fail-closed default).
- **Exit contract byte-compatible.** 0/1/3/4/2 unchanged; advisory-by-default unchanged;
  `--no-budget-check` skips canary, screen, and confirm alike; `--strict-budget` unchanged in
  meaning. No new flags by default; the canary is not user-tunable.

### 2 — Re-seed the whole table once, and record the conditions

- Re-seed **every row** — not only the eight breachers — from a measured quiet serial run
  (`run-tests.sh -j 1 --no-budget-check --timings`, three consecutive passes) on the current
  host, applying the unchanged sizing rule (next multiple of 5 plus 5s margin, min 10s). Most
  floor rows stay 10; the drifted above-floor rows land where the host actually is. The canary is
  calibrated from the same session, so table basis and canary basis are one measurement.
- Record the seeding conditions in the TSV header: date, host identity (model + core count), the
  three-pass method, and the canary calibration — so the next drift is diagnosable against a
  stated baseline instead of re-litigated.
- `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` re-seeded with a dated comment naming this
  as a third legitimate move-case: **basis re-seed** (whole-table, host-relative migration, this
  change) — added once to the remedy prose of assertion (5) and the TSV header, so a future
  whole-table re-seed is recognized as a named case, not an evasion. The remedy hierarchy is
  unchanged: for a single drifting file, shard — a basis re-seed is only legitimate together with
  a canary recalibration in the same diff, which is what keeps it from being "raise the number"
  wearing a new name.

### 3 — Guards and mutation proof

- `tests/test_runtime_budgets.sh` gains: the TSV header carries a parseable canary calibration
  line (presence + integer shape); `tests/lib/budget-canary.sh` exists and is executable; the
  runner references both (wiring, same shape as its existing assertion (6)).
- Mutation proofs in `tests/test_run_tests.sh`, driven through `--budgets` fixtures carrying a
  fixture canary line:
  - a file at ~2x its unpadded fixture row **confirms OVER BUDGET** when the canary measures at
    calibration (ratio ~1) — the check still reddens on genuine self-doubling;
  - a healthy near-ceiling file on a simulated slow host (fixture calibration set well below what
    the canary will measure, driving ratio ~2) is **cleared** by the scaled verdict — the
    host-drift false positive this change exists to kill;
  - a ratio outside the clamp band produces the `CANARY ANOMALY` line, `unconfirmed` reporting,
    exit 4 under `--strict-budget`, exit 0 at the default;
  - `--no-budget-check` runs no canary (no anomaly line, no scaled comparison).
  Each proof is mutation-tested per the AGENTS.md bar: strip the scaling (or the anomaly branch)
  and watch the assert redden.
- Docs move in the same change: `run-tests.sh`'s budget comment block, `run-tests.md`'s budget
  sections (the "calibrated to one machine" caveat is rewritten — the strict path becomes honest
  on both the contention axis (0251) and the speed axis (this change)), the TSV header, and the
  stale caveat line the runner prints after an advisory breach ("the slack factor is calibrated
  to one machine") is retired or repointed.

## Out of scope

- The contention axis, the confirm-pass mechanics, and the config-suite shard — all #0251.
- The advisory-by-default posture and the 0/1/3/4 exit contract.
- Sharding any file; per-host stored calibration state (none is introduced — the canary is
  measured fresh each run).

## Assumptions

1. **Mechanism: run-time serial canary, not stored per-host calibration and not re-seed-only.**
   Chosen: measured fresh each run, no per-host state — which is also what makes the CI open
   question moot by construction (an ephemeral CI host calibrates itself every run). Rejected:
   re-seed-only — the stub's own evidence shows drift is recurring (20–25% swings on one commit
   in hours), so a value-only fix re-litigates within months, which is the exact failure named in
   the Why. Rejected: a stored per-host calibration file — state to manage, wrong on shared/CI
   runners, and 0251 already rejected that shape when parking this follow-up. This is the
   direction 0251's spec names verbatim ("a serial-canary rescale is the named follow-up shape"),
   so the house preference is stated, not invented here.
2. **Canary is a dedicated workload script, not an existing test file.** Chosen because a test
   file's cost moves when its coverage moves, silently recalibrating every ceiling; a dedicated
   script changes only deliberately, and a change to it must re-seed the calibration in the same
   diff (guarded by the header line living next to the rows). Single-point-estimator risk (one
   canary's fork/IO profile standing for every file) is acknowledged from 0251's rejection
   rationale and bounded three ways: the profile is chosen to match the suite's dominant cost
   (forks + git + IO, not pure CPU), the 3/2 slack and 5–10s seed padding absorb residual
   mismatch, and the clamp band caps how far a bad reading can move any verdict. Rejected:
   averaging multiple canaries — complexity for a risk the band already bounds.
3. **Both the 5/2 screen and the 3/2 verdict are ratio-scaled once a canary reading exists.**
   (Revised from "screen stays unscaled" after critique: an unscaled screen makes ratio < 1
   unreachable at `-j > 1` — nothing ever screens on a fast host, the confirm never runs, and the
   guard stays exactly as vacuous on fast hardware as run-tests.md documents today. That defect
   is the half of the design's value assumption 4 stakes out.) The screen remains a contention
   filter — the ratio scales its baseline, the 5/2 covers contention on top — so the axes stay
   separate in meaning while sharing the basis. A too-tight scaled screen costs one bounded
   confirm re-run, 0251's own accepted cost. Rejected: unscaled screen (above); rejected:
   distinct screen/verdict ratios — one measurement, one basis.
4. **Ratio honored in both directions, inside a [0.5, 3.0] clamp.** Downward scaling (a faster
   host tightening effective ceilings) is what keeps the guard from going vacuous on fast
   hardware — the silent failure mode the calibration learning names as the one that kills a
   check. The band bounds bogus readings; outside it the run falls back to ratio 1 with a named
   anomaly and fail-closed strict behavior (mirrors 0251's assumption 10: a bogus measurement
   never clears). Rejected: floor at 1.0 (never tighten) — preserves today's vacuous-on-fast-hosts
   defect; rejected: unclamped — one swap-storm reading could 10x every ceiling.
5. **Lazy canary, triggered by a clamp-floor pre-screen.** (Revised after critique: the original
   trigger — "a run with no screened candidate never pays it" — keyed the canary's existence on
   the unscaled screen, making downward scaling dead code independently of assumption 3.) The
   trigger is: any measurement exceeding its threshold scaled by the clamp floor 0.5 — the
   tightest verdict the band permits — which is exactly the set of runs where a scaled comparison
   could trip. This preserves zero cost on comfortably-green runs in both directions, and the
   reading is taken at confirm time, idle by 0251's construction, in the same conditions as the
   serial re-measures it scales. At `-j 1` the pre-screen applies identically before the
   comparison loop. Rejected: always-at-start — pays seconds on every run and measures a
   differently-loaded machine than the confirms run on; rejected: canary-on-screened-candidate
   (the original) — unreachable down direction.
6. **Whole-table re-seed, once, in this change.** Chosen so rows and canary calibration share one
   measurement session — re-seeding only the eight breachers would leave the table on two bases
   and make the ratio wrong for one of them. Sizing rule unchanged; most floor rows are expected
   to stay at 10, so the diff is small and reviewable. The "raise = evasion" guard is preserved
   by naming the basis-re-seed case (always row-edits + canary recalibration together, stated in
   assertion (5)'s remedy and the TSV header) rather than by weakening any counter. Rejected:
   partial re-seed (two bases); rejected: leaving values and letting the ratio absorb the known
   drift — it can, but then the recorded basis is a host nobody can re-measure against, and the
   next calibration question is unanswerable.
7. **Exit contract and posture untouched.** Same rationale as 0251's assumption 1
   (bare-non-zero callers; learning `exit-code-encodes-a-non-failure`); this change alters what a
   breach measures, never what it exits.
8. **Dependency state: #0251 is groomed but not built.** This spec is written against 0251's
   settled spec, not against merged code; if 0251's build lands with drift in the confirm-pass
   mechanics, implement-next's reconcile pass re-validates this design against what actually
   merged (learning `moving-base`). Build order is enforced by `depends_on: [251]`. No coupling
   to #0258 (different files). #0229's learnings are cited; it is consolidated into 0251 and
   needs no link beyond the existing `related`.
9. **The re-seed measurements are build-time work, not spec-time constants.** The table values
   this change ships are whatever the quiet three-pass run measures on the build host at build
   time — the spec fixes the method and the sizing rule, deliberately not the numbers (learning
   `optimization-needs-a-measured-oracle`: a performance artifact is accepted on measured wall
   clock, and this spec's job is to pin the measurement protocol).
10. **Timing resolution: the canary self-times sub-second, with a duration-floor fallback.**
    (Surfaced by critique: the runner clocks files with `date +%s`, and a 3–5s canary measured at
    whole-second granularity carries ±25–30% quantization — the size of the drift being
    cancelled, silently inside the clamp band.) The canary script times ITSELF and prints its
    measurement in milliseconds: under Bash 5+ (`EPOCHREALTIME` present) it times one pass
    directly; otherwise it loops passes until at least 10 whole seconds have elapsed by
    `date +%s` and reports elapsed-ms-per-pass from the loop total, bounding quantization at
    ~10%, inside the headroom the 3/2 slack plus seed padding already carry. The runner never
    times the canary externally — the value it consumes is the canary's own printed ms figure,
    the same format as the TSV calibration line. Rejected: `date +%N` (BSD date lacks it);
    rejected: requiring Bash 5 (the runner's floor is 4.3 and stays there); rejected: accepting
    whole-second timing of a short canary (quantization comparable to the signal).
11. **A budgets table without a calibration line runs unscaled, loudly.** When the resolved
    `--budgets` table has rows but no canary line, the runner prints one informational
    `CANARY: no calibration in <table>; comparisons unscaled` line and behaves pre-0273
    (ratio 1, ordinary — not `unconfirmed` — verdicts): fixtures and consuming repos that never
    opted into calibration keep exactly today's semantics. What prevents silent disarming of the
    repo's own table is the pairing: the printed line makes the state visible in every run's
    output, and `tests/test_runtime_budgets.sh` pins the line's presence in the repo TSV, so
    deleting it reddens the suite. Rejected: silent ratio-1 (the critique's disarm-by-deletion
    hole); rejected: treating a missing line as an anomaly/fail-closed (it would break every
    existing `--budgets` fixture and any downstream table for a feature they never adopted).
12. **Calibration is captured by the canary itself, standalone, in the seeding session.** The
    seeding run (`-j 1 --no-budget-check --timings`) deliberately skips the canary, so the
    calibration cannot come from it: the build task invokes `tests/lib/budget-canary.sh` directly
    — it prints its measurement in TSV-header-line format — three times in the same session as
    the three seeding passes, and records the median in the header. One session, one basis; no
    new runner flag. Rejected: a runner `--calibrate` mode (a flag whose only caller is one build
    task); rejected: capturing during a budget-checked run (circular — the run would consume the
    calibration it is producing).
13. **Fixed-point arithmetic truncates, which is fail-closed, and the proofs sit off the
    boundary.** The scaled threshold (`ceiling * ratio * slack`, ms-resolution fixed point)
    truncates toward zero, making every comparison marginally tighter — errors surface as a rare
    extra confirm or a boundary breach, never as a silently cleared one. Mutation-proof fixtures
    are specified at ratios ≥2x away from any threshold so a one-ms rounding step cannot flip a
    proof. Rejected: round-half-up (fail-open on the breach side); rejected: leaving it to the
    implementation (the critique's point — it flips verdicts exactly at the boundary).
