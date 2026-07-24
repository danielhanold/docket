# Artifact back-links — a generated link at the top of every artifact pointing home to the change

> Spec for change 0136. Brainstormed 2026-07-23.

## Problem

The change file is the hub of a docket change: change 0035 (`artifact-links`) stamps a generated
`## Artifacts` block at the top of every change that hyperlinks **out** to the spec, plan, results,
ADRs, and PR. But there is no link **back**. A reviewer reading a spec, a plan, a results file, or a
PR has no clickable path home to the change that owns it — they must hand-search the backlog or the
board. The forward direction is solved; the reciprocal is not.

This spec adds the reciprocal: a generated, marker-bounded back-link block at the very top of each
artifact docket touches, pointing to the change file. It is the mirror image of change 0035 — same
sole-writer, script-vs-model discipline (ADR-0012), same GitHub-blob-or-bare-path rendering, same
frontmatter-is-truth stance.

## Scope

**In:** spec, plan, results, and the PR body. **Out:** ADRs — they already back-reference the change
via their `change:` frontmatter field and the ADR index's `← change #N` rendering, so a body block
would be redundant.

The two superpowers-authored artifacts (spec, plan) are in scope: docket does not own their bodies,
but it *does* run a step right after they are written (it already calls `render-change-links.sh`
then), and the back-link block is docket's to own even though the surrounding prose is superpowers'.

## Key design decisions (from the brainstorm)

1. **Docket post-write stamp, not superpowers cooperation.** Docket never patches the vendored
   superpowers skills. It stamps the back-link block itself, immediately after superpowers writes
   the spec/plan, treating the artifact body as superpowers' and the marker block as docket's. This
   mirrors how 0035 already runs `render-change-links.sh` after the spec is written.

2. **Uniform target: the change on `metadata_branch`.** Every back-link points to the change file on
   `metadata_branch` (docket), at its current canonical path — `active/<id>-<slug>.md` while the
   change is live, `archive/<YYYY-MM-DD>-<id>-<slug>.md` once it is terminal. The target is *always*
   docket, so `terminal_publish` never changes the link **target** — only whether the close-out
   **re-render** fires. This keeps the renderer's link logic branchless and identical in every repo.

3. **The change file moves, so the link must be re-rendered.** The change is renamed `active/ →
   archive/` on its terminal transition, which breaks any back-link baked before then. Durability
   therefore requires re-rendering at close-out — exactly as 0035's forward block is re-rendered at
   `done`. Back-links re-render only at **creation + close-out** (the only moments the change's
   canonical path or title change), so they are far lighter than the forward renderer, which
   re-renders on every field write.

