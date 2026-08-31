---
id: 193
slug: default-build-review-roles-to-docket-owned-skills
title: Default the build and review roles to docket-build and docket-review
status: done
priority: medium
type: chore
created: 2026-08-02
updated: 2026-08-02
depends_on: []
related: [167, 170]
discovered_from: [170]
adrs: [63, 66]
spec:
plan: docs/superpowers/plans/2026-08-02-default-build-review-roles-to-docket-owned-skills.md
results: docs/results/2026-08-02-default-build-review-roles-to-docket-owned-skills-results.md
trivial: true
auto_groomable:
branch: feat/default-build-review-roles-to-docket-owned-skills
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/152
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-08-02-default-build-review-roles-to-docket-owned-skills.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-02-default-build-review-roles-to-docket-owned-skills.md) |
| Results | [2026-08-02-default-build-review-roles-to-docket-owned-skills-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-02-default-build-review-roles-to-docket-owned-skills-results.md) |
| ADRs | [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md) |
<!-- docket:artifacts:end -->

## Why

Changes 0167 and 0170 built docket's own build and review roles — `docket-build` (profile-routed
per-task workers) and `docket-review` (one bounded read-only whole-branch reviewer behind three
pinned rungs) — but shipped them as opt-in. The built-in defaults in `docket-config.sh` still
resolve `skills.build` to `superpowers:subagent-driven-development` and `skills.review` to
`superpowers:requesting-code-review`; docket's own repo picks up the new roles only because
`.docket.yml` overrides them.

Both roles have now been exercised enough on this repo to be trusted as the shipped default. Every
consuming repo should get them without opting in, and docket's own override should go away so the
repo genuinely dogfoods the default rather than a pin over it.

## What changes

- Flip the two built-in defaults in `scripts/docket-config.sh`: `SKILL_BUILD=docket-build`,
  `SKILL_REVIEW=docket-review`.
- Remove the now-redundant `skills:` block from this repo's `.docket.yml` (and its explanatory
  comment), so docket runs on the shipped defaults.
- Update the live documentation that names the old defaults: `.docket.example.yml`, `README.md`,
  `scripts/docket-config.md`, and the *Skill layer* role table in `skills/docket-convention/SKILL.md`
  plus any restatement in `skills/docket-implement-next/SKILL.md`.
- Update the affected tests — `tests/test_docket_config.sh` (default-resolution assertions),
  `tests/test_docket_example_yml.sh` (mirror/drift guards), and any default-naming assertions in
  `tests/test_docket_build.sh` / `tests/test_docket_review.sh`.
- Note the reversal of intent against ADR-0063 and ADR-0066 only if their text asserts the
  opt-in-not-default posture; if so, record the flip per the ADR rules rather than editing them.

Verified call-site inventory (reconcile pass, 2026-08-02) — every live site naming an old default:

- `scripts/docket-config.sh:511-512` — the two built-in default strings.
- `.docket.yml:32-34` — the `skills:` block to remove (the sibling `build: checkpoint: true` block stays).
- `.docket.example.yml:399-400` (the `skills:` template) and `:157-159` (the docket-build blurb calling
  itself "the alternative to superpowers:subagent-driven-development ... inert unless bound").
- `README.md:378-379` (role table) and `:705`, `:714`, `:728`, `:752` (four passages of opt-in prose,
  including "this stays the shipped default for everyone who hasn't opted in" and "the shipped
  cross-harness default stays `superpowers:requesting-code-review`").
- `scripts/docket-config.md:177`.
- `skills/docket-convention/SKILL.md:49-50` (config sample) and `:120-121` (Skill layer role table).
- `skills/docket-implement-next/SKILL.md:74`, `:80`, `:82` — three restatements, including step 6's
  rung-default sentence "the shipped default `superpowers:subagent-driven-development` emits none",
  whose *rule* (no build record ⇒ `docket-review-standard`) survives the flip but whose naming of SDD
  as the shipped default does not.
- Tests: `tests/test_docket_config.sh:456-457` and `:604`; `tests/test_docket_example_yml.sh:116-117`;
  `tests/test_docket_build.sh:506`, `:516`; `tests/test_docket_review.sh:196`.

`skills/docket-build/SKILL.md:8` describes docket-build as "the lean alternative to
`superpowers:subagent-driven-development`" — a comparative framing, not a default claim; out of scope.

## Out of scope

- Any behavior change inside `docket-build` or `docket-review` themselves — this is a default flip,
  not a redesign of either role.
- The other three roles (`brainstorm`, `plan`, `finish`) keep their superpowers defaults.
- Historical records (archived changes, plans, specs, results, existing ADR bodies) are immutable
  and are not rewritten to match the new default.

## Open questions

- Non-Claude harnesses: `docket-build`/`docket-review` dispatch profile/rung subagents, so verify a
  Codex/Cursor-shaped install still resolves sensibly on the new default (the Tier-C
  authorized-or-halt posture should cover it, but confirm the docs say so).

## Reconcile log

### 2026-08-02 — claim reconcile

Change body verified against `origin/main` and `origin/docket` at claim time; still valid, scope
unchanged, no work done elsewhere. Three refinements:

1. **The conditional in "What changes" resolves to yes.** Both ADRs *do* assert the opt-in-not-default
   posture in their **Consequences**, not in their Decisions: ADR-0063:89 ("default stays
   `superpowers:subagent-driven-development`, so users who do nothing see no behavior [change]") and
   ADR-0066:78 ("The shipped cross-harness default for `skills.review` stays
   `superpowers:requesting-code-review`"). Neither *decision* is reversed — both still hold that docket
   owns the role — so this is a consequence that has since been overtaken, which the convention handles
   as a dated `## Update` note appended to each ADR, not a new reversing ADR and not an edit to the
   decision text. Whether that is the right call is the one non-obvious judgment in this change and is
   the natural candidate for the step-6 ADR dispatch.
2. **The call-site inventory above was verified line-by-line** rather than left as a category list; it
   adds four README opt-in passages and the `.docket.example.yml` docket-build blurb that the original
   body's "documentation that names the old defaults" implied but did not enumerate.
3. **One rule survives its example.** `skills/docket-implement-next/SKILL.md:80` uses SDD's
   record-less-ness to motivate the `docket-review-standard` rung default. Post-flip the default build
   skill *does* emit a record, so the sentence needs rewording without dropping the rule — the fallback
   still has to cover a bound role that emits no record.

No dependency drift (`depends_on` empty). `trivial: true` still correct — mechanical default flip plus
doc/test updates, no design surface.
