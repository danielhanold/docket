<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0528 — Make the solo budget re-check detect a concurrent suite in another worktree](../../changes/active/0528-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a.md)**
<!-- docket:backlink:end -->

# Make the solo budget re-check detect a concurrent suite in another worktree — design

Change #528, groomed 2026-10-16.

## Problem

The suite runner (`docket development test`, `internal/suiterunner`) gives each test file a wall-clock
ceiling in `tests/runtime-budgets.tsv`. A parallel run over `ceiling * 5/2` is only a screening
signal. After repeated overruns the runner re-runs that one file by itself (`ScheduleConfirmation` →
`soloConfirm` in `internal/suiterunner/budgetstate.go`, or every candidate at once under
`DOCKET_RUNTESTS_STRICT=1` via `StrictConfirmCandidates`). If the solo time is over `ceiling * 3/2`
(`SoloOver`), it prints `SERIAL CONFIRMED OVER BUDGET:`, the one budget line the repo treats as a real
breach (`AGENTS.md`, `tests/README.md`; the release-candidate workflow fails a release on it).

"By itself" only means the runner's own other files have finished. The re-check cannot see a suite
running in another worktree of the same repository, and concurrent autonomous loops do exactly that.
Change #507 found that every over-the-line reading behind its timing items was contention. Measured
alone on a quiet machine (2026-10-05), `test_go_finalize_e2e.sh` read about 36s against a 45s line,
although a contended re-check had certified 49s; app_merge's 92s and go_race's 117s and 94s readings
were really 41s and 54s. PR 361 recorded its 94s race reading at load 45–69 while another change's
gate was running. Each false breach turned into results-file follow-ups and grooming work.

## Goals

- A re-check reading over the line, taken while another `docket development test` run in any worktree
  of this repository was running, is never reported or stored as a confirmed breach.
- A re-check reading under the line still counts as a clear, contended or not. Contention only slows a
  test, so a test that passes while slowed would pass on a quiet machine.
- The suspected breach is not lost: the test stays due and the next quiet run re-checks it first.

## Non-goals

- Waiting, queueing, or making suites take turns. Nothing in this change makes a run wait.
- Detecting load that does not come from a `docket development test` run: a hand-run `go test` or
  `bash tests/<file>.sh` in another worktree, other repositories, other programs, load average. This
  blind spot is accepted.
- Direct readings from a single-job run (`DOCKET_RUNTESTS_JOBS=1`) or a serial-mode budget row (the
  `OVER BUDGET:` path in `Run`). Nothing uses them today: no gate, no CI, no docs, and no serial-mode
  rows exist. #557 reviews whether that path should stay at all.
