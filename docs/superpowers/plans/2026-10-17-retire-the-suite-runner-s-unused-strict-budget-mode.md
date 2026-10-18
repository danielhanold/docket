<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0557 — Retire the suite runner's unused strict budget mode](../../changes/archive/2026-10-18-0557-retire-the-suite-runner-s-unused-strict-budget-mode.md)**
<!-- docket:backlink:end -->

# Retire the suite runner's unused strict budget mode — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build`, task by task, under the
> `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the suite runner's strict budget mode (`DOCKET_RUNTESTS_STRICT`, `Config.Strict`,
`StrictConfirmCandidates`, exit 4, the strict note and the stale hint) so that no budget finding
ever changes a run's exit code, while keeping and pinning the direct reading on single-job runs and
serial-mode rows.

**Architecture:** Strict mode is a dead opt-in branch threaded through four files of
`internal/suiterunner` plus one env read in `internal/cli`. Task 1 pins the direct reading that must
survive (a `Run`-level characterization test). Task 2 deletes strict end to end in one atomic step,
because removing the `strict`/`strictArmed` parameters breaks every caller at once. Task 3 corrects
the docs and comments that describe the mode.

**Tech Stack:** Go (`internal/suiterunner`, `internal/cli`), Markdown docs, a GitHub Actions
workflow (comments only).

**Spec:** `docs/superpowers/specs/2026-10-17-retire-the-suite-runner-s-unused-strict-budget-mode-design.md`
(on the `docket` metadata branch; the builder reads it from the metadata checkout at
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/`).

All paths below are relative to the feature worktree
`/Users/homer/dev/docket/.worktrees/retire-the-suite-runner-s-unused-strict-budget-mode`.

## Global Constraints

- After this change **every budget finding exits 0**. No new exit code, no new failing path.
- The direct reading stays: `cfg.Jobs == 1 || o.Target.Mode == ModeSerial` → `SoloOver(secs, ceil)`
  → `TargetOutcome.OverDirect`, row tag `OVER BUDGET (ceiling Ns)`, `OVER BUDGET:` block, exit 0.
- The scheduled re-check (`cfg.Jobs > 1 && clean`), screening thresholds, `SERIAL CONFIRMATION NOT
  COUNTED:` handling, and the budget-state store format are unchanged.
- The release-candidate workflow's greps do not change; only two comments do.
- Do not edit point-in-time records: `docs/results/`, `docs/superpowers/`, `docs/adrs/`,
  `docs/changes/`, and the frozen `testdata/repositories/status-corpus/` fixture (its `.docket.yml`
  comment naming `--strict-budget` / `scripts/run-tests.md` stays).
- `tests/README.md` describes current behavior only: no "retired" paragraphs, no exit 4, no exit 5.
  Code comments may note that exit 4 is retired.
- Comment cross-references anchor on symbol names, never line numbers.
- Every Go verification and mutation probe uses `go test -count=1` (the Go test cache can serve a
  pre-mutation pass).
- Mutation restore uses a backup copy (`cp f f.bak; <mutate>; <run>; mv -f f.bak f`), never
  `git checkout --` (that discards the uncommitted edit under test).
- Unrelated `strict` words (`strictly`, "the strict read" in `results.go`, `relUnderRoot`'s
  "strictly inside") are not budget-mode hits; leave them.

## Builder decisions already made (spec "Left to the builder")

- **`Tally.OverDirect` survives.** `RenderReport` keeps counting it; `ExitCode` never reads it. The
  `TestExitPrecedence` row `{"advisory breach clean", Tally{OverDirect: 1}, 0, 0}` stays as the guard
  that a direct crossing never changes the exit code.
- **`run.go`'s header notes exit 4 is retired**, in one short paragraph beside the existing
  "EXIT 5 IS RETIRED" note (code comments may; `tests/README.md` must not).
- **The new `Run` test lives in `internal/suiterunner/run_test.go`** as
  `TestRunSingleJobDirectReading`.

## Review Focus

1. **A serial-mode budget row on a parallel run** — must still get the direct reading (exit 0,
   `OVER BUDGET:`), not screening. Covered by the unchanged branch condition; Task 1's mutation
   deletes the whole branch, so this rides on the same guard. No serial rows exist today
   (`TestAdvisoryBreachExitsZeroWithLoudReport` uses `ModeSerial` at the render level).
2. **A red run that also crosses a budget** — the `OVER BUDGET:` block must still render the exit-1
   / exit-3 note, never the advisory line. Unchanged red-branch notes; existing
   `aggregate_test.go` coverage stays. Confirm Task 2 does not reorder the `switch`.
