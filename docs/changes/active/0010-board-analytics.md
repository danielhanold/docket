---
id: 10
slug: board-analytics
title: Board analytics — throughput and cycle-time stats derived from git history, rendered on BOARD.md
status: proposed
priority: low
created: 2026-06-11
updated: 2026-06-11
depends_on: []
related: [4]
adrs: []
spec:
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
type: feat
---

## Why

Synthesized from the AgentRQ competitive review (2026-06-11). AgentRQ ships per-workspace
analytics — tasks completed, daily activity timeseries, manual-vs-auto approval counts — powered
by an append-only telemetry table in its database. The insight worth keeping is that a backlog
system should answer "how is this flowing?" at a glance; the implementation (a DB) is excluded.

docket already *has* the telemetry, for free: every lifecycle event is a dated commit on the
metadata branch, every change file carries `created:`/`updated:`, and every archive filename
carries its UTC merge/kill date. Nobody aggregates it. The board says what *is*; nothing says how
the backlog is *moving* — throughput, cycle time, kill rate, or how long the oldest proposed
change has been sitting.

## What changes

- A compact stats block on `BOARD.md` (rendered by the Board pass in `docket-status`), derived
  purely from data already in git — candidate metrics, to be settled in the brainstorm:
  - Throughput: changes done per recent window (from archive date prefixes).
  - Cycle time: `created:` → archive date, median/range over recent done changes.
  - Kill rate: killed vs. done among archived changes.
  - Backlog age: oldest `proposed` change and the active-state count breakdown.
- Convention note that the stats block is derived output (same never-hand-edit rule as the board).

## Out of scope

- Databases, telemetry stores, or any state beyond what git and the change files already record.
- Charts/sparklines, per-day timeseries, or anything needing more than a markdown table.
- New frontmatter fields purely for measurement — if a metric needs new bookkeeping, drop it.

## Open questions

- Stats on `BOARD.md` itself vs. a separate generated `STATS.md` linked from the board?
- Time-window choice (last 30 days? last N changes?) given a small-N backlog where medians jump.
- Is git-log mining (e.g. claim→PR duration from commit dates) worth it, or frontmatter-only?

## Reconcile log
