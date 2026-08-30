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

- `docket-implement-next` calls this at step 6 whenever a non-obvious technical decision is made during implementation; a human calls it directly for any uncaptured decision.
- You need to supersede or reverse an existing ADR (a new decision replaces an old one).
- The ADR index (`docs/adrs/README.md`) is stale or needs validation, or you want to audit the ledger for gaps, dangling links, or status inconsistencies.

## Convention (load first — blocking)

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *Step-0 preamble* (load the convention; `docket.sh preflight` as its own Bash call; read the printed `KEY=value` block; act on the verdict). Everything below uses its vocabulary without redefinition. All ADR reads and writes land in the metadata working tree on `metadata_branch`, pushed to its remote immediately.

## Actions

### Create

Build the ADR as a JSON request and hand it to the record transaction on stdin:

```
docket adr record --request -
```

The request object (`ADRRecordRequest`, decoded with `DisallowUnknownFields` — an unknown key is rejected) carries:

- `request_id` — a caller-chosen idempotency key (the transaction is safe to retry under the same key).
- `title` — the decision's title.
- `context`, `decision`, `consequences`, `alternatives` — the ADR body sections.
- `relates_to` — an array of existing ADR ids this decision relates to (optional; each id must resolve or the transaction refuses with `adr-dangling-reference`).
- `change` — the producing change as `{id, path, version}` (optional; supply it when a change produces this ADR, e.g. `docket-implement-next` step 6).

One validated transaction lands atomically, in a single metadata commit: the next ADR number allocated (max `id:` + 1, 4-digit zero-pad `0024-…`), the new `<NNNN>-<slug>.md` record (`status: Accepted`, today's UTC `date:`, the optional `change:` back-link), graph validation, the re-rendered `<adrs_dir>/README.md` index, and — when `change` is supplied — that change's `adrs:` append and re-rendered `## Artifacts` block. There is **no** separate index commit and **no** hand-allocation, template-write, or manual compare-and-swap-rename step: a typed conflict or refusal returns without writing anything, so re-read and retry the operation rather than patching the tree by hand.

**Return the number** — read the allocated ADR id from the operation's result envelope so the caller (e.g. `docket-implement-next` step 6) can cite it in the change's `adrs:` field.

**Publish on acceptance (deferred)** — the ADR and its index live on `metadata_branch` (`docket`). terminal publication is deferred from Go v1 — integration-branch publication of ADR bytes is not performed, so neither a change-tied nor a standalone `Accepted` ADR is copied to the integration branch; the decision ledger lives on `docket`. An ADR already published on the integration branch by an earlier tool version stays there as history (the `adr-unpublished` health check keeps the drift visible). See *How an ADR reaches the integration branch* below.

### Supersede / reverse

Never edit an `Accepted` ADR's body. To replace a decision, hand the replace transaction a JSON request on stdin:

```
docket adr supersede --request -
```

(or `docket adr reverse --request -` to reverse rather than supersede). The request object (`ADRReplaceRequest`, `DisallowUnknownFields`) carries:

- `request_id` — the caller-chosen idempotency key (the outer key governs; the `successor`'s own `request_id` is ignored).
- `target` — the ADR being replaced, as `{id, path, version}`. The target must be `Accepted`, else the transaction refuses.
- `successor` — the new ADR, as a full record request (the same fields as *Create*'s `ADRRecordRequest`; give it its own producing `change` if one exists).

One transaction lands atomically: the new ADR (carrying its `supersedes:`/`reverses:` edge to the old one), the old ADR's `status:` line flipped to `"Superseded by ADR-NN"` / `"Reversed by ADR-NN"` (its frozen body otherwise byte-for-byte unchanged — that status value is the **only** change to the old file), and the re-rendered index. There is no separate index commit. In the index the old ADR's row shows its `Superseded by ADR-NN` / `Reversed by ADR-NN` status, and the new ADR's row (in the Active group) shows `→ supersedes ADR-NN` / `→ reverses ADR-NN`. A typed conflict or refusal returns without writing — re-read and retry rather than hand-editing. The status flip lands on `metadata_branch` with the re-rendered index; terminal publication is deferred from Go v1, so the flipped ADR is **not** re-published to the integration branch. A previously published copy of the old ADR remains as history — the `adr-unpublished` health check keeps that drift visible.

### Update note

For a non-reversing material change in context — where the decision still stands but important surrounding information has changed — append a dated `## Update` section to the ADR body. The `## Decision` section itself is never edited. Commit the updated ADR file in `.docket/` and push `origin/docket`; regenerate the index only if the update changes how the entry reads in the index. terminal publication is deferred from Go v1, so an already-published ADR is not re-published; its integration-branch copy stays as history (the `adr-unpublished` health check surfaces the drift).

## How an ADR reaches the integration branch (deferred)

ADRs and their index are authored and live on `metadata_branch` (`docket`). terminal publication is deferred from Go v1 — integration-branch publication of ADR bytes is not performed: none of the three historical cases — a change-tied ADR on its change's terminal transition, a standalone ADR on acceptance, or a status flip to an already-published ADR — copies ADR bytes onto the integration branch. The `Accepted` decision ledger lives on `docket` only.

Records already published onto the integration branch by an earlier tool version are left untouched as history: a status flip to such an ADR leaves the previously published copy in place, and the `adr-unpublished` health check keeps that drift visible (the marker is *read*; acting on it is deferred). The frozen Bash publisher is not a supported fallback, and an enabled `terminal_publish:` key activates nothing.

In `main`-mode the metadata working tree *is* the integration branch, so writing the ADR there is itself the record — there is nothing to publish and nothing deferred.

### Index / validate

Every ADR transaction (record / supersede / reverse) re-renders `<adrs_dir>/README.md` atomically inside its own commit, so there is **no follow-up index render** after an ADR operation — the index is current the instant the transaction lands.

The standalone renderer survives only for **stale-index repair (no transaction in flight; no Go verb — frozen)** — regenerating a `README.md` that drifted out of band (a hand-edit, an interrupted legacy write) when no ADR transaction is running. Invoke the deterministic generator — never hand-render it:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-adr-index --adrs-dir <metadata tree>/<adrs_dir> > <metadata tree>/<adrs_dir>/README.md
```

In `docket`-mode the metadata tree is `.docket/`, so: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-adr-index --adrs-dir .docket/<adrs_dir> > .docket/<adrs_dir>/README.md` (contract: `scripts/render-adr-index.md` — the grouping, ordering, annotations, and determinism). Commit the regenerated index and push `origin/docket`. On a git conflict on the index, **re-run the script** rather than hand-merging (the regenerate-don't-3-way-merge rule).

Validate the ledger by invoking the checker and surfacing each finding line:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh adr-checks --adrs-dir <metadata tree>/<adrs_dir>
```

It is warn-only — one finding per line; `--strict` exits 1 for a future CI gate. The checks it runs (numbering gaps, dangling `supersedes:`/`reverses:`/`relates_to:` links, status inconsistencies) and their output format are the contract `scripts/adr-checks.md`.
