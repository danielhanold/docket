#!/usr/bin/env bash
# sync-agents.sh — generate docket's model/effort-pinned subagent wrappers into each PRESENT
# agent-harness dir, resolving FOUR-LAYER config (built-in ⊕ global ⊕ per-repo committed
# ⊕ per-repo machine-local).
#
# Unlike link-skills.sh (which SYMLINKS skills/<name>), agent files bake resolved model/effort,
# so they are GENERATED COPIES this script owns and OVERWRITES on every run. Per-repo generated
# files are machine-local artifacts (intended to be gitignored, not committed). A managed
# .gitignore block is maintained by this script; a one-time migration untracks any 0048-era
# committed wrappers so regenerated copies stay machine-local.
#
# Layers & precedence, per FIELD (model/effort independently) — local > committed > global > built-in:
#   built-in   agents/docket-*.md in this repo (each ships its default model/effort)
#   global     ~/.config/docket/config.yml `agents:` block -> user-level ~/.claude/agents/docket-*.md
#              (the legacy ~/.config/docket/agents.yaml is auto-migrated into it, then renamed .migrated)
#   committed  <repo>/.docket.yml `agents:` block          -> project-level <repo>/.claude/agents/docket-*.md
#   local      <repo>/.docket.local.yml `agents:` block     -> project-level <repo>/.claude/agents/docket-*.md
#              (gitignored, machine-scoped; a missing/unreadable file is warned + skipped, never fatal)
# Per-repo generation is opt-in: either the LOCAL or the COMMITTED file declaring `agent_harnesses:`
# or an `agents:` block opts the repo in (key-level precedence — the first of local/committed that
# HAS the `agent_harnesses:` key wins the target-harness list outright, not a merge of the two).
# A global `agent_harnesses:` (config.yml top-level key) scopes the USER-LEVEL pass only —
# overriding presence-on-disk detection; it never opts a repo into per-repo generation.
# Claude Code applies project-over-user precedence natively, so the generator writes two passes
# (user = built-in⊕global, project = built-in⊕local⊕committed⊕global) and never hand-merges
# the user-level and project-level output onto the same file.
#
# Usage:
#   bash sync-agents.sh           # write user-level (built-in ⊕ global); and, if <repo>/.docket.yml
#                                 # or <repo>/.docket.local.yml opts in, project-level (all four layers).
#                                 # A one-time migration first untracks any 0048-era committed wrapper/
#                                 # rule files (change 0051 — they are machine-local now) and prints the
#                                 # single commit that finishes it; a managed .gitignore block is then
#                                 # written/refreshed so the regenerated local copies stay untracked.
#   bash sync-agents.sh --check   # CI gate, THREE legs (per repo):
#                                 #   (a) the committed .gitignore docket block is present and current
#                                 #       (a legacy docket:generated spelling upgrades on the next run)
#                                 #       — CI-meaningful, exit non-zero if missing/stale.
#                                 #   (b) no generated agent/rule file is TRACKED by git (0048-era
#                                 #       leftovers or a re-add) — CI-meaningful, exit non-zero if any
#                                 #       are tracked, naming them + the migration remedy.
#                                 #   (c) the machine-local files on THIS disk match what the resolved
#                                 #       config would generate — ADVISORY ONLY: reported with an
#                                 #       "advisory:" prefix, never changes the exit code (vacuous on a
#                                 #       fresh clone, where no local files exist yet).
#                                 # A legacy bare-agent-key agents: shape in the COMMITTED .docket.yml
#                                 # is also CI-meaningful (exit non-zero) since that file is CI-visible.
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME for harness dirs and the global-config root
# (the latter only when XDG_CONFIG_HOME is unset — a set XDG_CONFIG_HOME wins; tests unset it).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$SCRIPT_DIR/scripts/lib/docket-gitignore-block.sh"
AGENTS_SRC="$SCRIPT_DIR/agents"
CURSOR_RULES_SRC="$SCRIPT_DIR/cursor-rules"
REPO="$PWD"

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
GLOBAL_CFG_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
GLOBAL_CFG="$GLOBAL_CFG_DIR/config.yml"
LEGACY_GLOBAL_CFG="$GLOBAL_CFG_DIR/agents.yaml"
DOCKET_YML="$REPO/.docket.yml"
LOCAL_CFG="$REPO/.docket.local.yml"
# Malformed/unreadable local file: warn + skip (0050's malformed-global posture) — a broken
# machine-local file must never break the run; committed + global layers still apply.
if [ -e "$LOCAL_CFG" ] && { [ ! -f "$LOCAL_CFG" ] || [ ! -r "$LOCAL_CFG" ]; }; then
  printf '%s\n' "sync-agents: WARN $LOCAL_CFG is not a readable regular file — machine-local layer ignored" >&2
  LOCAL_CFG=/dev/null
fi

# Harness agent dirs, derived from the lib's canonical roster (single source of truth).
HARNESS_AGENT_DIRS=()
for _tok in $DOCKET_GI_HARNESS_TOKENS; do HARNESS_AGENT_DIRS+=("$HARNESS_ROOT/.$_tok/agents"); done
unset _tok

