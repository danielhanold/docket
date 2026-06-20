#!/usr/bin/env bash
# scripts/render-adr-index.sh — deterministic, idempotent renderer for the ADR index
# (<adrs_dir>/README.md), change 0030. The exact analog of render-board.sh (0022): reads the ADR
# files and emits the index to STDOUT byte-for-byte per docket-adr's *Index / validate* structure.
# No git writes (the caller redirects + commits), offline (no gh, no git, no network). Same ADR
# files => identical bytes. Reuses lib/docket-frontmatter.sh.
#
# Usage: render-adr-index.sh --adrs-dir DIR
set -uo pipefail

ADRS_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-adr-index: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$ADRS_DIR" ] || { printf 'render-adr-index: missing --adrs-dir\n' >&2; exit 2; }
[ -d "$ADRS_DIR" ] || { printf 'render-adr-index: adrs dir not found: %s\n' "$ADRS_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

pad(){ printf '%04d' "$1"; }                       # bare id -> 4-digit
adr_list(){ # "1 5" -> "ADR-0001, ADR-0005"
  local out="" x
  for x in $1; do [ -n "$out" ] && out+=", "; out+="ADR-$(pad "$x")"; done
  printf '%s' "$out"
}

# --- single scan: collect every ADR (excluding README.md) into parallel maps + group buckets ---
declare -A T_FILE T_TITLE T_STATUS T_CHANGE T_SUPS T_REVS T_REL
ACTIVE_IDS=""; SUPREV_IDS=""; DEPR_IDS=""
mapfile -t FILES < <(find "$ADRS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  T_FILE["$id"]="$(basename "$f")"
  T_TITLE["$id"]="$(field "$f" title)"
  T_STATUS["$id"]="$(field "$f" status)"
  T_CHANGE["$id"]="$(field "$f" change)"
  T_SUPS["$id"]="$(list_field "$f" supersedes)"
  T_REVS["$id"]="$(list_field "$f" reverses)"
  T_REL["$id"]="$(list_field "$f" relates_to)"
  case "${T_STATUS[$id]}" in
    "Superseded by"*|"Reversed by"*) SUPREV_IDS+="$id"$'\n' ;;
    Deprecated)                      DEPR_IDS+="$id"$'\n' ;;
    *)                               ACTIVE_IDS+="$id"$'\n' ;;   # Accepted/Proposed/draft/unknown
  esac
done

row(){ # row ID GROUP (group: active|suprev|depr)
  local id="$1" group="${2:-active}" line ann=""
  line="- [ADR-$(pad "$id")](${T_FILE[$id]}) — ${T_TITLE[$id]} (${T_STATUS[$id]})"
  # change back-ref only emitted for Active ADRs (golden contract)
  [ "$group" = "active" ] && [ -n "${T_CHANGE[$id]}" ] && ann+=" ← change #${T_CHANGE[$id]}"
  [ -n "${T_SUPS[$id]}" ]    && ann+=" → supersedes $(adr_list "${T_SUPS[$id]}")"
  [ -n "${T_REVS[$id]}" ]    && ann+=" → reverses $(adr_list "${T_REVS[$id]}")"
  [ -n "${T_REL[$id]}" ]     && ann+=" · relates to $(adr_list "${T_REL[$id]}")"
  printf '%s%s\n' "$line" "$ann"
}

emit_group(){ # emit_group HEADER IDSTR GROUP
  printf '\n## %s\n\n' "$1"
  local sorted id
  sorted="$(printf '%s' "$2" | sed '/^$/d' | sort -n)"
  if [ -z "$sorted" ]; then printf '_None._\n'; return; fi
  while IFS= read -r id; do [ -n "$id" ] && row "$id" "${3:-active}"; done <<<"$sorted"
}

printf '# Architecture Decision Records\n\n'
printf 'Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.\n'
emit_group "Active" "$ACTIVE_IDS" "active"
emit_group "Superseded / Reversed" "$SUPREV_IDS" "suprev"
emit_group "Deprecated" "$DEPR_IDS" "depr"
