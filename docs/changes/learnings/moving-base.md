---
slug: moving-base
hook: "A change is designed against a SNAPSHOT and the base moves under it — reconcile against what has actually MERGED."
topics: [process, reconcile, rebase]
changes: [37, 59, 60, 74, 111]
created: 2026-06-21
updated: 2026-07-21
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
- 2026-07-21 (#111, PR #117) — **The clean fold-to-residual, and a baseline that moved inside the
  measurement itself.** Two instances in one change, both benign because reconcile caught them.
  (a) **Scope.** The change was groomed when the check-id correspondence was guarded *zero* ways. By
  build time a sibling (#104) had shipped a block whose own comment names this change ("tracked
  structurally as change 0111") — already providing the emitted-set derivation, the header
  extraction, and their `comm -3` equality. Reconcile rewrote the change to *complete* that block
  rather than author a new one, because a second block beside it would have created **two
  derivations of one set** — precisely the duplication the change exists to close. The residual was
  real and shipped: #104 guarded both doc surfaces **subset-only**, so a retired-but-still-documented
  check-id passed green. This is the healthy version of (a) above, with a wrinkle worth naming — the
  residual was a *direction* of an existing guard, not a missing feature, which is easy to mistake
  for "already covered" and kill.
  (b) **Measurement.** The spec's baseline (11 check-ids / 16 call sites) was correct when taken and
  wrong by build time: #83's `publish-deferred` check merged after that measurement, making it
  12 / 17. Verified by checking out the pre-#83 tip and re-counting rather than trusting either the
  spec or the current tree. **A count written into a spec is a snapshot with no timestamp on it** —
  when a guard's assert is keyed to a baseline number, re-derive it at build time and establish
  where the difference came from. Here #83 had registered its id correctly on all three surfaces, so
  the guard inherited a clean baseline; knowing that is what separated "the spec's number is stale"
  from "there is real drift to repair," and those need opposite responses.