VALID_HARNESS_TOKENS="$DOCKET_GI_HARNESS_TOKENS"

CHECK=0
[ "${1:-}" = "--check" ] && CHECK=1

log(){ printf '%s\n' "sync-agents: $*" >&2; }

# --- agents.yaml -> config.yml auto-migration (change 0050) -------------------
# Idempotent: (1) live agents.yaml + config.yml WITHOUT an agents: block -> rewrite the old
# top-level harness-first map under agents: in config.yml (creating the file if needed),
# rename the original to .migrated (git-less users keep a copy), log loudly. (2) config.yml
# already has agents: and a live agents.yaml is also present -> warn stale, do not read it.
# After this change the global agent config is read ONLY from config.yml (no dual-read).
migrate_legacy_global(){
  [ -f "$LEGACY_GLOBAL_CFG" ] || return 0
  if [ -f "$GLOBAL_CFG" ] && grep -qE '^agents[[:space:]]*:' "$GLOBAL_CFG"; then
    log "WARN $LEGACY_GLOBAL_CFG is STALE and unread — global agent config lives under agents: in $GLOBAL_CFG; delete or rename the old file"
    return 0
  fi
  # A pre-existing config.yml without a trailing newline would glue agents: onto its last line.
  if [ -s "$GLOBAL_CFG" ] && [ -n "$(tail -c1 "$GLOBAL_CFG")" ]; then printf '\n' >> "$GLOBAL_CFG"; fi
  {
    printf 'agents:\n'
    sed 's/^\(.\)/  \1/' "$LEGACY_GLOBAL_CFG"    # indent every non-empty line under agents:
  } >> "$GLOBAL_CFG"
  mv "$LEGACY_GLOBAL_CFG" "$LEGACY_GLOBAL_CFG.migrated"
  log "MIGRATED global agent config: $LEGACY_GLOBAL_CFG -> agents: block in $GLOBAL_CFG (original kept at $LEGACY_GLOBAL_CFG.migrated)"
}

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
  local f
  for f in "$LOCAL_CFG" "$DOCKET_YML"; do
    [ -f "$f" ] || continue
    if grep -qE '^agent_harnesses[[:space:]]*:' "$f"; then
      raw="$(sed -n -E 's/^agent_harnesses[[:space:]]*:[[:space:]]*([^#]*).*/\1/p' "$f")"
      raw="$(head -n1 <<<"$raw" | sed -E 's/[[:space:]]+$//')"
      break
    fi
  done
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

# Resolve the GLOBAL agent_harnesses (config.yml top-level key) — change 0050. Scope: it
# overrides the user-level pass's presence-on-disk selector ONLY; the per-repo committed
# pass is governed solely by the repo's own agent_harnesses (a global value shaping
# committed files would fail --check on every other machine). Unset => USER_HARNESSES_SET=0
# (presence detection); set (even to []) => the list governs.
resolve_global_agent_harnesses(){
  local raw list tok
  USER_HARNESSES_SET=0; USER_HARNESSES=""
  raw=""
  if [ -f "$GLOBAL_CFG" ]; then
    raw="$(sed -n -E 's/^agent_harnesses[[:space:]]*:[[:space:]]*([^#]*).*/\1/p' "$GLOBAL_CFG")"
    raw="$(head -n1 <<<"$raw" | sed -E 's/[[:space:]]+$//')"
  fi
  [ -n "$raw" ] || return 0
  USER_HARNESSES_SET=1
  list="${raw#[}"; list="${list%]}"; list="${list//,/ }"
  set -f
  for tok in $list; do
    if is_valid_harness "$tok"; then
      USER_HARNESSES="$USER_HARNESSES $tok"
    else
      log "unknown agent_harnesses token '$tok' in $GLOBAL_CFG — ignored"
    fi
  done
  set +f
  USER_HARNESSES="$(echo $USER_HARNESSES)"
}

# The user-level pass's final harness token list: the global agent_harnesses when set
# (extends: absent dirs are created; narrows: unlisted present dirs are skipped), else
# every harness root present on disk. Space-separated string (bash-3.2-safe under set -u).
compute_user_targets(){
  local dir
  if [ "$USER_HARNESSES_SET" = "1" ]; then
    USER_TARGETS="$USER_HARNESSES"
  else
    USER_TARGETS=""
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      if [ -d "$(dirname "$dir")" ]; then
        USER_TARGETS="$USER_TARGETS $(harness_of_dir "$dir")"
      fi
    done
    USER_TARGETS="$(echo $USER_TARGETS)"
  fi
  return 0
}

