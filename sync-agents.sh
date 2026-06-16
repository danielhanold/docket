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
PROJECT_AGENT_DIR="$REPO/.claude/agents"

# Mirror link-skills.sh's HARNESS_SKILL_DIRS, swapping skills -> agents.
HARNESS_AGENT_DIRS=(
  "$HARNESS_ROOT/.claude/agents"
  "$HARNESS_ROOT/.codex/agents"
  "$HARNESS_ROOT/.cursor/agents"
  "$HARNESS_ROOT/.agents/agents"
  "$HARNESS_ROOT/.kiro/agents"
  "$HARNESS_ROOT/.windsurf/agents"
)

CHECK=0
[ "${1:-}" = "--check" ] && CHECK=1

log(){ printf '%s\n' "sync-agents: $*" >&2; }

short_name(){ local b; b="$(basename "$1")"; b="${b#docket-}"; printf '%s' "${b%.md}"; }

# --- config helpers ----------------------------------------------------------
# Print the single config line for <name> from <file>, optionally only within an `agents:` block.
# Captures each pipeline stage into a variable (never `producer | grep -q`) to stay SIGPIPE-safe.
entry_line() {  # $1=file  $2=name  $3=under_block(0|1)
  local file="$1" name="$2" under="$3" body stripped matched
  [ -f "$file" ] || return 0
  if [ "$under" = "1" ]; then
    body="$(awk '/^agents:[[:space:]]*$/{f=1;next} f&&/^[^[:space:]#]/{f=0} f{print}' "$file")"
  else
    body="$(cat "$file")"
  fi
  stripped="$(printf '%s\n' "$body" | sed 's/#.*//')"
  matched="$(printf '%s\n' "$stripped" | grep -E "^[[:space:]]*${name}[[:space:]]*:" || true)"
  printf '%s\n' "$matched" | head -n1
}

# Extract one field value (model/effort) from a config entry line. Empty if absent.
field_of() {  # $1=line  $2=field
  printf '%s' "$1" | sed -nE "s/.*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}

# Names listed under <file>'s `agents:` block, one per line.
block_names() {  # $1=file
  [ -f "$1" ] || return 0
  awk '
    /^agents:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+[A-Za-z0-9._-]+[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/^[[:space:]]+/,"",line); sub(/[[:space:]]*:.*/,"",line);
      if(line!="") print line
    }' "$1"
}

# Resolve an override for <name> from <file>/<under_block> into RES_MODEL / RES_EFFORT (empty = none).
resolve_from() {  # $1=file  $2=name  $3=under_block
  local line; line="$(entry_line "$1" "$2" "$3")"
  RES_MODEL="$(field_of "$line" model)"
  RES_EFFORT="$(field_of "$line" effort)"
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

# --- passes ------------------------------------------------------------------
user_level_pass() {  # built-in ⊕ global -> each present harness */agents dir
  local src dir name
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    resolve_from "$GLOBAL_CFG" "$name" 0
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      # Intentional divergence from link-skills.sh (which leaf-checks <harness>/skills): check the
      # harness ROOT and mkdir agents/, because agents/ is docket-introduced and won't pre-exist
      # even on a harness you use. So: write into every PRESENT harness root, creating agents/.
      [ -d "$(dirname "$dir")" ] || continue
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"
    done
  done
}

project_level_pass() {  # built-in ⊕ per-repo -> <repo>/.claude/agents (committed)
  [ -f "$DOCKET_YML" ] || return 0
  local names name src
  names="$(block_names "$DOCKET_YML")"
  [ -n "$names" ] || return 0
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    src="$AGENTS_SRC/docket-$name.md"
    if [ ! -f "$src" ]; then
      log "skip '$name' — no built-in wrapper (advisory/interactive skills have no agent file)"
      continue
    fi
    resolve_from "$DOCKET_YML" "$name" 1
    mkdir -p "$PROJECT_AGENT_DIR"
    emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$PROJECT_AGENT_DIR/docket-$name.md"
  done <<EOF
$names
EOF
}

user_level_pass
project_level_pass
log "done"
