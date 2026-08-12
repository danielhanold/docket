<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0298 — Stacked changes — build a new change on top of a parent change's branch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0298-stacked-changes-build-a-new-change-on-top-of-a-parent-change.md)**
<!-- docket:backlink:end -->

# Stacked changes — build a new change on a parent change's branch

**Change:** 0298 · **Date:** 2026-08-11 · **Status:** Approved design (brainstormed with
Daniel; revised same day after his review of `f5573ea` — two lifecycle blockers fixed by
introducing the `stacked-merged` state and a shared effective-base resolver)

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

## Governing invariant

> **`done` means the change's code is reachable from the integration branch.**

Everything below preserves this. A child merged only into its parent is NOT done — it enters
the new non-terminal `stacked-merged` state and is promoted to `done` when its stack root
lands on the integration branch. This keeps `depends_on` satisfaction, artifact-link
retargeting, terminal publishing, and the archive rule all meaning what they mean today.

## Design

### 1. Declaration — `stacked_on:` frontmatter

- New optional change-manifest field `stacked_on: <parent change id>` — an integer **scalar**
  (one parent, unlike the `depends_on` list). Set at creation ("stack this on 0281") or added
  to a still-`proposed` change.
- Docket resolves the parent's branch itself; the human never supplies raw branch names.
  (Rejected alternatives: ad-hoc branch instruction — relationship dies with the session,
  finalize and board can't see it; overloading `depends_on` — silently changes its
  wait-until-done meaning and cannot distinguish "wait for" from "build on".)
- `stacked_on:` is the **single source of truth** for the relationship. The parent id is NOT
  copied into `related:` or `depends_on` — reconcile and every other reader consume
  `stacked_on:` directly. (`depends_on` would deadlock: it waits for `done`, and stacking
  exists precisely to start earlier. A `related:` copy would be a second reference with its
  own drift and removal semantics for no reader that needs it.)
- Nesting is allowed (C stacked on B stacked on A). A cycle check refuses the write — a
  `stacked_on` chain must terminate at an un-stacked change.

### 2. Lifecycle — the `stacked-merged` state

```text
proposed → in-progress → implemented
                            │
                            ├─ PR merges into integration branch ──────────────→ done
                            │
                            └─ PR merges into parent branch → stacked-merged
                                                                  │
              stack root's code reaches the integration branch ───┘ → done
```

- `stacked-merged` is **non-terminal**: the change file stays in `active/`, is never archived,
  and never terminal-publishes while in this state. It satisfies `depends_on` for **nothing**
  (the existing done-only rule is unchanged — an unrelated change depending on the child waits
  until the child's code actually reaches the integration branch). Within-stack ordering needs
  no depends_on: express it by stacking C on B.
- Promotion to `done` happens in the **stack close-out** (§7): when the root's code reaches
  the integration branch, every `stacked-merged` descendant is promoted, archived, and
  published through the normal terminal close-out. The archive date prefix is the **root's**
  merge date — that is when the child's code reached the integration branch.
- The board renders `stacked-merged` as its own cell (e.g. *merged into #A — awaiting stack
  root*). The `publish-deferred` health check is untouched: this is an ordinary non-terminal
  state, not an incomplete terminal operation, so no `## Publish deferred` marker is ever
  written for it.
- Cost, stated honestly: an eighth lifecycle state touches every state-enumerating surface
  (board renderer, selectors, sweeps, health checks, `verify-run`). The alternative —
  overloading `done` — was reviewed and rejected as a correctness hole (broken links, premature
  dependency satisfaction, false terminal records under a killed root).

### 3. The effective-base resolver (one shared operation)

A stacked change's **effective base** is resolved by one shared routine, used by *every*
consumer — readiness, branch creation, reconcile, review diff, PR base, the finalize gate,
artifact links, board rendering, and health checks — never re-derived ad hoc:

1. **Live parent** (`in-progress`/`implemented`/`blocked`/`stacked-merged`) whose
   `origin/<parent branch>` exists and fetches → that branch.
2. **Merged parent** (`done`, or `stacked-merged` whose branch is already gone) → recursively
   resolve the **parent's** effective base (grandparent's branch in a nested stack, else the
   integration branch).
3. **Killed parent** → no base: stop with a human-required disposition (§9).
4. **Missing parent id, cycle, or a `branch:` whose remote ref is missing** → invalid: the
   change is flagged by a health check and is not build-ready.

Rule 1's remote-ref requirement is load-bearing: today `branch:` is stamped at claim but the
feature branch is only pushed at the PR step, so an `in-progress` parent can carry a
valid-looking `branch:` with no `origin/` ref behind it. The ref must exist and fetch — which
makes `implemented` parents (branch pushed, PR open) the ordinary stacking case, while still
admitting an `in-progress` parent whose branch has been pushed.

### 4. Readiness & selection

