---
id: 588
slug: 'update-tickets-automatically-in-a-private-repository'
title: 'Update tickets automatically in a private repository'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-11'
depends_on: [586, 587]
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

A private repository's automatic updates must go out without a person reviewing the cleanup, so something else has to stop a leak.

## What changes

Unattended, the update takes docket-export's file-request output, skips the before → after review, and runs a final check over the comment and every attachment with the leak check's existing patterns; any hit, or a docket-export word-check hit, posts nothing and writes one report line. Fixed by the spec: exactly the existing patterns, what they deliberately miss. Left to grooming: the matcher operation's name and request shape.

**Acceptance criteria:** The matcher returns the same hits as the publish leak check for the same text, with tests; the skill text states the post-nothing rule for both checks; results list an Important live check from a private scratch repository.

**Covers (581 spec):** decision 9 (unattended private posting, the final check, unknown visibility treated as private), decision 10's leak matcher exposed and the docket-export skip (answer (b))

**Likely touches:** a read-only operation exposing `internal/leakscan` for arbitrary text; `docket-update-ticket`'s private-delivery reference (unattended branch using docket-export's file-request path)

**Design source:** this change is position 7 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 7, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 9 (Delivery by visibility — unattended private posting, the final check, unknown visibility treated as private) and 10 (Layering — the leak matcher), plus *Vocabulary* and *Out of scope*; and the results' answer (b) on skipping docket-export's private stops. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

**Carried over from change 587:** 587 makes an automatic update in a private repository post nothing and report one line, and it says so in several places. This change lifts that rule, so its spec must name each place and update it:

- the concepts page `docs/concepts/tickets.md`, which 587 adds: it says that in a private repository an on-request update attaches cleaned copies while an automatic update posts nothing and reports one line. Update that statement wherever the page makes it (what goes with an update, the events, the invariants), and add this change's final check to the page;
- `docket-update-ticket`'s automatic path: the rule that any visibility other than `shared` posts nothing, and its report line (`automatic updates are not available in a private repository`);
- the prose-contract tests that pin that rule and that page;
- the reference entries for `docket-update-ticket` (glossary, skills list), if they mention the private case.

Derive the full list at grooming from a whole-repository search, not from this list. 587 may not be built when this stub is groomed, so read its spec for what it will add: decisions 4 (the automatic path), 7 (private repositories), and 9 (the concepts page).

## Out of scope

Widening the leak patterns.
