---
id: 576
slug: 'run-the-brainstorm-builder-check-on-a-pinned-cheaper-model-a'
title: 'Run the brainstorm builder check on a pinned, cheaper model and effort'
status: 'proposed'
priority: 'high'
type: 'feat'
deliverable: 'pr'
created: '2026-10-22'
updated: '2026-10-22'
depends_on: []
stacked_on:
related: [566]
discovered_from: [575]
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

`superpowers:brainstorming` (7.0.0) ends every written design with a builder check: a fresh `general-purpose` subagent reads the spec as a would-be planner and reports the questions it would still need answered. `docket-groom-next` and `docket-new-change` run that skill for their brainstorm role, so every interactive groom pays for one.

It is extremely useful. In recent grooms it caught real gaps the design had missed: retry-after-contended semantics and the cross-clone lease on #360, the reviewer needing a worktree on #559, a sweep entry that dropped its findings on #562, terminal output hiding the refusal message on #568, and shared callers of the changed function on #574.

It is also far too expensive. It is dispatched with no model and no effort, so it runs on the parent session's model at the parent session's effort, and it reads the code at length. In this repository's saved sessions there are 21 builder checks (2026-10-16 to 2026-10-22). Every one ran on `claude-opus-5-5`, at a median of about 4.8M cache-read and 150K cache-write input tokens. The latest, on #575's spec, made 110 tool calls over 13.5 minutes and passed 17.8M cache-read tokens.

Docket pins the model and effort of every agent it dispatches itself: a per-harness row in `agents/harness-defaults.yml`, overridable in the global docket config, written into each agent's frontmatter by `docket install`. This reviewer is dispatched by the vendored skill instead, so no docket pin reaches it. Found while grooming #575.

## What changes

Run the builder check on a deterministic, cheaper model and effort, without dropping it and without patching the vendored plugin. These options were found while grooming #575 and checked against the Claude Code docs (v2.1.306); weigh them at grooming:

1. **A docket-owned pinned reviewer agent** (for example `docket-builder-check`), shaped like `docket-brainstorm-consultant` and `docket-plan-writer`. It gets a row per harness in `agents/harness-defaults.yml`, can be overridden through the global docket config (`agents.<harness>.<agent>.model|effort`), and is installed by `docket install`. `docket-groom-next` and `docket-new-change` direct the brainstorm to send its builder check to that agent, phrased by shape rather than by citing the vendored heading. This is deterministic and reuses docket's existing pinning; it is the likely shape, but treat it as a hypothesis.
2. **A per-dispatch `model`/`effort` stated in those skills' direction.** The Agent tool takes both parameters (`effort` since Claude Code 2.1.292). This is the cheapest to build, but it relies on the coordinator obeying the direction, and the model names are Claude Code's.
3. **A user-level `~/.claude/agents/<name>.md`** with `model:`/`effort:` frontmatter, outside docket. It is per-machine and not distributed.
4. **`CLAUDE_CODE_SUBAGENT_MODEL`** in the settings `env` block. It sets a default model for every subagent without its own; docket's pinned agents keep their frontmatter. It is blunt, and there is no matching default for subagent effort. `CLAUDE_CODE_EFFORT_LEVEL` and `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` override every agent, docket's pins included, so they are not options.
5. **A custom agent named `general-purpose`** that shadows the built-in. This is undocumented: the docs promise shadowing only for `Explore`.

## Out of scope

- Removing the builder check: #566 proposes stopping it, while this change keeps it and makes it cheaper. Whichever is groomed second must reconcile the two.
- Patching or forking the superpowers plugin.
- The already-pinned `docket-brainstorm-consultant` and `docket-auto-groom-critic`.
- Changing a person's own Claude Code settings: docs may mention options 3 and 4, but docket does not set them.
