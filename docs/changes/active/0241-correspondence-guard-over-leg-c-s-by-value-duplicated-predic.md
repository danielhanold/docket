---
id: 241
slug: correspondence-guard-over-leg-c-s-by-value-duplicated-predic
title: Correspondence guard over leg C's by-value duplicated predicate (ADR-0072 drift risk)
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [219]
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

**Trigger** — surfaced at #0219's close-out harvest. ADR-0072 accepts, by design, that leg C's
orphan-PR predicate is duplicated **by value** across `docket-status.sh` and `board-checks.sh` —
the idle floor, the ref resolution, the ahead-of-both-verified-bases guard with its empty-array
count gate, and the pushed/unpushed discrimination — because `board-checks.sh` must stay runnable
offline with no dependency on `docket-status.sh`. The ADR names the cost explicitly: nothing links
the two implementations, so a change that retunes leg C's floor or its base handling and forgets
`detect_orphan_pr` breaks the documented agreement **silently, with no test saying so**. Today's
only mitigation is a prose comment at each site naming the other.

**Opportunity** — a correspondence guard over the two implementations: assert they agree on the
idle-floor constant and on the shape of the base handling, anchored on the consuming code rather
than an allowlist, so a one-sided retune reddens. The agreement is already sold as load-bearing in
the spec, in both code comments, and in `docket-status.md`; nothing enforces it.

**Independent value** — stands with #0219 reverted in the sense that matters: the duplication and
its drift risk are what #0219 *shipped*, and this guard is what makes that shipped decision safe
over time. Any future retune of either site is the event it protects against.

**Boundary** — a test-only change. It adds a correspondence guard (and whatever minimal shared
sentinel the two scripts need to be compared against); it does **not** refactor the duplication
into a shared helper, which ADR-0072 deliberately rejected, and it does not change either leg's
behavior.

**Reason for deferral** — #0219's branch is merged; it was scoped to building leg D and the leg C
enrichment, and its own results file records this as explicitly left to a human's judgment rather
than repaired in-branch.
