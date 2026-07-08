#!/usr/bin/env bash
# sync-agents.sh — generate docket's model/effort-pinned subagent wrappers into each PRESENT
# agent-harness dir, resolving layered config (built-in ⊕ global ⊕ per-repo).
#
# Unlike link-skills.sh (which SYMLINKS skills/<name>), agent files bake resolved model/effort,
# so they are GENERATED COPIES this script owns and OVERWRITES on every run.
#
# Layers & precedence — per-repo > global > built-in:
#   built-in  agents/docket-*.md in this repo (each ships its default model/effort)
#   global    ~/.config/docket/agents.yaml        -> user-level    ~/.claude/agents/docket-*.md
#   per-repo  <repo>/.docket.yml `agents:` block  -> project-level <repo>/.claude/agents/docket-*.md (committed)
# Claude Code applies project-over-user precedence natively, so the generator writes two layers
# (user = built-in⊕global, project = built-in⊕per-repo) and never hand-merges all three.
#
# Usage:
#   bash sync-agents.sh           # write user-level (built-in ⊕ global); and, if <repo>/.docket.yml
#                                 # has an `agents:` block, project-level (built-in ⊕ per-repo)
#   bash sync-agents.sh --check   # CI gate: exit non-zero (with a diff) if committed project-level
#                                 # files drift from what the resolved config would generate
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME for harness dirs and the global-config root
# (the latter only when XDG_CONFIG_HOME is unset — a set XDG_CONFIG_HOME wins; tests unset it).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_SRC="$SCRIPT_DIR/agents"
REPO="$PWD"

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
GLOBAL_CFG="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket/agents.yaml"
DOCKET_YML="$REPO/.docket.yml"

# Mirror link-skills.sh's HARNESS_SKILL_DIRS, swapping skills -> agents.
HARNESS_AGENT_DIRS=(
  "$HARNESS_ROOT/.claude/agents"
  "$HARNESS_ROOT/.codex/agents"
  "$HARNESS_ROOT/.cursor/agents"
  "$HARNESS_ROOT/.agents/agents"
  "$HARNESS_ROOT/.kiro/agents"
  "$HARNESS_ROOT/.windsurf/agents"
)

# Valid harness tokens, derived from HARNESS_AGENT_DIRS (single source of truth):
# ".../.claude/agents" -> "claude". The project-level dir for token H is $REPO/.<H>/agents.
VALID_HARNESS_TOKENS=""
for _hd in "${HARNESS_AGENT_DIRS[@]}"; do
  _hb="$(basename "$(dirname "$_hd")")"          # ".claude"
  VALID_HARNESS_TOKENS="$VALID_HARNESS_TOKENS ${_hb#.}"   # "claude"
done
unset _hd _hb

CHECK=0
[ "${1:-}" = "--check" ] && CHECK=1

log(){ printf '%s\n' "sync-agents: $*" >&2; }

is_valid_harness(){  # $1=token -> rc 0 if it is a known harness token
  [ -n "$1" ] || return 1
  case " $VALID_HARNESS_TOKENS " in *" $1 "*) return 0;; *) return 1;; esac
}

# Resolve the per-repo agent_harnesses flow-list from .docket.yml into HARNESSES
# (space-separated). Unset/empty-value => default "claude". Unknown tokens warned + dropped.
# Self-contained (no docket-config.sh); mirrors board_surfaces flow-list parsing.
resolve_agent_harnesses(){
  local raw list tok
  raw=""
  if [ -f "$DOCKET_YML" ]; then
    # top-level (column 0) key only; strip a trailing comment; capture-then-head (SIGPIPE-safe).
    raw="$(sed -n -E 's/^agent_harnesses[[:space:]]*:[[:space:]]*([^#]*).*/\1/p' "$DOCKET_YML")"
    raw="$(head -n1 <<<"$raw" | sed -E 's/[[:space:]]+$//')"
  fi
  if [ -z "$raw" ]; then
    HARNESSES="claude"                            # unset / bare key => default [claude]
    return 0
  fi
  list="${raw#[}"; list="${list%]}"; list="${list//,/ }"   # strip flow brackets, commas -> spaces
  HARNESSES=""
  set -f   # disable globbing: tokens are externally-sourced, e.g. a bare "*" must not glob-expand
  for tok in $list; do
    if is_valid_harness "$tok"; then
      HARNESSES="$HARNESSES $tok"
    else
      log "unknown agent_harnesses token '$tok' — ignored"
    fi
  done
  set +f
  HARNESSES="$(echo $HARNESSES)"                  # trim/collapse ("[]" or all-unknown => "")
}

