# Design — Pluggable workflow skills (superpowers optional)

Change: #0049 · slug `pluggable-workflow-skills` · spec drafted 2026-07-08
Standalone (no build-blocking deps). Related: [44, 16].

## Problem

docket's workflow quality comes from five superpowers skill invocations hard-coded into the skill
bodies:

| Role | Skill (hard-coded today) | Invoked by |
|---|---|---|
| brainstorm | `superpowers:brainstorming` | `docket-new-change` step 2, `docket-groom-next` |
| plan | `superpowers:writing-plans` | `docket-implement-next` step 4 |
| build | `superpowers:subagent-driven-development` | `docket-implement-next` step 5 |
| review | `superpowers:requesting-code-review` | `docket-implement-next` step 6 |
| finish | `superpowers:finishing-a-development-branch` | `docket-implement-next` step 7; `docket-finalize-change`'s optional non-standard close-out |

(Every other superpowers mention in the skill bodies is a prose reference — "do NOT continue to
`writing-plans`", auto-groom's "do NOT invoke `brainstorming`", status borrowing the worktree
provenance guard — not an invocation; those are untouched.)

A human who does not want or does not have the superpowers plugin cannot use docket: the
invocations fail, and there is no way to substitute a different skill or no skill at all. SDD in
particular (a fresh implementer subagent per task + per-task review) is heavyweight machinery some
users don't need. The motivating case is SDD, but the same hard-coding applies to all five roles.

## Decision

Add a **role-keyed `skills:` map** to `.docket.yml`. Each key names one of the five pluggable
invocation points; each value is either a **skill name passed verbatim to the Skill tool** or the
sentinel **`auto`** (no skill — the running agent performs the step inline, at its own model).
Unset keys default to today's superpowers skills, so absent config is byte-identical to current
behavior.

### Config shape

```yaml
# .docket.yml — all keys optional; unset = the superpowers default shown
skills:
  brainstorm: superpowers:brainstorming
  plan:       superpowers:writing-plans
  build:      superpowers:subagent-driven-development
  review:     superpowers:requesting-code-review
  finish:     superpowers:finishing-a-development-branch
```

- A value is a **verbatim skill name** — docket never validates it against a registry (the same
  unvalidated-passthrough philosophy as ADR-0015's harness-neutral model IDs; the passthrough is
  exactly what lets any third-party or in-repo skill plug in).
- **`auto`** is the reserved sentinel: no skill is invoked; the running agent does the step itself
  at whatever model it is already running at (the wrapper-resolved model for subagent runs, the
  session model for inline runs). No new model plumbing — "fall back on the selected model" is
  inherent in not dispatching.
- Unknown role keys are **warned-and-ignored** (same posture as `board_surfaces` tokens — a typo
  must never abort a run).
- Resolution is deterministic via `docket-config.sh --export`, which gains five new emitted
  variables (`SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH`),
  each defaulted when the key is unset. Consuming skills read the variables; none re-parse YAML.

### Auto fallback — final artifact only

When a role resolves to `auto`, the invoking skill performs the step inline. The fallback clause in
each skill body defines **only the final artifact / stop-point** — never the method (no mandated
TDD, dialogue shape, question cadence, or commit granularity; the agent chooses how):

| Role | Artifact / stop-point |
|---|---|
| brainstorm | a spec file at the configured spec path (`docs/superpowers/specs/…` on `metadata_branch`); stop at the spec |
| plan | a plan file on the feature branch, recorded in `plan:` |
| build | the plan executed on the feature branch |
| review | a whole-branch review before the PR opens |
| finish | a pushed feature branch with an open PR — never merged; stop |

Existing gates elsewhere are untouched and still hold regardless of build method — in particular
`docket-finalize-change`'s rebase-retest merge gate (`finalize.gate`) still validates the suite
before any merge.

### Missing-skill rule — degrade to auto + warn

If the resolved skill cannot be invoked at runtime (superpowers not installed, plugin
unavailable, typo'd custom name), the invoking skill **degrades to that role's `auto` fallback and
warns prominently** — in the run output, and (for build-time roles) as a note in the PR body. This
is deliberate: a repo with no superpowers plugin works **out of the box with zero config**, which
is the point of the change. The warning is the typo backstop. This is a softer posture than the
autonomous abort-and-report rule, chosen because skill availability is a per-machine property, not
a repo-state error — aborting would make docket unusable exactly for the users this change serves.

### Touched surfaces

- `scripts/docket-config.sh` + `scripts/docket-config.md` — parse `skills:`, emit the five
  `SKILL_*` variables with defaults.
- `skills/docket-convention/SKILL.md` — new "Skill layer" section (the table above + sentinel +
  missing-skill rule) and the `.docket.yml` example block gains `skills:`.
- `skills/docket-new-change/SKILL.md` — step 2 invokes the resolved brainstorm skill; auto
  fallback clause.
- `skills/docket-groom-next/SKILL.md` — same for its brainstorm.
- `skills/docket-implement-next/SKILL.md` — steps 4–7 invoke the resolved plan/build/review/finish
  skills; per-role auto fallback clauses.
- `skills/docket-finalize-change/SKILL.md` — the "Where finishing-a-development-branch fits"
  section references the resolved finish skill.
- Tests for the config parsing/emission (matching existing `docket-config.sh` test coverage).

### Relationship to #0044 (configurable SDD build models)

Separate and cross-linked. #0044's `build.implementer` / `build.reviewer` model IDs parameterize
**SDD's internal dispatches**; they apply **only when `skills.build` resolves to SDD**. When
`skills.build` is `auto` or a non-SDD skill, #0044's knobs are inert (a warning if set is #0044's
call, not this change's). #0044 remains its own change; this spec only records the guard.

## Alternatives considered

- **SDD-only opt-out** (a single `build.strategy: sdd | inline` knob) — rejected: the same
  hard-coding problem exists at all five invocation points, and the role-keyed map costs little
  more while covering brainstorm/plan/review/finish too.
- **Per-skill + step keys** (`skills.implement-next.build`, `skills.new-change.brainstorm`) —
  rejected: deeper schema, duplicated values for shared roles; no identified need for
  per-invocation-site divergence.
- **Prescriptive auto fallbacks** (mandated TDD, per-task commits, one-question-at-a-time) —
  rejected by the owner: only the artifact contract matters; method is the executing agent's
  choice at its own model.
- **Abort-and-report on missing skill** — rejected: would require a no-superpowers user to set all
  five keys to `auto` before docket runs at all, defeating the change's purpose.

## Open questions (resolve at build)

- Exact availability probe for "the resolved skill cannot be invoked" (Skill-tool listing vs
  attempt-and-catch) — decide against harness behavior at build time.
- Whether `sync-agents.sh`-generated wrapper directives need any wording change (they currently
  name no superpowers skills directly — expected no-op; verify).
- Likely one ADR: skill-name passthrough + degrade-to-auto posture (mirrors ADR-0015).
