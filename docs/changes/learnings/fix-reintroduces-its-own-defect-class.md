---
slug: fix-reintroduces-its-own-defect-class
hook: "New code added by a change that fixes a defect class is the likeliest place for that class to reappear — audit the change's OWN additions against its thesis before review, and check the twin it did not touch."
topics: [review, refactoring, contracts]
changes: [135, 173]
created: 2026-07-28
updated: 2026-07-31
promotion_state: candidate
promoted_to:
---

## Apply
A change whose whole purpose is "stop emitting X in shape A when the consumer expects shape B"
usually also *adds* a new producer or a new adapter. That new code was written by the same hands,
against the same mental model, in the same session — and it is routinely the one place the fix does
not reach, because the audit is aimed at the code being repaired, not at the code being introduced.

Two moves, both cheap:

- **Run the change's own thesis over its diff.** Take the one-sentence defect statement and apply it
  literally to every file the branch adds. If the thesis is "a sentinel value must be normalized
  before it reaches the harness," grep the additions for that sentinel.
- **Find the twin.** A copy-paste sibling (the adapter for the *other* harness, the second emitter,
  the upstream helper both call) almost always carries the same gap. Fix it, or mint it explicitly
  as a follow-up with the root cause named — never leave it as an unrecorded observation.

Watch especially for a **defensive warning that keys on the wrong condition**: a guard testing
`-z "$VAR"` never fires for a sentinel like `inherit`, which is non-empty and equally invalid, so
the code both mis-handles the value and suppresses its own diagnostic.

Related: [[escape-ere-metacharacters-in-key]] (the un-fixed twin of a duplicated helper),
[[correspondence-guard-runs-one-way]], [[verify-the-claim]].

## War story
- 2026-07-28 (#135, PR #127) — The change existed because docket emitted Claude-shaped wrappers for
  Cursor, silently pinning one thing while the harness honored another. The branch's **new**
  `scripts/runners/cursor.sh` adapter did not normalize docket's `inherit` model sentinel, though
  `emit_cursor_md` in the same branch does. `emit_shim` bakes `--model $2` whenever the resolved
  override is non-empty, so `runner: cursor` plus an explicit `model: inherit` would have sent
  `--model inherit[effort=xhigh]` to `cursor-agent` — a non-existent model ID handed to a CLI with a
  documented compatible-model fallback, the effort pin destroyed with it. The adapter's own WARN was
  unreachable because it keyed on `-z "$MODEL"`. Caught at final whole-branch review, not by the
  suite; fixed by normalizing the sentinel before the flag mapping so it routes into the existing
  correct WARN. The Codex adapter's identical twin, rooted upstream in `emit_shim`, was still live
  and became **#0140**.
- 2026-07-31 (#173, PR #142 — merged) — The change existed to widen three over-narrow value classes
  that silently truncated config values. Its **own new** block-mapping reader in
  `scripts/runner-dispatch.sh` then over-captured in the opposite direction: for a comment-only line
  (`sandbox:   # TODO decide later`) it exported the comment text as the value, because the
  capture's trailing `[[:space:]]*` is greedy and eats the space the strip-comment step keys on, so
  that strip could never fire. `scripts/runners/codex.sh` would have run
  `codex exec --sandbox '# TODO decide later'`, or `die`d outright on a commented-out `network:` —
  turning a cosmetic comment into a failed dispatch, the exact harm the reader's deliberately
  tolerant posture exists to prevent. Caught at whole-branch review, not by the suite. Alongside it,
  the change's **new gate** was self-inconsistent: it exempted one already-warned-and-dropped config
  shape while hard-failing two others, so a quoted value in dead config blocked *all* wrapper
  generation. Both fixed in `ff9f0962`. The generalizable addition to this finding: when the defect
  class is "the pattern is the wrong width," the replacement pattern is wrong in the *other*
  direction just as easily — widening is not a safe direction, it is a second chance to be wrong.