short_name(){ local b; b="$(basename "$1")"; b="${b#docket-}"; printf '%s' "${b%.md}"; }

# --- config helpers ----------------------------------------------------------
# Print the body nested under the first bare `<key>:` header from stdin, DEDENTED to column 0
# at the block's base indent (so a nested doc's harness keys land at column 0 regardless of the
# parent's indentation). Body = lines strictly more-indented than the header, up to the next line
# at the header's indent-or-less. Values are printed raw (comment-stripping is the caller's job).
section_body() {  # $1=key ; reads stdin
  awk -v key="$1" '
    function ind(s,   m){ m=match(s, /[^ ]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }                 # first line back at/above header -> block done
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }         # dedent by the base indent
  '
}

# field_of() — UNCHANGED (kept verbatim from the prior version).
field_of() {  # $1=line  $2=field
  local out
  out="$(printf '%s' "$1" | sed -nE "s/.*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p")"
  head -n1 <<<"$out"
}

# Print the `agents.<harness>.<agent>` entry line from <file>. under_agents=1 => the harness map is
# nested under a top-level `agents:` key (.docket.yml); 0 => the harness map is the whole file (global).
harness_agent_line() {  # $1=file  $2=harness  $3=agent  $4=under_agents(0|1)
  local sub hbody stripped matched
  [ -f "$1" ] || return 0
  if [ "$4" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  hbody="$(printf '%s\n' "$sub" | section_body "$2")"                        # body under <harness>/<default>
  stripped="$(printf '%s\n' "$hbody" | sed 's/#.*//')"
  matched="$(printf '%s\n' "$stripped" | grep -E "^[[:space:]]*$3[[:space:]]*:" || true)"
  head -n1 <<<"$matched"
}

# Resolve (harness, agent) into RES_MODEL / RES_EFFORT via INDEPENDENT field-level fallback:
#   agents.<harness>.<agent>  ->  agents.default.<agent>   (built-in floor is handled by emit()).
# RES_MODEL_FROM_HARNESS=1 iff the model value came from the harness-specific line (not default).
resolve_agent() {  # $1=file  $2=harness  $3=agent  $4=under_agents(0|1)
  local hline dline hm he
  hline="$(harness_agent_line "$1" "$2" "$3" "$4")"
  dline="$(harness_agent_line "$1" default "$3" "$4")"
  hm="$(field_of "$hline" model)"; he="$(field_of "$hline" effort)"
  RES_MODEL_FROM_HARNESS=0
  if [ -n "$hm" ]; then RES_MODEL="$hm"; RES_MODEL_FROM_HARNESS=1; else RES_MODEL="$(field_of "$dline" model)"; fi
  if [ -n "$he" ]; then RES_EFFORT="$he"; else RES_EFFORT="$(field_of "$dline" effort)"; fi
}

# Union (sorted-unique) of agent keys configured under any harness sub-block or `default`.
agent_keys() {  # $1=file  $2=under_agents(0|1)
  local sub
  [ -f "$1" ] || return 0
  if [ "$2" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  printf '%s\n' "$sub" | awk '
    function ind(s,   m){ m=match(s,/[^ ]/); return (m==0?length(s):m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ { basei=ind(nc); inb=1; next }   # a harness/default header (col 0, bare)
    inb && nc ~ /[^[:space:]]/ && ind(nc) <= basei { inb=0 }
    inb && nc ~ /^[[:space:]]+[A-Za-z0-9._-]+[[:space:]]*:/ {
      k=nc; sub(/^[[:space:]]+/,"",k); sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }' | sort -u
}

# --- emit a resolved wrapper to stdout ---------------------------------------
# Rewrites model:/effort: lines inside the frontmatter. Empty override => keep built-in.
# effort override "auto" => drop the effort line entirely (inherit model default).
emit() {  # $1=src file  $2=model  $3=effort
  awk -v model="$2" -v effort="$3" '
    /^---[[:space:]]*$/ { d++; print; infm=(d==1); next }
    {
      if (infm && model!=""  && $0 ~ /^model[[:space:]]*:/)  { print "model: " model; next }
      if (infm && effort!="" && $0 ~ /^effort[[:space:]]*:/) { if (effort!="auto") print "effort: " effort; next }
      print
    }' "$1"
}

# Non-fatal footgun warning — body added in Task 2. No-op stub keeps Task 1's positive-path build green.
warn_fallback_model(){ :; }   # $1=harness $2=agent  (consumes RES_MODEL / RES_MODEL_FROM_HARNESS)

# --- passes ------------------------------------------------------------------
# Map a user-level harness *dir* ("$HARNESS_ROOT/.cursor/agents") to its token ("cursor").
harness_of_dir(){ local b; b="$(basename "$(dirname "$1")")"; printf '%s' "${b#.}"; }

user_level_pass() {  # built-in ⊕ global -> each present harness */agents dir, resolved per (harness, agent)
  local src dir name harness
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      [ -d "$(dirname "$dir")" ] || continue          # only write into a PRESENT harness root
      harness="$(harness_of_dir "$dir")"
      resolve_agent "$GLOBAL_CFG" "$harness" "$name" 0
      warn_fallback_model "$harness" "$name"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"
    done
  done
}

project_level_pass() {  # built-in ⊕ per-repo -> <repo>/.<H>/agents for each H in HARNESSES (committed)
  [ -f "$DOCKET_YML" ] || return 0
  local names name src harness dir
  names="$(agent_keys "$DOCKET_YML" 1)"
  [ -n "$names" ] || return 0
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    if [ ! -f "$src" ]; then
      log "skip '$name' — no built-in wrapper (advisory/interactive skills have no agent file)"
      continue
    fi
    for harness in $HARNESSES; do
      resolve_agent "$DOCKET_YML" "$harness" "$name" 1
      warn_fallback_model "$harness" "$name"
      dir="$REPO/.$harness/agents"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.md"
    done
  done <<EOF
$names
EOF
}

check_project_level() {  # diff committed <repo>/.<H>/agents files against freshly-resolved config (per harness)
  local rc=0 names name src got tmp d harness
  [ -f "$DOCKET_YML" ] || { log "no .docket.yml in $REPO — nothing to check"; return 0; }
  names="$(agent_keys "$DOCKET_YML" 1)"
  [ -n "$names" ] || { log "no agents: block — nothing to check"; return 0; }
  tmp="$(mktemp -d)"
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    [ -f "$src" ] || continue
    for harness in $HARNESSES; do
      resolve_agent "$DOCKET_YML" "$harness" "$name" 1      # harness-specific bytes (no longer harness-independent)
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"
      got="$REPO/.$harness/agents/docket-$name.md"
      if [ ! -f "$got" ]; then
        log "drift: missing $got (run: bash sync-agents.sh)"; rc=1; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"
      if [ -n "$d" ]; then log "drift in .$harness/agents/docket-$name.md:"; printf '%s\n' "$d" >&2; rc=1; fi
    done
  done <<EOF
$names
EOF
  rm -rf "$tmp"
  return $rc
}

resolve_agent_harnesses

if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

user_level_pass
project_level_pass
log "done"
