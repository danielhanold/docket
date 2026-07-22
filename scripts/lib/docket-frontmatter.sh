#!/usr/bin/env bash
# scripts/lib/docket-frontmatter.sh — shared frontmatter, dependency-resolution, and vocabulary
# helper for
# docket's deterministic board/mirror scripts (change 0022). SOURCE this; it has no side effects
# on source beyond declaring functions and the dependency-resolution globals. No git, no network.
#
# Provides:
#   field FILE KEY        — first matching scalar for KEY anywhere in the file, trimmed.
#   fm_field FILE KEY     — like field(), but ONLY inside the first ---...--- block. Use this for
#                           any key that may be ABSENT from frontmatter (e.g. type:), where field()
#                           would fall through and return body prose.
#   list_field FILE KEY   — `[a, b]` -> space-separated `a b` (empty for `[]` / unset).
#   int_field FILE KEY    — like field(), but empty unless the value is a well-formed non-negative integer.
#   has_section FILE STR  — exit 0 iff the body contains the literal line STR (whole-line match:
#                           a prose mention of the marker is NOT the section).
#   iso_to_epoch ISO      — UTC ISO-8601 timestamp -> epoch seconds; empty on parse failure.
#   resolve_deps DIR      — scan DIR/active + DIR/archive once; populate the globals below.
#   readiness FILE        — build-ready | needs-brainstorm | auto-groom-blocked | waiting.
#   finalize_blocked FILE — exit 0 iff the body carries `## Finalize blocked` (implemented only).
#   docket_status_is_active STATUS   — exit 0 iff STATUS is a non-terminal lifecycle status.
#   docket_status_is_terminal STATUS — exit 0 iff STATUS is a terminal lifecycle status.
#   docket_priority_is_member VALUE  — exit 0 iff VALUE is a declared priority (empty is false).
#   docket_priority_rank VALUE       — print the rank index; empty/unknown uses the default rank.
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
# fm_field FILE KEY — like field(), but reads ONLY inside the FIRST ---...--- block (change 0127).
#
# field() scans the whole file and takes the first match. For the pre-0127 fields that is safe: the
# frontmatter sits at the top, so its line always wins over any body prose discussing the same key.
# It is NOT safe for a key that may be ABSENT from frontmatter while present in body prose — the
# match then falls through to the body and returns prose as a value. `type:` is exactly that case
# during the migration window: every un-backfilled change has no frontmatter type:, and a change
# whose body happens to open a line with `type:` would otherwise render its prose as the type and
# make the backfill refuse to touch it. Anchoring is the same discipline AGENTS.md already requires
# for frontmatter WRITES, applied to the read.
fm_field(){ # fm_field FILE KEY -> value on stdout (empty when absent from the first block)
  awk -v key="$2" '
    BEGIN { n = 0 }
    /^---[[:space:]]*$/ { n++; if (n >= 2) exit; next }
    n == 1 {
      if ($0 ~ ("^" key ":")) {
        sub("^" key ":[[:space:]]*", "")
        sub(/[[:space:]]+$/, "")
        print
        exit
      }
    }
  ' "$1"
}

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
# has_section FILE STR — exit 0 iff some line of FILE is EXACTLY STR. `-x` is load-bearing, not a
# nicety: these markers are presence-encoded state, and change files routinely *mention* them in
# prose (`… a dated `## Finalize blocked` body section …`). An unanchored substring match turns any
# such mention into a false "this change is blocked" cell on the board. Whole-line only.
has_section(){ grep -qxF "$2" "$1"; }

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

publish_deferred(){ # publish_deferred FILE  (meaningful on any change file, active or archived)
  # `## Publish deferred` is presence-encoded state written by mark-publish-deferred.sh when a
  # terminal close-out's publish step was EXPECTED but deferred or blocked (change 0083). Unlike
  # finalize_blocked(), this has NO status gate: the marker is written on the ARCHIVED file, at
  # which point the change is terminal, so gating on a lifecycle status would make it unreadable
  # exactly where it is written. Presence is the whole state.
  has_section "$1" "## Publish deferred"
}

# --- status vocabulary (change 0104) ----------------------------------------------------------
# The seven lifecycle statuses, authored as the convention's two semantic groups: `active/` holds
# every non-terminal status, `archive/` holds the two terminal outcomes. DOCKET_STATUSES is the
# concatenation, in the renderer's display order — the order IS the contract (BOARD.md's section
# order and the digest's `backlog` rollup order both come from iterating it), so never reorder
# these without re-blessing tests/test_render_board.sh's golden.
#
# Single source for render-board.sh's section iteration AND board-checks.sh's `status` field-domain
# check. Duplicating the list makes the checker and the renderer drift in two directions and only
# one of them is detectable: a status added to the renderer but not the checker makes field-domain
# fire a FALSE finding on every file carrying it (and suppresses the board-row-dropped backstop,
# which would otherwise be the thing that noticed), while the reverse direction is caught.
DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented)
DOCKET_STATUSES_TERMINAL=(done killed)
DOCKET_STATUSES=("${DOCKET_STATUSES_ACTIVE[@]}" "${DOCKET_STATUSES_TERMINAL[@]}")

