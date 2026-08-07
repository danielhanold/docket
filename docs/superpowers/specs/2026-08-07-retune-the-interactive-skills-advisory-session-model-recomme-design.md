<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0166 — Retune the interactive skills' advisory session-model recommendation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0166-retune-the-interactive-skills-advisory-session-model-recomme.md)**
<!-- docket:backlink:end -->

# Retune the interactive skills' advisory session-model recommendation — design

**Change:** 0166 · **Date:** 2026-08-07 · **Type:** chore (values + one test-anchoring mechanism)

## Problem

`docket-new-change` and `docket-groom-next` get no generated wrapper (a skill cannot force the
session model), so each surfaces an advisory session-model recommendation at startup. Both still
name `claude-sonnet-5`. Since change 0164 (and 0168's relocation of shipped defaults into
`agents/harness-defaults.yml`), every judgment-bearing claude wrapper — including
`brainstorm-consultant`, the agent that authors specs inside these very skills' design flow —
sits on `claude-opus-5`. A human following the advisory runs the design conversation on a model
no other docket surface names. The advisory was pinned by bare string assertions, which is
exactly why 0164 could not carry it and why it drifted.

## Current state (verified 2026-08-07)

- `skills/docket-new-change/SKILL.md:21` — "Recommended: `claude-sonnet-5`, effort: model
  default"; `skills/docket-groom-next/SKILL.md:20` — "Recommended: `claude-sonnet-5` / `high`".
- The stub cites `tests/test_sync_agents.sh:494-495`; those assertions have MOVED (change 0227
  sharding) to `tests/test_sync_agents_drift_docs.sh:99-106` (Task 6 block). Scope follows the
  moved file.
- Shipped claude defaults live in `agents/harness-defaults.yml` (change 0168):
  `brainstorm-consultant: { model: claude-opus-5, effort: medium }`.
- `tests/lib/sync_agents_common.sh` already defines `HD` (path to harness-defaults.yml) and
  `hd_field` (line 85 of `scripts/lib/harness-defaults.sh` provides the implementation), so a
  test can resolve the shipped value instead of hardcoding it.

## Design

### 1. What the advisory recommends

Both skills anchor the recommendation to the shipped claude `brainstorm-consultant` default —
today `claude-opus-5`. Rationale: the consultant is the same design flow's authoring agent; the
interactive session that runs the dialogue should not sit below the agent that writes the spec
from it.

- `docket-new-change`: "Recommended: `claude-opus-5`, effort: model default" — keep the
  effort-unpinned posture and its existing rationale (wide variance from a trivial stub to a
  full brainstorm). `/model claude-opus-5` in the set-to-match sentence.
- `docket-groom-next`: "Recommended: `claude-opus-5` / `medium`" — mirror the consultant PAIR.
  The recorded rationale for `high` ("the cold-start recap is genuine synthesis") is task-based
  and still true; the effort call is nonetheless re-made here on fleet-pricing grounds: every
  comparable claude synthesis role ships at opus-5/`medium` (`brainstorm-consultant`,
  `auto-groom-critic`, `implement-next`, `integration-repair`; autonomous self-brainstorm at
  `low`), so an interactive groom at opus-5/`medium` sits at-or-above its autonomous
  equivalents, and the full-pair mirror makes the Task 6 assertion a real pair-check rather
  than model-only. Advisory only — the human is free to `/effort high`. `/model claude-opus-5`
  and `/effort medium` in the set-to-match sentence.

Each advisory gains one clause naming its anchor — that the value mirrors the shipped
`brainstorm-consultant` claude default in `agents/harness-defaults.yml` and retunes with it —
plus one sentence for non-claude harnesses: use your harness's `brainstorm-consultant` row from
`agents/harness-defaults.yml` in the docket clone (the file is not present in consuming repos). No per-harness literals in the skills (that would mint three more drift
surfaces).

### 2. Literal vs. tier-pointer (the stub's open question)

