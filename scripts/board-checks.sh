#!/usr/bin/env bash
# scripts/board-checks.sh — the mechanical docket-status health checks (change 0023). Sources the
# shared frontmatter/dependency-resolution helper (change 0022) and walks the change files, emitting
# one finding per line on stdout. Git-only (no gh, no network) and warn-only (never auto-fixes); the
# caller (docket-status) surfaces the lines. The one judgment-bearing check (blocked_by: re-examination)
# stays model-driven in the skill — it is NOT here.
#
# Usage: board-checks.sh --changes-dir DIR --metadata-branch BR --integration-branch BR [--strict]
#   Findings: TAB-separated  <check-id>\t<change-id>\t<message>  on stdout, sorted by (check-id, change-id).
#     check-id ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall}
#   Clean tree ⇒ no output, exit 0. --strict ⇒ exit 1 if any finding (for a future CI gate).
#   Branch args are passed to `git cat-file -e <ref>:<path>` verbatim; in main-mode the two refs
#   coincide and both link checks resolve on the same branch with no special-casing.
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
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'board-checks: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ]        || { printf 'board-checks: missing --changes-dir\n' >&2; exit 2; }
[ -d "$CHANGES_DIR" ]        || { printf 'board-checks: changes dir not found: %s\n' "$CHANGES_DIR" >&2; exit 2; }
[ -n "$METADATA_BRANCH" ]    || { printf 'board-checks: missing --metadata-branch\n' >&2; exit 2; }
[ -n "$INTEGRATION_BRANCH" ] || { printf 'board-checks: missing --integration-branch\n' >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

resolve_deps "$CHANGES_DIR"            # populates STATUS_OF / DEP_STATE / DEP_REASON / DEP_ON

# git_has REF PATH — exit 0 iff REF:PATH resolves in the changes-dir's repo (no network).
git_has(){ "$GIT" -C "$CHANGES_DIR" cat-file -e "$1:$2" 2>/dev/null; }

FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end
emit(){ FINDINGS+="$1"$'\t'"$2"$'\t'"$3"$'\n'; }

# Walk every change file (active + archive); per-check filters apply inside.
mapfile -t FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  status="$(field "$f" status)"
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"

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

  # --- stale-in-progress: in-progress, branch exists, newest commit older than 3 days ---
  # Carve-out: branch: set but the branch does not exist ⇒ not stale (just-claimed / indistinguishable).
  if [ "$status" = "in-progress" ]; then
    branch="$(field "$f" branch)"
    if [ -n "$branch" ] && "$GIT" -C "$CHANGES_DIR" rev-parse --verify --quiet "$branch" >/dev/null 2>&1; then
      ts="$("$GIT" -C "$CHANGES_DIR" log -1 --format=%ct "$branch" 2>/dev/null)"
      if [ -n "$ts" ] && [ "$(( NOW - ts ))" -gt "$(( 3*86400 ))" ]; then
        emit stale-in-progress "$id" "branch $branch idle >3 days (last commit $(( (NOW - ts) / 86400 ))d ago)"
      fi
    fi
  fi

  # --- merge-gate-stall: build-ready, but its worst-unmet dep is stuck at 'implemented' ---
  if [ "$status" = "proposed" ] && { [ -n "$spec" ] || [ "$trivial" = "true" ]; }; then
    if [ "${DEP_REASON[$id]:-}" = "needs your merge" ]; then
      emit merge-gate-stall "$id" "build-ready but waiting on #${DEP_ON[$id]} — needs your merge"
    fi
  fi
done

# --- dep-cycle: DFS over depends_on; mark every node that lies on a cycle ---
declare -A ADJ COLOR INSTACK ONCYCLE
for f in "${FILES[@]}"; do
  cid="$(field "$f" id)"; [ -n "$cid" ] || continue
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

# Emit findings sorted by (check-id asc, change-id numeric asc) for determinism.
if [ -n "$FINDINGS" ]; then
  printf '%s' "$FINDINGS" | sort -t"$(printf '\t')" -k1,1 -k2,2n
fi

if [ "$STRICT" = 1 ] && [ -n "$FINDINGS" ]; then exit 1; fi
exit 0
