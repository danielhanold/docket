# render-board.sh — deterministic board renderer

## Purpose

Reads the change files (`active/` and `archive/`) and emits the board to **STDOUT**; it performs
no git writes and never touches `BOARD.md` on disk itself. It is the pure *renderer* — the inner
layer. Since change 0059 its immediate caller is `board-refresh.sh`, which captures this stdout
into a temp file and owns the surface-gated decision to atomically replace `BOARD.md` (see
`board-refresh.md`); the git add/commit/push of that file stays the skill caller's job. Skills
never construct the board by hand. Running it with the same change files always produces
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
| `--format markdown\|digest` | no | Output projection. `markdown` (default) emits the board. `digest` emits the line-oriented backlog digest (change 0069). Any other value is an argument error (exit 2). |

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
| implemented | `# · Title · Priority · PR` |

The `#` cell links to the change file (`active/<filename>`). IDs are zero-padded to four digits.
Sections are emitted in the fixed order: in-progress → proposed → blocked → deferred →
implemented. The **Implemented** heading suffix is `— awaiting merge`. Empty statuses are omitted.

**Readiness cell (proposed).** Calls `readiness()` from `lib/docket-frontmatter.sh`; maps the
token: `waiting` → `⏳ waiting on #N — <reason>`; `auto-groom-blocked` → `auto-groom blocked —
needs you`; `needs-brainstorm` → `needs-brainstorm`; `build-ready` → `build-ready`.

**PR cell (implemented).** If `pr:` is a full URL, renders `[#N](url)`. If it is a bare number
and `--repo` is set, constructs `https://github.com/OWNER/REPO/pull/N`. Otherwise renders `#N`.

**Dependency graph (Mermaid).** After the active sections, emits a fenced `mermaid` block with a
`graph TD`. Each active change is a node (ID padded to four digits). Changes with `depends_on:`
emit `PARENT --> CHILD` edges; standalone changes emit a bare node. Done changes from archive are
listed with `:::done` and a `classDef done fill:#d3f9d8;` rule. Killed archive entries are omitted
from the graph.

**Archive section.** If `archive/` contains any `*.md` files, emits a collapsible `<details>` block
with a `| # | Title | Merged |` table. Rows are sorted by merged date descending, then by ID
descending. The `#` cell links to `archive/<filename>`. The merged date is the first ten characters
of the archive filename (the `YYYY-MM-DD` prefix).

**Digest projection (`--format digest`).** A second projection of the **same**
dependency-resolution/readiness pass the board renders from — so `readiness()` keeps exactly one
owner and the digest can never disagree with the board's Readiness cell. Emits, in order: one
`backlog <status> <count>` line per non-zero status (fixed order: in-progress, proposed, blocked,
deferred, implemented, done, killed; `done`/`killed` counted from `archive/`), then one
`change <id> <status> <readiness> <slug>` line per **active** change, ascending by id. `<readiness>`
is `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`,
`waiting-on-<N>-needs-merge`, or `-` for any non-`proposed` status (where readiness does not apply).
No markdown, no mermaid graph, no archive table.

The digest is **report output, not a board surface**: `docket-status.sh` pipes it straight to its
report and never persists it. It is therefore emitted regardless of `board_surfaces` — which is
what lets `board_surfaces: []` keep meaning "no board is rendered or committed" while backlog state
still reaches the report. `board-refresh.sh` remains the sole gated writer of `BOARD.md`:
**board-refresh gates the surface, render-board serves the report.**

## Exit codes

| Code | Meaning |
|---|---|
| 0 | BOARD.md written to stdout successfully. |
| 2 | Missing or invalid argument (`--changes-dir` absent or not a directory; unknown flag; unknown `--format` value). |

## Invariants

- **STDOUT only.** All board content goes to stdout; all diagnostics go to stderr. The immediate
  caller (`board-refresh.sh`) captures this stdout into a temp file and owns the atomic replace of
  `BOARD.md`; the skill above it commits.
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
