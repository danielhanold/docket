---
slug: moving-base
hook: "A change is designed against a SNAPSHOT and the base moves under it — reconcile against what has actually MERGED."
topics: [process, reconcile, rebase]
changes: [37, 59, 60, 74]
created: 2026-06-21
updated: 2026-07-14
promotion_state: retained
promoted_to:
---

## Apply
Reconcile against what has actually MERGED, never against what a proposed sibling will do,
and fold a change whose scope a sibling already shipped down to its residual (kill only if genuinely
covered). Anchor a spec's edit sites to STRUCTURE the reader can re-find (the clause, the shape) —
never to line numbers, which a sibling merge invalidates without touching your change. Rebase the
most conflict-prone change (the one touching many shared files) LAST, once, onto the settled base,
and resolve every hunk by INTENT: a same-file change that merged AFTER you diverged SUPERSEDES your
edit (drop yours) rather than being a conflict to win.

## War story
- 2026-06-21 / 2026-07-11 / 2026-07-14 (#37 PR #48; #59 PR #64; #60 PR #70; #74 PR #82 — merged, one
  moving-base family) — A change is designed against a SNAPSHOT and the base moves under it.
  (a) **Design.** 0059 was designed around what a still-`proposed` sibling (0058) would "later"
  compose; 0058 merged first and built the same gate independently, inverting 0059's scope twice.
  Conversely 0060's spec was ~90% delivered by that same sibling, and reconcile correctly folded 0060
  to its one residual sub-case rather than killing it or rebuilding the overlap.
  (b) **Coordinates.** #74's spec pinned its two edit sites by LINE NUMBER (`~78`/`~110`) in a file
  sibling #71 reshaped before the build even began — stale on arrival; reconcile re-anchored them to
  shape-descriptions and the edits then coexisted cleanly.
  (c) **Conflict.** #37 was mid-build when a PR merged newer fixes into the very file it was
  stripping; the reflexive "keep my side" would have **silently reverted** them — the branch's version
  simply predated them.
