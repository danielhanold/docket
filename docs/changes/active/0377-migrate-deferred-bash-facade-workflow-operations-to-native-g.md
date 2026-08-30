---
id: 377
slug: 'migrate-deferred-bash-facade-workflow-operations-to-native-g'
title: 'Migrate deferred Bash-facade workflow operations to native Go CLI verbs'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: [372]
stacked_on:
related: [318, 369, 370, 371, 372]
discovered_from: [370]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Change 0370 cannot delete the frozen Bash facade while retained docket workflow skills still use it for bootstrap and lifecycle operations that have no Go CLI equivalent. Change 0369 deliberately deferred this surface, so it needs an explicit predecessor rather than an in-run expansion of 0370's deletion scope.

## What changes

- Derive the complete maintained executable-consumer inventory for the deferred facade operations.
- Add native Go CLI operations for preflight/bootstrap, board refresh and status, change-link rendering, ADR index rendering and checks, stack base/children/closeout, and plain status.
- Cut the canonical workflow skills and their generated maintained copies over to those operations while preserving fail-closed bootstrap, typed outcomes, configuration resolution, metadata-worktree synchronization, derived-view ownership, and stack semantics.
- Add mutation-sensitive Go coverage and a shape-derived absence guard proving no maintained executable consumer still needs these facade operations.
- Leave the repository ready for 0370 to be re-groomed and resumed solely for physical Bash-facade and legacy-test deletion.

## Out of scope

Deleting scripts/docket.sh, scripts/lib, the legacy runner, or legacy tests (0370); resuming, re-dispatching, or implementing 0370; redesigning settled Go-migration architecture or lifecycle policy; changing deferred product features; and rewriting archived changes, Accepted ADR history, frozen specs/results, or v0.9.2 artifacts.
