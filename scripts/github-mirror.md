# github-mirror.sh — one-way GitHub Issues + Projects v2 mirror

## Purpose

The deterministic engine for docket's `github` board surface (change 0011). Reads change files
from `active/` and `archive/`, upserts one GitHub issue per change, reconciles the `docket:`
label namespace, closes terminal changes with the appropriate reason, and optionally syncs
Projects v2 items. **Strictly one-way**: change files are the source of truth; this script never
reads GitHub state back into change files. Invoked by `docket-status`'s Board pass when
`github` is in `board_surfaces`. The **surface semantics** (the `issue:` upsert contract, label
namespace, status→issue mapping, and Projects v2 field design) are documented in
[`docket-convention/github-board-mirror.md`](../skills/docket-convention/github-board-mirror.md);
this contract covers the executable mechanics.

## Usage

```
github-mirror.sh [--dry-run] --changes-dir DIR [--repo OWNER/REPO]
                 [--metadata-branch BR] [--changes-path P] [--integration-branch BR]
                 [--adrs-dir DIR] [--project OWNER/NUMBER]
                 [--auto-create-project [--project-owner OWNER]]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the changes directory (`active/` and `archive/` are children). Must be the metadata worktree — not an integration-branch checkout where `active/` is pruned. |
| `--repo OWNER/REPO` | no | GitHub repo for issue creation and link building. Required for Projects v2 item linking. |
| `--dry-run` | no | Print `gh` calls to stderr instead of executing them. Issue/project numbers are unknown; `issue-minted`/`project-minted` lines are emitted with `(dry-run)` suffix. |
| `--metadata-branch BR` | no | Branch that hosts change files and spec/ADR blobs. Default: `docket`. |
| `--changes-path P` | no | Repo-relative path to the changes directory, used in issue body links. Default: `docs/changes`. |
| `--integration-branch BR` | no | Branch where plan/results files land after merge. Default: `main`. |
| `--adrs-dir DIR` | no | Local dir to resolve ADR slugs for issue body links. |
| `--project OWNER/NUMBER` | no | Sync items into an existing Projects v2 board. |
| `--auto-create-project` | no | When `--project` is unset, mint a private board under the repo owner (or `--project-owner`), seed the `Docket Status` single-select field, and print `project-minted <owner> <number>` for the caller to write into `.docket.yml`. Opt-in — never silently mints a board. |
| `--project-owner OWNER` | no | Override the auto-create board owner (default: the `OWNER` portion of `--repo`). |

Mock seam: `GH="${GH:-gh}"` — set `GH` to a stub in tests; combine with `--dry-run` for fully
offline test runs.

## Behavior

### Validation and setup

Exits 2 if `--changes-dir` is missing. If the directory does not exist, logs a warning and exits
0 (best-effort no-op).

**Wrong-tree guard.** If `active/` is empty but `archive/` is non-empty the script logs a prominent
warning: this is the signature of an integration-branch checkout where the live backlog has been
pruned. The run continues but will only mirror archived changes — the caller should point
`--changes-dir` at the metadata worktree instead.

### Pass 1 — dependency resolution + issue index

Calls `resolve_deps` from `lib/docket-frontmatter.sh` to populate `STATUS_OF`, `DEP_STATE`,
`DEP_REASON`, and `DEP_ON`. Scans all `active/` and `archive/` files and seeds an `ISSUE_NUM`
map from each change's `issue:` field. This map is updated when issues are freshly minted so
Projects v2 can link them in the same pass.

### External-write chokepoint — `run_gh`

Every `gh` invocation routes through `run_gh`. Under `--dry-run`, the call is traced to stderr
and returns 0 immediately; real output (e.g. a created issue URL) is never available in dry mode.
On the live path, a `gh` failure is swallowed: the error is logged to stderr and the script
continues (best-effort contract).

### Per-change upsert — `mirror_change`

For each file in `active/` and `archive/`:

1. **Label reconciliation.** Constructs the `docket:` label set for the change:
   - `docket:status/<status>` — always present.
   - `docket:priority/<priority>` — when `priority:` is non-empty.
   - `docket:readiness/<token>` or `docket:waiting/<reason-slug>` — for `proposed` changes only,
     derived from `readiness()`.
   Each label is created or force-updated via `gh label create --force` before being attached.

2. **Issue body.** Built by `build_body`: a generated-mirror preamble (noting that edits/comments
   are not read back), a bold status/priority/id line, the first one-to-two sentences from
   `## Why` (distilled via `awk`), and a **Links** section with `blob/` URLs for the change file,
   spec, plan, results, and each ADR (using `--metadata-branch` for spec/ADR/change-file links and
   `--integration-branch` for plan/results links). All links use `blob()`, which degrades to bare
   paths when `--repo` is unset.