# Per-repo generation is OPT-IN: a repo opts in by declaring an `agents:` override block OR an
# explicit top-level `agent_harnesses:` key in EITHER .docket.local.yml or .docket.yml (checked in
# that order — a machine can opt a tracking-only repo in locally without touching committed config).
# A repo with neither file declaring either key gets NO per-repo wrappers — preserving pre-0048
# behavior for tracking-only repos (no surprise files from `sync-agents.sh`, and `--check` stays a no-op).
per_repo_opted_in() {
  local f
  for f in "$LOCAL_CFG" "$DOCKET_YML"; do
    [ -f "$f" ] || continue
    grep -qE '^agent_harnesses[[:space:]]*:' "$f" && return 0
    grep -qE '^agents[[:space:]]*:' "$f" && return 0
  done
  return 1
}

short_name(){ local b; b="$(basename "$1")"; b="${b#docket-}"; printf '%s' "${b%.md}"; }

# Extract the single-line `description:` frontmatter value from a wrapper source file.
agent_description(){ sed -n '/^description:/{s/^description:[[:space:]]*//;p;q;}' "$1"; }

# Harnesses that get a generated Cursor-style dispatch rule. Both Cursor and Claude Code exhibit
# the inline quirk (a directly-invoked skill runs at the session model, defeating the wrapper's
# model/effort pin), but they fix it differently: Cursor needs this generated alwaysApply dispatch
# rule, while Claude Code uses native per-skill `context: fork` frontmatter (see skills/docket-*/
# SKILL.md). So only Cursor belongs in this list.
HARNESS_HAS_DISPATCH_RULES="$DOCKET_GI_DISPATCH_HARNESSES"
harness_has_dispatch_rule(){ case " $HARNESS_HAS_DISPATCH_RULES " in *" $1 "*) return 0;; *) return 1;; esac; }

# Codex reads a committed AGENTS.md; only codex gets the AGENTS.md dispatch block (change 0077).
AGENTS_MD_DISPATCH_HARNESSES="codex"
DISPATCH_START='<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->'
DISPATCH_END='<!-- docket:dispatch:end -->'

# --- config helpers ----------------------------------------------------------
# Print the body nested under the first bare `<key>:` header from stdin, DEDENTED to column 0
# at the block's base indent (so a nested doc's harness keys land at column 0 regardless of the
# parent's indentation). Body = lines strictly more-indented than the header, up to the next line
# at the header's indent-or-less. Values are printed raw (comment-stripping is the caller's job).
section_body() {  # $1=key ; reads stdin
  awk -v key="$1" '
    function ind(s,   m){ m=match(s, /[^[:space:]]/); return (m==0 ? length(s) : m-1) }
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
  hbody="$(printf '%s\n' "$sub" | section_body "$2" || true)"                # body under <harness>/<default>
  stripped="$(printf '%s\n' "$hbody" | sed 's/#.*//')"
  matched="$(printf '%s\n' "$stripped" | grep -E "^[[:space:]]*$3[[:space:]]*:" || true)"
  head -n1 <<<"$matched"
}

# Resolve (harness, agent) per-field across the given layer files, highest precedence
# first (each read under a top-level agents: wrapper). Within a layer the harness line
# beats the default line; across layers the first layer supplying a field wins; the
# built-in floor is handled by emit(). RES_MODEL_FROM_HARNESS=1 iff the model came from
# a harness-specific line in ANY layer (drives warn_fallback_model).
resolve_agent_layers() {  # $1=harness  $2=agent  $3..=layer files (precedence order)
  local harness="$1" agent="$2" f hline dline hm he dm de
  shift 2
  RES_MODEL=""; RES_EFFORT=""; RES_MODEL_FROM_HARNESS=0
  for f in "$@"; do
    hline="$(harness_agent_line "$f" "$harness" "$agent" 1)"
    dline="$(harness_agent_line "$f" default "$agent" 1)"
    hm="$(field_of "$hline" model)";  he="$(field_of "$hline" effort)"
    dm="$(field_of "$dline" model)";  de="$(field_of "$dline" effort)"
    if [ -z "$RES_MODEL" ]; then
      if   [ -n "$hm" ]; then RES_MODEL="$hm"; RES_MODEL_FROM_HARNESS=1
      elif [ -n "$dm" ]; then RES_MODEL="$dm"; fi
    fi
    if [ -z "$RES_EFFORT" ]; then
      if   [ -n "$he" ]; then RES_EFFORT="$he"
      elif [ -n "$de" ]; then RES_EFFORT="$de"; fi
    fi
    if [ -n "$RES_MODEL" ] && [ -n "$RES_EFFORT" ]; then break; fi
  done
  return 0
}

