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

**`publish-deferred`** — The change carries the `## Publish deferred` body section
(`publish_deferred`), written by `mark-publish-deferred.sh` when a terminal close-out's publish
step was **expected** (`terminal_publish: true`, docket-mode) but consciously deferred or blocked.
The finding names the integration branch the record never reached and the metadata branch it is
still confined to. **No status gate and no directory gate:** the marker is written on the
*archived* file, so gating on a lifecycle status would make it unreadable exactly where it is
written; presence is the entire state, and `terminal-publish.sh` removes the marker on a
successful publish (so a marker in the tree always means a pending deferral). It reads the marker
in the change file, **never** a `git cat-file -e origin/<integration>:<path>` set-diff — a
branch-set diff would reintroduce the standing detector change 0083 deliberately declined, fire
forever under `terminal_publish: false`, and break this script's git-only/offline invariant.
Warn-only; it never mutates the change file.

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

**`field-domain`** — A frontmatter value that is well-formed *text* but outside its field's
*domain*. These are the four fields the board renderers consume; a value outside the domain does
not error, it silently drops the change's row from every board surface (`status`, `slug`) or
injects columns into it (`title`). One finding per violated field, per change.

| Field | Domain | Empty | Failure mode without the check |
|---|---|---|---|
| `status` | one of the seven lifecycle statuses (`DOCKET_STATUSES` in `lib/docket-frontmatter.sh`) | **fails** | The row is bucketed under an unrecognized key and never emitted, while the file is still counted in the board's total — the count line and the tables disagree. The change also vanishes from the digest's `ready` queue. |
| `slug` | `^[a-z0-9-]+$` — `slugify`'s own alphabet | **fails** | Leaks raw into the digest's space-joined `change` line. |
| `priority` | one of `low`, `medium`, `high`, `critical` | **legal** (`medium`) | Sorts as `medium` in the `ready` queue while rendering raw in the Priority cell. |
| `title` | contains no `|` | legal | Injects extra columns into the `BOARD.md` table row. |

`id` is deliberately **not** covered here — `malformed-id` already detects a non-integer id, and a
second overlapping check would double-report the same file. Every domain is a shape or membership
test; none enumerates bad values.

**`board-row-dropped`** — Backstop for the count-vs-rows invariant, and the only check whose trigger
is **computed rather than enumerated**. An `active/` change file is *rendered* iff `int_field id`
yields a non-empty integer **and** its `status:` is one of the five statuses `render-board.sh`
actually calls `print_section` for (`DOCKET_STATUSES_ACTIVE`). Anything else in `active/` is counted
in the board's `total` and rendered nowhere — the count line and the tables disagree. The predicate
(`renders_row` in the script) reads the *same array the renderer's own section iteration uses*, so
it is a mirror of the renderer's bucketing, not a restatement of the causes the other checks name:
a drop path added to the renderer starts reporting here with **no edit to `board-checks.sh`**.

Emitted **only when no finding already accounts for the drop**. Exactly two arms suppress it, and
both describe a row *disappearing*:

| Suppressing finding | Why it explains the drop |
|---|---|
| `malformed-id` | A non-integer `id:` — `render-board.sh` skips the row outright. |
| `field-domain` on **`status`** | A status outside the seven-name vocabulary is outside the five-name active set too, so the row buckets under a key nothing iterates. |

A `field-domain` finding on `slug`, `priority` or `title` does **not** suppress: none of them drops
a row (a piped `title` injects columns into a row that is still emitted; `priority` renders raw;
`slug` is not read by the markdown renderer at all). Were they to suppress, an unrelated pipe in a
change's title would silence the backstop on a row that vanished for a different reason.

Two live triggers today:

- A change file with **no `id:` field at all** — `malformed-id` requires a non-empty (if
  non-integer) value, so nothing else reports it.
- An `active/` file carrying a **terminal status** (`done` / `killed`) — a *legal* status in the
  *wrong directory*. `field-domain` is correctly silent (`done` is in `DOCKET_STATUSES`) and the id
  is valid, so the computed invariant is the only thing that sees it. This state is reachable and
  documented: `docket-status`'s `sweep-failed <id> archive <reason>` is exactly "status flipped to
  `done`, archive move failed".

Beyond those, its remaining trigger is a future renderer-added drop path.

**Scope: the check covers `active/` only.** This is a deliberate bound, not a claim that `archive/`
is safe. The symmetric archive-side violation is real and currently undetected: an `archive/` file
carrying a **non-terminal** status is counted in `total` and rendered nowhere (the archive block is
gated on `ndone + nkilled > 0` and its summary count comes from `ARC_COUNT`, which such a file does
not join). It arises from the same interrupted operation as the active-side case — `archive-change.sh`
does its `git mv` before the status flip and the commit, so a failure between them leaves the file
moved but not re-statused. Extending the invariant to `archive/` is tracked as follow-up work.

**`malformed-id`** — Guard/carve-out, not counted among the named checks above. A change file
whose `id:` field is non-empty but non-integer emits a `malformed-id` finding. The change-id column
carries the **filename-derived** padded id (`?` when the filename yields none) — never the raw
frontmatter value, which is untrusted input and would shift the caller's TAB-separated fields; the
raw value appears in the message instead. The file is then skipped for all other checks.

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
- **The findings channel's COLUMNS are not forgeable.** `emit` escapes TAB and CR to visible
  `\t` / `\r` in both embedded columns, and the change-id column never carries a raw frontmatter
  value. The caller splits findings with `IFS=$'\t' read -r check_id change_id message`, so an
  un-escaped TAB in an untrusted value would shift every later field.
- **Message TEXT is untrusted; consumers must anchor on the check-id column.** The guarantee above
  is about column integrity, *not* content: `field-domain` messages quote raw frontmatter —
  including free-form `title` prose — verbatim by design, so any token a consumer keys on
  (`[reclaimable]`, say) can appear inside some *other* check's message. A consumer that
  substring-scans the whole findings blob is forgeable by anyone who can write a change file. Key on
  the check-id column: `docket-status.sh`'s `reclaim_pass` anchors its mutating gate at `^check
  stale-in-progress ` and requires the marker at end-of-line, so a marker inside a `field-domain`
  message can never satisfy it — that line begins with a different check-id.
