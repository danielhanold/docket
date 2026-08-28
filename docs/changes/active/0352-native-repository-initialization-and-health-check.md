---
id: 352
slug: native-repository-initialization-and-health-check
title: 'Native repository initialization, migration, and health checks'
status: 'in-progress'
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
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/native-repository-initialization-and-health-check'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-28T15:48:33Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-native-repository-initialization-migration-and-health-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-native-repository-initialization-migration-and-health-design.md) |
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