# Union (sorted-unique) of agent keys configured under any harness sub-block or `default`.
agent_keys() {  # $1=file  $2=under_agents(0|1)
  local sub
  [ -f "$1" ] || return 0
  if [ "$2" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  printf '%s\n' "$sub" | awk '
    function ind(s,   m){ m=match(s,/[^[:space:]]/); return (m==0?length(s):m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ { basei=ind(nc); inb=1; next }   # a harness/default header (col 0, bare)
    inb && nc ~ /[^[:space:]]/ && ind(nc) <= basei { inb=0 }
    inb && nc ~ /^[[:space:]]+[A-Za-z0-9._-]+[[:space:]]*:/ {
      k=nc; sub(/^[[:space:]]+/,"",k); sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }' | sort -u
}

# Pre-0046 flat shape: bare agent keys sitting DIRECTLY under agents: (or top level for global),
# i.e. neither `default` nor a known harness. One per line. Used to warn + drop + flag as --check drift.
legacy_agent_keys() {  # $1=file  $2=under_agents(0|1)
  local sub
  [ -f "$1" ] || return 0
  if [ "$2" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
  printf '%s\n' "$sub" | awk '
    { nc=$0; sub(/#.*/,"",nc) }
    /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*\{/ {                  # col-0 key WITH an inline {…} value == a bare agent entry
      k=nc; sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }'
}

# Harness/default header names present under agents: (the top-level keys of the harness map).
agents_block_harnesses() {  # $1=file  (docket.yml, under_agents=1)
  local sub
  [ -f "$1" ] || return 0
  sub="$(section_body agents < "$1")"
  printf '%s\n' "$sub" | awk '{ nc=$0; sub(/#.*/,"",nc) } /^[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ { k=nc; sub(/[[:space:]]*:.*/,"",k); if(k!="") print k }'
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

# --- per-harness emitter registry (change 0077) ------------------------------
# Map a harness token to the on-disk extension docket generates for it.
harness_ext(){ case "$1" in codex) printf 'toml';; *) printf 'md';; esac; }

# Dispatch to the harness-appropriate emitter. MODEL/EFFORT are resolved OVERRIDES
# (empty => keep the built-in), identical in meaning to emit()'s args.
emit_for_harness(){  # $1=src md  $2=harness  $3=model  $4=effort
  case "$2" in
    codex) emit_codex_toml "$1" "$3" "$4";;
    *)     emit "$1" "$3" "$4";;
  esac
}

# Escape a value for a TOML basic (double-quoted) string: backslash then double-quote.
toml_escape_basic(){ printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }

# Transform a built-in markdown wrapper into a Codex TOML agent document on stdout.
# Field mapping (ADR-0015 verbatim passthrough for model/effort):
#   frontmatter name:        -> name
#   frontmatter description: -> description
#   effective model          -> model                  (override||built-in; omit if empty/inherit)
#   effective effort         -> model_reasoning_effort  (override||built-in; omit if empty/auto)
#   skills: preload + body   -> developer_instructions  (multi-line basic string)
emit_codex_toml(){  # $1=src md  $2=model_override  $3=effort_override
  local src="$1" mo="$2" eo="$3"
  local name desc bi_model bi_effort model effort skills_csv body dev esc
  name="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$name" ] || name="docket-$(short_name "$src")"
  desc="$(agent_description "$src")"
  bi_model="$(sed -n '/^model:/{s/^model:[[:space:]]*//;p;q;}' "$src")"
  bi_effort="$(sed -n '/^effort:/{s/^effort:[[:space:]]*//;p;q;}' "$src")"
  model="${mo:-$bi_model}"
  effort="${eo:-$bi_effort}"
  skills_csv="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  body="$(awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$src" | awk 'NF{p=1} p{print}')"
  # developer_instructions text: skills-preload preamble (if any) + the wrapper body.
  if [ -n "$skills_csv" ]; then
    dev="Before acting, load these docket skills from your linked Codex skills directory: ${skills_csv}.

${body}"
  else
    dev="$body"
  fi
  # Emit TOML.
  printf 'name = "%s"\n' "$(toml_escape_basic "$name")"
  printf 'description = "%s"\n' "$(toml_escape_basic "$desc")"
  if [ -n "$model" ] && [ "$model" != "inherit" ]; then
    printf 'model = "%s"\n' "$(toml_escape_basic "$model")"
  fi
  if [ -n "$effort" ] && [ "$effort" != "auto" ]; then
    printf 'model_reasoning_effort = "%s"\n' "$(toml_escape_basic "$effort")"
  fi
  # Multi-line basic string. Escape backslashes; defend against a literal """ terminator
  # (built-in bodies have neither, but keep the emitter robust). Closing """ on its own line.
  esc="$(printf '%s' "$dev" | sed -e 's/\\/\\\\/g' -e 's/"""/""\\"/g')"
  printf 'developer_instructions = """\n%s\n"""\n' "$esc"
}

# Assemble the Cursor dispatch rule to stdout: static head + one subsection per built-in agent
# (glob order). A built-in agent with a fragment uses it verbatim; one without gets a minimal
# auto-block derived from its description + a warning (a new agent is never silently un-dispatched).
assemble_dispatch_rule() {
  cat "$CURSOR_RULES_SRC/dispatch.head.md"
  local src name frag desc
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    frag="$CURSOR_RULES_SRC/dispatch/docket-$name.md"
    printf '\n'
    if [ -f "$frag" ]; then
      cat "$frag"
    else
      desc="$(agent_description "$src")"
      printf '## docket-%s — dispatch only\n\n' "$name"
      printf '%s\n\n' "$desc"
      printf 'When this applies, do NOT run the skill inline. Launch a Task with `subagent_type: "docket-%s"`, `run_in_background: false`, and relay its result.\n' "$name"
      log "WARN no dispatch fragment for docket-$name — emitted a minimal auto-block; add cursor-rules/dispatch/docket-$name.md"
    fi
  done
}

# Write the dispatch rule into a harness root's rules/ dir (<root>/.<harness>/rules/docket-dispatch.mdc).
write_dispatch_rule() {  # $1 = <root>/.<harness> base path
  mkdir -p "$1/rules"
  assemble_dispatch_rule > "$1/rules/docket-dispatch.mdc"
}

# Assemble the committed AGENTS.md docket dispatch block (markers included) to stdout.
# Machine-neutral: agent names + delegation prose only, NO model IDs (pins live in the .toml).
assemble_agents_md_dispatch(){
  printf '%s\n' "$DISPATCH_START"
  cat <<'HEAD'
## Docket agents — dispatch, don't run inline

Docket ships model/effort-pinned agent definitions in `.codex/agents/docket-*.toml`. When you are
asked to run one of the docket skills below, run the matching **agent** (its pinned model and
reasoning effort are the whole point) instead of executing the skill inline at the session model.
Pass the request through unchanged, including any change or ADR id.
HEAD
  printf '\n'
  local src name desc
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    desc="$(agent_description "$src")"
    printf -- '- **docket-%s** — %s Delegate to the `docket-%s` agent.\n' "$name" "$desc" "$name"
  done
  printf '%s\n' "$DISPATCH_END"
}

# Write the AGENTS.md dispatch block when codex is a targeted per-repo harness; strip it when
# codex is de-listed (within an opted-in repo). Logs a one-time commit notice on write/remove.
sync_codex_agents_md_dispatch(){
  local f="$REPO/AGENTS.md" status
  case " $HARNESSES " in
    *" codex "*)
      status="$(ensure_managed_block "$f" "$DISPATCH_START" "$DISPATCH_END" "$(assemble_agents_md_dispatch)")"
      case "$status" in
        wrote)   log "wrote/updated the docket dispatch block in $f — COMMIT THIS (machine-neutral; no model IDs).";;
        refused) log "WARN $f has a malformed docket:dispatch block — refusing to rewrite; repair the markers by hand and re-run.";;
      esac
      ;;
    *)
      status="$(remove_managed_block "$f" "$DISPATCH_START" "$DISPATCH_END")"
      case "$status" in
        removed) log "removed the docket dispatch block from $f (codex de-listed) — COMMIT THIS.";;
        refused) log "WARN $f has a malformed docket:dispatch block — refusing to strip; repair the markers by hand.";;
      esac
      ;;
  esac
}

