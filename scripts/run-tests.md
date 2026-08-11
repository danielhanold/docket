# run-tests.sh — parallel runner for docket's own test suite

## Purpose

Runs docket's `tests/test_*.sh` **N at a time** instead of one after another, each job in its own
`HOME` / `TMPDIR` / git-config sandbox, and reports a single deterministic aggregate.

It exists because the suite is the de-facto build gate (no GitHub Actions CI) and had grown to
roughly ten minutes of wall clock — long enough that agents and humans alike started running
subsets, which is the failure mode a gate cannot afford. The files are hermetic and have no
ordering dependencies, so serial execution bought nothing and cost the whole ten minutes.

Dev tooling for **this** repo's suite, in the same family as `profile-asserts.sh` and
`profile-one-test.sh`. It is not part of the convention a consuming repo adopts, and it is
deliberately **not** a `docket.sh` facade op.

## Usage

```
run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check]
             [--strict-budget] [TEST ...]
```

| Flag | Required | Description |
|---|---|---|
| `-j N` | no | Parallel jobs. Default: the CPU count (`nproc`, else `sysctl -n hw.ncpu`, else 4). `-j 1` is serial. |
| `--verbose` | no | Print every file's output, not only failing files'. |
| `--timings PATH` | no | Write `<path>\t<seconds>\t<rc>\t<passes>\t<failures>`, one row per file. |
| `--budgets PATH` | no | Budget table to compare against. Default: `tests/runtime-budgets.tsv` when it exists. |
| `--no-budget-check` | no | Skip the comparison entirely — no breach is measured, and none is reported. |
| `--strict-budget` | no | Make a breach **fatal** (exit 4). By default a breach is reported and the run still exits 0. |
| `TEST ...` | no | Test files to run, repo-relative or absolute. Default: `tests/test_*.sh`. |

`--no-budget-check` and `--strict-budget` together are a usage error (exit 2), not a
winner-takes-all: letting the first silently win would hand a caller that explicitly asked to be
gated on budgets a guard that is disarmed and green.