The advisory KEEPS a literal model ID — a pointer-only advisory ("run whatever your config
resolves") is not actionable as a `/model` command and not assertable. The drift class is closed
on the test side instead: the string pins become mirror assertions that resolve the expected
value from `harness-defaults.yml` via `hd_field` at test time — the mirror rule first recorded as ADR-0039 and carried forward through its
successors (ADR-0048, ADR-0064): a surface that advertises the shipped defaults is
machine-checked against their source, applied here to the advisory surface. A future retune
that edits `harness-defaults.yml` but forgets the SKILL.md advisories now fails the suite
instead of drifting silently.

### 3. Test changes — `tests/test_sync_agents_drift_docs.sh` (Task 6 block, ~lines 99-106)

- Keep: "carries an advisory recommendation", "frames it as advisory" (structure assertions).
- Drop: the two `grep -qi "sonnet"` value assertions and the two hardcoded
  `claude-sonnet-5` pins.
- Add (mirror-enforced): resolve `EXP_M="$(hd_field "$HD" claude brainstorm-consultant model)"`
  and `EXP_E="$(hd_field "$HD" claude brainstorm-consultant effort)"`; assert each SKILL.md
  contains `$EXP_M`; assert groom-next's advisory pairs `$EXP_M` with `$EXP_E` (the existing
  model/effort adjacency regex, parameterized); assert new-change still says "model default"
  for effort. Update the block's comments — the "change 0042 explicit pin" note becomes "0166:
  mirror-enforced against harness-defaults.yml".

### 4. Sites touched

- `skills/docket-new-change/SKILL.md` — the one advisory paragraph.
- `skills/docket-groom-next/SKILL.md` — the one advisory paragraph.
- `tests/test_sync_agents_drift_docs.sh` — Task 6 block only.
- Nothing else: wrappers, `harness-defaults.yml`, `.docket.example.yml`, README are 0164/0168
  surfaces and already correct.

Verification: `bash tests/test_sync_agents_drift_docs.sh` green; grep confirms no
`claude-sonnet-5` remains under `skills/docket-new-change/` or `skills/docket-groom-next/`.

## Assumptions

1. **Anchor tier = `brainstorm-consultant` (claude row), so the recommendation becomes
   `claude-opus-5`.** Rejected: keep `claude-sonnet-5` (perpetuates the exact drift 0166 exists
   to fix — no docket surface names it anymore); anchor to `implement-next` or a new
   "interactive" row in harness-defaults.yml (the consultant is the same design flow's authoring
   role, and minting a new defaults row for a non-wrapper would violate that file's "key set
   equals agents/docket-*.md" rule).
2. **The advisory keeps a literal model ID; drift is closed by mirror-enforcing tests, not by
   removing the literal.** Rejected: prose tier-pointer only (not actionable as `/model`, not
   assertable); keeping bare string pins (that reliance on future retunes remembering is the
   failure mode that produced this stub).
3. **groom-next's effort moves `high` → `medium` (full-pair mirror); new-change stays
   effort-unpinned ("model default").** This is a fresh effort judgment, not a mechanical
   consequence of the model move — the recorded `high` rationale ("the cold-start recap is
   genuine synthesis") is task-based and survives the model change. Grounds for `medium`
   anyway: the fleet prices every comparable claude synthesis role at opus-5/`medium`, so the
   interactive groom still sits at-or-above its autonomous equivalents, the pair then mirrors
   the anchor tier exactly (making the test a pair-check), and the advisory is non-binding —
   a human who wants `high` types `/effort high`. Rejected: keep `high` on opus (breaks the
   pair mirror and prices the interactive session above every autonomous synthesis role with
   no fleet precedent); pin new-change's effort too (its stated wide-variance rationale still
   holds and mirroring model-only there costs nothing).
4. **Effort is mirror-enforced only where pinned (groom-next).** new-change's "model default"
   is asserted as the literal phrase, not against `hd_field` effort.
5. **Scope tracks the moved assertions** in `tests/test_sync_agents_drift_docs.sh` (change 0227
   shard), not the stub's stale `test_sync_agents.sh:494-495` citation. No edits to
   `test_sync_agents.sh` itself.
6. **Non-claude harnesses get one prose sentence** pointing at their harness's
   `brainstorm-consultant` row; no per-harness literals and no new test assertions for them.
7. **Dependency state:** 0164 is done/merged (PR #138); 0168, 0169, 0192, 0227 are archived.
   Nothing blocks; `related: [168, 227]` recorded as the anchor-file and moved-test couplings.
