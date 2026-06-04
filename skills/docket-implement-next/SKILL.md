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

<!-- docket:convention:begin -->
## Convention

docket tracks planned work as **changes** — one markdown file each, roughly one PR — and records architecture decisions as **ADRs**. This block is the shared contract every docket skill embeds. It is kept byte-identical across the five skills by `sync-convention.sh` (canonical source: `docket-new-change/SKILL.md`); never hand-edit it in a non-canonical skill.

### Configuration — `.docket.yml` (optional, committed on the default branch)

Read at startup by every docket skill. Absent ⇒ all defaults. It is **committed** (never gitignored), because it governs cross-agent coordination and must be identical for every clone, agent, and device.

```yaml
# .docket.yml — committed on the repo's DEFAULT branch (origin/HEAD); read by every docket skill at startup
metadata_branch: docket      # docket (default) | main  — where PM commits land (see "Branch model")
integration_branch: auto     # auto (→origin/HEAD, fallback main) | main | develop  — where code lands; feature branches cut from origin/<this>
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
results_dir: docs/results    # default  — close-out 'results' artifacts (build-time files, like plans)
```

`.docket.yml` lives on the repo's **default branch (`origin/HEAD`)**, NOT on the integration branch — `integration_branch` is a value *read from* the file, so the file cannot be located *by* it. The default branch is discoverable with zero prior config, but `origin/HEAD` is not reliably populated, so skills **repair it first**: `git remote set-head origin -a`, then resolve `git symbolic-ref refs/remotes/origin/HEAD`. Read config authoritatively via `git show origin/HEAD:.docket.yml` (after a fetch); the working-tree copy is trusted only on the default branch's *primary* checkout. **A ref-unresolvable `origin/HEAD` ≠ a file-absent default branch:** if `origin/HEAD` resolves but the file is genuinely absent ⇒ defaults apply (`metadata_branch: docket`, `integration_branch: auto`); if `origin/HEAD` is unresolvable or `origin` is unreachable ⇒ do **not** assume defaults (abort with a clear error, keying on the `set-head`/fetch return code, never on `git show` — a cached `origin/HEAD` lets `git show` succeed with stale bytes). The file then **declares `integration_branch`**, which may differ from the default branch (default `main`, integration `develop`). `metadata_branch` resolves where PM commits land; `integration_branch` (default `auto` → `origin/HEAD`, fallback `main`; explicit `main`/`develop` verbatim) resolves where code lands — feature branches always cut from `origin/<integration_branch>`. **Backward-compatible opt-out:** pinning `metadata_branch: main` (with `integration_branch: main`) reproduces today's single-branch behavior exactly — no `docket` branch, no `.docket/` worktree.

### Directory layout (paths relative to the configured knobs)

```
<changes_dir>/            # default docs/changes/
  active/                 # every NON-terminal change:   <id>-<slug>.md            (id zero-padded to 4 digits)
  archive/                # the two terminal outcomes:    <YYYY-MM-DD>-<id>-<slug>.md
  BOARD.md                # generated board (NEVER hand-edited); spans active + archive
  README.md               # small static blurb linking to BOARD.md (NOT generated)
<adrs_dir>/               # default docs/adrs/  — flat; ADRs are NEVER archived
  <NNNN>-<slug>.md        # immutable once Accepted (only its status: line ever changes)
  README.md               # generated ADR index
<results_dir>/            # default docs/results/  — optional close-out artifacts (feature-branch build files; NEVER archived)
  <YYYY-MM-DD>-<slug>-results.md
```

The `archive/` filename date prefix is **UTC**: the **merge commit's** date for `done`, the **kill commit's** date for `killed`.

In `docket`-mode all of the above lives on the `docket` branch, written through a persistent, gitignored **`.docket/` metadata worktree** parked on that branch (it is `.docket/`, **not** under `.worktrees/`, to avoid slug collisions and the ephemeral-worktree prune blast radius — see "Branch model").

### Change manifest (frontmatter at the top of each change file)

