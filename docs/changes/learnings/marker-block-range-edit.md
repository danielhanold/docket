---
slug: marker-block-range-edit
hook: "Before rewriting a marker-delimited managed block, validate marker order and balance — presence alone lets the range consume to EOF."
topics: [shell, markers, dataloss]
changes: [51, 57, 306]
created: 2026-07-10
updated: 2026-08-13
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Before stripping/rewriting a marker-delimited block, validate marker *order and balance* — refuse-and-warn
on dangling / out-of-order / nested / unbalanced markers (either spelling) and leave the file untouched;
never presence alone, never let the range consume to EOF.

Second arm (added #306, **not yet reflected in the AGENTS.md paragraph** — a human amending it should
fold this in): *identify* the markers by a whole-line, column-zero match, never a substring match on
any line. Authored artifact text legitimately contains marker-shaped literals — a plan or spec that
documents the block grammar quotes it verbatim — so a substring probe finds the example instead of
the block, and the "balanced markers" check then passes against the wrong pair. Balance validation is
only as good as the anchoring underneath it.

## War story
- 2026-08-13 (#306, PR #206) — `render-artifact-backlink.sh` matched its start marker with
  `grep -qF` plus an awk `index()` substring test, so a marker-shaped literal inside the change's own
  authored plan text was taken for the real backlink block; everything from plan Task 8 to EOF was
  consumed at stamp time. Live data loss in shipped tooling, against the branch whose entire purpose
  was preventing exactly this class. The plan was restored from the authoring context and the
  triggering literal defanged; the bash-side fix is stub 0321, and the Go document layer built in
  #306 refuses the class by construction (whole-line column-zero grammar, population validation,
  fence awareness — each mutation-tested).
- 2026-07-10/11 (#51 PR #60; #57 PR #63 — merged, re-hit class) — An awk/sed **range** edit
  (`/start/,/end/`) over a marker-bounded "do-not-hand-edit" managed block is a data-loss hazard
  whenever the end marker is lost (truncation / bad merge) or the markers are out of order
  (END-before-START, same spelling): the range runs to EOF and silently deletes the user's own
  content after the dangling start (`.gitignore` bytes here). A guard checking marker *presence*
  alone is bypassed by the corrupted block.
