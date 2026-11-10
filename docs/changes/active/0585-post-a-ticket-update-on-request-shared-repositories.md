---
id: 585
slug: 'post-a-ticket-update-on-request-shared-repositories'
title: 'Post a ticket update on request (shared repositories)'
status: 'implemented'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-10'
depends_on: [583]
stacked_on:
related: [581, 584, 586, 587, 591]
discovered_from: [581]
adrs: [162]
spec: 'docs/superpowers/specs/2026-11-10-post-a-ticket-update-on-request-shared-repositories-design.md'
plan: 'docs/superpowers/plans/2026-11-10-post-a-ticket-update-on-request-shared-repositories.md'
results: 'docs/results/2026-11-10-post-a-ticket-update-on-request-shared-repositories-results.md'
auto_groomable:
hold:
branch_prefix:
branch: 'feat/post-a-ticket-update-on-request-shared-repositories'
pr: 'https://github.com/danielhanold/docket/pull/452'
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-10-post-a-ticket-update-on-request-shared-repositories-design.md](../../superpowers/specs/2026-11-10-post-a-ticket-update-on-request-shared-repositories-design.md) |
| Plan | [2026-11-10-post-a-ticket-update-on-request-shared-repositories.md](../../superpowers/plans/2026-11-10-post-a-ticket-update-on-request-shared-repositories.md) |
| Results | [2026-11-10-post-a-ticket-update-on-request-shared-repositories-results.md](../../results/2026-11-10-post-a-ticket-update-on-request-shared-repositories-results.md) |
| ADRs | [ADR-0162](../../adrs/0162-ticket-work-is-two-inline-skills-split-by-attended-linking-v.md) |
<!-- docket:artifacts:end -->

## Why

People working in the ticket system should get short, deliberate updates at the moments that matter, with a link to the artifact as it stood then.

## What changes

One read operation, `context.ticket`, returns everything an update needs for a change and an event: the ticket link, the repository's visibility, whether the event has happened and the facts behind it, and spec and results links pinned to a commit. A new inline skill, `docket-update-ticket`, posts a short comment on request after any of four events (spec written, PR opened, PR merged, results completed), and for revision, blocker, and kill updates. It refuses when the change has no ticket or the event has not happened, writes the comment for a person reading it in the ticket system, mentions the people the requester names so they are notified, and reports an update as posted only on the connector's confirmation. The PR-opened update links the PR, the spec, and the results. Links live in the comment only. In a private repository it posts nothing yet.

It also fixes two leftovers from change 584: `docket-export`'s example that implies it posts ticket comments, and the config reference calling `tickets.provider`, `tickets.site`, and `tickets.project` unused.

## Acceptance criteria

`context.ticket` returns the ticket state, visibility, the event's switch and whether it happened, the facts the update needs, and pinned spec and results links, with tests for each event; the skill refuses a change without a ticket, a broken link, a mismatched ticket, an event that has not happened, and a private repository; a mention goes out only for exactly one match; repository guards check the skill's text; results list an Important live check from another repository.

## Out of scope

Private repositories (change 586); automatic posting (change 587); following a team's own ticket skill (change 591); native ticket web links; attachments.

## Reconcile log

### 2026-11-10

Unchanged against main 6dacf9706: the spec was verified at this tip; dependency #583 and related #584 are done, and #586/#587/#591 remain proposed and out of scope.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: ab87dd3b6503c662eae7891dbd18f515a8824cb5
ran_at:   2026-11-10T15:20:58Z
<!-- docket:build-evidence:end -->
