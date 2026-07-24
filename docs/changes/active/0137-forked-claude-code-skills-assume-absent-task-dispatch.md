---
id: 137
slug: forked-claude-code-skills-assume-absent-task-dispatch
title: "Forked Claude Code docket skills assume a Task subagent-dispatch tool the fork does not have, silently degrading SDD build and review"
status: proposed
priority: critical
type: fix
created: 2026-07-24
updated: 2026-07-24
depends_on: []
related: [16, 17, 49, 61, 113, 135]
discovered_from: [136]
adrs: [8, 17, 24]
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
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0017](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0017-cursor-dispatch-rule-full-agent-set.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

A live `docket-implement-next 136` run — invoked as a Claude Code forked subagent via the
`context: fork` + `agent:` dispatch of ADR-[[0024]] — reported that its runtime had **no
subagent-dispatch (`Task`) tool**. Because of that, the resolved `build`
(`superpowers:subagent-driven-development`) could not dispatch its fresh per-task implementer
subagents, and the resolved `review` (`superpowers:requesting-code-review`) could not dispatch a
reviewer. Both roles fell back to their inline `auto` fallbacks. The run still produced a plausible
PR (#124), but it did **not** execute SDD's fresh per-task implementers, its per-task TDD gates, or
a dispatched code review — exactly the disciplines docket's wrapper advertised as running.

This is the **Claude Code instance** of the defect class that change [[135]] records for **Cursor**:
a generated/forked wrapper advertises workflow discipline that the harness path docket itself chose
cannot actually execute, and a successful-looking artifact conceals it. Both are the
`skill-fallback-degrades-discipline` learning (change 0066) biting in a real build.

The structural cause is that ADR-[[0024]] closed the fork-exclusion question only for the **human**
channel — a forked subagent is denied `AskUserQuestion` / `EnterPlanMode`, so only
human-non-interactive skills are forked. It never addressed that a fork is **also denied the `Task`
subagent-dispatch tool**. Yet three of the four forked autonomous skills depend on re-dispatch:

- `docket-implement-next` dispatches the `docket-status` subagent (§0) and the `docket-adr`
  subagent (§6) **foreground** (docket-convention *Composition*), and its `build`/`review` roles
  dispatch nested Superpowers subagents.
- `docket-auto-groom` dispatches the `docket-auto-groom-critic` subagent for its adversarial gate.

If a forked child truly has no `Task` tool, none of these dispatches can happen — the composition
model and the SDD/review disciplines are **structurally unreachable** on the very
skill-invoke path (`/docket-implement-next`) that ADR-[[0024]] names as first-class. The
Skill-layer *missing-skill rule* (degrade to `auto` + warn) then fires **every time** rather than
as a rare per-machine fallback, so the warning is honest but the discipline never runs. The change-136
run surfaced this in its report and PR body rather than hiding it — the honest-degradation posture
worked — but the capability gap it revealed is the bug.

## What changes

Establish whether, and how, a forked (or agent-dispatched) Claude Code docket skill can reach a
subagent-dispatch tool, and make docket's advertised composition + workflow disciplines either
genuinely reachable or honestly unavailable on Claude Code. Design lives in the brainstorm/spec;
at scope altitude this covers:

- Determine empirically what tools a Claude Code forked subagent actually has — specifically whether
  `Task` (or any nested-dispatch mechanism) is present — and whether the **agent-dispatch** path
  (`@docket-implement-next`, ADR-0024's second first-class path) differs from the **skill-invoke**
  (forked) path on this point.
- Decide the correct execution model for the three dispatch-dependent forked skills when no nested
  `Task` is available: run the composition/build/review inline in a defined, auditable way; route
  through a path that *does* grant dispatch; or halt rather than silently degrade.
- Extend ADR-[[0024]]'s fork-exclusion reasoning (or record a new decision) to cover the
  `Task`/dispatch channel, so the 4-forked/3-not split is justified against *re-dispatch* capability,
  not only the human channel.
- Define an honest, auditable failure posture for an autonomous Claude Code build whose configured
  `build`/`review` discipline cannot dispatch: the current warn-and-inline is honest but always-on;
  decide whether that is acceptable or whether such a run must halt (mirrors change [[135]]'s
  identical open question for Cursor).
- Add a Claude Code runtime check (structural test and/or smoke test) proving the configured
  discipline is actually reachable on the fork/dispatch path docket uses, rather than only asserting
  the wrapper's generated frontmatter.

## Out of scope

- The **Cursor** instance of this defect — owned by change [[135]]; this change is the Claude Code
  counterpart, and the two should share a consistent honest-failure posture without merging.
- Changing the Superpowers SDD / TDD / code-review skills themselves.
- Reworking the agent-layer wrapper generation (`sync-agents.sh`) beyond what a Claude Code fix
  requires.
- Retrofitting the already-open PR #124 from the change-136 run.

## Open questions

- Does a Claude Code forked subagent have **any** nested-dispatch tool, or is re-dispatch
  categorically unavailable inside a fork? Does the agent-dispatch path behave differently?
- If re-dispatch is unavailable in a fork, must `docket-implement-next` / `docket-auto-groom` run
  their composition (`docket-status`, `docket-adr`, `docket-auto-groom-critic`) **inline** — and is
  inline composition faithful to the foreground-blocking, git-state contract of docket-convention's
  *Composition* paragraph?
- Should an autonomous Claude Code build whose configured `build`/`review` cannot dispatch **halt**,
  or is warn-and-inline the accepted posture (must match whatever change [[135]] settles for Cursor)?
- Is this a refinement of ADR-[[0024]] (append an `## Update`) or a new ADR about dispatch-tool
  availability across harness invocation paths?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
