# Design — Configurable TDD build model for docket-implement-next

Change: #0044 · slug `configurable-build-model` · spec drafted 2026-07-07 (interactive brainstorm, owner)
Depends on #0043 (`done`). Related: [16].

## Problem

`docket-implement-next` builds each change by running `superpowers:subagent-driven-development`
(SDD), which dispatches a fresh **implementer** subagent per plan task, a **task-reviewer** after
each, **fix** subagents for findings, and a final whole-branch **code-reviewer**. This is where the
overwhelming majority of a build's tokens are spent.

SDD chooses each dispatch's model by **controller judgment** — its "Model Selection" section is
prose ("mechanical → cheap, integration → standard, architecture → most capable") and the
controller (which is `docket-implement-next`) fills the required `model:` field in
`implementer-prompt.md` / `task-reviewer-prompt.md` at dispatch time. There is **no config knob**.
So a docket repo cannot express a policy like "build implementers run on Haiku 4.5, reviewers on
Sonnet 5" — the biggest cost lever in the whole system is unconfigurable.

#0016 explicitly scoped this out ("the TDD build's model … is `subagent-driven-development`'s own
config"). This change closes that gap, reusing #0043's tier map so the build model is expressed in
the same vocabulary as every other agent.

## Decision

Add a **`build:` config surface** with **two per-role tiers**, resolved through #0043's tier map,
and a behavioral rule in `docket-implement-next` that applies them to SDD's dispatches. When unset,
behavior is exactly today's — SDD's own Model Selection judgment — so this is purely additive and
backward-compatible.

### Config shape

```yaml
# .docket.yml (per-repo) or ~/.config/docket/agents.yaml (global)
build:
  implementer: economy    # the per-task TDD work + fix subagents (the biggest spend)
  reviewer:    standard    # task-reviewer after each task, and the final code-reviewer
```

- Values are **tier names** resolved via #0043's tier map (so `economy` → `claude-haiku-4-5-…`,
  etc.), keeping one model vocabulary. An explicit model ID is also accepted (same
  explicit-wins-over-tier rule as #0043's agent entries).
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
task-by-task"), add a rule: resolve `build.implementer` / `build.reviewer` from config (via the
#0043 resolver), and when set, use those concrete models as the `model:` for the corresponding SDD
dispatches; when unset, instruct nothing (SDD judges). No new script and no fork of SDD — the
controller simply fills SDD's already-required `model:` field from docket config instead of from
its own heuristic. This depends on SDD keeping its prompt-template dispatch shape (verified
2026-07-07: `implementer-prompt.md` / `task-reviewer-prompt.md` both take `model: [REQUIRED]`).

## What the implementer edits

- **`scripts/docket-config.sh`** (+ `docket-config.md`) — parse and expose the `build:` block
  (two roles), resolving tier names through the #0043 machinery. Emit them in `--export` so the
  skill reads them in one turn.
- **`skills/docket-implement-next/SKILL.md`** — the build-dispatch rule above: resolve build tiers,
  apply to SDD implementer/fix and reviewer/final-review dispatches, else defer to SDD. State the
  unset-→-SDD default explicitly.
- **`docket-convention`** — document `build:` in the config schema (the `.docket.yml` block) and in
  the Agent-layer/composition prose, noting it reuses the tier vocabulary and defers to SDD when
  unset.
- **Tests** — config parsing/resolution of `build.implementer`/`build.reviewer` (tier and explicit
  forms; unset → no override); a skill-level assertion that the build-dispatch rule names the
  resolved models. (The actual SDD dispatch is not unit-testable in the hermetic suite — assert the
  resolution + the documented rule, and verify live behavior at build time, per the repo's
  metadata-branch-artifact testing convention.)

## Open questions (resolve at build)

1. **Config placement** — top-level `build:` (as above) vs nesting under `agents:`. Lean top-level:
   the build roles are not docket agent wrappers, so a sibling block reads clearer.
2. **Reviewer split** — fold the final code-reviewer into `build.reviewer` (this design) vs give it
   its own `build.final-reviewer`. Lean: fold (the brainstorm chose the two-role cut); revisit if a
   repo wants the final review on a different tier.
3. **SDD override point** — exactly how the controller injects the model (fill the template field
   per dispatch vs a one-time instruction to SDD at hand-off). Confirm against the SDD version in
   use at build; keep the override where SDD reads `model:`.
4. **Interaction with SDD adaptivity** — a set role disables SDD's per-complexity tiering for that
   role. Documented as intended; no attempt to preserve adaptivity under an override in this cut.

## Non-goals

- Re-designing SDD or forking its prompts — this only supplies the `model:` value SDD already
  requires.
- Per-task or per-complexity build-model config (mechanical/integration/architecture buckets) —
  a possible future refinement; out of scope for this first cut.
- The reconcile/plan/escalation model of implement-next itself — that stays the `implement-next`
  agent wrapper's tier (#0042/#0043). `build:` governs only the SDD sub-dispatches.

## ADR

Likely warranted (small): "the TDD build model is docket-configurable via `build:` roles that reuse
the tier map and default to SDD's own selection when unset." Decide at build via `docket-adr`.