# --- managed .gitignore block (change 0051; mechanics moved into scripts/lib/docket-gitignore-block.sh
# in change 0057, which sync-agents.sh sources — that lib is the single home for ALL docket-owned
# ignores and is shared by all three writers: migrate-to-docket.sh, docket-config.sh --bootstrap, and
# this script). Trigger policy stays HERE — sync-agents.sh decides WHEN the block is wanted; the lib
# only knows HOW to emit/ensure it.
GITIGNORE="$REPO/.gitignore"

# The block is maintained for opted-in repos, any repo carrying a .docket.local.yml, any repo
# with a docket branch (the bootstrap guard's DOCKET probe — an explicit repo-level signal,
# LEARNINGS #48), or any repo already carrying the block (heal-if-present, either spelling).
gitignore_block_wanted(){
  per_repo_opted_in && return 0
  [ -e "$REPO/.docket.local.yml" ] && return 0
  git -C "$REPO" rev-parse --verify --quiet refs/remotes/origin/docket >/dev/null 2>&1 && return 0
  git -C "$REPO" rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1 && return 0
  [ -f "$GITIGNORE" ] && grep -F -x -q -- "$DOCKET_GI_START" "$GITIGNORE" && return 0
  [ -f "$GITIGNORE" ] && grep -F -x -q -- "$DOCKET_GI_LEGACY_START" "$GITIGNORE" && return 0
  return 1
}

# --- 0048-era migration: generated files must not be tracked (change 0051) ----
tracked_docket_files() {  # tracked generated agent/rule paths, one per line (empty outside git)
  git -C "$REPO" rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 0
  local tok
  {
    for tok in $VALID_HARNESS_TOKENS; do
      git -C "$REPO" ls-files -- ".$tok/agents/docket-*.$(harness_ext "$tok")" 2>/dev/null
    done
    for tok in $HARNESS_HAS_DISPATCH_RULES; do
      git -C "$REPO" ls-files -- ".$tok/rules/docket-dispatch.mdc" 2>/dev/null
    done
  } | sort -u
}

