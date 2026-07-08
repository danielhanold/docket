---
id: 47
slug: readme-agent-config-discoverability
title: Make the agent-config refresh workflow discoverable in the README
status: proposed
priority: low
created: 2026-07-08
updated: 2026-07-08
depends_on: []
related: [16, 45, 46]
adrs: [8, 15]
spec:
plan:
results:
trivial: true
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

The workflow for refreshing an agent's model/effort **already exists** in the README — the Install
section says to re-run `sync-agents.sh` (or `install.sh`) after editing `~/.config/docket/agents.yaml`
and to run `sync-agents.sh --check` in CI for drift. But it is buried in dense Install-section prose,
so a user whose actual task is *"how do I change the model/effort an agent runs at?"* cannot find it.
Verified 2026-07-08: the docket author himself hit this exact discoverability gap while wiring a
non-Claude (Cursor) model override. This is a **discoverability** fix, not net-new documentation.

## What changes

Add a short, discoverable **agent-config** section (or clearly-titled subsection) to the repo-root
`README.md` that answers "how do I change an agent's model/effort, and how do I make it take effect?":

- Names the two config layers: **global** `~/.config/docket/agents.yaml` (user-level) and **per-repo**
  `.docket.yml` `agents:` (committed).
- The refresh command: `bash sync-agents.sh` (or re-run `install.sh`) after editing a config layer —
  it regenerates the generated wrapper copies.
- Which targets get refreshed: user-level writes every **present** harness root
  (`~/.<harness>/agents/`); per-repo project-level writes the committed `agent_harnesses` list (#0045).
- `sync-agents.sh --check` fails on drift (CI gate).
- **References docket-convention's "Agent layer" for the config *shape*** rather than duplicating field
  examples — so #0046's harness-first `agents:` rework changes the shape in one place and does not
  stale the README.

## Out of scope

- Any behavior change to `sync-agents.sh` (docs only).
- The `agents:` config **shape** itself — owned by #0046 (harness-first rework); this change references
  it, never restates it.
- The effort-omission ergonomics (point 3) — decided 2026-07-08 to rely on `effort: auto`; no change.

## Reconcile log
