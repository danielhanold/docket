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
| `--board-only` | Only run steps 1–4 (config/bootstrap, worktree sync, board pass, backlog pass) and exit; skip sweep detection/execution, health checks, judgment emission, and integration sync. |
| `--repo OWNER/REPO` | GitHub repo for PR-link resolution and sweep merge detection. Defaults to deriving from the `origin` remote (see `render-board.sh`) and, for sweep detection, from `gh repo view` when unset. |
| `--project OWNER/NUMBER` | GitHub Project to sync during the github board surface. Passed through to `github-mirror.sh`. |
| `--auto-create-project` | Create the GitHub Project if `--project` doesn't resolve. Passed through to `github-mirror.sh`. |
| `--project-owner OWNER` | Owner to create the project under when auto-creating. Passed through to `github-mirror.sh`. |
| `-h`, `--help` | Print the usage synopsis (script header lines 2–12) and exit 0. |

Any other argument is a hard error (`docket-status: unknown argument: <arg>`, exit 2).

Configuration (`DOCKET_MODE`, `METADATA_WORKTREE`, `METADATA_BRANCH`, `INTEGRATION_BRANCH`,
`CHANGES_DIR`, `ADRS_DIR`, `BOARD_SURFACES`, `TERMINAL_PUBLISH`, `BOOTSTRAP`, …) comes entirely from
`docket-config.sh --export`, evaluated with `eval` into `main()`'s scope by the shared
`docket_preflight` call at the top of `main` (see Behavior, steps 1–2). The script defines no
config of its own.

## Behavior

The pass runs as a fixed 8-step sequence:

**1–2. Config, bootstrap gate, and metadata worktree ensure + sync — delegated.** Step-0 sync is
delegated to the shared `scripts/lib/docket-preflight.sh` (`docket_preflight`), the single sync
implementation shared with the `docket.sh` facade. `main()` calls `docket_preflight "$SCRIPTS_DIR"`,
which: (1) runs config export (normally `docket-config.sh --export`, overridable via the
`CONFIG_EXPORT_CMD` mock seam) and `eval`s the output into `main()`'s scope — a non-zero exit from
config export is a hard error; gates the resulting `BOOTSTRAP` verdict (`PROCEED` continues;
`STOP_MIGRATE` and `CREATE_ORPHAN` each print a remedy to stderr and return non-zero; any other
value is an unknown-verdict hard error); then (2) in `DOCKET_MODE=docket`, ensures the metadata
worktree (`METADATA_WORKTREE`, default `.docket`) exists — creating it from `METADATA_BRANCH` or
`origin/METADATA_BRANCH` if missing — then fetches and rebase-pulls `METADATA_BRANCH` inside it; in
non-docket mode, rebase-pulls the current checkout directly. A non-zero return from
`docket_preflight` (config export failure, bootstrap gate, or an unusable metadata worktree) is a
hard error and this script exits 1 immediately.

**3. Board pass**, once per surface token in the space-separated `BOARD_SURFACES` config value.
The reserved token **`none`** is the deliberate off-state and emits a positive `board off` line
(change 0069) — never silence. An **empty** `BOARD_SURFACES` is a wiring bug, not a
configuration: the pass exits 2 with a diagnostic (change 0071), because `docket-config.sh` never
emits an empty value and an unresolved config must never masquerade as a disabled board.
- **inline** — Renders and writes the board through `board-refresh.sh` (change 0059), which owns
  the surface gate and the atomic, truncation-safe replace of `BOARD.md`; this script never calls
  `render-board.sh` to produce the board. A failed render leaves the existing `BOARD.md`
  untouched, logs to stderr, and is treated as success for sequencing purposes (best-effort) — but
  it emits the positive stdout line `board inline failed` (change 0071 review, finding 1), never
  just the stderr diagnostic: the report-line channel must never go silent on a path that still
  exits 0, or a must-land caller keying on the report line (never the exit code) would read the
  silence as "the board landed". This line is terminal, not retryable. If
  `BOARD.md` is unchanged, `board inline clean` requires TWO things to hold, not just a clean
  working tree (change 0071 review, finding 3): the render produced no diff, **and** the local
  metadata branch carries no commit touching `BOARD.md` that is unpushed relative to its upstream
  (`@{u}..HEAD`, count > 0; no upstream at all counts as nothing-to-push, not an error). A clean
  working tree alone is not evidence the board landed — a prior run may have committed it locally
  and then failed to push. When the tree is clean but such an unpushed commit exists, nothing new
  is committed; execution falls through into the same push/rebase retry loop as a changed render,
  reporting `board inline changed pushed` / `board inline changed push-failed` from its outcome.
  When the render actually changed `BOARD.md`, it is `git add`ed and committed with message
  `docket: board refresh`, then pushed with up to 5 retry attempts: on push failure it
  rebase-pulls; if the rebase conflicts only on `BOARD.md`, it regenerates through the same gated
  helper (never a raw redirect) and continues the rebase; a rebase conflict on anything else, or a
  failed regeneration mid-rebase, aborts the rebase and stops retrying.