migrate_tracked_wrappers() {  # one-time: untrack 0048-era committed wrappers; idempotent
  local tracked f cmd
  tracked="$(tracked_docket_files)"
  [ -n "$tracked" ] || return 0
  log "MIGRATING (change 0051): generated agent files are machine-local now and must not be tracked"
  while IFS= read -r f; do rm -f "$REPO/$f"; done <<<"$tracked"
  log "deleted the tracked copies from the working tree (regenerated locally below); complete with ONE commit:"
  cmd="git rm -r --cached $(tr '\n' ' ' <<<"$tracked")"
  # only tell them to `git add .gitignore` when this run actually wrote/refreshed the block
  # (gitignore_block_wanted() below); otherwise there may be no .gitignore to add, and the
  # printed remedy would fail at that clause (pathspec error) leaving the rm --cached
  # staged but uncommitted.
  if gitignore_block_wanted; then
    cmd="${cmd}&& git add .gitignore "
  fi
  cmd="${cmd}&& git commit -m 'docket: generated agent files go machine-local (change 0051)'"
  log "  $cmd"
}

# Non-fatal footgun warning: when generating a NON-claude harness file whose `model` resolved from
# default/built-in (no agents.<harness> override supplied it), the ID is likely wrong for that
# harness (ADR-0015: some harnesses silently run their house default on an unknown model). Never
# an error; sync still succeeds. Scoped to non-claude — the claude built-ins/default ARE Claude IDs.
warn_fallback_model(){  # $1=harness $2=agent ; consumes RES_MODEL_FROM_HARNESS / RES_MODEL
  [ "$1" = "claude" ] && return 0
  [ "$RES_MODEL_FROM_HARNESS" = "1" ] && return 0
  log "WARN $1/docket-$2: model '${RES_MODEL:-<built-in>}' came from default/built-in; may not be a valid model ID for harness '$1'."
}

warn_legacy_shape(){  # $1=file $2=under_agents ; warns once per bare agent key
  local k
  while IFS= read -r k; do
    [ -n "$k" ] || continue
    log "WARN legacy agents: shape — bare agent key '$k' is neither 'default' nor a known harness; ignored (use agents.default.$k or agents.<harness>.$k)."
  done < <(legacy_agent_keys "$1" "$2")
}

# --- passes ------------------------------------------------------------------
# Map a user-level harness *dir* ("$HARNESS_ROOT/.cursor/agents") to its token ("cursor").
harness_of_dir(){ local b; b="$(basename "$(dirname "$1")")"; printf '%s' "${b#.}"; }

user_level_pass() {  # built-in ⊕ global -> each user-level target harness, resolved per (harness, agent)
  local src dir name harness
  warn_legacy_shape "$GLOBAL_CFG" 1
  compute_user_targets
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $USER_TARGETS; do
      dir="$HARNESS_ROOT/.$harness/agents"
      resolve_agent_layers "$harness" "$name" "$GLOBAL_CFG"
      warn_fallback_model "$harness" "$name"
      mkdir -p "$dir"
      emit_for_harness "$src" "$harness" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.$(harness_ext "$harness")"
    done
  done
  # Cursor-only dispatch rule, user-level, for each targeted dispatch-rule harness.
  local drh
  for drh in $HARNESS_HAS_DISPATCH_RULES; do
    case " $USER_TARGETS " in *" $drh "*) write_dispatch_rule "$HARNESS_ROOT/.$drh" ;; esac
  done
}

project_level_pass() {  # built-in ⊕ local ⊕ committed ⊕ global -> <repo>/.<H>/agents for each H in HARNESSES
  per_repo_opted_in || return 0
  local src name harness dir cfg_h cfgname layer_f
  for layer_f in "$LOCAL_CFG" "$DOCKET_YML"; do
    warn_legacy_shape "$layer_f" 1
  done
  # Warn on any agents.<harness> block whose harness is NOT in agent_harnesses (dead config).
  for layer_f in "$LOCAL_CFG" "$DOCKET_YML"; do
    while IFS= read -r cfg_h; do
      [ -n "$cfg_h" ] || continue
      [ "$cfg_h" = "default" ] && continue
      case " $HARNESSES " in *" $cfg_h "*) : ;; *) log "WARN agents.$cfg_h: block is not in agent_harnesses — ignored (dead config)." ;; esac
    done < <(agents_block_harnesses "$layer_f")
  done
  # Typo guard: an agents: entry that overrides no real built-in is a no-op — warn (do not fail).
  for layer_f in "$LOCAL_CFG" "$DOCKET_YML"; do
    while IFS= read -r cfgname; do
      [ -n "$cfgname" ] || continue
      [ -f "$AGENTS_SRC/docket-$cfgname.md" ] || log "WARN agents: '$cfgname' overrides no built-in agent (no agents/docket-$cfgname.md) — ignored (typo? advisory/interactive skills have no wrapper)."
    done < <(agent_keys "$layer_f" 1)
  done
  # Always generate the FULL built-in set (config is override-only) into each listed harness.
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      warn_fallback_model "$harness" "$name"
      dir="$REPO/.$harness/agents"
      mkdir -p "$dir"
      emit_for_harness "$src" "$harness" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.$(harness_ext "$harness")"
    done
  done
  # Cursor-only dispatch rule, per-repo (committed) when cursor is a targeted harness.
  local h
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    write_dispatch_rule "$REPO/.$h"
  done
  # Codex-only committed AGENTS.md dispatch block (change 0077).
  sync_codex_agents_md_dispatch
}

