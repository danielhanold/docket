# scripts/docket-status.sh — deterministic docket-status orchestrator

## Purpose

One-invocation, deterministic orchestrator for the docket-status pass. It sequences the shared
docket scripts (`docket-config.sh`, `render-board.sh`, `github-mirror.sh`, `archive-change.sh`,
`render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`, `board-checks.sh`,
`sync-integration-branch.sh`) inside one process and emits a single line-oriented report on
stdout. It performs no mechanics of its own beyond sequencing and thin glue — each shared script
still owns its own contract. Change 0058.

## Usage

```
docket-status.sh [--board-only] [--repo OWNER/REPO] [--project OWNER/NUMBER]
                  [--auto-create-project] [--project-owner OWNER]
docket-status.sh -h | --help
```

| Flag | Description |
|---|---|
| `--board-only` | Only run steps 1–3 (config/bootstrap, worktree sync, board pass) and exit; skip sweep detection/execution, health checks, judgment emission, and integration sync. |
| `--repo OWNER/REPO` | GitHub repo for PR-link resolution and sweep merge detection. Defaults to deriving from the `origin` remote (see `render-board.sh`) and, for sweep detection, from `gh repo view` when unset. |
| `--project OWNER/NUMBER` | GitHub Project to sync during the github board surface. Passed through to `github-mirror.sh`. |
| `--auto-create-project` | Create the GitHub Project if `--project` doesn't resolve. Passed through to `github-mirror.sh`. |
| `--project-owner OWNER` | Owner to create the project under when auto-creating. Passed through to `github-mirror.sh`. |
| `-h`, `--help` | Print the usage synopsis (script header lines 2–12) and exit 0. |

Any other argument is a hard error (`docket-status: unknown argument: <arg>`, exit 2).

