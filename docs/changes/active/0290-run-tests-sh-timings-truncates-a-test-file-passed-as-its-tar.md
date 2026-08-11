---
id: 290
slug: run-tests-sh-timings-truncates-a-test-file-passed-as-its-tar
title: 'run-tests.sh --timings truncates a test file passed as its target'
status: proposed
priority: medium
type: fix
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: []
discovered_from: [281]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced during change 0281's build. The plan directed
`scripts/run-tests.sh -j 1 --timings <test path>`. `--timings` takes an **output** path, so the
named test file became the timings sink and was truncated to **zero bytes**. It happened once for
real; the file was reconstructed byte-identically from git and the worker worked around the
malformed invocation rather than fixing the tool (the plan is a frozen build record). It is
currently written up only in that change's results file.

**Opportunity** — `run-tests.sh` has no protection against a caller putting a source file where an
output path belongs. The option takes a path, writes to it unconditionally, and the argument grammar
gives no signal that the next token is a sink rather than a subject — every other positional
argument to the script *is* a test path. Nothing in the script distinguishes "a path I am about to
destroy" from "a path I am about to read". Candidate shapes: refuse a `--timings` target that
matches a tracked test path (or `tests/test_*.sh`), refuse a target that already exists unless
`--force`, require an explicit `.tsv`-shaped sink, and/or write via a temp file so a rejected target
is never opened for truncation. Exact leg is for the groom.

**Independent value** — this is silent data loss in a script humans and agents run directly against
their own working tree, on a repo whose own convention treats `run-tests.sh` as the canonical suite
entry point. It stands entirely on its own: change 0281 is reverted and the bug is unchanged. The
damage class is the worst kind — a destroyed source file with a **green-looking** command, no error,
and no diagnostic; the 0281 build only noticed because it happened to re-read the file.

**Boundary** — the fix is confined to `scripts/run-tests.sh`'s `--timings` argument handling, its
`scripts/run-tests.md` contract, and a guard under `tests/`. It deliberately leaves alone: the
timings feature itself and its output format, the runner's parallelism and budget machinery, and
every other option's argument handling (a survey of the other path-taking options is in scope only
as far as reporting whether the same shape exists elsewhere).

**Reason for deferral** — 0281 was a prose-and-guard change to the auto-groom critic return channel
with no script edits at all; fixing a runner's argument handling and adding a destructive-write
guard would have expanded that branch past its stated scope, and its plan was already frozen.
