#!/usr/bin/env bash
# sync-agents.sh — generate docket's model/effort-pinned subagent wrappers into each PRESENT
# agent-harness dir, resolving FOUR-LAYER config (built-in ⊕ global ⊕ per-repo committed
# ⊕ per-repo machine-local).
#
# Unlike link-skills.sh (which SYMLINKS skills/<name>), agent files bake resolved model/effort,
# so they are GENERATED COPIES this script owns and OVERWRITES on every run. Per-repo generated
# files are machine-local artifacts (intended to be gitignored, not committed — the managed
# .gitignore block and migration of any pre-existing committed copies land in a following
# change; this change only wires the resolution + opt-in).
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
#                                 #   (a) the committed .gitignore docket:generated block is present
#                                 #       and current — CI-meaningful, exit non-zero if missing/stale.
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

# Harnesses that get a generated Cursor-style dispatch rule (only cursor exhibits the inline quirk).
HARNESS_HAS_DISPATCH_RULES="cursor"
harness_has_dispatch_rule(){ case " $HARNESS_HAS_DISPATCH_RULES " in *" $1 "*) return 0;; *) return 1;; esac; }

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

# --- managed .gitignore block (change 0051) -----------------------------------
# sync-agents.sh owns a marker-bounded block in <repo>/.gitignore covering every
# machine-local artifact it generates. Patterns are emitted from the SAME harness
# table generation uses (VALID_HARNESS_TOKENS / HARNESS_HAS_DISPATCH_RULES), so a
# new harness extends the block without a second roster. Nothing outside the
# markers is ever touched.
GITIGNORE="$REPO/.gitignore"
GI_START='# docket:generated:start (managed by sync-agents.sh — do not hand-edit)'
GI_END='# docket:generated:end'

emit_gitignore_block() {
  printf '%s\n' "$GI_START"
  printf '.docket.local.yml\n'
  local tok
  for tok in $VALID_HARNESS_TOKENS; do printf '.%s/agents/docket-*.md\n' "$tok"; done
  for tok in $HARNESS_HAS_DISPATCH_RULES; do printf '.%s/rules/docket-dispatch.mdc\n' "$tok"; done
  printf '%s\n' "$GI_END"
}

# The block is maintained for opted-in repos AND any repo carrying a .docket.local.yml
# (a tracking-only repo using it for skills:/finalize: must never risk committing it).
# NOTE: test the RAW path — LOCAL_CFG may have been redirected to /dev/null.
gitignore_block_wanted(){ per_repo_opted_in && return 0; [ -e "$REPO/.docket.local.yml" ]; }

current_gitignore_block() {
  [ -f "$GITIGNORE" ] || return 0
  awk -v s="$GI_START" -v e="$GI_END" '$0==s{f=1} f{print} $0==e{f=0}' "$GITIGNORE"
}

# True if $GITIGNORE contains a GI_START line with no matching GI_END line — a truncated/corrupt
# block whose true extent we cannot know. grep runs directly on the file path (no producer|grep -q
# pipeline), so this is safe under `set -o pipefail`; bash-3.2-safe.
gitignore_block_unterminated() {
  [ -f "$GITIGNORE" ] || return 1
  grep -F -x -q -- "$GI_START" "$GITIGNORE" || return 1
  grep -F -x -q -- "$GI_END" "$GITIGNORE" && return 1
  return 0
}

ensure_gitignore_block() {  # create/refresh; bytes outside the markers are never touched
  gitignore_block_wanted || return 0
  if gitignore_block_unterminated; then
    log "WARN $GITIGNORE has an UNTERMINATED docket:generated block (start marker present, end marker missing) — corrupt; refusing to rewrite so no bytes are lost. Repair or remove the dangling '$GI_START' line by hand, then re-run."
    return 0
  fi
  local want have rest
  want="$(emit_gitignore_block)"
  have="$(current_gitignore_block)"
  [ "$want" = "$have" ] && return 0
  rest=""
  if [ -f "$GITIGNORE" ]; then
    rest="$(awk -v s="$GI_START" -v e="$GI_END" '$0==s{f=1} !f{print} $0==e{f=0}' "$GITIGNORE")"
  fi
  {
    if [ -n "$rest" ]; then printf '%s\n\n' "$rest"; fi
    printf '%s\n' "$want"
  } > "$GITIGNORE"
  log "UPDATED $GITIGNORE managed block (docket:generated) — COMMIT THIS so machine-local generated files stay untracked"
}

# --- 0048-era migration: generated files must not be tracked (change 0051) ----
tracked_docket_files() {  # tracked generated agent/rule paths, one per line (empty outside git)
  git -C "$REPO" rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 0
  local tok
  {
    for tok in $VALID_HARNESS_TOKENS; do
      git -C "$REPO" ls-files -- ".$tok/agents/docket-*.md" 2>/dev/null
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
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/$(basename "$src")"
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
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.md"
    done
  done
  # Cursor-only dispatch rule, per-repo (committed) when cursor is a targeted harness.
  local h
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    write_dispatch_rule "$REPO/.$h"
  done
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
  # leg (a) — the committed .gitignore block is present and current (CI-meaningful).
  if [ "$(emit_gitignore_block)" != "$(current_gitignore_block)" ]; then
    log "check: .gitignore docket:generated block missing or stale — run: bash sync-agents.sh and commit .gitignore"
    rc=1
  fi
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
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"
      got="$REPO/.$harness/agents/docket-$name.md"
      if [ ! -f "$got" ]; then
        log "advisory: .$harness/agents/docket-$name.md not generated on this machine (run: bash sync-agents.sh)"; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"
      if [ -n "$d" ]; then log "advisory: drift in .$harness/agents/docket-$name.md:"; printf '%s\n' "$d" >&2; fi
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
      for f in "$dir"/docket-*.md; do
        [ -e "$f" ] || continue
        name="$(short_name "$f")"
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
      for f in "$REPO/.$tok/agents"/docket-*.md; do
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
    for f in "$HARNESS_ROOT/.$tok/agents"/docket-*.md; do
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

resolve_agent_harnesses

if [ "$CHECK" = "1" ]; then
  if check_project_level; then exit 0; else exit 1; fi
fi

migrate_legacy_global
resolve_global_agent_harnesses
user_level_pass
migrate_tracked_wrappers
ensure_gitignore_block
project_level_pass
prune_orphans all
log "done"
