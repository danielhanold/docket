---
id: 443
slug: 'clarify-gate-operation-ids-versus-executable-argv'
title: 'Clarify gate operation IDs versus executable argv'
status: 'proposed'
priority: 'low'
type: 'docs'
created: '2026-09-22'
updated: '2026-09-22'
depends_on: []
stacked_on:
related: [394, 395]
discovered_from: []
adrs: [104]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

Observed once on Sonnet while starting docket-implement-next: the agent attempted `docket run.gate-before implement-next 2>&1`, then corrected it to `docket run gate-before implement-next`. This single reported occurrence warrants a small, low-priority wording improvement; recurrence is not established.

## What changes

Suggested direction for later grooming: clarify the shared run-gate instructions in `cursor-rules/run-gate.md` so the parent coordinator first fetches `docket capabilities --json`, finds the entry whose `id` is `run.gate-before`, and executes that entry's `argv` with `implement-next` appended. Explicitly distinguish a dotted operation ID from executable arguments and say never to execute the ID itself. Carry the clarification through the normal generated instruction surfaces. Keep the change narrowly scoped to this wording.

## Out of scope

Full grooming, a spec or implementation plan, implementation now, CLI aliases or behavior changes, and broader dispatch redesign.
