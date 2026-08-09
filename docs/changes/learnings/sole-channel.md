---
slug: sole-channel
hook: "When a channel becomes the SOLE source of some state, re-prove on the survivor every property the fallback used to give you free."
topics: [design, contracts, retries]
changes: [69, 71, 271]
created: 2026-07-13
updated: 2026-08-09
promotion_state: retained
promoted_to:
---

## Apply
When a change removes the fallback channel for some state, re-audit the survivor's ORDERING
against every pass that MUTATES that state (a snapshot taken before a mutating pass is only tolerable
while something downstream can correct it), and prove the channel is TOTAL — every path, including the
warn-and-ignore and failure paths, emits exactly one line — because "no line" is otherwise
indistinguishable from success and you have merely moved the silence somewhere quieter. Enumerate a
retry contract by its RETRYABLE set, never its terminal set: the terminal set is open-ended, and the
legitimate line you forgot becomes an infinite loop.

## War story
- 2026-07-13/14 (#69 PR #77; #71 PR #81 — merged, one sole-channel family) — When a channel becomes the
  SOLE source of some state, every property you used to get for free from the fallback has to be
  re-proven on the survivor. Both changes shipped a hole here, in both directions.
  (a) **Ordering** — #69's digest was spec'd to emit BEFORE the merge sweep, fine while `BOARD.md` was
  a second channel, but the same change forbade the skill from ever opening the board: a full pass then
  printed `change 60 implemented` and `swept 60` in one report with no corrective path left.
  (b) **Totality** — #71 collapsed six duplicated Board-pass call sites onto a stdout report line
  ("key on the LINE, never the exit code"), and the whole-branch review found two exit-0 paths through
  `docket-status.sh` that emit **no `board …` line at all** (an unknown/typo'd surface token; an inline
  render failure). A must-land caller seeing no line concludes "terminal → it landed" and proceeds on a
  silently stale board — the very defect class the change exists to kill, relocated from the script
  boundary into the caller contract and made QUIETER than before.
  (c) **Terminality** — #71's first retry contract listed the three lines meaning "done" and retried on
  everything else, so a legitimate `board_surfaces: [github]` repo (which prints only `board github ok`)
  would have re-invoked forever.
- 2026-08-09 (#271, PR #188 — merged) — **A redirect made a file the sole carrier, and nothing
  carried it back.** Detached delegation rewrote the adapter launch to `--launch` the child with its
  stdout and stderr redirected into the dispatch dir, and made every `--observe` diagnostic go to
  stderr. Both halves are individually right; together they left **no path that writes the child's
  captured stdout to the caller's stdout**. The child's report file was now the sole channel, and the
  survivor property nobody re-proved was *delivery*. Every in-context-report agent —
  `docket-build-*` returning COMPLETE/NEEDS_ESCALATION/BLOCKED, `docket-review-*` returning findings —
  would have returned **nothing** to its caller, while the git-state agents (status, adr) kept
  working, because their contract never used the channel that was removed. That asymmetry is the
  tell: when a change removes a channel, the callers that break are exactly the ones that were using
  it, so a smoke test over the git-state agents proves nothing. The relay is now sentinel-gated —
  it fires only where the child is finished and its output complete, never on the still-running poll
  path and never on the own-group refusal where the child is still writing.