3. **`DOCKET_RUNTESTS_STRICT=1` left in a user's shell** — must do nothing. Nothing reads it after
   Task 2; no test (it would be decoration, per spec). The Task 2 sweep grep proves no reader is left.
4. **A scheduled re-check over the line on a parallel run** — still prints `SERIAL CONFIRMED OVER
   BUDGET:` and exits 0. `TestRunOwnSignIsNotContention`'s surviving `scheduled` subtest covers it.
5. **Stale wording elsewhere naming the mode** — every maintained hit of the sweep patterns is fixed;
   Task 3's final whole-repo grep must return only point-in-time records and unrelated `exit 4`
   literals in shell fixtures (`tests/test_release_downloader*.sh`, `tests/test_install_bootstrap.sh`).

---

### Task 1: Pin the direct reading through `Run`

**Files:**
- Test: `internal/suiterunner/run_test.go` (add `TestRunSingleJobDirectReading`)

**Interfaces:**
- Consumes (existing helpers): `devFixture(t, map[string]string) string`,
  `runCfg(t, testsDir, out, errBuf *bytes.Buffer) Config` (Jobs 2, no StatePath),
  `writeDurations(t, [][3]string) string` (in `budgetstate_test.go`; row is
  `base.sh, parallel-secs, solo-secs`; `Run` reads column 2), `DefaultCeiling = 60`.
- Produces: nothing for later tasks; the test must stay green through Task 2.

This is a characterization test: it passes on today's code, and its mutation proves it guards the
direct reading the spec keeps.

- [ ] **Step 1: Write the test**

```go
// TestRunSingleJobDirectReading pins the direct budget reading a single-job run
// gets: no screening and no solo re-check run on this path, so the measured time
// is compared straight against ceiling*3/2. A crossing is loud in the report and
// still exits 0 — no budget finding changes the exit code.
func TestRunSingleJobDirectReading(t *testing.T) {
	dir := devFixture(t, map[string]string{"slow": "echo 'ok - slow'\n"})
	var out, errBuf bytes.Buffer
	cfg := runCfg(t, dir, &out, &errBuf)
	cfg.Jobs = 1
	// No budget row: default ceiling 60, direct line 90. 100 crosses it.
	cfg.DurationsPath = writeDurations(t, [][3]string{{"test_slow.sh", "100", "100"}})

	code := Run(context.Background(), cfg)
	report := out.String()
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, report, errBuf.String())
	}
	if !strings.Contains(report, "OVER BUDGET (ceiling 60s)") {
		t.Fatalf("want the OVER BUDGET row tag:\n%s", report)
	}
	if !regexp.MustCompile(`(?m)^OVER BUDGET: test_slow$`).MatchString(report) {
		t.Fatalf("want an OVER BUDGET: line naming test_slow:\n%s", report)
	}
	if strings.Contains(report, "SERIAL CONFIRMATION") {
		t.Fatalf("a single-job run takes no solo re-check:\n%s", report)
	}
}
```

- [ ] **Step 2: Run it — expect PASS on current code**

Run: `go test -count=1 -run 'TestRunSingleJobDirectReading' ./internal/suiterunner/`
Expected: `ok`

- [ ] **Step 3: Mutation-prove it**

In `internal/suiterunner/run.go`, back up the file, then change
`if cfg.Jobs == 1 || o.Target.Mode == ModeSerial {` to `if false {`:

```bash
f=internal/suiterunner/run.go; cp "$f" "$f.bak"
# edit: replace the direct-reading condition with `false`
go test -count=1 -run 'TestRunSingleJobDirectReading' ./internal/suiterunner/   # expect FAIL (no row tag)
mv -f "$f.bak" "$f"
go test -count=1 -run 'TestRunSingleJobDirectReading' ./internal/suiterunner/   # expect ok
```

Expected: FAIL with "want the OVER BUDGET row tag" under the mutation; `ok` after restore. Confirm
`git diff --stat` shows only `run_test.go` changed.

- [ ] **Step 4: Commit**

```bash
git add internal/suiterunner/run_test.go
git commit -m "test(suiterunner): pin the direct budget reading on a single-job run"
```

---

### Task 2: Remove strict mode from the runner

**Files:**
- Modify: `internal/suiterunner/aggregate.go` (`RenderReport`, `ExitCode`, file header)
- Modify: `internal/suiterunner/run.go` (`Run` budget block, header exit contract)
- Modify: `internal/suiterunner/budgetstate.go` (delete `StrictConfirmCandidates`; header and
  `budgetState` comment)
