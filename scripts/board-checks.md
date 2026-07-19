# board-checks.sh — mechanical docket-status health checks

## Purpose

Performs the deterministic git-only health checks over the change files
(`active/` and `archive/`) and cross-references integration-branch commit subjects against them
and emits one TAB-separated finding per line on stdout.
It is the sole mechanical checker; the caller (`docket-status`) surfaces the findings
and owns human-facing display. The one judgment-bearing check — `blocked_by:` re-examination
— stays model-driven in the skill and is NOT performed here. Introduced in change 0023.

## Usage

```
board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
                 [--lease-ttl-hours N]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the directory that contains `active/` and `archive/` subdirectories. |
| `--metadata-branch BR` | yes | The branch (e.g. `docket` or `main`) against which spec paths are resolved via `git cat-file -e`. |
| `--integration-branch BR` | yes | The branch against which `plan:` / `results:` paths for `done` changes are resolved. |
| `--strict` | no | Exit 1 if any finding is emitted (a CI gate). Default: exit 0 regardless of findings. |
| `--lease-ttl-hours N` | no | Claim-lease TTL (hours) for the `stale-in-progress` check's `claimed_at:` signal. Default `72` when absent, so standalone use stays sane. |

**Output format:** every finding is `<check-id>\t<change-id>\t<message>` on stdout, sorted
by `(check-id asc, change-id numeric asc)`. A clean tree produces no output.

**Mock seams:** `GIT="${GIT:-git}"` and `NOW="${NOW:-$(date +%s)}"` — override in tests
for hermetic staleness checks and git injection.

## Behavior

### Check enumeration

The script walks every `*.md` file under `active/` and `archive/` (sorted), sources
`lib/docket-frontmatter.sh`, and calls `resolve_deps` once to populate the dependency
state maps. Then it runs the following named checks:

**`broken-spec`** — The change has a non-empty `spec:` field, `trivial: false` is not set,
and the spec path is absent on `--metadata-branch` (checked via
`git cat-file -e <metadata-branch>:<path>`). Changes with `trivial: true` are exempt even
if they carry an unresolvable spec path (carve-out).

**`broken-plan-results`** — The change has `status: done` and at least one of its `plan:` or
`results:` paths is absent on `--integration-branch`. Carve-out: changes at `status:
implemented` are never flagged — their build artifacts still live on the unmerged feature
branch and are not yet on the integration branch.

**`stale-in-progress`** — The change has `status: in-progress`. Two independent signals feed
this check (change 0089); at most one finding is emitted per change:

- **Branch idle >3 days.** `branch:` is set and a `feat/<slug>` ref resolves (`refs/heads/<branch>`
  or `refs/remotes/origin/<branch>`), and its newest commit is older than 3 days (compared against
  `$NOW`). Message: `branch <branch> idle >3 days (last commit <N>d ago)` — unchanged from before
  0089.
- **Claim lease expired.** `claimed_at:` is set, parses via `iso_to_epoch`, and
  `NOW - claimed_at > --lease-ttl-hours * 3600`. This is the signal that catches the
  **crashed-before-branch** blind spot the branch-age signal misses (a claim can expire before any
  branch is ever pushed). Its message depends on whether a branch ref exists:
  - **No branch ref** (the reclaimable case): `claim lease expired <N>h ago; no feature branch —
    self-heal with docket.sh reclaim-claims [reclaimable]`. The trailing **`[reclaimable]`** token
    is a **stable, machine-readable suffix** — `docket-status` keys on its literal presence to
    decide whether to print a reclaim-sweep remedy. Do not reword or relocate it.
  - **Branch ref exists**: `claim lease expired <N>h ago; branch <branch> exists — needs your
    review (not auto-reclaimable)`. A live branch means a human should look before anything
    auto-reclaims, so this case never carries `[reclaimable]`.

Priority when both signals fire on the same change (branch exists, idle >3 days, AND the lease is
separately expired): the branch-idle message wins and is the only finding emitted — idle-branch
evidence is the older, more specific signal and is preserved unchanged.

**`merge-gate-stall`** — The change is build-ready (`status: proposed` with a spec or
`trivial: true`) and `resolve_deps` determined it is blocked because its worst-unmet
dependency is stuck at `implemented` (needs your merge). The finding message names the
blocking dependency ID.

**`stale-finalize-blocked`** — The change has `status: implemented` and carries the
`## Finalize blocked` body section (`finalize_blocked`), and that marker has outlived a fixed
staleness horizon (`FINALIZE_BLOCKED_STALE_SECS`, hardcoded 72 h). Marker age is the change file's
last-commit timestamp (`git log -1 --format=%ct -- <file>`) — the marker heading is deliberately
undated and its in-body date is model-authored, so git's clock is the tamper-proof signal. The
finding names the age in hours and advises re-running finalize with the id. Git-only and warn-only:
it cannot probe whether the underlying cause still holds (that needs `gh`/network, forbidden here),
so it fires on **any** marker past the horizon — a still-blocked marker that old is itself worth a
human glance. It never mutates the change file or auto-clears the marker; that stays
`docket-finalize-change`'s job. The horizon is a hardcoded constant (mirroring `stale-in-progress`'s
own `3*86400` branch-idle horizon), not a config knob.

**`merged-orphan`** — A change id is referenced by a commit *subject* on `--integration-branch`
while the change is still non-terminal (a file under `active/`, not yet archived). This is the
classic orphan: work merged, but the docket record was never closed out. It is a git-history
signal that complements the PR-status sweep — it catches orphans the sweep structurally cannot
(squash-merge under a differently-named branch, an unrecorded `pr:`, or a sweep that never ran).
The message names the evidence commit (short sha + subject). Warn-only; a legitimately
just-merged change has already been archived by the time health checks run (they run after the
sweep), and a transient orphan from a skipped sweep self-clears next pass.

**`unknown-commit-ref`** — A change id is referenced by an `--integration-branch` commit subject
but no change file with that id exists under `active/` or `archive/` (a typo'd or deleted id).
The change-id column is the referenced id; the message names the evidence commit.

**Id-extraction grammar (both checks).** Ids are parsed from commit *subject* lines only, in
exactly two docket-convention forms: a numeric conventional-commit scope `<type>(<id>):`
(e.g. `docket(0085):`, `results(0085):`) and a `(change <id>)` tag (conventionally trailing,
matched anywhere in the subject; e.g. `… (change 0085)`).
Zero-padding is tolerated and normalized to the integer value. Bare `#NNNN` and body text are
deliberately excluded — `#NNNN` collides with PR numbers, and subject-only parsing drops free-text
mentions. The full integration-branch history is scanned on every run (stateless; no `--since`
window, no persisted cursor).

**`dep-cycle`** — A depth-first search (DFS) over `depends_on:` edges marks every node that
lies on a cycle (including both members of a mutual `A→B→A` loop and self-loops `C→C`).
Only edges to known change IDs (present in the file set) are followed; dangling references
to unknown IDs are silently skipped. Every node on a cycle is emitted as a separate finding.

**`malformed-id`** — Guard/carve-out, not counted among the named checks above. A change file
whose `id:` field is non-empty but non-integer emits a `malformed-id` finding (using the raw
string as the change-id column). The file is then skipped for all other checks.

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
