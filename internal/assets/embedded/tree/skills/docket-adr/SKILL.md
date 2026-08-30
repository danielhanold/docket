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

**Publish on acceptance (opt-in)** — an `Accepted` ADR belongs with the code, so a repo that opts in with `terminal_publish: true` gets it copied to the integration branch (see *How an ADR reaches the integration branch* below); the default is to leave it on `docket`. A **change-tied** ADR (the common case — invoked by `docket-implement-next` and carrying a `change:` back-link) rides its change's terminal publish and needs no publish here; a **standalone** ADR (this skill invoked directly, no in-flight change) is published by this skill's own ADR-only terminal-publish invocation.

### Supersede / reverse

Never edit an `Accepted` ADR's body. To replace a decision, hand the replace transaction a JSON request on stdin:

```
docket adr supersede --request -
```

(or `docket adr reverse --request -` to reverse rather than supersede). The request object (`ADRReplaceRequest`, `DisallowUnknownFields`) carries:

- `request_id` — the caller-chosen idempotency key (the outer key governs; the `successor`'s own `request_id` is ignored).
- `target` — the ADR being replaced, as `{id, path, version}`. The target must be `Accepted`, else the transaction refuses.
- `successor` — the new ADR, as a full record request (the same fields as *Create*'s `ADRRecordRequest`; give it its own producing `change` if one exists).

One transaction lands atomically: the new ADR (carrying its `supersedes:`/`reverses:` edge to the old one), the old ADR's `status:` line flipped to `"Superseded by ADR-NN"` / `"Reversed by ADR-NN"` (its frozen body otherwise byte-for-byte unchanged — that status value is the **only** change to the old file), and the re-rendered index. There is no separate index commit. In the index the old ADR's row shows its `Superseded by ADR-NN` / `Reversed by ADR-NN` status, and the new ADR's row (in the Active group) shows `→ supersedes ADR-NN` / `→ reverses ADR-NN`. A typed conflict or refusal returns without writing — re-read and retry rather than hand-editing. **Re-publish the status change** to the integration branch via this skill's own ADR-only terminal-publish invocation for the old ADR's file (see below) — its producing change is long since `done` and cannot drive the re-publish; the new ADR publishes the same way (standalone) or via its own change's terminal publish if it is change-tied.

### Update note

For a non-reversing material change in context — where the decision still stands but important surrounding information has changed — append a dated `## Update` section to the ADR body. The `## Decision` section itself is never edited. Commit the updated ADR file in `.docket/` and push `origin/docket`; regenerate the index only if the update changes how the entry reads in the index. If the ADR is already published on the integration branch, re-publish the updated file the same ADR-only way (it is still `Accepted`).

## How an ADR reaches the integration branch

The rule: **an `Accepted` ADR publishes to the integration branch only when the repo opts in** with `terminal_publish: true` — the decision ledger is then a durable record sitting with the code (the default is `false`, which keeps it on `docket`; see the gate at the end of this section). ADRs are authored on `docket`; the copy onto the integration branch goes through the shared terminal-publish procedure (contract: `scripts/terminal-publish.md`) — a `git checkout` copy from `origin/docket`, never a `git merge docket`. Three cases, all reusing that one procedure (do **not** restate its git sequence here):

- **Change-tied ADR** (the common case) — it is in its change manifest's `adrs:`, so the terminal publish copies it on that change's `done` (or `killed`) transition, driven by `docket-finalize-change` / the kill origin. `docket-adr` does nothing extra; the `Accepted` gate at the copy site skips it if it is still `Proposed`/draft.
- **Standalone ADR** (`docket-adr` invoked directly, not tied to an in-flight change) — `docket-adr` publishes it itself: on acceptance it invokes:

  ```
  "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh terminal-publish --adr <NN> --integration-branch <integration_branch> --metadata-branch <metadata_branch> --changes-dir <changes_dir> --adrs-dir <adrs_dir> --enabled <terminal_publish>
  ```

  Trust the exit code. Without this, a change-less ADR would be stranded on `docket` and the integration-branch ledger would be silently incomplete.

- **Status change to an already-published ADR** (`Superseded by`/`Reversed by`/`Deprecated`) — whether or not the ADR was originally change-tied, it is re-published by `docket-adr` invoking the same ADR-only call as the standalone case — `docket.sh terminal-publish --adr <NN> … --enabled <terminal_publish>` — trusting the exit code. The producing change is long since `done` and can no longer drive the re-publish; `--adr` mode publishes the ADR's current bytes (including a just-flipped `status:` line).

All three cases are **gated by `TERMINAL_PUBLISH`** (changes 0064/0084): the same `--enabled` flag the close-out passes. Under the default `terminal_publish: false` the ADR publish is a no-op that exits 0 — the ledger lives on `docket` only (never retroactive: flipping the knob off keeps what was already published; it simply stops being added to). Trust the exit code either way; do not branch on the knob.

In `main`-mode there is no `docket` branch and no terminal-publish — the metadata working tree *is* the integration branch, so writing the ADR there is itself the publish; this whole section is a `docket`-mode-only concern.

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
