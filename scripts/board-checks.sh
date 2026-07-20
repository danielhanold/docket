#!/usr/bin/env bash
# scripts/board-checks.sh — the mechanical docket-status health checks (change 0023). Sources the
# shared frontmatter/dependency-resolution helper (change 0022) and walks the change files, emitting
# one finding per line on stdout. Git-only (no gh, no network) and warn-only (never auto-fixes); the
# caller (docket-status) surfaces the lines. The one judgment-bearing check (blocked_by: re-examination)
# stays model-driven in the skill — it is NOT here.
#
# Usage: board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
#                         [--lease-ttl-hours N]
#   Findings: TAB-separated  <check-id>\t<change-id>\t<message>  on stdout, sorted by (check-id, change-id).
#     check-id ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain,
#                 stale-in-progress, merge-gate-stall, stale-finalize-blocked, merged-orphan,
#                 unknown-commit-ref, malformed-id}
#   Clean tree ⇒ no output, exit 0. --strict ⇒ exit 1 if any finding (for a future CI gate).
#   Branch args are passed to `git cat-file -e <ref>:<path>` verbatim; in main-mode the two refs
#   coincide and both link checks resolve on the same branch with no special-casing.
#   --lease-ttl-hours N defaults to 72 when absent (standalone use stays sane); a non-numeric or
#   negative N is rejected up front (exit 2), never crashed into the staleness arithmetic. It sets the
#   claim-lease TTL for stale-in-progress (change 0089): claimed_at + TTL expiry, on top of the
#   pre-existing branch-idle >3d signal. See that check's block below for the two trigger messages.
#   Mock seams: GIT="${GIT:-git}"  (the only external dependency); NOW="${NOW:-$(date +%s)}" (staleness clock).
set -uo pipefail

GIT="${GIT:-git}"
NOW="${NOW:-$(date +%s)}"
CHANGES_DIR=""; METADATA_BRANCH=""; INTEGRATION_BRANCH=""; STRICT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --metadata-branch) METADATA_BRANCH="$2"; shift ;;
    --integration-branch) INTEGRATION_BRANCH="$2"; shift ;;
    --strict) STRICT=1 ;;
    --lease-ttl-hours) LEASE_TTL_HOURS="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'board-checks: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
LEASE_TTL_HOURS="${LEASE_TTL_HOURS:-72}"   # default when --lease-ttl-hours is absent (standalone use)
# Validate the resolved TTL UNCONDITIONALLY (mirrors reclaim-claims.sh's own guard). A non-numeric or
# negative value must fail here, cleanly — not crash the staleness arithmetic (`$(( LEASE_TTL_HOURS *
# 3600 ))`) unbound, which would otherwise only surface on repos that carry an in-progress change.
case "$LEASE_TTL_HOURS" in
  ''|*[!0-9]*) printf 'board-checks: invalid --lease-ttl-hours: %s (must be a non-negative integer, hours)\n' "$LEASE_TTL_HOURS" >&2; exit 2 ;;