- **github** — Runs `github-mirror.sh` (passing through `--repo`, `--project`,
  `--auto-create-project`, `--project-owner`), best-effort. Lines it emits of the shape
  `issue-minted <id> <n>` / `project-minted <id> <n>` are translated to `minted issue <id> <n>` /
  `minted project <id> <n>` on this script's stdout; the surface's own success/failure is
  reported as one final `board github ok|failed` line regardless of what was minted.
- Any other token is an unrecognized-surface warning on stderr (non-fatal) — and, alongside it, a
  positive `board <token> unknown` stdout line (change 0071 review, finding 1) so a typo can never
  silently vanish from the report the way it used to when the warning lived on stderr alone. This
  line is terminal, not retryable — a typo is a config problem, not a transient one.

**4. Backlog pass — UNGATED, once per path.** Runs `render-board.sh --format digest` and passes
its lines through (`backlog <status> <count>` rollups, then one `change <id> <status> <readiness>
<slug>` line per active change). It runs **regardless of `board_surfaces`**, because **the digest
is report output, not a board surface**: it persists nothing, commits nothing, pushes nothing, and
never touches `BOARD.md`. That boundary is exactly what lets `board_surfaces: []` keep meaning "no
board is rendered or committed" while backlog state still reaches the report. Best-effort: a
digest failure logs to stderr, emits no digest lines, and never aborts the pass. Resolution is
**not** reimplemented here — `render-board.sh` stays the single owner of readiness.

The digest is a snapshot of the change files **at the moment it runs**, so it is called **once per
path** and the placement is part of the contract:

- **Under `--board-only`** (no sweep runs) it fires **here**, right after the board pass: the
  **state as-is** projection. That is what makes the "just show me the backlog" path useful in a
  board-off repo, where it previously did nothing at all. The process then prints `pass ok` and
  exits 0 — no sweep, health checks, judgment, or integration sync.
- **On a full pass** it fires **after** steps 5–7, once the sweep and the check/judgment lines are
  done: the **state after the pass** projection. **A change swept to `done` during this very pass
  therefore appears in the digest as `done` — not as the `implemented` it was when the pass
  began** — and is counted in `backlog done <n>`, never in `backlog implemented <n>`. A pre-sweep
  snapshot would make the report contradict its own `swept` lines, and since the digest is the
  sole backlog channel, that staleness would have no corrective path.

Report line order on a full pass is therefore: board → sweep lines → check/judgment lines →
backlog digest → `pass ok`.

**5. Batched sweep detection.** `detect_merged` scans `active/*.md` for `status: implemented`
changes, resolves each PR's merge state with one batched `gh api graphql` call keyed by change ID
(for changes with a known `pr:` number) plus a per-change `gh pr list --head feat/<slug> --state
merged` fallback for changes without one, and emits merged changes as TAB-separated
`<id>\t<slug>\t<pr>\t<merged-date>` (merged-date is the UTC date portion of GitHub's `mergedAt`,
never derived from local time / `now()`). Any `gh`/network/parse failure is swallowed and reported
as `sweep-skipped <reason>` (`gh-unavailable` or `repo-unresolved`); detection never aborts the
pass.

