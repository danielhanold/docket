---
id: 123
slug: machine-check-the-docket-config-md-export-list-order-against
title: Machine-check the docket-config.md export list order against the resolver
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [102]
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
type: chore
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`scripts/docket-config.md` carries a fenced list of the resolver's exported keys. Two guards touch
it today and neither covers sequence: `R7` anchors the resolver's *runtime emission order*, and `R8`
anchors the doc's *presence* of each key. Nothing verifies that the fenced list's **order** matches
the order the resolver actually emits.

That leaves the documented export list free to drift into a wrong sequence while both guards stay
green — the same documented-but-unverified shape change #0102 spent five review rounds closing for
key *wiring*, one axis over. It is pre-existing, and #0102 widened the list by one entry.

Surfaced in #0102's results as **capped overflow**: the build's auto-capture cap of 3 was already
spent on #120/#121/#122, so this was reported with "file this one by hand if you want it tracked."
Filing it rather than letting it decay in a results file.

## What changes

- Add a check that the fenced export list in `scripts/docket-config.md` matches the resolver's
  emission order exactly — sequence, not just set membership.
- Derive the expected order from the resolver itself, never from a second hand-maintained list
  (see `enumerated-floor` and `correspondence-guard-runs-one-way`).
- Mutation-test it: reorder two adjacent entries in the doc and confirm it reddens. Per #0102's
  pairing lesson, also confirm it reddens on a *rename* that keeps the count stable.

## Out of scope

- The set-membership guarantee, which `R8` already provides.
- Any change to the resolver's actual emission order or to which keys it exports.

## Open questions

- Is emission order a property worth pinning at all, or is the doc's list better re-specified as an
  explicitly unordered set (which would close the drift by deleting the claim rather than guarding
  it)? Decide this first — it may make the guard unnecessary.
- If ordered: does the resolver's emission order have a stable derivation, or is it incidental to
  the code's layout and therefore a churn source on every future edit?
