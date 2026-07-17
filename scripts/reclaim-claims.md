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
human runs `docket.sh reclaim-claims` explicitly. The lease TTL comes from the config key
`reclaim.lease_ttl` (integer hours — the `_hours` suffix belongs only to this script's
`--lease-ttl-hours` flag), resolved by `scripts/docket-config.sh` (exported as
`RECLAIM_LEASE_TTL`) and forwarded here as `--lease-ttl-hours` by `docket-status.sh`, or
passed directly by a human.

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

## Preconditions

**The caller must ensure remote-tracking refs are current before invoking this script.** The
no-branch orphan guard's REMOTE arm (`refs/remotes/<remote>/<b>`, inside `any_branch_ref`)
reads whatever is already in the local object store — it does **not** `git fetch`. If a
`feat/<slug>` branch was pushed to origin from another clone but this clone has not yet
fetched, that remote-tracking ref is stale/absent here, and the guard sees "no branch" even
though real work exists on origin: a **false negative** on the highest-value safety property,
against a change that carries real remote work. Run `git fetch` (or a docket preflight sync)
against `--changes-dir`'s worktree immediately before invoking `reclaim-claims.sh` — do not
rely on refs left over from an earlier point in the session.

This is the spec's documented **§7-H cross-machine residual**
(`docs/superpowers/specs/2026-07-17-claim-leases-reclaim-script-design.md`): a build on one
machine has a feature-branch ref another machine's local object store cannot see until it
fetches. It is *contained*, not eliminated, by three things together — the lease must also be
expired (a fresh claim is never touched regardless of ref visibility), `reclaim.auto` defaults
off (so this only bites an explicit opt-in), and callers are expected to sync remote-tracking
refs first. It is not a bug in the guard's logic; it is a property of reading local refs
without a network round-trip, and is the documented, accepted cost of staying git-only/offline.

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

The commit touches exactly the one change file. The push runs inside a single bounded retry
loop (up to 5 attempts). On **every** non-fast-forward — not just the first — the script
**fetches, drops its own stale commit by resetting the working tree to the remote tip, then
re-reads eligibility against that concurrent reality** — deliberately not against the working
tree it just wrote, which would always read back its own flip. If the change no longer
qualifies (claim refreshed, a branch appeared, it was archived, or it was already reclaimed) it
emits `skipped <id> raced` and moves on to the next change; otherwise it redoes the reclaim on
the fresh base and retries the push. This re-check applies uniformly to every retry in the
loop, not only the first, so a second (or later) concurrent writer can never ride an unchecked
retry onto origin. A genuine `fetch`/`reset --hard` failure (network, unreachable remote, a
missing ref) is a hard git error, not a race — it `die`s with a diagnostic rather than being
mislabeled `skipped … raced`. If the loop exhausts its 5 attempts without a clean push (the
remote keeps advancing faster than this sweep converges), it also `die`s.

Immediate push-per-item is what keeps `reset --hard` safe: the local branch never carries more
than the current item's one unpushed commit, so `reset --hard <remote>/<branch>` only ever
discards that single commit — never another item's already-pushed work.

### Sweep hygiene

The script runs under `set -uo pipefail` (not `set -e`): every per-item skip uses
`|| continue`, so one ineligible or malformed change never aborts the sweep — later changes
still process.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Clean sweep (including zero reclaims, and including per-change `skipped … raced`). |
| 1 | Missing/invalid argument, a non-worktree `--changes-dir`, or an unrecoverable git failure (`add`/`commit`/`fetch`/`reset --hard`, or the CAS retry loop exhausting its 5 attempts without a clean push). Diagnostic on stderr. |

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
