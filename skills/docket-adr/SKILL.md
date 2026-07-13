---
name: docket-adr
description: Use when recording, superseding, reversing, or indexing an architecture decision (ADR) — capturing why a non-obvious technical decision was made into the immutable docs/adrs ledger, or regenerating and validating the ADR index. Invoked by docket-implement-next, or directly any time a decision must be recorded or changed.
context: fork
agent: docket-adr
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

## Step 0

Run the convention's *Step-0 preamble*: load the convention, resolve config + the bootstrap verdict (`eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"`, fail-closed; act on `BOOTSTRAP`), then ensure + sync the metadata working tree. All ADR reads and writes land in that tree on `metadata_branch`, pushed to its remote immediately — ADR files and the regenerated index in `.docket/docs/adrs/…` on `origin/docket` in `docket`-mode; the primary working tree on `origin/<integration_branch>` in `main`-mode.

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

The rule: **an `Accepted` ADR publishes to the integration branch by default** — the decision ledger is a durable record that belongs with the code (a repo may suppress the copy with `terminal_publish: false`; see the gate at the end of this section). ADRs are authored on `docket`; the copy onto the integration branch goes through the shared terminal-publish procedure (contract: `scripts/terminal-publish.md`) — a `git checkout` copy from `origin/docket`, never a `git merge docket`. Three cases, all reusing that one procedure (do **not** restate its git sequence here):

- **Change-tied ADR** (the common case) — it is in its change manifest's `adrs:`, so the terminal publish copies it on that change's `done` (or `killed`) transition, driven by `docket-finalize-change` / the kill origin. `docket-adr` does nothing extra; the `Accepted` gate at the copy site skips it if it is still `Proposed`/draft.
- **Standalone ADR** (`docket-adr` invoked directly, not tied to an in-flight change) — `docket-adr` publishes it itself: on acceptance it invokes:

  ```
  "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --adr <NN> --integration-branch <integration_branch> --metadata-branch <metadata_branch> --changes-dir <changes_dir> --adrs-dir <adrs_dir> --enabled <terminal_publish>
  ```

  Trust the exit code. Without this, a change-less ADR would be stranded on `docket` and the integration-branch ledger would be silently incomplete.

- **Status change to an already-published ADR** (`Superseded by`/`Reversed by`/`Deprecated`) — whether or not the ADR was originally change-tied, it is re-published by `docket-adr` invoking the same script (trust the exit code):

  ```
  "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/terminal-publish.sh --adr <NN> --integration-branch <integration_branch> --metadata-branch <metadata_branch> --changes-dir <changes_dir> --adrs-dir <adrs_dir> --enabled <terminal_publish>
  ```

  The producing change is long since `done` and can no longer drive the re-publish; `--adr` mode publishes the ADR's current bytes (including a just-flipped `status:` line), which is exactly what the supersede/reverse and deprecate paths need.

All three cases are **gated by `TERMINAL_PUBLISH`** (change 0064): the same `--enabled` flag the close-out passes. In a repo that sets `terminal_publish: false`, the ADR publish is a no-op that exits 0 — the decision ledger lives on `docket` only, and the integration branch receives **no new** ADR files and no index refresh. (The knob is never retroactive: a repo that flips it off mid-life keeps whatever ADRs and index it had already published — they simply stop being added to.) Trust the exit code either way; do not branch on the knob.

In `main`-mode there is no `docket` branch and no terminal-publish — the metadata working tree *is* the integration branch, so writing the ADR there is itself the publish; this whole section is a `docket`-mode-only concern.

### Index / validate

(Re)render `<adrs_dir>/README.md` by invoking the deterministic generator — never hand-render it:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-adr-index.sh --adrs-dir <metadata tree>/<adrs_dir> > <metadata tree>/<adrs_dir>/README.md
```

In `docket`-mode the metadata tree is `.docket/`, so: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-adr-index.sh --adrs-dir .docket/<adrs_dir> > .docket/<adrs_dir>/README.md` (contract: `scripts/render-adr-index.md` — the grouping, ordering, annotations, and determinism). Commit the regenerated index as a **separate commit** (like `BOARD.md`, so concurrent ADR creates never conflict on the shared index) and push `origin/docket`. On a git conflict on the index, **re-run the script** rather than hand-merging (the regenerate-don't-3-way-merge rule).

Validate the ledger by invoking the checker and surfacing each finding line:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/adr-checks.sh --adrs-dir <metadata tree>/<adrs_dir>
```

It is warn-only — one finding per line; `--strict` exits 1 for a future CI gate. The checks it runs (numbering gaps, dangling `supersedes:`/`reverses:`/`relates_to:` links, status inconsistencies) and their output format are the contract `scripts/adr-checks.md`.
