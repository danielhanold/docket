---
id: 186
slug: bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui
title: Bare mv prompts on a tty — backfill-change-types hangs the suite and can exit 0 without installing
status: proposed
priority: high
type: fix
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: []
discovered_from: [185]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_backfill_change_types.sh` hangs forever when the suite is run from an interactive
terminal, which makes the full local suite unfinishable for a human — the exact environment a
maintainer runs the merge gate in. It passes green in every non-tty context (agent shells, the
finalize gate), so the suite has been reporting clean while being unrunnable by hand since
2026-07-23 (change 0127).

**The hang.** `scripts/backfill-change-types.sh:161` installs each staged file with a bare `mv`.
The test (line 201) makes one destination immutable via `chflags uchg` to force a mid-loop install
failure and exercise the rollback path. BSD `mv` handles an unwritable destination by *prompting*
— `override rw-r--r-- homer/wheel for dst.md? (y/n [n])` — but only when stdin is a terminal. In a
terminal it blocks on that read forever; with stdin at /dev/null or a pipe it skips the prompt,
fails `EPERM`, and the rollback assertions pass.

**The worse bug the hang is hiding.** Probed under a pty with stdin at EOF, `mv` declines the
overwrite and exits **0**:

```
override rw-r--r--  homer/wheel for dst.md? (y/n [n]) not overwritten
MV_RC=0
```

So there is an environment where the staged file is silently not installed, `if ! mv` never fires,
no rollback runs, and the script reports success — a half-migrated backlog with a zero exit. That
is precisely the outcome the test's own comment (lines 186–191) says the install's undo exists to
make impossible. The guard is sound only because the environments that run it never have a tty.

Diagnosed 2026-08-01 while investigating why `scripts/profile-one-test.sh
tests/test_backfill_change_types.sh` produced no output; the process tree showed the profiler
blocked 6m47s in `backfill-change-types.sh --map 1=fix,2=docs,4=chore` against the `rollback`
fixture.

## What changes

- **`mv -f` at `scripts/backfill-change-types.sh:161`** — suppresses the prompt and returns
  non-zero on `EPERM`, preserving the test's intent rather than working around it. This is the only
  part that unblocks the suite.
- **Audit the `cp -p` twin at line 163** (the rollback restore) and any other install/restore call
  in this script for the same tty-dependent prompt exposure.
- **Make the test prove the property in both environments** — the current assertions are only
  honest without a tty. Pin that the install returns non-zero and rolls back regardless of whether
  stdin is a terminal, so the silent-success path cannot come back.
- **Two `profile-one-test.sh` ergonomics fixes** (from change 0185, merged): print the trace path
  *before* launching the child — today it is printed only on success (line 137), so the one
  artifact that explains a hang is unreachable while it hangs; and flush the `tracing …` line
  (line 77), which is invisible under any redirect or pipe and turned "hanging" into "no output at
  all". `scripts/profile-asserts.sh` shares both shapes and should be checked.
- **A learnings finding**: a guard that forces a failure via a filesystem flag is only sound if the
  tool under test does not prompt — non-tty stdin was concealing both a hang and a silent-success
  path. A sharper instance of the existing `agent-shell-noop-reads-as-success` finding.

## Out of scope

- No broader audit of BSD-vs-GNU prompting across the whole script tree. This change fixes the one
  proven site, its immediate twin, and records the rule; a sweep is its own change if the finding
  justifies one.
- No change to how the suite is invoked (no blanket `</dev/null` at the runner level) — that would
  re-hide this class of defect rather than fix it.

## Open questions

- Should the tty-vs-non-tty property be pinned by running the affected assertions under a pty
  (`script -q /dev/null …`), or by asserting the `-f` flag's presence at the call site? The first
  tests behavior and is portable-ish; the second is cheap but is a sentinel over source text.