- Modify: `internal/suiterunner/schedule.go` (delete `Config.Strict`)
- Modify: `internal/cli/development_test_cmd.go` (delete the `Strict:` field and the env read; seam
  comment)
- Test: `internal/suiterunner/aggregate_test.go`, `internal/suiterunner/budgetstate_test.go`,
  `internal/suiterunner/run_test.go`

**Interfaces:**
- Consumes: Task 1's `TestRunSingleJobDirectReading` (must stay green).
- Produces (new signatures):
  - `func RenderReport(w io.Writer, outcomes []TargetOutcome, unknown []string, wall int, verbose bool, logsDir string) Tally`
  - `func ExitCode(t Tally, unknownCount int) int` — precedence 1 > 3 > 0.
  - `Config` has no `Strict` field.
  - `(*budgetState).StrictConfirmCandidates` no longer exists. `soloConfirm` and `notCountedLine`
    stay (used by `ScheduleConfirmation`).

- [ ] **Step 1: Write the failing assertion (the stale hint is gone)**

In `TestAdvisoryBreachExitsZeroWithLoudReport` (`aggregate_test.go`), after the `want` loop, add:

```go
	for _, stale := range []string{"DOCKET_RUNTESTS_STRICT", "scripts/run-tests.md", "--strict-budget", "exit 4"} {
		if contains(out, stale) {
			t.Fatalf("report names %q, which no longer exists:\n%s", stale, out)
		}
	}
```

- [ ] **Step 2: Run it — expect FAIL**

Run: `go test -count=1 -run 'TestAdvisoryBreachExitsZeroWithLoudReport' ./internal/suiterunner/`
Expected: FAIL with `report names "DOCKET_RUNTESTS_STRICT"`.

- [ ] **Step 3: Remove strict from the production code**

- `aggregate.go`:
  - `RenderReport`: drop the `strict` parameter (signature above) and the `case strict:` branch.
    The `default:` branch keeps only
    `Advisory: the tests all passed, so this run does not fail on the breach (exit 0).` — delete the
    `Set DOCKET_RUNTESTS_STRICT=1 …` `Fprintf` and its comment block (the cobra.NoArgs /
    differential-harness explanation). With one non-red arm left, `default:` stays a `default:`;
    do not reorder the red-first cases.
  - Delete the doc paragraph starting "strict is the run's confirmed exit disposition".
  - `ExitCode`: drop `strictArmed` and its `case`. Doc comment: precedence 1 > 3 > 0; a clean run,
    budget findings included, exits 0; no budget finding changes the exit code.
  - File header: "(precedence 1 > 3 > 4 > 0)" → "(precedence 1 > 3 > 0)".
  - `Tally.OverDirect` stays (decision above).
- `run.go`:
  - Header exit contract: precedence 1 > 3 > 0; list 0 / 1 / 2 / 3 / 130 / 143; drop the
    "4 a strict-mode confirmed/failed budget breach" clause. Add a short note beside "EXIT 5 IS
    RETIRED": exit 4 (the opt-in strict budget mode, `DOCKET_RUNTESTS_STRICT`) is retired, and no
    budget finding changes the exit code.
  - `RenderReport` call: drop `cfg.Strict`.
  - Budget block: delete `strictArmed`, `strictCandidates` and its loop, the strict `case`, and the
    trailing `cfg.Strict && tally.OverDirect > 0` arming with its comment. What remains under
    `if !didFire` is `clean := …` and a single `if cfg.Jobs > 1 && clean { … }` holding the
    unchanged scheduled body (Load, selfSign, ApplyScreenObservations, ScheduleConfirmation, Save,
    EmitScreenReport).
  - Rewrite the comment above the block: the budget-state machinery runs below the report, is
    skipped on an interrupted run, is fail-open, and never changes the exit code.
  - Return: `return ExitCode(tally, len(unknown))`.
- `budgetstate.go`: delete `StrictConfirmCandidates` and its doc comment. Header: drop
  "--strict re-measures every current candidate immediately and fails closed (exit 4)." (keep the
  sentence grammatical). `budgetState` comment: candidates are "for EmitScreenReport" only.
- `schedule.go`: delete the `Strict` field line.
- `internal/cli/development_test_cmd.go`: delete `Strict: os.Getenv("DOCKET_RUNTESTS_STRICT") == "1",`;
  seam comment "JOBS/STATE/STRICT tune the run" → "JOBS/STATE tune the run". Re-run `gofmt` on the
  struct literal (field alignment shifts).

- [ ] **Step 4: Update the tests for the removal**

