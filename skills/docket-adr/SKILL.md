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

## Convention (load first — blocking)

Before anything else in this skill, invoke the `docket-convention` skill via the Skill tool — unless it was already invoked earlier in this session and its content is in context. Everything below uses its vocabulary (build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` bootstrap probes, …) without redefinition; no step below is executable without the convention loaded.

## Where ADRs are read and written

All ADR reads and writes happen in the **metadata working tree** on `metadata_branch`, pushed to its remote immediately. In `docket`-mode that tree is the persistent `.docket/` worktree parked on `docket` — ensure it (state-specific create per the convention's Branch model, idempotent) and **sync it to `origin/docket` before any read** (`git -C .docket fetch origin docket && git -C .docket pull --rebase origin docket`); ADR files and the regenerated index in `.docket/docs/adrs/…` are committed and pushed to `origin/docket`. In single-branch/`main`-mode this degrades to the primary working tree on the integration branch (no `.docket/`): `git pull --rebase` and push on `origin/<metadata_branch>` (which equals `origin/<integration_branch>` there). The actions below say "`.docket/`" / "`origin/docket`" for the common (`docket`-mode) case; read those as the metadata working tree / `origin/<metadata_branch>` in `main`-mode.

## Actions

### Create

1. **Allocate the next ADR number** — scan the `id:` frontmatter of every file in `.docket/<adrs_dir>/`, take max + 1. The filename uses the 4-digit zero-pad: `0024-…`.
2. **Write `<NNNN>-<slug>.md`** from `adr-template.md`: set `status: Accepted`, `date: <UTC today>`, and the optional `change:` back-link to the producing change.
3. **Commit the new ADR file only** in `.docket/` and push `origin/docket`. The `README.md` index is regenerated in a separate commit (see Index / validate) — like `BOARD.md`, so two concurrent creates never conflict on the shared index.
4. **On a lost compare-and-swap push** to `origin/docket` (someone minted the same id first): re-read max id, rename the file to the new `NNNN` and update the `id:` field in the new ADR's frontmatter, re-push.
5. **Return the number** so the caller (e.g. `docket-implement-next` step 6) can cite it in the change's `adrs:` field.
6. **Publish on acceptance** — an `Accepted` ADR belongs with the code, so it is copied to the integration branch (see *How an ADR reaches the integration branch* below). A **change-tied** ADR (the common case — invoked by `docket-implement-next` and carrying a `change:` back-link) rides its change's terminal publish and needs no publish here; a **standalone** ADR (this skill invoked directly, no in-flight change) is published by this skill's own ADR-only terminal-publish invocation.

### Supersede / reverse

Never edit an `Accepted` ADR's body. Write a new ADR with `supersedes:` or `reverses:` pointing at the old one. Flip only the old ADR's `status:` line (that is the only change to the old file) to `"Superseded by ADR-NN"` or `"Reversed by ADR-NN"`. Commit the new ADR file and the old ADR's flipped `status:` line together in **one commit** in `.docket/` and push `origin/docket`; regenerate the index in a **separate** commit (consistent with Create's separate-index-commit rule). In the index, the old ADR's row shows its `Superseded by ADR-NN` / `Reversed by ADR-NN` status, and the new ADR's row (in the Active group) shows `→ supersedes ADR-NN` / `→ reverses ADR-NN`. **Re-publish the status change** to the integration branch via this skill's own ADR-only terminal-publish invocation for the old ADR's file (see below) — its producing change is long since `done` and cannot drive the re-publish; the new ADR publishes the same way (standalone) or via its own change's terminal publish if it is change-tied.

### Update note

For a non-reversing material change in context — where the decision still stands but important surrounding information has changed — append a dated `## Update` section to the ADR body. The `## Decision` section itself is never edited. Commit the updated ADR file in `.docket/` and push `origin/docket`; regenerate the index only if the update changes how the entry reads in the index. If the ADR is already published on the integration branch, re-publish the updated file the same ADR-only way (it is still `Accepted`).

## How an ADR reaches the integration branch

The rule: **an `Accepted` ADR publishes to the integration branch** — the decision ledger is a durable record that belongs with the code. ADRs are authored on `docket`; the copy onto the integration branch goes through the shared **terminal-publish procedure (the *Terminal publish (docket-mode)* procedure in `docket-finalize-change`)** — the same `git checkout origin/docket -- <paths>` copy mechanism, never a `git merge docket`. Three cases, all reusing that one procedure (do **not** restate its git sequence here):

- **Change-tied ADR** (the common case) — it is in its change manifest's `adrs:`, so the terminal publish copies it on that change's `done` (or `killed`) transition, driven by `docket-finalize-change` / the kill origin. `docket-adr` does nothing extra; the `Accepted` gate at the copy site skips it if it is still `Proposed`/draft.
- **Standalone ADR** (`docket-adr` invoked directly, not tied to an in-flight change) — `docket-adr` publishes it itself: on acceptance it runs the procedure's **ADR-only** entry (token `T = adr-<NN>`, copy-set = that single ADR file, **step 1 archive is skipped** — there is no change file) and the integration branch gets the file. Without this, a change-less ADR would be stranded on `docket` and the integration-branch ledger would be silently incomplete.
- **Status change to an already-published ADR** (`Superseded by`/`Reversed by`/`Deprecated`) — whether or not the ADR was originally change-tied, it is re-published by `docket-adr`'s **own ADR-only** invocation (`T = adr-<NN>`, copy-set = that one file), because its producing change is long since `done` and can no longer drive a publish.

In `main`-mode there is no `docket` branch and no terminal-publish — the metadata working tree *is* the integration branch, so writing the ADR there is itself the publish; this whole section is a `docket`-mode-only concern.

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