```yaml
---
id: 7                     # integer; zero-padded to 4 digits in the filename
slug: quicklook-interactions
title: Quick Look interactions — external links + local images
status: proposed          # proposed | in-progress | blocked | deferred | implemented | done | killed
priority: medium          # low | medium | high | critical   (default: medium)
created: 2026-05-30
updated: 2026-05-30
depends_on: [4]           # change ids that must reach `done` (PR merged) first
related: [4, 6]           # cross-links the reconcile pass reads
adrs: [24]                # ADRs this change cites or produces
spec:                     # superpowers design doc path; set at brainstorm (propose) time, on metadata_branch
plan:                     # plan FILE lives on the feature branch; this FIELD is set in the main tree at build time
results:                  # results FILE on the feature branch; this FIELD set in the main tree at close-out (optional)
trivial: false            # true = no spec needed (small mechanical change); still build-ready
branch:                   # planned feat/<slug> name, set on claim; branch itself created at build (step 4)
pr:                       # set when the PR is opened
blocked_by:               # free text; set only when status: blocked
reconciled: false         # set true after the just-in-time reconcile pass
---
```

### Change body sections

- `## Why` — the motivation, as detailed as warranted (no length limit).
- `## What changes` — scope of the work.
- `## Out of scope` — explicit non-goals.
- `## Open questions` — unknowns to resolve during reconcile/design.
- `## Reconcile log` — dated entries appended by the implementer's reconcile pass.
- `## Why deferred` / `## Why killed` — added when entering those states.

The change body is a **PM-altitude proposal** (intent + scope). Detailed design lives in the linked superpowers spec; the task breakdown in the linked superpowers plan. Different zoom levels, no duplication.

### ADR file (`<adrs_dir>/<NNNN>-<slug>.md`)

```yaml
---
id: 24                    # integer; zero-padded to 4 digits in the filename
slug: quicklook-interaction-limits
title: Quick Look interaction limits under sandbox
status: Accepted          # Accepted | Superseded by ADR-NN | Reversed by ADR-NN | Deprecated
date: 2026-05-20
supersedes: []            # ADR ids this replaces (sets the old one's status)
reverses: []              # ADR ids this undoes
relates_to: [22]          # cross-links
change: 4                 # back-link: the change that produced this decision, if any
---

## Context       — the forces / problem that prompted the decision
## Decision      — what was chosen, and the rule a reader needs to know
## Consequences  — what it enables, what it costs, what is given up
```

An `Accepted` ADR is immutable except its `status:` line; a non-reversing context change is appended as a dated `## Update` note, never an edit to the decision. A reversal/supersession is always a **new** ADR.

### Lifecycle — seven states

```
                         ┌──────────────── deferred ──────────────┐
                         │ (conscious shelve; revive → proposed)   │
                         ▼                                          │
  proposed ──claim──▶ in-progress ──PR open──▶ implemented ──merge+sweep──▶ done
     │                    │                                                  (archive/)
     │                    └──blocker──▶ blocked ──clears──▶ in-progress
     │
     └──── killed (obsolete — from proposed, or from in-progress via reconcile; → archive/) ────▶
```

| status | meaning | directory |
|---|---|---|
| `proposed` | drafted, awaiting work | `active/` |
| `in-progress` | claimed, being built | `active/` |
| `blocked` | external blocker (`blocked_by:`) | `active/` |
| `deferred` | consciously shelved, may revive | `active/` |
| `implemented` | built, PR open — **human merge gate** | `active/` |
| `done` | PR merged, filed away (happy terminal) | `archive/` |
| `killed` | abandoned — obsolete or never shipped (sad terminal) | `archive/` |

**Rules.** `active/` holds every non-terminal status; `archive/` holds the two terminal outcomes. The single physical move (`active/ → archive/`, date-prefixed) happens once on the terminal transition and is **idempotent**: re-pull, re-read `status` on `metadata_branch`, no-op if already terminal. `deferred` may be entered from `proposed` or `in-progress` (add `## Why deferred`) and revived to `proposed`; clearing a blocker or reviving is a one-line frontmatter edit, no move. A change whose `depends_on` is unsatisfied is *implicitly* blocked — the selector skips it (no status change) and the board shows it **waiting on #N**. A dependency is **satisfied when it reaches `done`**. If `#N` is still `implemented` (PR open, unmerged), the dependent is gated on a human merge — the board flags **waiting on #N — needs your merge**, distinct from **waiting on #N — not yet built**. Reserve explicit `blocked` for external blockers the system can't infer.

