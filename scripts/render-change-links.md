# render-change-links.sh — per-change Artifacts block renderer

## Purpose

Reads one change file's frontmatter and rewrites the marker-bounded `## Artifacts` block in
place. Frontmatter is the single source of truth; this script is the **sole writer** of that
block (ADR-0012 script-vs-model boundary). Skills never construct or patch the block by hand;
they call this script after every frontmatter field write and the block edit rides in the same
commit. Offline: no network calls, no `gh`. Deterministic and idempotent: same frontmatter
values → byte-identical block. Introduced in change 0035.

## Usage

```
render-change-links.sh --change-file FILE [--repo OWNER/REPO] [--adrs-dir DIR]
```

| Flag | Required | Description |
|---|---|---|
| `--change-file FILE` | yes | Path to the change markdown file to update in place. |
| `--repo OWNER/REPO` | no | Build GitHub `blob/` and `pull/` URLs. Defaults to deriving `OWNER/REPO` from the `origin` remote of the change file's repo. Absent or non-GitHub remote: falls back to bare code-formatted paths. |
| `--adrs-dir DIR` | no | Local directory to resolve ADR slugs to filenames. Defaults to `METADATA_WORKTREE/ADRS_DIR` from `docket-config.sh`. |

The changes directory scanned for stacked children is not a flag: it is derived from the change
file's own location (`<changes>/active/` or `<changes>/archive/`), and `CHANGES_DIR` from
`docket-config.sh` supplies the repo-relative prefix for the resulting links.

Mock seams: `GIT="${GIT:-git}"`, `DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}"`.

## Behavior

**Validation.** Exits 2 if `--change-file` is missing or the file does not exist. Exits 1 if
`docket-config.sh --export` fails.

**Repo / GitHub mode.** When `--repo` is explicit, GitHub mode is active. Otherwise the script
calls `git remote get-url origin` on the change file's directory and pattern-matches for
`github.com` hosts (`git@github.com:`, `https://github.com/`, `ssh://git@github.com/`). Any other
remote — or no remote — leaves GitHub mode off.

**Offline fallback.** When GitHub mode is off every artifact cell renders as a backtick-fenced
bare path (`\`path\``) instead of a hyperlink. No network calls are made in either mode.

**Row construction.** For each populated frontmatter field, one table row is appended to the block:

| Field | Link target in GitHub mode | Fallback (no GitHub) |
|---|---|---|
| `spec:` | `blob/<metadata_branch>/<spec>` | bare code-formatted path |
| `plan:` | `blob/<build_ref>/<plan>` | bare code-formatted path |
| `results:` | `blob/<build_ref>/<results>` | bare code-formatted path |
| `pr:` | `[#N](url)` when `pr:` is a URL | verbatim `pr:` value |
| `adrs:` | `[ADR-NNNN](blob/<metadata_branch>/<slug>)` per id | backtick path (slug resolved) or `ADR-NNNN` (slug missing) |
| *(derived — no field)* | **Stacked children**: `[#NNNN](blob/<metadata_branch>/<child path>) <title> (<status>)` per child | `#NNNN <title> (<status>)` per child |

**Build ref.** `spec:` always links to `<metadata_branch>`. `plan:` and `results:` link to
`<integration_branch>` when the change is `done` (the file has merged); otherwise they link to
`<branch>` (the feature branch). The test is `status = done` exactly — **not** "is terminal" and not
"is not active". Those three used to coincide; `stacked-merged` (change 0298) splits them. A
stacked-merged change merged into its *parent's* branch, not into the integration branch, so its
plan and results are not reachable from `<integration_branch>` yet and the feature branch stays the
only ref that resolves them.

**Stacked children (derived, never stored).** The parent side of a `stacked_on:` link has no
frontmatter field of its own (change 0298). Instead, each render scans the `active/` and `archive/`
directories of the changes tree the change file itself sits in, and emits one **Stacked children**
row naming every change whose *anchored* `stacked_on:` resolves to this change's id, sorted by
padded id. Nothing is emitted — not even the label — when the set is empty. Consequences worth
knowing: a child that is renamed, re-parented, or killed simply stops appearing on the next render,
and the two sides of the relationship cannot disagree because only one side is stored.

The scan reads `stacked_on:` with the **anchored** `fm_field`. An unanchored read of an absent
optional key returns body prose, and a change file discussing `stacked_on:` in its body is ordinary
content in this repo — it would render a phantom child. A `grep` for the *key's* shape narrows the
candidate set before that read; it is a prefilter over a superset, never the decision, and it is
keyed on the key rather than the value so a padded or inline-commented spelling still survives it.

**ADR slug resolution.** For each id in `adrs:`, globs `<adrs-dir>/<NNNN>-*.md`. If a match is
found its relative path is used for the link; if not, the link targets the ADR directory
(GitHub mode) or degrades to the bare `ADR-NNNN` label (fallback mode).

**Killed changes.** When `status: killed` the feature branch is gone and not merged. `plan:` and
`results:` rows link to the PR URL if `pr:` is a URL; if `pr:` is a non-URL value the filename
renders as plain text (no broken link); if `pr:` is absent the row is omitted entirely.

**Marker block replacement.** The block is delimited by:
```
<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->
```
If the start marker already exists in the file, the entire inclusive marker region is replaced via
`awk`. If the start marker is absent (new file, template-seeded empty), the block is inserted as
the first body section immediately after the frontmatter closing `---`, preceded by an
`## Artifacts` heading. Blank rows (e.g. from killed + no PR) are stripped before writing.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Block written successfully (or no rows: empty block with markers only). |
| 1 | `docket-config.sh` resolution failed. |
| 2 | Missing or invalid argument (`--change-file` absent/missing, unknown flag). |

## Invariants

- **Sole writer.** The `## Artifacts` block is never hand-edited by skills or agents. On
  disagreement between the block and frontmatter, re-run this script to regenerate.
- **ADR-0012 boundary.** The script-vs-model boundary: models write frontmatter fields; this
  script owns the derived block. Both edits commit together.
- **In-place edit.** The script modifies `--change-file` directly (via a temp file + `mv`); the
  caller commits the file after the script exits.
- **Offline.** No network calls in either GitHub or fallback mode.
- **Deterministic.** Same frontmatter → same block bytes every time. The **Stacked children** row is
  derived from other files, so "same frontmatter" there means the whole changes directory: the row
  is sorted by padded id rather than by glob order, because `archive/` globs by date.
- **No `stacked_children:` field.** The parent-side link is derived on every render and never
  written back (change 0298). A field would be a second copy of a fact that already has an owner.
- **No git writes.** The script never touches the git index; the caller owns the commit.
