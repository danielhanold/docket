---
id: 377
slug: 'migrate-deferred-bash-facade-workflow-operations-to-native-g'
title: 'Migrate deferred Bash-facade workflow operations to native Go CLI verbs'
status: 'implemented'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-31'
depends_on: [372, 378]
stacked_on:
related: [318, 352, 363, 367, 369, 370, 371, 372, 378]
discovered_from: [370]
adrs: [12, 14, 29, 30, 33, 36, 52, 74, 92, 99]
spec: 'docs/superpowers/specs/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md'
plan: 'docs/superpowers/plans/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/migrate-deferred-bash-facade-workflow-operations-to-native-g'
pr: 'https://github.com/danielhanold/docket/pull/257'
blocked_by:
reconciled: true
claimed_at: '2026-08-30T23:40:49Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g-design.md) |
| Plan | [2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g.md) |
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

## Reconcile log

### 2026-08-30 — predecessor decision from grooming 0378

The human approved 0378 as a separate, independently mergeable prerequisite for the shared
metadata-root ownership fix. Wait for 0378 to reach `done`; on the later explicitly authorized
continuation, reconcile/rebase this work onto it and make `repository prepare` consume its shared
verifier rather than retaining `prepareAugment`'s copied root-equals-tip predicate. Preserve the
prepare-specific clean-worktree, fast-forward-only, and divergence guards.

This records the scope decision only. The run remains halted; its branch, committed tasks,
uncommitted artifacts, and halt record are unchanged. This grooming does not resume 0377 or 0370.

### 2026-08-30

### 2026-08-30 — reconcile at claim

Verified the design against the current `docket`/`main` tree at claim time; no scope change required.

- **Dependency 372 is `done`** and its work (retire deferred feature families) is merged; 377's dependency boundary holds.
- **`docket repository prepare` is genuinely absent** — the `repository` command group currently exposes only `init`, `check`, and `migrate` (`internal/cli/repository.go`, `internal/app/repository_{init,check,migrate}.go`). The new `repository.prepare` operation must be added, not reconciled away.
- **The Go config/topology foundation prepare builds on already exists**: `internal/config` carries a full resolver including `preflight.go`, `resolve.go`, `schema.go`, and the bootstrap 2×2; `internal/app/repository_facts.go` supplies the classifier facts; `SetupDeps`/`RunRepository*` provide the CLI→app seam. Preparation is a new closed operation over these, not new topology logic.
- **The Bash facade remains frozen and present** — `scripts/docket.sh preflight` still resolves config for the shared Step-0 preamble, confirming the migration host is intact and 0370 has not deleted it. `Out of scope` (physical facade/legacy-suite deletion, 0370 resume) is unchanged and still correct.
- **370 remains run-halted** and is untouched by this change; the independent-merge relationship (377 not stacked on 370) is intact.

No obsolescence and no fundamental design invalidation found. Proposal sections and relations left as authored.

### 2026-08-30

### 2026-08-30 — resume after 0378 merged (blocker resolution b)

The halt's recommended resolution (b) was taken: the shared metadata-root ownership classifier was split into change 0378 and is now MERGED to origin/main. This branch was rebased cleanly onto main (tip 7722a8f5); Task 1 (repository_prepare.go app op) and Task 2 (repository prepare CLI verb) remain committed and correct.

Verified against the rebased base:

- 0378 landed the shared verifier `verifyMetadataOwnership` (internal/app/metadata_ownership.go) plus the receiptless legacy-equivalence path, and removed the duplicated root-equals-tip predicates from check/init/migrate (commits 857a07f7, 336f23f8, b35a4991, ...). `repository check --json` now classifies the real multi-commit docket branch as owned (RootParentless), not `metadata-root-foreign`.
- 0377's own `prepareAugment` (internal/app/repository_prepare.go, committed at Task 1) still carries the OLD copied predicate `len(roots)==1 && roots[0]==metaTip`, which misclassifies the real docket chain as RootForeign. Per the predecessor decision recorded above, prepare must be rewired to consume 0378's shared `verifyMetadataOwnership` (setting `f.MetadataRoot = own.Shape`, mirroring repository_check.go incl. the fetch-error→RootUnknown handling), while preserving prepare's own clean-worktree, fast-forward-only, and divergence guards. This rewire is folded into Task 3, whose integration tests against a real multi-commit local repo are its proof.
- Relations unchanged: depends_on [372, 378] both `done`; no scope change to proposal sections or non-goals.

Continuing the build from Task 3 onward (Tasks 3–14) with TDD; the two uncommitted Task-3 artifacts left for inspection are re-derived against the merged 0378 fix rather than adopted as-is.
