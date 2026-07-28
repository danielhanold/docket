---
id: 149
slug: make-the-prelude-guard-s-exemption-bound-proportional-and-cl
title: Make the prelude guard's exemption bound proportional, and close the partial-rename gap
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [126]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0126's correspondence guard defends itself against a degenerate key set with
`assert '[ "$t_exempt" -le 5 ]'` — a fixed ceiling against a real value of 3. The whole-branch
review accepted it as non-blocking and flagged two residual weaknesses, both recorded at the merge
gate rather than fixed:

- **The bound is absolute, not proportional.** A drift-proof form (`[ $((t_exempt * 5)) -le "$t_sites" ]`,
  or a floor on `ok` rather than a ceiling on `exempt`) would cost nothing and would not need
  revisiting as the file grows. Today's two sites of headroom are thin: several legitimately-exempt
  fixtures landing together would trip it.
- **Only wholesale degeneracy is caught.** The ceiling exists because a *wrong* key set makes every
  site "exempt by derivation" and the guard goes fully vacuously green. But a **partial** rename —
  say 5 of 28 export keys — raises `exempt` only slightly and slips under 5. Those sites go silently
  unguarded while the suite stays green.

The failure mode of the current bound when it ages is a loud false red, not a silent pass, which is
why it was parked rather than treated as blocking. The partial-rename gap is the more interesting
half: it is a silent hole, just a narrow one.

## What changes

- Replace the absolute ceiling with a proportional bound or an `ok` floor.
- Decide whether the partial-rename gap is worth closing, and if so how — the honest options are a
  per-site expectation that each site's intersection be non-empty unless derivably exempt, or
  anchoring the key set against a second independent source.

## Out of scope

- Re-litigating the guard's clearing-window semantics or its mirror-vs-subset ruling, both settled
  in 0126.
