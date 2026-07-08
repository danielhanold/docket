# Design — Configurable SDD build models for docket-implement-next

Change: #0044 · slug `configurable-build-model` · spec drafted 2026-07-07, reshaped 2026-07-08 (owner)
Standalone (no build-blocking deps). Related: [16, 42].

> **Reshape note (2026-07-08).** The original cut expressed the two build roles as **tier names**
> resolved through #0043's tier map. #0043 (tier indirection) was **killed**: the tier layer was
> Claude-lineage-specific and worked against the goal that actually surfaced — pointing docket's
> subagents at an arbitrary, harness-provided model roster (the motivating case: running docket
> through Cursor at work, where the available models are not Claude's). This reshape drops the tier
> dependency and makes the build roles take **direct model IDs**, the same harness-neutral
> passthrough the `.docket.yml` `agents:` block already uses.

## Problem

`docket-implement-next` builds each change by running `superpowers:subagent-driven-development`
(SDD), which dispatches a fresh **implementer** subagent per plan task, a **task-reviewer** after
each, **fix** subagents for findings, and a final whole-branch **code-reviewer**. This is where the
overwhelming majority of a build's tokens are spent.

SDD chooses each dispatch's model by **controller judgment** — its "Model Selection" section is
prose ("mechanical → cheap, integration → standard, architecture → most capable") and the
controller (which is `docket-implement-next`) fills the required `model:` field in
`implementer-prompt.md` / `task-reviewer-prompt.md` at dispatch time. There is **no config knob**.
So a docket repo cannot express a policy like "build implementers run on the cheap model, reviewers
on the strong one" — the biggest cost lever in the whole system is unconfigurable. This bites
hardest on a **non-Claude / mixed model roster** (e.g. through Cursor), where the operator — not
SDD's Claude-shaped heuristic — knows which of *their* models fits each role.

#0016 explicitly scoped this out ("the TDD build's model … is `subagent-driven-development`'s own
config"). This change closes that gap.

## Decision

Add a **`build:` config surface** with **two per-role model IDs**, and a behavioral rule in
`docket-implement-next` that applies them to SDD's dispatches. When unset, behavior is exactly
today's — SDD's own Model Selection judgment — so this is purely additive and backward-compatible.

### Config shape

```yaml
# .docket.yml (per-repo) or ~/.config/docket/agents.yaml (global)
build:
  implementer: <model-id>   # the per-task TDD work + fix subagents (the biggest spend)
  reviewer:    <model-id>   # task-reviewer after each task, and the final code-reviewer
```

- Values are **model IDs passed straight through** to SDD's `model:` field — whatever the running
  harness honors. Under Claude Code that is a Claude alias or full ID (`sonnet`, `claude-opus-4-8`);
  under Cursor it is one of Cursor's model IDs. docket does not interpret or validate the string
  (same passthrough contract as the `agents:` block), so the surface is harness-neutral.
- **Two roles, mapped to SDD's four dispatch kinds:**
  - `build.implementer` → the per-task implementer **and** the fix subagents (both do
    implementation work).
  - `build.reviewer` → the per-task reviewer **and** the final whole-branch code-reviewer.
- **Unset role → defer to SDD.** If `build:` is absent, or a role is unset, that dispatch keeps
  SDD's own per-complexity judgment. Configuring a role is an explicit override of that judgment
  (blunt by design: a set role pins every dispatch of that kind, trading SDD's adaptivity for a
  predictable cost/quality policy).

### Wiring in `docket-implement-next`

At the step where implement-next hands the plan to SDD (SKILL.md step 5, "executes the plan
task-by-task"), add a rule: resolve `build.implementer` / `build.reviewer` from config
(`docket-config.sh --export`), and when set, use those model IDs as the `model:` for the
corresponding SDD dispatches; when unset, instruct nothing (SDD judges). No new script and no fork
of SDD — the controller simply fills SDD's already-required `model:` field from docket config
instead of from its own heuristic. This depends on SDD keeping its prompt-template dispatch shape
(verified 2026-07-07: `implementer-prompt.md` / `task-reviewer-prompt.md` both take
`model: [REQUIRED]`).

## What the implementer edits

- **`scripts/docket-config.sh`** (+ `docket-config.md`) — parse and expose the `build:` block
  (two roles) as literal model-ID strings. Emit them in `--export` so the skill reads them in one
  turn.
- **`skills/docket-implement-next/SKILL.md`** — the build-dispatch rule above: resolve build roles,
  apply to SDD implementer/fix and reviewer/final-review dispatches, else defer to SDD. State the
  unset-→-SDD default explicitly.
- **`docket-convention`** — document `build:` in the config schema (the `.docket.yml` block) and in
  the Agent-layer/composition prose, noting it takes direct model IDs and defers to SDD when unset.
- **Tests** — config parsing/resolution of `build.implementer`/`build.reviewer` (including that an
  arbitrary non-Claude ID passes through unchanged; unset → no override); a skill-level assertion
  that the build-dispatch rule names the resolved models. (The actual SDD dispatch is not
  unit-testable in the hermetic suite — assert the resolution + the documented rule, and verify
  live behavior at build time, per the repo's metadata-branch-artifact testing convention.)

## Open questions (resolve at build)

1. **Config placement** — top-level `build:` (as above) vs nesting under `agents:`. Lean top-level:
   the build roles are not docket agent wrappers, so a sibling block reads clearer.
2. **Reviewer split** — fold the final code-reviewer into `build.reviewer` (this design) vs give it
   its own `build.final-reviewer`. Lean: fold (the two-role cut); revisit if a repo wants the final
   review on a different model.
3. **SDD override point** — exactly how the controller injects the model (fill the template field
   per dispatch vs a one-time instruction to SDD at hand-off). Confirm against the SDD version in
   use at build; keep the override where SDD reads `model:`.
4. **Does the target harness honor the dispatch model?** — the whole surface assumes the harness
   running the build (Cursor, in the motivating case) honors the `model:` field on SDD's subagent
   dispatches, exactly as it honors it on docket's own agent wrappers. Confirm this concretely for
   the target harness at build — the same verification that gates the direct per-agent `agents:`
   config.
5. **Interaction with SDD adaptivity** — a set role disables SDD's per-complexity tiering for that
   role. Documented as intended; no attempt to preserve adaptivity under an override in this cut.

## Non-goals

- Re-designing SDD or forking its prompts — this only supplies the `model:` value SDD already
  requires.
- Re-introducing a tier abstraction — #0043 was killed; the build roles take direct model IDs,
  matching the `agents:` block. If a repo later wants named model groups, that is a separate
  proposal.
- Per-task or per-complexity build-model config (mechanical/integration/architecture buckets) —
  a possible future refinement; out of scope for this first cut.
- The reconcile/plan/escalation model of implement-next itself — that stays the `implement-next`
  agent wrapper's own model (#0042 pinned the built-in default; the `agents:` block overrides it).
  `build:` governs only the SDD sub-dispatches.

## ADR

Likely warranted (small): "the SDD build model is docket-configurable via `build:` roles that take
direct model IDs and default to SDD's own selection when unset." Decide at build via `docket-adr`.
