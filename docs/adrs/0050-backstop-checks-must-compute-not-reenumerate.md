---
id: 50
slug: backstop-checks-must-compute-not-reenumerate
title: A backstop check must compute the invariant it guards, never re-enumerate the causes it backs up
status: Accepted
date: 2026-07-20
supersedes: []
reverses: []
relates_to: [49]
change: 104
---

## Context

Change 0104 added two layers to `scripts/board-checks.sh`. `field-domain` *enumerates* the
specific frontmatter-value violations that make `render-board.sh` silently drop a change's row (a
`status` outside the vocabulary, a malformed `slug`, and so on). Beneath it, `board-row-dropped`
was specified as the *invariant* backstop: an `active/` file counted in the board's total but
rendered in no section, suppressed whenever an enumerated finding already explains that id — so
that when it does fire it means exactly one thing, a row vanished and nothing enumerated explains
why, and it survives a future renderer-added drop path.

The first implementation satisfied every test and was still wrong in kind. It populated its
`DROPPED` set from two hand-written conditions — no-usable-id and status-not-in-vocabulary — which
are precisely the two conditions `malformed-id` and `field-domain` already enumerate. Nothing
compared a count to a row set. Consequences, all found by whole-branch review rather than by the
suite:

- The stated purpose was unreachable by construction: a new renderer drop path would have to be
  hand-added to the checker to be noticed, which is exactly what a backstop exists to make
  unnecessary.
- A live fourth drop path went undetected. An `active/` file carrying a terminal status (`done` /
  `killed`) is counted in `render-board.sh`'s `total` but rendered in no section —
  `print_section` is called only for the five active statuses, and the count line's
  `done|killed` arm reads the archive-only `ARC_COUNT`. `field-domain` is silent (`done` IS in
  the vocabulary), `malformed-id` is silent (the id is valid), and nothing set `DROPPED`. The
  board rendered `**2 changes**` above a single row while every check passed — the exact symptom
  change 0104 exists to eliminate, on a state the toolchain documents as reachable
  (`sweep-failed <id> archive <reason>`: status flipped to `done`, archive move failed).
- One half of the population was dead code masquerading as coverage: the status path set
  `EXPLAINED` unconditionally in the same block, so every entry it created was guaranteed
  suppressed. The "yields exactly ONE finding" suppression test was asserting a self-cancelling
  pair, not a suppression decision — and deleting that code left the suite green.

## Decision

The backstop derives a `renders_row` predicate that mirrors the renderer's own bucketing —
`int_field id` non-empty AND status ∈ `DOCKET_STATUSES_ACTIVE`, the same array
`render-board.sh`'s section iteration uses — and reports any counted active file the predicate
says is not rendered. The two hand-written population sites collapsed into one computed site.
Suppression is unchanged in intent but now gates a real decision, because `DROPPED` and
`EXPLAINED` are written at independent sites.

The general rule: **when a check is justified as a backstop for an enumeration, its trigger
condition must be derived from the consuming code's actual behavior, not restated from the same
enumeration.** A backstop whose population set is a copy of the enumeration it backs is a fourth
restatement wearing the word "invariant," and it inherits every blind spot of the thing it was
supposed to catch. A corollary for review: mutation-test a backstop by deleting its population,
not only its suppression — a suppression assert passes vacuously when the invariant never
computes.

## Consequences

The predicate is a deliberate, documented mirror of `render-board.sh`'s section-rendering
behavior; that correspondence is currently asserted by comment and fixtures rather than by a
mechanical guard, because the renderer's `print_section` call list is not yet single-sourced
(tracked as follow-up change 116). The check is bounded to `active/`; the symmetric archive-side
violation — an `archive/` file with a non-terminal status, counted in `total` and rendered
nowhere — is real, currently undetected, and tracked as follow-up change 115. `EXPLAINED` is now
marked only by arms that genuinely explain a dropped row (`malformed-id`, and `field-domain` on
`status`); marking it from the `slug` / `priority` / `title` arms would have caused false
suppression, since none of those violations drops a row.
