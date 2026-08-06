---
id: 70
slug: fix-loop-profile-envelope-blocker-floor-and-max-ceiling
title: The fix loop's profile envelope — a blocker floor at standard, a ceiling below max
status: Accepted
date: 2026-08-06
supersedes: []
reverses: []
relates_to: [66]
change: 218
---

## Context

Change 0218 added a bounded fix loop inside `docket-implement-next` Step 6 that repairs review
findings on the open feature branch before the PR opens. Its organizing rule is that the fix's
**character** picks the model profile — via the routing rubric now shared at
`skills/docket-build/references/task-routing.md` — while the finding's **severity** picks only the
failure posture. That orthogonality is deliberate: a `minor` finding whose fix is genuinely subtle
must not be handed to a cheap model for being labelled minor, and a `blocker` whose fix is a
one-word typo must not burn a premium dispatch for being a blocker.

Two boundary questions fall out of the loop, and pure orthogonality answers both wrongly.

- **How big a fix may happen in-branch at all?** A finding discovered at review time is an
  unplanned side-quest on a branch that already has a plan behind it.
- **How small may a blocker's fix start?** Character routing can send a blocker whose fix *looks*
  mechanical to the cheapest profile — and misclassification is exactly what the blocker gate
  exists to survive.

## Decision

The fix loop's profile envelope is bounded at both ends. Each bound is an exception in one
direction.

**1. Ceiling — no fix task dispatches the `max` profile, at any severity.** `premium` is
"consequential but correctable" — still walk-backable inside a reviewed diff. `max` is defined by
irreversibility (unresolved architecture, an irreversible data change), and an irreversible act must
never happen to a branch as an unplanned side-quest discovered at review time. The routing rubric
therefore doubles as the in-branch size ceiling; there is **no separate "too big to fix in-branch"
knob**. A max-character `blocker` halts (abort-and-report). A max-character `important` or `minor`
becomes a PR-body record for the human's merge-time judgment — never a follow-up change.

**2. Floor — a blocker's fix task starts no lower than `standard`, regardless of its character.**
This *is* a deliberate exception to the character/severity orthogonality and is stated as such
wherever the rule is written. Without it, character routing silently weakens the one gate the change
insists must never weaken: pre-0218, every blocker fix ran `standard` and escalated to `premium`
before halting; under pure character routing a blocker whose fix looks mechanical routes `economy`,
escalates once to `standard`, then halts — `premium` is never tried, and a
misclassified-as-easy blocker halts an autonomous run that previously would have recovered. The
blocker is the one gate that must not fail open, so it may not start below the uncertainty sink.

(The floor was surfaced by the deep whole-branch review of 0218's own branch and fixed in commit
`c2055a47`.)

## Consequences

- **Enables:** the fix loop repairs findings autonomously with no risk of an irreversible unplanned
  change, and with no separate size knob to tune or let drift out of sync with the routing rubric.
- **Costs:** the orthogonality claim is no longer clean. The prose carrying these rules must state
  the floor as *the one deliberate exception* and say why, or a later reader reads the two rules as
  contradictory. Blocker fixes also give up the theoretical cost saving of an `economy` dispatch.
- **Gives up:** the ability to fix a max-character finding in-branch at all. Such findings leave the
  loop as PR-body records (non-blockers) or halts (blockers) — the conservative direction.
- **Unchanged:** ADR-0066 (docket owns the review role; the suite runs in the build gate). The
  reviewer stays read-only; the fixing is the implementer's.
