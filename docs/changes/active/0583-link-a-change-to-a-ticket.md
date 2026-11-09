---
id: 583
slug: 'link-a-change-to-a-ticket'
title: 'Link a change to a ticket'
status: 'in-progress'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: []
stacked_on:
related: [581]
discovered_from: [581]
adrs: [161]
spec: 'docs/superpowers/specs/2026-11-09-link-a-change-to-a-ticket-design.md'
plan: 'docs/superpowers/plans/2026-11-09-link-a-change-to-a-ticket.md'
results: 'docs/results/2026-11-09-link-a-change-to-a-ticket-results.md'
auto_groomable:
hold:
branch_prefix:
branch: 'feat/link-a-change-to-a-ticket'
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-09-link-a-change-to-a-ticket-design.md](../../superpowers/specs/2026-11-09-link-a-change-to-a-ticket-design.md) |
| Plan | [2026-11-09-link-a-change-to-a-ticket.md](../../superpowers/plans/2026-11-09-link-a-change-to-a-ticket.md) |
| Results | [2026-11-09-link-a-change-to-a-ticket-results.md](../../results/2026-11-09-link-a-change-to-a-ticket-results.md) |
| ADRs | [ADR-0161](../../adrs/0161-ticket-fields-are-three-flat-stored-scalars-and-issue-is-ret.md) |
<!-- docket:artifacts:end -->

## Why

Every ticket capability needs a durable, typed link from a change to its ticket and a place for the team's ticket settings. The old `issue:` field is an unused leftover of the removed GitHub board mirror and would confuse the new vocabulary.

## What changes

A change gets three flat fields, `ticket_provider`, `ticket_id`, and `ticket_url`, stored as given rather than derived. One new operation, `change.link-ticket`, writes them together in any status: link, replace (only on an explicit request, which a human may make in plain words), and unlink. It refuses a partial set and re-renders `## Artifacts` with a Ticket row. `change.create` accepts the same three fields. A hand-edited partial or invalid set counts as not linked and is reported as a warning finding, never a block.

The `tickets.*` config keys (`provider`, `site`, `project`, and the five `auto.*` event switches, all off) arrive as normal per-repo-only settings; nothing reads them until later ticket changes. implement-next's PR body gets one plain line linking the ticket, read from a typed field in the implementation context. The unused `issue:` field is removed from every read path; the four archived records that carry it empty are left alone. A stale "issue" in `docket open --help` is corrected to "ticket".

## Acceptance criteria

- Linking writes all three fields and a Ticket row in one commit; re-linking the same ticket writes nothing; linking over a different ticket is refused without an explicit replace; unlinking clears all three; a partial request is refused.
- `change.create` with all three fields creates a linked change; with one or two it refuses.
- Linking works on an archived (`done`) change.
- A hand-edited partial or invalid set produces a warning finding and is treated as not linked.
- `tickets.*` set in global or machine-local config is warned and ignored.
- implement-next's PR body carries the ticket line when the change is linked.
- No record or operation reads `issue:`.
- docket's own `.docket.yml` carries no `tickets.*` key, and a test guards it.

## Out of scope

Any connector call; skills that create or update tickets (changes 584–588); board changes.

## Reconcile log

### 2026-11-09

Unchanged against main 64018716f: the export-kind rename (KindTicket) the folded-in open-help fix depends on has landed, and the issue: read sites the spec names are still present.
