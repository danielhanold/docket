---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
---

# docket-finalize-change — close out a change (human)

## Overview

`docket-finalize-change` is the human's deliberate close-out for a change that has reached the merge gate. It mirrors `docket-new-change` — the opening bookend that starts a change's life — by providing the closing bookend that ends it. Rather than waiting for the safety-net merge-sweep that `docket-status` and `docket-implement-next` run in bulk, finalize handles one or more specific changes now: merging the approved PR into the integration branch, then driving the **`done`** terminal transition — archiving the change on `metadata_branch`, publishing its terminal records onto the integration branch (`docket`-mode), cleaning up the branch and worktree, and refreshing the board. It reuses the same idempotent archive-and-publish flow as `docket-status`'s safety-net sweep, so it is safe to run even if the sweep already ran first.

`done` is one of docket's two **terminal transitions** — the other is `killed` (driven by the producer from `proposed`, or the implementer from `in-progress` via reconcile). The publish-to-integration step is a **shared procedure** owned by this skill (see *Terminal publish (docket-mode)* below); the kill origins and `docket-status`'s sweep reference it. This skill is its single source.

## When to use

- A PR was approved and you want to merge it and close the change in one step.
- A PR was merged via the GitHub button and you want to archive the change immediately rather than waiting for the next `docket-status` or `docket-implement-next` sweep.
- You want to clean up the feature branch and worktree after a merge and refresh the board in one pass.
- You are closing out multiple merged changes at once after a sprint or review cycle.

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
- **`LIVE`** = the **live planning surface** still sits on the integration branch: `git ls-tree origin/<integration_branch> -- <changes_dir>/active <changes_dir>/README.md <changes_dir>/BOARD.md` (non-empty ⇒ present). Probe **only** this pruned surface — `archive/`, `<adrs_dir>/`, and pre-migration specs deliberately *stay* on integration, so probing them would read `LIVE` forever. Use `git ls-tree`, never bare `<ref>:<path>`. If `origin/<integration_branch>` does not resolve, `ls-tree` exits ≠0 with empty stdout — treat that as a **hard config error**, not `¬LIVE`.

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | existing single-branch repo → **STOP**, point to `migrate-to-docket.sh`; never auto-create or move data | fresh repo → create the empty orphan `docket`, push, **proceed** |
| **`DOCKET`** | **half-migrated** (interrupted run) → **STOP**, point back to `migrate-to-docket.sh` to finish its prune | migrated → **proceed** |

The guard is a no-op in `main`-mode (`DOCKET`/`LIVE` are only evaluated when `metadata_branch == docket`). The migration itself lives in the standalone `migrate-to-docket.sh`.

### Branch model

Metadata (change files, `BOARD.md`, ADRs, specs) commits to `metadata_branch` (default `docket`) via the **metadata working tree** — which is the **primary working tree on the integration branch** in single-branch (`main`) mode, and the persistent **`.docket/` worktree** in `docket`-mode — and is **always pushed to its remote immediately** (a local-only orphan branch defeats the purpose; the backlog, board, specs, and ADRs stay browsable on the remote at all times). A change's `feat/<slug>` branch is **ALWAYS cut from `origin/<integration_branch>`** (default trunk `main`; `develop` for GitFlow) — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + results + code and **never modifies** docket metadata. On a terminal transition (`done` *or* `killed`), the driving skill runs the shared **terminal-publish** procedure: it **copies** the change's terminal records (the archived change file + its `spec:` if set + the **`Accepted`** ADRs in `adrs:`) from `origin/docket` onto the integration branch in one dedicated commit and pushes (`git checkout origin/docket -- <paths>`, never a `git merge docket`) — the only flow of metadata onto the code line. (In `main`-mode the metadata working tree *is* the integration branch, so terminal-publish is skipped — the archive move there is itself the terminal record.)
<!-- docket:convention:end -->

## Selection

Given an explicit change id, OR auto-detect:

- Auto-detect FINALIZES every `implemented` change whose `pr:` is already merged (safe, idempotent), AND
- For any that are only approved-and-mergeable (not yet merged), PROMPT before merging — merging is a deliberate act.

The per-change steps below run for each selected change.

## Per-change steps

**Steps 1–4 run per selected change** (check → verify → archive → clean up), exactly mirroring `docket-status`'s per-change archive loop. **Step 5 (Board) runs once after all selected changes are processed** — it is wholesale and idempotent, so a single regen at the end is correct and avoids redundant regenerations.

