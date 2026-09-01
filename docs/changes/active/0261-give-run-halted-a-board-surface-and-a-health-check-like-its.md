---
id: 261
slug: give-run-halted-a-board-surface-and-a-health-check-like-its
title: 'Give ''## Run halted'' a board surface and a health check, like its two family members'
status: proposed
priority: high
type: feat
created: 2026-08-07
updated: 2026-08-16
depends_on: []
related: [222]
discovered_from: [237]
adrs: []
spec: docs/superpowers/specs/2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced by the whole-branch review of change 0237, which introduced the
`## Run halted` presence-encoded body section and its `verify-run` reader.

**Opportunity** — `## Run halted` is the only member of its family with no derived-view surface.
Both named siblings have one: `## Finalize blocked` renders `finalize blocked — needs you` and is
guarded by the `stale-finalize-blocked` health check; `## Auto-groom blocked` renders
`auto-groom blocked — needs you`. `## Run halted` renders nothing and no check reads it, so a run
that deliberately stopped needing a human is visible only to whoever reads the dispatch seam's
stderr, or later and indirectly through `aborted-run`'s floor-gated backstop.

**Independent value** — with 0237 reverted the section would not exist, so this is strictly
downstream of it; but with 0237 merged the surface is valuable on its own and on any schedule. A
human scanning the board is the intended consumer of every other needs-you cell, and the halt is
the one disposition the contract defines as *stop + surface*.

**Boundary** — a board cell for an `in-progress` change carrying `## Run halted`, plus a
`board-checks.sh` check-id for it (with the `BOARD_CHECK_IDS` array, `board-checks.md`,
`docket-status.md`, and the `--help` enumeration all updated together, as that array's contract
requires). It deliberately leaves alone: `verify-run`'s verdict vocabulary, the dispatch seam's
exit contract, and every existing `aborted-run` leg and floor.

**Reason for deferral** — change 0237's spec explicitly rules it out: *"Board rendering of the
section is out of scope — `aborted-run` already surfaces the change, and a new board cell is scope
this change does not need."* Building it on 0237's branch would reverse a settled scope decision
and pull `board-checks.sh` — which that spec deliberately leaves untouched — into the diff.

## What changes

Settled design (2026-08-09, auto-groom; detail in the linked spec):

- **Board cell** — the In progress table gains a trailing `Readiness` column rendering
  `run halted — needs you` when the change file carries the bare `## Run halted` section,
  mirroring the implemented table's `finalize blocked — needs you` cell, plus the matching
  `digest_readiness()` in-progress arm (`run-halted` token, else `-`).
- **Shared predicate** — a `run_halted()` helper in `lib/docket-frontmatter.sh` beside
  `finalize_blocked()`, called by both render-board.sh and board-checks.sh.
- **Health check** — a new `stale-run-halted` check-id in `board-checks.sh`, gated on
  `status: in-progress`, firing when the marker's file-tip age (git `%ct`) outlives a
  `RUN_HALTED_STALE_SECS` horizon (same default as `FINALIZE_BLOCKED_STALE_SECS`); advisory
  only, never mutates the file. Adding the id edits all four pinned surfaces of the
  `BOARD_CHECK_IDS` contract together.

## Out of scope

`verify-run`'s verdict vocabulary, the dispatch seam's exit contract, every existing
`aborted-run` leg and floor (including its known 12h double-fire on a halted change — left
deliberately), and the marker's write/remove lifecycle (0237 owns it).