**6. Sweep execution**, one change at a time, chaining the ADR-0035 close-out scripts in order:
rebase-pull the metadata worktree, then (skipping silently if the change is already archived or
already `done`/`killed` — idempotent no-op) `archive-change.sh` → locate the archived file →
`render-change-links.sh` → **artifacts refresh** (see below) → `terminal-publish.sh` (always
passed `--enabled "${TERMINAL_PUBLISH:-false}"`, so the headless sweep honors the repo's publish
policy — unset defaults to no publish since change 0084; a suppressed publish is a no-op that
exits 0 and is logged, never a failure) →
`cleanup-feature-branch.sh`. Each step's failure emits `sweep-failed <id> <step> <reason>` and
abandons the rest of that change's close-out, but the loop always continues to the next change;
the **artifacts refresh** and a `cleanup-feature-branch.sh` failure are the two exceptions — both
still emit the terminal `swept`/`harvest` lines for that change. Full success for a change emits
`swept <id> <merged-date>` followed by `harvest <id> <archived-path>`. Self-heal is idempotent for
a failure at `sync` (rebase-pull) or `archive`, and for a `cleanup` failure (all retry cleanly next
pass) — but a `sweep-failed` at `render-change-links` (`skipped-publish`, i.e. the renderer itself
exited non-zero) or at `terminal-publish` leaves the change **archived but its terminal record
unpublished**, invisible to future detection (which only scans `active/*.md`), and requires a manual
`terminal-publish.sh --id <id> --enabled true` follow-up. The knob narrows only the
`terminal-publish` leg: under `terminal_publish: false` that step is a no-op that cannot fail, so
this recovery path never arises there — but the renderer leg still can fail in such a repo, leaving
the archived change with a stale `## Artifacts` block on `metadata_branch` that no later sweep
resumes; the follow-up there is a manual re-render on the metadata branch, not a publish.

**6a. The artifacts refresh (change 0075).** After `render-change-links.sh` rewrites the archived
change's `## Artifacts` block in the metadata worktree, the sweep **commits and pushes** that file
on `metadata_branch` (`docket(<id>): refresh artifacts links`) — but only when the render actually
changed bytes; an unchanged file is a silent no-op. `$mw` (the metadata worktree, and therefore the
`$archived` pathspec this step tests) is **absolute** — anchored to the repo's MAIN worktree by
`lib/docket-root.sh`, so the step means the same thing from every CWD, including a linked worktree.
**This step never aborts the close-out.** A failure emits `sweep-failed <id> render-change-links
commit-failed` or `sweep-failed <id> render-change-links push-failed` on the report channel and the
sweep **continues** to `terminal-publish.sh` and `cleanup-feature-branch.sh`. That posture is
deliberate: a stale link block is cosmetic and self-heals on a manual re-render, whereas an aborted
close-out leaves the change archived-but-unpublished (invisible to every future sweep) plus an
orphaned worktree and remote branch — a strictly worse, non-self-healing state. Callers key on the
report **line**, never on the exit code.

**7. Health checks.** Runs `board-checks.sh` over the current changes-dir and metadata/integration
branches, and prefixes each of its TSV findings as `check <check-id> <change-id> <message>` on
this script's stdout. Also emits one `judgment blocked <id> <blocked_by-text>` line per `active`
change with `status: blocked`, leaving the actual re-examination judgment to the caller/skill.
Both are best-effort/warn-only: a clean tree, or a `board-checks.sh` failure, produces no extra
output and never aborts the pass.

**8. Integration sync.** If step 6 swept at least one change (`swept ` line count ≥ 1), runs
`sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH"` once, best-effort
(failures are swallowed). Skipped entirely when nothing was swept. Runs after the full-pass
backlog digest and emits nothing on stdout, so it does not affect the report's line order.

### Failure postures (summary)

- **Board pass: best-effort, but never silent.** A failed inline render or failed github mirror
  never aborts the pass; it degrades to a diagnostic on stderr and (for inline) leaves the
  last-known-good `BOARD.md` in place — but every path, including a failed render and an unknown
  surface token, also emits a positive `board …` stdout line (`board inline failed`,
  `board <token> unknown`), never just the stderr diagnostic (change 0071 review, finding 1). A
  caller that sees the pass exit 0 with **no** `board …` line at all has found a bug in this
  script, not evidence the board landed.
- **Sweep: per-change log-and-continue.** A failed step for one change emits `sweep-failed` and
  abandons only the rest of *that* change's close-out (except a cleanup failure and an
  artifacts-refresh `commit-failed`/`push-failed`, which report and continue, still emitting
  `swept`/`harvest`); the sweep loop proceeds to the next change regardless. The close-out is never
  abandoned for a cosmetic reason: publishing the terminal record and tearing down the branch +
  worktree outrank a stale link block (change 0075).
- **Health checks: warn-only.** Findings and judgments are reported, never enforced; the script
  never modifies a change file or blocks the pass because of a finding.

## Output contract

All report lines are stdout, one shape per line, diagnostics go to stderr:

