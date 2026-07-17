---
id: 90
slug: discovered-from-provenance
title: discovered-from provenance links — record which change's build surfaced a new stub
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [35, 91]
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

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads has a
`discovered-from` dependency type — "Found during work on another issue" — which its FAQ calls
"agent-specific": it exists because agents constantly surface follow-up work mid-task, and the
origin of that work is valuable graph data (what fraction of the backlog is planned vs
discovered? which changes spawn the most follow-ups?).

docket already generates discovered work — implement-next's reconcile and review passes, groom
brainstorms, and close-out findings all propose follow-up changes (this repo's own board shows
chains like 0075→0076, 0086/0087 from 0062) — but the provenance lives only in prose. `related:`
is symmetric and unordered; nothing says "this stub exists *because* building #NN surfaced it."
One frontmatter field fixes that.

## What changes

- A new optional manifest field (e.g. `discovered_from: [62]`) recording the change id(s) whose
  work surfaced this one; empty/absent for deliberately planned work.
- The convention (docket-convention manifest section + change template) documents it; skills that
  mint follow-up stubs mid-run (implement-next, finalize's harvest, auto-groom, new-change when
  the human names an origin) populate it.
- Render surfaces pick it up where cheap: the `## Artifacts` block or board could show
  "discovered from #NN" (brainstorm decides how far rendering goes; the field is the point).

## Out of scope

- Auto-creating the stubs themselves — that is #0091 (possibly merged with this change at groom
  time).
- New blocking semantics: `discovered_from` is informational, like `related:`, never a
  readiness gate.
- Analytics over the provenance graph (#0010's territory).

## Open questions

- Field shape: list of ids (parallel to `related:`) vs single id; does it imply an automatic
  `related:` back-link on the origin change?
- Merge with #0091 into one change, or land the field first as a trivial-adjacent step?

## Reconcile log
