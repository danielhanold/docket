# render-board.sh — deterministic board renderer

## Purpose

Reads the change files (`active/` and `archive/`) and emits the requested projection to **STDOUT**;
it performs no git writes and never touches `BOARD.md` on disk itself. It is the pure *renderer* —
the inner layer. It has **two immediate callers**, one per projection:

- `board-refresh.sh` (change 0059) consumes the **markdown** projection: it captures this stdout
  into a temp file and owns the surface-gated decision to atomically replace `BOARD.md` (see
  `board-refresh.md`); the git add/commit/push of that file stays the skill caller's job.
- `docket-status.sh` (change 0069) consumes the **digest** projection (`--format digest`): it pipes
  those lines straight into its report and persists nothing. That call is read-only — no file is
  written, so it does not go through (and does not need) the `board-refresh.sh` gate.

Skills never construct the board by hand. Running it with the same change files always produces
byte-identical output (deterministic and idempotent). Offline: no network calls, no `gh`.
Introduced in change 0022.

## Usage

```
render-board.sh --changes-dir DIR [--repo OWNER/REPO] [--format markdown|digest]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the changes directory (`active/` and `archive/` are children of this dir). |
| `--repo OWNER/REPO` | no | Used to build `pr:` hyperlinks in the **Implemented** column. Defaults to deriving `OWNER/REPO` from the `origin` remote of `--changes-dir` (best-effort, offline). Absent or non-GitHub remote: PR numbers render as bare `#N`. |
| `--format markdown\|digest` | no | Output projection. `markdown` (default) emits the board. `digest` emits the line-oriented backlog digest (change 0069) plus its trailing `ready` queue line (change 0094). Any other value is an argument error (exit 2). |

Mock seam: `GIT="${GIT:-git}"` — override in tests.

## Behavior

**H1 title.** The first line emitted is `# Backlog` (followed by a blank line).

**Validation.** Exits 2 if `--changes-dir` is missing or is not a directory.

**Repo derivation.** When `--repo` is not supplied, extracts `OWNER/REPO` from `git remote get-url
origin` on the `--changes-dir` repo by stripping the `.git` suffix and `git@github.com:` /
`https://github.com/` prefixes. Failure is silently swallowed; PR links degrade to bare numbers.

**Count line.** Counts all `*.md` files across `active/` and `archive/`. Emits a bold count +
per-status emoji summary: `**N changes** — 🟢 N in progress · 🟡 N proposed · …`. Only statuses
with at least one change appear.

**Active sections.** For each status that has at least one change, emits a `## <Emoji> <Status>
(N)` heading followed by a Markdown table. Column layout varies by status:

| Status | Columns |
|---|---|
| in-progress | `# · Title · Priority · Spec · Branch` |
| proposed | `# · Title · Priority · Readiness` |
| blocked | `# · Title · Priority · Blocked by` |
| deferred | `# · Title · Priority` |
| implemented | `# · Title · Priority · PR · Readiness` |

The `#` cell links to the change file (`active/<filename>`). IDs are zero-padded to four digits.
Sections are emitted in the fixed order: in-progress → proposed → blocked → deferred →
implemented. The **Implemented** heading suffix is `— awaiting merge`. Empty statuses are omitted.

**Readiness cell (proposed).** Calls `readiness()` from `lib/docket-frontmatter.sh`; maps the
token: `waiting` → `⏳ waiting on #N — <reason>`; `auto-groom-blocked` → `auto-groom blocked —
needs you`; `needs-brainstorm` → `needs-brainstorm`; `build-ready` → `build-ready`.

**PR cell (implemented).** If `pr:` is a full URL, renders `[#N](url)`. If it is a bare number
and `--repo` is set, constructs `https://github.com/OWNER/REPO/pull/N`. Otherwise renders `#N`.

**Readiness cell (implemented).** The `implemented` table carries a `Readiness` column: a change
whose body has a `## Finalize blocked` section (written by `docket-finalize-change` when a gate
failure needs a human) renders `finalize blocked — needs you`; every other implemented change
renders an empty cell. The `digest` format reports the same state as the token `finalize-blocked`
(or `-`), so the two projections cannot disagree.

**Dependency graph (Mermaid).** After the active sections, emits a fenced `mermaid` block with a
`graph TD`. Each active change is a node (ID padded to four digits). Changes with `depends_on:`
emit `PARENT --> CHILD` edges; standalone changes emit a bare node. A done change from the archive
is styled `:::done` **only when an active change's `depends_on` references it** (so it already
appears as an edge parent); unreferenced done ids — floating, edgeless nodes — are dropped. The
`classDef done fill:#d3f9d8;` rule is emitted only when at least one `:::done` node remains. Killed
archive entries are omitted from the graph. This pruning is universal: it changes every board that
has an unreferenced done id, not only large archives.

