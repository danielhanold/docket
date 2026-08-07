# verify-run.sh — the terminal-disposition consumer

## Purpose

docket's terminal-disposition contract (`advanced` / `contended` / `drained` / `halted`) had a
producer and no consumer: `advanced` is claimable only when `docket-implement-next`'s **Step 7
postcondition** holds — a statement entirely readable from git that no code read. Six autonomous
runs executed a prefix of the seven steps and reported success. This script is the missing reader
(change 0237).

It is a **pure reader**: git and filesystem only. No network, no `gh`, no harness. It flips no
status, releases no claim, and writes no file. The only thing that acts on a verdict is
`runner-dispatch.sh`.

## Usage

```
docket.sh verify-run <id>
docket.sh verify-run --in-progress-ids [--with-claimed-at]
```

- `<id>` — the change id (integer; the file is located by its zero-padded name in `active/`, then
  `archive/`). Either form is accepted — `237` and `0237` name the same change — and the verdict
  line always echoes the canonical unpadded id, never the typed one.
- `--in-progress-ids` — print the id of every `status: in-progress` change in `active/`, one per
  line, numerically sorted. This is the snapshot half `runner-dispatch.sh` diffs across a hand-off.
- `--with-claimed-at` — only with `--in-progress-ids` (a hard error otherwise). Widens each line to
  `<id> <claimed_at-epoch>`, or `<id> -` when the change carries no `claimed_at:` or one that does
  not parse. The conversion happens **here**, through the shared `iso_to_epoch` (GNU and BSD
  `date`), so the consumer compares two integers and no second frontmatter reader or timestamp
  parser has to exist: `runner-dispatch.sh`'s run gate needs the claim instant to tell its own
  claim from a foreign one, and this script stays the single owner of that read. `-` is deliberate
  — an unreadable stamp is **no positive evidence** of anything, the same posture
  `reclaim-claims.sh` and `board-checks.sh` take on an unreadable lease.
- `--changes-dir DIR` — bypass config resolution and read this directory. For hermetic tests and
  for a caller that has already resolved a repo root.

Mock seams: `GIT`, `CONFIG_EXPORT_CMD`.

When `--changes-dir` is not given, the changes directory is resolved by sourcing
`docket-config.sh --export` (the same config resolver every other facade op uses), then anchoring
its repo-relative `CHANGES_DIR` value onto the metadata worktree via `lib/docket-root.sh`'s
`docket_metadata_worktree` — the same anchor used elsewhere in the facade, so both `docket` mode
and non-`docket` mode resolve correctly. A non-`PROCEED` bootstrap verdict is a hard failure
(`STOP_MIGRATE` or `CREATE_ORPHAN` both `die`); there is no silent fallback.

## Behavior

Verdict mode evaluates three conjuncts, each read with the **anchored** `fm_field` (ADR-0057):

| Conjunct | Read | Token when unmet |
|---|---|---|
| status advanced | `status: implemented` | `status` |
| PR recorded | `pr:` non-empty | `pr` |
| branch delivered | `refs/remotes/origin/<branch:>` resolves | `branch` |

One report line on stdout — the same house pattern as the Board pass, where **callers key on the
line, never on the exit code**:

- `run-complete <id>` — every conjunct holds.
- `run-halted <id>` — a `## Run halted` record is present; the run ended deliberately.
- `run-incomplete <id> <unmet…>` — tokens in the fixed order `status pr branch`.
- `run-unclaimed <id>` — the change is neither `in-progress` nor `implemented`; there is no run to
  verify (`proposed` after a reclaim, `deferred`, or archived).

**Precedence.** The conjuncts are evaluated **before** the halt record, so a satisfied
postcondition outranks a stale `## Run halted`. The section's removal is owned by
`docket-implement-next`'s Step 2 claim, which does not run on a resume, so a stale record is
reachable; this ordering means it can never downgrade a complete run.

**No time floor.** This is the point of the script, and it is sound only because of where it is
called: at a seam where the child process has already returned, so "stopped" and "still working"
are not ambiguous. `board-checks.sh` cannot make that assumption and keeps its floors — it is
deliberately untouched by change 0237.

## Exit codes

- `0` — **whenever a verdict was produced**, including `run-incomplete`. A finding is not a script
  failure, and a bare non-zero consumer must not read it as one.
- `2` — the check could not run: bad usage, non-numeric or unknown id, unreadable change file,
  failed config export, non-`PROCEED` bootstrap verdict, missing changes dir.

## Invariants

- Never writes: no status flip, no claim release, no file write, no `gh`, no network.
- Every *conjunct* read (`status`, `pr`, `branch`) is `fm_field`, never `field` — `pr:`, `branch:`
  and `claimed_at:` are optional keys and this repo's change bodies routinely open lines with
  them. The `id` read uses the shared `int_field` (unanchored, whole-file); `id:` is a mandatory
  key always on line 2 of the template, so the exposure is low-likelihood but real and untested —
  tracked by change 0237, not fixed here.
- A verdict is always exactly one line on stdout; diagnostics always go to stderr.
- `run-incomplete` never exits non-zero.
