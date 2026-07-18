---
slug: concurrent-edits-compose-at-rebase
hook: "When two open changes touch one function, keep each additive and funnel through a shared chokepoint; at rebase reconcile by INTENT — compose, don't choose."
topics: [git, rebase, concurrency]
changes: [79, 89]
created: 2026-07-16
updated: 2026-07-18
promotion_state: retained
promoted_to:
---

## Apply
When two open changes touch one function, keep each additive and funnel through a single
shared chokepoint; at rebase reconcile by INTENT (compose, don't choose), then run BOTH changes'
twin test suites green before trusting the merge — `bash -n` + no-conflict-markers is not enough.

When the conflicted hunk is a COUNT (an asserted line count, an inventory size, a prose "N ops"),
neither side is likely to be correct: one branch added items, the other removed them, and the
merged truth is a third number. Derive it by counting the merged artifact itself, never by taking
a side or by arithmetic on the two claims. Then hunt the count's SEMANTIC TWINS — the same fact
restated in files that auto-merged cleanly, where git's line-based merge saw no conflict at all.
A clean auto-merge is not evidence of a correct merge for any fact stated in more than one place.

## War story
- 2026-07-16 (#79, PR #86) — Two in-flight changes (0077 codex-TOML emission, 0079 runner shim)
  edited the same `sync-agents.sh` emitter; the second to merge hit a REAL semantic rebase conflict,
  not a textual one — both features had to COMPOSE (route the shim's native paths through the
  harness-aware `emit_for_harness` chokepoint) rather than pick a side. It stayed resolvable only
  because 0079's edits were additive (new functions + call-site swaps, per its own results note).
- 2026-07-18 (#89, PR #99) — Change 0095 retired the `auto_approve` subsystem on main while 0089
  added two reclaim knobs; the rebase produced six hunks that were almost all COUNTERS moving in
  opposite directions (a `KEY=value` line-count assertion 22→21 vs 22→24, resolving to 23; a
  facade op list 13 vs 15, resolving to 14). Taking either side verbatim would have routed a facade
  op to a script main had deleted. The counts were settled by counting the merged artifacts, not by
  arithmetic. The trap was in `scripts/docket-config.md`, which auto-merged CLEANLY and so silently
  took the PR's now-wrong numbers — the same fact as the conflicted test assertion, invisible to a
  line-based merge.
