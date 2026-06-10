---
name: docket-implement-next
description: Use when you want the next build-ready change in the docket backlog implemented end-to-end to an open PR with no human interaction — picking, claiming, reconciling against current reality, planning, building with TDD, reviewing, and stopping at the human merge gate. The autonomous backlog-drainer; runs solo per change.
---

# docket-implement-next — the implementer (autonomous)

## Overview

`docket-implement-next` runs with **no human interaction**: it picks the next build-ready change from the docket backlog and drives it all the way to an open PR, then stops at the human merge gate. One invocation handles one change — select, claim, reconcile, plan, build, review, PR, stop.

## When to use

- You want the backlog drained autonomously — pick the highest-priority build-ready change and ship it to a PR without human steering.
- A specific change id is ready and you want it implemented now (pass the id explicitly to skip selection).
- You are running a batch drain and want each change handled end-to-end in its own invocation.
- Do NOT use if you want to interact during brainstorm or design — that is `docket-new-change`'s job. This skill re-brainstorms nothing; the escape hatch for a fundamentally invalidated design is to STOP and hand back to the human.

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Procedure

### Step 0 — Sync & sweep

Ensure the **metadata working tree** is synced to its remote, then invoke `docket-status` (whose merge-sweep pass archives any `implemented` change whose PR has merged) before selection — the self-cleaning safety net for changes not closed via `docket-finalize-change`. All bookkeeping in this skill (claim, reconcile, `status`, `pr:`, the `plan:` field, `adrs:`) happens in the metadata working tree on `metadata_branch`, pushed to its remote immediately; only the plan + results + code land on the feature branch. In `docket`-mode the metadata working tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`); pushes target `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there).

### Step 1 — Select

Among `active/` changes that are `proposed`, BUILD-READY (have a `spec:` or `trivial: true`), and have all `depends_on` satisfied (satisfied = `done`), rank by priority (`critical` > `high` > `medium` > `low`) → age (`created`) → LOWEST `id` (the final deterministic tie-break, so two implementers — if ever run concurrently — converge on the same winner and never claim the same change). Pick the top, or accept an explicit id passed by the caller. Skip `in-progress`, `blocked`, `deferred`, and not-build-ready stubs.

### Step 2 — Claim (compare-and-swap)

Re-read the manifest after the sync, in the **metadata working tree**; if still `proposed`, set `status: in-progress` + `branch: feat/<slug>` + `updated: <UTC today>`; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket` via `.docket/`). On a non-fast-forward rejection: DISCARD the pending local claim commit (it edits the same `status:`/`branch:` lines and would conflict on replay), re-sync (`git pull --rebase`, or `git -C .docket pull --rebase origin docket`), RE-READ (mandatory); if still `proposed`, re-claim and push — LOOP until the push lands (it can be rejected repeatedly under load). The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds. No worktree yet.

Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board reflects the change as `in-progress` rather than build-ready.

> Two agents must NOT share one local clone — each needs its own.

### Step 3 — Reconcile ⭐

In the **metadata working tree** (re-synced to its remote), re-read the change + its spec against `related` + recently-archived changes, cited + recent ADRs, and CURRENT code; refresh the change body and spec to what is true NOW (drop work done elsewhere, adjust scope, fold in new constraints), NON-INTERACTIVELY. The spec lives alongside the change on `metadata_branch` (in `docket`-mode, `.docket/docs/superpowers/specs/…`). A trivial change has no spec — refresh the body only. Append a dated `## Reconcile log` entry; set `reconciled: true`; commit and push on `metadata_branch` (in `docket`-mode, `origin/docket`).

Two escape hatches:

- Change now **OBSOLETE** → set `status: killed` (+ `## Why killed`) + `updated: <UTC kill date>` in the metadata working tree. In `docket`-mode: push `origin/docket`, then run the shared **terminal-publish procedure (the *Terminal publish (docket-mode)* procedure in `docket-finalize-change`)** with outcome `killed` (token `T = <id>`) — any `Accepted` ADRs already in the change's `adrs:` ride along — and prune any feature worktree/branch already created for this change before looping back to Step 1. In `main`-mode (no `docket` branch / no terminal-publish): do the archive move (`active/ → archive/<UTC kill date>-<id>-<slug>.md`) + `status: killed` + `## Why killed` directly in the metadata working tree (= the integration branch) and push `origin/<integration_branch>`, then loop back to Step 1. The `<UTC kill date>` is the same date used for the `archive/<date>-…` filename prefix. In both modes, after the kill is archived, run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit so the board drops the killed change before looping back to Step 1.
- Design **FUNDAMENTALLY invalidated** (not just scope-adjustable) → STOP and escalate to the human. This skill cannot re-brainstorm alone; re-brainstorming is a human act handled by `superpowers:brainstorming` + `docket-new-change`.