check_project_level() {  # three legs: (a) gitignore block current [CI-meaningful], (b) nothing
                          # tracked [CI-meaningful], (c) local content staleness [advisory only]
  local rc=0 tracked legacy
  # leg (b) — migration enforcement runs even without opt-in (stale 0048 leftovers).
  tracked="$(tracked_docket_files)"
  if [ -n "$tracked" ]; then
    log "check: TRACKED generated agent files (machine-local since change 0051) — run: bash sync-agents.sh, then make the printed migration commit:"
    printf '%s\n' "$tracked" >&2
    rc=1
  fi
  if ! gitignore_block_wanted; then
    log "no per-repo agent opt-in (agents:/agent_harnesses) and no .docket.local.yml in $REPO — nothing else to check"
    return $rc
  fi
  # leg (a) — the .gitignore block is present and current, evaluated against the NEW markers.
  if [ "$(emit_docket_gitignore_block)" != "$(_docket_gi_current_block "$GITIGNORE" "$DOCKET_GI_START" "$DOCKET_GI_END")" ]; then
    log "check: .gitignore docket block missing or stale (a legacy docket:generated block upgrades on the next run) — run: bash sync-agents.sh and commit .gitignore"
    rc=1
  fi
  # AGENTS.md dispatch block currency (change 0077) — CI-meaningful, symmetric with the
  # .gitignore leg. The block is committed (exempt from the tracked-file leg); assert it
  # is present & current when codex is targeted, and absent when codex is not.
  local am_want am_have
  am_want="$(assemble_agents_md_dispatch)"
  am_have="$(_docket_gi_current_block "$REPO/AGENTS.md" "$DISPATCH_START" "$DISPATCH_END")"
  case " $HARNESSES " in
    *" codex "*)
      if [ "$am_want" != "$am_have" ]; then
        log "check: AGENTS.md docket dispatch block missing or stale — run: bash sync-agents.sh and commit AGENTS.md"
        rc=1
      fi
      ;;
    *)
      if [ -n "$am_have" ]; then
        log "check: AGENTS.md carries a docket dispatch block but codex is not in agent_harnesses — run: bash sync-agents.sh and commit AGENTS.md"
        rc=1
      fi
      ;;
  esac
  # committed-config shape (the committed .docket.yml is CI-visible): legacy bare agent keys.
  legacy="$(legacy_agent_keys "$DOCKET_YML" 1)"
  if [ -n "$legacy" ]; then
    log "check: legacy bare-agent-key agents: shape ($(printf '%s' "$legacy" | tr '\n' ' ')) — reshape to agents.default.<agent> (run: bash sync-agents.sh)"
    rc=1
  fi
  # leg (c) — local staleness (ADVISORY: reported, never fails CI; vacuous on a fresh clone).
  local src name got tmp d harness
  tmp="$(mktemp -d)"
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      local ext; ext="$(harness_ext "$harness")"
      emit_for_harness "$src" "$harness" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.$ext"
      got="$REPO/.$harness/agents/docket-$name.$ext"
      if [ ! -f "$got" ]; then
        log "advisory: .$harness/agents/docket-$name.$ext not generated on this machine (run: bash sync-agents.sh)"; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.$ext" || true)"
      if [ -n "$d" ]; then log "advisory: drift in .$harness/agents/docket-$name.$ext:"; printf '%s\n' "$d" >&2; fi
    done
  done
  rm -rf "$tmp"
  local h rule_got rule_tmp rd
  rule_tmp="$(mktemp)"
  assemble_dispatch_rule > "$rule_tmp"
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    rule_got="$REPO/.$h/rules/docket-dispatch.mdc"
    if [ ! -f "$rule_got" ]; then
      log "advisory: .$h/rules/docket-dispatch.mdc not generated on this machine (run: bash sync-agents.sh)"; continue
    fi
    rd="$(diff -u "$rule_got" "$rule_tmp" || true)"
    if [ -n "$rd" ]; then log "advisory: drift in .$h/rules/docket-dispatch.mdc:"; printf '%s\n' "$rd" >&2; fi
  done
  rm -f "$rule_tmp"
  ORPHAN_DRIFT=0
  prune_orphans per-repo          # handle_orphan logs advisory only; ORPHAN_DRIFT unused for rc
  return $rc
}

# Handle one orphaned docket-owned file: report it as an advisory under --check, else rm it.
handle_orphan() {  # $1 = path ; sets ORPHAN_DRIFT=1 under --check (advisory only, never fails CI)
  if [ "$CHECK" = "1" ]; then
    log "advisory: orphaned docket-owned file $1 (run: bash sync-agents.sh)"
    ORPHAN_DRIFT=1
  else
    rm -f "$1"
  fi
}

# rmdir a dir ONLY if docket emptied it this run (never a pre-existing empty/user dir). Delete mode only.
rmdir_if_docket_emptied() {  # $1 = dir
  [ "$CHECK" = "1" ] && return 0
  [ -d "$1" ] || return 0
  rmdir "$1" 2>/dev/null || true
}

