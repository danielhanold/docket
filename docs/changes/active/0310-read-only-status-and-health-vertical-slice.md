---
id: 310
slug: read-only-status-and-health-vertical-slice
title: 'Read-only status and health vertical slice'
status: proposed
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-15
depends_on: [307, 308]
stacked_on:
related: [261]
discovered_from: [303]
adrs: [1, 28, 34, 47, 92, 93]
spec: docs/superpowers/specs/2026-08-15-read-only-status-and-health-vertical-slice-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-15-read-only-status-and-health-vertical-slice-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-15-read-only-status-and-health-vertical-slice-design.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0028](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0028-report-channel-is-not-a-board-surface.md), [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0047](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0047-digest-only-read-tier-skips-preflight.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0093](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0093-repository-reference-severity-graded-by-structural-role.md) |
<!-- docket:artifacts:end -->

## Why

The first retained user workflow should prove that Go can open, interpret, and report an existing
repository without mutation before it is trusted to write.

## What changes

Add one `docket status` application and CLI slice that composes pinned authoritative Git objects
with the landed configuration, document, repository, and domain APIs. Report active status,
readiness, ordered selection, dependency and stack context, artifact integrity, and deterministic
health through human text and protocol-v1 JSON. Keep targeted Git fetches observational and all
metadata maintenance explicit and separate.

## Out of scope

Behavior owned by changes 0305 through 0309 or 0311 through 0318; board writes; lifecycle mutation;
maintenance sweeps; transaction worktrees; feature workspaces; pull requests; evidence capture;
supervision; release; and Bash cutover. Change 0261's unmerged board and health-check behavior also
remains separate.

## Design decisions

The focused design is approved in the linked spec. One `docket status` operation produces a shared
application result for deterministic human and JSON presenters. Repository defects remain health
data under an applied result; only failures that prevent trustworthy authoritative context fail the
operation. Filters affect the backlog projection but never reduce full-corpus health validation.

New frozen compatibility fixtures are historical snapshots derived from the refreshed `v0.9.3`
tag at peeled commit `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`. The added plan-writer feature is
fixture input only and does not expand this change into later workflow behavior.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
