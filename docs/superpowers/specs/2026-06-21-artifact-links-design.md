# Artifact links — a generated link block at the top of every change

**Change:** #35
**Status:** design (brainstorm output; stops at spec per `docket-new-change`)
**Date:** 2026-06-21

## Problem

A change file references its artifacts through bare frontmatter paths — `spec:`, `plan:`,
`results:`, `adrs:`, `pr:`. At a human review stop (spec review after grooming, the merge
gate, results review at close-out) the reviewer has to manually locate each file. This is
made materially worse because the artifacts do not all live on one branch:

- `spec:` and the ADRs in `adrs:` are authored on and permanently live on the **metadata
  branch** (`docket`).
- `plan:` and `results:` live **only on the feature branch** (`feat/<slug>`) while the
  change is being built, then land on the **integration branch** when the PR merges — and
  the feature branch is deleted at finalize.
- `pr:` is already a URL.

Because of this split, a naive relative markdown link from the change file (viewed on
`docket`) 404s for exactly the artifacts that are hardest to find (plan/results, which are
not on `docket`). The change therefore needs a generated, always-current, branch-aware link
section at the top of every change.

## Goal

Add a generated **`## Artifacts`** block to the top of every change body that hyperlinks
directly to the spec, plan, results, ADRs, and PR — each pointing at the branch the artifact
actually lives on, kept current automatically as artifacts come into existence and as the
change moves through its lifecycle.

## Decisions (locked in brainstorm)

1. **Scope** — link spec, plan, results, ADRs (each id in `adrs:`), and the PR. A one-stop
   "everything about this change" index for a reviewer.
2. **Link form** — absolute GitHub blob URLs
   (`https://github.com/<owner>/<repo>/blob/<ref>/<path>`), branch-pinned per artifact.
   On a non-GitHub remote, degrade to the bare code-formatted path (same graceful-drop
   posture as `github-mirror.sh`).
3. **Mechanism** — a new deterministic script renders the block from frontmatter; frontmatter
   stays the single source of truth. This is the ADR-0012 script-vs-model boundary: the model
   sets field *values*, the script renders the derived *view*. No skill hand-edits the block.
4. **Per-artifact ref across the lifecycle** — only plan/results ever re-point (see table);
   spec/ADRs are stable on `docket`.
5. **Placement** — the `## Artifacts` block is the **first body section**, immediately after
   the YAML frontmatter (which carries `title:` and renders as GitHub's top table) and above
   `## Why`. No new `# Title` H1 is introduced.
6. **Rows appear as artifacts are created** — omit-until-set: a field that is unset produces
   no row (a trivial change with no `spec:` simply has no Spec row).

## The `## Artifacts` block

Bounded by HTML-comment markers so the renderer can replace it idempotently without touching
hand-written prose. Layout is a two-column markdown table; rows are omitted until the backing
field is set.

```markdown
## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec    | [2026-06-21-artifact-links-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-artifact-links-design.md) |
| Plan    | [2026-06-21-artifact-links.md](https://github.com/danielhanold/docket/blob/feat/artifact-links/docs/superpowers/plans/2026-06-21-artifact-links.md) |
| PR      | [#44](https://github.com/danielhanold/docket/pull/44) |
| ADRs    | [ADR-0007](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0007-github-board-mirror-boundary.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->
```

The HTML comments are invisible in rendered Markdown; on GitHub the block renders as a clean
table directly under the frontmatter table.

When **no** artifact field is set yet (e.g. a fresh scan-stub), the block still exists but
contains only the marker pair and the heading — nothing to link. (This keeps the marker in
place so later renders are pure replacements.)

## Per-artifact branch ref

| Artifact | Ref the URL is pinned to | Re-point over lifecycle? |
|---|---|---|
| Spec    | `metadata_branch` (`docket`) — authored there, never deleted | No |
| ADRs    | `metadata_branch` (`docket`) — same | No |
| Plan    | `branch:` (`feat/<slug>`) while non-terminal → `integration_branch` once `done` | **Yes, at `done`** |
| Results | `branch:` (`feat/<slug>`) while non-terminal → `integration_branch` once `done` | **Yes, at `done`** |
| PR      | the `pr:` URL verbatim | No |

The key simplification: because `spec`/`adrs` live permanently on `docket`, their links are
stable and never re-pointed. Only `plan`/`results` flip, and only at the terminal `done`
transition — exactly when the feature branch is cleaned up and the files have merged to the
integration branch.

ADR link text is `ADR-<NNNN>` (zero-padded), pointing at `<adrs_dir>/<NNNN>-<slug>.md` on
`docket`. The ADR's slug is resolved from the ADR file on `docket` (the renderer has access
to the metadata worktree); if a cited ADR file cannot be found, fall back to link text
`ADR-<NNNN>` pointing at the `<adrs_dir>/` directory listing rather than emitting a broken
deep link.

## The renderer — `scripts/render-change-links.sh`

