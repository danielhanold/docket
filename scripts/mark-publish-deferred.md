# scripts/mark-publish-deferred.sh — contract

## Purpose

The sole writer of the `## Publish deferred` marker (change 0083). A terminal close-out whose
publish step is **expected** (`terminal_publish: true`, docket-mode) but consciously **deferred**
or **blocked** leaves this dated section on the archived change file, so the gap is visible where
a human reads it rather than living only in a chat thread — the #0043 failure mode, invisible for
eight days.

Pure file editor: **no git, no network, no commit, no push.** The caller stages, commits, and
pushes on the metadata branch per docket's field-write rule. The model never hand-writes the
section (ADR-0012 script-vs-model boundary).

## Usage

```
mark-publish-deferred.sh --mode add --change-file PATH --reason deferred|blocked
                         [--detail TEXT] [--date YYYY-MM-DD] [--integration-branch B] [--id N]
mark-publish-deferred.sh --mode remove --change-file PATH
```

| Flag | Meaning |
|---|---|
| `--mode add` | Write the marker. **Idempotent by replacement** — an existing section is stripped first, so a re-mark never appends a second heading. Appended last in the file. |
| `--mode remove` | Strip the marker. A file carrying none is a no-op that exits 0 and writes nothing — the file is left byte-untouched, not merely line-equivalent. |
| `--change-file` | Path to the change file **in the metadata working tree**. Required; must exist and be writable. |
| `--reason` | Fixed prefix, `add` only: `deferred` (a human gate never answered) or `blocked` (a wall the run could not pass). |
| `--detail` | Short single-line free text after the prefix. Optional. |
| `--date` | UTC `YYYY-MM-DD` for the sub-heading. Defaults to today (UTC). |
| `--integration-branch` | Named in the marker prose. Defaults to `main`. |
| `--id` | Change id, inlined into the re-arm command hint. Optional. |

## Behavior

`add` renders, in order: the exact heading `## Publish deferred`; a dated
`### <date> — terminal-publish to \`<branch>\` not completed` sub-heading; a
`**<reason>** — <detail>` line; the standing prose naming what did not run and where the record
lives; and a `**Re-arm:**` line. `remove` deletes from the heading through the line before the
next column-0 `## ` heading (or EOF), then trims trailing blank lines — but only when a marker was
actually found and stripped. If the file carries no marker, `remove` performs no write at all (not
even a no-op rewrite): the trailing-blank-line trim never runs, so pre-existing trailing blank
lines in a markerless file are preserved exactly.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | The file now matches the requested state (including a no-op `remove`). |
| `1` | A real error: bad `--mode`/`--reason`/`--date`, missing `--change-file`, an unreadable or unwritable file, or a `--detail` carrying control characters. **The file is left byte-untouched.** |

## Invariants

- **Whole-line heading match.** The section is located by `$0 == "## Publish deferred"`, never a
  substring: change files routinely *mention* marker names in prose, and a substring match would
  delete from an inline mention to the next heading. Mirrors `has_section`'s `-x` rule.
- **`### ` does not terminate the section.** The terminator is a column-0 `## ` heading; `^## `
  cannot match `### ` (whose third character is `#`).
- **`--detail` is untrusted input.** A model authors it, so it is rejected at intake by *shape*
  (any control character) and written through `awk`'s `ENVIRON[...]` — never interpolated into a
  `sed` replacement, where an `&` in ordinary English ("approval & sign-off") would be
  reinterpreted.
- **Never written under suppression.** Callers must not invoke `--mode add` when
  `terminal_publish: false` or in `main`-mode: a suppressed publish is legitimate *success*, not a
  deferral. The gate lives at the call site (`terminal-publish.sh` and the close-out drivers),
  not here — this script edits whatever file it is handed.
- **Atomic write.** Content is rendered to a temp file and moved into place; the target is never
  the redirect target of a producer that could fail mid-render.
