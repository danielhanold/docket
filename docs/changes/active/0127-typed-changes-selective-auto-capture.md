---
id: 127
slug: typed-changes-selective-auto-capture
title: Typed changes — configurable taxonomy, selective auto-capture, and backlog filters
status: proposed
priority: medium
type: feat
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: [90, 91, 94, 124]
discovered_from: []
adrs: [12, 19, 45, 52]
spec: docs/superpowers/specs/2026-07-22-typed-changes-selective-auto-capture-design.md
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
| Spec | [2026-07-22-typed-changes-selective-auto-capture-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-22-typed-changes-selective-auto-capture-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0045](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0045-auto-capture-is-best-effort.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Auto-capture has preserved follow-up work, but it has also filled the needs-brainstorm queue with
narrow fixes, guards, documentation work, and chores that compete with product features. At design
time, 16 of 23 proposed changes carried discovery provenance. Docket cannot apply a better policy
because every backlog record is currently an undifferentiated “change.”

An explicit type gives the backlog a useful vocabulary and lets repositories choose which kinds of
discovered work are durable enough to auto-capture. The same field also makes backlog review
practical without changing Docket's lifecycle or replacing its portable Markdown board.

## What changes

- Add a configurable `type:` field with default types `chore`, `docs`, `feat`, `fix`, `refactor`,
  and `perf`; every new-change and mint path classifies new work.
- Replace scalar `auto_capture` with an intentionally breaking nested configuration containing
  `enabled` and an optional whole-list-replacing `types` allowlist. All new settings resolve through
  built-in, user/global, repo-committed, and repo-local layers.
- Gate best-effort auto-capture by the discovered work's type and report excluded candidates.
- Add Type to active board tables and add report-only `docket-status --type` and `--priority`
  filters. Filters never narrow lifecycle work or canonical board generation.
- Provide a human-approved, deterministic one-time categorization pass for active changes. Migrate
  this repository's active backlog; never edit archived changes.

## Out of scope

- Interactive filtering inside Markdown, GitHub Projects, plugins, or GitHub mirror changes.
- Reclassifying archived records.
- Changing lifecycle states, readiness, selection order, or auto-capture's materiality bar and
  best-effort failure posture.

## Open questions

None. The configuration shape, layer semantics, migration posture, and filter boundaries are
settled in the linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
