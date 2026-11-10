---
id: 585
slug: 'post-a-ticket-update-on-request-shared-repositories'
title: 'Post a ticket update on request (shared repositories)'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-10'
depends_on: [583]
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

People working in the ticket system should get short, deliberate updates at the moments that matter, with a link to the artifact as it stood then.

## What changes

One read returns everything an update needs for a change and an event. `docket-update-ticket` composes the update under the editorial contract and posts it as a comment with a commit-pinned link, only for a linked ticket and only if the event has happened; a late update never misstates the current state. Failures are one report line; nothing is reported posted without the connector's confirmation. Fixed by the spec: the four comment events (spec written, PR opened, PR merged, results completed) and their content, no ledger or retry, plans never go out. An on-request change-created update is ticket creation, which belongs to `docket-link-ticket` (change 584); this skill points the person there. Left to grooming: the read operation's exact fields, whether shared mode also adds a native web link.

**Acceptance criteria:** The read returns ticket state, visibility, switch, event facts, artifact paths, and pinned URLs, with tests; the skill refuses a change without a ticket and an event that has not happened; results list an Important live check.

**Covers (581 spec):** decision 1 (on request), decision 2 (the four comment events' preconditions and content; the on-request-only cases), decision 3 (except attachment order), decision 9 (shared delivery), decision 10's read operation, the `docket-update-ticket` skill and its progressive-disclosure layout, decision 11, decision 12

**Likely touches:** new read operation (`context.ticket`, see answer (c)); a commit-pinned URL builder beside `BlobURL` in `internal/render/link.go`; new inline skill `docket-update-ticket` with its per-event table and a shared-delivery reference

**Design source:** this change is position 4 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 4, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 1 (How an update reaches a ticket), 2 (Events), 3 (Who posts, and what happens on failure), 9 (Delivery by visibility — shared), 10 (Layering — `docket-update-ticket` and the read operation), 11 (Editorial contract), and 12 (Harness responsibility), plus *Vocabulary*, *Out of scope*, and *What the code says today*; and the results' answer (c) on the read operation's shape. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

**Known leftover from change 584:** `docket-export` still uses the example "add the spec for 9 as a comment on PROJ-123", which could steer an agent toward posting a ticket comment through export. Ticket work now lives in `docket-link-ticket` (584), and this change adds `docket-update-ticket`, so this change also updates that example (or removes it) so export no longer implies it can post ticket comments, and points that kind of request to `docket-update-ticket`.

## Out of scope

Private repositories (change 586); automatic posting (change 587).