### Step 4 — Worktree + plan

CONFIRM the step-3 reconcile push has landed on the **metadata branch** before continuing — by **SHA-compare**, not "the push exited 0": after a fetch, the local metadata tip must equal the remote tip. In `docket`-mode: `git -C .docket fetch origin docket` then assert `git -C .docket rev-parse @ == git rev-parse origin/docket`; in `main`-mode: `git fetch origin` then assert the primary tree's tip equals `origin/<integration_branch>`. If they differ (push was rejected by a concurrent writer): re-sync (`pull --rebase`), re-push, re-fetch — loop until the SHAs match BEFORE continuing, so the build never reads bytes older than origin. Then cut the feature branch — **ALWAYS from `origin/<integration_branch>`**, in both modes (fetch it first for freshness):

```
git fetch origin <integration_branch>
git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>
```

`metadata_branch` only redirects bookkeeping commits — it NEVER determines where code branches start. The reconciled **spec is read from the metadata working tree** (in `docket`-mode, `.docket/docs/superpowers/specs/…`); **re-sync `.docket/` immediately before reading it** (`git -C .docket pull --rebase origin docket`) so it matches `origin/docket`. Run `superpowers:writing-plans`: it performs an intentional **cross-tree** step — reading the spec from the metadata working tree (`.docket/` in `docket`-mode) and writing the plan into `.worktrees/<slug>` (`docs/superpowers/plans/` ON THE FEATURE BRANCH). The feature tree never carries the spec — code branches from `origin/<integration_branch>`, the spec lives on `metadata_branch`, so the spec read is deliberately cross-tree, not a bug. Record the plan path in `plan:` — this is a **metadata edit made in the metadata working tree on `metadata_branch`** (the change file is never edited in the feature worktree). The plan **file** that `writing-plans` produces is the feature-branch/build-time artifact and merges to the integration branch with the code, so the `plan:` link resolves on `<integration_branch>` only after the PR merges (which is why `docket-status` ignores a missing `plan:` on an `implemented` change).

### Step 5 — Build

`superpowers:subagent-driven-development` executes the plan task-by-task with TDD + per-task review.

### Step 6 — Review + ADRs

`superpowers:requesting-code-review` (whole-branch). For any non-obvious decision made during implementation, invoke `docket-adr` to record it (it assigns the number + updates the index); append the returned number to the change's `adrs:`. Update `adrs:` in the **metadata working tree on `metadata_branch`** (in `docket`-mode, `.docket/` on `origin/docket`) — never in the feature worktree (the change file is metadata; the same discipline as step 7). (`docket-adr` itself already commits the new ADR file on `metadata_branch` and, on acceptance, publishes it onto the integration branch.)

### Step 6.5 — Results close-out (optional)

Write a results file ONLY if at least one is true: **(a)** the human must run interactive/manual checks at the merge gate beyond automated tests, **(b)** the build surfaced findings worth recording (including any that became ADRs), or **(c)** there are follow-ups or notable plan deviations to capture. Otherwise SKIP it — the PR description + green CI are the receipt.

When warranted: author `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` from `results-template.md` **IN THE FEATURE WORKTREE** and commit it on `feat/<slug>` with the code — it is a build artifact, like the plan. Keep build-receipt detail (what shipped, full test tables) in the PR description, not here. The `results:` FIELD is set in the metadata working tree in step 7 (the file is feature-branch, the field is metadata — same split as `plan:`).

### Step 7 — PR + stop

Invoke `superpowers:finishing-a-development-branch`, DIRECTED to: push the feature branch and open a PR — do NOT merge — then stop. Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics.

Then, BACK IN THE **METADATA WORKING TREE** (in `docket`-mode, `.docket/`), set `status: implemented` + `pr:` (and `results:` if a results file was written in step 6.5) and commit + push on `metadata_branch` (in `docket`-mode, `origin/docket`) — NEVER in the feature worktree (metadata always lands on `metadata_branch`; this is also what lets the sweep read `pr:`). Then run the Board pass (best-effort — see *Best-effort board refresh*) as a separate commit, so the board shows the change as `implemented` — needs your merge.

