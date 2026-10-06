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
Claude Code, Cursor, Codex, or OpenCode — is the vendor tool underneath. And
**dispatch** — launching a named agent to do a step and waiting for it to return
— is how one step hands work to a named agent and reads back its result.

Because the three are separate, one skill can run on any harness, at whatever
model and effort that harness pins for the agent, and a workflow can route each
step to the right worker without rewriting a line of the instructions.

## The moving parts

```
  built-in model/effort table (compiled into docket, indexed by harness)
        │
        ▼  overridden per field by
  global config: agents.<harness>.<agent>.model / effort
        │
        │  docket install writes, per harness, into your user directory
        ▼
  agent wrappers  ───────────────►  harness registry
  (name + model + effort + skill)    (one row per supported harness)
        │                                    │
        │  a workflow step dispatches …      │  … the harness launches the named agent
        ▼                                    ▼
   named agent  ── loads ──►  skill (the instruction set for one job)
        │
        ├── most results come back as the dispatch return
        └── ADR, status, and plan-writer results land as git state
```

Agent wrappers are user-level files that `docket install` writes for each harness
it installs into — `~/.claude/agents/`, `~/.codex/agents/`, `~/.cursor/agents/`,
and `${XDG_CONFIG_HOME:-~/.config}/opencode/agents/` — 17 agents per harness. No
repository holds wrapper files. Each wrapper names an agent, pins its model and
effort, and names the skill it preloads in its `skills` frontmatter field. The
model and effort come from a built-in table compiled into the binary, indexed by
harness (`agents/harness-defaults.yml` is its shipped mirror), overridden field
by field from your global config. A repository's `.docket.yml` cannot change an
agent's model or effort.

Docket's workflow steps use fixed default roles: superpowers for brainstorm,
plan, and finish (`superpowers:brainstorming`, `superpowers:writing-plans`,
`superpowers:finishing-a-development-branch`), and docket's own skills for build
and review (`docket-build`, `docket-review`). The roles are fixed.

A workflow step names the agent it wants and dispatches it. Most dispatches hand
their result back in the dispatch return. Three do not: the ADR, status, and
plan-writer agents record their result in git — an ADR, a board refresh, or a
plan on the `docket` branch — and the caller reads it from there.

Whether a dispatch capability actually exists is resolved on the machine,
by trying it, never guessed from a tool name. Where dispatch is genuinely
unavailable, the workflow falls back by kind instead of crashing: the status and
ADR dispatches run the same work inline, since their result is git state either
way; the auto-groom critic abstains, because a draft cannot critique itself; and
the plan-writer, build, and review dispatches stop the run for a human. The
finalize rebase resolver and integration repair have no fallback: finalize stops
with the pull request still open. On the harness that supports it, an inline
skill invocation rides a fork of the current context — the worker runs as a
forked child rather than a fresh launch — and that fork has two documented
invocation paths rather than one tool call.

Where autonomy matters — whether a step may run unattended — the precedence is
pinned at the call site, not left for the dispatched agent to infer.

## The invariants

- Skills, agents, and harnesses are independent: one skill runs unchanged across
  every supported harness.
- Agent wrappers are user-level and machine-local — written by `docket install`
  per machine, never committed to a repository.
- An agent's model and effort come from the built-in harness-indexed table,
  overridden only from the global config; the wrapper template carries no model
  floor of its own.
- Workflow roles are fixed defaults, the same in every repository.
- Dispatch capability is resolved on the machine, never inferred from a tool
  name, and unavailability falls back by kind.
- Autonomy precedence is fixed by pre-specification at the call site, not decided
  by the dispatched agent.
- A generated wrapper conforms to its target harness's own documented contract.

## Decided in

- [ADR-0008](../adrs/0008-agent-layer-generated-subagents.md) — established the
  agent layer: thin generated wrappers that pin model and effort and preload
  the skill, which stays the single source of instructions.
- [ADR-0015](../adrs/0015-harness-portable-agent-config.md) — made agent model
  values direct model IDs, passed to the harness verbatim with no tier layer.
- [ADR-0016](../adrs/0016-harness-first-agent-config.md) — organized agent
  configuration harness-first, with per-harness model and effort and field-level
  default fallback.
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
  per-kind fallbacks.
- [ADR-0060](../adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md)
  — required a generated wrapper to conform to its target harness's own documented
  contract.
- [ADR-0064](../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md)
  — kept the shipped agent model and effort defaults in a harness-indexed table,
  so wrapper templates carry no model floor.
