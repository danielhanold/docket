# render-adr-index.sh — ADR index renderer

## Purpose

Reads the ADR files in `<adrs_dir>` and emits `docs/adrs/README.md` to **STDOUT**. The caller
redirects the output and commits it; this script performs no git writes. It is the **sole writer**
of the ADR index — skills never construct or patch `README.md` by hand. Running it with the same
ADR files always produces byte-identical output (deterministic and idempotent). Offline: no
network calls, no `gh`, no git. Introduced in change 0030.

The index content is derived verbatim from ADR frontmatter with no embellishment. Field values
are reproduced exactly as written in each ADR file; the renderer does not infer, reformat, or
augment them.

## Usage

```
render-adr-index.sh --adrs-dir DIR
```

| Flag | Required | Description |
|---|---|---|
| `--adrs-dir DIR` | yes | Local path to the directory containing ADR `*.md` files (e.g. `.docket/docs/adrs`). |

## Behavior

**Validation.** Exits 2 if `--adrs-dir` is missing or is not a directory.

**File scan.** Finds all `*.md` files in `--adrs-dir` (non-recursive, `maxdepth 1`), excluding
`README.md`. Files without a valid `id:` frontmatter field are silently skipped.

**Row format.** Each ADR produces one list item:

```
- [ADR-NNNN](<filename>) — <title> (<status>)<annotations>
```

Annotations are appended in this order when the corresponding frontmatter field is non-empty:

| Field | Annotation |
|---|---|
| `change:` | `← change #<N>` |
| `supersedes:` | `→ supersedes ADR-NNNN[, ADR-MMMM, …]` |
| `reverses:` | `→ reverses ADR-NNNN[, ADR-MMMM, …]` |
| `relates_to:` | `· relates to ADR-NNNN[, ADR-MMMM, …]` |

**Grouping.** ADRs are sorted into three groups by `status:`:

| Group | Status values |
|---|---|
| Active | Any status not matching `Superseded by*`, `Reversed by*`, or `Deprecated` (e.g. `Accepted`, `Proposed`, drafts). |
| Superseded / Reversed | `status:` starts with `Superseded by` or `Reversed by`. |
| Deprecated | `status: Deprecated`. |

Within each group, rows are sorted by ascending numeric ID. A group with no members emits
`_None._`.

**Output structure:**

```markdown
# Architecture Decision Records

Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only
the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.

## Active

- [ADR-NNNN](<file>) — <title> (<status>)<annotations>
…

## Superseded / Reversed

…

## Deprecated

…
```

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Index written to stdout successfully. |
| 2 | Missing or invalid argument (`--adrs-dir` absent or not a directory; unknown flag). |

## Invariants

- **STDOUT only.** All index content goes to stdout; diagnostics go to stderr. The caller
  redirects stdout to `<adrs_dir>/README.md` and commits.
- **Sole writer.** Skills never construct or patch `README.md` by hand. On a git conflict,
  re-run the script rather than hand-merging (regenerate-don't-3-way-merge rule).
- **Verbatim frontmatter.** Field values are reproduced exactly as written; no inference or
  embellishment (LEARNINGS #30).
- **Offline.** No network, no `gh`, no `git`.
- **Deterministic.** Same ADR files → identical bytes every time.
- **No git writes.** The script never touches the git index; the caller owns the commit. The
  index commit is always separate from ADR-content commits so concurrent ADR creates never
  conflict on the shared index.