Tests run under `$DOCKET_BASH_PATH` when it is set (docket's configured `runtime.bash`), otherwise
under the first `bash` on `PATH`.

## Behavior

### Discovery and ordering

With no `TEST` arguments the runner takes `tests/test_*.sh` at depth 1 — the same glob the suite has
always used, so a new shard self-registers by existing. Explicit arguments replace the glob
entirely; a `TEST` that does not exist is a **usage error** (exit 2), not a silent `rc=127` row,
because a mistyped path and a genuinely broken test read identically in a summary.

Two targets that share a **basename** are a usage error too, checked in the same pre-flight pass and
reported the same way — the message names the colliding basename and both paths that produced it.
Everything downstream is keyed on the basename (see "Basename is the join key" below), so
`a/test_x.sh b/test_x.sh` — or one path passed twice, which is the likelier way to hit it — would
put two concurrent jobs on the same log file and the same stat record: interleaved output, doubled
assert counts, a double-printed report row, and a `SUITE` line that is quietly wrong. It is a
stricter case than the mistyped path, because nothing about the result *looks* like an error. The
default glob cannot produce a collision; explicit arguments can, so the check runs before any job
launches.

Files are launched **longest budget first**, ties broken by path, so the tail starts while there is
still work to overlap it with. A file with no budget row is ordered at `DEFAULT_CEILING` (60s).
Files whose budget row says `serial` are held out of the parallel phase entirely and run one at a
time after it finishes.

Ordering is a scheduling decision only. It never changes a verdict.

### Source-hygiene preflight

Before the budget table is read and before the first job launches, the runner scans every target
with `scripts/check-test-source-hygiene.sh` and refuses to start if it comes back dirty. A violation
exits **5**, having executed **zero** test files.

The placement is the whole point, not a convenience. A backtick in test source runs when the shell
**reads that line** — before the file's first `assert`, before any helper prints anything — so by
the time a post-hoc lint could report it, a peer file's source has already been evaluated and
whatever it carried has already run. Detection after execution is not prevention, which is why this
scan is synchronous and upstream of every launch rather than a pass over the logs. Change 0212
shipped exactly that hazard: a multi-line double-quoted block whose quoted anchor text carried
`git checkout .`.

What the checker rejects — four backtick classes and one definition-drift class — is
`scripts/check-test-source-hygiene.md`'s contract, not this one's. Two of its properties change how a
run-tests verdict reads:

- **The scan is not limited to the targets.** Its definition-drift rule sweeps the whole `tests/`
  tree (excluding `tests/fixtures/`, whose red half is drifted on purpose), because assert helpers
  live in `tests/lib/*.sh` files that the `tests/test_*.sh` glob never passes. So a single-file run
  can abort on drift in a file it was not asked to run. That is deliberate: the drift is real either
  way, and the alternative leaves those definitions permanently unguarded.
- **It protects suite runs only.** `bash tests/test_x.sh` run directly bypasses the preflight
  entirely, because nothing but this runner calls it. That residual is accepted knowingly — the
  alternative is a preamble in 100+ test files — and it is why `tests/README.md` carries the rule in
  prose as well.

**An unusable checker refuses the run; it never skips itself.** If
`scripts/check-test-source-hygiene.sh` is missing or unreadable, or it exits with anything other
than "clean" or "violations found", the runner exits **2** — the same *will not start* family as the
Bash floor — and launches nothing. A gate that waves the run through when its own checker is absent
certifies safety it did not provide, which is worse than having no gate, because the run still looks
inspected. Readability is what is tested, not the execute bit: the checker is invoked as
`bash <path>`, which never needs one.

The preflight runs **after** the usage checks above, so a mistyped target and an unwritable
`--timings` path stay the exit-2 usage errors they have always been rather than being pre-empted by
a scan of files the caller got wrong. `tests/test_run_tests.sh` pins the ordering claim with a
marker file: its hazard fixture writes one from inside the substitution itself, and the run is only
green if that marker is still absent afterwards.

### Per-job isolation

Each job runs in a subshell with a private sandbox under the runner's temp directory:

| Variable | Value |
|---|---|
| `HOME` | `<work>/jobs/<test>/home` — **per job**, not one shared shim |
| `TMPDIR` | `<work>/jobs/<test>/tmp` |
| `XDG_CONFIG_HOME` | `<work>/jobs/<test>/home/.config` |
| `GIT_CONFIG_GLOBAL` | `<work>/jobs/<test>/home/.gitconfig`, carrying a synthetic identity |
| `GIT_CONFIG_SYSTEM` | an empty file, so a machine-wide `gitconfig` cannot reach a test |
| `GIT_TERMINAL_PROMPT`, `GIT_ASKPASS`, `GIT_EDITOR`, `EDITOR`, `VISUAL`, `GIT_PAGER`, `PAGER`, `GIT_MERGE_AUTOEDIT` | pinned non-interactive |

The git identity is **synthetic, not absent** (`docket test <test@docket.invalid>`): a test that
commits must still be able to commit. The identity file is written at `$HOME/.gitconfig`, which is
also where a pre-2.32 git — one that ignores `GIT_CONFIG_GLOBAL` — will look, so the isolation does
not depend on the git version.

Per-job rather than per-run is the load-bearing part. A single shared sandbox would isolate the
tests from the developer but not from **each other**, which is exactly the race parallelism
introduces.

**What this is not.** It is not a container. A test that writes inside the repo working tree still
writes inside the repo working tree, and two such tests can still collide. That residual is what
the budget table's `serial` mode exists for, and what the parallel-safety audit recorded below
went looking for.

### Output

Per-file output is buffered to a log and nothing is printed from it until every job has finished.
The report is then emitted **sorted by basename**, so it is byte-stable across `-j` values apart
from the wall-clock numbers. A failing file's log is always printed; a passing file's only under
`--verbose`.

Two streams, two contracts:

- **stdout** — the deterministic report, the `SUITE` line, and the `FAILED:` / `OVER BUDGET:` lines.
- **stderr** — a live progress ticker, one `<test> PASS|FAIL` line as each job finishes. It is in
  **completion order**, which is racy by construction. Nothing keys on it; redirect it away if you
  want a stable capture.

### Interruption

Ctrl-C (or a `SIGTERM`) is handled, not left to the default action. On a two-minute gate run it is
the likeliest interactive ending, and untrapped it is a data-destroying one: output is buffered
until every job finishes, so an interrupted run has printed nothing but the ticker, and the `EXIT`
trap would then delete the work directory while live jobs are still writing into it.

The jobs would also **survive**. The runner has no job control, so bash sets `SIGINT` to ignored in
every async child, and the test processes those children fork inherit it — the terminal's Ctrl-C
reaches the runner and nothing else. Orphaned test processes writing into a deleted directory is
the actual untrapped outcome.

So the handler reaps first and removes the work directory second, and it signals **exactly the
processes this runner launched**, by pid. Each in-flight job publishes its subshell pid and its test
pid and unlinks that record on the way out, so the handler's view is the set of jobs still running
and it never signals a pid that has been reused. Both pids are needed: killing the subshell alone
leaves the test itself orphaned, which is half of what the handler exists to prevent.

It is deliberately **not** `kill 0`. A process-group kill is only contained when the runner leads
its own process group, which happens only when an interactive shell's job control made it one.
Invoked the way this script is actually invoked in anger — from another script, from
`docket-finalize-change`'s `eval`, from a test file — job control is off, the runner shares its
caller's process group, and `kill 0` would take the caller and its siblings down with it. It would
also re-enter the handler by signalling the runner itself.

The handler then says what was lost — elapsed seconds and how many of the targets had finished —
and exits `130` (`INT`) or `143` (`TERM`). No report is produced; a partial run certifies nothing.

The summary line is stable and greppable:

```
SUITE files=<n> passed=<n> failed=<n> asserts=<n> wall=<n>s
```

`asserts` is the sum of `ok -` and `NOT OK` lines across the run — the same assertion protocol
`profile-asserts.sh` keys on. It is **reporting only**. Pass/fail authority is each test file's own
exit status, never a scraped count.

### Budget enforcement

When a budget table is in effect, each file's measured wall clock is compared against its ceiling
with a 5/2 slack factor — a breach is `measured > ceiling * 2.5`. The slack is deliberate, and it
covers two distinct effects. First, a wall-clock assertion with no headroom becomes a flake on a
loaded laptop, and a flaky gate teaches people to pass `--no-budget-check`, which is worse than no
gate. Second, and larger: a ceiling is a claim about a file's cost measured *serially*, but
enforcement happens inside a parallel run where every job competes for the machine. Measured
contention inflation on the change-0227 hardware reached 2.22x, so the original 3/2 factor rejected
11 healthy files. 5/2 covers that worst case with margin.

**What that leaves the comparison able to catch, stated honestly.** A breach needs `measured >
ceiling * 2.5`, and the seeded ceiling already sits 5–10s *above* the file's measured serial cost
(rounded up to the next multiple of 5, plus a 5s margin, minimum 10s). So the growth multiple a
file must actually reach is not 2x — it runs from roughly **2.75x** for the largest rows to about
**25x** for a ~1s file sitting on the 10s floor, and 69 of the table's 86 rows are on that floor.
This comparison is therefore a check on the **tail**: it catches a big file getting much bigger,
which is the shape that produced the original 629s suite. It does not notice a small file tripling.
What covers the small rows is not this comparison at all but the table itself — `EXPECTED_TOTAL` in
`tests/test_runtime_budgets.sh` pins the sum of every ceiling, so a row cannot be raised to absorb
growth without reddening. A contention-independent basis that would let the per-file comparison
bite lower down is change **0229**'s.

A breach does not mask a failure. Failures win: a run with both reports exit 1.

The breach message names the file and the remedy, and the remedy is **shard the file or extend an
existing shard** — never "raise the ceiling". A budget guard whose stated remedy is to raise the
number teaches the evasion it exists to catch (repo learning `guard-remedy-must-not-teach-the-evasion`).

A malformed seconds field in the table falls back to the default ceiling rather than crashing the
run; making a malformed row loud is `tests/test_runtime_budgets.sh`'s job, not the runner's.

#### Why a breach is advisory by default

A breach is **reported, not fatal**, unless the caller passes `--strict-budget`. That is a
deliberate reversal of this script's first posture, and the reasoning is worth keeping.

The slack factor above is calibrated to **one machine's** measured contention. The comparison it
drives is therefore hardware- and load-dependent in both directions: on a smaller machine relative
to the job count, inflation exceeds 2.5x and healthy files breach; on a much larger one, 2.5x makes
enforcement nearly vacuous. Change **0229** exists to settle a contention-independent basis for it.
A measurement that shaky may usefully *inform* a merge. It must not *block* one.

And blocking is exactly what a non-zero exit does here, because "non-zero" is the only budget
vocabulary this runner's callers have. All three read any non-zero exit as *the suite is red*:

- `docket-finalize-change`'s `configured-bash-finalize` block is a bare `eval` of the configured
  test command, and its step 5 answers red by dispatching `docket-integration-repair`;
- `docket-build`'s build gate turns red into a synthetic repair task on the
  `premium → max → halt` ladder;
- a human or agent following `AGENTS.md` reads a non-zero exit the same way.

None of them can tell 4 from 1, and the first two would send a repair agent to root-cause failing
tests when `failed=0`. Encoding a diagnostic as a failure exit only works if every caller is
budget-aware; here, none is. So the breach leaves by the channel every caller *does* read — the
report — and turns fatal only for a caller that opted in.

**What this costs, stated plainly.** Nothing in this repo runs `--strict-budget` automatically
today (there is no CI; the suite is the gate). So the third pillar of change 0227 — a runtime
budget so the tail cannot regrow — is currently defended by three things, none of which turns a
*measured* breach into an automatic red:

1. Every default run **prints** `OVER BUDGET:` with the offending files and the shard remedy,
   including the merge-gate run, whose output a human or agent reads.
2. `tests/test_runtime_budgets.sh` still hard-fails on the table itself — a missing row for a new
   test file, a row above the 60s ceiling, any `serial` pin, **any change to the sum of every
   ceiling**, and a configured `finalize.test_command` that passes `--no-budget-check`. The last
   two are what make this a real defence rather than a structural one: a row raised from 35 to 60
   breaks no ceiling and pins nothing serial, and disarming the check at the merge gate leaves
   every other assertion green. Both now redden on their own.
3. `--strict-budget` exists for a caller that knows what it is asking for. Run it at `-j 1`, where
   a serial ceiling is the honest comparison, if you want the sharp answer today.

Closing that gap — an automatic, contention-independent regrowth check — is **change 0229's job,
and it is explicitly deferred to it**, not quietly dropped.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every test file exited 0 — **including** a run where a file exceeded its budget, which is reported and not fatal without `--strict-budget`. |
| 1 | At least one test file exited non-zero. Takes precedence over a budget breach and over a missing result. |
| 3 | Every target that produced a result passed, but at least one produced **no result at all** — its job died before recording one. The run certified nothing about that file. |
| 4 | `--strict-budget` was given, every test file exited 0, and at least one exceeded its budget. |
| 5 | The source-hygiene preflight found a violation. **Zero test files were executed** — no job launched, no report produced. |
| 2 | Usage error — unknown flag, a flag missing its argument, a non-positive `-j`, a `TEST` that does not exist, **two `TEST`s that share a basename**, an unwritable `--timings` path, an empty test set, `--no-budget-check` together with `--strict-budget` — or the runner will not start: the interpreter is pre-Bash-4.3 and no usable `runtime.bash` was configured to re-exec under, or the source-hygiene checker is missing, unreadable, or could not complete. |
| 130 / 143 | Interrupted by `SIGINT` / `SIGTERM`. The in-flight jobs are reaped, the work directory is removed after them, and a one-line loss report goes to stderr. No report is produced. |

Exit **4** is separated from **1** on purpose: "the suite is red" and "the suite is green but
something got slow" are different problems with different owners, and collapsing them would make
the budget table's failures indistinguishable from real regressions. It is behind `--strict-budget`
for the mirror-image reason — a caller that cannot tell 4 from 1 collapses them right back, and
every caller wired to this script today is such a caller (see "Why a breach is advisory by
default"). Exit-code semantics beyond this are change 0224's, not this script's.

Exit **3** is the harness saying it lost a job. A per-file verdict is a stat record written by the
job's own subshell after the test exits; if that subshell dies first — an OOM kill under
`-j <CPU count>` with git-heavy jobs, a full disk, an external `kill` — the record is never
written. `files=` counts the records that exist, so without this check the file simply drops out of
a well-formed report and the run still exits 0, certifying a suite that ran fewer files than it was
asked to. The runner therefore compares `files` against the number of targets, prints a
`NO RESULT:` line naming each absent file, and exits non-zero. It is **3** rather than **1**
because no test failed: a caller that answers 1 by dispatching a repair agent to root-cause failing
tests would find none, and the remedy here is to re-run (and, if it recurs, lower `-j`). It ranks
below **1** because when a run is both red and incomplete the real failure is the more actionable
signal — the `NO RESULT:` block is printed either way.

Exit **5** is a **failure**, not one of this runner's non-failure outcomes. ADR-0074 makes the build
gate read a bare non-zero and delegate that judgment to the resolved runner's documented contract,
so this row is where the judgment is recorded: unlike **3** (nothing failed and there is nothing to
root-cause) and **4** (the suite is green, something got slow), a **5** names a concrete defect at a
concrete `file:line` and the remedy is an edit to that file. A caller that answers it by dispatching
a repair agent is doing the right thing. It is distinct from **1** because no test ran to fail —
reading the logs for a failing assertion would find none — and because the abort is the *only* thing
standing between the run and executing that source.

## Invariants

- **Parallelism changes wall time, never a verdict.** `-j 1` is the serial reference; `-j N` must
  agree with it on `files=`, `failed=`, and `asserts=`. `tests/test_run_tests.sh` asserts the
  agreement directly, and asserts that concurrency is exactly `N` — an off-by-one in the slot loop
  is invisible in the verdict, so it gets its own observation.
- **Never more than `-j N` jobs in flight.** Slots are held with `wait -n`, so a slot frees the
  moment any one job finishes rather than at a batch boundary.
- **Zero-touch against the tests.** No test file is modified, sourced, or wrapped. The runner adds
  environment and reads exit status; it injects nothing into the test's shell. The source-hygiene
  preflight *reads* every target's bytes before the first launch, and only reads them — a checker
  that had to run a file to judge it would be the hazard it exists to catch.
- **Nothing executes until the preflight has passed.** A hygiene violation, or a checker that cannot
  be used, aborts with no job launched at all. `tests/test_run_tests.sh` asserts it against a marker
  file rather than against the exit code alone.
- **Read-only against the repo.** It writes only into its own temp directory and, if asked, the
  `--timings` path. It touches no branch, no change file, and no board surface.
- **The budget comparison is on by default; its teeth are opt-in.** Three states, not two: on and
  advisory (the default — measured, reported, exit 0), on and fatal (`--strict-budget` — exit 4),
  and off (`--no-budget-check` — not measured, not reported, for measurement runs where enforcing a
  ceiling against the number you are trying to measure is circular). The middle state is what the
  merge gate runs, so a breach stays visible there without being mistaken for a red suite.
- **Not a facade op.** `run-tests` is absent from `WRAPPED_OPS` in `scripts/docket.sh` and from its
  dispatch `case`, which `tests/test_docket_facade.sh` enforces. It is repo-local dev tooling, like
  `profile-asserts.sh`; a consuming repo gets nothing from it.
- **Bash 4.3+, and loud about it.** `wait -n` is 4.3, above docket's own 4+ runtime floor. Under an
  older interpreter the script re-execs through `$DOCKET_BASH_PATH` when that is usable, and
  otherwise exits 2 naming the remedy. A sentinel variable keeps a configured runtime that is
  itself pre-4.3 from re-exec'ing forever. Test files gain no new floor from this.
- **Basename is the join key.** Budget rows, log files, and stat records are all keyed on the test
  file's basename, so a table row written repo-relative matches a target passed as an absolute
  path. The corollary is **defended, not merely noted**: two targets sharing a basename would write
  the same log and the same stat record, so the pre-flight pass rejects them as a usage error
  (exit 2) naming the basename and both paths. The suite is flat, so the collision only arises from
  explicit arguments today; a future `tests/<topic>/` layout would have to re-key the joins rather
  than relax this check.
- **An interrupted run reaps before it cleans up, and never signals its caller.** `SIGINT` and
  `SIGTERM` are trapped: the handler kills each in-flight job's test process and subshell by pid,
  waits for them, reports how much had finished, and only then removes the work directory. It is
  not `kill 0` — the runner shares its caller's process group whenever job control is off, which is
  every non-interactive invocation. See "Interruption" above.
- **Audited for parallel-execution races on 2026-08-06 — the 80 files that existed then, none
  found.** The suite is now 86 files; the six added by the later sharding commits were never in
  this sweep or in the equivalence proof below. Every
  `tests/test_*.sh` was swept for the shapes that make a file unsafe to run beside its neighbours:
  a read of — or a write through — the ambient `$HOME`, a `git config --global` or `--system`
  write, a write into this repo's working tree (its `.docket/` metadata worktree or a `.worktrees/`
  entry), a fixed path under `/tmp`, network access, and a fixed port. Every `HOME=` in the suite
  turned out to be an *assignment* into a fixture sandbox, never a read of the developer's home;
  there are no `--global` or `--system` config writes; every `worktree add` targets a `mktemp -d`
  fixture rather than this repo; every sandbox root comes from `mktemp -d`, which follows the
  per-job `TMPDIR`; no test writes to a relative path, so none depends on the invoker's cwd; and
  there is no network or port use at all. The suite's one direct `/tmp` write, `test_render_board`'s
  `/tmp/render-board-stderr.$$`, is PID-suffixed and so cannot collide between concurrent jobs.
- **That audit was proven by running, not by reading.** Inspection finds shapes; only a run finds
  races. Three fresh full parallel runs at the default `-j` (11 on the audit machine, 245–247s
  each) were diffed per file against the `-j 1` baseline on **both** exit status and per-file
  assert counts — six diffs, all empty. Every run and the baseline reported
  `SUITE files=80 passed=80 failed=0 asserts=6040`, i.e. the proof covers the 80-file set as of the
  audit commit, not the six files sharded in afterwards. 593s of per-file work compressing into ~246s of
  wall clock confirms the runs were genuinely concurrent rather than accidentally serialized, and
  both working trees stayed clean throughout — which is the direct evidence for the "not a
  container" residual above: nothing in this suite writes into the repo.
- **No file needs `serial` mode.** The budget table therefore carries `parallel` on every row and
  its serial-pin count is **0**. A row that later asks for `serial` must arrive with the shared
  resource named in a comment at the top of that test file: "it went flaky" is the symptom the
  audit exists to root-cause, not a reason to pin. Equally, a race is never resolved by loosening
  the assertion that caught it.
