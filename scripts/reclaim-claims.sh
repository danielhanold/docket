#!/usr/bin/env bash
# scripts/reclaim-claims.sh — deterministic claim-lease reclaim (change 0089). Sweeps active/*.md for
# in-progress changes whose claim lease (claimed_at:) is EXPIRED and that have NO feature branch (the
# crashed-before-push blind spot — the one case reclaim is provably collision-free and orphan-free),
# and flips them back to build-ready `proposed` so the queue self-heals. Git-only (no gh, no network);
# reads the metadata working tree it is pointed at. Mutation is the caller's choice (docket-status runs
# it only under reclaim.auto; a human runs `docket.sh reclaim-claims` explicitly). ADR-0012: a
# deterministic script, never model prose. ADR-0021: authors its own mechanical commit.
#
# Usage: reclaim-claims.sh --changes-dir DIR --lease-ttl-hours N [--remote R]
#   Reclaimable iff (1) status: in-progress, (2) claimed_at present AND NOW-claimed_at > N*3600,
#   AND (3) no feat/<slug> ref resolves (refs/heads OR refs/remotes/<remote>). A change with no
#   claimed_at is NEVER reclaimed (no positive evidence of expiry). A change whose branch ref
#   resolves is NEVER reclaimed (orphan/collision guard). Report: one line per change on stdout —
#   "reclaimed <id> <slug> (lease <age>h, no branch)" | "skipped <id> raced". Exit 0 on a clean sweep.
#   Mock seams: GIT="${GIT:-git}"; NOW="${NOW:-$(date +%s)}".
set -uo pipefail
GIT="${GIT:-git}"
NOW="${NOW:-$(date +%s)}"
CHANGES_DIR=""; TTL_HOURS=""; REMOTE="origin"
die(){ printf '%s\n' "reclaim-claims: $*" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --lease-ttl-hours) TTL_HOURS="$2"; shift ;;
    --remote) REMOTE="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"
case "$TTL_HOURS" in ''|*[!0-9]*) die "missing/invalid --lease-ttl-hours" ;; esac

# shellcheck source=/dev/null
. "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"   # field / int_field / iso_to_epoch

WT="$($GIT -C "$CHANGES_DIR" rev-parse --show-toplevel)" || die "not a git worktree: $CHANGES_DIR"
# changes-dir path relative to the worktree root (git add/commit want worktree-relative paths).
# pwd -P resolves symlinks: macOS mktemp gives /var/... but rev-parse gives /private/var/....
REL_ABS="$(cd "$CHANGES_DIR" && pwd -P)"; REL="${REL_ABS#"$WT"/}"
TTL_SECS=$(( TTL_HOURS * 3600 ))
TODAY="$(date -u +%Y-%m-%d)"
cur_branch="$($GIT -C "$WT" rev-parse --abbrev-ref HEAD)"

set_field(){ # set_field FILE KEY VALUE — replace a scalar in the first ---…--- block only (portable sed).
  local f="$1" k="$2" v="$3" t; t="$(mktemp)"     # clearing a field ⇒ VALUE="" (leaves "key: ").
  sed -E "/^---$/,/^---$/ s|^($k:)[[:space:]]*.*|\1 $v|" "$f" > "$t" && mv "$t" "$f"
}
any_branch_ref(){ # 0 iff ANY given branch name resolves to a local OR remote-tracking ref.
  local b                                          # checks both the recorded branch: field and the
  for b in "$@"; do                                # convention name feat/<slug> — the orphan guard is
    [ -n "$b" ] || continue                        # the highest-value safety property, so cast wide.
    $GIT -C "$WT" show-ref --verify --quiet "refs/heads/$b"           && return 0
    $GIT -C "$WT" show-ref --verify --quiet "refs/remotes/$REMOTE/$b" && return 0
  done
  return 1
}
cas_push(){ # push cur_branch to REMOTE, rebasing on a non-fast-forward until it converges.
  until $GIT -C "$WT" push "$REMOTE" "$cur_branch"; do
    $GIT -C "$WT" pull --rebase "$REMOTE" "$cur_branch" || die "rebase during push failed for $cur_branch"
  done
}