A stacked change is **build-ready** when the existing conditions hold (`proposed`, spec or
`trivial: true`, all `depends_on` done) AND its effective base resolves via §3. A child whose
parent merged early therefore resolves upward and stays viable (fixing the
readiness-vs-fallback contradiction); a child whose parent is killed, or whose resolution is
invalid, is not ready and is surfaced. Until a base resolves the board shows
**waiting on #A — stack base not built**. The deterministic selection order is unchanged;
stacking adds an eligibility condition, not a ranking one.

### 5. Build mechanics

`docket-implement-next` on a stacked change cuts `feat/<slug>` from the **resolved effective
base** instead of `origin/<integration_branch>` — the one deliberate, declared exception to
the branch model's "always cut from the integration branch" rule (the convention text gains
this exception alongside the rule, not as a silent contradiction). Worktree creation, plan,
TDD build, and review proceed exactly as today. The reconcile pass treats the **effective base
tip** (not the integration branch alone) as current reality for the child's scope.

### 6. Child PR & merge gate

- The child's PR is opened with **base = the resolved effective base branch**. The human merge
  gate is unchanged: review the child's own diff, merge it into the parent's branch.
- Finalize's rebase-retest gate rebases the child onto its effective base and runs the suite
  there — the gate's "base" is generalized from "integration branch" to "effective base".
- When a child's PR merges into a **parent branch**, the sweep moves it to `stacked-merged`
  (not `done`): no archive, no terminal publish, links stay branch-addressed. When a PR merges
  into the **integration branch** (a stack root, or any un-stacked change), the existing
  done-path runs unchanged, plus the stack close-out below.

### 7. Stack close-out — shared, idempotent, both merge sites

When a stack **root** reaches the integration branch, one shared operation — invoked by
`docket-finalize-change` *and* by the `docket-status` merge sweep, since roots merged through
the GitHub button are closed out by the sweep — settles the stack:

- Snapshot the transitive descendant graph (scan `active/` for `stacked_on:` chains rooted at
  the merged change).
- Promote every `stacked-merged` descendant to `done`: normal terminal close-out each
  (archive with the root's merge date, terminal-publish under `terminal_publish: true`,
  artifact links retargeted to the integration branch).
- Idempotent and resumable: each descendant's promotion is independently re-runnable; a
  partial run (crash, race) is completed by the next sweep. A sweep that sees the root's merge
  before it has processed a just-merged child handles the child in the same pass (the graph
  snapshot is taken fresh, and a child still `implemented` with a merged-into-parent PR is
  first swept to `stacked-merged`, then promoted).
- Open (non-`stacked-merged`) children at root-merge time are handled by §8/§9, not silently
  promoted.

### 8. Parent finalize gate & explicit retargeting

Finalizing a parent with open (non-terminal, non-`stacked-merged`) stacked children:

- **Autonomous finalize: hard block** — abort-and-report, the parent stays `implemented`, the
  reason recorded via finalize's existing failure channels.
- **Interactive finalize: warn**; the human may accept the warning and merge anyway.
- On an accepted override, or any merged-parent case: each open child's effective base
  resolves upward per §3, and — because the parent's remote branch is deleted by cleanup, and
  docket must not lean on GitHub's delete-time auto-retarget (explicitly out of scope) — the
  close-out **explicitly retargets every open child PR to the new effective base BEFORE
  deleting the parent's branch**; a parent branch with open child PRs that cannot be
  retargeted is retained, not deleted. The child's **rebase** itself stays lazy — performed at
  the child's own next finalize gate — and the board flags the child
  **stack base merged — rebase pending**.

### 9. Killed-parent policy (distinct from merged)

A merged parent's commits survive in its base, so falling back upward is safe. A killed
parent's commits do not — rebasing a child "onto the parent's base" either drops code the
child builds on or resurrects killed commits. So kill is its own policy, never the merge
fallback:

- **Killed parent with open descendants** → each open descendant flips to `blocked`
  (`blocked_by: stack parent #A killed — re-scope, re-parent, or kill`). A human decides;
  nothing auto-rebases.
- **Killed root (or intermediate parent) with `stacked-merged` descendants** → those
  descendants' code now can never reach the integration branch through this stack: they flip
  to `blocked` with the same human-decision marker, their branches and the killed parent's
  branch are **preserved** for recovery, and no false `done` record is ever written (the
  `stacked-merged` state is what makes this representable). The kill path warns loudly,
  enumerating the affected descendants.

### 10. Artifact flow

- **Plan and results files** live on the child's feature branch, merge into the parent with
  the code, and reach the integration branch when the root lands. The `plan:`/`results:` links
  stay branch-addressed while the change is non-terminal (`stacked-merged` included) and
  retarget to the integration branch only at promotion to `done` — so links are never broken
  in the interim.
- **Change file and spec** live on `metadata_branch` and terminal-publish exactly once, at
  promotion to `done` in the stack close-out. No `## Publish deferred` marker is involved.
- **Parent spec: never updated.** A point-in-time record of the parent's own designed scope;
  the child's design lives in the child's spec.
