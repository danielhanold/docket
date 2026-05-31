---
name: docket-adr
description: Use when recording, superseding, reversing, or indexing an architecture decision (ADR) — capturing why a non-obvious technical decision was made into the immutable docs/adrs ledger, or regenerating and validating the ADR index. Invoked by docket-implement-next, or directly any time a decision must be recorded or changed.
---

# docket-adr — the decision ledger

## Overview

`docket-adr` maintains the project-wide, immutable, numbered record of *why* — the decisions that shaped the codebase. Changes cite ADRs and produce them; ADRs are never archived, rewritten, or moved. Once an ADR is `Accepted` its body is frozen; only its `status:` line ever changes, and that only when a newer ADR supersedes or reverses it.

## When to use

- `docket-implement-next` calls this at step 6 whenever a non-obvious technical decision is made during implementation.
- A human recognizes a decision that should be captured but hasn't been.
- You need to supersede or reverse an existing ADR (a new decision replaces an old one).
- The ADR index (`docs/adrs/README.md`) is stale, missing, or needs validation.
- You want to audit the ledger for gaps, dangling links, or status inconsistencies.

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

Metadata (change file, `BOARD.md`, ADRs) commits to `metadata_branch` (default `main`). **A change's `feat/<slug>` branch is ALWAYS cut from `origin/main`** in both modes — `metadata_branch` only redirects bookkeeping commits, never where code branches start. The feature branch adds only the plan + code and **never modifies** docket metadata.
<!-- docket:convention:end -->

## Actions

### Create

1. **Allocate the next ADR number** — scan the `id:` frontmatter of every file in `<adrs_dir>/`, take max + 1. The filename uses the 4-digit zero-pad: `0024-…`.
2. **Write `<NNNN>-<slug>.md`** from `adr-template.md`: set `status: Accepted`, `date: <UTC today>`, and the optional `change:` back-link to the producing change.
3. **Commit the new ADR file only.** The `README.md` index is regenerated in a separate commit (see Index / validate) — like `BOARD.md`, so two concurrent creates never conflict on the shared index.
4. **On a lost compare-and-swap push** (someone minted the same id first): re-read max id, rename the file to the new `NNNN` and update the `id:` field in the new ADR's frontmatter, re-push.
5. **Return the number** so the caller (e.g. `docket-implement-next` step 6) can cite it in the change's `adrs:` field.

### Supersede / reverse

Never edit an `Accepted` ADR's body. Write a new ADR with `supersedes:` or `reverses:` pointing at the old one. Flip only the old ADR's `status:` line (that is the only change to the old file) to `"Superseded by ADR-NN"` or `"Reversed by ADR-NN"`. Commit the new ADR file and the old ADR's flipped `status:` line together in **one commit**; regenerate the index in a **separate** commit (consistent with Create's separate-index-commit rule). In the index, the old ADR's row shows its `Superseded by ADR-NN` / `Reversed by ADR-NN` status, and the new ADR's row (in the Active group) shows `→ supersedes ADR-NN` / `→ reverses ADR-NN`.

### Update note

For a non-reversing material change in context — where the decision still stands but important surrounding information has changed — append a dated `## Update` section to the ADR body. The `## Decision` section itself is never edited. Commit the updated ADR file; regenerate the index only if the update changes how the entry reads in the index.

### Index / validate

(Re)render `<adrs_dir>/README.md` grouped into three sections: **Active**, **Superseded / Reversed**, and **Deprecated**. Row format examples:

```
## Active
- [ADR-0024](0024-quicklook-interaction-limits.md) — Quick Look interaction limits (Accepted) ← change #4
- [ADR-0027](0027-page-size-and-margins-via-pagedjs.md) — Page size & margins via Paged.js (Accepted) → supersedes ADR-0025

## Superseded / Reversed
- [ADR-0025](0025-pdf-page-size-via-webview-frame.md) — PDF page size via WebView frame (Superseded by ADR-0027)
```

The index is regenerated wholesale (like `BOARD.md`); on a git conflict, regenerate from the ADR files rather than hand-merging.

Validate the ledger and flag:
- **Numbering gaps** — ids that are missing from the sequence.
- **Dangling links** — `supersedes:`, `reverses:`, or `relates_to:` values that reference an id with no corresponding file.
- **Status inconsistencies** — e.g. an ADR whose `status:` says `Superseded by ADR-NN` but no ADR with that number exists, or an ADR that `supersedes:` another without the old ADR's `status:` being updated.
