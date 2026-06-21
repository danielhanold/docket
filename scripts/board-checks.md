# board-checks.sh — mechanical docket-status health checks

## Purpose

Performs the five deterministic git-only health checks over the change files
(`active/` and `archive/`) and emits one TAB-separated finding per line on stdout.
It is the sole mechanical checker; the caller (`docket-status`) surfaces the findings
and owns human-facing display. The one judgment-bearing check — `blocked_by:` re-examination
— stays model-driven in the skill and is NOT performed here. Introduced in change 0023.

## Usage

```
board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the directory that contains `active/` and `archive/` subdirectories. |
| `--metadata-branch BR` | yes | The branch (e.g. `docket` or `main`) against which spec paths are resolved via `git cat-file -e`. |
| `--integration-branch BR` | yes | The branch against which `plan:` / `results:` paths for `done` changes are resolved. |
| `--strict` | no | Exit 1 if any finding is emitted (a CI gate). Default: exit 0 regardless of findings. |

**Output format:** every finding is `<check-id>\t<change-id>\t<message>` on stdout, sorted
by `(check-id asc, change-id numeric asc)`. A clean tree produces no output.

**Mock seams:** `GIT="${GIT:-git}"` and `NOW="${NOW:-$(date +%s)}"` — override in tests
for hermetic staleness checks and git injection.

## Behavior

### Check enumeration

The script walks every `*.md` file under `active/` and `archive/` (sorted), sources
`lib/docket-frontmatter.sh`, and calls `resolve_deps` once to populate the dependency
state maps. Then it evaluates each of the following checks for every change:

**`broken-spec`** — The change has a non-empty `spec:` field, `trivial: false` is not set,
and the spec path is absent on `--metadata-branch` (checked via
`git cat-file -e <metadata-branch>:<path>`). Changes with `trivial: true` are exempt even
if they carry an unresolvable spec path (carve-out).

**`broken-plan-results`** — The change has `status: done` and at least one of its `plan:` or
`results:` paths is absent on `--integration-branch`. Carve-out: changes at `status:
implemented` are never flagged — their build artifacts still live on the unmerged feature
branch and are not yet on the integration branch.

**`stale-in-progress`** — The change has `status: in-progress`, `branch:` is set, the branch
exists locally (`git rev-parse --verify --quiet`), and its newest commit is older than 3 days
(compared against `$NOW`). Carve-out: if `branch:` is set but the branch does not exist
(just-claimed, branch not yet pushed), the change is not flagged.

**`merge-gate-stall`** — The change is build-ready (`status: proposed` with a spec or
`trivial: true`) and `resolve_deps` determined it is blocked because its worst-unmet
dependency is stuck at `implemented` (needs your merge). The finding message names the
blocking dependency ID.

**`dep-cycle`** — A depth-first search (DFS) over `depends_on:` edges marks every node that
lies on a cycle (including both members of a mutual `A→B→A` loop and self-loops `C→C`).
Only edges to known change IDs (present in the file set) are followed; dangling references
to unknown IDs are silently skipped. Every node on a cycle is emitted as a separate finding.

**`malformed-id`** — A change file whose `id:` field is non-empty but non-integer gets a
`malformed-id` finding (using the raw string as the change-id column). The file is then
skipped for all other checks.

### Sorting and strict mode

All findings are accumulated and sorted by `(check-id asc, change-id numeric asc)` before
output, ensuring deterministic ordering. With `--strict`, the script exits 1 if any findings
were emitted; otherwise it always exits 0.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | No findings (clean tree), or findings present without `--strict`. |
| 1 | One or more findings emitted and `--strict` was passed. |
| 2 | Missing or invalid argument (`--changes-dir` absent/not a directory, unknown flag). |

## Invariants

- **Git-only, offline.** No network calls, no `gh`. All checks use `git cat-file -e` or
  `git log`/`git rev-parse` against the local object store.
- **Warn-only, never auto-fixes.** The script emits findings and exits; it never modifies
  change files, the git index, or any branch.
- **STDOUT for findings, STDERR for errors.** Callers capture stdout to surface findings;
  usage errors and hard failures go to stderr.
- **Deterministic.** Same inputs produce identical output. Sorted by `(check-id, change-id)`
  so the caller can pipe or diff without ordering surprises.
- **`docket-status` owns display.** This script is an implementation detail of `docket-status`
  and surfaces nothing to the user directly — `docket-status` formats and presents the lines.
- **`blocked_by:` re-examination is model-driven.** The skill, not this script, evaluates
  whether a `blocked` change's blocking reason still holds. That judgment is intentionally
  outside the mechanical checker.
