---
slug: concurrent-edits-compose-at-rebase
hook: "When two open changes touch one function, keep each additive and funnel through a shared chokepoint; at rebase reconcile by INTENT — compose, don't choose."
topics: [git, rebase, concurrency]
changes: [79]
created: 2026-07-16
updated: 2026-07-16
promotion_state: retained
promoted_to:
---

## Apply
When two open changes touch one function, keep each additive and funnel through a single
shared chokepoint; at rebase reconcile by INTENT (compose, don't choose), then run BOTH changes'
twin test suites green before trusting the merge — `bash -n` + no-conflict-markers is not enough.

## War story
- 2026-07-16 (#79, PR #86) — Two in-flight changes (0077 codex-TOML emission, 0079 runner shim)
  edited the same `sync-agents.sh` emitter; the second to merge hit a REAL semantic rebase conflict,
  not a textual one — both features had to COMPOSE (route the shim's native paths through the
  harness-aware `emit_for_harness` chokepoint) rather than pick a side. It stayed resolvable only
  because 0079's edits were additive (new functions + call-site swaps, per its own results note).
