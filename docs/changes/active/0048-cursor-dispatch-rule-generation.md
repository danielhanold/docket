---
id: 48
slug: cursor-dispatch-rule-generation
title: Generate Cursor dispatch rules; always write the full agent set per harness
status: proposed
priority: medium
created: 2026-07-08
updated: 2026-07-08
depends_on: [46]
related: [45, 16, 15]
adrs: [15, 16]
spec: docs/superpowers/specs/2026-07-08-cursor-dispatch-rule-generation-design.md
plan:
results:
trivial: false
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
| Spec | [2026-07-08-cursor-dispatch-rule-generation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-08-cursor-dispatch-rule-generation-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0016](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0016-harness-first-agent-config.md) |
<!-- docket:artifacts:end -->

## Why

Cursor has a quirk that defeats docket's agent layer: when a skill is invoked directly in
Cursor's agent chat, Cursor does not dispatch to the skill's bound subagent — it runs the skill
inline at whatever model is currently selected. The model/effort-pinned wrappers (0016/0045/0046)
therefore run at an arbitrary model on Cursor, which is exactly what they exist to prevent.

The proven workaround is a Cursor rule file (`.cursor/rules/docket-dispatch.mdc`, `alwaysApply: true`)
that intercepts the request and forces a Task dispatch to the matching `subagent_type`. Today these
rules are hand-authored per Cursor repo. docket already generates the Cursor agents (via
`sync-agents.sh`); it should generate the dispatch rule alongside them, at both the user-level
(`~/.cursor/rules/`) and per-repo (`<repo>/.cursor/rules/`) layers.

A dispatch rule is only correct if every `subagent_type` it names has a matching agent file in the
same layer, or Cursor silently falls back to inline. But per-repo generation is listed-only (it
writes only the agents keyed in the `.docket.yml` `agents:` block), while the block was only ever
meant to carry model/effort overrides — not to decide which agents exist. The agents compose
(implement-next → status/adr; finalize → rebase-resolver/integration-repair; auto-groom → its
critic), so a harness needs all of them. Conflating "listed in config" with "gets generated" is the
real friction, and it makes the dispatch rule's targets unreliable.

## What changes

Three coherent pieces in `sync-agents.sh`, layered on 0046 (full design in the linked spec):

- **Always-full-set generation** — flip the per-repo pass to iterate the full built-in agent set
  (mirroring the user-level pass), resolving each agent's model per harness through the existing
  fallback chain. The `agents:` block becomes override-only; an unconfigured agent still generates
  at its built-in default. Every targeted harness gets every agent in every layer, so dispatch
  targets resolve by construction.
- **Cursor dispatch rule** — authored source in a new `cursor-rules/` dir (a static `dispatch.head.md`
  preamble + one `dispatch/docket-<name>.md` fragment per agent); the generator assembles
  `docket-dispatch.mdc` as head + the fragments of the agents that exist, written user-level
  (`~/.cursor/rules/`) and per-repo committed (`<repo>/.cursor/rules/`, when `cursor ∈ agent_harnesses`),
  and joins the `--check` drift gate.
- **Prune step** — make the generator idempotent under removal: after regeneration, delete orphaned
  docket-owned files (a built-in agent docket no longer ships; a harness de-listed from
  `agent_harnesses`), scoped strictly to `docket-*` names.

## Out of scope

- A rules mechanism for non-Cursor harnesses (only Cursor has the quirk).
- Validating model IDs against a harness roster (docket stays passthrough, ADR-0015).
- A single dense dispatch-table layout (rejected in favor of per-agent subsections).
- Migrating pre-existing hand-authored per-agent `.mdc` rule files.
- Folding this into 0046 (it stays "harness-first resolution"; this is layered on it).

## Open questions

- ADR shape: a new ADR vs a dated `## Update` on ADR-0015/0016 — decided at the build's ADR step.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
