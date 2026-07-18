#!/usr/bin/env bash
# scripts/lib/docket-frontmatter.sh — shared frontmatter + dependency-resolution helper for
# docket's deterministic board/mirror scripts (change 0022). SOURCE this; it has no side effects
# on source beyond declaring functions and the dependency-resolution globals. No git, no network.
#
# Provides:
#   field FILE KEY        — first top-level frontmatter scalar for KEY, trimmed.
#   list_field FILE KEY   — `[a, b]` -> space-separated `a b` (empty for `[]` / unset).
#   int_field FILE KEY    — like field(), but empty unless the value is a well-formed non-negative integer.
#   has_section FILE STR  — exit 0 iff the body contains the literal line STR.
#   iso_to_epoch ISO      — UTC ISO-8601 timestamp -> epoch seconds; empty on parse failure.
#   resolve_deps DIR      — scan DIR/active + DIR/archive once; populate the globals below.
#   readiness FILE        — build-ready | needs-brainstorm | auto-groom-blocked | waiting.
#   finalize_blocked FILE — exit 0 iff the body carries `## Finalize blocked` (implemented only).
#
# resolve_deps globals (keyed by integer id):
#   STATUS_OF[id]   the change's own status
#   DEP_STATE[id]   clear | waiting
#   DEP_REASON[id]  "" | "not yet built" | "needs your merge"   (worst unmet; needs-your-merge wins)
#   DEP_ON[id]      bare id of the worst unmet dependency ("" when clear) — display support for #N

# --- frontmatter accessors (lifted from github-mirror.sh, which now sources them here) --------
field(){
  local raw; raw="$(sed -n "s/^$2:[[:space:]]*//p" "$1")"
  raw="${raw%%$'\n'*}"                              # keep only the first matching line — no pipe
  printf '%s\n' "${raw%"${raw##*[![:space:]]}"}"   # strip trailing whitespace; trailing \n matches the
}                                                  # original sed form (callers that pipe field directly,
                                                   # e.g. the mermaid done-id list, rely on the separator)
list_field(){
  local raw; raw="$(field "$1" "$2")"
  raw="${raw#[}"; raw="${raw%]}"
  printf '%s' "$raw" | tr ',' ' ' | xargs 2>/dev/null || true
}
# int_field FILE KEY — like field(), but returns the value ONLY when it is a well-formed
# non-negative integer (^[0-9]+$); empty string otherwise. Pure; no side effects on source.
int_field(){
  local v; v="$(field "$1" "$2")"
  case "$v" in (''|*[!0-9]*) printf '' ;; (*) printf '%s' "$v" ;; esac
}
has_section(){ grep -qF "$2" "$1"; }

# iso_to_epoch ISO — convert a UTC ISO-8601 second-precision timestamp (YYYY-MM-DDTHH:MM:SSZ) to
# epoch seconds on stdout. Tries GNU date first, then BSD/macOS date. Returns 1 (empty stdout) on
# a parse failure — callers treat "no epoch" as "no positive evidence" (never as expired). Single
# source: both board-checks.sh and reclaim-claims.sh use it (do NOT duplicate — escape-ere twin rule).
iso_to_epoch(){
  local iso="$1" e
  e="$(date -u -d "$iso" +%s 2>/dev/null)"                         && { printf '%s' "$e"; return 0; }
  e="$(date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$iso" +%s 2>/dev/null)" && { printf '%s' "$e"; return 0; }
  return 1
}

# --- dependency resolution ----------------------------------------------------
declare -gA STATUS_OF DEP_STATE DEP_REASON DEP_ON

resolve_deps(){ # resolve_deps CHANGES_DIR
  local dir="$1" f id dep dstat worst worst_on
  STATUS_OF=(); DEP_STATE=(); DEP_REASON=(); DEP_ON=()
  local -a files
  mapfile -t files < <(find "$dir/active" "$dir/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  # pass 1: id -> own status
  for f in "${files[@]}"; do
    id="$(int_field "$f" id)"; [ -n "$id" ] || continue
    STATUS_OF["$id"]="$(field "$f" status)"
  done
  # pass 2: resolve each change's depends_on into the worst unmet reason + its id
  for f in "${files[@]}"; do
    id="$(int_field "$f" id)"; [ -n "$id" ] || continue
    worst=""; worst_on=""
    for dep in $(list_field "$f" depends_on); do
      dstat="${STATUS_OF[$dep]:-}"
      if [ "$dstat" = "done" ]; then
        continue                                   # satisfied
      elif [ "$dstat" = "implemented" ]; then
        if [ "$worst" != "needs your merge" ]; then worst="needs your merge"; worst_on="$dep"; fi
      else
        if [ -z "$worst" ]; then worst="not yet built"; worst_on="$dep"; fi
      fi
    done
    if [ -n "$worst" ]; then
      DEP_STATE["$id"]="waiting"; DEP_REASON["$id"]="$worst"; DEP_ON["$id"]="$worst_on"
    else
      DEP_STATE["$id"]="clear"; DEP_REASON["$id"]=""; DEP_ON["$id"]=""
    fi
  done
}

# --- readiness (precedence pinned: waiting > missing-spec > build-ready) -------
readiness(){ # readiness FILE  (only meaningful for a proposed change)
  local f="$1" id spec trivial
  id="$(int_field "$f" id)"
  if [ "${DEP_STATE[$id]:-clear}" = "waiting" ]; then printf 'waiting'; return; fi
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"
  if [ -z "$spec" ] && [ "$trivial" != "true" ]; then
    if has_section "$f" "## Auto-groom blocked"; then printf 'auto-groom-blocked'
    else printf 'needs-brainstorm'; fi
    return
  fi
  printf 'build-ready'
}

finalize_blocked(){ # finalize_blocked FILE  (only meaningful for an implemented change)
  # `## Finalize blocked` is presence-encoded state written by docket-finalize-change when a gate
  # failure leaves a change needing a human. Deliberately NOT part of readiness(), which is by
  # contract meaningful only for a `proposed` change.
  has_section "$1" "## Finalize blocked"
}