### Build-readiness & selection (shared definition)

A change is **build-ready** — eligible for `docket-implement-next` — only when it is `proposed`, has a `spec:` **or** `trivial: true`, and all `depends_on` are satisfied (`done`). A `proposed` change with neither a spec nor `trivial: true` is **needs-brainstorm** (not build-ready). The implementer's deterministic selection order is `priority` (`critical` > `high` > `medium` > `low`) → age (`created`) → **lowest `id`**.

### Bootstrap guard (`docket`-mode first-run safety)

At startup, after resolving config, when `metadata_branch == docket`, fetch origin and evaluate a 2×2 over two probes (both stated over the **same vocabulary**):

- **`DOCKET`** = the `docket` branch exists (origin OR local): `git rev-parse --verify --quiet refs/remotes/origin/docket || git rev-parse --verify --quiet refs/heads/docket`.
- **`LIVE`** = the **live planning surface** still sits on the integration branch: `git ls-tree origin/<integration_branch> -- <changes_dir>/active <changes_dir>/README.md BOARD.md` (non-empty ⇒ present). Probe **only** this pruned surface — `archive/`, `<adrs_dir>/`, and pre-migration specs deliberately *stay* on integration, so probing them would read `LIVE` forever. Use `git ls-tree`, never bare `<ref>:<path>`. If `origin/<integration_branch>` does not resolve, `ls-tree` exits ≠0 with empty stdout — treat that as a **hard config error**, not `¬LIVE`.

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to `migrate-to-docket.sh`; never auto-create or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | **half-migrated** (interrupted run) → **STOP**, point back to `migrate-to-docket.sh` to finish its prune | migrated → **proceed** |

The guard is a no-op in `main`-mode (`DOCKET`/`LIVE` are only evaluated when `metadata_branch == docket`). The migration itself lives in the standalone `migrate-to-docket.sh`.

### Branch model

Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — which is the **primary working tree on the integration branch** in single-branch (`main`) mode, and the persistent **`.docket/` worktree** in `docket`-mode — and is **always pushed to its remote immediately** (a local-only orphan branch defeats the purpose; the backlog, board, specs, and ADRs stay browsable on the remote at all times). A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** (default trunk `main`; `develop` for GitFlow) — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata. On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal-publish** procedure: it **copies** the change's terminal records (the archived change file + its `spec:` if set + the **`Accepted`** ADRs in `adrs:`) from `origin/docket` onto the integration branch in one dedicated commit and pushes (`git checkout origin/docket -- <paths>`, never a `git merge docket`) — the only flow of metadata onto the code line. (In `main`-mode the metadata working tree *is* the integration branch, so terminal-publish is skipped — the archive move there is itself the terminal record.)
<!-- docket:convention:end -->

## Procedure

### Step 0 — Sync & sweep

`git pull --rebase`; invoke `docket-status` (whose merge-sweep pass archives any `implemented` change whose PR has merged) before selection — the self-cleaning safety net for changes not closed via `docket-finalize-change`.

### Step 1 — Select

Among `active/` changes that are `proposed`, BUILD-READY (have a `spec:` or `trivial: true`), and have all `depends_on` satisfied (satisfied = `done`), rank by priority (`critical` > `high` > `medium` > `low`) → age (`created`) → LOWEST `id` (the final deterministic tie-break, so two implementers — if ever run concurrently — converge on the same winner and never claim the same change). Pick the top, or accept an explicit id passed by the caller. Skip `in-progress`, `blocked`, `deferred`, and not-build-ready stubs.

### Step 2 — Claim (compare-and-swap)

