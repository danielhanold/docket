#!/usr/bin/env bash
# scripts/lib/docket-frontmatter.sh — shared frontmatter + dependency-resolution helper for
# docket's deterministic board/mirror scripts (change 0022). SOURCE this; it has no side effects
# on source beyond declaring functions and the dependency-resolution globals. No git, no network.
#
# Provides:
#   field FILE KEY        — first top-level frontmatter scalar for KEY, trimmed.
#   list_field FILE KEY   — `[a, b]` -> space-separated `a b` (empty for `[]` / unset).
#   has_section FILE STR  — exit 0 iff the body contains the literal line STR.
#   resolve_deps DIR      — scan DIR/active + DIR/archive once; populate the globals below.
#   readiness FILE        — build-ready | needs-brainstorm | auto-groom-blocked | waiting.
#
# resolve_deps globals (keyed by integer id):
#   STATUS_OF[id]   the change's own status
#   DEP_STATE[id]   clear | waiting
#   DEP_REASON[id]  "" | "not yet built" | "needs your merge"   (worst unmet; needs-your-merge wins)
#   DEP_ON[id]      bare id of the worst unmet dependency ("" when clear) — display support for #N

# --- frontmatter accessors (verbatim from github-mirror.sh) -------------------
field(){
  local raw; raw="$(sed -n "s/^$2:[[:space:]]*//p" "$1")"
  raw="${raw%%$'\n'*}"                            # keep only the first matching line — no pipe
  printf '%s' "${raw%"${raw##*[![:space:]]}"}"    # strip trailing whitespace
}
list_field(){
  local raw; raw="$(field "$1" "$2")"
  raw="${raw#[}"; raw="${raw%]}"
  printf '%s' "$raw" | tr ',' ' ' | xargs 2>/dev/null || true
}
has_section(){ grep -qF "$2" "$1"; }

# --- dependency resolution ----------------------------------------------------
declare -gA STATUS_OF DEP_STATE DEP_REASON DEP_ON

resolve_deps(){ # resolve_deps CHANGES_DIR
  local dir="$1" f id dep dstat worst worst_on
  STATUS_OF=(); DEP_STATE=(); DEP_REASON=(); DEP_ON=()
  local -a files
  mapfile -t files < <(find "$dir/active" "$dir/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  # pass 1: id -> own status
  for f in "${files[@]}"; do
    id="$(field "$f" id)"; [ -n "$id" ] || continue
    STATUS_OF["$id"]="$(field "$f" status)"
  done
  # pass 2: resolve each change's depends_on into the worst unmet reason + its id
  for f in "${files[@]}"; do
    id="$(field "$f" id)"; [ -n "$id" ] || continue
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
  id="$(field "$f" id)"
  if [ "${DEP_STATE[$id]:-clear}" = "waiting" ]; then printf 'waiting'; return; fi
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"
  if [ -z "$spec" ] && [ "$trivial" != "true" ]; then
    if has_section "$f" "## Auto-groom blocked"; then printf 'auto-groom-blocked'
    else printf 'needs-brainstorm'; fi
    return
  fi
  printf 'build-ready'
}
