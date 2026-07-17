---
id: 93
slug: archive-decay-digest
title: Archive decay — a rolling one-line digest so board and context cost stay flat as the archive grows
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [10, 67]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads treats
context-window economics as a first-class feature: semantic "memory decay" summarizes old closed
tasks (`bd admin compact` — "compact old closed issues to save space", summarize instead of
delete) so an agent's view of history stays cheap no matter how much history accumulates.

docket's archive only grows. This repo is at ~76 archived changes and `BOARD.md` re-renders every
one of them (id, title, merge date, link) in its Archive section on every board pass; every agent
that loads the board pays for the full list, and the mermaid graph enumerates every done id. The
change files themselves are fine — they're read on demand — but the *always-loaded surfaces*
(board, and in the same spirit the learnings ledger as it approaches its `cap`) need a decay
story: recent history verbatim, older history as a rolling one-line digest.

## What changes

- The board's archive rendering gets a decay policy (settled in brainstorm), e.g.: last N
  merged/killed changes listed as today, older entries collapsed to a count + a digest line per
  period ("2026-06: 31 changes done, 2 killed") with the full detail one click away (the archive
  directory itself, or a generated `ARCHIVE.md` index).
- The mermaid graph stops enumerating every done id (it already carries no edges for most).
- Same spirit applied to the learnings ledger's index as it approaches `learnings.cap` —
  whether that's part of this change or a recommendation for the existing curation flow is
  decided at groom time (#0067 built the promotion destination; this is the compaction side).
- Everything stays derived: renderers change, source files are never summarized-in-place or
  deleted (unlike beads, docket's archive files are immutable records).

## Out of scope

- Deleting or rewriting archived change files, specs, or ADRs — decay applies to *rendered
  views* only. ADRs are explicitly never archived or decayed.
- Throughput/cycle-time analytics over the archive (#0010 owns that; the digest line may share
  its date-bucketing).

## Open questions

- Recency window (count-based, e.g. last 15, vs time-based, e.g. 30 days)?
- Digest granularity (per month? per quarter?) and whether killed changes stay individually
  listed (they carry more signal than routine dones).
- Does the GitHub board surface need the same decay, or is it exempt (Issues scale fine)?

## Reconcile log
