<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0557 — Retire the suite runner's unused strict budget mode](../changes/active/0557-retire-the-suite-runner-s-unused-strict-budget-mode.md)**
<!-- docket:backlink:end -->

# Results — Retire the suite runner's unused strict budget mode

**Human action:** None needed. The change removes an unused mode, the full suite passed, and the review found nothing.

## Outcome

The suite runner no longer has a strict budget mode. The `DOCKET_RUNTESTS_STRICT` variable, `Config.Strict`, `StrictConfirmCandidates`, and exit code 4 are gone, along with the strict tests and the report line that pointed at the deleted `scripts/run-tests.md`. No budget finding changes a run's exit code now: the runner exits 1 for a failing test, 3 for an unknown outcome, and 0 otherwise.

The direct check on a single-job run or a serial-mode budget row is unchanged. It still prints `OVER BUDGET:` and exits 0. The new `TestRunSingleJobDirectReading` pins it, and a mutation probe showed the test fails when that check is removed. `TestAdvisoryBreachExitsZeroWithLoudReport` now also fails if the report mentions `DOCKET_RUNTESTS_STRICT`, `scripts/run-tests.md`, `--strict-budget`, or `exit 4`.

`tests/README.md`, the glossary, `AGENTS.md` ("by default" removed), and the comments in `.github/workflows/release-candidate.yml` now describe current behavior. The workflow's greps did not change.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the branch head through the build gate. The budget report had only screening lines (`BUDGET WATCH:` / `PARALLEL-SENSITIVE:`) for integration test files this change does not touch, on a machine running 11 jobs in parallel. It had no `SERIAL CONFIRMED OVER BUDGET:` line.
- Focused runs of the suiterunner and cli packages passed, with mutation probes on both new or extended guards.
- A standard-tier whole-branch review found nothing to fix.