# --- priority vocabulary (change 0116) --------------------------------------------------------
# Ordered by rank, descending. The order IS the convention's deterministic selection semantics:
# critical > high > medium > low. The array index is the ready-queue sort rank.
DOCKET_PRIORITIES=(critical high medium low)
# The default is an independent documented fact, not a positional consequence of the array.
DOCKET_PRIORITY_DEFAULT=medium

# --- change-type vocabulary (change 0127) -----------------------------------------------------
# The BUILT-IN taxonomy. `change_types` in .docket.yml can replace this whole list (never merge
# with it), so every consumer takes an EFFECTIVE list as an argument and this array is only the
# default the resolver falls back to. Order is significant: it survives the resolver's export and
# is the canonical sequence any type-ordered output follows.
DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf)

# Pseudo-values that are legal as a config selector or a QUERY token but never legal in a stored
# manifest: `all` is the auto_capture.types selector and the --type wildcard; `untyped` is the
# --type query token for a change carrying no type: yet, and the backfill's migration-set name.
# Storing either would make a selector indistinguishable from a real value.
DOCKET_CHANGE_TYPE_RESERVED=(all untyped)

_docket_array_has(){
  local needle="$1"; shift
  local value
  [ -n "$needle" ] || return 1
  for value in "$@"; do [ "$needle" = "$value" ] && return 0; done
  return 1
}
docket_status_is_active(){ _docket_array_has "$1" "${DOCKET_STATUSES_ACTIVE[@]}"; }
docket_status_is_terminal(){ _docket_array_has "$1" "${DOCKET_STATUSES_TERMINAL[@]}"; }
docket_priority_is_member(){ _docket_array_has "$1" "${DOCKET_PRIORITIES[@]}"; }

# Membership over the EFFECTIVE list the caller resolved — never over the built-in array. A change
# file may legitimately carry a type absent from THIS machine's effective list (another machine's
# config wrote it), so readers must not use this to decide whether to RENDER a stored value; it
# gates creation and admission only.
docket_change_type_is_member(){ # docket_change_type_is_member VALUE TYPE...
  local value="$1"; shift
  _docket_array_has "$value" "$@"
}

docket_change_type_is_reserved(){ # docket_change_type_is_reserved VALUE
  _docket_array_has "$1" "${DOCKET_CHANGE_TYPE_RESERVED[@]}"
}

# Shape gate for the spec's `[a-z][a-z0-9-]*`, keyed on shape rather than an enumerated set of bad
# spellings (AGENTS.md). Deliberately pure `case` — no `printf | grep -Eq`, which would be a
# producer piped into an early-exiting consumer (SIGPIPE 141 under pipefail), and whose line-wise
# match would accept a multi-line value on the strength of its first line alone. The two patterns
# together reject: empty, a non-lowercase-alpha first character, and any subsequent character
# outside [a-z0-9-] — including a space, colon, underscore, or embedded newline.
docket_change_type_is_wellformed(){ # docket_change_type_is_wellformed VALUE
  case "$1" in
    ''|[!a-z]*)   return 1 ;;
    *[!a-z0-9-]*) return 1 ;;
  esac
  return 0
}
docket_priority_rank(){
  local wanted="$1" value i=0
  docket_priority_is_member "$wanted" || wanted="$DOCKET_PRIORITY_DEFAULT"
  for value in "${DOCKET_PRIORITIES[@]}"; do
    [ "$wanted" = "$value" ] && { printf '%s' "$i"; return 0; }
    i=$(( i + 1 ))
  done
  return 1
}

# --- board-checks check-id vocabulary (change 0111) --------------------------------------------
# The CLOSED check-id vocabulary board-checks.sh emits. Declared HERE, beside DOCKET_STATUSES,
# rather than in board-checks.sh itself, because board-checks.sh is not sourceable — a guard
# wanting the set would have to parse its source text, manufacturing exactly the tokenizer that
# can drift from what bash actually assigns. This lib IS sourceable (board-checks.sh's
# `source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"` runs well before its
# `emit()` definition), so tests/test_board_checks.sh reads the real runtime array.
#
# Accepted impurity: this lib's name says "frontmatter" and a check-id is not a frontmatter field.
# Noted deliberately; rationalising the lib's naming is change 0116's charter.
#
# Every entry is pinned in BOTH directions against the set board-checks.sh emits, against the
# script's own --help header enumeration, against scripts/board-checks.md's per-check sections, and
# against scripts/docket-status.md's `check` report-line row. Adding a check-id means editing the
# array plus the four surfaces it is pinned against; the guard's failure messages name them.
BOARD_CHECK_IDS=(board-row-dropped broken-plan-results broken-spec dep-cycle field-domain
                 malformed-id merge-gate-stall merged-orphan publish-deferred
                 stale-finalize-blocked stale-in-progress unknown-commit-ref)
