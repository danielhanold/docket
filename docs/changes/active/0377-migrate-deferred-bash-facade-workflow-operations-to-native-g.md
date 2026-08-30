---
id: 377
slug: 'migrate-deferred-bash-facade-workflow-operations-to-native-g'
title: 'Migrate deferred Bash-facade workflow operations to native Go CLI verbs'
status: 'in-progress'
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
plan: 'docs/superpowers/plans/2026-08-30-migrate-deferred-bash-facade-workflow-operations-to-native-g.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/migrate-deferred-bash-facade-workflow-operations-to-native-g'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-30T18:15:21Z'
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

### 2026-08-30

### 2026-08-30 — reconcile at claim

Verified the design against the current `docket`/`main` tree at claim time; no scope change required.

- **Dependency 372 is `done`** and its work (retire deferred feature families) is merged; 377's dependency boundary holds.
- **`docket repository prepare` is genuinely absent** — the `repository` command group currently exposes only `init`, `check`, and `migrate` (`internal/cli/repository.go`, `internal/app/repository_{init,check,migrate}.go`). The new `repository.prepare` operation must be added, not reconciled away.
- **The Go config/topology foundation prepare builds on already exists**: `internal/config` carries a full resolver including `preflight.go`, `resolve.go`, `schema.go`, and the bootstrap 2×2; `internal/app/repository_facts.go` supplies the classifier facts; `SetupDeps`/`RunRepository*` provide the CLI→app seam. Preparation is a new closed operation over these, not new topology logic.
- **The Bash facade remains frozen and present** — `scripts/docket.sh preflight` still resolves config for the shared Step-0 preamble, confirming the migration host is intact and 0370 has not deleted it. `Out of scope` (physical facade/legacy-suite deletion, 0370 resume) is unchanged and still correct.
- **370 remains run-halted** and is untouched by this change; the independent-merge relationship (377 not stacked on 370) is intact.

No obsolescence and no fundamental design invalidation found. Proposal sections and relations left as authored.

## Run halted

### 2026-08-30

**Halted at build Phase 1, Task 3 (integration tests), 2026-08-30.** A `standard` build worker returned `BLOCKED` on a genuine, empirically confirmed defect that requires a human scope/design decision the autonomous loop cannot make.

### What is built and committed on `refactor/migrate-deferred-bash-facade-workflow-operations-to-native-g`

- **Task 1** (731aa3e1): `internal/app/repository_prepare.go` + test — `RunRepositoryPrepare`, `PrepareContext`, refusal matrix, `const OperationRepositoryPrepare`.
- **Task 2** (029ab59c): `docket repository prepare --json` CLI verb (+ asset-independent allowlist entry).

Tasks 3–14 are unbuilt. Task 3's uncommitted artifacts (`internal/app/repoprepare_integration_test.go`, `tests/test_go_integration_app_repoprepare.sh`) were left in the worktree for inspection; they encode the empirical proof below.

### The blocker

`repository prepare` — the change's central deliverable, intended to replace `docket.sh preflight` as the shared Step-0 operation on real repos — reuses the shared metadata-root classifier predicate `len(roots)==1 && roots[0]==metadataTip` (`internal/app/repository_prepare.go` `prepareAugment`, copied verbatim from `repository_check.go`, `repository_init.go`, and `metadataRootParentless` in `repository_migrate.go`).

That predicate is only true for a **one-commit** metadata branch. The real docket branch is a **3722-commit chain** with a single orphan seed root (`f8b226f2` — "seed metadata branch from main"); its tip has one parent. `git rev-list --max-parents=0 origin/docket` returns exactly that one root, which is not the tip, so the predicate evaluates FALSE and the branch is classified `RootForeign`.

**Confirmed against the live repo:** `docket repository check --json` (the binary built from `main`) already emits `metadata-root-foreign` (severity error) on this very repository. So `prepare` as built refuses every real docket repo, and the plan's clean-behind fast-forward and diverged-refusal deliverables (Task 3) are unreachable end-to-end. `RootCommits`' own doc states the correct orphan proof is "len == 1 **and that root's tree/receipt**" — not `root == tip` — so the shared predicate is a latent bug on `main` that 0377 is the first change to depend on.

### Why this needs a human (not an autonomous fix)

1. **Security-relevant.** The predicate is the foreign-branch adoption boundary — which remote metadata branches `prepare`/`init`/`migrate` will attach, fast-forward, or adopt. A bare `len(roots)==1` relaxation weakens foreign detection; the safe fix must additionally verify the single root carries the docket seed receipt (`OpInitRoot` / `publishedSeedReceipt`).
2. **Cross-service scope the plan did not authorize.** To stay coherent the fix must change the shared predicate in `repository_check.go`, `repository_init.go`, `repository_migrate.go`, and the `reposetup` `RootParentless` contract — none of which are in this change's plan tasks (scoped to `repository_prepare.go` + CLI + integration tests). A prepare-only fix would make `prepare` accept a chain that `repository check` still calls foreign — a knowingly-introduced incoherence.
3. **Against a stated non-goal.** The spec fences "metadata topology ... redesign" as a non-goal. Whether correcting this shared classifier belongs inside 0377 or in a separate predecessor change is a scoping decision for the maintainer.

### Recommended resolution

Decide one of: (a) authorize expanding 0377 to fix the shared metadata-root predicate (chain model: `len(roots)==1` AND the single root carries the docket seed receipt) consistently across prepare/check/init/migrate/reposetup, with Task 3's integration tests as the proof; or (b) split that classifier fix into a new predecessor change (it is a pre-existing `main` bug independent of 0377) and rebuild 0377 on top of it. Then resume 0377 via `docket change resume-halted --id 377 --acknowledge-quiescent`.
