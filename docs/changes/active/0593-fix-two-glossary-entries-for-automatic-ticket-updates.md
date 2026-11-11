---
id: 593
slug: 'fix-two-glossary-entries-for-automatic-ticket-updates'
title: 'Fix two glossary entries for automatic ticket updates'
status: 'proposed'
priority: 'low'
type: 'docs'
deliverable: 'pr'
created: '2026-11-11'
updated: '2026-11-11'
depends_on: []
stacked_on:
related: [588]
discovered_from: [587]
adrs: []
spec:
plan:
results:
auto_groomable: false
hold:
branch_prefix:
branch:
pr:
ticket_provider:
ticket_id:
ticket_url:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 587 (PR #457) added an automatic ticket-update path: the groom-next, auto-groom, implement-next (Step 7 and its results-only ending) and finalize-change workflows now call docket-update-ticket on their own when a tickets.auto.* switch is on. Two entries in docs/reference/glossary.md still describe only the old, manual path. "Ticket context" says the bundle is read by docket-update-ticket, but those five workflow sites now read it too. "Interactive vs autonomous skills" says docket-update-ticket stays inline because it posts "with your session's own tools"; dispatched autonomous agents now invoke it automatically, and whether they can reach the ticket connector is not yet confirmed. That sentence is pinned verbatim by an earlier prose-contract test, so the test contract has to change together with the glossary row.

## What changes

Reword the "Ticket context" glossary entry so it names the five workflow sites as readers alongside docket-update-ticket. Reword the "Interactive vs autonomous skills" entry so it is accurate for the automatic path, and update the prose-contract test that pins the old sentence in the same commit. Do this only after the live ticket-integration check listed in change 587's results file (run from a shared scratch repository against a sandbox ticket project) has settled whether dispatched agents can reach the ticket connector, because the wording of the second entry depends on that answer.

## Out of scope

Any change to the automatic ticket-update behavior itself, to the tickets.auto.* switches, or to the live check. Glossary entries unrelated to ticket updates.
