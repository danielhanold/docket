# render-artifact-backlink.sh — artifact back-link block renderer

## Purpose

Stamps a marker-bounded `docket:backlink` block at the **top** of an artifact (spec, plan, or
results), pointing home to its change file on `metadata_branch` at the change's current canonical
path. The reciprocal of `render-change-links.sh`'s forward `## Artifacts` block. Frontmatter
(`id`, `title`) + the change-file path are the single source of truth; this script is the **sole
writer** of the block (ADR-0012 script-vs-model boundary). Skills never construct or patch the
block by hand. Offline: no network, no `gh`. Deterministic and idempotent: same inputs →
byte-identical block. Introduced in change 0136.

## Usage

```
render-artifact-backlink.sh --artifact-file FILE --change-file CHANGE [--repo OWNER/REPO]
```

| Flag | Required | Description |
|---|---|---|
| `--artifact-file FILE` | yes | The spec/plan/results markdown file to update in place. |
| `--change-file CHANGE` | yes | The change file at its current canonical path (`active/…` or `archive/…`). `id` + `title` are read from its frontmatter; the URL path is derived from this path. |
| `--repo OWNER/REPO` | no | Build GitHub `blob/` URLs. Defaults to deriving `OWNER/REPO` from the artifact file's `origin` remote. Absent or non-GitHub remote: bare code-formatted path fallback. |

Mock seams: `GIT="${GIT:-git}"`, `DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}"`.

## Behavior

**Validation.** Exits 2 if `--artifact-file` or `--change-file` is missing, does not exist, or an
unknown flag is passed. Exits 1 if `docket-config.sh --export` fails.

**Config.** Resolves `METADATA_BRANCH` and `CHANGES_DIR` from `docket-config.sh --export`.

**Link construction.** Reads `id` and `title` from `--change-file` frontmatter via the
frontmatter-scoped `fm_field` (first `---…---` block only). The change's repo-relative path is
`<CHANGES_DIR>/<active|archive>/<basename>`, derived from the path the caller passed — its current
canonical location, so `terminal_publish` never changes the link target. GitHub mode links to
`blob/<metadata_branch>/<relpath>`; fallback renders the bare code-formatted path.

**Block shape.**
```
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change NNNN — <title>](<url>)**      # GitHub mode
> ↩ **Change NNNN — <title>** — `<relpath>` # fallback
<!-- docket:backlink:end -->
```

**Placement.** If the start marker exists, the inclusive marker region is replaced in place via
`awk`. If absent, the block is inserted as the very first lines of the file, followed by one blank
line, then the original content. No template seeding is needed — superpowers artifacts are not
docket-templated, so first-write always inserts.

**Untrusted title.** The model-authored `title` is written with `printf '%s'` into a block temp
file that `awk` inserts verbatim — never a `sed`/string-interpolated replacement (which would
reinterpret `&`, `\1` in a real title). `fm_field` returns a single line, so no structural/newline
injection.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Block written (or unchanged). |
| 1 | `docket-config.sh` resolution failed. |
| 2 | Missing/invalid argument (`--artifact-file`/`--change-file` absent or missing, unknown flag). |

## Invariants

- **Sole writer.** The `docket:backlink` block is never hand-edited. Re-run to regenerate.
- **In-place edit.** Modifies `--artifact-file` via a temp file + `mv`; the caller commits.
- **Offline.** No network calls in either mode.
- **Deterministic.** Same inputs → byte-identical block.
- **No git writes.** Never touches the git index; the caller owns the commit.
- **Uniform target.** The link always points to the change on `metadata_branch`; `terminal_publish`
  changes only whether the close-out re-render fires.
