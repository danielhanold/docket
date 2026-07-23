---
id: 19
slug: finalize-ci-gate-functional-test
title: Finalize ci/both gate — functional test against real GitHub CI (poll/retry)
status: proposed
priority: low
created: 2026-06-17
updated: 2026-06-17
depends_on: [15]
related: [15]
adrs: [10]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: chore
---

## Why

Change 0015 added the finalize **rebase-retest gate** with four modes (`local` ·
`ci` · `both` · `off`), but only the **`local`** path is meaningfully exercised. By
design (0015 spec §2), the test suite asserts the gate's *mechanics* — config-parse of
all four modes, dispatch sentinels, abort paths — not end-to-end behavior; and docket
itself has no GitHub CI, so the **`ci`/`both`** paths have never run against a real CI
system. They are the least-proven branch of the gate: the agent reads CI status via
`gh pr checks <pr>` after a `--force-with-lease`, but the polling, the red-or-absent-checks
abort, and how `docket-integration-repair` reconciles a CI-only failure it must reproduce
locally are all untested. This change closes that gap so a consuming repo on `ci`/`both`
can trust the gate.

## What changes

Add a **functional test** for the `ci`/`both` gate path against real (or faithfully
simulated) GitHub CI, including the **poll/retry logic** — GitHub checks can take several
minutes, so the gate must poll `gh pr checks` with a bounded timeout + backoff rather than
read once. Scope to be settled at brainstorm, but covers: a green-CI → merge path, a
red-CI → `docket-integration-repair` → re-validate path, the **absent-CI-checks** straight
abort, and a **timeout** abort.

## Out of scope

- The gate's design itself (owned by change 0015 / ADR-0010) — this is test + any
  poll/retry hardening the test reveals, not a redesign.
- The `local`/`off`/config-parse coverage already shipped in 0015.

## Open questions

- **How to test against real CI without flakiness?** A dedicated fixture repo with a
  trivial GitHub Actions workflow, a recorded/replayed `gh pr checks` fixture, or a mock
  `gh` shim? Each trades fidelity for speed/determinism.
- **Poll/retry policy:** timeout ceiling, backoff interval, and how "pending forever"
  (a stuck/queued check) is distinguished from "absent checks" (nothing configured) — both
  currently fold into the §7 "red or absent CI checks → abort" path.
- Does this also need to harden the *production* `ci`-mode prose in
  `docket-finalize-change` (the 0015 results flagged a `ci`-mode abort-prose ambiguity), or
  is that a separate doc tweak?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
