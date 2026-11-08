---
id: 576
slug: 'run-the-brainstorm-builder-check-on-a-pinned-cheaper-model-a'
title: 'Run the brainstorm builder check on a pinned, cheaper model and effort'
status: 'proposed'
priority: 'high'
type: 'feat'
deliverable: 'pr'
created: '2026-10-22'
updated: '2026-11-08'
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

### Upstream status (researched 2026-11-08)

**History of the check.**

- v5.0.0 (2026-03-09) added a spec-document reviewer subagent (`spec-document-reviewer-prompt.md`) after the written design.
- v5.0.6 (2026-03-24) replaced it with an inline self-review checklist, with no subagent.
- v7.0.0 (2026-10-09, obra/superpowers PR #2463) brought a subagent back as the builder check (`builder-check-prompt.md`), replacing the spec-document reviewer.
- v7.0.1 (2026-10-21) runs the builder check **only for project-sized work**. A small change gets a short design doc that goes straight to the human, without the check. This alone may remove most of the cost, if brainstorming sizes docket grooms as small changes often enough.
- v7.0.2 (2026-11-03) is the latest release. It retunes brainstorming's questions and leaves the builder check text unchanged, on both `main` and `dev`. This repository's sessions above ran 7.0.0.

**No way to pin it.** The prompt says only "dispatch a fresh general-purpose subagent". It names no model and no effort, and superpowers has no setting for either. This contradicts superpowers' own rule in subagent-driven development (since v6.0.0): every dispatch names its model, because "an omitted model silently inherits the session's most expensive one."

**Related upstream issues and PRs (obra/superpowers).**

- #2034 (open): subagent dispatches outside SDD have no model guidance, so they inherit the session model. It is this problem, filed against the old spec reviewer.
- PR #2400 (open draft, idle since 2026-10-22, "needs evals"): the fix for #2034. It adds a required `model:` placeholder to dispatch prompts, but it edits `spec-document-reviewer-prompt.md`, which 7.0.0 deleted, so as written it does not reach the builder check. Even merged, it would be prose the coordinator may ignore, and it covers model only, not effort.
- #1747 (open): SDD has no effort dimension. Its claim that effort cannot be set per dispatch is out of date: the Agent tool has taken `effort` since Claude Code 2.1.292.
- #2567 (open): the builder check has no fallback for a harness that allows spawning only when the user asks. It bears on the fallback question below.
- #512 (open): general token cost of brainstorming.

No issue targets the builder check's model or effort, and no open PR touches `builder-check-prompt.md`. Upstream has no announced plan to pin it, so the best upstream outcome is roughly option 2 below (a per-dispatch direction), not a deterministic per-harness pin.

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

## Open questions

- **Still needed after upgrading superpowers?** Upgrade from 7.0.0 to the latest release (7.0.2 at research time) and watch how often brainstorming sizes a docket groom as project-sized and runs the check. If v7.0.1's project-only gating removes enough of the cost, this change may shrink or be killed; groom it only after that observation.
- **Mechanism.** Which option: a docket-owned pinned agent (the likely shape), a per-dispatch direction, or something else? And how does the direction reach a dispatch the vendored skill makes, without patching it?
- **Which model and effort keep it useful.** What default per harness, and how is it proven? One way: re-run a few past builder checks, whose findings are recorded in their grooms, at the cheaper setting and compare what each catches.
- **Code reading.** Reading the code is what makes the check both useful and expensive (110 tool calls on #575). Should the brief bound or focus it, for example the files the spec names plus one whole-repo grep, rather than only lowering the tier?
- **Fallback.** What happens when the pinned agent is not installed, or dispatch fails: fall back to `general-purpose` at the session defaults, or skip with a warning? A builder check is not a gate, so it must never block a groom. Upstream #2567 covers a related case: the harness allows spawning only on the user's request.
- **Fit with #566.** If #566 also suppresses other superpowers steps, or moves the brainstorm role to a docket-owned skill, the pinned reviewer may belong there. Settle the two scopes together.
- **Other harnesses.** Do Cursor, Codex, and OpenCode brainstorms dispatch the check at all, and does each need a `harness-defaults.yml` row?
