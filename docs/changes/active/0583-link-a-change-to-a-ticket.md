---
id: 583
slug: 'link-a-change-to-a-ticket'
title: 'Link a change to a ticket'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-09'
depends_on: []
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

Every ticket capability needs a durable, typed link from a change to its ticket and a place for the team's ticket settings. The old `issue:` field is an unused leftover of the removed GitHub board mirror and would confuse the new vocabulary.

## What changes

Three flat fields, `ticket_provider`, `ticket_id`, `ticket_url`, written together by one operation (link, replace, unlink; refuses a partial set; any status; `## Artifacts` re-rendered with a Ticket row) and accepted by `change.create`. A partial set is reported as a finding only. The `tickets.*` keys, per-repo only, all automatic switches off. The PR body gets one plain line linking the ticket. `issue:` is removed from decode, domain, builder, repair, and implement-next. Fixed by the spec: field names, flatness, stored provider and id, no ticket-type key, no master switch. Left to grooming: the operation's name and request shape; whether the four archived empty `issue:` values stay as unknown data or are dropped.

**Acceptance criteria:** Linking writes all three fields and a Ticket row in one commit; unlinking clears them; a partial request is refused; `tickets.*` set in global or local config is warned and ignored; implement-next's PR body carries the ticket line; no record or operation reads `issue:`; docket's own `.docket.yml` carries no `tickets.*` key (a test guards it).

**Covers (581 spec):** decision 5 (all parts, including the retirement of `issue:` and the PR-body ticket line), decision 8 (config keys), decision 13's repository guard

**Likely touches:** frontmatter decode, domain model, `newChangeBuilder`, `ChangeRecord`, repair's field lists, `ArtifactBlockContent` (Ticket row); a new revision-pinned operation that sets or clears the three fields in any status (modelled on `change.relink`); `change.create` ticket keys; `internal/config` (`tickets.provider`, `site`, `project`, `auto.*`, all `scopeRepoOnly`) with defaults, `assemble`, `diagnostic.config`, `.docket.example.yml` (with the tool table as a comment); implement-next Step 7 (ticket line replaces the "Best-effort PR→issue reference"); convention field sketch and embedded twins

**Design source:** this change is position 2 of the ticket-integration split planned by change 581. The settled design lives in 581's spec, `docs/superpowers/specs/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-design.md`, and its plan in `docs/results/2026-11-09-split-the-ticket-integration-design-into-buildable-changes-results.md` (the entry for position 2, *Coverage check*, *ADRs*, and *Trace notes*). Binding sections: Decisions 5 (Ticket fields on the change), 8 (Config), and 13 (Testing — the repository guard), plus *Vocabulary*, *Out of scope*, and *What the code says today*. Grooming must read those sections and **restate the settled decisions this change carries in its own spec** — do not merely link to them — so the build needs nothing from change 581. Anything 581 marks as fixed is not reopened; only the points it leaves to grooming are decided here.

## Out of scope

Any connector call; skills that create or update tickets (changes 584–588); board changes.
