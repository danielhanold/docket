---
name: docket-new-change
description: Use when capturing a new unit of planned work (a change, roughly one PR) into the docket backlog — turning an idea into a tracked, build-ready change through up-front design brainstorming, or (opt-in) scanning a project for candidate work into proposed stubs. Interactive; the entry point a human runs to propose work before it is implemented. Writes markdown only — never branches, worktrees, or code.
---

# docket-new-change — the producer (interactive)

## Overview

`docket-new-change` is where the human is in the loop. It turns an idea into a build-ready change by brainstorming the design up front with the human before any implementation begins. It only ever mints new `proposed` ids — scanning the max existing id and incrementing — so it structurally cannot collide with the autonomous implementer. It writes markdown only: a change file, an optional spec, and a refreshed `BOARD.md`. It never branches, creates worktrees, or touches code.

## When to use

- You have a new idea, feature request, or known gap you want to track and eventually build.
- You want to brainstorm and spec out a change before handing it to `docket-implement-next`.
- You want to quickly stub several `proposed` candidates without brainstorming yet (scan mode — opt-in).
- A trivial mechanical change needs to be tracked but has no real design questions (trivial path).

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

**Board refresh on status writes.** Any skill that writes a change's `status:` regenerates `BOARD.md` (the Board pass) in a separate commit immediately after — the board is a derived view and must never trail the change files.

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

## Where everything is read and written

All reads and writes happen in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately so the backlog is reviewable on GitHub and visible to the autonomous implementer. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`); the change, spec, and refreshed `BOARD.md` are committed in `.docket/` and pushed to `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there). The steps below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Brainstorm mode (default)

The default path for any non-trivial new change. Five steps:

1. **Allocate** — sync the metadata working tree (`git -C .docket pull --rebase origin docket`); scan the `id:` frontmatter of EVERY change in `active/` + `archive/` (archive filenames are date-prefixed, so frontmatter is the reliable id source); next id = max + 1; derive slug from title. The id is finalized at the step-5 push (compare-and-swap): if that push to `origin/docket` is rejected because another `docket-new-change` minted the same id first, re-pull → re-read max id → re-allocate, RENAME `active/<id>-<slug>.md` and fix any id-bearing links, then re-push.

2. **Brainstorm** — run `superpowers:brainstorming` WITH THE HUMAN. This is the decision point. STOP AT THE SPEC — do NOT continue to `writing-plans` (that is build-time). The spec is written natively to `.docket/docs/superpowers/specs/…` (on `docket`) and committed to `metadata_branch`; record its path in `spec:`.

3. **Scan related context** — scan neighbouring changes (`active/` + recent `archive/`) and the ADR index to pre-fill `related`, `depends_on`, `adrs`. In practice, do this quick read just *before* step 2 so the brainstorm is informed by neighbouring work; record the resulting `related`/`depends_on`/`adrs` after the design settles.

4. **Draft the change** — write the thin `active/<id>-<slug>.md` from `change-template.md`: frontmatter (`status: proposed`, `spec:`, `created`/`updated` = UTC today (the UTC date of the commit), priority default `medium`) + a PM-altitude why/what/scope body distilled from the brainstorm. Design detail lives in the linked spec, NOT here.

5. **Board, commit & push** — refresh `BOARD.md` (via `docket-status`'s Board pass), commit the change + spec, and PUSH to `origin/docket` (immediately reviewable on GitHub; visible to the autonomous implementer). STOP. Never implements.

## Trivial path

For a small mechanical change with no real design questions: skip the brainstorm, set `trivial: true`, write the change body directly — no spec, still build-ready. It still follows Brainstorm mode's steps 1 (Allocate), 3 (Scan related context), 4 (Draft — but omit `spec:`), and 5 (Board, commit & push) — only step 2 (Brainstorm) is skipped.

## Scan mode (opt-in)

Survey TODOs, deferred changes, known gaps, and the ADR backlog; emit several lightweight `proposed` STUBS in one pass — WITHOUT specs. Scan-stubs are NOT build-ready (no spec, not trivial) — the board calls this state `needs-brainstorm`. They form an "ideas to brainstorm" backlog a later brainstorm pass turns build-ready. Kept opt-in so routine runs don't generate speculative noise. Once all stubs are written, commit them together with a refreshed `BOARD.md` and push to `origin/docket` (same push pattern as Brainstorm mode's step 5, but no spec).

## Proposed-kill sub-path

When a `proposed` change is abandoned (obsolete, decided against, a duplicate) the producer drives it to the `killed` terminal state — this is one of the two kill origins the shared terminal-publish serves (the other is the implementer's reconcile-kill from `in-progress`).

In `docket`-mode: in `.docket/` (synced to `origin/docket` first), set `status: killed`, add a `## Why killed` section, set `updated: <UTC kill date>`, commit and push `origin/docket`. Then run the shared **terminal-publish procedure (the *Terminal publish (docket-mode)* procedure in `docket-finalize-change`)** with outcome `killed` (token `T = <id>`) to publish the terminal record onto the integration branch — its archive move (step 1) does the `active/ → archive/<UTC kill date>-<id>-<slug>.md` rename, and any `Accepted` ADRs already in the change's `adrs:` ride along. Do **not** restate the git sequence here; that procedure is its single source.

In `main`-mode (no `docket` branch / no terminal-publish): do the archive move (`active/ → archive/<UTC kill date>-<id>-<slug>.md`) + `status: killed` + `## Why killed` directly in the metadata working tree (= the integration branch) and push `origin/<integration_branch>` — exactly as the `done` archive degrades in `docket-finalize-change`. The `<UTC kill date>` is the same date used for the `archive/<date>-…` filename prefix.

A `proposed` change never had a feature branch or open PR, so there is nothing to clean up — and usually no plan/results, so the kill publishes only what is on `docket`: the change file, plus its `spec:`/`adrs:` if set. This skill still writes markdown only — the terminal-publish copy touches no code.

In both modes, after the kill is archived, refresh `BOARD.md` via the **must-land Board pass** (a separate commit, same as the create path's step 5) so the killed change leaves the board. terminal-publish copies records to the integration branch but never touches `BOARD.md`, so the board refresh is this skill's responsibility.
