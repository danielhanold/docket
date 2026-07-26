---
id: 10
slug: board-analytics
title: Board analytics — throughput and cycle-time stats derived from git history, rendered on BOARD.md
status: deferred
priority: low
created: 2026-06-11
updated: 2026-07-26
depends_on: []
related: [4, 93]
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

## Why deferred

Deferred 2026-07-26 by the change 0124 backlog triage pass. This is the weakest of the four
2026-06-11 competitive-review stubs and came closest to being killed outright.

Partly delivered: change 0093 added a per-month `| Month | Done |` archive digest to
`scripts/render-board.sh`, which gives throughput. `BOARD.md` already carries a status count line.
Cycle time, kill rate, and backlog age remain unbuilt — no `cycle-time`/`throughput`/`STATS.md` hits
anywhere in `scripts/` or `skills/`.

Deferred because the remaining metrics now sit in **direct tension with a decision the repo has
since made**. Change 0093 deliberately *shrank* the board surface for context economy — the board
is read by agents on every run, and every row costs context. This stub proposes growing it with
derived statistics that nothing consumes and no decision depends on. Adding a stats block would
partially reverse 0093's intent.

Not killed only because the "separate generated `STATS.md`" option in its own open questions
sidesteps that tension cleanly — the data really is free in git. Revive only with that shape, and
only if a real question ("is the backlog keeping up?") ever needs answering by something other
than reading the board. Absent that, this should be killed on its next review rather than deferred
again.
