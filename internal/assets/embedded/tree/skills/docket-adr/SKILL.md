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

Invoke the `docket-convention` skill via the Skill tool first — unless already invoked this session — and run its *startup check* (load the convention; run the capability bootstrap; run the `repository.prepare` operation with `--repo-dir <dir> --json` as its own Bash call; validate the protocol-v1 envelope and carry its typed context forward as literals; act on the verdict). Everything below uses its vocabulary without redefinition. All ADR reads and writes land in the metadata worktree (`metadata_worktree_path` in the `repository.prepare` context) on the metadata branch, pushed to the metadata remote (`metadata_remote`) immediately.

## Actions

### Create

Build the ADR as a JSON request and hand it to the record transaction on stdin:

```
adr.record  --request -   # resolve argv from the capability catalog
```

The request object (`ADRRecordRequest`, decoded with `DisallowUnknownFields` — an unknown key is rejected) carries:

- `request_id` — a caller-chosen idempotency key (the transaction is safe to retry under the same key).
- `title` — the decision's title.
- `context`, `decision`, `consequences`, `alternatives` — the ADR body sections.
- `relates_to` — an array of existing ADR ids this decision relates to (optional; each id must resolve or the transaction refuses with `adr-dangling-reference`).
- `change` — the producing change as `{id, path, revision}` (optional; supply it when a change produces this ADR, e.g. `docket-implement-next` step 6).

One validated transaction lands atomically, in a single metadata commit: the next ADR number allocated (max `id:` + 1, 4-digit zero-pad `0024-…`), the new `<NNNN>-<slug>.md` record (`status: Accepted`, today's UTC `date:`, the optional `change:` back-link), graph validation, the re-rendered `<adrs_dir>/README.md` index, and — when `change` is supplied — that change's `adrs:` append and re-rendered `## Artifacts` block. There is **no** separate index commit and **no** hand-allocation, template-write, or manual conflict-checked-rename step: a typed conflict or refusal returns without writing anything, so re-read and retry the operation rather than patching the tree by hand.

**Return the number** — read the allocated ADR id from the operation's result envelope so the caller (e.g. `docket-implement-next` step 6) can cite it in the change's `adrs:` field.

**Where an ADR lives** — the ADR and its index live on the `docket` branch; no ADR, change-tied or standalone, is copied to the integration branch.

### Supersede / reverse

Never edit an `Accepted` ADR's body. To replace a decision, hand the replace transaction a JSON request on stdin:

```
adr.supersede  --request -   # resolve argv from the capability catalog
```

(or the `adr.reverse` operation with `--request -` to reverse rather than supersede). The request object (`ADRReplaceRequest`, `DisallowUnknownFields`) carries:

- `request_id` — the caller-chosen idempotency key (the outer key governs; the `successor`'s own `request_id` is ignored).
- `target` — the ADR being replaced, as `{id, path, revision}`. The target must be `Accepted`, else the transaction refuses.
- `successor` — the new ADR, as a full record request (the same fields as *Create*'s `ADRRecordRequest`; give it its own producing `change` if one exists).

One transaction lands atomically: the new ADR (carrying its `supersedes:`/`reverses:` edge to the old one), the old ADR's `status:` line flipped to `"Superseded by ADR-NN"` / `"Reversed by ADR-NN"` (its frozen body otherwise byte-for-byte unchanged — that status value is the **only** change to the old file), and the re-rendered index. There is no separate index commit. In the index the old ADR's row shows its `Superseded by ADR-NN` / `Reversed by ADR-NN` status, and the new ADR's row (in the Active group) shows `→ supersedes ADR-NN` / `→ reverses ADR-NN`. A typed conflict or refusal returns without writing — re-read and retry rather than hand-editing. The status flip lands on the `docket` branch with the re-rendered index.

### Update note

For a non-reversing material change in context — where the decision still stands but important surrounding information has changed — append a dated `## Update` section to the ADR body. The `## Decision` section itself is never edited. Commit the updated ADR file in the metadata worktree and run `git -C <metadata_worktree_path> push <metadata_remote> HEAD`; regenerate the index only if the update changes how the entry reads in the index.

### Index / validate

Every ADR transaction (record / supersede / reverse) re-renders `<adrs_dir>/README.md` atomically inside its own commit, so there is **no follow-up index render** after an ADR operation — the index is current the instant the transaction lands.

There is no standalone renderer to run by hand: the transactions own the render, and **out-of-band index drift** (a `README.md` that a hand-edit or an interrupted legacy write left stale when no ADR transaction is running) is surfaced and repaired through the typed derived-view path, never a hand-render:

```
repository.check  --json   # resolve argv from the capability catalog
```

surfaces the drift as a structured finding (`adr-index-stale` when the bytes differ deterministically, `adr-index-malformed` when the markers are broken) — never a false "all clean" on a read error. A deterministic `adr-index-stale` finding is `Repairable`; regenerate the drifted index through the authorized mechanical repair — the `repository.repair` operation, which previews the repair set; a human authorizes its `--yes` apply:

```
repository.repair   # resolve argv from the capability catalog; previews only
```

Repair re-renders only the canonical derived bytes it owns (marker order and balance validated first — a malformed index refuses and leaves the file untouched) and never edits an authored ADR body. It re-proves the pinned revision before writing and commits on the `docket` branch. On a git conflict on the index, re-run the repair rather than hand-merging (the regenerate-don't-3-way-merge rule).

Validate the ledger the same way — the `repository.check` operation runs the ADR-ledger consistency findings (numbering gaps, dangling `supersedes:`/`reverses:`/`relates_to:` links, status inconsistencies) alongside the derived-view drift findings, each as one structured finding; a non-`Repairable` finding (illegal ADR evolution, a missing referenced record) is left for manual review.
