---
id: 96
slug: suppress-plan-skill-execution-handoff
title: An autonomous run can be halted by a sub-skill's interactive hand-off — neutralize writing-plans' execution choice
status: proposed
priority: high
created: 2026-07-18
updated: 2026-07-18
depends_on: []
related: [16, 44, 49, 61, 95]
adrs: [24]
spec:
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
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` guarantees it "runs with **no human interaction**". On 2026-07-18, during
the build of change 0095, it broke that guarantee: the forked run stopped after planning and asked
the human which execution mode to use, ending the fork. Everything after — the whole six-task
build — ran in the parent session instead of the isolated subagent.

**The prompt is not docket's.** It is `superpowers:writing-plans` §"Execution Handoff"
(`SKILL.md:156-174`), which instructs: *"After saving the plan, offer execution choice… 1.
Subagent-Driven (recommended) / 2. Inline Execution… Which approach?"* — the text that surfaced,
verbatim. `docket-implement-next` §4 invokes that skill as the resolved `$SKILL_PLAN`, so its
closing instruction lands inside an autonomous run.

**Three live instructions collide.** The plan skill says *ask the human*. The agent wrapper says
*"never an interactive prompt"* (the abort-and-report rule every autonomous wrapper carries). The
skill body's §5 says the build method is already decided — *"the resolved build skill —
`$SKILL_BUILD` from the Step-0 config export"*. Two of the three say don't ask; the third says ask;
nothing states which wins. Resolution is left to model judgment, and it is not stable.

**Measured, not theorized.** Across every `docket-implement-next` fork on this machine — 39 runs
from 06-21 to 07-17 — none surfaced the question; each picked SDD and dispatched 8–31 subagents
from inside the fork. Run 40 complied with the plan skill instead. Machinery was verified intact
and is *not* the cause: `context: fork` frontmatter present, wrapper present, and the model pin
held exactly (`claude-opus-4-8`, effort `xhigh`, read from the fork's own log). It is a latent
ambiguity that had simply always landed the same way.

The cost is not correctness — 0095 completed and was fully reviewed — it is the autonomy contract
and context isolation, failing unpredictably at roughly 1-in-40 with no signal that it will.

## What changes

- **Neutralize the plan role's hand-off** in `docket-implement-next` §4: after `$SKILL_PLAN`
  returns, the build proceeds via `$SKILL_BUILD` with no prompt. Any execution-mode question the
  plan skill poses is answered internally from the already-resolved config, never surfaced.
- **Decide the general rule.** The same shape can occur wherever a resolved sub-skill ends with a
  human hand-off — the `plan`, `review`, and `finish` roles all invoke third-party skills from
  inside autonomous wrappers. Either state the precedence once (an autonomous wrapper's
  no-prompt rule outranks any invoked skill's interactive step) in `docket-convention`'s *Skill
  layer*, or patch each role. The convention already owns the missing-skill and `auto`-fallback
  rules for this layer, so it is the natural home.
- **Guard it.** A sentinel asserting the precedence survives in the skill/convention prose, so a
  future re-slim cannot quietly drop it.

## Out of scope

- **Editing the superpowers plugin.** `writing-plans` is vendored under
  `~/.claude/plugins/cache/.../superpowers/6.1.1/`; a local edit is overwritten on upgrade. The fix
  must live in docket's own prose.
- Changing which build skill is used, or the `skills.build` binding itself — `$SKILL_BUILD`
  already resolves correctly (#0049). This change makes the run honor it without asking.
- SDD's per-dispatch model selection (#0044, blocked) and the interactive skills
  (`docket-new-change`, `docket-groom-next`), which are *supposed* to prompt.

## Open questions

- **One-off or general?** Patch §4 alone, or state the precedence once in the *Skill layer* so
  every role inherits it? The general rule is more durable but touches the convention, which every
  skill loads — a wider blast radius for a 1-in-40 defect.
- **Is silent suppression right?** Answering internally hides that an invoked skill tried to stop.
  A one-line note in the run output ("plan skill offered an execution choice; resolved from
  `skills.build`") would leave a trace, at the cost of noise on every run.
- Do the `review` and `finish` roles' default skills contain a comparable interactive step, or is
  `writing-plans` the only one? A read of all four resolved defaults would size the problem.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