**Archive section.** If `archive/` contains any `*.md` files, emits a collapsible `<details>`
block. The `| # | Title | Merged |` table lists a **verbatim window** — every `killed` entry (any
age) plus the `ARCHIVE_RECENT` (default 15) most-recent `done` entries — sorted by merged date
descending, then by ID descending; the `#` cell links to `archive/<filename>` and the merged date
is the first ten characters of the filename (the `YYYY-MM-DD` prefix). `done` entries older than
the window are **not** listed individually: they collapse into an "Older done (collapsed)"
`| Month | Done |` digest, one row per `YYYY-MM` bucket (newest first, each linking to the
`archive/` directory), keeping the always-loaded board flat as the archive grows. `killed` never
collapses. When the archive's `done` count is at or below `ARCHIVE_RECENT`, no digest is emitted
and the **archive table is byte-identical to the pre-window renderer** — the window is inert until
it is needed. (The mermaid pruning above is separate and universal, not inert.) The window is
count-based, not time-based, so the renderer stays deterministic — same change files, identical
bytes.

**Digest projection (`--format digest`).** A second projection of the **same**
dependency-resolution/readiness pass the board renders from — so `readiness()` keeps exactly one
owner and the digest can never disagree with the board's Readiness cell. Emits, in order: one
`backlog <status> <count>` line per non-zero status (fixed order: in-progress, proposed, blocked,
deferred, implemented, done, killed; `done`/`killed` counted from `archive/`), then one
`change <id> <status> <readiness> <slug>` line per **active** change, ascending by id. `<readiness>`
is `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`, or
`waiting-on-<N>-needs-merge` for a `proposed` change; `finalize-blocked` for an `implemented`
change carrying the `## Finalize blocked` section; and `-` for everything else — an `implemented`
change *without* the marker, plus every change in any other status (where readiness does not
apply). Readiness has exactly one owner per status, so the digest and the board cannot
disagree. The marker is detected by `has_section`, a **whole-line** match: a change file that
merely mentions `## Finalize blocked` inline in prose does not carry the section. No
markdown, no mermaid graph, no archive table.

**The `ready` line (change 0094).** The digest's final line is always `ready [<id> …]` — the
**build-ready queue in selection order**: `priority` (`critical` > `high` > `medium` > `low`) →
`created` (ascending) → `id` (ascending), the convention's *Build-readiness & selection* order. An
unset or unrecognized `priority` is treated as `medium`; a `created:` that is unset, empty, or
malformed (not a well-formed `YYYY-MM-DD`) sorts **last** within its priority band, never first —
an unstamped or unparseable change must never preempt dated work.

Its membership is exactly the set of changes the `change` lines report as `proposed build-ready`:
it is a **second call** to the same pure `digest_readiness()` function with identical arguments, so
the line can never disagree with them — the parity rests on `digest_readiness` being pure and the
`DEP_*` globals staying unmutated between the two loops. What it adds is **order**, which the
id-ascending `change` lines deliberately do not carry. Both sort keys are static frontmatter, so the
renderer performs no wall-clock read and stays deterministic.

The line is **always emitted**, bare (`ready`, no ids) when nothing is build-ready. Absence of a
`ready` line therefore means **no queue was produced** — an older `render-board.sh`, or a failed
render — and never "nothing is ready". Consumers must treat the two cases differently:
`docket-implement-next` Step 1 falls back to walking `active/` on absence, but reports `drained` on
a bare line.

The digest is **report output, not a board surface**: `docket-status.sh` pipes it straight to its
report and never persists it. It is therefore emitted regardless of `board_surfaces` — which is
what lets `board_surfaces: []` keep meaning "no board is rendered or committed" while backlog state
still reaches the report. `board-refresh.sh` remains the sole gated writer of `BOARD.md`:
**board-refresh gates the surface, render-board serves the report.**

## Exit codes

| Code | Meaning |
|---|---|
| 0 | The requested projection (the markdown board, or the digest under `--format digest`) was written to stdout successfully. |
| 2 | Missing or invalid argument (`--changes-dir` absent or not a directory; unknown flag; unknown `--format` value). |

## Invariants

- **STDOUT only.** All rendered content goes to stdout; all diagnostics go to stderr. What the
  caller does with that stdout is projection-specific: `board-refresh.sh` captures the **markdown**
  projection into a temp file and owns the atomic replace of `BOARD.md` (the skill above it
  commits), while `docket-status.sh` pipes the **digest** projection into its report and writes it
  nowhere.
- **Renderer, not writer.** This script is the inner renderer: it never writes, truncates, or
  deletes `BOARD.md` — `board-refresh.sh` owns that write decision. Skills never construct or patch
  `BOARD.md` by hand. On a git conflict, re-run the gated helper rather than hand-merging.
- **Offline.** No network calls, no `gh`. Depends only on the change files and (optionally) a
  local `git remote get-url` call.
- **Deterministic.** Same change files → identical bytes every time. Safe to re-run at any point.
- **No git writes.** The script never touches the git index or working tree; the caller owns the
  commit.
- **Default output is byte-identical.** `--format` defaults to `markdown`; the digest is purely
  additive. The golden byte-compare in `tests/test_render_board.sh` is the regression guard.
