---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
---

# docket-finalize-change — close out a change (human)

## Overview

`docket-finalize-change` is the human's deliberate close-out for a change that has reached the merge gate. It mirrors `docket-new-change` — the opening bookend that starts a change's life — by providing the closing bookend that ends it. Rather than waiting for the safety-net merge-sweep that `docket-status` and `docket-implement-next` run in bulk, finalize handles one or more specific changes now: merging if the PR is only approved, archiving the change file, cleaning up the branch and worktree, and refreshing the board. It reuses `docket-status`'s idempotent-archive procedure exactly, so it is safe to run even if the sweep already ran first.

## When to use

- A PR was approved and you want to merge it and close the change in one step.
- A PR was merged via the GitHub button and you want to archive the change immediately rather than waiting for the next `docket-status` or `docket-implement-next` sweep.
- You want to clean up the feature branch and worktree after a merge and refresh the board in one pass.
- You are closing out multiple merged changes at once after a sprint or review cycle.

<!-- docket:convention:begin -->
## Convention

docket tracks planned work as **changes** — one markdown file each, roughly one PR — and records architecture decisions as **ADRs**. This block is the shared contract every docket skill embeds. It is kept byte-identical across the five skills by `sync-convention.sh` (canonical source: `docket-new-change/SKILL.md`); never hand-edit it in a non-canonical skill.

### Configuration — `.docket.yml` (optional, committed at repo root)

Read at startup by every docket skill. Absent ⇒ all defaults. It is **committed** (never gitignored), because it governs cross-agent coordination and must be identical for every clone, agent, and device. It always lives on `main` (it is *not* routed by `metadata_branch`), so any checkout can read it before it knows where metadata lives.

```yaml
# .docket.yml — committed; read by every docket skill at startup
metadata_branch: main        # main (default) | docket  — where PM commits land (see "Branch model")
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
results_dir: docs/results    # default  — close-out 'results' artifacts (build-time files, like plans)
```

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

### Branch model (one-line rule)

Metadata (change file, `BOARD.md`, ADRs) commits to `metadata_branch` (default `main`). **A change's `feat/<slug>` branch is ALWAYS cut from `origin/main`** in both modes — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata.
<!-- docket:convention:end -->

## Selection

Given an explicit change id, OR auto-detect:

- Auto-detect FINALIZES every `implemented` change whose `pr:` is already merged (safe, idempotent), AND
- For any that are only approved-and-mergeable (not yet merged), PROMPT before merging — merging is a deliberate act.

The per-change steps below run for each selected change.

## Per-change steps

**Steps 1–4 run per selected change** (check → verify → archive → clean up), exactly mirroring `docket-status`'s per-change archive loop. **Step 5 (Board) runs once after all selected changes are processed** — it is wholesale and idempotent, so a single regen at the end is correct and avoids redundant regenerations.

1. **Check the PR** (`gh`). Already merged → straight to archive. Approved + mergeable but not merged → MERGE IT (invoking finalize on an **explicit change id** IS the merge decision — the gate is respected; under **auto-detect**, PROMPT first per the Selection rules above before merging), then continue.

2. **Verify the merge landed on main** (optionally: tests green on the merged result).

   > **Close-out (optional).** If the change carries a `results:` file, this is the moment to append interactive-verification **outcomes** and any late findings to it — on `main`, post-merge. The results file is the durable record of what was hand-verified at the gate.

3. **Archive (idempotent):**

   a. `git pull --rebase` on `metadata_branch`; re-read `status`.
      Already `done` (or already under `archive/`) → no-op, continue to the next change.

   b. **Compute the merge date in UTC** — use `gh`'s `mergedAt`, or
      `TZ=UTC git show -s --date=format-local:%Y-%m-%d <merge-sha>`. Never `now()`.

   c. `git mv active/<id>-<slug>.md archive/<merge-date>-<id>-<slug>.md`.

   d. Set `status: done` and `updated: <merge-date>` (the **same** UTC date — never `now()`).

   e. **Commit on `metadata_branch` — the CHANGE FILE ONLY** (`BOARD.md` regen is the separate Board step, so concurrent archivers stay byte-identical). Push; on non-fast-forward, `pull --rebase` and retry.

4. **Clean up** — remove the merged feature branch + worktree (provenance-guarded, like `superpowers:finishing-a-development-branch` — only auto-remove worktrees under `.worktrees/<slug>`).

5. **Board** — regenerate `BOARD.md` (`docket-status`'s Board pass) and commit + push it on `metadata_branch` as a separate commit from the archive commits above.

**Note:** This archive procedure is **identical** to `docket-status`'s merge-sweep archive — same UTC merge date, same change-file-only commit, same idempotency. Both skills describe the same operation; they must not diverge.

## Where finishing-a-development-branch fits

When a human is present, `superpowers:finishing-a-development-branch` can drive a **non-standard close-out** (keep the branch, discard it, or merge locally without a PR) — its merge/keep/discard chooser fits naturally at step 4. docket also borrows its **worktree provenance-guard**: only auto-remove a worktree whose path is under `.worktrees/<slug>` — never remove a worktree outside that known path.

## docket mode caveat

**`docket` mode caveat (v1 rough edge):** finalize spans two branches — merges code into `main` (step 1) but commits the archive to `docket` (step 3), so the `done` state won't appear on `main` until the periodic `docket → main` sync. The bulk sweep (and `docket-implement-next` step 0) remain a self-healing safety net for changes merged via the GitHub button without running this skill.
