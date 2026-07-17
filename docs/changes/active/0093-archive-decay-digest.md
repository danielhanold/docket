---
id: 93
slug: archive-decay-digest
title: Archive decay — a rolling one-line digest so board and context cost stay flat as the archive grows
status: implemented
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [10, 67]
adrs: []
spec: docs/superpowers/specs/2026-07-17-archive-decay-digest-design.md
plan: docs/superpowers/plans/2026-07-17-archive-decay-digest.md
results:
trivial: false
auto_groomable: true
branch: feat/archive-decay-digest
pr: https://github.com/danielhanold/docket/pull/96
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-17-archive-decay-digest-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-archive-decay-digest-design.md) |
| Plan | [2026-07-17-archive-decay-digest.md](https://github.com/danielhanold/docket/blob/feat/archive-decay-digest/docs/superpowers/plans/2026-07-17-archive-decay-digest.md) |
| PR | [#96](https://github.com/danielhanold/docket/pull/96) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads treats
context-window economics as a first-class feature: semantic "memory decay" summarizes old closed
tasks (`bd admin compact` — "compact old closed issues to save space", summarize instead of
delete) so an agent's view of history stays cheap no matter how much history accumulates.

docket's archive only grows. This repo is at ~75 archived changes and `BOARD.md` re-renders every
one of them (id, title, merge date, link) in its Archive section on every board pass; every agent
that loads the board pays for the full list, and the mermaid graph enumerates every done id. The
change files themselves are fine — they're read on demand — but the *always-loaded surface* (the
board) needs a decay story: recent history verbatim, older history as a rolling one-line digest.
(The learnings ledger index was considered under the same banner but is out of scope — it is a
relevance-indexed hint surface, not a chronological record, so recency-decay would harm it; see
Out of scope.)

## What changes

Board-only, `render-board.sh` and its contract + golden tests — no config, no caller change, no new
file. Full rationale (every default, the rejected alternatives) is in the linked spec's
`## Assumptions`.

- The archive `<details>` gains a **count-based recency window** over `done` entries plus a
  **per-month digest** of older `done` (`| Month | Done |`, each row linking to `archive/`). The
  window is a fixed constant (`ARCHIVE_RECENT`, default 15), always-on; inert (byte-identical
  archive table) below the threshold, so small repos see no change.
- **Killed changes are always listed verbatim** — never collapsed. They carry more abandonment
  signal than routine dones and are a rare, slow-growing minority, so the digest is done-only.
- The **mermaid graph stops enumerating every done id**: a `:::done` node renders only for a done
  id an active change actually `depends_on` (the floating, edgeless green nodes are dropped). This
  change is deliberate and universal — it alters every board on the next render, not just large ones.
- Everything stays derived: only the renderer changes; no archived change file, spec, or ADR is
  summarized-in-place, rewritten, or deleted (docket's archive files are immutable records).

## Out of scope

- Deleting, rewriting, or summarizing-in-place archived change files, specs, or ADRs — decay
  applies to *rendered views* only. ADRs are explicitly never archived or decayed.
- Throughput/cycle-time analytics over the archive (#0010 owns that; the per-month bucketing may
  be shared with it later).
- **Learnings-ledger index decay** — recommendation only, not built here: the learnings index is a
  relevance-indexed hint surface (topic/slug), not a chronological record, so recency-decay would
  actively harm it. Its compaction lever already exists: promotion + `learnings.cap`-gated
  consolidation (ADR-0041, #0067). Tune the cap / consolidate if it ever grows uncomfortably.
- The **GitHub board surface** — exempt: Issues are queried on demand, natively paginated, and the
  mirror already closes issues on done/killed (hidden by default).
- A **config knob** for the window and a generated **`ARCHIVE.md`** full index — deferred
  follow-ups (see spec `## Assumptions` #5, #6).

## Open questions

Resolved at groom (2026-07-17); the rationale + rejected alternatives are the spec's `## Assumptions`:

- Recency window — **count-based**, fixed at 15 (`ARCHIVE_RECENT`). Time-based was rejected: a
  wall-clock window would break the renderer's same-input-same-bytes determinism and its golden test.
- Digest granularity — **per-month**; **killed changes stay individually listed** (only `done`
  collapses).
- GitHub board surface — **exempt**, no decay.

## Reconcile log

2026-07-17 — Reconciled at claim, before planning. Verified the spec's code assumptions against
`origin/main` (tip `250ff7c`): `render-board.sh` (243 lines) still emits a `NNNN:::done` node for
**every** archived done id (the `DONE_IDS` mermaid loop) and lists **all** archive rows verbatim in
the `| # | Title | Merged |` table — no `ARCHIVE_RECENT` constant exists yet. Last substantive
render-board change was #0069 (digest projection, already in tree); nothing has touched the
archive/mermaid rendering since, so the spec's L-references (structure, not exact lines) hold. The
golden fixture (`tests/test_render_board.sh`) has 3 archive entries (done 0010/0012, killed 0011)
with active #0002 `depends_on: [10]` — confirming the spec's stated golden delta: mermaid pruning
drops `0012:::done` (unreferenced), keeps `0010:::done` (referenced), and the 3-entry archive table
stays byte-identical (well under the window). Related state: #0010 (board-analytics) still
`proposed`/unbuilt — no analytics sharing to fold in; #0067 (learnings promotion valve) now `done` —
the learnings-side compaction lever cited in Assumptions #8 exists as designed. No scope drift, no
obsolescence, design fully valid. Scope, defaults, and out-of-scope boundaries unchanged.
