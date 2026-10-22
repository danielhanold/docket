---
id: 578
slug: 'invoke-the-artifact-export-from-an-agent-session'
title: 'Invoke the artifact export from an agent session'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-10-22'
updated: '2026-10-22'
depends_on: []
stacked_on:
related: [561]
discovered_from: [561]
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

#561 added a clean Markdown export of a change, spec, plan, or results file, reached today only by a person typing `docket open <what> <id> --export` in a shell (for example `docket open spec 9 --export`). Inside an agent session there is no docket workflow that does this, so a request like "export the spec for 9" leaves the agent to guess: run the command by hand, or worse, rebuild the cleanup itself in prose. A hand-rebuilt export drifts from the real one and skips its self-check warnings.

## What changes

Give agent sessions a docket-owned way to ask for an export that runs the exact same export logic as the CLI flag, with no second implementation. The stub's starting hypothesis is a new skill (and, if it earns one, a matching `docket-*` agent) that resolves the export operation from the capability catalog, passes the artifact kind, change id, and any output path or warning filter through unchanged, and reports back the written file's path and any warnings. Grooming decides whether this is a skill, an agent, both, or simply an existing workflow learning to route the request; whether the export needs its own catalog entry or the `open` entry is enough; and how the agent gets the path without opening a viewer when no one is at the screen.

## Out of scope

Any change to what the export produces or how it cleans an artifact (owned by #561's code). New export formats. Exporting the board. Sending or attaching the exported file anywhere.
