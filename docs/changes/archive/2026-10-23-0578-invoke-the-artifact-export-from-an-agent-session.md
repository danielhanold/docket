---
id: 578
slug: 'invoke-the-artifact-export-from-an-agent-session'
title: 'Invoke the artifact export from an agent session'
status: 'done'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-10-22'
updated: '2026-10-23'
depends_on: []
stacked_on:
related: [561, 542, 532]
discovered_from: [561]
adrs: [144]
spec: 'docs/superpowers/specs/2026-10-22-invoke-the-artifact-export-from-an-agent-session-design.md'
plan: 'docs/superpowers/plans/2026-10-22-invoke-the-artifact-export-from-an-agent-session.md'
results: 'docs/results/2026-10-22-invoke-the-artifact-export-from-an-agent-session-results.md'
auto_groomable:
hold:
branch_prefix:
branch: 'feat/invoke-the-artifact-export-from-an-agent-session'
pr: 'https://github.com/danielhanold/docket/pull/444'
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-22-invoke-the-artifact-export-from-an-agent-session-design.md](../../superpowers/specs/2026-10-22-invoke-the-artifact-export-from-an-agent-session-design.md) |
| Spec (merged) | [2026-10-22-invoke-the-artifact-export-from-an-agent-session-design.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/specs/2026-10-22-invoke-the-artifact-export-from-an-agent-session-design.md) |
| Plan | [2026-10-22-invoke-the-artifact-export-from-an-agent-session.md](../../superpowers/plans/2026-10-22-invoke-the-artifact-export-from-an-agent-session.md) |
| Results | [2026-10-22-invoke-the-artifact-export-from-an-agent-session-results.md](../../results/2026-10-22-invoke-the-artifact-export-from-an-agent-session-results.md) |
| ADRs | [ADR-0144](../../adrs/0144-private-visibility-leak-check-blocks-outgoing-pushes-and-pr.md) |
<!-- docket:artifacts:end -->

## Why

#561 added a clean Markdown export of a change, spec, plan, or results file, reached today only by a person typing `docket open <what> <id> --export` in a shell (for example `docket open spec 9 --export`). Inside an agent session nothing points at it, and nothing tells the agent that docket content needs cleaning before it leaves the repository. Asked to export an artifact, or to put one into another tool such as a Jira ticket through an MCP integration, agents have more than once skipped the export entirely and passed the content on with every docket reference, link, and generated block still in it.

In a private repository that is a leak the team must never see, and the export alone does not prevent it: by design it leaves sentences that name docket as written and only reports them.

## What changes

A new inline skill, `docket-export`, whose description matches any request to export a docket change, spec, plan, or results file, or to put one into anything outside docket. It has the agent run the real export — the `open` command from the capability catalog with `--export --print --json`, so no viewer opens — never the raw file or a hand cleanup. A failed export means nothing is posted.

In a shared repository the exported text goes to the target unchanged. In a private repository the agent also rewrites what the export leaves (lines naming docket, leftovers, docket's own change and ADR numbers) so they read naturally to an outsider, checks the result for the words docket and dckt, and, when it rewrote anything, shows the person before → after and posts only on their go. No agent, no new catalog entry, and no change to the export itself.

## Out of scope

Any change to what the export produces or how it cleans an artifact (owned by #561's code), its flags, or its catalog entry. New export formats. Exporting the board or a pull request. Sending logic in the skill: posting is done by the agent's own tools, and the skill carries nothing specific to Jira or any other tool. A rewrite pass in a shared repository. An always-loaded instruction or a `docket-convention` entry for the export.

## Reconcile log

### 2026-10-22

Unchanged against main 2dcf560b0: no docket-export skill exists yet, the export (#561) and its catalog entry are as the spec describes.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: b75f6fd9b712a78dc34483c0a652d940d8f52278
ran_at:   2026-10-22T23:43:01Z
<!-- docket:build-evidence:end -->
