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

**Build ref.** `spec:` always links to `<metadata_branch>`. `plan:` and `results:` link to
`<integration_branch>` when the change is `done` (the file has merged); otherwise they link to
`<branch>` (the feature branch).

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
- **Deterministic.** Same frontmatter → same block bytes every time.
- **No git writes.** The script never touches the git index; the caller owns the commit.
