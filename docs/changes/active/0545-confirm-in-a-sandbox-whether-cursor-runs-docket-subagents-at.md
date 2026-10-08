---
id: 545
slug: 'confirm-in-a-sandbox-whether-cursor-runs-docket-subagents-at'
title: 'Confirm in a sandbox whether Cursor runs docket subagents at their pinned models'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-08'
updated: '2026-10-08'
depends_on: []
stacked_on:
related: [512]
discovered_from: [512]
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

In the v1.0.0-alpha.2 Cursor acceptance run (change 0512), docket's subagents appeared to run at a different model effort than their installed pins. `docket-implement-next` and `docket-review-lean` are pinned to grok-4.5 medium and `docket-plan-writer` to grok-4.5 low, yet Cursor's agent records show all three at grok-4.5 high. The dispatching agent also passed an explicit `model` parameter on each dispatch, so the observation may be the parent overriding the pin rather than Cursor ignoring the wrapper's `model:` line. The alpha.2 release notes list it as a known gap. Before anything is fixed, we need to know which it is.

## What changes

- **A sandbox probe, no product code.** In an isolated Cursor test home (the 0512 method: its own `--user-data-dir`, `HOME`, and a release install with `--harness cursor`), install agents with distinct, easy-to-tell-apart pins and dispatch them:
  - with no `model` parameter from the parent;
  - with a `model` parameter that differs from the pin;
  - from a parent chat running a different model.
- Read the model each child actually ran at from Cursor's own records (the test profile's `state.vscdb`: each subagent conversation's `modelConfig`), not from the agent's self-report.
- **Report** which of these holds: Cursor honors the wrapper's `model:` line unless the parent overrides it; Cursor ignores it for user-level agents; or something else. Note the Cursor version.
- If a docket-side fix follows (for example, the dispatch rule telling the parent never to pass `model`), the report names it for a human to capture as its own change.

## Out of scope

- Fixing anything; this change only finds out.
- Other harnesses.
- The other alpha.2 Cursor findings (the sandboxed Allowlist mode, the orphaned `cursor-agent` worker).
