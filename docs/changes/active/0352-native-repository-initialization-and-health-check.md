---
id: 352
slug: native-repository-initialization-and-health-check
title: 'Native repository initialization, migration, and health checks'
status: 'implemented'
priority: high
type: feat
created: 2026-08-26
updated: '2026-08-28'
depends_on: [351]
stacked_on:
related: [351, 311, 318, 363]
discovered_from: [303, 351]
adrs: [1, 2, 20, 21, 25, 34, 36, 52, 78]
spec: docs/superpowers/specs/2026-08-28-native-repository-initialization-migration-and-health-design.md
plan: 'docs/superpowers/plans/2026-08-28-native-repository-initialization-and-health-check.md'
results:
trivial: false
auto_groomable:
branch: 'feat/native-repository-initialization-and-health-check'
pr: 'https://github.com/danielhanold/docket/pull/249'
blocked_by:
reconciled: true
claimed_at: '2026-08-28T16:03:49Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-native-repository-initialization-migration-and-health-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-native-repository-initialization-migration-and-health-design.md) |
| Plan | [2026-08-28-native-repository-initialization-and-health-check.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-28-native-repository-initialization-and-health-check.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0002](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0002-docket-mode-default-and-bootstrap.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0021](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0021-pipeline-script-authored-mechanical-commits.md), [ADR-0025](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0025-docket-worktrees-disable-git-hooks.md), [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md), [ADR-0078](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

The approved Go migration architecture names `repository init`, `repository migrate`, and
`repository check` as a required operation family, but no migration child implemented it. The Go
machine installer deliberately excluded complete repository setup, leaving the hard-cutover
product without a native way to establish, migrate, or verify a Docket repository.

Change 0351 adds reusable, ownership-safe reconciliation for parent-facing repository surfaces, but
does not initialize metadata routing, migrate a legacy live planning surface, attach the persistent
metadata worktree, or report repository health. The missing repository operation needs its own
design and lifecycle before change 0318 can remove the Bash surface.

## What changes

- Add native `docket repository init`, `docket repository migrate`, and `docket repository check`
  over one authoritative state classifier and one docket-mode destination.
- Make initialization and migration idempotent, ownership-safe, recoverable from durable remote
  postconditions, and explicit about every working-tree, commit, ref, and worktree effect.
- Validate every active and archived change before migration and optionally apply only a closed,
  previewed, explicitly authorized set of mechanically safe frontmatter repairs.
- Reuse change 0351's repository-surface planner rather than creating a second parent-instruction
  writer; keep `check` content-read-only with precise human and JSON remedies.
- Put every long-running real-Git, remote, interruption, and concurrency test in the established
  integration partition, split so no individual test or shard reaches 60 seconds.

## Out of scope

- Machine installation, uninstall, or version-tree collection.
- Reimplementing change 0351's global cleanup, fresh-binary handoff, or repository-surface renderer.
- Silently migrating an existing repository as a side effect of machine installation.
- Restoring or modifying legacy repository-migration scripts.
- Creating a Git repository or remote, supporting local-only setup, or retaining `main` mode as a
  healthy repository topology.
- General-purpose frontmatter normalization or repairs that require identity, lifecycle, path,
  relationship, or other semantic judgment.

## Design decisions

- `init` creates and pushes an empty orphan metadata root, attaches `.docket/`, and leaves managed
  integration-file edits unstaged for review; it never creates or rewrites `.docket.yml`.
- `migrate` is an explicit confirmed command that copies the complete metadata corpus before
  pruning only the live integration surface, removes the obsolete `metadata_branch` key, publishes
  deterministic receipt-bearing commits, and resumes provable native or legacy partial states.
- `check` may fetch authoritative remote state but never changes content, branches, worktree
  registration, ownership records, or remote refs; `0` means healthy, `1` means diagnosed action is
  required, and `2` means invalid usage or unknown authority.
- A single-branch repository is migration input only. Change 0363 removes the remaining unused
  main-mode implementation before change 0318's release-candidate cutover.
- The linked spec is the complete approved scope, including the closed frontmatter-repair roster,
  safety/refusal model, JSON receipts, and hard integration-test/runtime boundary.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-28

Reconciled against current `main`. Dependency 0351 is archived (done): its repository-surface planner and per-worktree ownership record land in `internal/reposeed` (`plan.go`, `record.go`), so the reuse premise holds. No native `docket repository init/migrate/check` command group exists yet (`internal/cli` has no `repository.go`; no repository setup service in `internal/app`), so the change remains fully unbuilt and non-duplicated. Referenced seams are present: config resolver (`internal/config`), source-preserving document layer (`internal/document`), repository validator and transaction concepts (`internal/repository`, `internal/repository/transaction`), `internal/gitcli` primitives, protocol-v1 envelope, and the `internal/reposeed` planner. Downstream changes remain pending as the spec assumes: 0318 (config contraction / hard cutover) and 0363 (remove main-mode compatibility) are still active/unbuilt; 0311 (installer) is archived. The integration-test partition infrastructure the spec mandates is in place: `tests/lib/go-integration-shard.sh`, the `tests/test_go_integration_*.sh` runners, and `tests/runtime-budgets.tsv`. Scope, relations, ADR citations, and the linked spec are all still accurate; no section edits or relation changes required.
