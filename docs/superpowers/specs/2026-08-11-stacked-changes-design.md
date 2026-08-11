<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0298 — Stacked changes — build a new change on top of a parent change's branch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0298-stacked-changes-build-a-new-change-on-top-of-a-parent-change.md)**
<!-- docket:backlink:end -->

# Stacked changes — build a new change on a parent change's branch

**Change:** 0298 · **Date:** 2026-08-11 · **Status:** Approved design (brainstormed with Daniel)

## Problem

While testing an implemented change, the human routinely discovers follow-up work that is
genuinely its own change (own spec, own PR-sized unit) but must be built **on top of the
unmerged parent branch** — otherwise half-finished work has to merge into main before the
feature is complete. Today the convention forbids this: feature branches are always cut from
`origin/<integration_branch>`, and `depends_on` is satisfied only at `done`, so the only ways
to express "B continues A's unmerged work" are hand-rolled branches outside docket or merging
A prematurely.

## Golden path

Branch B (and possibly C) is cut from A's feature branch, built and reviewed as its own change
with its own PR whose **base is A's branch**, and merged **into A**. A's single PR then carries
the whole stack into main. Sequential-PRs-onto-main and fold-into-one-PR were considered and
rejected: the first blocks B's merge on A's, the second destroys B as a reviewable unit
(that is 0158 batch-mode territory).

## Design

### 1. Declaration — `stacked_on:` frontmatter

- New optional change-manifest field `stacked_on: <parent change id>` — an integer **scalar**
  (one parent, unlike the `depends_on` list). Set at creation ("stack this on 0281") or added
  to a still-`proposed` change.
- Docket resolves the parent's `branch:` itself at build time; the human never supplies raw
  branch names. (Rejected alternatives: ad-hoc branch instruction — relationship dies with the
  session, finalize and board can't see it; overloading `depends_on` — silently changes its
  wait-until-done meaning and cannot distinguish "wait for" from "build on".)
- The parent id is also recorded in `related:` so the reconcile pass reads it. It is **not**
  added to `depends_on` — that would deadlock (depends_on waits for `done`; stacking exists
  precisely to start earlier).
- Nesting is allowed (C stacked on B stacked on A) — it falls out of the mechanics naturally.
  A cycle check refuses the write (a `stacked_on` chain must terminate at an un-stacked change).

### 2. Readiness & selection

A stacked change is **build-ready** when the existing conditions hold (`proposed`, spec or
`trivial: true`, all `depends_on` done) AND its parent is `in-progress`, `implemented`, or
`blocked` with a non-empty `branch:`. Until then the board shows
**waiting on #A — stack base not built**. The deterministic selection order is unchanged;
stacking adds an eligibility condition, not a ranking one.

### 3. Build mechanics

`docket-implement-next` on a stacked change cuts `feat/<slug>` from **`origin/<parent branch>`**
instead of `origin/<integration_branch>` — the one deliberate, declared exception to the branch
model's "always cut from the integration branch" rule (the convention text gains this exception
alongside the rule, not as a silent contradiction). Worktree creation, plan, TDD build, and
review proceed exactly as today. The reconcile pass treats the **parent branch tip** (not main
alone) as current reality for the child's scope.

### 4. Child PR & merge gate

- The child's PR is opened with **base = the parent's feature branch**. The human merge gate is
  unchanged: review B's own diff, merge B into A's branch.
- Finalize's rebase-retest gate rebases the child onto `origin/<parent branch>` and runs the
  suite there — the gate's "base" is generalized from "integration branch" to "the change's
  effective base": parent branch when `stacked_on` is live, integration branch otherwise.
- When the child's PR merges, the normal sweep moves it to `done`. The terminal close-out runs
  as usual except its publish step — see §6: a stacked child's terminal-publish is deferred to
  the stack root's merge.

### 5. Parent finalize gate

Finalizing a parent with open (non-terminal) stacked children:

