---
slug: parse-error-truncates-the-rest-of-the-file
hook: "A parse error kills every line after it, so an older interpreter can silently delete most of a test file's asserts — gate the version at the top of the file, not only in the wrapper that re-execs."
topics: [testing, shell-portability, environment]
changes: [246]
created: 2026-08-08
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
A missing assert and a *failing* assert look nothing alike in a report, but an interpreter that
cannot **parse** a construct produces neither: it stops reading the file there and everything below
is simply not code any more. On a test file that means the run reports an error about one line
while silently discarding every assert after it — and the count it prints is the count it reached,
which reads as the file's real size to anyone who does not already know better.

Two rules:

1. **Gate the interpreter version at the top of the file the documented run line actually invokes.**
   A wrapper that re-execs under a newer interpreter protects only the wrapped path. The file's own
   header ("run this with `bash tests/test_x.sh`") is a second entry point with no such protection,
   and it is the one a human uses when debugging a single test.
2. **Treat an unexplained assert count as a truncation hypothesis.** When a file's totals move a
   lot for a change that did not add or remove tests, ask what stopped being parsed rather than
   what stopped passing.

The specific hazard worth remembering: bash 3.2 — still `/bin/bash` on macOS — cannot parse a
heredoc **inside** a `$(...)` command substitution. Modern bash accepts it, so the construct is
invisible in every local run until `PATH` resolves to the system binary. See [[shell-portability]]
for the broader class and [[agent-shell-noop-reads-as-success]] for the sibling failure where a
verification step reports success having examined nothing.

## War story
- 2026-08-08 (#246, PR #179) — The stub filed this as a **grep portability** defect: a `{0,600}`
  interval that the system grep rejects. The live reproduction found something else entirely.
  Under `PATH=/usr/bin:/bin`, `bash` is `/bin/bash` 3.2.57, whose parser cannot see the heredoc
  inside the `scope_guard_awk` command substitution at line 688 — it chokes on a backtick within
  it and everything from line 684 to EOF stops being parsed. The file ran **103 of its 393
  asserts** and exited 2 with a cryptic parse error; the 290 that vanished included the entire
  region a *second* open change (#0187) existed to harden. `scripts/run-tests.sh` was already
  protected by its Bash 4.3+ re-exec, so the whole-suite path was green and honest — the exposed
  path was direct invocation, which is the run line printed in the file's own header. Fixed with a
  `bash >= 4` fail-fast gate at the top of the file, deliberately in preference to de-backticking
  the heredoc: the gate covers every construct with this hazard, present and future, while the
  rewrite covers one. What makes it worth recording is the misdiagnosis, not the fix — a
  reproduction that only confirmed the *symptom* (system-bash run is broken) would have shipped
  the ERE conversion and left 290 asserts dark.