A new deterministic script, consistent with the existing `render-board.sh`,
`github-mirror.sh`, and the ADR-index renderer.

**Inputs**
- The change file path (operates on one change at a time).
- Resolved config via `docket-config.sh --export` — `METADATA_BRANCH`, `INTEGRATION_BRANCH`,
  `CHANGES_DIR`, `ADRS_DIR`, and the remote owner/repo (reuse `github-mirror.sh`'s remote
  resolution; a non-GitHub or absent remote ⇒ fallback mode).

**Behavior**
1. Parse the change frontmatter (`status`, `branch`, `spec`, `plan`, `results`, `adrs`,
   `pr`) — reuse the existing frontmatter-reading helper the script family already shares.
2. Compute each artifact's URL per the ref table (status-driven for plan/results).
3. Build the table body, omitting unset rows.
4. Replace the content between `<!-- docket:artifacts:start … -->` and
   `<!-- docket:artifacts:end -->`. If the markers are absent (older change file), insert the
   whole `## Artifacts` block as the first body section, immediately after the closing `---`
   of frontmatter and before the first body heading.
5. **Idempotent**: running twice with no field change produces a byte-identical file.

**Fallback (non-GitHub remote)**: render each artifact as a bare, code-formatted path
(`` `docs/superpowers/specs/…md` ``) — the pre-change behavior, just collected in one block.
PR stays a URL (it is one). The block still renders; only the form degrades.

**Output discipline**: the script edits the file in place and is the *sole* writer of the
block. It does not commit — the calling skill commits as part of its existing commit (the
block edit rides with the frontmatter write that triggered it).

## Call sites (every field-writing moment)

The renderer is invoked immediately after any skill writes one of the backing fields:

- `docket-new-change` — after writing `spec:` at draft time (also seeds the empty marker
  block via `change-template.md`).
- `docket-groom-next` — after writing `spec:` (or on the trivial verdict, which has no spec —
  block stays empty until build).
- `docket-auto-groom` — after writing `spec:`.
- `docket-implement-next` — after `plan:` (build), `pr:` (PR open), `adrs:` (ADR cite), and
  `results:` (close-out). Each of these is an existing frontmatter write; the renderer call
  is appended to it.
- `docket-finalize-change` **and** the `docket-status` sweep — at the `done` transition, so
  plan/results re-point from the feature branch to the integration branch. This rides the
  archive/board-refresh step that both already run.

`change-template.md` ships the empty `## Artifacts` block + marker pair as the first body
section so new changes start correctly shaped.

## Edge cases

- **Killed from in-progress** — plan/results may never have merged and the feature branch is
  gone. Render those rows pointing at the PR (`pr:`) if set; otherwise omit them. Spec, ADRs,
  and PR link normally. (Killed-from-proposed never had a plan/results, so omit-until-set
  already covers it.)
- **`trivial: true`** — no spec; the Spec row is simply omitted. Plan/results/PR render once
  they exist.
- **`board_surfaces: []` / offline** — the block lives in the change body, not the board, so
  it always renders; only the link *form* degrades to bare paths off GitHub. Independent of
  any board surface.
- **Older change files without markers** — the renderer inserts the block on first run
  (back-fill happens naturally the next time any field is written; a one-time sweep over
  existing active changes is a follow-up, see Out of scope).

## Testing approach

TDD-style content/invariant assertions (the repo's established pattern for docs+script
changes), driven before the script is written:

- Given fixture frontmatter → exact expected block output (table, omit-until-set, ADR list).
- Idempotency: a second run on rendered output is a byte-for-byte no-op.
- Ref correctness: plan/results point at `feat/<slug>` while `in-progress`/`implemented`, and
  flip to `integration_branch` once `status: done`.
- Marker insertion: a change file lacking markers gets the block inserted as the first body
  section, after frontmatter, before `## Why`.
- Non-GitHub remote → bare-path fallback.
- A sync-style check (mirroring the existing convention/skill-coverage checks) asserting that
  every field-writing skill body invokes `render-change-links.sh`.

## Touch-points

- New: `scripts/render-change-links.sh` + its tests.
- New: marker block in `change-template.md`.
- Edited skill bodies (add the renderer call at each field write): `docket-new-change`,
  `docket-groom-next`, `docket-auto-groom`, `docket-implement-next`, `docket-finalize-change`,
  `docket-status` (sweep).
- The synced convention block — document the `## Artifacts` generated section in
  "Change body sections" and note the renderer in the derived-views/script family.

## Out of scope

- A one-time back-fill pass stamping the block onto every existing active change (the block
  appears naturally on the next field write; a bulk pass is a separate follow-up if desired).
- Linking the change-file-on-integration location (deliberately excluded in brainstorm).
- Changing BOARD.md's own link rendering.
- Multi-remote / multi-host URL schemes beyond GitHub + bare-path fallback.

## ADRs

Cites **ADR-0007** (one-way, change-files-authoritative derived output) and **ADR-0012**
(script-vs-model boundary). No new ADR — this is a direct application of both.
