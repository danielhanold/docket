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
related: [318, 352, 363, 367, 369, 370, 371, 372]
discovered_from: [370]
adrs: [12, 14, 29, 30, 33, 36, 52, 74, 92, 99]
spec: 'docs/superpowers/specs/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md'
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
| Spec | [2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Change 0370 cannot delete the frozen Bash facade while retained workflow skills still rely on it for repository preparation and lifecycle helpers. Change 0369 intentionally deferred capabilities without an exact Go home, and the halted 0370 build proved that deletion cannot safely absorb them. Change 0377 closes that gap as a separate, independently mergeable predecessor.

## What changes

- Add structured `docket repository prepare --json` as the shared Step-0 operation, with typed context and fail-closed metadata-worktree synchronization.
- Map retained facade consumers to existing typed context, status, maintenance, finalize, ADR, and change capabilities, extending only narrow gaps rather than cloning Bash verbs.
- Eliminate routine Board-pass and direct-renderer calls; board, artifact-link, and ADR-index views are rendered by their owning mutation transactions.
- Extend repository check and authorized migrate repair for deterministic board, artifact-link, ADR-index, and ADR-ledger drift.
- Rewire canonical skills and generated products, then add a mutation-tested, shape-derived guard proving no maintained executable consumer needs the migrated facade operations.
- Leave the Bash facade present and frozen for change 0370 to delete after an explicit human resume.

## Out of scope

Physical facade or legacy-suite deletion (0370); resuming, dispatching, planning, implementing, or opening a PR for 0370; one-for-one compatibility verbs or a forwarding shim; retired/deferred product features, GitHub board mirroring, or main mode; lifecycle, topology, or transaction redesign; historical and frozen-artifact rewrites; and release or fresh-host work.
