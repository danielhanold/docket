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

Files are launched **longest budget first**, ties broken by path, so the tail starts while there is
still work to overlap it with. A file with no budget row is ordered at `DEFAULT_CEILING` (60s).
Files whose budget row says `serial` are held out of the parallel phase entirely and run one at a
time after it finishes.

Ordering is a scheduling decision only. It never changes a verdict.

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
| 1 | At least one test file exited non-zero. Takes precedence over a budget breach. |
| 4 | `--strict-budget` was given, every test file exited 0, and at least one exceeded its budget. |
| 2 | Usage error — unknown flag, a flag missing its argument, a non-positive `-j`, a `TEST` that does not exist, an unwritable `--timings` path, an empty test set, `--no-budget-check` together with `--strict-budget` — or the interpreter is pre-Bash-4.3 and no usable `runtime.bash` was configured to re-exec under. |

Exit **4** is separated from **1** on purpose: "the suite is red" and "the suite is green but
something got slow" are different problems with different owners, and collapsing them would make
the budget table's failures indistinguishable from real regressions. It is behind `--strict-budget`
for the mirror-image reason — a caller that cannot tell 4 from 1 collapses them right back, and
every caller wired to this script today is such a caller (see "Why a breach is advisory by
default"). Exit-code semantics beyond this are change 0224's, not this script's.

## Invariants

- **Parallelism changes wall time, never a verdict.** `-j 1` is the serial reference; `-j N` must
  agree with it on `files=`, `failed=`, and `asserts=`. `tests/test_run_tests.sh` asserts the
  agreement directly, and asserts that concurrency is exactly `N` — an off-by-one in the slot loop
  is invisible in the verdict, so it gets its own observation.
- **Never more than `-j N` jobs in flight.** Slots are held with `wait -n`, so a slot frees the
  moment any one job finishes rather than at a batch boundary.
- **Zero-touch against the tests.** No test file is modified, sourced, or wrapped. The runner adds
  environment and reads exit status; it injects nothing into the test's shell.
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
  path. The corollary: two targets with the same basename in different directories would collide.
  The suite is flat, so this does not arise; a future `tests/<topic>/` layout would have to
  revisit it.
- **Audited for parallel-execution races on 2026-08-06 — 80 files inspected, none found.** Every
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
  `SUITE files=80 passed=80 failed=0 asserts=6040`. 593s of per-file work compressing into ~246s of
  wall clock confirms the runs were genuinely concurrent rather than accidentally serialized, and
  both working trees stayed clean throughout — which is the direct evidence for the "not a
  container" residual above: nothing in this suite writes into the repo.
- **No file needs `serial` mode.** The budget table therefore carries `parallel` on every row and
  its serial-pin count is **0**. A row that later asks for `serial` must arrive with the shared
  resource named in a comment at the top of that test file: "it went flaky" is the symptom the
  audit exists to root-cause, not a reason to pin. Equally, a race is never resolved by loosening
  the assertion that caught it.
