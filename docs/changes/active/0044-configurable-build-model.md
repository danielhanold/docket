---
id: 44
slug: configurable-build-model
title: Configurable SDD build models for docket-implement-next
status: in-progress
priority: low
created: 2026-07-07
updated: 2026-07-11
depends_on: []
related: [16, 42]
adrs: [23]
spec: docs/superpowers/specs/2026-07-07-configurable-build-model-design.md
plan: docs/superpowers/plans/2026-07-11-configurable-build-model.md
results:
trivial: false
auto_groomable:
branch: feat/configurable-build-model
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-07-configurable-build-model-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-07-configurable-build-model-design.md) |
| Plan | [2026-07-11-configurable-build-model.md](https://github.com/danielhanold/docket/blob/feat/configurable-build-model/docs/superpowers/plans/2026-07-11-configurable-build-model.md) |
| ADRs | [ADR-0023](https://github.com/danielhanold/docket/blob/docket/docs/adrs) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` builds each change through `superpowers:subagent-driven-development` (SDD),
which dispatches an implementer per plan task, a task-reviewer after each, fix subagents, and a
final whole-branch code-reviewer — where the vast majority of a build's tokens are spent. SDD picks
each dispatch's model by **controller judgment** (a prose "Model Selection" heuristic); there is
**no config knob**. So a repo cannot express a policy like "build implementers on the cheap model,
reviewers on the strong one" — the single biggest cost lever in the system is unconfigurable. This
bites hardest on a **non-Claude / mixed roster** (e.g. running docket through Cursor), where the
operator knows which of *their* models fits each role better than SDD's Claude-shaped heuristic
does. #0016 explicitly scoped this out; this change closes it.

## What changes

Add a **`build:` config surface** with two per-role **model IDs**, plus a behavioral rule in
`docket-implement-next` (full detail in the linked spec):

- **`build.implementer`** governs the per-task implementer **and** fix subagents; **`build.reviewer`**
  governs the task-reviewer **and** the final code-reviewer. Values are **direct model IDs** passed
  straight to SDD's `model:` field — whatever the running harness honors (a Claude alias/ID under
  Claude Code, a Cursor model ID under Cursor). No tier indirection.
- **`docket-implement-next`** resolves these at plan-execution time and fills SDD's already-required
  `model:` field from them; **unset → SDD's own Model Selection** (purely additive, backward-
  compatible). No new script, no fork of SDD.

A set role is a blunt, deliberate override of SDD's per-complexity adaptivity — trading it for a
predictable cost/quality policy. Likely warrants a small ADR — decided at build.

## Out of scope

- Redesigning or forking SDD — this only supplies the `model:` value SDD already requires.
- A tier abstraction over the model IDs — #0043 (tier indirection) was killed; `build:` takes
  direct model IDs, matching the `agents:` block.
- Per-task / per-complexity build-model config (mechanical/integration/architecture buckets) — a
  possible future refinement.
- The reconcile/plan/escalation model of implement-next itself — that stays the `implement-next`
  wrapper's own model (#0042); `build:` governs only the SDD sub-dispatches.

## Open questions

- Config placement — top-level `build:` vs nested under `agents:` (lean top-level).
- Whether the final code-reviewer folds into `build.reviewer` (this design) or gets its own role.
- Exact SDD override point — confirm against the SDD version in use at build.
- Whether the target harness (Cursor, in the motivating case) honors the `model:` field on SDD's
  subagent dispatches the way it honors it on docket's agent wrappers — verify at build.

## Reconcile log

- 2026-07-11 — Reconciled at claim. `depends_on: []`; related #16/#42 present; #0043 (tier
  indirection) confirmed killed, so `build:` takes direct model IDs (no tier layer). Verified the
  implementation surface: `scripts/docket-config.sh` line 80 **already anticipates** a top-level
  `build:` key ("a future top-level `build:`/`review:` could otherwise shadow…"); the `skills:`
  block parser (`yaml_block_body` + `skill_role`, layered local > repo-committed > global) is the
  exact pattern `build:` mirrors — add `build_role` emitting `BUILD_IMPLEMENTER`/`BUILD_REVIEWER`
  (empty when unset). `build:` is **global-able** (a per-machine model preference, same class as
  `skills:`/`agents:` — not a coordination key, so NOT fenced in the machine-scoped layers).
  Confirmed SDD's prompt templates still carry `model: [MODEL — REQUIRED]` in BOTH
  `implementer-prompt.md` and `task-reviewer-prompt.md` (the spec's 2026-07-07 check holds), so the
  wiring fills SDD's already-required field — no SDD fork. Open questions resolved by the spec's
  leans: top-level `build:` (Q1), fold final-reviewer into `build.reviewer` (Q2), fill the template
  `model:` per dispatch (Q3); Q4 (does the target harness honor the dispatch model) is a
  build-time/live note — the same verification that gates the `agents:` block, not hermetically
  testable. Scope unchanged.
