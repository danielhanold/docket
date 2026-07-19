---
slug: presence-encoded-state
hook: "When state is encoded by an artifact's presence, every transition out of that state must remove the artifact."
topics: [design, state, views]
changes: [14, 87]
created: 2026-06-12
updated: 2026-07-19
promotion_state: candidate
promoted_to:
---

## Apply
When state is encoded by an artifact's presence, every transition out of that state must remove the
artifact.

## War story
- 2026-06-12 (#14, PR #10) — Two views keyed off a body section's *presence* (board cell,
  selection band), but the state transition out (re-arm) didn't remove the section — a re-armed
  stub stayed mislabeled.
- 2026-07-19 (#87, PR #103) — Re-hit three times over in **one** change, on a second marker
  (`## Finalize blocked`) designed with this finding already in hand: (a) a re-mark **appended** a
  second heading instead of replacing — reachable, since retries are explicitly supported via the
  named-id override; (b) a change carrying a stale marker that the human then merged **by hand**
  was skipped by finalize forever — never archived, board showing `finalize blocked — needs you`
  for an already-merged change; the skip is now scoped to *unmerged* candidates; (c) the
  `drained`/`halted` boundary was decidable two ways, depending on whether a marker-skipped
  candidate counted toward the non-empty set. Promoted to **candidate** on this hit: retrieval was
  not the failure mode — the rule was retrieved and the transitions were still missed at authoring
  time, so it needs to fire unprompted whenever a marker's *presence* carries state. The
  enumeration to run at design time is: what **re-**enters this state, what leaves it by a path the
  system doesn't drive (a human acting out of band), and what merely *reads* it.
