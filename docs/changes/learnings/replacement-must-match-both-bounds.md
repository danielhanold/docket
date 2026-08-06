---
slug: replacement-must-match-both-bounds
hook: "When a new routing rule replaces a fixed escalation ladder, check the FLOOR as well as the ceiling — a matching ceiling makes the claim of equivalence look proven while the strongest tier silently becomes unreachable."
topics: [design, guards, review]
changes: [218]
created: 2026-08-06
updated: 2026-08-06
promotion_state: candidate
promoted_to:
---

## Apply
A ladder has two bounds. Replacing it with content-based routing preserves equivalence only if
both are preserved; verify the floor explicitly for the cases the old ladder started high on, and
if you carve out an exception, state it as a deliberate exception to the orthogonality the rest of
the rule claims. Do not let a file assert equivalence it has not shown for both bounds.

## War story
- 2026-08-06 (#218, PR #162) — Character-based routing matched the old ladder's ceiling but not its
  floor: pre-change every blocker fix ran `standard` and escalated to `premium` before halting;
  under pure character routing a blocker whose fix *looks* mechanical routes `economy`, escalates
  once to `standard`, and halts — `premium` never tried. It weakened the one gate the change
  existed to protect. Fixed with a blocker floor at `standard`, recorded as ADR-0070 together with
  the never-`max` ceiling, since the two bounds are one decision surface.
