---
id: 139
slug: extend-the-tiered-dispatch-unavailability-posture-to-finaliz
title: Extend the tiered dispatch-unavailability posture to finalize's two in-context-gating dispatches
status: killed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [137]
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

Change 0137 added a normative *Dispatch-capability resolution* rule to docket-convention: a
dispatch-dependent step may be declared unavailable only after resolving a dispatch mechanism
(including searching deferred tool surfaces) and, if inconclusive, attempting one trivial dispatch.
The rule binds **every** dispatch-dependent step. The tiered posture that follows it, however, has
rows for only four dispatch kinds:

- Tier A — the `docket-status` and `docket-adr` composition dispatches (run inline; the contract is
  git state)
- Tier B — the `docket-auto-groom-critic` gate (abstain)
- Tier C — the `build` and `review` role skills (authorized-or-halt)

`docket-finalize-change` dispatches two more that match no row: **`docket-rebase-resolver`** (on a
merge-gate rebase conflict) and **`docket-integration-repair`** (on a red rebased suite). Those two
are genuinely different in kind from the four above — their reports flow back to finalize
**in-context to gate the merge**, rather than landing as git state on `metadata_branch`. So neither
Tier A's "inline is an equivalent path because the contract is git state" nor Tier C's
"authorized-or-halt" is obviously the right posture for them.

Surfaced by the whole-branch review of change 0137 (2026-07-25) and left deliberately unresolved:
inventing a posture mid-build would have shipped a design decision that no brainstorm raised and no
critic gated. Recorded here instead so it gets real design attention.

**This is not currently unsafe.** `docket-finalize-change` already carries an explicit
*abort-and-report points (the full set)* section naming both situations these helpers exist for —
an ambiguous rebase conflict, and a repair that cannot reach green in ≤2 attempts. In both, the
existing rule already fires: leave the PR open, leave the change `implemented`, surface to the
human. An agent that cannot dispatch a resolver therefore falls into a documented posture one file
away rather than improvising. The gap is a **completeness** gap in the taxonomy, not a behavioral
hole — which is exactly why it was judged safe to defer.

## What to decide

- Do the two in-context-gating dispatches get a **fourth tier** of their own, or an explicit
  **carve-out** clause stating they sit outside the taxonomy and keep finalize's existing failure
  postures?
- If a fourth tier: what *is* the posture? Candidates — halt (safest, matches finalize's existing
  abort-and-report), or attempt inline resolution/repair with the existing sign-off gate still
  applying. Note that inline repair by the agent that will then merge its own repair is the same
  self-approval shape Tier B rejects for the critic, which argues for halt.
- Whichever way it goes, the convention's tier table and `docket-finalize-change` should agree, and
  the guard in `tests/test_dispatch_capability.sh` should cover the new site(s) — its reverse check
  derives the dispatch-site population by shape from the skill files, so extending the taxonomy
  without wiring the sites will redden it (by design).

## Out of scope

- Re-litigating change 0137's four tiers; they are settled and shipped.
- Any change to when finalize dispatches these helpers at all — this is only about what happens
  when the dispatch mechanism itself is unavailable.

## Why killed

Consolidated into #0260 at the 2026-08-07 backlog triage: the two untiered finalize dispatches get the halt/carve-out default this change's own body argued for; the PENDING_TIER test block is rewired in the same change.