- **Autonomous finalize: hard block** — abort-and-report, the parent stays `implemented`, the
  reason recorded via finalize's existing failure channels.
- **Interactive finalize: warn**; the human may accept the warning and merge anyway.
- On an accepted override — or whenever a parent reaches a terminal state with open children —
  each open child's **effective base falls back to the parent's own base** (normally the
  integration branch; the grandparent's branch in a nested stack). The child's branch is rebased
  and its PR retargeted at the child's own next finalize gate — not eagerly — and the board
  flags it **stack base merged — rebase pending**.
- A parent **killed** while it has already-`done` children warns loudly: those children's merged
  code dies with the parent branch (they are `done` in metadata but never reached main).

### 6. Artifact flow at a child's close-out

The child's artifacts split by where they live:

- **Plan and results files** live on the child's feature branch, so they merge into the parent
  with the code and reach main when the stack root merges. No copying, no new mechanics; the
  `plan:`/`results:` fields are valid the moment the root lands.
- **Change file and spec** live on `metadata_branch`; under `terminal_publish: true` the normal
  close-out would copy them onto the integration branch at `done` — ahead of the code. A stacked
  child's close-out instead **defers**: it marks the existing `## Publish deferred` state
  (`mark-publish-deferred.sh`) rather than publishing. When the **stack root's** merge lands, its
  finalize runs the deferred terminal-publish for every `done` descendant; `terminal-publish.sh`
  auto-removes the deferred marker on success, exactly as today. Main never carries a done-record
  for code it doesn't have. A parent **killed** with done children surfaces their deferred
  publishes as part of the killed-parent warning (their records then stay on `metadata_branch`
  unless the human publishes explicitly). Under `terminal_publish: false` nothing changes —
  a skipped publish is already success.

### 7. Board

The inline board renders the relationship (e.g. `↳ stacked on #0281`) plus the two derived
cells above (*stack base not built*, *rebase pending*).

## Out of scope

- **Batch mode** (0158) — several changes on one branch/one PR stays a separate design.
- **Continuous restacking** — when the parent branch moves mid-flight, the child picks up the
  parent's new commits at its own rebase gate, never automatically.
- **GitHub auto-retarget reliance** — retargeting is done by docket at the child's finalize,
  not by trusting GitHub's base-branch auto-retarget on branch deletion.

## Touch points (implementation altitude, non-exhaustive)

**Progressive disclosure rule for every skill touched:** stacking mechanics land in a shared
reference (e.g. `docket-convention/references/stacked-changes.md`), read **blocking, on
trigger** — i.e. only when the change at hand carries `stacked_on:` (or, for finalize, has
stacked children). Skill bodies gain one trigger line each, never the mechanics; keep the
always-loaded surface flat.

- Convention (`docket-convention`): manifest field, branch-model exception (stated alongside
  the rule, not as a silent contradiction), readiness definition, board cells; owns the
  `references/stacked-changes.md` mechanics file.
- `docket-new-change` / `docket-groom-next`: accept and validate `stacked_on` (existence +
  cycle check).
- `docket-implement-next`: readiness filter, branch-cut base, reconcile input, PR base.
- `docket-finalize-change`: effective-base generalization for the rebase-retest gate; parent
  open-children gate (block vs. interactive warn-and-override); child fallback/retarget flow;
  child close-out publish deferral + root-merge deferred-publish sweep; killed-parent warning.
- Board renderer + health checks: relationship cell, waiting/rebase-pending states, a stale
  `stacked_on` (parent gone terminal) check.

## Testing

Suite coverage for: cycle refusal; readiness gating on parent branch presence; branch cut from
parent; finalize effective-base selection; autonomous block vs. interactive override; fallback
retarget bookkeeping; child close-out publish deferral and the root-merge deferred-publish
sweep (including `terminal_publish: false` no-op); killed-parent warning; board rendering of
all three cells.
