---
id: 278
slug: test-docket-example-yml-s-fidelity-fixture-goes-intermittent
title: 'test_docket_example_yml''s fidelity fixture goes intermittently red under parallel contention'
status: killed
priority: medium
type: fix
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [271]
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

**Trigger** — surfaced at change 0271's finalize merge gate (2026-08-09). The first full
`scripts/run-tests.sh` run reported `SUITE files=100 passed=99 failed=1` with
`test_docket_example_yml` at `rc=1 ok=428 notok=1`. The single failing assert was the fidelity
fixture's own vacuity guard: `NOT OK - fidelity fixture: example reached the fixture's origin/main`.
Re-running the file in isolation was green (exit 0, 429 asserts, 0 failures), and a second full
parallel suite run was green (100/100, 7485 asserts). The gate only passed because a human-directed
re-run was performed; an unattended finalize would have read a red suite and dispatched
`docket-integration-repair` against a defect that does not exist in the code under test.

**Opportunity** — `tests/test_docket_example_yml.sh` builds its fidelity fixtures with an
*unchecked* `mkrepo` + `cp` + `git add` + `git commit` + `git push` sequence (the file's own comment
says so, which is why the vacuity guard exists at all). Under the parallel runner's load one of
those git operations intermittently does not land, and the guard correctly reports it — the guard is
working; the fixture setup is what is unreliable. Making the fixture setup fail loudly and
deterministically (check each command's exit status at the point it runs, retry or abort with the
failing command named) converts an intermittent whole-suite red into either a green run or an
immediately diagnosable one.

**Independent value** — an intermittently red file in the suite is a defect in the merge gate
itself: it makes `finalize`'s red/green verdict non-deterministic, which is the one property the
gate exists to provide. The value stands with change 0271 entirely reverted — nothing about
0271 touched the fixture-setup path, and the same flake can redden any future finalize run.

**Boundary** — the fixture setup inside `tests/test_docket_example_yml.sh` (and, if the same
unchecked `mkrepo`-then-push shape is used elsewhere, the sibling files sharing it). It deliberately
leaves alone: the wall-clock budget regime (changes 0251 / 0273 own that), the parallel runner's
scheduling in `scripts/run-tests.sh`, and the separately-tracked reclaim-leg flake in
`test_docket_status`, which is a different file and a different mechanism.

**Reason for deferral** — 0271 is a delegation-execution-posture change; its branch touched
`.docket.example.yml` and this test file only to add config-key coverage. Root-causing a
contention-dependent git failure inside a test fixture, and re-proving the fix over repeated full
parallel runs, is its own investigation with its own evidence burden, and folding it into 0271
would have expanded that branch well past its stated scope.

## Why killed

Consolidated into #0252 at the 2026-08-09 backlog triage: 0252 (via absorbed #0243) already
designs away exactly this defect — its spec names `test_docket_example_yml.sh`'s `mkrepo`
(:24-32) *and* the fidelity-fixture `cp`/`add`/`commit`/`push` sequence (:45-50, "the site that
reddened 0190's gate") as `fx`-adoption sites, converting every unchecked fixture step into a
loud, named abort. This stub's sketch ("check each command's exit status, retry or abort naming
the failing command") is `fx`, and its retry-or-abort question is already ruled there: hard
abort, no retry. The 2026-08-09 live occurrence is recorded in 0252's Why as fresh evidence.