3. **Create or update.**
   - **No `issue:` field (new):** calls `gh issue create` with title, body, and labels. Parses
     the returned URL for the issue number and prints `issue-minted <id> <number>` to stdout for
     the caller to persist into `issue:` on the metadata branch. The script does no git writes.
     In dry mode prints `issue-minted <id> (dry-run)`.
   - **`issue:` present (existing):** calls `gh issue edit` with updated title, body, and
     `--add-label` for each current label. Labels are additive within the `docket:` namespace;
     stale `docket:*` labels are not removed here (idempotent across reruns).

4. **Close state.** For `done` changes: `gh issue close --reason completed`. For `killed` changes:
   `gh issue close --reason "not planned"`. The close call uses the issue number from `issue:` if
   already set, or the freshly minted number from a create in the same pass — so a change that is
   terminal on its first sync closes in a single run rather than requiring a second pass. Non-terminal
   changes are left open.

### Projects v2 sync — `sync_projects`

The optional half of the `github` surface. Any failure (missing `project` scope, network error,
unresolvable owner, bad board number) logs to stderr and returns 0 — Issues are always fully
mirrored regardless of Projects status. The Board pass likewise never fails because of Projects.

**Board resolution:**
- `--project OWNER/NUMBER` — uses the existing board; no create.
- `--auto-create-project` (with `--project` unset) — mints a private board under the repo owner
  (or `--project-owner`), seeds a `Docket Status` single-select field with five options:
  `proposed`, `in-progress`, `blocked`, `deferred`, `implemented`. Prints
  `project-minted <owner> <number>` to stdout for the Board pass to write into `.docket.yml` on
  the default branch. The script does no git writes.
- Neither set — skips Projects silently (Issues still mirrored).

**`github_project: auto` (change 0101) — documentation-only today.** The literal lowercase `auto`
is the explicit spelling of "unminted", identical in effect to an absent key, so that
`.docket.example.yml` can ship the key active at its default instead of as a commented-out note.
It changes no behavior, because **nothing currently reads `github_project` from config at all**:
this script resolves its board solely from `--project` / `--auto-create-project`, and
`docket-status.sh` populates those only from its own CLI flags — the `--project) PROJECT_FLAG="$2"`
arg-parse arm, forwarded through `${PROJECT_FLAG:+--project "$PROJECT_FLAG"}` — which no skill
passes. The key's only live effect anywhere is the coordination-key fence in `docket-config.sh`'s
`for _fkey in metadata_branch integration_branch …` loop, which warns-and-ignores it in the two
machine-scoped layers. When the
config read is eventually wired, `auto` must resolve to the same "no board configured" state as an
absent key, and the `project-minted` write-back must **overwrite** a literal `auto` rather than
mistake it for a minted board reference.

**Item sync.** For every change in `ISSUE_NUM` (seeded above, including freshly minted issues):
adds the issue as a board item via `gh project item-add`, then sets its `Docket Status` option via
`gh project item-edit --single-select-option-id`. Terminal changes (`done`, `killed`) and changes
with no issue number are skipped — terminal status is expressed by the closed issue, not a column.

### Drive loop

Calls `mirror_change` for every file found in `active/` and `archive/`, then calls
`sync_projects`. Exits 0 unconditionally.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Mirror completed (or degraded best-effort). All issues and Projects updates were attempted; individual `gh` failures are logged and swallowed. |
| 2 | Missing or invalid argument (`--changes-dir` absent; unknown flag). |

Note: the script never exits non-zero due to a `gh` failure. The best-effort contract means the
caller's build is never aborted by a network or auth issue.

## Invariants

- **One-way only.** GitHub issues, labels, comments, and Projects state are never read back into
  change files. The change file is always the source of truth.
- **Sole writer of issue open/closed state and reason.** PRs only reference issues; they never
  emit `Closes #N`.
- **No git writes.** `issue-minted` and `project-minted` lines are printed to stdout; the caller
  (the Board pass) persists them into the metadata branch. This script never touches the git index.
- **Best-effort.** Missing network, expired auth, or missing `project` scope degrades gracefully:
  each `gh` call is individually swallowed, logged, and skipped. Projects failures never block
  Issues; the whole mirror never blocks the build. Re-running heals missed updates.
- **Deterministic and idempotent.** Same change files + same GitHub state → same `gh` calls.
  Safe to re-run at any time.
- **Metadata worktree required.** `--changes-dir` must point to the metadata branch worktree (where
  `active/` is populated). The integration-branch checkout prunes `active/`; the wrong-tree guard
  warns but does not abort.
- **`docket:` label namespace.** All labels created and attached by this script are prefixed
  `docket:`. No foreign labels are created or removed.
- **Issue body is generated output.** The preamble explicitly states that edits and comments on
  the GitHub issue are not read back — users are directed to the change file.
