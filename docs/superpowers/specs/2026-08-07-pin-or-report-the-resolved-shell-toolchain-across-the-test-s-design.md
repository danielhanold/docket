<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0150 — Pin or report the resolved shell toolchain across the test suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0150-pin-or-report-the-resolved-shell-toolchain-across-the-test-s.md)**
<!-- docket:backlink:end -->

# Toolchain report across the test suite — design (change 0150)

**Date:** 2026-08-07 · **Change:** 0150 · **Type:** chore

## Problem

Change 0130 proved the failure shape: a portability suite can silently exercise a different tool
than the one it targets (PATH `grep` = ugrep accepts what BSD grep rejects), running green while
the bug is real. 0130 shipped a static guard plus one informational line naming the resolved
`grep` inside `tests/test_grep_portability.sh` (:84-93). Nothing else in the suite records which
toolchain a run actually resolved, so every other gate log is silent on the question.

The 2026-08-07 triage settled the posture: **report, never pin** (a pin generalizes 0130's
recorded false-failure hazard); no per-site absolute-path guard; no CI matrix. This spec designs
only the report.

## Design

### 1. `tests/lib/toolchain-report.sh` — a sourceable helper, single implementation

A new sourceable file (sibling of `tests/lib/sync_agents_common.sh`, outside the `tests/test_*.sh`
discovery glob, so it never runs as a test) defining one function:

```
toolchain_report()   # prints one line per tool; always returns 0
```

- Tool set, fixed in the helper: `grep sed awk date readlink` — the stub's list, the five
  PATH-resolved utilities whose GNU/BSD divergence has actually bitten this repo.
- Per-tool line keeps 0130's exact shape: `#    - resolved <tool>: <path> (<first version line>)`,
  falling back to `unknown` / `version unavailable`. The `#` prefix keeps the lines outside the
  runner's assert counting (`^ok -` / `^NOT OK`).
- Implementation lifted from `tests/test_grep_portability.sh:87-93` generalized to a loop:
  `command -v` for the path; `<tool> --version 2>/dev/null || true` captured into a variable and
  first-line-extracted via here-string — the capture-then-here-string SIGPIPE discipline carries
  over verbatim (a producer feeding an early-exiting consumer under pipefail can become an
  intermittent 141; AGENTS.md, Shell).
- **Gating nothing, permanently.** The function never exits non-zero and asserts nothing. BSD
  `sed`/`date`/`readlink` reject `--version`; on those the line reports the path plus
  `version unavailable`, and the path is the discriminating datum anyway.
- Sourced, not executed: a `command -v` probe must run in the consumer's own process/PATH, and an
  executable's own shebang could resolve a different environment than the caller's.

### 2. `tests/test_grep_portability.sh` calls the helper

Replace the inline block at :87-93 with:

```
. "$ROOT/tests/lib/toolchain-report.sh"
toolchain_report
```

keeping the explanatory comment above it (trimmed to point at the helper). One implementation,
two call sites — no drifting restatement. Verified before designing this: nothing in the repo
greps the `resolved grep:` line, so the format is not load-bearing anywhere
(learnings: restatement-accumulates-its-own-guards).

The file gains one smoke assert (it already has the ok/nok harness): the helper's captured output
has exactly 5 lines, each matching `^#    - resolved [a-z]+: `, and `toolchain_report` returned 0.
Capture into a variable first, same SIGPIPE discipline. This is structural (line count/shape),
not a content grep, so BSD-vs-GNU version text cannot redden it.

### 3. `scripts/run-tests.sh` emits the report once per suite run

At the top of the run — after argument parsing, target validation, and the budget-table load
(so usage errors stay clean), immediately before the launch loop:

```
. "$REPO/tests/lib/toolchain-report.sh"
toolchain_report
printf '#    - test bash: %s (%s)\n' "$TEST_BASH" "<first line of "$TEST_BASH" --version>"
```

- **stdout**, not stderr: gate logs (finalize's `test_command`, docket-build's gate) capture
  stdout — "every gate log records which toolchain actually ran" is the whole point. The runner's
  stdout contract is "byte-stable across `-j`" (run-tests.md); toolchain lines are machine-
  dependent but `-j`-independent, so the contract holds.