Configuration (`DOCKET_MODE`, `METADATA_WORKTREE`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`,
`CHANGES_DIR`, `ADRS_DIR`, `BOARD_SURFACES`, `BOOTSTRAP`, …) comes entirely from `docket-config.sh
--export`, evaluated with `eval` at the top of `main`. The script defines no config of its own.

## Behavior

The pass runs as a fixed 7-step sequence:

**1. Config + bootstrap gate.** Runs `config_export` (normally `docket-config.sh --export`,
overridable via the `CONFIG_EXPORT_CMD` mock seam) and `eval`s the output. A non-zero exit from
config export is a hard error. The resulting `BOOTSTRAP` verdict is then gated: `PROCEED`
continues; `STOP_MIGRATE` and `CREATE_ORPHAN` each print a remedy to stderr and exit 1; any other
value is an unknown-verdict hard error (exit 1).

**2. Metadata worktree ensure + sync.** In `DOCKET_MODE=docket`, ensures the metadata worktree
(`METADATA_WORKTREE`, default `.docket`) exists — creating it from `METADATA_BRANCH` or
`origin/METADATA_BRANCH` if missing — then fetches and rebase-pulls `METADATA_BRANCH` inside it.
In non-docket mode, rebase-pulls the current checkout directly. Any fetch/pull/create failure is a
hard error (exit 1) with a diagnostic on stderr.

**3. Board pass**, once per surface token in the space-separated `BOARD_SURFACES` config value
(no surfaces configured is a silent no-op):
- **inline** — Renders the board via `render-board.sh` into a `BOARD.md.tmp` file next to
  `BOARD.md`. A failed render (non-zero exit or empty output) discards the tmp file, leaves the
  existing `BOARD.md` untouched, logs to stderr, and is treated as success for sequencing
  purposes (best-effort). If the render is byte-identical to the existing `BOARD.md`, the tmp
  file is discarded and nothing is committed. Otherwise the tmp file replaces `BOARD.md`, is
  `git add`ed and committed with message `docket: board refresh`, then pushed with up to 5
  retry attempts: on push failure it rebase-pulls; if the rebase itself conflicts only on
  `BOARD.md`, it re-renders the board fresh (discarding the conflicted merge state) and
  continues the rebase; a rebase conflict on anything else, or a failed re-render mid-rebase,
  aborts the rebase and stops retrying.
- **github** — Runs `github-mirror.sh` (passing through `--repo`, `--project`,
  `--auto-create-project`, `--project-owner`), best-effort. Lines it emits of the shape
  `issue-minted <id> <n>` / `project-minted <id> <n>` are translated to `minted issue <id> <n>` /
  `minted project <id> <n>` on this script's stdout; the surface's own success/failure is
  reported as one final `board github ok|failed` line regardless of what was minted.
- Any other token is an unrecognized-surface warning on stderr (non-fatal).

If `--board-only` was passed, the process exits 0 here — no sweep, health checks, judgment, or
integration sync.

**4. Batched sweep detection.** `detect_merged` scans `active/*.md` for `status: implemented`
changes, resolves each PR's merge state with one batched `gh api graphql` call keyed by change ID
(for changes with a known `pr:` number) plus a per-change `gh pr list --head feat/<slug> --state
merged` fallback for changes without one, and emits merged changes as TAB-separated
`<id>\t<slug>\t<pr>\t<merged-date>` (merged-date is the UTC date portion of GitHub's `mergedAt`,
never derived from local time / `now()`). Any `gh`/network/parse failure is swallowed and reported
as `sweep-skipped <reason>` (`gh-unavailable` or `repo-unresolved`); detection never aborts the
pass.

**5. Sweep execution**, one change at a time, chaining the ADR-0035 close-out scripts in order:
rebase-pull the metadata worktree, then (skipping silently if the change is already archived or
already `done`/`killed` — idempotent no-op) `archive-change.sh` → locate the archived file →
`render-change-links.sh` (committing and pushing the refreshed links if the archived file
changed) → `terminal-publish.sh` → `cleanup-feature-branch.sh`. Each step's failure emits
`sweep-failed <id> <step> <reason>` and abandons the rest of that change's close-out, but the
loop always continues to the next change; a `cleanup-feature-branch.sh` failure is the one
exception — it still emits the terminal `swept`/`harvest` lines for that change since publish
already succeeded. Full success for a change emits `swept <id> <merged-date>` followed by
`harvest <id> <archived-path>`.

**6. Health checks.** Runs `board-checks.sh` over the current changes-dir and metadata/integration
branches, and prefixes each of its TSV findings as `check <check-id> <change-id> <message>` on
this script's stdout. Also emits one `judgment blocked <id> <blocked_by-text>` line per `active`
change with `status: blocked`, leaving the actual re-examination judgment to the caller/skill.
Both are best-effort/warn-only: a clean tree, or a `board-checks.sh` failure, produces no extra
output and never aborts the pass.

**7. Integration sync.** If step 5 swept at least one change (`swept ` line count ≥ 1), runs
`sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH"` once, best-effort
(failures are swallowed). Skipped entirely when nothing was swept.

### Failure postures (summary)

- **Board pass: best-effort.** A failed inline render or failed github mirror never aborts the
  pass; it degrades to a diagnostic on stderr and (for inline) leaves the last-known-good
  `BOARD.md` in place.
- **Sweep: per-change log-and-continue.** A failed step for one change emits `sweep-failed` and
  abandons only the rest of *that* change's close-out (except cleanup failure, which still emits
  `swept`/`harvest`); the sweep loop proceeds to the next change regardless.
- **Health checks: warn-only.** Findings and judgments are reported, never enforced; the script
  never modifies a change file or blocks the pass because of a finding.

## Output contract

All report lines are stdout, one shape per line, diagnostics go to stderr:

| Shape | Meaning |
|---|---|
| `board inline clean` | Inline render matched the existing `BOARD.md`; nothing committed. |
| `board inline changed pushed` | `BOARD.md` changed and the commit was pushed successfully. |
| `board inline changed push-failed` | `BOARD.md` changed and committed locally, but push retries were exhausted or a rebase conflict outside `BOARD.md` forced an abort. |
| `board github ok` | `github-mirror.sh` exited 0. |
| `board github failed` | `github-mirror.sh` exited non-zero. |
| `minted issue <id> <n>` | Passthrough of `github-mirror.sh`'s `issue-minted <id> <n>`. |
| `minted project <id> <n>` | Passthrough of `github-mirror.sh`'s `project-minted <id> <n>`. |
| `swept <id> <date>` | Change `<id>` fully closed out (archived, links refreshed, terminal record published, branch cleaned up) as of `<date>` (UTC, from merge). |
| `harvest <id> <path>` | The archived file path for a swept change — a hook for the caller to harvest learnings. |
| `sweep-failed <id> <step> <reason>` | Step `<step>` (`sync`, `archive`, `render-change-links`, `terminal-publish`, or `cleanup`) failed for change `<id>` with `<reason>`; that change's remaining close-out steps were abandoned. |
| `sweep-skipped <reason>` | Batched merge detection itself was skipped (`gh-unavailable` or `repo-unresolved`); no changes were evaluated this pass. |
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. |
| `judgment blocked <id> <text>` | Change `<id>` is `status: blocked`, with its `blocked_by:` text, for the caller to re-judge. |

## Exit codes

- `0` — the pass completed. Findings, `sweep-failed`, `sweep-skipped`, `board *-failed`, and
  `judgment` lines on stdout are all normal, expected pass outcomes, not errors.
- non-zero — a hard error only: config export failure, an unrecognized `BOOTSTRAP` verdict,
  `STOP_MIGRATE`/`CREATE_ORPHAN` bootstrap gate (exit 1), an unusable metadata worktree (create or
  sync failure, exit 1), or an unknown CLI argument (exit 2).

## Invariants

- **Determinism.** Archive commits touch only the change file being archived. All dates are UTC,
  taken from GitHub's `mergedAt`, never `now()` or local time. Two runs over the same change files
  produce byte-identical `BOARD.md` output (inherited from `render-board.sh`'s determinism).
  Concurrent runs converge: a losing writer's push failure triggers a rebase-and-regenerate retry
  rather than a hand merge, and idempotent no-ops (already-archived, already-terminal changes)
  make repeated sweeps safe.
- **No duplication of shared-script internals.** This script only sequences the shared scripts and
  translates/prefixes their output lines; it does not reimplement rendering, archiving, health
  checks, or publishing logic that already lives in `render-board.sh`, `archive-change.sh`,
  `render-change-links.sh`, `terminal-publish.sh`, `cleanup-feature-branch.sh`, or
  `board-checks.sh`.
- **Surface-specific failure postures.** See Failure postures above: board best-effort, sweep
  per-change log-and-continue, health checks warn-only. No single surface's failure aborts another
  surface's work within the same pass.
- **Mock seams.** `GIT="${GIT:-git}"`, `GH="${GH:-gh}"`, `SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"`
  (where the chained scripts are looked up), and `CONFIG_EXPORT_CMD` (overrides the
  `docket-config.sh --export` call) — all overridable in tests for hermetic runs.