**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.

### Best-effort board refresh

The Board pass this skill runs after its own status writes (claim, reconcile-kill, `implemented`) is **best-effort**: attempt the regen + push with bounded retries, then **log and continue** — never abort the build for it. The build's correctness rests on the change-file CAS, not the board; any residual staleness self-heals at the next must-land Board pass (the next change's Step 0 `docket-status`, a manual `docket-status`, or finalize). The board is always a **separate commit** from the `status:` write (keeping the claim CAS byte-identical across concurrent agents).

## The reconcile pass and the `reconciled` flag

This is docket's quiet superpower.

### Why it exists

A change is drafted against a *snapshot*: the codebase, the ADRs, and the other in-flight changes as they stood on the brainstorm day. In an async backlog the world moves — other changes ship, ADRs land, interfaces shift, constraints emerge. Most backlog systems build the stale ticket as written and discover the mismatch mid-implementation. Reconcile is the antidote, run at the **last responsible moment**: right before the worktree is created, after claim (the change is ours) but before planning (nothing is committed to yet). Reconciling earlier would just go stale again; reconciling later wastes plan and build work.

### The `reconciled` flag semantics

`reconciled: false` at birth. Set to `true` only after the reconcile pass completes and commits.

It is two things:

1. **An audit signal** — paired with the dated `## Reconcile log` entry, it proves the change was freshened against current reality before implementation began. The log entry records what was dropped, adjusted, or folded in.

2. **A resume-safety guard** — on any resume of an `in-progress` change, check `reconciled`. If still `false`, reconcile didn't finish (crash, interruption) → re-run the full pass before continuing. On ANY resume, also re-run reconcile if `origin/<integration_branch>` has advanced since the last pass (idempotent, non-interactive; must always reflect the last responsible moment).

`reconciled` is **NOT a selection criterion**. Build-readiness is determined by `spec:`-or-`trivial: true` and satisfied `depends_on`. A change sitting at `reconciled: false` is still build-ready; reconcile happens in step 3, after selection and claim.

## Branch & metadata discipline

### The one-line rule

New change ⇒ `git worktree add .worktrees/<slug> -b feat/<slug> origin/<integration_branch>` — in BOTH modes (`metadata_branch: docket` and `metadata_branch: main`). `metadata_branch` only redirects bookkeeping commits; it never determines where code branches start.

### Feature branch invariants

The feature branch is cut from `origin/<integration_branch>` AFTER claim + reconcile, adds only plan + results + code, and **never modifies** docket metadata (the change file, `BOARD.md`, ADRs). This means: at merge, the 3-way merge takes the integration branch's side for the change file unconditionally — the feature branch's copy equals the base at that path, so there is no conflict and no revert needed. (In `docket`-mode the change file does not even exist on `origin/<integration_branch>` until terminal-publish copies it there, so the feature branch never touches it.)

### Metadata vs. code commit separation

- **Metadata commits** (claim, reconcile, `status: implemented`, `pr:`, and all change-file field updates including `plan:` and `adrs:`) happen in the **metadata working tree** on `metadata_branch` — the `.docket/` worktree in `docket`-mode, the primary working tree on the integration branch in `main`-mode.
- **Code and plan/results file commits** happen in the **feature worktree** (`.worktrees/<slug>`) on `feat/<slug>`. The worktree commits the plan + results *files* + code; the change file's `plan:`/`results:` *fields* (like all change-file fields) are written on `metadata_branch` in the metadata working tree — the manifest comments on `plan:`/`results:` say exactly this.

Never cross these streams. A metadata write that lands in the feature worktree would either be silently diverged at merge or create a conflict. (The single deliberate cross-tree touch is the spec **read** in step 4: `writing-plans` reads the spec from the metadata working tree and writes the plan into the feature worktree — a read across trees, never a metadata write into the feature tree.)

### Invocation-branch-agnostic

This skill is safe to invoke from any branch. It `git fetch`es and operates against `origin/<integration_branch>` (feature base) and `metadata_branch` (bookkeeping, via the metadata working tree) explicitly; the branch the human happened to be on when they typed the command is irrelevant.
