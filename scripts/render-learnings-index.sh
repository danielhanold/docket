#!/usr/bin/env bash
# scripts/render-learnings-index.sh — deterministic, idempotent renderer for the learnings index
# (<changes_dir>/learnings/README.md), change 0067. The exact analog of render-adr-index.sh (0030)
# and render-board.sh (0022): reads the finding files and emits the index to STDOUT byte-for-byte.
# No git writes (the caller redirects + commits), offline (no gh, no git, no network). Same finding
# files => identical bytes. Reuses lib/docket-frontmatter.sh.
#
# PURE BY DESIGN: this script has no learnings.enabled awareness. The CALLERS gate on it — exactly
# as render-board.sh stays pure while board-refresh.sh/docket-status.sh own the write decision.
#
# Usage: render-learnings-index.sh --learnings-dir DIR
set -uo pipefail

LEARNINGS_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --learnings-dir) LEARNINGS_DIR="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-learnings-index: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$LEARNINGS_DIR" ] || { printf 'render-learnings-index: missing --learnings-dir\n' >&2; exit 2; }
[ -d "$LEARNINGS_DIR" ] || { printf 'render-learnings-index: learnings dir not found: %s\n' "$LEARNINGS_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

# `field` returns the RAW scalar — quotes intact. `hook` is REQUIRED to be quoted (it carries a
# colon-space; YAML-scalar family), so it must be dequoted here or the index ships quote bytes.
# A double-quoted scalar may carry YAML's own backslash escapes (e.g. a hook that itself discusses
# quoting: `hook: "Never \"fix\" a guard by widening it."`), so dequoting it is a real left-to-right
# unescape (`\"` -> `"`, `\\` -> `\`), not just outer-char stripping — a single-quoted scalar's only
# escape is `''` -> `'`. Only a MATCHED outer pair is stripped: an unquoted scalar (even one
# containing quote characters) or an unterminated/mismatched quote passes through unchanged.
dequote(){
  local v="$1"
  local n=${#v}
  if [ "$n" -ge 2 ] && [ "${v:0:1}" = '"' ] && [ "${v: -1}" = '"' ] && _dq_dquote_closer_is_real "$v"; then
    _dq_unescape_dquote "${v:1:n-2}"
    return
  fi
  if [ "$n" -ge 2 ] && [ "${v:0:1}" = "'" ] && [ "${v: -1}" = "'" ]; then
    local inner="${v:1:n-2}"
    printf '%s' "${inner//\'\'/\'}"
    return
  fi
  printf '%s' "$v"
}

# _dq_dquote_closer_is_real STR -> true iff STR's last character (already confirmed to be a ")
# is an unescaped closing quote rather than the second half of a `\"` escape: count the run of
# backslashes immediately before it — an escaped quote always sits behind an ODD-length run, a
# real delimiter behind an EVEN-length run (0 included). Guards the unterminated/mismatched case:
# a value like `"foo\"` (no real closing quote at all) must NOT be treated as a matched pair.
_dq_dquote_closer_is_real(){
  local s="$1"
  local i=$(( ${#s} - 2 ))
  local run=0
  while [ "$i" -ge 0 ] && [ "${s:i:1}" = '\' ]; do
    run=$((run + 1))
    i=$((i - 1))
  done
  [ $((run % 2)) -eq 0 ]
}

# _dq_unescape_dquote STR -> single left-to-right pass over a double-quoted scalar's INNER content
# (outer quotes already confirmed matched and stripped): `\"` -> `"`, `\\` -> `\`; any other
# backslash is copied through literally (not consumed, not paired with what follows). Single pass,
# not two sequential global replaces: composing an isolated `\\`->`\` pass with a separate `\"`->`"`
# pass can regroup characters the first pass already resolved. The case this guards: a literal
# backslash immediately before the closing quote (`...\\"` — an escaped backslash, THEN the real
# delimiter) must consume as one `\\` pair; a naive two-pass replace can instead let the backslash
# left over from that pair re-pair with the quote as if it were `\"`, corrupting or dropping bytes.
_dq_unescape_dquote(){
  local s="$1"
  local n=${#s}
  local out=""
  local i=0
  local c nc
  while [ "$i" -lt "$n" ]; do
    c="${s:i:1}"
    if [ "$c" = '\' ] && [ $((i + 1)) -lt "$n" ]; then
      nc="${s:i+1:1}"
      if [ "$nc" = '"' ] || [ "$nc" = '\' ]; then
        out+="$nc"
        i=$((i + 2))
      else
        out+="$c"
        i=$((i + 1))
      fi
    else
      out+="$c"
      i=$((i + 1))
    fi
  done
  printf '%s' "$out"
}

declare -A F_HOOK F_TOPICS F_STATE F_TO
SLUGS=""
# This `sort` is defense-in-depth, not the determinism source: scan order is fully
# re-canonicalized downstream by TOPICS_SORTED (sort -u), the per-topic row loop (sort), and
# PROMOTED_SORTED (sort) — those three are what byte-identical output actually depends on.
mapfile -t FILES < <(find "$LEARNINGS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  slug="$(field "$f" slug)"; [ -n "$slug" ] || continue
  F_HOOK["$slug"]="$(dequote "$(field "$f" hook)")"
  F_TOPICS["$slug"]="$(list_field "$f" topics)"
  state="$(field "$f" promotion_state)"
  # Positive off-state (ADR-0032): an unset/unknown state is NOT silently "retained" for the
  # purposes of the hint surface — it renders as retained, but only `promoted` leaves the groups
  # and only `candidate` earns the marker, so an empty value degrades to the safe, visible tier.
  F_STATE["$slug"]="${state:-retained}"
  F_TO["$slug"]="$(field "$f" promoted_to)"
  SLUGS+="$slug"$'\n'
done

primary_topic(){ # primary_topic SLUG -> first tag, or "uncategorized"
  local t; t="$(printf '%s' "${F_TOPICS[$1]}" | awk '{print $1}')"
  printf '%s' "${t:-uncategorized}"
}
rest_topics(){ # rest_topics SLUG -> "b, c" (empty when only one tag)
  printf '%s' "${F_TOPICS[$1]}" | awk '{ for(i=2;i<=NF;i++){ printf "%s%s", (i>2 ? ", " : ""), $i } }'
}

# --- partition: promoted findings leave the paid surface for the appendix -------------------
ACTIVE=""; PROMOTED=""
while IFS= read -r s; do
  [ -n "$s" ] || continue
  if [ "${F_STATE[$s]}" = "promoted" ]; then PROMOTED+="$s"$'\n'; else ACTIVE+="$s"$'\n'; fi
done <<<"$SLUGS"

# --- topic buckets (derived, sorted — no hand-listed topic set) -----------------------------
TOPICS_SEEN=""
while IFS= read -r s; do
  [ -n "$s" ] || continue
  TOPICS_SEEN+="$(primary_topic "$s")"$'\n'
done <<<"$ACTIVE"
TOPICS_SORTED="$(printf '%s' "$TOPICS_SEEN" | sed '/^$/d' | sort -u)"

row(){ # row SLUG
  local s="$1" line rest marker=""
  [ "${F_STATE[$s]}" = "candidate" ] && marker=" ⟨needs promotion⟩"
  line="- [$s]($s.md) — ${F_HOOK[$s]}"
  rest="$(rest_topics "$s")"
  [ -n "$rest" ] && line+=" · also: $rest"
  printf '%s%s\n' "$line" "$marker"
}

printf '# Learnings — the build loop'"'"'s memory\n\n'
printf 'One curated finding per file; this index is the hint surface. Load it, then read only the findings that bear on the change at hand. Generated by `render-learnings-index.sh` — do not hand-edit. Contract: docket-convention, "Learnings ledger".\n'

if [ -n "$TOPICS_SORTED" ]; then
  while IFS= read -r topic; do
    [ -n "$topic" ] || continue
    printf '\n## %s\n\n' "$topic"
    while IFS= read -r s; do
      [ -n "$s" ] || continue
      [ "$(primary_topic "$s")" = "$topic" ] && row "$s"
    done <<<"$(printf '%s' "$ACTIVE" | sed '/^$/d' | sort)"
  done <<<"$TOPICS_SORTED"
fi

PROMOTED_SORTED="$(printf '%s' "$PROMOTED" | sed '/^$/d' | sort)"
if [ -n "$PROMOTED_SORTED" ]; then
  printf '\n## Promoted\n\n'
  printf 'Graduated to an always-in-context agent-instructions file. Kept as the rule'"'"'s receipt, the harvest'"'"'s dedup memory, and a one-line-reversible demotion path — they no longer count against the cap.\n\n'
  while IFS= read -r s; do
    [ -n "$s" ] || continue
    printf -- '- [%s](%s.md) → %s\n' "$s" "$s" "${F_TO[$s]}"
  done <<<"$PROMOTED_SORTED"
fi