- **Parent plan: never updated.** The child's plan file arrives on the parent branch with the
  child's merge; the branch carries both plans side by side.
- **Parent results: optional and immutable, never retro-edited.** The stack's carried-work
  summary does NOT go there — finalize merges before it closes out, and a GitHub-button merge
  has no pre-merge authoring moment at all, so a results file cannot enumerate "what this PR
  carries" in time. Instead the stack close-out writes a generated **Stack carried** table —
  each descendant's id, title, PR — into the **root's terminal change record** (the archived
  change file, alongside its other generated blocks), derived from the descendant graph at
  close-out. Root record = "what this merge carried"; child records = "what each child built".

### 11. Reciprocal visibility — derived, never denormalized

The child's `stacked_on:` remains the single source of truth. The parent gets the reverse link
as a **derived view**, not a frontmatter field: `render-change-links.sh` scans `active/` +
`archive/` for `stacked_on: <parent id>` and renders a **Stacked children** row (id, title,
status) into the parent's generated `## Artifacts` block — sole-writer, regenerated on every
frontmatter write, drift-free by construction. A denormalized `stacked_children:` list was
rejected: every writer would have to maintain the second copy, and a missed update would
silently lie to the finalize gate — which therefore also derives the child set by scanning
(via the same shared graph routine as §7), never by reading a parent-side list.

### 12. Board

The inline board renders the relationship (e.g. `↳ stacked on #0281`) plus the derived cells:
**waiting on #A — stack base not built**, **merged into #A — awaiting stack root**
(`stacked-merged`), **stack base merged — rebase pending**, and the killed-parent `blocked`
rows surface through the existing blocked rendering with their `blocked_by:` reason.

## Out of scope

- **Batch mode** (0158) — several changes on one branch/one PR stays a separate design.
- **Continuous restacking** — when the parent branch moves mid-flight, the child picks up the
  parent's new commits at its own rebase gate, never automatically.
- **GitHub auto-retarget reliance** — retargeting is performed explicitly by docket before any
  parent-branch deletion (§8), never by trusting GitHub's delete-time base retargeting.

## Touch points (implementation altitude, non-exhaustive)

**Progressive disclosure rule for every skill touched:** stacking mechanics land in a shared
reference (e.g. `docket-convention/references/stacked-changes.md`), read **blocking, on
trigger** — i.e. only when the change at hand carries `stacked_on:` (or, for finalize/status,
the affected change has stacked children). Skill bodies gain one trigger line each, never the
mechanics; keep the always-loaded surface flat.

- Convention (`docket-convention`): manifest field, the `stacked-merged` lifecycle state and
  governing invariant, branch-model exception (stated alongside the rule, not as a silent
  contradiction), readiness definition, board cells; owns the `references/stacked-changes.md`
  mechanics file.
- **Shared routines** (script-owned, single implementation each): the effective-base resolver
  (§3), the descendant-graph scan, and the idempotent stack close-out (§7).
- `docket-new-change` / `docket-groom-next`: accept and validate `stacked_on` (parent exists +
  cycle check).
- `docket-implement-next`: readiness via the resolver, branch cut from the effective base,
  reconcile input, PR base.
- `docket-finalize-change`: effective-base rebase-retest gate; parent open-children gate
  (autonomous block vs. interactive warn-and-override); explicit child-PR retarget before
  parent-branch deletion (retain the branch if retarget fails); stack close-out invocation;
  killed-parent policy; Stack carried table in the root's terminal record.
- `docket-status` sweep: merged-into-parent PRs sweep to `stacked-merged`; root merges invoke
  the same shared stack close-out; health checks for invalid resolution (missing parent,
  cycle, missing remote ref) and killed-parent blocked descendants.
- `render-change-links.sh`: derived **Stacked children** row in the parent's `## Artifacts`
  block; branch-addressed links for `stacked-merged` changes.

## Testing

Suite coverage for: cycle refusal and missing-parent refusal; the resolver's four rules,
including a populated `branch:` with a missing/stale remote ref and nested stacks whose
intermediate parents are already merged; readiness gating through the resolver (proposed and
implemented children when the parent is interactively merged early); branch cut from the
effective base; child merge sweeping to `stacked-merged` (never `done`, no archive, no
publish, no false `publish-deferred` warning); an ordinary `depends_on` referencing a
`stacked-merged` child staying unsatisfied; root merged through finalize AND through the
`docket-status` sweep; the root sweep racing a just-merged child sweep; multiple sibling
children merged in different orders; partial descendant close-out followed by idempotent
retry; explicit child-PR retarget before parent cleanup (and branch retention on retarget
failure); interim branch-addressed artifact links before the root lands, retargeted at
promotion; autonomous block vs. interactive override; killed parent with open and with
already-`stacked-merged` descendants (blocked flips, preserved branches, loud enumeration);
the Stack carried table in the root's terminal record; board rendering of all stack cells.
