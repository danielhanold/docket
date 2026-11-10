---
id: 586
slug: 'post-a-ticket-update-on-request-in-a-private-repository'
title: 'Post a ticket update on request in a private repository'
status: 'implemented'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-09'
updated: '2026-11-10'
depends_on: [585]
stacked_on:
related: [581, 585, 588]
discovered_from: [581]
adrs: []
spec: 'docs/superpowers/specs/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-design.md'
plan: 'docs/superpowers/plans/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository.md'
results: 'docs/results/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-results.md'
auto_groomable:
hold:
branch_prefix:
branch: 'feat/post-a-ticket-update-on-request-in-a-private-repository'
pr: 'https://github.com/danielhanold/docket/pull/453'
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-design.md](../../superpowers/specs/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-design.md) |
| Plan | [2026-11-10-post-a-ticket-update-on-request-in-a-private-repository.md](../../superpowers/plans/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository.md) |
| Results | [2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-results.md](../../results/2026-11-10-post-a-ticket-update-on-request-in-a-private-repository-results.md) |
<!-- docket:artifacts:end -->

## Why

In a private repository the artifact cannot be linked; it must be cleaned and attached without leaking docket.

## What changes

In a private repository, `docket-update-ticket` stops refusing and delivers the update as attachments plus a comment. Each artifact the update names (the same per-update list shared delivery links) is exported and cleaned through `docket-export` as a file request, uploaded to the ticket first (in two steps where the connector needs it), and named in the comment, which goes last. The PR stays a link. The agent writes the comment under the private-repository writing rule and quietly fixes its own text after a whole-word check over the comment and the attachment file names. The person is asked once, only when the export rewrote something, with the before → after list; otherwise the update posts straight away. Any failed or uncertain upload posts no comment, and the report names what already landed and the exported paths.

No binary change: `context.ticket` and `docket-export` already provide what this needs. The work is a new `references/private.md`, the private route in `SKILL.md`, docs, guards, and budgets.

## Acceptance criteria

- In a private repository the skill no longer refuses; it loads its private-delivery reference.
- The skill text covers the attachment list, the export call, the comment's writing rule and quiet self-fix, the single review stop on a rewrite, attachments first and comment last, the two-step upload, and the no-comment rule on any failed upload; guards pin each and redden when it is removed.
- The results list an Important live check from a private scratch repository against a sandbox ticket project, including a forced upload failure.

## Out of scope

Unattended private posting, the final leak-pattern check, and unknown visibility treated as private (change 588); automatic posting (change 587); any change to `context.ticket`, `docket-export`, or the leak patterns; deleting attachments from a ticket, retrying, or recording what was posted.

## Reconcile log

### 2026-11-10

Unchanged against `main` 3243fa5f2. Change 585 is done and its follow-up fixes (pinned reads, killed reason, mention placeholder) do not touch the private route; the `visibility` refusal in SKILL.md section 2 and the 0585 sentinel are still as the spec describes.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: f3b5c34e2bc499beaef7c123c0ada3bd901ff2f1
ran_at:   2026-11-10T18:49:39Z
<!-- docket:build-evidence:end -->