esac
[ -n "$CHANGES_DIR" ]        || { printf 'board-checks: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ]        || { printf 'board-checks: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }
[ -n "$METADATA_BRANCH" ]    || { printf 'board-checks: missing --metadata-branch\n' >&2; exit 2; }
[ -n "$INTEGRATION_BRANCH" ] || { printf 'board-checks: missing --integration-branch\n' >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

resolve_deps "$CHANGES_DIR"            # populates STATUS_OF / DEP_STATE / DEP_REASON / DEP_ON

# git_has REF PATH — exit 0 iff REF:PATH resolves in the changes-dir's repo (no network).
git_has(){ "$GIT" -C "$CHANGES_DIR" cat-file -e "$1:$2" 2>/dev/null; }

declare -A ID_ACTIVE ID_EXISTS                # id -> 1; populated in the FILES walk below
declare -A EXPLAINED DROPPED                  # change-id -> 1; drive board-row-dropped (change 0104)
FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end

# sanitize VALUE — render TAB and CR as the visible two-character escapes \t and \r (change 0104).
# Findings are TAB-separated and the caller splits them with `IFS=$'\t' read -r check_id change_id
# message` (docket-status.sh:627), so an interior TAB in ANY embedded value shifts every later
# field. field() truncates at the first newline and strips trailing whitespace, but an interior TAB
# survives it — these values are untrusted frontmatter, not program constants. Pure bash parameter
# expansion: BSD sed does not interpret \t in a pattern, so a sed form would be silently wrong.
sanitize(){ local v="$1"; v="${v//$'\t'/\\t}"; v="${v//$'\r'/\\r}"; printf '%s' "$v"; }

emit(){ FINDINGS+="$1"$'\t'"$(sanitize "$2")"$'\t'"$(sanitize "$3")"$'\n'; }

# padded_id_from_file FILE — the zero-padded id encoded in the BASENAME (`0104-slug.md`, or
# `2026-07-20-0104-slug.md` in archive/), or `?` when the filename yields none. Used for the
# change-id column whenever the frontmatter id is unusable: that column is what the caller splits
# on, so it must never carry a raw frontmatter value. A validated int_field id is ^[0-9]+$ and
# cannot shift a field, so checks that have one keep emitting it verbatim (unpadded, as before).
padded_id_from_file(){
  local b; b="$(basename "$1")"
  b="${b#[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-}"   # strip an archive/ date prefix, if any
  case "$b" in
    [0-9][0-9][0-9][0-9]-*) printf '%s' "${b%%-*}" ;;
    *) printf '?' ;;
  esac
}

# Staleness horizon for the stale-finalize-blocked check (change 0098): an 'implemented' change's
# `## Finalize blocked` marker older than this fires the advisory. Hardcoded, no config knob —
# mirrors stale-in-progress's own hardcoded 3*86400 branch-idle horizon; 72h matches the lease-TTL
# default's sense of "a few days is normal, longer is suspicious". Promote to a flag only if
# independent tuning is ever wanted.
FINALIZE_BLOCKED_STALE_SECS=$(( 72 * 3600 ))

