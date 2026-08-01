# profile-one-test.sh — command-level profile of a single test

## Purpose

Answers the question `profile-asserts.sh` deliberately cannot: within a slow assertion segment,
**which command** spent the time. It traces one test file at the command level and reports self
time per source line and per invocation.

The two are a pair and neither subsumes the other. `profile-asserts.sh` is zero-overhead and covers
the whole suite, so it is the instrument for *finding* the slow region; this one perturbs timing to
buy per-command attribution, so it is the instrument for *explaining* one. Use it after the sweep,
on the file the sweep named.

Dev tooling for this repo's suite. It is not part of the convention a consuming repo adopts, is
invoked by no skill, and gates nothing.

## Usage

```
profile-one-test.sh [--top N] [--trace PATH] [--asserts] TEST
```

| Flag | Required | Description |
|---|---|---|
| `--top N` | no | Rows per table. Default `30`. |
| `--trace PATH` | no | Keep the raw trace at `PATH`. Default: a temp file, whose path is printed at the end. |
| `--asserts` | no | Also print each assertion segment in run order, measured call to call. |
| `TEST` | yes | The test file to profile, repo-relative or absolute. Exactly one. |

The test runs under `$DOCKET_BASH_PATH` when it is set, otherwise under the interpreter running the
script.

**Output:** the test's exit status and assertion counts, then the top source lines by cumulative
self time, then the top individual command invocations, then the traced wall time — plus the
assertion segments under `--asserts` — and finally the trace and captured-stdout paths — which are
ALSO printed up front, before the test launches, so a hung run can be diagnosed by reading the
growing trace file from another shell. The test's own output is captured to a file rather than
printed, so the tables are not buried.

## Behavior

### Tracing mechanism

xtrace is enabled through the **environment**, not by editing or sourcing the test:

- `SHELLOPTS=xtrace` — Bash reads this at startup and applies the listed options before running the
  script, which is the one way to trace a file you may not modify. It must be set with `env` rather
  than a command-prefix assignment, because `SHELLOPTS` is readonly in the shell that would carry
  the prefix, and the assignment aborts the command instead of exporting it.
- `BASH_XTRACEFD=9`, with fd 9 redirected to the trace file, so trace output never reaches the
  test's own stdout or stderr and cannot corrupt what the test asserts on.
- `PS4` carrying `$EPOCHREALTIME`, `$BASHPID`, and the source file and line, expanded by the child
  once per traced command.

Sourcing the test was rejected: many files in this suite read `$0` to locate the repo root, and
under `source` they would resolve it against the profiler. Running the test as its own process
keeps `$0`, `BASH_SOURCE`, EXIT traps, and the exit status exactly as a plain run leaves them.

### Attribution

A traced command's **self time** is the gap until the next trace line. Processes here run
sequentially — the parent waits on each child — so the interleaved trace is one sound timeline, and
`$BASHPID` in the prefix keeps the producing process identifiable when it is not.

This yields the useful property that an external command (`git`, `grep`) or a child script is
charged its full runtime, while a function-call line is charged only the gap before its first inner
command. Timestamps are parsed by splitting on the separator and recombining as integer
microseconds — exact under awk's doubles, where parsing the whole value as a float would not be.

Because `SHELLOPTS` is exported, **child Bash processes are traced too**: the profile reaches inside
docket's own scripts under test and names the offending line there, instead of charging the whole
cost to the one line that invoked them. This is the main reason to prefer this tool over reasoning
from the segment view.

Under `--asserts`, the segment table is rebuilt from depth-1 trace lines whose command is an
assertion helper — PS4 repeats its first character per nesting level, which is what makes the depth
filter possible. Measured call to call, these are the exact boundaries the stream-based segments in
`profile-asserts.sh` approximate.

### Fidelity, and its limits

**Read the ranking, not the clock.** xtrace writes a line per executed command, so absolute times
run above a clean run's, and the inflation is not uniform: a loop of cheap builtins is taxed far
more heavily than one long `git` call. Comparing two runs of this tool is meaningful; comparing its
output against `profile-asserts.sh` is not.

A test that asserts on a child process's exact stderr could in principle be perturbed, since every
child inherits the traced environment. The reported exit status is the signal: a test that passes
plainly and fails here has been perturbed, and its numbers should be discarded.

## Exit codes

| Code | Meaning |
|---|---|
| 0–255 | The profiled test's own exit status, passed through unchanged. |
| 1 | Also the code when the interpreter is pre-Bash-5 and no usable `runtime.bash` was configured to re-exec under (the message names the remedy). |
| 2 | Usage error: unknown flag, a flag missing its argument, a non-numeric `--top`, more than one `TEST`, or a `TEST` that does not exist. |

## Invariants

- **The test file is never modified and never sourced.** Tracing is environmental, so `$0`,
  `BASH_SOURCE`, EXIT traps, and the exit status behave as in a plain run.
- **Trace output cannot reach the test's own streams.** It goes to fd 9 by way of `BASH_XTRACEFD`;
  a test's stdout and stderr are captured separately.
- **One awk implementation of self-time attribution.** The per-line and per-invocation tables are
  two aggregations of a single computed dataset, never two independently written scans that could
  drift.
- **Artifacts outlive the run.** The trace and the captured stdout are deliberately not removed on
  exit — their paths are printed for follow-up reading.
- **Bash 5+, and loud about it.** Same `EPOCHREALTIME` floor, same re-exec path through
  `$DOCKET_BASH_PATH`, and the same sentinel against an endless re-exec as `profile-asserts.sh`.
- **No early-exiting consumer under `pipefail`.** Tables are truncated with `awk NR<=top`, never
  `head` (AGENTS.md, *Shell*).
- **Reports, never gates.** Nothing in docket consumes its output and no skill invokes it.