- Retiring strict mode (#557).
- Host-relative budgets or re-seeding the table (#273).
- Changing thresholds, the screening regime, the 0/1/3/4 exit contract, or the release-candidate
  workflow.

## Anti-goals

- A gate that waits, slows down, or fails because another worktree is running tests. The one exception
  is the existing opt-in strict mode (below), which no gate turns on.
- A problem with the new signs (unwritable folder, odd files) failing a run on its own.

## Design

### 1. Every run puts up a sign

When `Run` starts (after input validation, before the lanes launch), it puts up a **sign**: a file in
a sign folder, held under an exclusive kernel advisory lock for the rest of the run. Reuse the existing
flock helpers, `process.TryExclusiveLock` and `process.ProbeLock` (`internal/process/lock.go`, already
used by the worktree lock). The OS drops the lock when the process exits for any reason, crash and
SIGKILL included, so a dead run can never look alive. Pid reuse cannot fool it either.

- **Location:** a folder beside the budget-state store, i.e. in the directory of `cfg.StatePath`.
  By default that is the per-repo state folder under the git common dir, shared by every worktree.
  A test or caller that points `DOCKET_RUNTESTS_STATE` elsewhere gets its own sign folder, so unit
  tests and real runs never see each other's signs. Folder name: my call, e.g.
  `development-test-runs/`.
- **Content:** at least the run's worktree root (`cfg.RepoRoot`) so a report can name it. Pid and
  start time are useful for a human reading the folder. The format is the builder's call.
- **Never visible unlocked.** A checker must never find a sign that belongs to a live run but is not
  yet locked. The suggested shape: create and lock the file under a temporary name in the same folder,
  then rename it into its visible name (the lock stays with the file). Checkers ignore temporary
  names, and only regular files matching the sign naming pattern count as signs.
- **Unique names, owner-only creation.** Each sign name is used once (for example pid plus a random
  suffix), and only its owner ever creates it. Checkers only probe and, for a dead sign, delete. This
  is what makes deletion safe here. `internal/gatedrive`'s worktree lock never deletes its lock
  files, because a fixed lock name can be re-opened by a concurrent locker after an unlink. That
  hazard does not exist with single-use names.
- **The lock lives as long as the run.** `Run` keeps a live reference to the sign's `*os.File` until
  it returns. Go closes an unreferenced file when it is garbage-collected, which would silently drop
  the lock.
- **Removal:** the run removes its own sign when `Run` returns. A checker may delete a visible sign it
  finds unlocked, since that run is dead. Leftover temporary files from a crash mid-publish must never
  count as signs, and must not pile up without bound.
- **Failure is advisory.** If the sign cannot be put up, print one warning line on stderr (same style
  as the existing `budget-state lock not acquired` warning) and continue. Other runs simply won't see
  this one.
- **No state path:** when `cfg.StatePath` is empty (git common dir unresolvable), no sign is put up
  and no check runs. Behavior is exactly today's, matching the existing no-history posture.

### 2. The re-check looks for other signs, before and after

`soloConfirm` serves both the scheduled re-check and strict mode. It checks the sign folder right
before it launches the target and right after the target returns. Another run counts as **seen** if
its sign is held at either check.

- The run's own sign is excluded by its exact path.
- An unlocked visible sign means a dead run: ignored (and may be deleted).
- A missing sign folder means no signs: clean.
- An unreadable folder, or a `LockProbeUnknown` answer for any sign, means **can't tell**. That is
  treated like another run being seen, so a breach is never confirmed on a guess.
- A sign that appears only during the measurement is caught by the after-check. A run that both
  starts and finishes inside one re-check (20–120s) is missed. A full suite takes 6+ minutes, so this
  is accepted.

### 3. What the reading means

| Another run seen (or can't tell)? | Solo reading | Outcome |
|---|---|---|
| No | over the line | Today: `SERIAL CONFIRMED OVER BUDGET:`, state `confirmed-breach`, `breached` |
| No | at/under the line | Today: cleared, state `parallel-sensitive`, `cleared` |
| Yes | at/under the line | Same as today's clear, unchanged in every field |
| Yes | over the line | **Not counted** (below) |
| either | target exited non-zero | Today: `SERIAL CONFIRMATION FAILED:` (contention does not change this) |

**Not counted**, scheduled path (`ScheduleConfirmation`):

- Print, in place of the CONFIRMED line, one line starting `SERIAL CONFIRMATION NOT COUNTED: <path>`
  that names the solo seconds, the solo threshold, the reason, and that the re-check stays due. For
  example:
  `SERIAL CONFIRMATION NOT COUNTED: tests/x.sh — 49s solo while another suite was running (/path/to/worktree-y); solo threshold 45s; recheck stays due`.
  For can't tell, the parenthetical says the runner could not check for other suites. Several seen
  runs are all named. The wording around the fixed prefix is the builder's call. The line must not
  start with `OVER BUDGET` or `SERIAL CONFIRMED OVER BUDGET`, the prefixes the release-candidate
  workflow fails on.
- Store: `last_confirmation_result` = `not-counted`. Everything else stays as it was, like the
  FAILED branch: state, streak, since-counter, `last_solo_seconds` (still the last quiet reading), and
  the due stamp (still due). The next qualifying run's `dueCandidates` ordering puts it first, as it
  does today for any overdue record.
- `SERIAL CONFIRMATION DUE:` still prints before the solo run. `SERIAL CONFIRMATION DEFERRED:` lines
  for the other due records and the trailing `EmitScreenReport` lines are unchanged.

**Not counted**, strict path (`StrictConfirmCandidates`): the same line and the same stored result,
and it arms strict (exit 4), exactly as a failed confirmation does today. Strict mode never lets a
doubtful result through. A seen run with an under-the-line reading clears as today and does not arm.

### 4. Docs and comments

- `tests/README.md`, budget paragraph: say that a re-check over the line while another
  `docket development test` run in any worktree of this repository is running is not counted
  (`SERIAL CONFIRMATION NOT COUNTED:`), and the test stays due for the next quiet run; a re-check under
  the line counts either way. Docs describe current behavior only; no change numbers.
- `budgetstate.go` file header and the `soloConfirm` / `ScheduleConfirmation` /
  `StrictConfirmCandidates` doc comments: update "only a solo (uncontended) re-measurement … is ever
  authoritative" to include "with no other suite seen". Comment cross-references anchor on symbol
  names, never line numbers.
- `docs/reference/glossary.md`, entry *Budget watch / serially confirmed breach*: add the NOT COUNTED
  line to the list of budget lines it explains, as a re-check outcome that is neither a screening
  finding nor a breach.
- `AGENTS.md` and `.github/workflows/release-candidate.yml` need no change. Nothing else parses the
  budget lines: no skill, and no repoguard test pins the vocabulary. Check that this still holds when
  the build is done.

## Testing

Each guard is mutation-proved: remove the behavior it protects, watch the test go red, restore it.
Existing harness: `budgetstate_test.go` builds a due record with `ApplyScreenObservations`, and the
`DOCKET_RUNTESTS_TEST_DURATIONS` seam (`writeDurations`) injects the solo seconds. A test can play
"another run" by holding a lock on a file in the sign folder with `process.TryExclusiveLock`. A second
open file description in the same process is refused, so `ProbeLock` reports it held.

1. Another run's sign held, over-line solo: NOT COUNTED line, no CONFIRMED line, record has
   `not-counted` and an unchanged state, streak, since-counter, `lastSolo`, and due stamp.
2. Another run's sign held, under-line solo: cleared exactly as today.
3. No other sign (empty or missing folder): confirmed breach exactly as today.
4. Dead run's sign (visible file, unlocked): ignored, so a confirmed breach exactly as today.
5. The run's own sign up: still confirms (no self-contention).
6. A sign that appears only after the measurement starts is caught by the after-check. The seam is the
   builder's call.
7. Can't tell (unreadable folder, or an unknown probe): NOT COUNTED with the could-not-check reason.
8. Strict: seen + over-line → NOT COUNTED and armed (exit 4); seen + under-line → cleared, not armed.
9. A run's sign is visible and held for the run's lifetime, never visible unlocked, and gone after
   `Run` returns. A sign that cannot be put up warns and never fails the run.
10. Empty `StatePath`: no sign is written and behavior is unchanged. The existing tests, which build
    `Config` without `StatePath`, keep passing untouched.
11. The NOT COUNTED line does not match `^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):`.

Run the whole suite at the build gate and read the budget report. The suiterunner tests' budget row
must still fit after the new tests are added.

## Known limits

- Load from anything other than a `docket development test` run of this repository is invisible
  (non-goal above).
- A run shorter than one re-check that starts and finishes inside it is missed.
- While loops overlap constantly, a real breach can sit due for a while. It is never falsely
  confirmed, and its `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line keeps showing until a quiet run
  settles it. As after a FAILED re-check today, a `watching` record still loses its due stamp if its
  next parallel reading is under the screening line. The screening regime is unchanged.
- **Rollout.** The gate runs `go run ./cmd/docket development test`, which runs whatever runner source
  the branch carries. Feature branches cut before this change merges put up no signs until they are
  rebased onto it, so for a while after the merge their suites stay invisible to the check.

## Left to the builder

File and folder names, the sign file format, where the helper lives (a new file in
`internal/suiterunner` is the natural home), the after-check test seam, the exact wording around the
fixed `SERIAL CONFIRMATION NOT COUNTED:` prefix, and the handling of leftover temporary files within
the constraints above.

## Alternatives considered

- **Skip the re-check when another run is already up.** This saves 20–120s on a loaded machine, but
  it adds a second rule and throws away under-the-line clears, which are still valid.
- **Make the re-check wait for the other suite** (with a cap). This adds up to the cap to a gate whose
  tests have already passed, all for an advisory number. It needs two more rules: a waiting run must
  not count as running, or two runs wait on each other forever, and only one re-check may run at a
  time, or two runs released together contend with each other. It still needs the not-counted fallback
  when the cap runs out.
- **Make whole suites take turns.** This removes contention everywhere, but it serializes concurrent
  loops' gates. A full suite is 6–13 minutes and each gate has 30, so a third loop would wait near the
  gate limit. It is a policy change for concurrent loops, larger than this fix.
- **Scan the process list for test processes.** This would catch hand-run tests too, but it is
  guesswork about command lines, differs between macOS and Linux, and matches the wrong things.
- **Load average.** This is confounded by the run's own just-finished parallel lane and by the solo
  target itself, so it cannot tell "busy because of us" from "busy because of someone else".
- **Read gate-drive records** (`internal/gatedrive`). Those cover only gate-driven suites, not a
  `docket development test` typed by hand, and they would make the low-level runner depend on a
  higher-level package.
