---
id: 47
slug: readme-agent-config-discoverability
title: Make the agent-config refresh workflow discoverable in the README
status: done
priority: low
created: 2026-07-08
updated: 2026-07-08
depends_on: []
related: [16, 45, 46]
adrs: [8, 15]
spec:
plan: docs/superpowers/plans/2026-07-08-readme-agent-config-discoverability.md
results:
trivial: true
auto_groomable:
branch: feat/readme-agent-config-discoverability
pr: https://github.com/danielhanold/docket/pull/55
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-07-08-readme-agent-config-discoverability.md](https://github.com/danielhanold/docket/blob/feat/readme-agent-config-discoverability/docs/superpowers/plans/2026-07-08-readme-agent-config-discoverability.md) |
| PR | [#55](https://github.com/danielhanold/docket/pull/55) |
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
- Changing the effort-omission *mechanism* (point 3) — still out of scope: docket relies on the
  existing `effort: auto` sentinel, no code change. (Merge-gate re-scope 2026-07-08: the README now
  *documents* that existing behavior — that `effort: auto` drops the pinned effort line while omitting
  the key keeps the built-in — since surfacing it is precisely the model-override discoverability goal.
  The mechanism itself is unchanged.)

## Reconcile log

### 2026-07-08 — reconcile before build (no scope change)

- **Gap still real.** The refresh workflow is still buried in the Install section (repo-root
  `README.md` lines 61–64) plus the lone `agents:` line in the `.docket.yml` example (line 80); there
  is no discoverable section answering "how do I change an agent's model/effort?". The change's premise
  holds.
- **#0045 (multi-harness `agent_harnesses`) is `done` and live.** Verified `sync-agents.sh`: the
  per-repo project-level pass writes the committed `agent_harnesses` list, and the user-level pass
  writes every **present** harness root (`~/.<harness>/agents/`). The facts this change will document
  match current code exactly.
- **#0046 (per-harness models) is still `proposed`** (spec'd, not merged). The design decision to
  *reference* docket-convention's "Agent layer" for the config **shape** rather than restate field
  examples remains correct — it stays accurate whether or not #0046 lands first, which is the whole
  point (single source for the shape).
- **`sync-agents.sh --check`** confirmed as the CI drift gate. No work done elsewhere to drop; scope
  unchanged. Proceed to build as written.

### 2026-07-08 — merge-gate feedback (author): surface `effort: auto`

- The built section deferred all effort semantics to the docket-convention "Agent layer" reference.
  Author flagged at the merge gate that the most useful nugget for the motivating use case (overriding
  an agent's model, e.g. to a non-Claude Cursor model where Claude's effort tiers don't apply) is that
  `effort: auto` *drops* the effort line so the agent inherits the model default — verified against
  `sync-agents.sh:145`. Added a direct, code-accurate note + an owning section sentinel. PR #55 updated.
- **Discrepancy noted (not fixed here — #0046/convention territory):** docket-convention's Agent-layer
  comment reads "`effort: auto` (or omitted) → omit the effort line", but the code omits the line only
  for `auto`; **omitting the key keeps the built-in effort**. The README documents the code's actual
  behavior; the convention comment is left for a follow-up.