1. **Check the PR** (`gh`). Already merged → straight to archive. Approved + mergeable but not merged → **merge it into the integration branch** — `gh pr merge --merge "$pr" --repo … ` (or the team's merge mode) targeting the change's PR against **`<integration_branch>`** (resolved from `.docket.yml`; default `auto` → `origin/HEAD`, fallback `main`), **not hard-coded `main`** (a GitFlow repo merges into `develop`). Invoking finalize on an **explicit change id** IS the merge decision — the gate is respected; under **auto-detect**, PROMPT first per the Selection rules above before merging. Then continue. (Merging the PR is the only thing that lands plan + results + code on the integration branch — they ride the merge, not the terminal-publish.)

2. **Verify the merge landed on the integration branch** (optionally: tests green on the merged result).

   > **Close-out (optional).** If the change carries a `results:` file, this is the moment to append interactive-verification **outcomes** and any late findings to it — on the integration branch (where the results *file* now lives, via the merge), post-merge. The results file is the durable record of what was hand-verified at the gate.

3. **Archive (idempotent)** — in the **metadata working tree** (the `.docket/` worktree in `docket`-mode; the primary working tree on the integration branch in `main`-mode), synced to its remote first:

   a. `git pull --rebase` on `metadata_branch` (in `docket`-mode, `git -C .docket pull --rebase origin docket`); re-read `status`.
      Already `done` (or already under `archive/`) → no-op, continue to the next change.

   b. **Compute the merge date in UTC** — use `gh`'s `mergedAt`, or
      `TZ=UTC git show -s --date=format-local:%Y-%m-%d <merge-sha>`. Never `now()`.

   c. `mkdir -p <changes_dir>/archive` (git tracks no empty dirs), then `git mv active/<id>-<slug>.md archive/<merge-date>-<id>-<slug>.md`. **Reuse-existing-file idempotency:** first probe for an already-archived file (null-glob-safe, e.g. `find <changes_dir>/archive -name '*-<id>-<slug>.md'`) and reuse that filename rather than recomputing today's date — otherwise an interrupted-then-resumed finalize could mint a second archive file across a day boundary. (This dated-filename + reuse contract is **identical to the terminal-publish step 1 below** and to the kill paths in `docket-new-change` / `docket-implement-next` — only the tree it runs in differs.)

   d. Set `status: done`, write the `results:` link into the manifest if a results file exists (the *file* arrived via the PR merge; this *field* is set in the metadata working tree), and set `updated: <merge-date>` (the **same** UTC date — never `now()`).

   e. **Commit on `metadata_branch` — the CHANGE FILE ONLY** (`BOARD.md` regen is the separate Board step, so concurrent archivers stay byte-identical). Push to `origin/<metadata_branch>` immediately (in `docket`-mode, `origin/docket`); on non-fast-forward, `pull --rebase` and retry.

   This is **step 1 of the terminal publish** below (archive-on-`docket`-first). In `docket`-mode, after archiving, **invoke the *Terminal publish (docket-mode)* procedure with outcome `done`** (token `T = <id>`) to copy the terminal records from `origin/docket` onto the integration branch. In `main`-mode the metadata working tree *is* the integration branch, so this archive commit is itself the terminal record and terminal-publish is **skipped**.

4. **Clean up** — remove the merged feature branch + worktree (provenance-guarded, like `superpowers:finishing-a-development-branch` — only auto-remove worktrees under `.worktrees/<slug>`). Never the `.docket/` metadata worktree.

5. **Board** — regenerate `BOARD.md` (`docket-status`'s Board pass) in the metadata working tree and commit + push it on `metadata_branch` (in `docket`-mode, `origin/docket`) as a separate commit from the archive commits above. The board is the **live planning view and stays on `metadata_branch`** — it is never published to the integration branch.

**Note:** This archive procedure is **identical** to `docket-status`'s merge-sweep archive — same UTC merge date, same change-file-only commit, same reuse-existing-file idempotency, same terminal-publish invocation. Both skills describe the same operation; they must not diverge.

## Where finishing-a-development-branch fits

When a human is present, `superpowers:finishing-a-development-branch` can drive a **non-standard close-out** (keep the branch, discard it, or merge locally without a PR) — its merge/keep/discard chooser fits naturally at step 4. docket also borrows its **worktree provenance-guard**: only auto-remove a worktree whose path is under `.worktrees/<slug>` — never remove a worktree outside that known path.

## Terminal publish (docket-mode)

The shared procedure that copies a change's terminal records from `origin/docket` onto the integration branch. **This skill is its single source** — `docket-status`'s sweep (for `done`), the producer's proposed-kill and the implementer's reconcile-kill (for `killed`), and `docket-adr` (for a standalone or status-changed ADR) all invoke *this* procedure rather than restating it.

It runs on **every** terminal transition. A terminal transition is `done` (driven by this skill — step 3 above, and `docket-status`'s sweep) or `killed` (driven by the killing skill). Two entry shapes, distinguished by a **publish token `T`** used to name the throwaway branch:

- **Change publish** (`done` or `killed`): `T = <id>`. The copy-set is built from the change manifest (below); **step 1 (archive-first) applies**.
- **ADR-only publish** (a standalone or supersession/reversal ADR, from `docket-adr`): `T = adr-<NN>`. The copy-set is the single ADR file; **step 1 is skipped** (there is no change file to archive).

**Skipped entirely in `main`-mode.** It is guarded on `metadata_branch == docket`; in `main`-mode there is no `docket` branch — the metadata working tree *is* the integration branch, so the archive move there (step 3 above, or the kill clauses in `docket-new-change` / `docket-implement-next`) is itself the terminal record. The **archive-move contract is identical in both modes**: the dated `archive/<DATE>-<id>-<slug>.md` filename (UTC merge/kill-commit date, per the convention) and the reuse-existing-file idempotency rule from step 1 below apply to the `main`-mode archive too; only the tree it runs in differs.

### Step 1 — (change publish only) Archive on `docket` first

In `.docket/` (synced to `origin/docket`): `mkdir -p <changes_dir>/archive` (git tracks no empty dirs, so a fresh repo has none), move `active/<id>-<slug>.md` → `archive/<DATE>-<id>-<slug>.md`, set the terminal `status`, and — for `done`, write the `results:` link into the manifest (the same way the build writes `plan:` in `.docket/`; the results *file* arrived via the PR) or — for `killed`, add a `## Why killed` section; then commit + push `origin/docket`. `<DATE>` = the UTC date of **this archive commit** (for `done`, equivalently the merge date).

**Idempotent across re-runs and day boundaries:** first probe (null-glob-safe, e.g. `find <changes_dir>/archive -name '*-<id>-<slug>.md'`) for an existing archive file and reuse that filename rather than recomputing today's date — otherwise an interrupted-then-resumed run could mint a second archive file. **Ordering is load-bearing:** step 3 copies the *archived* path, so it must exist on `origin/docket` first.

(For the `done` path this is the same work as *Per-change steps* §3 above, run in `.docket/`. For a `killed` change there is usually no merged PR, so plan/results may not exist — kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set.)

### Step 2 — Provision a clean integration checkout

Without disturbing the main tree (which never switches branches): a **transient worktree in a temp dir** on a throwaway local branch `pub-<T>` so the push has a real ref to name. Use `-B` (reset-or-create) and prune leaks so the procedure is re-run safe even if a prior run died before teardown:

```bash
pub="$(mktemp -d)/pub"
git worktree prune                                                 # clear any leaked registration
git worktree add -B pub-<T> "$pub" origin/<integration_branch>     # -B: reset/adopt a leftover pub-<T>
```

(Temp-dir location ⇒ no in-repo path, no `.gitignore` entry, no `.worktrees/` slug-collision or prune hazard.)

### Step 3 — Copy the terminal records from `origin/docket`

From the *remote* tip, never the stale local ref, then commit and push with a **fast-forward-or-retry** loop (the integration branch is the most concurrency-exposed write in the design; it gets the same compare-and-swap discipline as `origin/docket`). Assemble the copy-set **as a list, not a fixed command**:

- the **archived change file** — always present, so the list is never empty;
- the `spec:` path — **iff** the manifest's `spec:` is non-empty;
- each `adrs:` entry **whose ADR is `Accepted`** — skip `Proposed`/draft ADRs (the **`Accepted` gate fires here, at the copy site**).

For an ADR-only publish (`T = adr-<NN>`) the list is the single ADR file.

```bash
git -C "$pub" fetch origin docket
git -C "$pub" checkout origin/docket -- "${copyset[@]}"     # copyset built per above; never empty
git -C "$pub" diff --cached --quiet || \
  git -C "$pub" commit -m "docket(<T>): publish terminal record (<done|killed>)"   # or "publish ADR-<NN>"
until git -C "$pub" push origin HEAD:<integration_branch>; do   # CAS retry on non-fast-forward
  git -C "$pub" pull --rebase origin <integration_branch> \
    || { git -C "$pub" checkout origin/docket -- "${copyset[@]}"; git -C "$pub" rebase --continue; }  # same-file race: re-copy authoritative bytes
done
```

**Push `HEAD:<integration_branch>` explicitly** — a bare `git push origin <integration_branch>` from this worktree resolves the *source* to the local `refs/heads/<integration_branch>` (a stale or absent local ref, never the publish commit on `pub-<T>`), silently dropping or rejecting it. The guarded commit (`diff --cached --quiet ||`) makes a no-op re-run safe under `set -e`. `BOARD.md` is **never** published; plan/results/code already arrived via the PR (`done`) or do not exist (`killed`).

### Step 4 — Tear down (force, since the temp tree is disposable)

```bash
git -C "$pub" checkout --detach 2>/dev/null
git worktree remove --force "$pub"
git branch -D pub-<T> 2>/dev/null || true
rm -rf "$(dirname "$pub")"
```

The whole procedure is **re-run safe**: step 1 reuses the existing archive filename; step 2's `-B` + `prune` adopt a leaked branch/registration; step 3's guarded copy+commit is a no-op when bytes already match, and the push loop completes an interrupted push. A sweep that races finalize on the same change is therefore a safe no-op.