Re-read the manifest after the pull; if still `proposed`, set `status: in-progress` + `branch: feat/<slug>` + `updated: <UTC today>`; commit and push on `metadata_branch`. On a non-fast-forward rejection: DISCARD the pending local claim commit (it edits the same `status:`/`branch:` lines and would conflict on replay), `git pull --rebase`, RE-READ (mandatory); if still `proposed`, re-claim and push — LOOP until the push lands (it can be rejected repeatedly under load). The arbiter is the re-read (abort if no longer `proposed`), not that any single push succeeds. No worktree yet.

> Two agents must NOT share one local clone — each needs its own.

### Step 3 — Reconcile ⭐

Re-read the change + its spec against `related` + recently-archived changes, cited + recent ADRs, and CURRENT code; refresh the change body and spec to what is true NOW (drop work done elsewhere, adjust scope, fold in new constraints), NON-INTERACTIVELY. A trivial change has no spec — refresh the body only. Append a dated `## Reconcile log` entry; set `reconciled: true`; commit and push on `metadata_branch`.

Two escape hatches:

- Change now **OBSOLETE** → set `killed` (+ `## Why killed`) + `updated: <UTC kill date>` (same UTC date used for the `archive/<date>-…` filename prefix), archive, loop back to Step 1.
- Design **FUNDAMENTALLY invalidated** (not just scope-adjustable) → STOP and escalate to the human. This skill cannot re-brainstorm alone; re-brainstorming is a human act handled by `superpowers:brainstorming` + `docket-new-change`.

### Step 4 — Worktree + plan

`git fetch origin`; CONFIRM the step-3 reconcile push has landed on `origin/main` (default mode). If it hasn't (push was rejected): `pull --rebase`, re-push, re-fetch — loop until the reconcile commit is on `origin/main` BEFORE continuing. For `metadata_branch: docket`, this cross-tree confirmation is a v1 rough edge — see the `docket` mode caveat at the end of this skill before proceeding. Then:

```
git worktree add .worktrees/<slug> -b feat/<slug> origin/main
```

The freshly-fetched `origin/main` carries the reconciled spec in default mode. NEVER base on a separate metadata branch; in default mode `metadata_branch` IS `main`, so `origin/main` is the correct base. Run `superpowers:writing-plans` (writes `docs/superpowers/plans/` ON THE FEATURE BRANCH); record the path in `plan:` — this is a **metadata edit made in the main working tree on `metadata_branch`** (the change file is never edited in the feature worktree). The plan **file** that `writing-plans` produces is the feature-branch/build-time artifact and merges to `main` with the code, so the `plan:` link resolves on `main` only after the PR merges (which is why `docket-status` ignores a missing `plan:` on an `implemented` change).

### Step 5 — Build

`superpowers:subagent-driven-development` executes the plan task-by-task with TDD + per-task review.

### Step 6 — Review + ADRs

`superpowers:requesting-code-review` (whole-branch). For any non-obvious decision made during implementation, invoke `docket-adr` to record it (it assigns the number + updates the index); append the returned number to the change's `adrs:`. Update `adrs:` in the **main working tree on `metadata_branch`** — never in the feature worktree (the change file is metadata; the same discipline as step 7). (`docket-adr` itself already commits the new ADR file on `metadata_branch`.)

### Step 6.5 — Results close-out (optional)

Write a results file ONLY if at least one is true: **(a)** the human must run interactive/manual checks at the merge gate beyond automated tests, **(b)** the build surfaced findings worth recording (including any that became ADRs), or **(c)** there are follow-ups or notable plan deviations to capture. Otherwise SKIP it — the PR description + green CI are the receipt.

When warranted: author `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` from `results-template.md` **IN THE FEATURE WORKTREE** and commit it on `feat/<slug>` with the code — it is a build artifact, like the plan. Keep build-receipt detail (what shipped, full test tables) in the PR description, not here. The `results:` FIELD is set in the main tree in step 7 (the file is feature-branch, the field is metadata — same split as `plan:`).

### Step 7 — PR + stop

