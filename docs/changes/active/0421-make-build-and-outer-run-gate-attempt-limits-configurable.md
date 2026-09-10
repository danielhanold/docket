---
id: 421
slug: 'make-build-and-outer-run-gate-attempt-limits-configurable'
title: 'Make build and outer run gate attempt limits configurable'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [349, 359, 407, 419]
discovered_from: []
adrs: [19, 74, 102, 107, 111]
spec: 'docs/superpowers/specs/2026-09-10-make-build-and-outer-run-gate-attempt-limits-configurable-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/make-build-and-outer-run-gate-attempt-limits-configurable'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-10T01:15:38Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-10-make-build-and-outer-run-gate-attempt-limits-configurable-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-10-make-build-and-outer-run-gate-attempt-limits-configurable-design.md) |
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md), [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md), [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md) |
<!-- docket:artifacts:end -->

## Why

The build gate's fixed repair bound can stop useful work before multiple test failures are resolved. The outer run gate separately hardcodes a single retry for an eligible incomplete implementation run. Users need explicit, independently configurable limits with consistent counting.

## What changes

- Add build.max_attempts with a built-in default of 4 and run.max_attempts with a built-in default of 2.
- Both limits count total attempts including the initial attempt. A value of 1 disables retries; only positive integers are valid.
- Build permits the initial full-suite run plus up to three repair-and-rerun cycles by default. The outer gate retains the initial implementation run plus one eligible retry by default.
- Resolve both through normal configuration precedence, propagate typed values to their owners, and expose clear attempt usage and exhaustion diagnostics.
- Preserve durable accounting, attribution, continuation and explicit-halt semantics: waiting and continuation consume no additional attempt, and an outer retry cannot override a build halt requiring human help.
- Update maintained workflow instructions, generated gate surfaces and tests consistently with the configured limits.

## Out of scope

Progress-detection heuristics, unlimited retries, changing finalize repair or resolver limits (tracked separately in change 0419), altering review-fix-loop limits, resetting budgets on continuation, weakening tests, and changing attribution, permission, or human-halt rules.
