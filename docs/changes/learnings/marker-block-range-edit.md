---
slug: marker-block-range-edit
hook: "Before rewriting a marker-delimited managed block, validate marker order and balance — presence alone lets the range consume to EOF."
topics: [shell, markers, dataloss]
changes: [51, 57]
created: 2026-07-10
updated: 2026-07-16
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Before stripping/rewriting a marker-delimited block, validate marker *order and balance* — refuse-and-warn
on dangling / out-of-order / nested / unbalanced markers (either spelling) and leave the file untouched;
never presence alone, never let the range consume to EOF.

## War story
- 2026-07-10/11 (#51 PR #60; #57 PR #63 — merged, re-hit class) — An awk/sed **range** edit
  (`/start/,/end/`) over a marker-bounded "do-not-hand-edit" managed block is a data-loss hazard
  whenever the end marker is lost (truncation / bad merge) or the markers are out of order
  (END-before-START, same spelling): the range runs to EOF and silently deletes the user's own
  content after the dangling start (`.gitignore` bytes here). A guard checking marker *presence*
  alone is bypassed by the corrupted block.
