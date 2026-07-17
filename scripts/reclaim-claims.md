# reclaim-claims.sh — deterministic claim-lease reclaim sweep

## Purpose

Reclaims a crashed, expired `in-progress` claim back to build-ready `proposed` so the
selection queue self-heals — but only in the one provably-safe case: an **expired claim
lease AND no feature branch**. That is the crashed-before-push blind spot, the single
situation in which reclaim is guaranteed collision-free (no live implementer is holding the
change) and orphan-free (no feature branch is left dangling). Every other in-progress change
is left untouched. Git-only (no `gh`, no network); it reads and rewrites the metadata working
tree it is pointed at, authoring its own mechanical change-file-only commit. Introduced in
change 0089. ADR-0012: a deterministic script, never model prose. ADR-0021: authors its own
mechanical commit.

Mutation is the caller's choice: `docket-status` runs it only under `reclaim.auto`, and a
human runs `docket.sh reclaim-claims` explicitly. The lease TTL is supplied by the caller
(resolved from `reclaim.lease_ttl_hours`; see `ensure-docket-env.sh`).

## Usage

```
reclaim-claims.sh --changes-dir DIR --lease-ttl-hours N [--remote R]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the directory that contains `active/` and `archive/`. Its git worktree root is the push target. |
| `--lease-ttl-hours N` | yes | Non-negative integer. A lease is expired once `NOW - claimed_at > N*3600` seconds. |
| `--remote R` | no | Push remote (default `origin`); also the remote-tracking namespace searched for a branch ref (`refs/remotes/<R>/…`). |

**Output format:** one line per acted-on change on stdout —
`reclaimed <id> <slug> (lease <age>h, no branch)` for a reclaim, or `skipped <id> raced`
when a concurrent writer advanced the change during the CAS push. Untouched changes emit
nothing. A sweep with zero reclaims produces no output.

**Mock seams:** `GIT="${GIT:-git}"` and `NOW="${NOW:-$(date +%s)}"` — override in tests for a
hermetic clock and git injection.

## Behavior

The script sources `lib/docket-frontmatter.sh` (`field`, `int_field`, `iso_to_epoch`) and
walks `active/*.md` in sorted glob order. A change is **reclaimable** iff ALL of:

1. `status: in-progress`.
2. `claimed_at:` is present **and** `NOW - iso_to_epoch(claimed_at) > lease_ttl_hours*3600`.
   A change with **no** `claimed_at`, or an unparseable one, is **never** reclaimed — there
   is no positive evidence the lease expired (a crash the moment before `claimed_at` was
   written must not be mistaken for an expiry).
3. **No** `feat/<slug>` ref resolves — neither the recorded `branch:` field value nor the
   convention name `feat/<slug>` matches any `refs/heads/<b>` or `refs/remotes/<remote>/<b>`.
   A change whose branch ref resolves is **never** reclaimed: it is either live or an orphan,
   and flipping it back to `proposed` would risk a collision or leave a dangling branch. This
   is the highest-value safety property; the probe casts wide (recorded name **and**
   convention name) so a missing `branch:` field cannot hide an orphan branch.

On a reclaim (no-branch case only), for the single change file:

- Append a dated `## Reclaim log` body section recording the lease age, the TTL, and the
  no-branch finding.
- Set `status: proposed`, clear `branch:` and `claimed_at:`, set `reconciled: false`, and set
  `updated:` to the UTC date (`date -u +%Y-%m-%d`). Frontmatter edits anchor to the first
  `---…---` block only.
- The `## Artifacts` block is **not** regenerated — no link-bearing field changed, per
  docket's field-write rule.
- Commit **change-file-only** (`git commit -- <path>`) and CAS-push.

### CAS discipline (concurrent-writer safety)

The commit touches exactly the one change file. The script pushes; on a non-fast-forward
(a concurrent writer advanced the metadata branch), it **fetches, drops its own stale commit
by resetting the working tree to the remote tip, then re-reads eligibility against that
concurrent reality** — deliberately not against the working tree it just wrote, which would
always read back its own flip. If the change no longer qualifies (claim refreshed, a branch
appeared, it was archived, or it was already reclaimed) it emits `skipped <id> raced` and
moves on; otherwise it redoes the reclaim on the fresh base and pushes to convergence.

### Sweep hygiene

The script runs under `set -uo pipefail` (not `set -e`): every per-item skip uses
`|| continue`, so one ineligible or malformed change never aborts the sweep — later changes
still process.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Clean sweep (including zero reclaims, and including per-change `skipped … raced`). |
| 1 | Missing/invalid argument, a non-worktree `--changes-dir`, or an unrecoverable git failure (fetch/commit/rebase). Diagnostic on stderr. |

## Invariants

- **Provably-safe narrowing only.** Reclaim acts solely on `in-progress` + expired-lease +
  no-branch. No `claimed_at` ⇒ never; branch ref present ⇒ never. Fresh leases and `proposed`
  / terminal changes are ignored.
- **Git-only, offline.** No `gh`, no network beyond the metadata remote push/fetch. All ref
  probes use `git show-ref` against the local object store.
- **Change-file-only commits.** Each reclaim stages and commits exactly one `active/*.md`
  file; nothing else is ever included in the commit.
- **STDOUT for the report, STDERR for errors.** Callers capture stdout to surface the
  per-change report lines; usage errors and hard failures go to stderr.
- **Idempotent.** A second sweep over an already-reclaimed change finds nothing eligible
  (it is now `proposed`) and is a no-op.
- **Caller owns the decision to mutate.** The script always mutates when it finds an eligible
  change; whether it runs at all is the caller's policy (`reclaim.auto` for `docket-status`,
  explicit invocation for a human).
