---
id: 243
slug: make-test-suite-git-fixture-setup-fail-loudly-instead-of-fla
title: Make test-suite git fixture setup fail loudly instead of flaking
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [190]
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

**Trigger** — surfaced during change 0190's post-fix suite gate. The full suite went red with a
single failure, `test_docket_example_yml` at the assert "fidelity fixture: example reached the
fixture's origin/main". The same file then passed 13 consecutive standalone runs, including an
8-way concurrent reproduction attempt. The red was a transient failure inside the test's own
`mkrepo` fixture setup (`git init`/`add`/`commit`/`push` to a local remote), not a regression.

**Opportunity** — the suite has no mechanism making fixture setup failures deterministic. Several
tests build throwaway git repos with unchecked `cp`/`add`/`commit`/`push` sequences under `set +e`;
`test_docket_example_yml.sh` is notable only because it already carries a hand-written non-vacuity
guard that *caught* the silent failure. A shared, checked fixture helper — one that fails loudly and
immediately on a setup command's non-zero exit, and optionally retries a transient local-remote
push — would give every such test the same protection without each author reinventing it.

**Independent value** — stands entirely with 0190 reverted. A flaky gate is expensive well beyond
one change: `docket-implement-next`'s fix loop treats a red post-fix suite as grounds to revert
every non-blocker fix commit, so a transient fixture failure can discard a run's worth of genuine
review remediation. That cost is paid by every future build, not by this one.

**Boundary** — a shared checked-fixture helper (or hardening of the existing per-test `mkrepo`
functions) plus adoption in the tests that build throwaway git repos. It deliberately leaves alone:
the suite runner's parallelism, the runtime budget table, and any test's assertions about product
behavior. Diagnosing whether the underlying transient is git, the filesystem, or contention is in
scope only as far as choosing between fail-loudly and retry.

**Reason for deferral** — 0190's branch is a merge-gate predicate extension plus its trust-boundary
guard. Reworking a shared test-fixture idiom across the suite is an orthogonal concern touching
files 0190 has no reason to open, and folding it in would expand the branch well past its spec.
