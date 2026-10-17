<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0557 — Retire the suite runner's unused strict budget mode](../../changes/active/0557-retire-the-suite-runner-s-unused-strict-budget-mode.md)**
<!-- docket:backlink:end -->

# Retire the suite runner's unused strict budget mode — design

Change #557, groomed 2026-10-17.

## Problem

The suite runner (`docket development test`, `internal/suiterunner`) has an opt-in strict budget
mode. Setting `DOCKET_RUNTESTS_STRICT=1` makes it re-check every current screening candidate solo
right away (`StrictConfirmCandidates` in `internal/suiterunner/budgetstate.go`) and fail an
otherwise-green run with exit 4 on a confirmed breach, a failed re-check, or a not-counted reading.
It also turns a direct `OVER BUDGET:` crossing into exit 4.

Nothing turns it on. The only reader of the variable is `runDevelopmentTest` in
`internal/cli/development_test_cmd.go`. No gate, `.docket.yml` key, skill, or CI job sets it, and no
code reads exit 4. The release-candidate workflow gets a failing budget check its own way, by
grepping the log for `^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):`. Yet every budget change still
has to design around the mode: #528 had to decide how strict should treat a contended reading.

Its user-facing text is also stale. On a green run with a direct breach, the report says
`Set DOCKET_RUNTESTS_STRICT=1 to gate on it — but see scripts/run-tests.md first: …`, and that file
no longer exists. The strict note says `Strict: --strict-budget was given …`, a flag the Go runner
does not have.

## Goals

- Strict mode is gone: no env seam, no config field, no code path, no exit 4, no tests, no docs.
- Every budget line the runner prints names only things that exist.
- The direct reading on a single-job run or a serial-mode row stays, is pinned by a test, and is
  described correctly in `tests/README.md`.

## Non-goals