4. **Durability is tiered by where the artifact lives, not by who authored it.**
   - **spec** lives on docket beside the change → re-render is cheap and always happens → always durable.
   - **PR** body is re-editable via `gh` → always durable.
   - **plan / results** live on the code line (feature → integration branch). Their durable
     re-render rides `terminal-publish.sh`'s **existing** integration-branch commit — no additional
     commit — but that commit only exists when `terminal_publish: true`. When `terminal_publish:
     false` (docket's default), there is no close-out integration commit to ride, so plan/results
     are **stamped once at creation and accepted to go stale after archive** (see decision 6).

5. **The plan/results re-render folds into terminal-publish's internals.** `terminal-publish.sh`
   already provisions a transient integration-branch worktree and makes exactly one publish commit
   when `--enabled true`; the plan/results files are already on that branch (they arrived via the PR
   merge). Re-stamping their back-links happens inside that worktree, in that same commit. This is
   the one genuine widening of terminal-publish: it goes from *copying records in* to *also
   re-stamping two existing code-line files*.

6. **`terminal_publish: false` — stamp once, accept staleness.** In a repo that opted out of putting
   records on the code line, plan/results back-links are stamped at creation (pointing to the change's
   then-current `active/` path on docket), valid through build/review/merge, and become stale only
   after the change archives — at which point the plan/results are historical. `spec` and `PR` stay
   durable regardless. No new machinery is added for the `terminal_publish: false` path.

## The back-link block

Inserted as the very first content in the artifact file, above everything else:

```markdown
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0136 — Artifact back-links](<url>)**
<!-- docket:backlink:end -->

<original artifact content, unchanged, below>
```

- `<url>` = GitHub blob URL to the change file at its current canonical path on `metadata_branch`
  when a GitHub remote is detected; otherwise the bare code-formatted path (`` `docs/changes/...` ``).
  Remote detection and fallback are identical to `render-change-links.sh`.
- Link text = `Change <padded-id> — <title>`, read from the change's frontmatter (`id`, `title`).
- The block is delimited by `<!-- docket:backlink:start (generated — do not hand-edit) -->` /
  `<!-- docket:backlink:end -->`, chosen to parallel 0035's `docket:artifacts:*` markers.

## The renderer — `render-artifact-backlink.sh`

A new deterministic script, the sole writer of the `docket:backlink` block (ADR-0012), sibling to
`render-change-links.sh` and sharing its idioms.

### Usage

```
render-artifact-backlink.sh --artifact-file FILE --change-file CHANGE [--repo OWNER/REPO]
```

| Flag | Required | Description |
|---|---|---|
| `--artifact-file FILE` | yes | The artifact markdown file to update in place (spec, plan, or results). |
| `--change-file CHANGE` | yes | The change file at its current canonical path. Frontmatter (`id`, `slug`, `title`) and the path itself are the render inputs. |
| `--repo OWNER/REPO` | no | Build GitHub blob URLs. Defaults to deriving `OWNER/REPO` from the artifact file's `origin` remote; non-GitHub / no remote → bare-path fallback. |

Mock seams mirror `render-change-links.sh`: `GIT="${GIT:-git}"`,
`DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}"`.

### Behavior

- **Config.** Resolves `metadata_branch` (and repo, when not passed) via `docket-config.sh --export`,
  exactly as `render-change-links.sh` does. Exit 1 if resolution fails.
- **Link construction.** Reads `id` + `title` from `--change-file` frontmatter (via the shared
  `lib/docket-frontmatter.sh` helpers). Computes the target as
  `blob/<metadata_branch>/<change-file-relative-path>` in GitHub mode, or the bare relative path in
  fallback mode. The relative path is derived from `--change-file` (already `active/…` or
  `archive/…` — the caller passes the current canonical path, so no state inference is needed).
- **Block placement.** If the start marker exists, replace the inclusive marker region via `awk`
  (same mechanism as 0035). If absent, insert the block as the very first lines of the file,
  followed by one blank line, then the original content. No template seeding is required — superpowers
  artifacts are not docket-templated, so first-write always takes the insert path.
- **Idempotency.** Same frontmatter + same path + same repo → byte-identical block. Re-running is a
  no-op.
- **Offline.** No network, no `gh`. Bare-path fallback when no GitHub remote.
- **PR body is NOT rendered by this script.** The PR body is not a file in a working tree; it is
  authored/edited through `gh` by the calling skill (see call sites). The script's domain is
  on-disk artifacts only.

### Exit codes (mirror render-change-links.sh)

| Code | Meaning |
|---|---|
| 0 | Block written (or unchanged). |
| 1 | `docket-config.sh` resolution failed. |
| 2 | Missing/invalid argument (`--artifact-file` or `--change-file` absent/missing, unknown flag). |

## Call sites

### Creation-time stamps

| Artifact | Skill / step | Action |
|---|---|---|
| **spec** | `docket-new-change §2`, `docket-groom-next`, `docket-auto-groom` | After superpowers writes the spec and the skill records `spec:` + re-renders the forward block, call `render-artifact-backlink.sh --artifact-file <spec> --change-file <change>` on the spec (on docket). The block edit rides the same spec-write commit. |
| **plan** | `docket-implement-next §4` | After superpowers writes the plan and the skill records `plan:`, stamp the plan's back-link on the feature branch (rides the PR). |
| **PR body** | `docket-implement-next §7` | When docket authors the PR body, include the back-link line at the top of the body pointing to the change on docket (built with the same link logic; skill-side, since there is no file). |
| **results** | close-out, when the `results:` file is written | Stamp the results back-link when the file is created. |

### Close-out re-renders (the terminal close-out sequence)

Added to `references/terminal-close-out.md`:

- **spec** — re-rendered on docket in **step 2**, beside the existing forward-block re-render on the
  archived change file (both files on docket, one extra renderer call — cheap). Must-land, like the
  forward re-render it accompanies.
- **plan / results** — re-rendered inside `terminal-publish.sh` (**step 3**) when `--enabled true`,
  folded into its existing publish commit (see below). No-op when `--enabled false`.
- **PR body** — re-rendered via `gh pr edit` at close-out, best-effort (like the GitHub board
  mirror): a network failure logs and continues, never aborts the close-out.

Kills follow the same sequence: a proposed-kill re-renders whatever artifacts exist (usually just the
spec); an in-progress reconcile-kill may also have a plan/PR. The re-render paths are identical —
only which artifacts are present differs.

## terminal-publish.sh extension

When `--enabled true` and in change mode, after the copy-set is assembled and before the publish
commit, `terminal-publish.sh` runs `render-artifact-backlink.sh` against the change's `plan:` and
`results:` files **if they exist on the integration-branch worktree it has already checked out**,
pointing them at the archived change on `metadata_branch`. The edits are staged into the same commit.

- **Inert when `--enabled false`:** the existing knob guard exits 0 before any of this, so a
  `terminal_publish: false` repo is untouched — exactly the tiering from decision 4.
- **Inert in `main`-mode:** the existing mode guard exits 0 first.
- **Missing plan/results is not an error:** absent fields (e.g. a killed change with no plan) are
  simply skipped — the re-stamp is per-artifact best-effort within the publish, never a gate on the
  publish's own success.
- **Idempotency preserved:** because the renderer is idempotent and the publish already has
  reuse-existing-file idempotency, two drivers racing on the same change still converge.

The contract (`scripts/terminal-publish.md`) is updated to document the new re-stamp step, its
guards, and that it is confined to `--enabled true` change mode.

## Testing

Plain-bash tests under `tests/`, matching the 0035 / docket house pattern:

- **`render-artifact-backlink.sh` golden-file tests:** GitHub mode (active + archive change paths),
  bare-path fallback (no/non-GitHub remote), insert-when-absent vs replace-when-present, idempotency
  (second run byte-identical), and title/id extraction from frontmatter.
- **Argument validation:** missing/invalid `--artifact-file` and `--change-file` → exit 2; config
  failure → exit 1.
- **Wiring sentinels:** assert each call site (`docket-new-change`, `docket-groom-next`,
  `docket-auto-groom`, `docket-implement-next`, terminal close-out) invokes the renderer / includes
  the PR-body line — the same sentinel-grep approach 0035 used to pin its call sites.
- **terminal-publish re-stamp test:** with `--enabled true`, a change carrying `plan:`/`results:`
  gets those files re-stamped inside the single publish commit (assert commit count unchanged and the
  block present); with `--enabled false`, the files are untouched.

## Out of scope

- A one-time back-fill pass stamping back-links onto artifacts of already-terminal changes. The block
  appears naturally on the next relevant write; a bulk pass is a separate follow-up if wanted.
- ADR body back-links (already covered by `change:` frontmatter + index rendering).
- Making plan/results back-links durable under `terminal_publish: false` (deliberately accepted stale
  after archive — decision 6).
- Any change to BOARD.md rendering or the forward `## Artifacts` block.
- URL schemes beyond GitHub blob + bare-path fallback.

## Open questions

None outstanding. The uniform-target choice, the durability tiering, the terminal-publish fold-in,
and the `terminal_publish: false` stamp-once behavior are all resolved above. One presentation call —
the exact back-link text/glyph (`↩ Change NNNN — <title>`) — is recorded here and open to revision at
build time.
