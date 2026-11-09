---
id: 587
slug: 'update-tickets-automatically-after-each-event'
title: 'Update tickets automatically after each event'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: [584, 585]
stacked_on:
related: [581]
discovered_from: [581]
adrs: []
spec:
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Teams that want it should get updates without the engineer remembering to ask, and nothing at all when they do not.

## What changes

At each event the running workflow reads its switch; when on, it invokes the ticket skill with a call-site direction that nobody is there to ask. Off means nothing happens: no prompt, no report line. In a private repository (or when visibility is unknown) this change posts nothing and reports one line; change 588 lifts that. The sweep never posts; stacked descendants get nothing. Fixed by the spec: per-event switches only, failures never halt. Left to grooming: whether change-created's readiness check runs before or after `change.create`.

**Acceptance criteria:** Each wiring site is guarded by a test; with every switch off no workflow output changes; results list an Important live check from a shared scratch repository.

**Covers (581 spec):** decision 1 (automatic, off means nothing), decision 2 (automatic per event; sweep and stack fan-out exclusions), decision 4's automatic ticket at change created, decision 10's workflow wiring

**Likely touches:** call sites in docket-new-change (change created, spec written), docket-groom-next, docket-auto-groom, docket-implement-next (PR handoff and results-artifact ending), docket-finalize-change (after close-out); `docket-update-ticket`'s unattended rules for a shared repository

**Design source:** this change is position 6 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 6, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 1 (automatic updates; off means nothing), 2 (Events — automatic per event, sweep and stack fan-out exclusions), 4 (automatic ticket at change created), and 10 (Layering — workflow wiring), plus *Vocabulary*, *Out of scope*, and *What the code says today*; and the results' answer (a) on what produces `stacked-merged`. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

## Out of scope

Unattended private posting (change 588); confirmation prompts; updates for revisions, blockers, or kills.