- The scheduled solo re-check and its not-counted handling (#528), the screening regime, the
  thresholds, and the budget-state store format.
- Host-relative budgets (#273).
- Peer detection for the direct reading (see *Known limits*).
- The release-candidate workflow's greps. Only its comments change.
- Point-in-time records that mention strict mode: plans, specs, results files, ADRs, and the frozen
  `testdata/repositories/status-corpus/` fixture (its `.docket.yml` comment names `--strict-budget`
  and `scripts/run-tests.md` as part of a recorded past repository state).

## Anti-goals

- Removing the direct reading along with strict. It stays (decision below).
- Any budget reading that fails a run. After this change every budget finding exits 0.

## Decision: the direct reading stays

On a single-job run (`DOCKET_RUNTESTS_JOBS=1`) or for a target whose budget row is `serial`, `Run`
compares the measured time straight against `ceiling * 3/2` (`SoloOver`). A crossing sets
`TargetOutcome.OverDirect`, tags the report row `OVER BUDGET (ceiling Ns)`, and prints the
`OVER BUDGET:` block (names, the Remedy line, and the posture note). The run exits 0.

It stays because it is the only budget check on those two paths. Screening and the scheduled
re-check run only when `cfg.Jobs > 1`, and a serial-mode target is never screened. Without the direct
reading, a single-job run and a serial-mode row would carry ceilings that nothing checks. Retiring it
cleanly would also mean retiring serial mode and the single-job budget path, a larger change. It is
also how a quiet serial pass shows which files are over budget, the method #507 and #273 used to
measure by hand.

Today nothing exercises it in CI: the release workflow runs with jobs = CPU count, and no serial rows
exist (the last one was retired in change 0333).

## Design

### 1. Runner code

Remove:

- `internal/cli/development_test_cmd.go`: the `DOCKET_RUNTESTS_STRICT` read and the `Strict:` field
  in the `suiterunner.Config` literal. The seam comment ("JOBS/STATE/STRICT tune the run") drops
  STRICT.
- `internal/suiterunner/schedule.go`: `Config.Strict`.
- `internal/suiterunner/run.go`: the strict case of the post-report budget switch, the
  `strictCandidates` count, the `cfg.Strict && tally.OverDirect > 0` arming, and `strictArmed`. The
  scheduled case (`cfg.Jobs > 1 && clean`) is unchanged. `Run` ends with
  `ExitCode(tally, len(unknown))`. The header's exit contract becomes precedence 1 > 3 > 0 with
  0 / 1 / 2 / 3 / 130 / 143. The comments above the budget machinery that say "only a
  strict-confirmed breach arms exit 4" and "gates the run only under --strict" are rewritten to say
  budget findings never change the exit code.
- `internal/suiterunner/aggregate.go`: `RenderReport` loses its `strict` parameter and the
  `case strict:` branch. The default branch keeps the `Advisory: the tests all passed, so this run
  does not fail on the breach (exit 0).` line and drops the `Set DOCKET_RUNTESTS_STRICT=1 …` line and
  the comment block that explains it. The red-branch notes (exit 1, exit 3) are unchanged.
  `ExitCode` loses its `strictArmed` parameter: precedence 1 > 3 > 0. Doc comments on both, and the
  file header's "(precedence 1 > 3 > 4 > 0)", drop strict and exit 4.
- `internal/suiterunner/budgetstate.go`: delete `StrictConfirmCandidates`. The file header drops the
  "--strict re-measures every current candidate immediately and fails closed (exit 4)" clause, and
  any other comment that names the strict path (for example the `candidates` field comment) drops
  it.
- Anything else that only strict used.

Find the leftovers with a whole-repo grep, never a hand list (`AGENTS.md`): `Strict`, `strict`,
`DOCKET_RUNTESTS_STRICT`, `strictArmed`, `exit 4`, `--strict-budget`, `strict-budget`,
`run-tests.md`, and the precedence spelling `3 > 4`. Sort the hits into maintained source (fix) and point-in-time records (leave). Most
`strict` hits in the repo are unrelated words ("strictly", strict parsing); only the budget-mode ones
matter.

### 2. Tests

Remove:

- `internal/suiterunner/run_test.go`: the `strict` subtest of `TestRunOwnSignIsNotContention`. Its
  scheduled subtest stays, and its doc comment ("on both confirmation paths") is rewritten for the one
  path left.
- `internal/suiterunner/budgetstate_test.go`: `TestStrictConfirmsAllAndArms` and
  `TestStrictNotCountedArms`.
- `internal/suiterunner/aggregate_test.go`: `TestStrictBreachRendersStrictNote`. The `ExitCode`
  table drops its `strictArmed` column and any row whose only point was exit 4.

Update every `RenderReport` and `ExitCode` caller for the new signatures, including
`TestScreeningNeverAuthoritative` in `budgetstate_test.go`.

Add, each mutation-proved (strip the behavior, watch it go red, restore):

1. **The stale hint is gone.** Extend `TestAdvisoryBreachExitsZeroWithLoudReport` (or add a test
   beside it): the report contains none of `DOCKET_RUNTESTS_STRICT`, `scripts/run-tests.md`,
   `--strict-budget`, `exit 4`. Mutation: put the old hint line back.
2. **The direct reading through `Run`.** No `Run`-level test covers it today. With `Jobs: 1` and one
   passing target whose injected duration (the `DOCKET_RUNTESTS_TEST_DURATIONS` seam, column 2, via
   the existing `writeDurations` helper) is over `ceiling * 3/2` (over 90 with no budget row, since
   the default ceiling is 60): the report has the
   `OVER BUDGET (ceiling Ns)` row tag and an `OVER BUDGET:` line, no `SERIAL CONFIRMATION` line, and
   `Run` returns 0. Mutation: delete the `cfg.Jobs == 1 || o.Target.Mode == ModeSerial` branch in
   `Run`. This pins the decision above.

No test for "setting `DOCKET_RUNTESTS_STRICT=1` does nothing": nothing reads the variable, so the
test would be decoration.

### 3. Docs and comments

- `tests/README.md`, the budget paragraph under *Running it*. Say:
  - A parallel run (the default) screens each file against `ceiling * 5/2` (`BUDGET WATCH:` /
    `PARALLEL-SENSITIVE:`), and after repeated overruns schedules a solo re-check. Only a re-check
    over `ceiling * 3/2` with no other suite running is an authoritative breach
    (`SERIAL CONFIRMED OVER BUDGET:`). The `SERIAL CONFIRMATION NOT COUNTED:` sentence stays as it is,
    minus its strict clause.
  - A single-job run (`DOCKET_RUNTESTS_JOBS=1`) or a file whose budget row is `serial` is measured
    directly against `ceiling * 3/2`, and a crossing prints `OVER BUDGET:`. This corrects today's
    text, which wrongly says a single-job breach prints `SERIAL CONFIRMED OVER BUDGET:`.
  - No budget finding fails a run: a run with only budget findings exits `0`.
  - The exit list: `0`, `1`, `2`, `3`, `130`/`143`. Remove exit `4`. Also remove the "Exit `5` … is
    **retired** (change 0370)" paragraph: docs describe current behavior only. The *Backticks in test
    source* section stands on its own; its parenthetical about change 0370 retiring the old exit-5
    preflight goes too, since nothing else in the doc mentions exit 5.
- `docs/reference/glossary.md`, entry *Budget watch / serially confirmed breach*: "None of these lines
  fails a default run" becomes "None of these lines fails a run".
- `AGENTS.md`, the "Read the budget report" rule: "Neither fails the run by default" becomes "Neither
  fails the run". `CLAUDE.md` is a symlink to `AGENTS.md`, so this is one edit. The human approved this
  edit at grooming.
- `.github/workflows/release-candidate.yml`, the budget-classification step. Two comments, no grep
  change:
  - The comment that calls the two greps "the runner's complete budget vocabulary" says instead that
    they cover the screening and authoritative lines, and that `SERIAL CONFIRMATION NOT COUNTED:` is
    left out on purpose because it is meant to fail nothing.
  - The comment "The suite itself exits 0 on an advisory breach (no --strict-budget), so an
    otherwise-green gate must fail here or nothing fails at all" drops the "(no --strict-budget)"
    aside and says the suite exits 0 on every budget finding.

Comment cross-references anchor on symbol names, never line numbers.

## Testing

Run the whole suite at the build gate and read the budget report. The suiterunner tests' budget row
must still fit: this removes three tests and a subtest and adds two small ones.

## Known limits

- The direct reading does not check for another suite running. #528's sign scan covers only the
  scheduled re-check. A single-job run taken while another worktree runs its suite can print a false
  `OVER BUDGET:` line. It exits 0 and no gate runs single-job, so the cost is a misleading line on a
  hand run. Accepted.
- The release workflow's `OVER BUDGET` grep can only fire on a single-CPU runner or after a serial
  row is added. It stays as it is.

## Left to the builder

- Whether `Tally.OverDirect` survives. Nothing in production reads the count once the strict arming
  goes; the row tag and the `OVER BUDGET:` block use `TargetOutcome.OverDirect`.
- Whether `run.go`'s header notes that exit 4 is retired, as it already does for exit 5. Code comments
  may; `tests/README.md` must not.
- Exact comment and README wording, test names, and where the new `Run` test lives.

## Alternatives considered

- **Retire the direct reading too.** Single-job runs and serial-mode rows would carry ceilings nothing
  checks, so a clean retirement would also take out serial mode and the single-job budget path. That
  is a larger change, and it would remove the one way to see over-budget files from a quiet serial
  pass.
- **Keep strict mode and only fix its stale text.** Nobody uses it, and each budget change still has
  to design around it.
- **Make the direct reading fail the run by default.** A wall-clock reading on an unknown host is not
  a test failure (ADR-0074's reasoning), and docket adds no new blocking gates for visibility checks.