# Prune orphaned docket-owned files. Scope:
#   scope=all      normal run — per-repo (HARNESSES) + user-level (present harnesses) removed-builtins,
#                  plus per-repo de-listed-harness cleanup, plus (when the global agent_harnesses
#                  list is set) user-level de-listed-harness cleanup.
#   scope=per-repo --check — per-repo only, report-only.
prune_orphans() {  # $1 = scope (all|per-repo)
  local scope="$1" dir f name tok pruned_agents pruned_rule
  local -a scan_dirs=()
  # (1a) per-repo removed-builtin dirs — only for a repo that opted into per-repo generation.
  if per_repo_opted_in; then
    for tok in $HARNESSES; do scan_dirs+=("$REPO/.$tok/agents"); done
  fi
  # (1b) user-level removed-builtin dirs — every present harness (normal run only).
  if [ "$scope" = "all" ]; then
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      [ -d "$(dirname "$dir")" ] && scan_dirs+=("$dir")
    done
  fi
  if [ ${#scan_dirs[@]} -gt 0 ]; then
    for dir in "${scan_dirs[@]}"; do
      [ -d "$dir" ] || continue
      local dtok dext
      dtok="$(harness_of_dir "$dir")"; dext="$(harness_ext "$dtok")"
      for f in "$dir"/docket-*."$dext"; do
        [ -e "$f" ] || continue
        name="$(basename "$f")"; name="${name#docket-}"; name="${name%.*}"
        [ -f "$AGENTS_SRC/docket-$name.md" ] || handle_orphan "$f"
      done
    done
  fi
  # (2) de-listed per-repo harness — only for an opted-in repo. A known harness NOT in HARNESSES that
  # still holds docket-owned per-repo files (agents + dispatch rule) is pruned; only the specific
  # dirs docket actually emptied are rmdir'd (never a pre-existing / user dir).
  if per_repo_opted_in; then
    for tok in $VALID_HARNESS_TOKENS; do
      case " $HARNESSES " in *" $tok "*) continue;; esac      # still listed -> not de-listed
      pruned_agents=0; pruned_rule=0
      for f in "$REPO/.$tok/agents"/docket-*."$(harness_ext "$tok")"; do
        [ -e "$f" ] || continue
        handle_orphan "$f"; pruned_agents=1
      done
      if [ -e "$REPO/.$tok/rules/docket-dispatch.mdc" ]; then
        handle_orphan "$REPO/.$tok/rules/docket-dispatch.mdc"; pruned_rule=1
      fi
      if [ "$pruned_agents" = "1" ]; then rmdir_if_docket_emptied "$REPO/.$tok/agents"; fi
      if [ "$pruned_rule" = "1" ]; then rmdir_if_docket_emptied "$REPO/.$tok/rules"; fi
      if [ "$pruned_agents" = "1" ] || [ "$pruned_rule" = "1" ]; then rmdir_if_docket_emptied "$REPO/.$tok"; fi
    done
  fi
  # (3) de-listed USER-LEVEL harness (change 0050): when the global agent_harnesses list is
  # SET, a known harness NOT in the user-level target list that still holds user-level
  # docket-owned files is pruned (mirrors the per-repo de-list rule — the files are
  # docket-owned generated copies). Never rmdir the harness root itself: it is the user's
  # own config dir, not a docket artifact. Delete-mode only concerns aside, --check never
  # reaches here (scope=per-repo returns above).
  [ "$scope" = "all" ] || return 0
  [ "${USER_HARNESSES_SET:-0}" = "1" ] || return 0
  for tok in $VALID_HARNESS_TOKENS; do
    case " ${USER_TARGETS:-} " in *" $tok "*) continue;; esac
    pruned_agents=0; pruned_rule=0
    for f in "$HARNESS_ROOT/.$tok/agents"/docket-*."$(harness_ext "$tok")"; do
      [ -e "$f" ] || continue
      handle_orphan "$f"; pruned_agents=1
    done
    if [ -e "$HARNESS_ROOT/.$tok/rules/docket-dispatch.mdc" ]; then
      handle_orphan "$HARNESS_ROOT/.$tok/rules/docket-dispatch.mdc"; pruned_rule=1
    fi
    if [ "$pruned_agents" = "1" ]; then rmdir_if_docket_emptied "$HARNESS_ROOT/.$tok/agents"; fi
    if [ "$pruned_rule" = "1" ]; then rmdir_if_docket_emptied "$HARNESS_ROOT/.$tok/rules"; fi
  done
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  resolve_agent_harnesses

  if [ "$CHECK" = "1" ]; then
    if check_project_level; then exit 0; else exit 1; fi
  fi

  migrate_legacy_global
  resolve_global_agent_harnesses
  user_level_pass
  migrate_tracked_wrappers
  if gitignore_block_wanted; then ensure_docket_gitignore_block "$REPO"; fi
  project_level_pass
  prune_orphans all
  log "done"
fi
