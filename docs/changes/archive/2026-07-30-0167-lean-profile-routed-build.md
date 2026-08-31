---
id: 167
slug: lean-profile-routed-build
title: Lean profile-routed build — fresh task workers without review loops
status: done
priority: high
type: feat
created: 2026-07-30
updated: 2026-07-30
depends_on: []
related: [42, 44, 135, 137]
discovered_from: []
adrs: [23, 63]
spec: docs/superpowers/specs/2026-07-30-lean-profile-routed-build-design.md
plan: docs/superpowers/plans/2026-07-30-lean-profile-routed-build.md
results: docs/results/2026-07-30-lean-profile-routed-build-results.md
trivial: false
auto_groomable:
branch: feat/lean-profile-routed-build
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/139
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-30-lean-profile-routed-build-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-30-lean-profile-routed-build-design.md) |
| Plan | [2026-07-30-lean-profile-routed-build.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-30-lean-profile-routed-build.md) |
| Results | [2026-07-30-lean-profile-routed-build-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-30-lean-profile-routed-build-results.md) |
| ADRs | [ADR-0023](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0023-configurable-sdd-build-model.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` currently wraps Superpowers SDD, whose per-task implementer/reviewer pairs,
fix/re-review rounds, and whole-branch review duplicate Docket's own review boundary and dominate
long-run token use. Fresh per-task implementers and focused tests remain valuable; the repeated
review topology and implicit effort inheritance do not.

## What changes

- Add a Docket-owned build controller (`docket-build`) and compact shared task-worker skill
  (`docket-build-task`).
- Ship three Claude profile agents (`docket-build-economy` / `-standard` / `-premium`) that preload
  the worker skill and differ only in model and effort; register them under `agents.claude`, never
  `agents.default`.
- Route each plan task to one of those profiles via an explicit plan override or the automatic
  rubric, with a single bounded escalation per task.
- Preserve focused TDD and one task commit while removing per-task review agents.
- Run the full suite once after the task sequence using `finalize.test_command` or the existing
  detection fallback, with one bounded integration-repair path.
- Add a nested `build.checkpoint` config key (global-able, default `false`) alongside the existing
  `learnings:` / `reclaim:` blocks in the layered resolver.
- Dogfood the new build through this repository's `skills.build` setting without changing the
  shipped cross-harness default.
- Update the surfaces that enumerate the generated agent set: `.docket.example.yml` (the
  `agents.claude` mirror plus its commented `codex:`/`cursor:` mirrors), the "nine wrappers" prose
  in the convention and README, `cursor-rules/dispatch/` fragments, and the tests that assert the
  agent roster, the example-yml key count, and per-skill size budgets.

## Out of scope

- Cursor and Codex profile dispatch (changes 0168 / 0169).
- Replacing the remaining independent whole-branch `skills.review` role (change 0170).
- Hard subagent turn caps.
- Closing change 0044 — already killed as superseded on 2026-07-30, before this build started.
- Minting the three follow-up changes — 0168, 0169, and 0170 already exist.

## Reconcile log

### 2026-07-30 — reconcile at claim

Re-read against `related: [42, 44, 135, 137]`, ADR-0023, the recently-archived changes, and current
`main`. The design holds; three scope items were already satisfied elsewhere and are dropped:

- **0044 is already terminal.** It was killed as superseded on 2026-07-30
  (`docs/changes/archive/2026-07-30-0044-configurable-build-model.md`), so the spec's "closes 0044
  as killed/superseded" step is a no-op. Its stale PR #69 and branch are not this change's problem.
- **The three follow-ups already exist** as changes 0168 (Cursor), 0169 (Codex), and 0170 (lean
  review), each already `waiting-on-167-unbuilt` in the digest. Nothing to mint.
- **ADR-0023 supersession is still owed.** ADR-0023 is `Accepted` and back-links `change: 44`; this
  change must produce a new ADR that supersedes it. Note for close-out: the current board health
  check already flags ADR-0023 as due for publication onto `main`, and a superseded (non-Accepted)
  ADR is skipped by the publish gate — so the supersession has to be published deliberately.

Folded in from current `main` (facts the spec was drafted without):

- `build.checkpoint` has an exact template: the `reclaim:` block in `scripts/docket-config.sh`
  (`yaml_block_body` + a per-leaf `reclaim_key` resolver + a `case true|false) … die` validator).
  Malformed booleans already fail closed there, matching the spec's "configuration errors rather
  than silent fallback".
- Adding `agents/docket-build-{economy,standard,premium}.md` is self-registering — `sync-agents.sh`
  discovers agents by the `agents/docket-*.md` glob. But several surfaces enumerate the roster by
  hand and will fail otherwise: `.docket.example.yml`'s `agents.claude` mirror and its commented
  `codex:`/`cursor:` mirrors, `tests/test_docket_example_yml.sh` (the mirror-equality loop and the
  exact `expected_key_count`), the "nine wrappers / five skills" prose in
  `skills/docket-convention/SKILL.md` and the README, the optional `cursor-rules/dispatch/`
  fragments, and `tests/test_dispatch_capability.sh`.
- `tests/test_skill_size_budgets.sh` has a completeness guard: each new `SKILL.md` must add its own
  `BUDGETS` row in the same diff, using the documented rounding rule.
- The `runner:` / `scripts/runners/` delegation layer (0079) is an orthogonal axis — it keys off an
  `agents:` entry, never off `skills.build` — so the profile agents need no runner work here.
- `.superpowers/` is already ignored by the committed root `.gitignore`, satisfying the spec's
  requirement that the opt-in ledger rely on a repository-level guarantee.

No auto-captured stubs from this pass: the two open health findings (ADR-0023 pending publish,
change 0044's `## Publish deferred` marker) are existing operational items the status sweep already
reports, not new follow-up work.
