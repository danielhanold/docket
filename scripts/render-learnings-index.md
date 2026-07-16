# render-learnings-index.sh — learnings index renderer

## Purpose

Reads the finding files in `<learnings_dir>` and emits `<changes_dir>/learnings/README.md` to
**STDOUT**. The caller redirects the output and commits it; this script performs no git writes.
It is the **sole writer** of the learnings index (ADR-0012) — skills never construct or patch
`README.md` by hand. Running it with the same finding files always produces byte-identical output
(deterministic and idempotent). Offline: no network calls, no `gh`, no git. A member of the
derived-view script family alongside `render-board.sh`, `render-adr-index.sh`, and
`render-change-links.sh`. Introduced in change 0067.

This script has **no `learnings.enabled` awareness**. The callers gate on it — exactly as
`render-board.sh` stays pure while `board-refresh.sh`/`docket-status.sh` own the write decision.

## Usage

```
render-learnings-index.sh --learnings-dir DIR
```

| Flag | Required | Description |
|---|---|---|
| `--learnings-dir DIR` | yes | Local path to the directory containing finding `*.md` files (e.g. `.docket/docs/changes/learnings`). |

Reached from a consuming repo as:

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-learnings-index --learnings-dir DIR
```

## Behavior

**Validation.** Exits 2 if `--learnings-dir` is missing or is not a directory.

**File scan.** Finds all `*.md` files in `--learnings-dir` (non-recursive, `maxdepth 1`), excluding
`README.md`. Files without a valid `slug:` frontmatter field are silently skipped. Field values
(`hook`, `topics`, `promotion_state`, `promoted_to`) are read via `lib/docket-frontmatter.sh`'s
`field`/`list_field`.

**Dequoting.** `field()` returns the raw scalar with surrounding quotes intact. `hook:` is required
to be quoted (it carries a colon-space, putting it in the YAML-scalar-that-must-be-quoted family),
so the renderer strips the surrounding `"..."` or `'...'` before emitting it — otherwise the index
would ship literal quote characters. Only a MATCHED outer pair is stripped: an unquoted value (even
one that itself contains quote characters) or an unterminated/mismatched quote passes through
unchanged, with no stray characters removed. A double-quoted scalar is also unescaped — a hook may
itself discuss quoting (`hook: "Never \"fix\" a guard by widening it."`) — via one left-to-right
pass recognizing YAML's `\"` -> `"` and `\\` -> `\`; a single-quoted scalar's only escape, `''` ->
`'`, is likewise resolved. The unescape is a single pass, not two independent global substitutions,
so that a literal backslash sitting immediately before the closing quote (`...\\"`) resolves as one
escaped-backslash pair followed by the real delimiter, rather than letting a leftover backslash
mis-pair with the quote.

**Grouping.** Findings are partitioned by `promotion_state:`:

| State | Placement |
|---|---|
| `promoted` | Removed from the topic groups; listed in the trailing `## Promoted` appendix. |
| `candidate` | Stays in its topic group; row carries the `⟨needs promotion⟩` marker. |
| `retained`, unset, or any other value | Stays in its topic group; no marker (an unset/unknown state degrades to the safe, visible `retained` tier — it never silently disappears). |

Active (non-promoted) findings are grouped under their **primary topic** — the first tag in
`topics:`. A finding with no topics groups under `uncategorized`. Any remaining tags render inline
on the row as `· also: b, c`. Topic groups are derived from the data (no hand-listed topic set) and
sorted alphabetically; rows within a group are sorted alphabetically by slug.

**Row format (active findings):**

```
- [<slug>](<slug>.md) — <hook><· also: rest><⟨needs promotion⟩ if candidate>
```

**Row format (Promoted appendix):**

```
- [<slug>](<slug>.md) → <promoted_to>
```

Promoted rows carry no `hook` text — the appendix is intentionally compressed and does not tax the
hint surface with prose for findings that have already graduated.

**Output structure:**

```markdown
# Learnings — the build loop's memory

One curated finding per file; this index is the hint surface. …

## <topic>

- [<slug>](<slug>.md) — <hook>…

## Promoted

Graduated to an always-in-context agent-instructions file. …

- [<slug>](<slug>.md) → <promoted_to>
```

An empty `--learnings-dir` (no findings) still renders the header and intro line — this is a valid,
non-crashing render, not an error.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Index written to stdout successfully (including an empty learnings dir). |
| 2 | Missing or invalid argument (`--learnings-dir` absent or not a directory; unknown flag). |

## Invariants

- **STDOUT only.** All index content goes to stdout; diagnostics go to stderr. The caller
  redirects stdout to `<learnings_dir>/README.md` and commits.
- **Sole writer.** Skills never construct or patch `README.md` by hand. On a git conflict,
  re-run the script rather than hand-merging (regenerate-don't-3-way-merge rule).
- **Offline.** No network, no `gh`, no `git`.
- **Deterministic.** Same finding files → identical bytes every time.
- **No git writes.** The script never touches the git index; the caller owns the commit.
- **A finding appears exactly once.** Either in its topic group or in the `## Promoted` appendix
  — never both, never omitted (once its `slug:` is valid).
- **No `learnings.enabled` awareness.** The callers gate on whether to invoke/write the index at
  all (the `render-board.sh` precedent) — this script always renders when given a valid directory.