- `aggregate_test.go`:
  - Delete `TestStrictBreachRendersStrictNote`.
  - `TestExitPrecedence`: drop the `strictArmed` field and argument; delete rows
    "failed beats strict", "noresult beats strict", "strict only when clean". Keep
    "failed beats noresult" (1), "noresult is 3", "invalid is 3", "unknown is 3",
    "advisory breach clean" (`Tally{OverDirect: 1}` → 0), "all clean" (0). Failure message:
    `ExitCode(%+v, %d) = %d, want %d`.
  - Every `RenderReport(…, false, false, "")` → `RenderReport(…, false, "")`; every
    `ExitCode(x, y, false)` → `ExitCode(x, y)`.
- `budgetstate_test.go`: delete `TestStrictConfirmsAllAndArms` and `TestStrictNotCountedArms`
  (including its doc comment). In `TestScreeningNeverAuthoritative`,
  `ExitCode(Tally{Files: 1, Passed: 1}, 0, false)` → `ExitCode(Tally{Files: 1, Passed: 1}, 0)`.
  Remove any import left unused (`go vet` reports it).
- `run_test.go`: in `TestRunOwnSignIsNotContention`, delete the `t.Run("strict", …)` subtest; keep
  the `scheduled` subtest. Rewrite the doc comment: it drives a real over-line scheduled solo
  re-check through `Run`; the run's own sign is in the sign folder the whole time and must never make
  that re-check NOT COUNTED (Run hands it to the budget state as `selfSign`).

- [ ] **Step 5: Run the package and CLI tests — expect PASS**

Run: `gofmt -l internal/suiterunner internal/cli && go vet ./internal/suiterunner/ ./internal/cli/ && go test -count=1 ./internal/suiterunner/ ./internal/cli/`
Expected: no `gofmt -l` output, vet clean, `ok` for both packages (including
`TestRunSingleJobDirectReading` and `TestAdvisoryBreachExitsZeroWithLoudReport`).

- [ ] **Step 6: Mutation-prove the stale-hint assertion**

```bash
f=internal/suiterunner/aggregate.go; cp "$f" "$f.bak"
# edit: in RenderReport's default branch, re-add after the Advisory line:
#   fmt.Fprintf(w, "Set DOCKET_RUNTESTS_STRICT=1 to gate on it — but see scripts/run-tests.md first.\n")
go test -count=1 -run 'TestAdvisoryBreachExitsZeroWithLoudReport' ./internal/suiterunner/   # expect FAIL
mv -f "$f.bak" "$f"
go test -count=1 -run 'TestAdvisoryBreachExitsZeroWithLoudReport' ./internal/suiterunner/   # expect ok
```

- [ ] **Step 7: Sweep for leftovers in Go source**

Run: `git grep -nIE 'DOCKET_RUNTESTS_STRICT|strictArmed|StrictConfirm|cfg\.Strict|Strict:|strict-budget|run-tests\.md|3 > 4|exit 4([^0-9]|$)|--strict' -- 'internal/' 'cmd/'`
Expected: no output (the `exit 4([^0-9]|$)` form keeps the unrelated `exit 47` literal in
`internal/app/maintenance_traffic_integration_test.go` out).
Then `git grep -nIi 'strict' -- internal/suiterunner internal/cli/development_test_cmd.go` and
confirm every remaining hit is an unrelated word ("strictly", "the strict read" in `results.go`).

- [ ] **Step 8: Commit**

```bash
git add internal/suiterunner internal/cli/development_test_cmd.go
git commit -m "refactor(suiterunner): retire the unused strict budget mode and exit 4"
```

---

### Task 3: Correct the docs and workflow comments

**Files:**
- Modify: `tests/README.md` (*Running it* budget paragraph and exit list; delete the exit-5
  paragraph; *Backticks in test source* enforcement parenthetical)
- Modify: `docs/reference/glossary.md` (entry *Budget watch / serially confirmed breach*)
- Modify: `AGENTS.md` ("Read the budget report" rule; `CLAUDE.md` is a symlink to it — edit
  `AGENTS.md` only, never replace the symlink)
- Modify: `.github/workflows/release-candidate.yml` (two comments in the budget-classification step)

**Interfaces:**
- Consumes: Task 2's behavior (no exit 4; every budget finding exits 0).
- Produces: nothing downstream.

No test pins any of this prose (checked: no Go or shell test reads these sentences), so the
verification is the sweep grep in Step 5.

- [ ] **Step 1: Rewrite the `tests/README.md` budget paragraph and exit list**

