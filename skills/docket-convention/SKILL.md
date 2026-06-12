---
name: docket-convention
description: Use when any docket skill runs — docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, and docket-adr load this first (their blocking Step 0) — or when you need to understand how docket tracks work. The shared contract — .docket.yml configuration, directory layout, the change manifest and lifecycle, ADR format, build-readiness and selection, the bootstrap guard, and the branch model. Pure reference — defines the convention; performs no reads, writes, or git operations.
---

# docket-convention — the shared contract (pure reference)

This skill defines the docket convention and does nothing else: no procedure, no reads or writes, no git. The six operating skills load it as their blocking Step 0 and use its vocabulary without restating it.

## Convention

docket tracks planned work as **changes** — one markdown file each, roughly one PR — and records architecture decisions as **ADRs**. This skill is the single source of the convention; the operating skills (docket-new-change, docket-groom-next, docket-implement-next, docket-status, docket-finalize-change, docket-adr) load it at startup as their blocking Step 0 and never restate it.

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
  LEARNINGS.md            # curated build-loop lessons; harvested at close-out (see "Learnings ledger")
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

### Learnings ledger

`<changes_dir>/LEARNINGS.md` — the project's **build-loop memory**: a curated, hand-edited file of lessons the build loop taught, living on `metadata_branch` only (like `BOARD.md`, it is never published to the integration branch — but unlike the board it is curated prose, never regenerated). Flat dated entries, **newest first**, one to three lines each, with provenance and an actionable phrasing — e.g. `- 2026-06-12 (#12, PR #7) — <what happened, one clause>. Apply: <the rule to follow next time>.`

**Writing.** Entries are added only by the **harvest** at close-out (its procedural single source is the *Harvest learnings* step in `docket-finalize-change`; `docket-status`'s sweep invokes it by reference). Zero entries for a change is normal. Kills are not harvested — `## Why killed` already records the rationale.

**Reading.** `docket-implement-next` reads the ledger at plan time and again at its review step; `docket-groom-next` reads it before a brainstorm. No other skill reads it.

**Distilling.** Append-only until the file exceeds **~300 lines**; the next harvest past the cap also distills — merge near-duplicates and drop entries since promoted to CLAUDE.md or this convention. Distillation is **compression, not destruction**: git history keeps everything dropped. Boundary: the ledger holds lessons for the build loop; durable project conventions belong in CLAUDE.md — promotion removes the entry here.

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
