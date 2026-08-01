# profile-asserts.sh — per-assertion clock times across the test suite

## Purpose

Runs docket's own test files and reports how long each **assertion** took, rather than only each
test file's total. It exists because the suite is the de-facto build gate (no GitHub Actions CI)
and runs several minutes, so "which test is slow" is a question with a routine answer and "which
part of that test is slow" is not.

It needs no cooperation from the tests: every file in `tests/` already prints exactly one line per
assertion — the `assert` helper most of them define, and the `ok` / `no` / `nok` trio the rest use,
all emit `ok - <name>` or `NOT OK - <name>` — so the existing output is treated as an assertion
protocol and timestamped as it arrives. Nothing is edited, sourced, or injected into the test's
shell.

Dev tooling for this repo's suite. It is not part of the convention a consuming repo adopts, is
invoked by no skill, and gates nothing.

## Usage

```
profile-asserts.sh [--top N] [--tsv PATH] [--verbose] [TEST ...]
```

| Flag | Required | Description |
|---|---|---|
| `--top N` | no | Rows in the slowest-segments table. Default `25`. |
| `--tsv PATH` | no | Keep the per-assertion records at `PATH`. Default: a temp file, whose path is printed at the end. |
| `--verbose` | no | Stream each test's own output through as it runs. Default: only the summary tables. |
| `TEST ...` | no | Test files to profile, repo-relative or absolute. Default: `tests/test_*.sh`. |

Tests run under `$DOCKET_BASH_PATH` when it is set (docket's configured `runtime.bash`), otherwise
under the interpreter running the script.

**Output:** the records path, then a `running <test>` line and a completion line per test file, then three tables — slowest assertion segments across
every profiled file, a per-test rollup, and the failing assertions — then a totals line and the
records path.

**Records format:** one TAB-separated line per assertion,
`<duration-us>\t<status>\t<test-path>\t<index>\t<assertion-name>`, where `status` is `PASS`, `FAIL`,
or `TAIL`. Written in run order, so the file is a timeline as well as a dataset.

## Behavior

### What a duration means

An assertion's time is its **segment**: everything since the previous assertion line — the fixture
setup plus the assertion itself — not the isolated cost of evaluating the assertion's expression.
That is the unit that locates an expensive region, and it is the honest one to report, because the
tests interleave setup and assertions freely rather than separating them into phases.

The consequence is directional and worth stating: a cheap assertion standing after slow setup
carries that setup's cost, so a segment names **where** the time went, not **what** spent it. The
companion `profile-one-test.sh` answers the second question by tracing commands.

Each test also gets one synthetic `TAIL` record — the time after its last assertion line, covering
teardown and any EXIT trap. It is sorted alongside the segments so a test whose cost is all in
cleanup cannot hide behind a fast final assertion.

### Timing mechanism

Each test is run with its stdout and stderr merged into a pipe, and a `read` loop stamps
`$EPOCHREALTIME` as each line arrives. This is sound because Bash flushes builtin output per
command, so a line reaches the reader when the test prints it rather than when a block buffer
fills — verified against this suite before the approach was adopted. The consequence is that the
profiler adds no measurable overhead: the numbers are the ones a plain run produces.

Timestamps are converted to integer microseconds by deleting the decimal separator, which keeps
every delta in shell integer arithmetic. `LC_ALL=C` is exported so that separator is a `.` under
any locale.

The reader runs in the pipe's subshell, so results travel through the records file rather than
through variables, which would die with it. The profiled test's exit status is routed around the
pipe through a temp file for the same reason.

### Assertion recognition

A line is an assertion iff it matches `^(ok|NOT OK)[[:space:]]*-[[:space:]]*(.*)` — both the
`ok - ` and `ok   - ` spacings the suite uses in practice. This keys on the emitted **shape**
rather than on a list of helper names, so a test file that introduces a fourth helper is covered
without an edit here.

The residual, stated plainly: an ordinary output line that happens to begin `ok -` would be counted
as an assertion. Nothing distinguishes the two at the stream level, and the alternative — an
enumerated list of helper names — fails on exactly the file that invents its own idiom.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Every profiled test exited 0. |
| 1 | At least one profiled test exited non-zero, or the interpreter is pre-Bash-5 and no usable `runtime.bash` was configured to re-exec under. |
| 2 | Usage error: unknown flag, a flag missing its argument, a non-numeric `--top`, a `TEST` that does not exist, or an unwritable `--tsv` path. |

## Invariants

- **Zero-touch.** No test file is modified, sourced, or wrapped, and nothing is injected into the
  test's environment. A profiled run and a plain run are the same run.
- **Read-only against the repo.** It runs tests and writes its records to a temp file or the
  supplied `--tsv` path. It touches no branch, no change file, and no board surface. The tests it
  runs are themselves hermetic (temp repos and bare origins).
- **Artifacts outlive the run.** The records file is deliberately not removed on exit — its path is
  printed for follow-up, and a cleanup trap would delete exactly what the caller was told to read.
- **Bash 5+, and loud about it.** `EPOCHREALTIME` is Bash 5.0, one major above docket's own 4+
  runtime floor. Under an older interpreter the script re-execs through `$DOCKET_BASH_PATH` when
  that is a usable Bash 5+, and otherwise exits 1 naming the remedy. A sentinel variable keeps a
  configured runtime that is itself pre-5 from re-exec'ing forever.
- **No early-exiting consumer under `pipefail`.** Every table is truncated with `awk NR<=top`, never
  `head` — the producer would take SIGPIPE and turn a clean profile into an intermittent 141
  (AGENTS.md, *Shell*).
- **Reports, never gates.** The exit status mirrors the tests' own outcome so the script can stand
  in for a plain suite run, but nothing in docket consumes its output and no skill invokes it.