Replace the paragraph starting "Budgets are enforced by a **screen-then-confirm** regime:" through the
exit list, and delete the following "Exit `5` … is **retired** (change 0370)" paragraph, with
content equivalent to (exact wording is the builder's; every claim below must be present):

```markdown
No budget finding fails a run: a run whose only findings are budget findings exits `0`. Budgets are
checked two ways.

- A parallel run (the default; `DOCKET_RUNTESTS_JOBS` sets the parallelism) screens each file against
  `ceiling * 5/2` and records an advisory screening finding (`BUDGET WATCH:` /
  `PARALLEL-SENSITIVE:`). After repeated overruns the runner schedules a solo re-check, and only a
  re-check over `ceiling * 3/2` with no other suite running is an authoritative breach
  (`SERIAL CONFIRMED OVER BUDGET:`). A scheduled re-check that reads over the line while another
  `docket development test` run of this repository, in any worktree, is running is not counted
  (`SERIAL CONFIRMATION NOT COUNTED:`): the test stays due and is re-checked on the next quiet run.
  A re-check under the line always counts.
- A single-job run (`DOCKET_RUNTESTS_JOBS=1`), or a file whose budget row is `serial`, is measured
  directly against `ceiling * 3/2`, and a crossing prints `OVER BUDGET:`.

Exit `0` green (including a green run with budget findings), `1` a test failed, `3` green but at
least one target produced no result at all — the run certified nothing about it, `2` usage error /
runner-internal fail-closed (unusable bash, missing or duplicate target, **undeclared or malformed
suite category**), `130`/`143` interrupted by `SIGINT`/`SIGTERM`.
```

Must not appear afterward: `DOCKET_RUNTESTS_STRICT`, exit `4`, exit `5`, "a single-job run … `SERIAL
CONFIRMED OVER BUDGET:`".

- [ ] **Step 2: Drop the exit-5 parenthetical under *Backticks in test source***

"The invariant is a **build-gate Go guard**, not a per-run preflight (change 0370, which retired the
old exit-5 source-hygiene preflight along with the frozen Bash runner):" → "The invariant is a
**build-gate Go guard**, not a per-run preflight:". Re-wrap the paragraph only as far as needed.

- [ ] **Step 3: Glossary and AGENTS.md**

- `docs/reference/glossary.md`, *Budget watch / serially confirmed breach*: "None of these lines
  fails a default run" → "None of these lines fails a run".
- `AGENTS.md`, "Read the budget report" bullet: "Neither fails the run by default" → "Neither fails
  the run". Keep the rest of the bullet. Verify afterward that `CLAUDE.md` is still a symlink
  (`test -L CLAUDE.md`).

- [ ] **Step 4: Release-candidate workflow comments (no grep change)**

In `.github/workflows/release-candidate.yml`, budget-classification step:
- "Budget-report classification (change 0361) — the runner's complete budget vocabulary, all
  patterns line-anchored …" → say the two greps cover the screening lines and the authoritative
  lines, all line-anchored so a per-file `  OVER BUDGET (ceiling …)` suffix never matches, and that
  `SERIAL CONFIRMATION NOT COUNTED:` is left out on purpose because it is meant to fail nothing.
- "The suite itself exits 0 on an advisory breach (no --strict-budget), so an otherwise-green gate
  must fail here or nothing fails at all." → "The suite itself exits 0 on every budget finding, so an
  otherwise-green gate must fail here or nothing fails at all."
- Confirm with `git diff -U0 .github/workflows/release-candidate.yml` that only `#` comment lines
  changed.

- [ ] **Step 5: Whole-repo sweep**

Run:
`git grep -nIE 'DOCKET_RUNTESTS_STRICT|strict-budget|strictArmed|StrictConfirm|run-tests\.md|3 > 4|exit 4([^0-9]|$)|exit .4.|fails a default run|fails the run by default|complete budget vocabulary' -- ':!docs/results' ':!docs/superpowers' ':!docs/adrs' ':!docs/changes' ':!testdata'`
Expected: only the unrelated `exit 4` stubs in `tests/test_release_downloader.sh`,
`tests/test_release_downloader_refusals.sh`, `tests/test_release_downloader_converge.sh`, and
`tests/test_install_bootstrap.sh` (fixture stubs, not budget mode). Anything else is a leftover to
fix.

- [ ] **Step 6: Commit**

```bash
git add tests/README.md docs/reference/glossary.md AGENTS.md .github/workflows/release-candidate.yml
git commit -m "docs: no budget finding fails a run; drop the strict budget mode from docs"
```

---

## Build gate

`docket-build` runs the whole suite through `build.test_command` (resolved from config). Read the
budget report even on green: this change removes three tests and one subtest and adds one small
`Run` test, so the `internal/suiterunner` tests' budget row should move down, not up. Report the
row's measured margin as a number in the results file.