# Walk every change file (active + archive); per-check filters apply inside.
mapfile -t FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  pid="$(padded_id_from_file "$f")"
  # cid — the change-id column for every finding about this file: the validated integer id when
  # there is one, else the filename-derived padded id. NEVER the raw frontmatter value.
  cid="${id:-$pid}"
  fd_active=0; case "$f" in */active/*) fd_active=1 ;; esac
  if [ -z "$id" ]; then
    if [ -n "$raw" ]; then
      emit malformed-id "$cid" "non-integer id '$raw' in $(basename "$f")"
      EXPLAINED["$cid"]=1
    fi
    # No usable id ⇒ render-board.sh:76 skips the row while :86 still counts the file.
    [ "$fd_active" = 1 ] && DROPPED["$cid"]=1
    continue
  fi
  ID_EXISTS["$id"]=1
  case "$f" in
    */active/*)  ID_ACTIVE["$id"]=1 ;;
  esac
  status="$(field "$f" status)"
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"

  # --- field-domain: a value that is well-formed TEXT but outside its field's DOMAIN (change 0104).
  # These four fields are what the board renderers consume. A value outside the domain does not
  # error — it silently drops the row from every surface (status, slug) or injects columns into it
  # (title), and since change 0094 the digest's `ready` line is the machine-parsed selection channel
  # for docket-implement-next, so a stray inline comment can remove a change from the autonomous
  # build queue while the board still reports a healthy count. One finding per violated field.
  # Every domain is a SHAPE or MEMBERSHIP test — never an enumeration of bad values.
  # `id` is deliberately absent: malformed-id already covers it (no double-reporting).
  fd_slug="$(field "$f" slug)"; fd_priority="$(field "$f" priority)"; fd_title="$(field "$f" title)"

  status_ok=0
  for fd_s in "${DOCKET_STATUSES[@]}"; do
    if [ "$status" = "$fd_s" ]; then status_ok=1; break; fi
  done
  if [ "$status_ok" != 1 ]; then
    emit field-domain "$cid" "status '$status' is not one of: ${DOCKET_STATUSES[*]}"
    EXPLAINED["$cid"]=1
    # An unrecognized status buckets into a SECTION key no renderer iterates.
    [ "$fd_active" = 1 ] && DROPPED["$cid"]=1
  fi

  # slugify's own alphabet (mint-stub.sh:88-91). Empty fails — slug has no documented default.
  case "$fd_slug" in
    ''|*[!a-z0-9-]*) emit field-domain "$cid" "slug '$fd_slug' is not ^[a-z0-9-]+\$"; EXPLAINED["$cid"]=1 ;;
  esac

  # Empty priority is LEGAL: the convention documents `medium` as the default and render-board.sh's
  # sort already implements it. Flagging it here would make the guard the noise source.
  case "$fd_priority" in
    ''|low|medium|high|critical) ;;
    *) emit field-domain "$cid" "priority '$fd_priority' is not one of: low medium high critical (empty = medium)"; EXPLAINED["$cid"]=1 ;;
  esac

  case "$fd_title" in
    *'|'*) emit field-domain "$cid" "title contains '|', which injects columns into the board row: $fd_title"; EXPLAINED["$cid"]=1 ;;
  esac

  # --- broken-spec: spec set, not trivial, path absent on the metadata branch ---
  if [ -n "$spec" ] && [ "$trivial" != "true" ]; then
    git_has "$METADATA_BRANCH" "$spec" || emit broken-spec "$id" "spec not found on $METADATA_BRANCH: $spec"
  fi

  # --- broken-plan-results: a done change's set plan:/results: must resolve on the integration branch ---
  # Carve-out: never flag an 'implemented' change — those files still live on the unmerged feature branch.
  if [ "$status" = "done" ]; then
    for key in plan results; do
      val="$(field "$f" "$key")"
      [ -n "$val" ] || continue
      git_has "$INTEGRATION_BRANCH" "$val" || emit broken-plan-results "$id" "$key not found on $INTEGRATION_BRANCH: $val"
    done
  fi

  # --- stale-in-progress: lease expired (claimed_at+TTL) OR branch idle >3 days ---
  # Complements the branch-age signal with a claimed_at signal that catches the crashed-BEFORE-branch
  # blind spot (branch ref absent). The reclaimable subset (expired AND no branch ref) carries the
  # trailing [reclaimable] marker — the machine contract docket-status keys on for its remedy print.
  if [ "$status" = "in-progress" ]; then
    branch="$(field "$f" branch)"
    claimed="$(field "$f" claimed_at)"
    has_branch=0
    if [ -n "$branch" ]; then
      if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/heads/$branch" \
         || "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
        has_branch=1
      fi
    fi
    lease_secs="$(( LEASE_TTL_HOURS * 3600 ))"
    expired=0; age_h=""
    if [ -n "$claimed" ]; then
      cepoch="$(iso_to_epoch "$claimed")" || cepoch=""
      if [ -n "$cepoch" ] && [ "$(( NOW - cepoch ))" -gt "$lease_secs" ]; then
        expired=1; age_h="$(( (NOW - cepoch) / 3600 ))"
      fi
    fi
    if [ "$has_branch" = 1 ]; then
      ts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$branch" 2>/dev/null)"
      if [ -n "$ts" ] && [ "$(( NOW - ts ))" -gt "$(( 3*86400 ))" ]; then
        emit stale-in-progress "$id" "branch $branch idle >3 days (last commit $(( (NOW - ts) / 86400 ))d ago)"
      elif [ "$expired" = 1 ]; then
        emit stale-in-progress "$id" "claim lease expired ${age_h}h ago; branch $branch exists — needs your review (not auto-reclaimable)"
      fi
    elif [ "$expired" = 1 ]; then
      emit stale-in-progress "$id" "claim lease expired ${age_h}h ago; no feature branch — self-heal with docket.sh reclaim-claims [reclaimable]"
    fi
  fi

  # --- merge-gate-stall: build-ready, but its worst-unmet dep is stuck at 'implemented' ---
  if [ "$status" = "proposed" ] && { [ -n "$spec" ] || [ "$trivial" = "true" ]; }; then
    if [ "${DEP_REASON[$id]:-}" = "needs your merge" ]; then
      emit merge-gate-stall "$id" "build-ready but waiting on #${DEP_ON[$id]} — needs your merge"
    fi
  fi

  # --- stale-finalize-blocked: an 'implemented' change carrying the `## Finalize blocked` marker
  # whose marker has outlived FINALIZE_BLOCKED_STALE_SECS (change 0098). The marker's only clearing
  # path is a docket-finalize-change run; when a human resolves the underlying cause out of band
  # (without re-running finalize with the id named) the marker sits on the board indefinitely. This
  # is a git-only, time-based advisory: it cannot know whether the cause still holds (that needs a
  # network probe this script forbids), so it fires on ANY marker past the horizon — a marker still
  # genuinely blocked that long is itself worth a human glance. Marker age = the change file's
  # last-commit timestamp (git ct is tamper-proof; the in-body date is model-authored prose). Never
  # mutates the file / auto-clears the marker — that stays docket-finalize-change's job.
  if [ "$status" = "implemented" ] && finalize_blocked "$f"; then
    # "$f" is a pathspec here (unlike the ref args elsewhere in this script). It comes from
    # `find "$CHANGES_DIR/..."`, so it is absolute whenever --changes-dir is absolute — which the
    # real docket-status invocation and the tests always are, and git resolves an absolute pathspec
    # against the worktree root regardless of the -C cwd. A RELATIVE --changes-dir would make this
    # resolve against the changed cwd and return empty (→ silently never fires); pass an absolute dir.
    fbts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct -- "$f" 2>/dev/null)"
    if [ -n "$fbts" ] && [ "$(( NOW - fbts ))" -gt "$FINALIZE_BLOCKED_STALE_SECS" ]; then
      emit stale-finalize-blocked "$id" "## Finalize blocked marker set $(( (NOW - fbts) / 3600 ))h ago — resolve the cause and re-run finalize $id, or it will sit on the board"
    fi
  fi
done

# --- board-row-dropped: an ACTIVE file counted in the board's total but rendered in no section ---
# render-board.sh counts every active *.md in `total` (:86) but buckets rows on the raw `status:`
# read (:78) and skips files whose id is not an integer (:76) — so such a file is counted and never
# emitted, and the board's count line disagrees with its tables. SUPPRESSED when a field-domain or
# malformed-id finding already explains that id: a backstop that fires alongside every domain
# finding trains the reader to ignore it. Suppressed, it says exactly one thing — a row vanished and
# nothing enumerated explains why. Its live trigger today is a file with NO `id:` field at all
# (malformed-id needs a non-empty raw value); with the domain checks in place, its remaining trigger
# is a future renderer-added drop path, which is what a backstop is for.
for drop_id in "${!DROPPED[@]}"; do
  [ -n "${EXPLAINED[$drop_id]:-}" ] && continue
  emit board-row-dropped "$drop_id" "counted in the board total but rendered in no section, and no field-domain or malformed-id finding explains it"
done

# --- dep-cycle: DFS over depends_on; mark every node that lies on a cycle ---
declare -A ADJ COLOR INSTACK ONCYCLE
for f in "${FILES[@]}"; do
  cid="$(int_field "$f" id)"; [ -n "$cid" ] || continue
  ADJ["$cid"]="$(list_field "$f" depends_on)"
done
PATH_STACK=()
dfs(){ # dfs NODE — colors: white(unset) / gray(on stack) / black(done)
  local node="$1" dep i seen
  COLOR["$node"]=gray; INSTACK["$node"]=1; PATH_STACK+=("$node")
  for dep in ${ADJ["$node"]:-}; do
    [ -n "${ADJ[$dep]+x}" ] || continue            # dep is not a known change ⇒ not a graph edge
    if [ "${INSTACK[$dep]:-0}" = 1 ]; then
      seen=0                                        # back edge: mark dep..top-of-stack
      for i in "${PATH_STACK[@]}"; do
        [ "$i" = "$dep" ] && seen=1
        [ "$seen" = 1 ] && ONCYCLE["$i"]=1
      done
    elif [ "${COLOR[$dep]:-white}" = white ]; then
      dfs "$dep"
    fi
  done
  COLOR["$node"]=black; INSTACK["$node"]=0
  PATH_STACK=("${PATH_STACK[@]:0:${#PATH_STACK[@]}-1}")   # pop (bash-4.0-safe; no unset arr[-1])
}
for node in "${!ADJ[@]}"; do
  [ "${COLOR[$node]:-white}" = white ] && dfs "$node"
done
for node in "${!ONCYCLE[@]}"; do
  emit dep-cycle "$node" "participates in a depends_on cycle"
done

# --- merged-orphan / unknown-commit-ref: cross-reference integration-branch commit subjects
#     against the active/archive change set. Git-only, subjects only, conservative grammar
#     (numeric conventional-commit scope + a "(change N)" tag (conventionally trailing;
#     matched anywhere in the subject)); bare #N and bodies excluded to bound PR-number
#     false positives. Zero-padding tolerated (10# strips it). Full history.
declare -A REF_EVIDENCE                       # id -> "<short-sha> <subject>" (first commit seen)
re_scope='^[a-zA-Z]+\(0*([0-9]{1,4})\):'      # docket(0085): … / results(0085): …
re_trailing='\(change 0*([0-9]{1,4})\)'       # … (change 0085)
while IFS=$'\t' read -r ev_sha ev_subject; do
  [ -n "$ev_subject" ] || continue
  refs=""
  [[ "$ev_subject" =~ $re_scope ]]    && refs+=" $(( 10#${BASH_REMATCH[1]} ))"
  [[ "$ev_subject" =~ $re_trailing ]] && refs+=" $(( 10#${BASH_REMATCH[1]} ))"
  for rid in $refs; do
    [ -n "${REF_EVIDENCE[$rid]:-}" ] || REF_EVIDENCE["$rid"]="$ev_sha $ev_subject"
  done
done < <("$GIT" -C "$CHANGES_DIR" log --format='%h%x09%s' "$INTEGRATION_BRANCH" 2>/dev/null)

for rid in "${!REF_EVIDENCE[@]}"; do
  ev="${REF_EVIDENCE[$rid]}"
  if [ -n "${ID_ACTIVE[$rid]:-}" ]; then
    emit merged-orphan "$rid" "merged on $INTEGRATION_BRANCH ($ev) but still active (not archived)"
  elif [ -z "${ID_EXISTS[$rid]:-}" ]; then
    emit unknown-commit-ref "$rid" "referenced by $INTEGRATION_BRANCH commit ($ev) but no change file exists"
  fi
  # archived (terminal) ⇒ properly closed out ⇒ no finding
done

# Emit findings sorted by (check-id asc, change-id numeric asc) for determinism.
if [ -n "$FINDINGS" ]; then
  printf '%s' "$FINDINGS" | sort -t"$(printf '\t')" -k1,1 -k2,2n
fi

if [ "$STRICT" = 1 ] && [ -n "$FINDINGS" ]; then exit 1; fi
exit 0
