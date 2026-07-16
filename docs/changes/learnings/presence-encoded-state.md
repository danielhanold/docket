---
slug: presence-encoded-state
hook: "When state is encoded by an artifact's presence, every transition out of that state must remove the artifact."
topics: [design, state, views]
changes: [14]
created: 2026-06-12
updated: 2026-06-12
promotion_state: retained
promoted_to:
---

## Apply
When state is encoded by an artifact's presence, every transition out of that state must remove the
artifact.

## War story
- 2026-06-12 (#14, PR #10) — Two views keyed off a body section's *presence* (board cell,
  selection band), but the state transition out (re-arm) didn't remove the section — a re-armed
  stub stayed mislabeled.
