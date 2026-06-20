#!/usr/bin/env bash
# scripts/adr-checks.sh — the ADR-ledger analog of board-checks.sh (change 0030). Sources the shared
# frontmatter helper (0022) and walks the ADR files, emitting one finding per line on stdout. Offline
# (no gh, no network) and warn-only (never auto-fixes); the caller (docket-adr) surfaces the lines.
#
# Usage: adr-checks.sh --adrs-dir DIR [--strict]
#   Findings: TAB-separated  <check-id>\t<adr-id>\t<message>  on stdout, sorted by (check-id, adr-id).
#     check-id ∈ {adr-numbering-gap, adr-dangling-link, adr-status-inconsistent}
#   Clean ledger => no output, exit 0. --strict => exit 1 if any finding (a future CI gate).
set -uo pipefail

ADRS_DIR=""; STRICT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --strict) STRICT=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'adr-checks: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$ADRS_DIR" ] || { printf 'adr-checks: missing --adrs-dir\n' >&2; exit 2; }
[ -d "$ADRS_DIR" ] || { printf 'adr-checks: adrs dir not found: %s\n' "$ADRS_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

pad(){ printf '%04d' "$1"; }

# --- single scan: existence + status + cross-ref lists, keyed by integer id ---
declare -A EXISTS STATUS SUPS REVS REL
IDS=""; MAXID=0
mapfile -t FILES < <(find "$ADRS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  EXISTS["$id"]=1
  STATUS["$id"]="$(field "$f" status)"
  SUPS["$id"]="$(list_field "$f" supersedes)"
  REVS["$id"]="$(list_field "$f" reverses)"
  REL["$id"]="$(list_field "$f" relates_to)"
  IDS+="$id"$'\n'
  [ "$id" -gt "$MAXID" ] && MAXID="$id"
done

FINDINGS=""
emit(){ FINDINGS+="$1"$'\t'"$2"$'\t'"$3"$'\n'; }

# status_target STATUS -> bare integer id from "Superseded by ADR-0006" / "Reversed by ADR-0006" ("" otherwise)
status_target(){
  case "$1" in
    "Superseded by ADR-"*|"Reversed by ADR-"*)
      local t="${1##*ADR-}"; t="${t%% *}"; printf '%d' "$((10#$t))" ;;
    *) printf '' ;;
  esac
}

# status_verb STATUS -> "supersedes" | "reverses" | "" — the edge a back-pointer status implies
status_verb(){
  case "$1" in
    "Superseded by ADR-"*) printf 'supersedes' ;;
    "Reversed by ADR-"*)   printf 'reverses' ;;
    *) printf '' ;;
  esac
}

# --- adr-numbering-gap: every id missing from 1..MAXID ---
n=1
while [ "$n" -le "$MAXID" ]; do
  [ -z "${EXISTS[$n]:-}" ] && emit adr-numbering-gap "$n" "no ADR file for id $n (gap in 1..$MAXID)"
  n=$(( n + 1 ))
done

# iterate ids in ascending numeric order for deterministic per-adr findings
SORTED_IDS="$(printf '%s' "$IDS" | sed '/^$/d' | sort -n)"
while IFS= read -r id; do
  [ -n "$id" ] || continue

  # --- adr-dangling-link: any cross-ref to an id with no file ---
  for ref in ${SUPS[$id]} ${REVS[$id]} ${REL[$id]}; do
    [ -z "${EXISTS[$ref]:-}" ] && emit adr-dangling-link "$id" "references ADR-$(pad "$ref") which has no file"
  done

  # --- adr-status-inconsistent arm (a): status says Superseded/Reversed by a non-existent ADR ---
  tgt="$(status_target "${STATUS[$id]}")"
  if [ -n "$tgt" ] && [ -z "${EXISTS[$tgt]:-}" ]; then
    emit adr-status-inconsistent "$id" "status '${STATUS[$id]}' but no ADR-$(pad "$tgt") exists"
  fi

  # --- adr-status-inconsistent arm (b): supersedes/reverses target NOT flipped back (verb-aware) ---
  # a supersedes edge requires the target status 'Superseded by ADR-X'; a reverses edge requires
  # 'Reversed by ADR-X'. Right id but wrong verb is a finding, not a silent pass.
  for ref in ${SUPS[$id]}; do
    [ -n "${EXISTS[$ref]:-}" ] || continue              # dangling already flagged above
    back="$(status_target "${STATUS[$ref]}")"
    verb="$(status_verb "${STATUS[$ref]}")"
    if [ "$back" != "$id" ] || [ "$verb" != supersedes ]; then
      emit adr-status-inconsistent "$ref" "ADR-$(pad "$id") supersedes it but its status is '${STATUS[$ref]}' (expected 'Superseded by ADR-$(pad "$id")')"
    fi
  done
  for ref in ${REVS[$id]}; do
    [ -n "${EXISTS[$ref]:-}" ] || continue              # dangling already flagged above
    back="$(status_target "${STATUS[$ref]}")"
    verb="$(status_verb "${STATUS[$ref]}")"
    if [ "$back" != "$id" ] || [ "$verb" != reverses ]; then
      emit adr-status-inconsistent "$ref" "ADR-$(pad "$id") reverses it but its status is '${STATUS[$ref]}' (expected 'Reversed by ADR-$(pad "$id")')"
    fi
  done
done <<<"$SORTED_IDS"

# --- emit sorted by (check-id asc, adr-id numeric asc) ---
if [ -n "$FINDINGS" ]; then
  printf '%s' "$FINDINGS" | sort -t"$(printf '\t')" -k1,1 -k2,2n
fi

if [ "$STRICT" = 1 ] && [ -n "$FINDINGS" ]; then exit 1; fi
exit 0
