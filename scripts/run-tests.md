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
run-tests.sh [-j N] [--verbose] [--timings PATH] [--budgets PATH] [--no-budget-check] [TEST ...]
```

| Flag | Required | Description |
|---|---|---|
| `-j N` | no | Parallel jobs. Default: the CPU count (`nproc`, else `sysctl -n hw.ncpu`, else 4). `-j 1` is serial. |
| `--verbose` | no | Print every file's output, not only failing files'. |
| `--timings PATH` | no | Write `<path>\t<seconds>\t<rc>\t<passes>\t<failures>`, one row per file. |
| `--budgets PATH` | no | Budget table to enforce. Default: `tests/runtime-budgets.tsv` when it exists. |
| `--no-budget-check` | no | Run the tests and report the times, but never fail on a breach. |
| `TEST ...` | no | Test files to run, repo-relative or absolute. Default: `tests/test_*.sh`. |

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
with a 3/2 slack factor — a breach is `measured > ceiling * 1.5`. The slack is deliberate: a
wall-clock assertion with no headroom becomes a flake on a loaded laptop, and a flaky gate teaches
people to pass `--no-budget-check`, which is worse than no gate.

A breach does not mask a failure. Failures win: a run with both reports exit 1.

The breach message names the file and the remedy, and the remedy is **shard the file or extend an
existing shard** — never "raise the ceiling". A budget guard whose stated remedy is to raise the
number teaches the evasion it exists to catch (repo learning `guard-remedy-must-not-teach-the-evasion`).

A malformed seconds field in the table falls back to the default ceiling rather than crashing the
run; making a malformed row loud is `tests/test_runtime_budgets.sh`'s job, not the runner's.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every test file exited 0 and every file was inside its budget. |
| 1 | At least one test file exited non-zero. Takes precedence over a budget breach. |
| 4 | Every test file exited 0, but at least one exceeded its budget. |
| 2 | Usage error — unknown flag, a flag missing its argument, a non-positive `-j`, a `TEST` that does not exist, an unwritable `--timings` path, an empty test set — or the interpreter is pre-Bash-4.3 and no usable `runtime.bash` was configured to re-exec under. |

Exit **4** is separated from **1** on purpose: "the suite is red" and "the suite is green but
something got slow" are different problems with different owners, and collapsing them would make
the budget table's failures indistinguishable from real regressions. Exit-code semantics beyond
this are change 0224's, not this script's.

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
- **The budget check is on by default and off by flag.** `--no-budget-check` is for measurement
  runs, where enforcing a ceiling against the number you are trying to measure is circular.
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