- The `test bash` line is runner-owned, not part of the helper: `command -v bash` is NOT what
  executes the tests — `$TEST_BASH` (`DOCKET_BASH_PATH` fallback `command -v bash`) is, and the
  runner is the sole authority on it. Same capture discipline, same `|| true` tolerance.
- One report per run, from the runner process: jobs inherit the runner's PATH (the launch sandbox
  overrides HOME/TMPDIR/git config, never PATH), so the runner's resolution is representative of
  every job.

### 4. Docs

One short paragraph in `scripts/run-tests.md` under the stdout/stderr section: the toolchain
header, why it gates nothing, and that its lines are machine-dependent (excluded from any
byte-comparison across machines, stable across `-j` on one machine).

Amend the three sites stating an interrupted run "has printed nothing but the stderr ticker" —
the runner's interruption comment (`scripts/run-tests.sh:189-195`), `run-tests.md` §Interruption,
and the comment at `tests/test_run_tests.sh:283` — to "nothing but the toolchain header and the
stderr ticker". Once the header prints before the launch loop that sentence is otherwise false;
no assert keys on it (critic-verified), but the prose must not lie.

## Out of scope

- Any pin — PATH pin or per-site assert (settled at triage; a pin is the false-failure hazard).
- Mandating that other test files source the helper.
- Re-opening 0130's static bound guard.
- CI-matrix runs; change 0229's budget-basis work.

## Assumptions

1. **Tool set = the stub's five (`grep sed awk date readlink`), plus a runner-owned `test bash`
   line.** Rejected: a longer roster (git, perl, jq, gh, sort, find) — version-stable across the
   BSD/GNU axis this report exists for, and roster growth is cheap to add later when a tool
   actually bites. Rejected: folding bash into the helper — `command -v bash` inside the helper
   would misreport, since the runner executes tests under `$TEST_BASH`, which the helper cannot
   know; the runner prints that line itself.
2. **Report, never pin — carried forward as settled.** The stub's triage re-scope fixed this
   after verifying the original abstain's analysis; this spec does not reopen it. No assert that
   resolved grep IS /usr/bin/grep.
3. **Sourceable function, not an executable script.** Rejected: standalone executable — its
   shebang could resolve a different bash/PATH than the consumer, and the probe must reflect the
   consumer's environment. `tests/lib/` already establishes the sourced-prologue pattern
   (`sync_agents_common.sh`, change 0227). The helper sets no shell options and defines one
   function, so sourcing is side-effect-free.
4. **Runner emits on stdout, before the launch loop.** Rejected: stderr — the live ticker channel
   is racy-by-design and gate logs may not keep it. Rejected: report footer — an interrupted run
   (SIGINT discards the buffered report) would then record no toolchain at all, and the header
   position costs nothing. Byte-stability across `-j` is preserved because resolution does not
   vary with `-j`.
5. **Best-effort version capture is acceptable.** `--version` fails on BSD sed/date/readlink; the
   helper tolerates it (`|| true`, `version unavailable`) rather than maintaining a per-tool
   version-flag matrix. The learnings caution about best-effort helpers on deliverable paths
   (best-effort-helper-on-a-sole-deliverable-path) does not apply: this output is informational
   by charter and gates nothing — a degraded line is still a truthful record of the path.
6. **No new test file.** The smoke assert rides in `tests/test_grep_portability.sh`, which already
   sources the helper as its production call site; a dedicated file would add a suite job (and a
   budget row) to assert five printf lines. Structural asserts only — no grep of version text.
7. **Coupling: depends on nothing open.** 0227's runner and `tests/lib/` are merged (the seam
   exists); 0130 is done; 0151 is killed. `related:` records 0151 (sibling finding) and 0227 (the
   seam this design hangs off); `discovered_from:` stays 0130.

## Build notes

- Touches: `tests/lib/toolchain-report.sh` (new), `tests/test_grep_portability.sh` (replace
  :84-93 block + smoke assert), `scripts/run-tests.sh` (header emission + interruption-comment
  amendment), `scripts/run-tests.md` (one paragraph + interruption prose), `tests/test_run_tests.sh`
  (interruption comment only — no assert changes needed).
- Verification: run the suite via `scripts/run-tests.sh tests/test_grep_portability.sh` and
  observe the header on stdout; mutation-check the smoke assert by dropping a tool from the
  helper's list (must redden). Also run once with `PATH=/usr/bin:$PATH` prepended to confirm the
  report survives BSD `--version` failures (0130's verification posture).
