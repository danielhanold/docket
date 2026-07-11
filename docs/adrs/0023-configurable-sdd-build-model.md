---
id: 23
slug: configurable-sdd-build-model
title: Configurable SDD build models — a `build:` surface of per-role direct model IDs
status: Accepted
date: 2026-07-11
supersedes: []
reverses: []
relates_to: [15, 16, 18]
change: 44
---

## Context

`docket-implement-next` builds each change through SDD
(`superpowers:subagent-driven-development`), which dispatches a per-task implementer, a
per-task reviewer, fix subagents, and a final whole-branch code-reviewer. **This build phase
is where the overwhelming majority of a build's tokens are spent** — and every one of those
dispatch models was chosen *only* by SDD's prose "Model Selection" controller judgment. The
single biggest cost lever in the system was unconfigurable: a repo could not express a policy
as basic as "build implementers on the cheap model, reviewers on the strong one."

This bites hardest on a **non-Claude / mixed model roster** — the motivating case is running
docket through **Cursor** — where the operator knows which of *their* models fits each role
better than SDD's Claude-shaped heuristic does. Change 0016 (the agent layer) explicitly
scoped build-dispatch model config out; change 0044 closes it.

The earlier approach — a tier vocabulary mapping `critical`/`standard`/`economy` to concrete
models (proposed change 0043) — was **killed** as Claude-lineage-specific by ADR-[[0015]].
`build:` deliberately takes **direct model IDs** instead, matching the harness-neutral,
unvalidated passthrough of the `agents:` block (ADR-[[0015]], ADR-[[0016]]).

## Decision

Add a top-level **`build:`** config surface with two per-role **model IDs**:

- **`build.implementer`** governs the SDD per-task implementer subagent **and** the fix
  subagents.
- **`build.reviewer`** governs the SDD per-task reviewer subagent **and** the Step-6 final
  whole-branch code-review dispatch.

Values are **direct model IDs, passed straight through** to each dispatch's `model:` field —
whatever the running harness honors (a Claude alias/ID under Claude Code; a Cursor model ID
under Cursor). docket neither interprets nor validates the string — the **same passthrough
contract as the `agents:` block** (ADR-[[0015]]), with **no tier indirection**.

When `build:` is absent, or a role is unset, that dispatch **keeps SDD's own per-complexity
Model Selection judgment**. The change is therefore **purely additive and
backward-compatible** — an absent `build:` is byte-identical to pre-0044 behavior.

`build:` is resolved by `docket-config.sh` (layered **local > repo-committed > global**;
**global-able** — a per-machine model preference in the same class as `skills:`/`agents:`,
**not** a coordination key, so not fenced in the machine-scoped layers) and applied by
`docket-implement-next` at the SDD hand-off. It fills SDD's already-required `model:` field —
**no new script, and no fork of SDD**.

## Consequences

- **Enables an explicit cost/quality policy per build role**, harness-neutral — the biggest
  token lever in the system becomes a two-line config knob.
- A **set role is a deliberate BLUNT override**: it disables SDD's per-complexity adaptivity
  for that role, trading it for a predictable, uniform model. This is **intended, not a
  regression** — an unset role restores SDD's judgment.
- The whole surface **assumes the running harness honors the `model:` field** on SDD's
  sub-dispatches the same way it honors it on docket's `agents:` wrappers. That is a
  **per-harness runtime verification — the same one that gates the `agents:` block** (ADR-[[0015]]'s
  silent-failure surface: some harnesses ignore an unrecognized `model:` and run their house
  default) — and is **NOT hermetically testable**.
- **Out of scope:** per-task / per-complexity buckets (mechanical / integration /
  architecture) — a possible future refinement; and the reconcile / plan / escalation model of
  `docket-implement-next` itself, which stays the implement-next wrapper's own model (change
  0042). `build:` governs only the SDD sub-dispatches.