| Shape | Meaning |
|---|---|
| `board inline clean` | Inline render matched the existing `BOARD.md` AND there is nothing unpushed touching it — no local commit on `BOARD.md` sits ahead of its upstream. Attests the board is caught up on the remote, not merely that the working tree is clean. |
| `board inline changed pushed` | `BOARD.md` changed and the commit was pushed successfully. |
| `board inline changed push-failed` | `BOARD.md` changed and committed locally, but push retries were exhausted or a rebase conflict outside `BOARD.md` forced an abort. |
| `board github ok` | `github-mirror.sh` exited 0. |
| `board github failed` | `github-mirror.sh` exited non-zero. |
| `board off` | `BOARD_SURFACES` is the reserved token `none` — the board is deliberately disabled (`board_surfaces: []`); no surface was rendered and nothing was committed. Positive evidence of a deliberate skip, never silence. |
| `board inline failed` | The `inline` render failed; the existing `BOARD.md` was left untouched (best-effort — the pass still continues to `pass ok`). Terminal, not retryable (change 0071 review, finding 1). |
| `board <token> unknown` | `<token>` in `BOARD_SURFACES` matched neither `inline`, `github`, nor `none`; warned on stderr and ignored. Terminal, not retryable (change 0071 review, finding 1). |
| `minted issue <id> <n>` | Passthrough of `github-mirror.sh`'s `issue-minted <id> <n>`. |
| `minted project <id> <n>` | Passthrough of `github-mirror.sh`'s `project-minted <id> <n>`. |
| `backlog <status> <count>` | One rollup per non-zero status across the active + archived change files (from the ungated backlog pass). On a full pass these are **post-sweep** counts: a change swept this pass is counted under `done`, not `implemented`. |
| `change <id> <status> <readiness> <slug>` | One line per **active** change, as of the moment the backlog pass ran (post-sweep on a full pass, so a change swept this pass has no `change` line at all — it is archived). `<readiness>` is `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`, `waiting-on-<N>-needs-merge`, or `-` when readiness does not apply (any non-`proposed` status). |
| `swept <id> <date>` | Change `<id>` fully closed out (archived, links refreshed, terminal record published only if the repo opted in with `terminal_publish: true`, branch cleaned up) as of `<date>` (UTC, from merge). |
| `harvest <id> <path>` | The archived file path for a swept change — a hook for the caller to harvest learnings. `<path>` is absolute (since change 0075, anchored to the main worktree via `lib/docket-root.sh`) — previously relative to the process CWD. |
| `sweep-failed <id> <step> <reason>` | Step `<step>` (`sync`, `archive`, `render-change-links`, `terminal-publish`, or `cleanup`) failed for change `<id>` with `<reason>`; that change's remaining close-out steps were abandoned — **except** for `cleanup` and for the artifacts-refresh reasons `commit-failed` / `push-failed` (step 6a), after which the close-out continues and the change still reports `swept`/`harvest`. |
| `sweep-failed <id> render-change-links commit-failed\|push-failed` | The refreshed `## Artifacts` block could not be committed/pushed on `metadata_branch` (step 6a). Cosmetic and non-terminal: `terminal-publish.sh` and `cleanup-feature-branch.sh` **still ran**, and the change is still reported `swept`. The archived record on `metadata_branch` keeps its previous link block until a manual re-render. |
| `sweep-skipped <reason>` | Batched merge detection itself was skipped (`gh-unavailable` or `repo-unresolved`); no changes were evaluated this pass. |
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. |
| `judgment blocked <id> <text>` | Change `<id>` is `status: blocked`, with its `blocked_by:` text, for the caller to re-judge. |
| `pass ok` | The orchestrator ran to completion. Always the last line of a successful pass; **stdout is never empty**. A hard error exits non-zero and never prints it, so it is a reliable completion signal. |

## Exit codes

- `0` — the pass completed (and printed `pass ok` as its last line). Findings, `sweep-failed`,
  `sweep-skipped`, `board *-failed`, `board off`, and `judgment` lines on stdout are all normal,
  expected pass outcomes, not errors — **a thin report is the success case.**
- non-zero — a hard error only: config export failure, an unrecognized `BOOTSTRAP` verdict,
  `STOP_MIGRATE`/`CREATE_ORPHAN` bootstrap gate (exit 1), an unusable metadata worktree (create or
  sync failure, exit 1), an unknown CLI argument (exit 2), or `BOARD_SURFACES` was empty (or
  whitespace-only — defence-in-depth, change 0071 review finding 6) / `none` was combined with
  another surface (a wiring bug — change 0071).

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
  `docket-config.sh --export` call) — all overridable in tests for hermetic runs. The shared
  `docket_preflight` (`scripts/lib/docket-preflight.sh`) honors the same `GIT` and
  `CONFIG_EXPORT_CMD` seams.
