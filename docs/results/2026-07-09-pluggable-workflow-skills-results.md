# Results — Pluggable workflow skills (change 0049)

**Change:** #0049 · PR: _(opened by this build)_ · Spec: `docs/superpowers/specs/2026-07-08-pluggable-workflow-skills-design.md` · Plan: `docs/superpowers/plans/2026-07-09-pluggable-workflow-skills.md` · ADR: **ADR-0018**

Built autonomously via `docket-implement-next` (SDD: 5 plan tasks, per-task spec+quality review, opus whole-branch final review). Build-receipt detail (files, full test output) lives in the PR description; this file records what the merge-gate reviewer should know beyond "green CI."

## Merge-gate note — what automated tests do and don't cover

The hermetic bash suite (`tests/test_docket_config.sh`, 18-line export, fixtures G–J) fully covers the **config-resolution** half: absent `skills:` ⇒ five superpowers defaults (byte-identical), `auto` passthrough, custom names, partial maps, **tab- and space-indented** blocks, nested-read isolation from a future top-level `build:`/`review:`, and unknown-role warn-and-ignore (exit 0). Wiring sentinels assert each switched skill body names its `$SKILL_*` var.

What the suite **cannot** exercise (documented contracts about live harness behavior, not unit-testable):

- **`auto` inline execution** — a role set to `auto` performing the step inline and producing the correct final artifact.
- **Degrade-on-missing** — a resolved-but-unavailable skill degrading to `auto` + prominent warning (run output; PR-body note for build-time roles).

These are contracts on the invoking agent, single-sourced in the convention's *Skill layer*. Recommended manual sanity check post-merge (optional, on a scratch repo): set `skills.build: auto` and run an implement to confirm the inline build path and the PR-body degrade note behave as documented.

## Findings

- **ADR-0018** recorded (on `origin/docket`; rides change 0049's terminal-publish onto `main` at merge): unvalidated skill-name passthrough + degrade-to-`auto` (not abort) on a missing skill — the intentional divergence from docket's autonomous abort-and-report rule, because skill availability is a per-machine property, not a repo-state error.
- Final whole-branch review verdict: **Ready to merge — Yes** (no Critical/Important). Two doc-consistency nits fixed in the build (plan-producer prose made rebind-clean; finalize's finish site got an `auto` note). Two non-blocking minors left as-is: `SKILLS_BLK` temp cleaned by explicit `rm -f` rather than a `trap` (safe — linear, no intervening exit); the ADR-0015 "passthrough" analogy label (attribution sound, carried verbatim from the spec).

## Follow-ups (candidates, none blocking)

- **#0044** (configurable SDD build models) stays `proposed`; its `build.implementer`/`build.reviewer` knobs become inert unless `skills.build` resolves to SDD — no action needed here, guard recorded in ADR-0018 and the spec.
- Optional future hardening: fold `SKILLS_BLK` under the same `trap … EXIT` as `$CFG` if that region is ever edited to add an early exit.
- Harness-keyed `skills:` (a `skills.<harness>` map paralleling 0046's harness-first `agents:`) was considered during reconcile and deliberately deferred — flat map + degrade-to-auto already covers the per-machine availability case; revisit only if a real need for per-harness skill divergence appears.
