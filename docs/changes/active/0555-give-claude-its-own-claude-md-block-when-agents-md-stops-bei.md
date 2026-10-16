---
id: 555
slug: 'give-claude-its-own-claude-md-block-when-agents-md-stops-bei'
title: 'Give Claude its own CLAUDE.md block when AGENTS.md stops being shared'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-16'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [537]
discovered_from: [537]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
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

In a shared repository where docket made CLAUDE.md a link to AGENTS.md, Claude reads the dispatch block that codex or opencode owns. If codex and opencode are dropped from `agent_harnesses` and claude stays, install removes the AGENTS.md block but keeps the link. Claude then reads an AGENTS.md without the dispatch and run-tracker rules, and every later install skips the link, so the repository stays without them until someone replaces the link by hand. Once #537 deletes an AGENTS.md that is left empty, the link points at nothing. Found while grooming #537.

## What changes

- When AGENTS.md is no longer shared and claude is still opted in, replace the CLAUDE.md link docket's ownership record holds with a regular CLAUDE.md carrying Claude's own dispatch block, the same file an install with no CLAUDE.md writes.
- The install transaction can replace a link with a file at the same path. Today it refuses to write a block through a link, and one run cannot both remove and write the same path.

## Out of scope

- Deleting an AGENTS.md left empty when a block is removed (#537).
- Private repositories, which keep no surfaces in the working tree.
