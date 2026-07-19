---
id: 104
slug: guard-tab-in-frontmatter-values-feeding-the-digest-s-tab-joi
title: Guard TAB in frontmatter values feeding the digest's TAB-joined sort rows
status: proposed
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: []
discovered_from: [94]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

The `ready` line emitted by `render-board.sh --format digest` is now **machine-parsed** — change
0094 made it the selection channel for `docket-implement-next`, with a documented
`^ready( [0-9]+)*$` grammar and an exit-status contract layered on top of it.

The sort rows behind that line are **TAB-joined**. A tab character inside `created:` therefore
shifts the field split and can emit a non-numeric token into the `ready` line, violating the
grammar the consumer now relies on. The `change`-line loop has the same shape via `slug`.

This is **pre-existing exposure**, deliberately left alone in 0094 — but 0094 raised the stakes by
making the line machine-parsed rather than human-read report output.

## What changes

Sanitize (or reject) TAB in the frontmatter values that feed TAB-joined sort rows — at minimum
`created:` and `slug:` — so a malformed value cannot produce an output line that violates the
documented grammar. Prefer a shape-keyed guard over an enumerated field list.

## Out of scope

Broader frontmatter validation unrelated to the digest's output grammar.

## Open questions

- Sanitize at read time (frontmatter helper) or at render time (the row builder)?
- Reject loudly vs. silently strip — a rejected change would need somewhere to surface.
