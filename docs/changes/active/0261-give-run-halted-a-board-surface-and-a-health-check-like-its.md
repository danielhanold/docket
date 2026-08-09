---
id: 261
slug: give-run-halted-a-board-surface-and-a-health-check-like-its
title: 'Give ''## Run halted'' a board surface and a health check, like its two family members'
status: proposed
priority: medium
type: feat
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [237]
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
