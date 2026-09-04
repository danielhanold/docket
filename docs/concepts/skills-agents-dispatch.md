# Skills, agents, and harness dispatch

## The problem it solves

Docket's workflows — grooming, building, reviewing, finalizing — are long, and
each step wants a different amount of model horsepower and a different set of
instructions. Bake one giant prompt into one model call and you lose every kind
of flexibility that matters: no way to route a cheap step to a cheap model, no
way to reuse the same instructions across workflows, and no way to run the same
workflow on a different vendor's tool.

Docket splits the problem three ways. A **skill** — a named, reusable instruction
set an agent loads for one job — holds the instructions. An **agent** — a
separately launched worker with its own context, pinned to a model and effort —
is the worker that runs them. A **harness** — the tool that runs the agent:
Claude Code, Cursor, Codex, or opencode — is the vendor tool underneath. And
**dispatch** — launching a named agent to do a step and waiting for it to return
— is how one step hands work to a named agent and reads back its result.

Because the three are separate, one skill can run on any harness, at whatever
model and effort that harness pins for the agent, and a workflow can route each
step to the right worker without rewriting a line of the instructions.

## The moving parts

```
  layered config (repo → user → machine-local)
        │  generates, per harness
        ▼
  agent wrappers  ───────────────►  harness registry
  (name + model + effort + skill)    (one row per supported harness)
        │                                    │
        │  a workflow step dispatches …      │  … the harness launches the named agent
        ▼                                    ▼
   named agent  ── loads ──►  skill (the instruction set for one job)
        │
        └── returns its result on one channel: the dispatch return
```

Agent wrappers are generated from layered configuration and are machine-local:
regenerated per machine, never committed. Each wrapper names an agent, pins its
model and effort, and points at the skill it loads. The model and effort defaults
ship in a harness-indexed sidecar, so the wrapper template itself carries no model
floor to drift out of date.

A workflow step names the agent it wants and dispatches it. Whether that dispatch
capability actually exists is resolved from the machine's registry, never guessed
from a tool name; where it is unavailable the workflow degrades in tiers instead
of crashing. On the harness that supports it, an inline skill dispatch rides a
fork of the current context — the worker runs as a forked child rather than a
fresh launch — and that fork has two documented invocation paths rather than one
tool call.

Which skill a workflow uses is itself pluggable: a skill name is passed through
unvalidated, and a missing skill degrades to the built-in default instead of
aborting the run. Where autonomy matters — whether a step may run unattended — the
precedence is pinned at the call site, not left for the dispatched agent to infer.

## The invariants

- Skills, agents, and harnesses are independent: one skill runs unchanged across
  every supported harness.
- Agent wrappers are generated from layered config and are machine-local —
  regenerated per machine, never committed.
- An agent's model and effort come from a harness-indexed defaults sidecar; the
  wrapper template carries no model floor of its own.
- Dispatch capability is resolved from the machine's registry, never inferred
  from a tool name, and unavailability degrades in tiers.
- A named skill is passed through unvalidated, and a missing one degrades to the
  built-in default rather than aborting the workflow.
- Autonomy precedence is fixed by pre-specification at the call site, not decided
  by the dispatched agent.
- A generated wrapper conforms to its target harness's own documented contract.

## Decided in

- [ADR-0008](../adrs/0008-agent-layer-generated-subagents.md) — established the
  agent layer as generated subagent wrappers built from layered config.
- [ADR-0015](../adrs/0015-harness-portable-agent-config.md) — made agent model
  config harness-portable with direct model IDs generated per repo to an explicit
  harness list.
- [ADR-0016](../adrs/0016-harness-first-agent-config.md) — organized the `agents:`
  config harness-first, with per-harness model and effort and field-level default
  fallback.
- [ADR-0018](../adrs/0018-pluggable-skills-passthrough-degrade.md) — made workflow
  skills pluggable with unvalidated name passthrough and degrade-to-auto on a
  missing skill.
- [ADR-0024](../adrs/0024-claude-context-fork-skill-dispatch.md) — chose
  context-fork frontmatter as one harness's inline-skill dispatch mechanism,
  forking only human-non-interactive skills.
- [ADR-0026](../adrs/0026-fork-dispatch-opacity-two-invocation-paths.md) —
  accepted fork-dispatch opacity and documented its two invocation paths rather
  than adding tooling.
- [ADR-0044](../adrs/0044-autonomy-precedence-call-site-pre-specification.md) —
  fixed autonomy precedence by pre-specification at the call site.
- [ADR-0059](../adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md)
  — made dispatch capability resolved rather than inferred from a tool name, with
  tiered unavailability.
- [ADR-0060](../adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md)
  — required a generated wrapper to conform to its target harness's own documented
  contract.
- [ADR-0064](../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md)
  — moved shipped agent model and effort defaults into a harness-indexed sidecar
  so wrapper templates carry no model floor.