Invoke `superpowers:finishing-a-development-branch`, DIRECTED to: push the feature branch and open a PR — do NOT merge — then stop. Pre-specifying the outcome keeps it non-interactive while reusing its push/PR mechanics.

Then, BACK IN THE MAIN WORKING TREE, set `status: implemented` + `pr:` (and `results:` if a results file was written in step 6.5) and commit + push on `metadata_branch` — NEVER in the feature worktree (metadata always lands on `metadata_branch`; this is also what lets the sweep read `pr:`).

**STOP.** The change stays in `active/` as `implemented` until a human merges it, or approves `docket-finalize-change` to merge it.

## The reconcile pass and the `reconciled` flag

This is docket's quiet superpower.

### Why it exists

A change is drafted against a *snapshot*: the codebase, the ADRs, and the other in-flight changes as they stood on the brainstorm day. In an async backlog the world moves — other changes ship, ADRs land, interfaces shift, constraints emerge. Most backlog systems build the stale ticket as written and discover the mismatch mid-implementation. Reconcile is the antidote, run at the **last responsible moment**: right before the worktree is created, after claim (the change is ours) but before planning (nothing is committed to yet). Reconciling earlier would just go stale again; reconciling later wastes plan and build work.

### The `reconciled` flag semantics

`reconciled: false` at birth. Set to `true` only after the reconcile pass completes and commits.

It is two things:

1. **An audit signal** — paired with the dated `## Reconcile log` entry, it proves the change was freshened against current reality before implementation began. The log entry records what was dropped, adjusted, or folded in.

2. **A resume-safety guard** — on any resume of an `in-progress` change, check `reconciled`. If still `false`, reconcile didn't finish (crash, interruption) → re-run the full pass before continuing. On ANY resume, also re-run reconcile if `origin/main` has advanced since the last pass (idempotent, non-interactive; must always reflect the last responsible moment).

`reconciled` is **NOT a selection criterion**. Build-readiness is determined by `spec:`-or-`trivial: true` and satisfied `depends_on`. A change sitting at `reconciled: false` is still build-ready; reconcile happens in step 3, after selection and claim.

## Branch & metadata discipline

### The one-line rule

New change ⇒ `git worktree add .worktrees/<slug> -b feat/<slug> origin/main` — in BOTH modes (`metadata_branch: main` and `metadata_branch: docket`). `metadata_branch` only redirects bookkeeping commits; it never determines where code branches start.

### Feature branch invariants

The feature branch is cut from `origin/main` AFTER claim + reconcile, adds only plan + code, and **never modifies** docket metadata (the change file, `BOARD.md`, ADRs). This means: at merge, the 3-way merge takes `main`'s side for the change file unconditionally — the feature branch's copy equals the base at that path, so there is no conflict and no revert needed.

### Metadata vs. code commit separation

- **Metadata commits** (claim, reconcile, `status: implemented`, `pr:`, and all change-file field updates including `plan:`) happen in the **main working tree** on `metadata_branch`.
- **Code and plan file commits** happen in the **feature worktree** (`.worktrees/<slug>`) on `feat/<slug>`. The worktree commits the plan *file* + code; the change file's `plan:` *field* (like all change-file fields) is written on `metadata_branch` in the main tree — the manifest comment on `plan:` says exactly this.

Never cross these streams. A metadata write that lands in the feature worktree would either be silently diverged at merge or create a conflict.

### Invocation-branch-agnostic

This skill is safe to invoke from any branch. It `git fetch`es and operates against `origin/main` (feature base) and `metadata_branch` (bookkeeping) explicitly; the branch the human happened to be on when they typed the command is irrelevant.

### `docket` mode caveat (v1 rough edge)

When `metadata_branch: docket`, code still branches from `origin/main`, but the spec lives on `docket` and must be read cross-tree (`git show docket:docs/superpowers/specs/<file>`), and the reconcile push lands on `docket`. These cross-branch mechanics are **not fully specified for v1** — default `main` mode is the supported path. The caveat is documented here so implementers do not attempt the `docket`-branch path and encounter silent failures.