# eligible FILE — prints "<age_hours>" and returns 0 iff FILE is reclaimable; returns 1 otherwise.
# Pure (no writes); safe to re-run against the post-rebase reality after a CAS non-fast-forward.
eligible(){
  local f="$1" status claimed epoch branch base slug
  [ -f "$f" ] || return 1
  status="$(field "$f" status)"; [ "$status" = "in-progress" ] || return 1
  claimed="$(field "$f" claimed_at)"; [ -n "$claimed" ] || return 1     # (2a) no positive evidence of expiry
  epoch="$(iso_to_epoch "$claimed")" || return 1                        # unparseable ⇒ treat as no evidence
  [ "$(( NOW - epoch ))" -gt "$TTL_SECS" ] || return 1                  # (2b) lease not yet expired
  branch="$(field "$f" branch)"
  base="$(basename "$f")"; slug="${base%.md}"; slug="${slug#*-}"
  any_branch_ref "$branch" "feat/$slug" && return 1                     # (3) branch exists ⇒ never reclaim here
  printf '%s' "$(( (NOW - epoch) / 3600 ))"; return 0
}

# reclaim_file FILE ID SLUG AGE — append the dated Reclaim log, flip the frontmatter to build-ready,
# stage + commit CHANGE-FILE-ONLY. Does NOT push (the caller drives CAS). No link-bearing field
# changes, so the ## Artifacts block is deliberately left untouched (docket field-write rule).
reclaim_file(){
  local f="$1" id="$2" slug="$3" age="$4" base pad; base="$(basename "$f")"; pad="$(printf '%04d' "$id")"
  {
    printf '\n## Reclaim log\n\n'
    printf '### %s — reclaimed by reclaim-claims.sh\n\n' "$TODAY"
    printf 'Claim lease expired (~%sh since claimed_at, TTL %sh) and no feature branch ref was found; ' "$age" "$TTL_HOURS"
    printf 'flipped in-progress → proposed so the change re-enters selection.\n'
  } >> "$f"
  set_field "$f" status proposed
  set_field "$f" branch ""
  set_field "$f" claimed_at ""
  set_field "$f" reconciled false
  set_field "$f" updated "$TODAY"
  $GIT -C "$WT" add "$REL/active/$base"                                    || die "git add failed for $id"
  $GIT -C "$WT" commit -q -m "docket($pad): reclaim — expired lease, no branch; back to proposed" \
       -- "$REL/active/$base"                                             || die "commit failed for $id"
}

shopt -s nullglob
for f in "$WT/$REL/active/"*.md; do
  age="$(eligible "$f")" || continue                # a skip (or any per-item non-zero) must not abort the sweep
  id="$(int_field "$f" id)"; [ -n "$id" ] || continue
  base="$(basename "$f")"; slug="${base%.md}"; slug="${slug#*-}"

  reclaim_file "$f" "$id" "$slug" "$age"
  if $GIT -C "$WT" push "$REMOTE" "$cur_branch" 2>/dev/null; then
    printf 'reclaimed %s %s (lease %sh, no branch)\n' "$id" "$slug" "$age"
    continue
  fi
  # Non-fast-forward: a concurrent writer advanced origin. Drop our now-stale commit and resync the
  # working tree to origin, then RE-READ eligibility against that concurrent reality (NOT the working
  # tree we just wrote — that would always read back our own flip). If the change no longer qualifies
  # (claim refreshed, branch pushed, archived, or already reclaimed) skip it; otherwise redo + push.
  $GIT -C "$WT" fetch "$REMOTE" >/dev/null 2>&1 || die "fetch during CAS failed for $id"
  if ! $GIT -C "$WT" reset --hard "$REMOTE/$cur_branch" >/dev/null 2>&1; then
    printf 'skipped %s raced\n' "$id"; continue
  fi
  age="$(eligible "$f")" || { printf 'skipped %s raced\n' "$id"; continue; }
  reclaim_file "$f" "$id" "$slug" "$age"
  cas_push
  printf 'reclaimed %s %s (lease %sh, no branch)\n' "$id" "$slug" "$age"
done
shopt -u nullglob
exit 0
