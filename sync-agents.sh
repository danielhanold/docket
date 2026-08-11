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
#   built-in   agents/harness-defaults.yml in this repo (harness-indexed and sparse; a pair it does
#              not map resolves to nothing and is generated unpinned. agents/docket-*.md are the
#              behavior-only wrapper sources — name, description, skills, body; never model/effort)
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
# shellcheck source=/dev/null
. "$SCRIPT_DIR/scripts/lib/harness-defaults.sh"
HARNESS_DEFAULTS="$SCRIPT_DIR/agents/harness-defaults.yml"
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

# Registered runner names (change 0079) — a runner: value must name one of these; each token
# has a matching scripts/runners/<name>.sh adapter (tests assert parity in both directions).
REGISTERED_RUNNERS="codex cursor opencode"
is_registered_runner(){ case " $REGISTERED_RUNNERS " in *" $1 "*) return 0;; *) return 1;; esac; }

usage() {
  printf '%s\n' 'Usage: sync-agents.sh [--check]'
}

CHECK=0
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  case "$#:${1:-}" in
    0:) ;;
    1:--check) CHECK=1 ;;
    1:--help) usage; exit 0 ;;
    *)
      printf 'sync-agents: unknown argument: %s\n' "${1:-<empty>}" >&2
      usage >&2
      exit 2
      ;;
  esac
fi

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
  mv -f "$LEGACY_GLOBAL_CFG" "$LEGACY_GLOBAL_CFG.migrated"
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

# Whether THIS run generates per-repo wrappers into <repo>/.<harness>/agents.
#
# INVARIANT: every site that writes, gates, diffs, or prunes per-repo wrappers uses this predicate —
# no site keyed on that concept calls per_repo_opted_in directly. That is what makes the gate unable
# to see fewer triples than a call site later resolves (the under-enumeration change 0220 fixed),
# and what keeps prune from deleting on a boundary the writer does not share. No count and no list
# of sites is stated here on purpose: an enumeration would not notice the next site added.
#
# It is a delegation, not an alias: the sites move together, and naming the concept is what makes
# that reviewable. gitignore_block_wanted is deliberately NOT this predicate — it is
# strictly weaker (a .docket.local.yml, a docket branch, or a pre-existing block all satisfy it),
# and legs (a)/(b) keep it because they are about the .gitignore block and tracked leftovers, which
# exist independently of whether any wrapper was generated.
project_wrappers_generated() { per_repo_opted_in; }

short_name(){ local b; b="$(basename "$1")"; b="${b#docket-}"; printf '%s' "${b%.md}"; }

# Extract the single-line `description:` frontmatter value from a wrapper source file.
agent_description(){ sed -n '/^description:/{s/^description:[[:space:]]*//;p;q;}' "$1"; }

# Extract the single-line `worktree-scope:` frontmatter value from a wrapper source file (change
# 0208). An agent's worktree scope is a DECLARED FACT, not a name shape: the delegation gates —
# emit_shim's required --worktree slot below, and runner-dispatch.sh's runtime gate — both key on
# this declaration, so a future feature-scoped agent cannot ship ungated by not matching a pattern.
agent_worktree_scope(){ sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' "$1"; }

# Harnesses that get a generated Cursor-style dispatch rule. Both Cursor and Claude Code exhibit
# the inline quirk (a directly-invoked skill runs at the session model, defeating the wrapper's
# model/effort pin), but they fix it differently: Cursor needs this generated alwaysApply dispatch
# rule, while Claude Code uses native per-skill `context: fork` frontmatter (see skills/docket-*/
# SKILL.md). So only Cursor belongs in this list.
HARNESS_HAS_DISPATCH_RULES="$DOCKET_GI_DISPATCH_HARNESSES"
harness_has_dispatch_rule(){ case " $HARNESS_HAS_DISPATCH_RULES " in *" $1 "*) return 0;; *) return 1;; esac; }

# Codex and opencode both read a committed project-root AGENTS.md; they share the single managed
# dispatch block (changes 0077, 0192). A repo targeting either gets it; targeting both gets it once.
AGENTS_MD_DISPATCH_HARNESSES="codex opencode"
# True if harness $1 is one that reads a committed AGENTS.md dispatch block.
harness_gets_agents_md(){ case " $AGENTS_MD_DISPATCH_HARNESSES " in *" $1 "*) return 0;; *) return 1;; esac; }
# True if the repo targets any AGENTS.md-dispatch harness (drives write-vs-strip + the --check leg).
repo_wants_agents_md_dispatch(){
  local h
  for h in $HARNESSES; do harness_gets_agents_md "$h" && return 0; done
  return 1
}
DISPATCH_START='<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->'
DISPATCH_END='<!-- docket:dispatch:end -->'

# --- the Claude parent-facing surface (change 0242) ---------------------------
# ADR-0024 solved Claude's *routing* natively (context: fork — "no generated file"), so no
# parent-facing Claude surface was ever built. The run gate is not routing: it is what the parent
# does AFTER a routed run returns, and it needs an always-loaded surface to live on. Claude Code's
# documented always-loaded file is CLAUDE.md; we target that documented surface rather than betting
# on a given version also reading AGENTS.md (LEARNINGS harness-behavior-is-mode-and-version-scoped).
repo_wants_claude_surface(){ case " $HARNESSES " in *" claude "*) return 0;; *) return 1;; esac; }

# Print $1 with every symlink resolved, absolute. A missing path prints its own absolute form, so
# this never fails and is safe to call on a surface the write pass is about to create.
#
# Bash-only by necessity: stock macOS ships no coreutils `realpath` and `readlink -f` is GNU-only
# (AGENTS.md, shell portability). Every hop re-canonicalises the DIRECTORY with `cd`/`pwd -P` —
# including the absolute-target hop, whose own spelling may route through a symlinked parent and
# would otherwise yield a second, non-equal name for one physical file, defeating the dedupe this
# exists to serve. The walk is bounded so a symlink cycle cannot hang the sync; a cycle exits the
# loop still pointing at a link, which is the honest answer for a path that has no physical form.
resolve_physical_path(){  # $1 = path
  local p="$1" d b t nd n=0
  d="$(dirname -- "$p")"; b="$(basename -- "$p")"
  d="$(cd "$d" 2>/dev/null && pwd -P)" || { printf '%s\n' "$p"; return 0; }
  while [ -L "$d/$b" ] && [ "$n" -lt 32 ]; do
    t="$(readlink "$d/$b")"
    [ -n "$t" ] || break
    # Land the hop in a SCRATCH variable, never in `d` itself: `d="$(failing-substitution)" || break`
    # assigns the failed command's empty output BEFORE the break is taken, and an empty `d` prints
    # as `/<basename>` — a path at the filesystem ROOT that the write pass would go on to act on.
    # A hop can fail on any dangling link whose parent directory is missing or unreadable (a
    # committed link to a path outside the checkout, a dotfiles target absent on this machine, an
    # unmounted volume), and a dangling link reaches here because it is `-L` and so is never
    # replaced by claude_surface_target. Falling back to the last resolvable value keeps the answer
    # inside the tree we were walking.
    case "$t" in
      /*) nd="$(cd "$(dirname -- "$t")" 2>/dev/null && pwd -P)" || break ;;
      *)  nd="$(cd "$d/$(dirname -- "$t")" 2>/dev/null && pwd -P)" || break ;;
    esac
    d="$nd"
    b="$(basename -- "$t")"
    n=$((n+1))
  done
  printf '%s/%s\n' "$d" "$b"
}

# Print this repository's own PHYSICAL root — the containment yardstick below. $REPO is $PWD, which
# is LOGICAL, so it must be canonicalised exactly the way resolve_physical_path canonicalises the
# directories it walks; otherwise a checkout reached through a symlinked parent (macOS's
# /tmp -> /private/tmp, a symlinked worktree root) fails its own containment test and docket would
# refuse to write the surfaces it does own.
repo_physical_root(){ ( cd "$REPO" 2>/dev/null && pwd -P ) || printf '%s\n' "$REPO"; }

# True when a RESOLVED physical path lies inside the repository. Both dispatch-surface passes gate
# on this, and both evaluate it on the very path they hand to the block helpers — never on a
# pre-resolution spelling (LEARNINGS decide-and-act-on-the-same-copy). AGENTS.md and CLAUDE.md are
# ordinary files a user may symlink anywhere: to a shared instructions file in a sibling checkout,
# to ~/dotfiles, to a mount. Docket owns the block only inside the checkout it was run in, so a
# surface that resolves elsewhere is neither written into nor stripped.
path_inside_repo(){  # $1 = resolved physical path ; $2 = physical repo root
  case "$1" in "$2"/*) return 0;; *) return 1;; esac
}

# Print the physical file the Claude block must be written into, creating the surface when absent.
# Three cases, in the spec's order:
#   CLAUDE.md exists (file or symlink) -> its physical path; never replaced, only written into.
#   absent, AGENTS.md wanted or present -> create CLAUDE.md as a committed relative symlink to it,
#                                         so Claude loads ONE physical instructions file (the gate
#                                         AND everything else AGENTS.md carries, e.g. promoted
#                                         learnings), identically to codex/opencode.
#   neither                            -> create a real, empty CLAUDE.md to seed the block into.
# The symlink predicate is "will this repo have an AGENTS.md", not "does it have one right now": on
# a virgin [claude, codex] repo AGENTS.md is created by the very write pass that resolves this
# target, and asking about the present tense there seeds a second real file carrying a duplicate of
# the same managed block forever after.
claude_surface_target(){
  repo_wants_claude_surface || return 1
  local c="$REPO/CLAUDE.md"
  if [ ! -e "$c" ] && [ ! -L "$c" ]; then
    if [ -e "$REPO/AGENTS.md" ] || repo_wants_agents_md_dispatch; then
      ( cd "$REPO" && ln -s AGENTS.md CLAUDE.md ) || return 1
      log "created CLAUDE.md as a symlink to AGENTS.md (one physical instructions file) — COMMIT THIS."
    else
      : > "$c" || return 1
      log "created CLAUDE.md to carry the docket dispatch block — COMMIT THIS."
    fi
  fi
  resolve_physical_path "$c"
}

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

# --- runners.<name>.<key>: per-key across layers (change 0269) ----------------
# The shim wrapper runs in the PARENT harness and does one thing: a foreground `docket.sh
# runner-dispatch` call plus a stdout relay. Its frontmatter pin therefore governs the parent-side
# agent and must be resolvable by the parent — the child's pin is the baked `--model` argument and
# only that. These two knobs are what the frontmatter carries.
#
# Layering is repo-local > repo-committed > global, PER KEY: the first layer that carries the key
# wins, and a layer supplying only one of the two leaves the other to resolve further down (or to
# its default). Same rule runner-dispatch.sh already applies to the rest of the block.
#
# Composed from section_body rather than re-implementing the dedenting walk. This makes
# sync-agents.sh the SECOND independent consumer of the `runners:` block — runner-dispatch.sh's
# `runner_block`/`yaml_section` pair is the first — which is a knowing deferral, not an oversight:
# unifying the two parsers is change #0256's scope, and it should absorb both readers when it lands.
# The value class here is the BLOCK-mapping one (rest of the line, comment stripped, trimmed), not
# the `{…}` flow-map class the agents entries use; a block mapping has one key per line.

# THE extraction primitive for that value class — one spelling of the key match and the value strip,
# shared by the reader (runner_key) and the gate (validate_runner_shim_values). The two differ in
# how they WALK the layers, not in how they read a key out of one already-resolved block: the reader
# stops at the first layer carrying the key (precedence), the gate visits every layer including the
# ones precedence masks. Keeping the walks separate is deliberate (see the gate's header); keeping
# TWO copies of the extraction was not — a fix to one (anchoring the key regex, changing the
# comment-strip rule, handling a quoted value) would leave the other behind, and the failure mode is
# the gate blessing one value while the emitter writes a different one into the frontmatter
# (LEARNINGS: duplicated-gate-copies-the-whole-predicate).
#
# PRESENT-BUT-EMPTY IS NOT ABSENT, and the return code is what carries the difference: returns 0
# having printed the value (possibly the empty string) when the block carries the key, 1 when it
# does not. Both callers need that split — the reader must let `shim_model:` with no value END the
# precedence walk (an explicit empty is a decision, not a fall-through), and the gate must report it
# as an offender rather than skip it.
runner_block_value() {  # $1=key ; runners.<name> block body on STDIN -> value; rc 1 = key absent
  local line
  line="$(awk -v k="$1" '
    { nc=$0; sub(/#.*/, "", nc) }
    nc ~ ("^[[:space:]]*" k "[[:space:]]*:") { print nc; exit }
  ')"
  [ -n "$line" ] || return 1
  sed -E -e 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*//' -e 's/[[:space:]]+$//' <<<"$line"
  return 0
}

runner_key() {  # $1=runner  $2=key  -> the value from the highest-precedence layer carrying it, else ''
  local f blk v
  for f in "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"; do
    [ -f "$f" ] || continue
    # AGENTS.md: never `producer | early-exiting-consumer` under `set -o pipefail`. section_body's
    # awk `exit`s the moment the block ends, so piping one into the next would leave the producer
    # taking a SIGPIPE on any config large enough to outrun the pipe buffer. Capture, then feed in.
    blk="$(section_body runners < "$f")"
    [ -n "$blk" ] || continue
    blk="$(section_body "$1" <<<"$blk")"
    [ -n "$blk" ] || continue
    if v="$(runner_block_value "$2" <<<"$blk")"; then
      printf '%s' "$v"
      return 0
    fi
  done
  return 0
}

# Run-scoped memo of the two shim pins per runner, WITH their defaults applied. runner_key costs a
# section_body pair plus the primitive's awk and sed on every layer for every key, and emit_wrapper
# asks for both keys on every delegated wrapper — the same answer, recomputed once per wrapper.
#
# Bash-3.2-safe by construction: a newline-separated string scanned with the shell's own `read`, not
# an associative array, and no fork on the hit path. Sound because the config layers this reads are
# FIXED for the run below migrate_legacy_global — the one rewrite this script performs sits above
# every generation pass (see validate_runner_config's placement bounds). If a future call site puts
# emit_wrapper inside a command substitution the memo simply stops carrying across calls; that costs
# the optimization, never a wrong answer.
_SHIM_PIN_MEMO=""
resolve_shim_pins() {  # $1=runner -> sets SHIM_MODEL, SHIM_EFFORT
  local runner="$1" r m e
  while IFS=$'\x1f' read -r r m e; do
    if [ "$r" = "$runner" ]; then SHIM_MODEL="$m"; SHIM_EFFORT="$e"; return 0; fi
  done <<<"$_SHIM_PIN_MEMO"
  SHIM_MODEL="$(runner_key "$runner" shim_model)"
  [ -n "$SHIM_MODEL" ] || SHIM_MODEL="inherit"
  SHIM_EFFORT="$(runner_key "$runner" shim_effort)"
  [ -n "$SHIM_EFFORT" ] || SHIM_EFFORT="low"
  _SHIM_PIN_MEMO="$_SHIM_PIN_MEMO$runner"$'\x1f'"$SHIM_MODEL"$'\x1f'"$SHIM_EFFORT"$'\n'
  return 0
}

if [ "${BASH_VERSINFO[0]}" -ge 4 ]; then
  declare -A _LAYER_BODY_CACHE=()
else
  _LAYER_BODY_CACHE=()
fi

# Prime on a synchronous caller path: cache writes made in `line="$(harness_agent_line ...)"`
# occur in a subshell and would not be available to subsequent command-substituted reads.
prime_layer_body() {  # $1=file $2=harness $3=under_agents(0|1)
  local file="$1" harness="$2" under_agents="$3" key sub body
  [ "${BASH_VERSINFO[0]}" -ge 4 ] || return 0
  key="${file}"$'\x1f'"${harness}"$'\x1f'"${under_agents}"
  [ -z "${_LAYER_BODY_CACHE[$key]+_}" ] || return 0
  if [ ! -f "$file" ]; then
    _LAYER_BODY_CACHE[$key]=""
    return 0
  fi
  if [ "$under_agents" = "1" ]; then
    sub="$(section_body agents < "$file")"
  else
    sub="$(<"$file")"
  fi
  body="$(section_body "$harness" <<<"$sub" || true)"
  _LAYER_BODY_CACHE[$key]="$body"
}

# Validate the fixed shipped sidecar shape in one parser pass. hd_validate remains the Bash 3.2
# fallback and the standalone library contract; on Bash 4+ its deliberately composable readers
# would otherwise reparse the same 24 rows hundreds of times before every generation.
validate_harness_defaults() {  # $1=file $2=sources-dir
  local file="$1" sources="$2" src name source_names="" rc
  if [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
    hd_validate "$file" "$sources"
    return $?
  fi
  if [ ! -f "$file" ] || [ ! -r "$file" ]; then
    printf 'harness-defaults: missing or unreadable: %s\n' "$file" >&2
    return 1
  fi
  for src in "$sources"/docket-*.md; do
    [ -e "$src" ] || continue
    name="${src##*/}"; name="${name#docket-}"; name="${name%.md}"
    source_names="${source_names}${name} "
  done
  awk -v known="$HD_KNOWN_HARNESSES" -v shipped="$HD_SHIPPED_HARNESSES" \
      -v sources="$source_names" -v source_dir="$sources" '
    function has(words, word) { return index(" " words " ", " " word " ") != 0 }
    function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); return s }
    function diag(s) { print "harness-defaults: " s > "/dev/stderr"; rc=1 }
    # Does the RAW line carry a `#` INSIDE its flow map? Twin of `flow_map_has_comment` below and of
    # `_hd_flow_map_has_comment` in scripts/lib/harness-defaults.sh — same rule, duplicated BY VALUE
    # for the same reason the quote leg is (the shipped-data reader is deliberately not coupled to
    # the user-config readers; extracting the shared helper is change #0256). Parity is held by test,
    # in tests/test_harness_defaults_flow_map.sh.
    #
    # The rule, exactly: it APPLIES only when the first `{` precedes any `#` on the line (a `#`
    # before the first `{`, or no `{` at all, never fires). It then FIRES iff, after that `{`, a `#`
    # appears before the first `}`, or a `#` appears with no `}` at all. A trailing comment after
    # `}`, a full-line comment, and a commented-out map all stay legal. index(), not a regex: both
    # braces are regex metacharacters, and there is nothing here a substring search cannot decide.
    # The closing-brace local is `shut`, not `close`: `close` is a BWK awk builtin (the awk macOS
    # ships) and using it as a parameter is a hard parse error, not a shadowing warning.
    function flow_comment(l,   b, hash, after, shut) {
      b = index(l, "{")
      if (b == 0) return 0
      hash = index(l, "#")
      if (hash != 0 && hash < b) return 0
      after = substr(l, b + 1)
      hash = index(after, "#")
      if (hash == 0) return 0
      shut = index(after, "}")
      if (shut == 0) return 1
      return hash < shut
    }
    {
      if ($0 ~ /^agents:[[:space:]]*$/) top=1
      nc=$0; sub(/#.*/, "", nc)
      if (nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:/) {
        if (nc !~ /^  [A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/) { h=""; next }
        h=nc; sub(/^  /, "", h); sub(/[[:space:]]*:.*/, "", h)
        harness_count[h]++
        if (h == "default") diag("a harness-neutral \047default:\047 block is forbidden — every entry must name a concrete harness")
        else if (!has(known, h)) diag("unknown harness \047" h "\047 (known: " known ")")
        next
      }
      if (h != "" && nc ~ /^    [A-Za-z0-9._-]+[[:space:]]*:/) {
        a=nc; sub(/^    /, "", a); sub(/[[:space:]]*:.*/, "", a)
        entry_count[h SUBSEP a]++
        present[h SUBSEP a]=1
        if (!has(sources, a)) diag(h "/" a " names no wrapper source (" source_dir "/docket-" a ".md)")
        # Judged on $0, the PRE-STRIP line: everything below reads `nc`, with the comment already
        # removed, so the truncation is structurally invisible to it. Without this leg the input is
        # still rejected — but only INCIDENTALLY, by the unterminated map falling into the
        # field-absent branch, which blames bareness and names neither the `#` nor the flow map.
        # `next` because every check below now judges a knowingly truncated line: emitting an
        # ABSENCE complaint on top of the accurate sentence is the wrong-cause diagnostic this leg
        # exists to remove, and hd_validate emits the flow-map sentence alone for the same input.
        if (flow_comment($0)) {
          diag(h "/" a " entry contains \047#\047 inside the flow map — comments cannot appear inside {…}; docket strips them before parsing")
          next
        }
        fields=nc
        if (fields !~ /\{.*\}/) { model=""; effort="" }
        else {
          sub(/^[^{]*\{/, "", fields); sub(/\}[^}]*$/, "", fields)
          model=""; effort=""
          n=split(fields, parts, ",")
          for (i=1; i<=n; i++) {
            part=parts[i]; key=part; sub(/:.*/, "", key); key=trim(key)
            raw=part; sub(/^[^:]*:/, "", raw); raw=trim(raw)
            if (key == "") continue
            if (key == "model") model=raw
            else if (key == "effort") effort=raw
            else if (key == "runner") diag(h "/" a " sets \047runner\047 — delegation is user policy, never a shipped default")
            else diag(h "/" a " has unknown field \047" key "\047 (allowed: model, effort)")
          }
        }
        values["model"]=model; values["effort"]=effort
        for (key in values) {
          raw=values[key]
          consumed=raw; sub(/[[:space:]].*$/, "", consumed)
          lead=substr(raw, 1, 1)
          # Two legs (ADR-0065), duplicated BY VALUE from hd_validate — the shipped-data reader is
          # deliberately not coupled to the user-config readers, so parity here is held by test
          # (tests/test_harness_defaults_flow_map.sh), not by a shared helper. The `consumed != raw`
          # leg catches whatever the value class cannot express (an embedded space). The quote leg
          # catches what that comparison structurally CANNOT see: the value class of hd_field,
          # [^,}[:space:]]+, consumes the quotes whole, so a quoted but space-free pin has
          # consumed == raw and would ride into the emitted wrapper verbatim while this very
          # diagnostic tells the reader to write it unquoted. Single quotes included — the remedy
          # says "unquoted", not "double-unquoted". \042 and \047 are the string escapes for the
          # two quote characters; this whole program sits inside a single-quoted shell word, so a
          # literal apostrophe cannot appear anywhere in it, comments included.
          if (consumed == "") diag(h "/" a " is missing a non-empty \047" key "\047")
          else if (consumed != raw || lead == "\042" || lead == "\047") diag(h "/" a " \047" key "\047 value \047" raw "\047 is not a bare scalar — the reader consumes only \047" consumed "\047; write model/effort values unquoted and space-free")
        }
      }
    }
    END {
      if (!top) diag("no top-level \047agents:\047 block")
      for (key in harness_count) if (harness_count[key] > 1) diag("duplicate harness block \047" key "\047")
      for (key in entry_count) if (entry_count[key] > 1) {
        split(key, pair, SUBSEP); diag("duplicate entry \047" pair[2] "\047 under \047" pair[1] "\047")
      }
      ns=split(shipped, hs, /[[:space:]]+/); na=split(sources, agents, /[[:space:]]+/)
      for (i=1; i<=ns; i++) for (j=1; j<=na; j++)
        if (hs[i] != "" && agents[j] != "" && !present[hs[i] SUBSEP agents[j]])
          diag(hs[i] " block is incomplete — no entry for \047" agents[j] "\047")
      exit rc
    }
  ' "$file"
  rc=$?
  return $rc
}

# Every built-in agent source must DECLARE its worktree scope (change 0208). Loud and fatal, and
# deliberately at GENERATION time rather than at runtime: this is where new agents get wired, so it
# is the seam at which an undeclared agent is still preventable. The facade's runtime read is
# tolerant by design — a missing file or key there must keep the adapter's more specific
# unknown-agent diagnostic rather than shadowing it.
validate_agent_scopes(){  # $1 = sources dir
  local src name scope bad=0
  for src in "$1"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    scope="$(agent_worktree_scope "$src")"
    case "$scope" in
      feature|metadata)
        # A CONSISTENCY BOND with scripts/runner-dispatch.sh, NOT a re-introduction of the name
        # shape as the source of truth for scope. That script still reads THIS SAME FAMILY by name
        # — `case "$AGENT" in build-*)`, the empty-payload refusal, which is build-specific on
        # purpose and is not widened to the feature-scoped set — so `build-*` is a live predicate in
        # this codebase either way. Two readings of one family that can DISAGREE is the defect, and
        # the disagreement is silent in the direction that matters: a build source declaring
        # `metadata` keeps the payload refusal while losing gate 1, gate 3b and the shim's
        # `--worktree` slot, shipping exactly the un-gated build worker change 0208 exists to
        # prevent. Every other agent's scope is still whatever it declares, read from the
        # declaration and nowhere else. Delete this arm only TOGETHER WITH runner-dispatch.sh's
        # `build-*` case — never before it.
        case "$name" in
          build-*)
            [ "$scope" = "feature" ] || { log "ERROR agent '$name' declares worktree-scope '$scope' — a build-* agent must declare 'feature': runner-dispatch.sh still keys its empty-payload refusal on the build-* name shape, so a build source declaring anything else makes the two readings of the same family disagree, and this one loses the --worktree requirement and the main-tree rejection silently ($src)"; bad=1; }
            ;;
        esac
        ;;
      '') log "ERROR agent '$name' declares no worktree-scope: — add 'worktree-scope: feature' or 'worktree-scope: metadata' to $src"; bad=1 ;;
      *)  log "ERROR agent '$name' declares an invalid worktree-scope '$scope' — the only values are 'feature' and 'metadata' ($src)"; bad=1 ;;
    esac
  done
  [ "$bad" = "0" ]
}

# field_of() — the flow-map value reader (change 0173).
#
# The value class is "everything up to the flow-map delimiters" — NOT a character allowlist.
# ADR-0015 makes model IDs opaque passthrough with no vendor allowlist, and provider-prefixed IDs
# (`anthropic/claude-opus-5`, `openrouter:vendor/model`) are ordinary. The pre-0173 class
# ([A-Za-z0-9._-]+) did not REJECT such an ID — which would at least be honest — it TRUNCATED it to
# a first segment that still looks well-formed, and the generator baked that wrong pin into the
# wrapper with no warning. This is the same class, and the same fix, as hd_field in
# scripts/lib/harness-defaults.sh (change 0168); the two readers deliberately match.
# Anything this class cannot express is caught by validate_user_agent_values, not silently clipped.
field_of() {  # $1=line  $2=field
  local re=".*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([^,}[:space:]]+).*"
  [[ $1 =~ $re ]] && printf '%s' "${BASH_REMATCH[1]}"
  return 0
}

# field_of_raw() — the RAW field text: everything between the colon and the next flow-map delimiter
# (`,` or `}`), trailing whitespace trimmed. This is what a YAML parser would see; field_of is what
# DOCKET's reader consumes. validate_user_agent_values rejects any entry where the two disagree, so
# a value the reader cannot consume whole fails loudly instead of shipping as a truncated prefix.
# The `_raw` tier follows the existing pair convention — docket-frontmatter.sh has field/field_raw
# (ADR-0058), harness-defaults.sh has hd_field/hd_field_raw — though the split here is
# reader-capability, not quote-style.
field_of_raw() {  # $1=line  $2=field
  local re=".*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([^,}]*).*" out
  [[ $1 =~ $re ]] || return 0
  out="${BASH_REMATCH[1]}"
  while [[ $out == *[[:space:]] ]]; do out="${out%?}"; done
  printf '%s' "$out"
}

# Print the `agents.<harness>.<agent>` entry line from <file>. under_agents=1 => the harness map is
# nested under a top-level `agents:` key (.docket.yml); 0 => the harness map is the whole file (global).
# keep_comments=1 returns the line WITHOUT the comment strip, for validate_user_agent_values' check
# that no `#` sits inside the flow map; every other caller wants the stripped default. Both bash
# paths match on the STRIPPED view in both modes, so they cannot select different lines.
harness_agent_line() {  # $1=file  $2=harness  $3=agent  $4=under_agents(0|1)  [$5=keep_comments]
  local key body line stripped sub hbody matched keep="${5:-0}"
  if [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
    [ -f "$1" ] || return 0
    if [ "$4" = "1" ]; then sub="$(section_body agents < "$1")"; else sub="$(cat "$1")"; fi
    hbody="$(printf '%s\n' "$sub" | section_body "$2" || true)"
    matched="$(awk -v a="$3" -v keep="$keep" '
      { nc=$0; sub(/#.*/,"",nc) }
      nc ~ ("^[[:space:]]*" a "[[:space:]]*:") { print (keep == "1" ? $0 : nc); exit }
    ' <<<"$hbody")"
    printf '%s\n' "$matched"
    return 0
  fi
  key="${1}"$'\x1f'"${2}"$'\x1f'"${4}"
  body="${_LAYER_BODY_CACHE[$key]-}"
  while IFS= read -r line || [ -n "$line" ]; do
    stripped="${line%%#*}"
    if [[ $stripped =~ ^[[:space:]]*${3}[[:space:]]*: ]]; then
      if [ "$keep" = "1" ]; then printf '%s' "$line"; else printf '%s' "$stripped"; fi
      return 0
    fi
  done <<<"$body"
}

# Does <raw entry line> carry a `#` INSIDE its `{…}` flow map? Returns 0 (fires) when it does.
#
# Comments are stripped before either field reader sees a line, so a `#` inside the flow map
# truncates the entry silently — `{ model: c#5 }` becomes `{ model: c` and every value-comparison
# leg agrees the value is fine. Such a `#` is OUT OF CONTRACT; the strip order is deliberately
# unchanged (reordering it would break the legitimate trailing and full-line comments used across
# every layer), and this predicate makes the corner a loud refusal instead.
#
# The rule, exactly: it APPLIES only when the entry's first `{` precedes any `#` on the line (a `#`
# before the first `{`, or no `{` at all, never fires). It then FIRES iff, after that `{`, a `#`
# appears before the first `}`, or a `#` appears with no `}` at all. So a trailing comment after
# `}`, a full-line comment, and a commented-out map all stay legal.
#
# Twin: `_hd_flow_map_has_comment` in scripts/lib/harness-defaults.sh — same body, different name.
# Duplicated by value on purpose: that library's header forbids coupling the shipped-data reader to
# these user-config readers, and extracting the shared helper is change #0256's scope.
flow_map_has_comment() {  # $1=raw entry line
  local l="$1" after
  case "$l" in *'{'*) : ;; *) return 1 ;; esac
  case "${l%%\{*}" in *'#'*) return 1 ;; esac          # a `#` before the first `{` never fires
  after="${l#*\{}"
  case "$after" in *'#'*) : ;; *) return 1 ;; esac     # no `#` after the `{` at all
  case "$after" in *'}'*) : ;; *) return 0 ;; esac     # `#` present, no `}` at all -> truncation
  case "${after%%\}*}" in *'#'*) return 0 ;; esac      # `#` before the first `}` -> truncation
  return 1
}

# Resolve (harness, agent) per-field across the given layer files, highest precedence
# first (each read under a top-level agents: wrapper). Within a layer the harness line
# beats the default line; across layers the first layer supplying a field wins; the
# shipped floor is agents/harness-defaults.yml (change 0168), applied below.
# RES_MODEL_FROM_HARNESS=1 iff the model came from a harness-specific line in ANY
# USER layer (drives warn_fallback_model) — the shipped sidecar never sets it.
# RES_MODEL_FROM_USER / RES_EFFORT_FROM_USER = 1 iff that field came from a user
# layer at all (harness-specific or default). RES_RUNNER (change 0079) resolves the
# same way; the pre-0079 early break is gone — runner rarely fills, and the loop
# spans at most three small files.
resolve_agent_layers() {  # $1=harness  $2=agent  $3..=layer files (precedence order)
  local harness="$1" agent="$2" f hline dline hm he dm de hr dr sline
  shift 2
  RES_MODEL=""; RES_EFFORT=""; RES_RUNNER=""; RES_MODEL_FROM_HARNESS=0
  RES_MODEL_FROM_USER=0; RES_EFFORT_FROM_USER=0
  for f in "$@"; do
    prime_layer_body "$f" "$harness" 1
    prime_layer_body "$f" default 1
    hline="$(harness_agent_line "$f" "$harness" "$agent" 1)"
    dline="$(harness_agent_line "$f" default "$agent" 1)"
    hm="$(field_of "$hline" model)";  he="$(field_of "$hline" effort)"
    dm="$(field_of "$dline" model)";  de="$(field_of "$dline" effort)"
    hr="$(field_of "$hline" runner)"; dr="$(field_of "$dline" runner)"
    if [ -z "$RES_MODEL" ]; then
      if   [ -n "$hm" ]; then RES_MODEL="$hm"; RES_MODEL_FROM_HARNESS=1; RES_MODEL_FROM_USER=1
      elif [ -n "$dm" ]; then RES_MODEL="$dm"; RES_MODEL_FROM_USER=1; fi
    fi
    if [ -z "$RES_EFFORT" ]; then
      if   [ -n "$he" ]; then RES_EFFORT="$he"; RES_EFFORT_FROM_USER=1
      elif [ -n "$de" ]; then RES_EFFORT="$de"; RES_EFFORT_FROM_USER=1; fi
    fi
    if [ -z "$RES_RUNNER" ]; then
      if   [ -n "$hr" ]; then RES_RUNNER="$hr"
      elif [ -n "$dr" ]; then RES_RUNNER="$dr"; fi
    fi
  done
  # Shipped floor (change 0168): the sidecar is harness-indexed, so it can only supply a value for
  # the harness being generated. It never sets RES_*_FROM_USER — that split is what keeps a shipped
  # native default out of a delegated child-runner's flags (see emit_wrapper).
  # RES_MODEL_FROM_SIDECAR records whether the sidecar ACTUALLY supplied the resolved model, which
  # is not the same question as whether the sidecar HOLDS an entry for the pair: a user
  # `agents.default` line wins the field above and leaves the sidecar entry unused. Only the former
  # licenses warn_fallback_model's silence.
  RES_MODEL_FROM_SIDECAR=0
  if [ "${BASH_VERSINFO[0]}" -ge 4 ]; then
    prime_layer_body "$HARNESS_DEFAULTS" "$harness" 1
    sline="$(harness_agent_line "$HARNESS_DEFAULTS" "$harness" "$agent" 1)"
  else
    sline=""
  fi
  if [ -z "$RES_MODEL" ]; then
    if [ "${BASH_VERSINFO[0]}" -ge 4 ]; then
      RES_MODEL="$(field_of "$sline" model)"
    else
      RES_MODEL="$(hd_field "$HARNESS_DEFAULTS" "$harness" "$agent" model)"
    fi
    [ -n "$RES_MODEL" ] && RES_MODEL_FROM_SIDECAR=1
  fi
  if [ -z "$RES_EFFORT" ]; then
    if [ "${BASH_VERSINFO[0]}" -ge 4 ]; then
      RES_EFFORT="$(field_of "$sline" effort)"
    else
      RES_EFFORT="$(hd_field "$HARNESS_DEFAULTS" "$harness" "$agent" effort)"
    fi
  fi
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

# --- user-config value validation (change 0173) ------------------------------
# field_of consumes bare scalars only. A value it cannot consume WHOLE — a quoted scalar, an
# embedded space — would otherwise be clipped to a prefix that still looks well-formed and baked
# into a wrapper as a wrong pin. Collect every offender across every layer, report them all, and
# fail BEFORE any wrapper is written: partial generation carrying a known-bad pin is exactly the
# harm this exists to prevent.
#
# Posture note: this is deliberately asymmetric with scripts/runner-dispatch.sh, which stays
# tolerant. Generation time has a human reading output and leaves a wrong pin persisted in a file;
# runner-dispatch runs mid-handoff on a live dispatch path, where dying would convert a cosmetic
# config typo into a failed dispatch.
#
# Only the harness-first shape is walked. The pre-0046 flat shape is warned about and DROPPED by
# warn_legacy_shape/legacy_agent_keys, so validating it would reject config that is already ignored.
validate_user_agent_values() {
  local rc=0 f h a k line rawline raw consumed
  for f in "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"; do
    [ -f "$f" ] || continue
    while IFS= read -r h; do
      [ -n "$h" ] || continue
      # Skip config every pass already DROPS, so the gate cannot hard-fail a repo over entries that
      # generate nothing — the same reasoning that exempts the pre-0046 flat shape above (change
      # 0173 review). A block for a harness outside agent_harnesses is warned "ignored (dead
      # config)"; an agent key overriding no built-in is warned "ignored (typo?)".
      #
      # The harness test deliberately ERRS TOWARD VALIDATING: it skips only when the block is
      # consumable by NEITHER pass, since USER_TARGETS is resolved after this gate runs. Missing a
      # live block would let the silent truncation this change exists to close slip through, which
      # is strictly worse than an over-rejection.
      if [ "$h" != "default" ]; then
        case " ${HARNESSES:-} ${USER_HARNESSES:-} " in
          *" $h "*) : ;;
          *) [ -d "$HARNESS_ROOT/.$h" ] || continue ;;
        esac
      fi
      prime_layer_body "$f" "$h" 1
      while IFS= read -r a; do
        [ -n "$a" ] || continue
        [ -f "$AGENTS_SRC/docket-$a.md" ] || continue
        line="$(harness_agent_line "$f" "$h" "$a" 1)"
        [ -n "$line" ] || continue
        # A `#` inside the `{…}` flow map truncates the entry before any reader sees it, so the
        # value legs below structurally cannot catch it — see `flow_map_has_comment`. This check
        # sits INSIDE the dead-config carve-outs above (the harness skip and the wrapper-source
        # test) so config that generates nothing still cannot hard-fail a repo.
        rawline="$(harness_agent_line "$f" "$h" "$a" 1 1)"
        if flow_map_has_comment "$rawline"; then
          log "$h/$a entry contains '#' inside the flow map — comments cannot appear inside {…}; docket strips them before parsing ($f)"
          rc=1
        fi
        for k in model effort runner; do
          # Key absent from this entry is normal — every field is optional in user config.
          # Herestring, not `printf … | grep -Eq`: under `set -o pipefail` an early-exiting consumer
          # SIGPIPEs its producer and the 141 becomes an intermittent skip (AGENTS.md, "Shell").
          grep -Eq "[{,[:space:]]${k}[[:space:]]*:" <<<"$line" || continue
          raw="$(field_of_raw "$line" "$k")"
          consumed="$(field_of "$line" "$k")"
          if [ -z "$raw" ]; then
            # Present-but-empty. A DIFFERENT diagnostic from the one below on purpose: without the
            # split, a clip that lands empty blames ABSENCE for what is really a quoting problem.
            log "$h/$a '$k' is present but has no value ($f)"; rc=1
          elif [ "$raw" != "$consumed" ] || case "$raw" in '"'*|"'"*) true;; *) false;; esac; then
            # Two legs. The != leg catches anything the value class cannot express (an embedded
            # space). The quote leg catches what != structurally CANNOT see: a quoted but
            # space-free value has consumed == raw, so the quotes would ride into the emitted pin
            # verbatim while the diagnostic's own remedy text tells the user to write them unquoted.
            log "$h/$a '$k' value '$raw' is not a bare scalar — the reader consumes only '$consumed'; write model/effort values unquoted and space-free ($f)"
            rc=1
          fi
        done
      done < <(agent_keys "$f" 1)
    done < <(agents_block_harnesses "$f")
  done
  return $rc
}

# --- the candidate-triple population, shared by both runner gates -------------
# Walks every (harness, agent) pair a generating pass would write and calls $1 once per pair with
# resolve_agent_layers' state (RES_MODEL, RES_RUNNER, RES_*_FROM_USER) still live:
#
#     $1 <harness> <agent>
#
# The callback MUST return 0: `set -e` is active and this walk is not wrapped.
#
# EXTRACTED, NOT COPIED (change 0269 review). Two gates now have to judge exactly the config that
# generates something, and THIS WALK IS THE DEFINITION of "something" — the user-level leg over
# USER_TARGETS resolved against the global layer only, plus the project-level leg gated by
# project_wrappers_generated. A second hand-written copy of it would agree on every ordinary repo
# and diverge on precisely the states these guards exist to exclude: a non-opted-in repo, a global
# agent_harnesses list that narrows the user-level targets (LEARNINGS:
# duplicated-gate-copies-the-whole-predicate). Sharing one body makes that divergence
# unrepresentable rather than merely tested for.
#
# It loops every agent x every harness and lets each caller's callback decide applicability —
# narrowing to `claude`, or to a runner-bearing entry, here would put the rule's scope in a second
# place, and the day that scope moves this walk would silently under-enumerate.
#
# USER_TARGETS is computed by user_level_pass, which runs BELOW both gates; on the --check path
# neither compute_user_targets nor resolve_global_agent_harnesses has run at all, so under `set -u`
# both USER_TARGETS and USER_HARNESSES_SET are unset. Resolve them here rather than reading
# `${USER_TARGETS:-}`: an empty list would silently skip the whole user-level leg, and a skipped
# leg is precisely the under-enumeration these gates exist to prevent. Both are idempotent config
# reads; the `-n` test keeps the real run — and the second gate of the pair — from re-emitting
# resolve_global_agent_harnesses' "unknown agent_harnesses token" warnings.
#
# Runs in the CALLER's shell, never under `$(…)`: the two globals above, CANDIDATE_RUNNERS below,
# and prime_layer_body's cache must all survive the walk.
#
# SIDE PRODUCT: every walk refreshes CANDIDATE_RUNNERS from scratch, whatever the callback does —
# see resolve_candidate_runners. A walk is always complete, so the set can never be left partial or
# stale WITHIN a run; it would only go stale across a config rewrite, and the only one this script
# performs (migrate_legacy_global) is fixed ABOVE both gates by the placement bounds below.
for_each_candidate_triple() {  # $1 = callback function name
  local cb="$1" src name harness
  [ -n "${USER_HARNESSES_SET:-}" ] || resolve_global_agent_harnesses
  compute_user_targets
  CANDIDATE_RUNNERS=""
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    # user-level pass: USER_TARGETS resolved over the global layer only
    for harness in $USER_TARGETS; do
      resolve_agent_layers "$harness" "$name" "$GLOBAL_CFG"
      collect_candidate_runner
      "$cb" "$harness" "$name"
    done
    # project-level pass: HARNESSES resolved over local + committed + global.
    # `continue`, not an early `return 0` hoisted out of the loop: it skips only THIS agent's
    # project-level leg, leaving the user-level leg above intact for a non-opted-in repo.
    project_wrappers_generated || continue
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      collect_candidate_runner
      "$cb" "$harness" "$name"
    done
  done
  CANDIDATE_RUNNERS_RESOLVED=1
  return 0
}

# The runners THIS run's triples actually resolve to — space-separated, unique, registered only.
# Written only by the walk above; read by validate_runner_shim_values.
CANDIDATE_RUNNERS=""
CANDIDATE_RUNNERS_RESOLVED=0

# Fold the triple resolve_agent_layers just resolved into CANDIDATE_RUNNERS. Not a callback: the
# walk does this for EVERY caller, so no caller can obtain a partial set.
collect_candidate_runner(){
  [ -n "${RES_RUNNER:-}" ] || return 0
  # Registration is tested with is_registered_runner and nowhere else. An unregistered runner never
  # reaches emit_shim — validate_runner_config refuses the run before any wrapper is written — so
  # its `runners:` sub-block is inert, which is the carve-out validate_runner_shim_values already
  # made for unregistered names.
  is_registered_runner "$RES_RUNNER" || return 0
  # Deliberately NOT narrowed to the claude harness, even though emit_shim is reached only there.
  # runner_config_error's header states that `runner:`'s harness scope lives in that one function;
  # a second copy of the test here is exactly the drift that header warns about. Erring wider is
  # also the safe direction — it can only make the gate judge one more runner the user really did
  # name in an `agents:` entry, never fewer.
  case " $CANDIDATE_RUNNERS " in *" $RES_RUNNER "*) return 0;; esac
  CANDIDATE_RUNNERS="$CANDIDATE_RUNNERS $RES_RUNNER"
  return 0
}

# Guarantee CANDIDATE_RUNNERS is populated, walking only if nothing has walked yet this run.
#
# The gate ORDER at the two call sites (validate_runner_config first) is what makes this a no-op in
# practice — one walk serves both gates. That ordering is a PERFORMANCE choice and nothing else:
# this function walks on its own if it is reached first, so re-ordering the gates costs a second
# walk and changes no verdict. Skipping the walk entirely is what must never happen — an empty
# CANDIDATE_RUNNERS is a gate that judges nothing, which is fail-OPEN.
resolve_candidate_runners(){
  if [ "$CANDIDATE_RUNNERS_RESOLVED" = "1" ]; then return 0; fi
  for_each_candidate_triple noop_candidate_triple
  return 0
}
noop_candidate_triple(){ return 0; }

# --- runners.<name> shim-pin value validation (change 0269) -------------------
# runner_key's value class consumes the rest of the line, so a quoted value would ride into the
# emitted frontmatter WITH its quotes and a present-but-empty key would read as "unset" — either
# way a wrong pin persisted in a generated file. Same posture and same two-leg diagnostic split as
# validate_user_agent_values: collect every offender across every layer, report them all, and fail
# BEFORE any wrapper is written.
#
# Deliberately does NOT call runner_key: that function returns the RESOLVED value from the winning
# layer, while this gate must report every offender in every layer, including ones precedence would
# mask — a bad value shadowed today goes live the moment the higher layer is edited. That argument
# covers the LAYER WALK and only that: the per-block extraction is one shared primitive
# (runner_block_value), so the value this gate judges is by construction the value the emitter
# writes. Two spellings of the extraction is exactly the "two halves disagree silently" defect this
# change exists to remove.
#
# SCOPED TO THE RUNNERS THIS RUN ACTUALLY CONSUMES (change 0269 review). The runner dimension comes
# from resolve_candidate_runners — the same population validate_runner_config enumerates, via the
# same walk — not from REGISTERED_RUNNERS. A `runners:` sub-block for a runner no `agents:` entry
# delegates to generates nothing, exactly like the unregistered sub-block already excused above and
# like the dead-config carve-outs in validate_user_agent_values; hard-failing over it would let one
# typo'd shim_model in ~/.config/docket/config.yml refuse `sync-agents.sh` and `--check` in every
# repo on the machine, including repos with no `runners:` usage at all
# (LEARNINGS: guard-keyed-on-presence-not-provenance — a guard must key on what got USED, not on
# what a layer merely HOLDS). The gate stays fail-CLOSED for every runner that is consumed: an
# unreferenced runner is not warned-and-continued, it is not this gate's business at all, and the
# moment an `agents:` entry names it the same bad value refuses the run.
#
# The LAYER dimension stays unscoped on purpose: runner_key reads all three layers for a consumed
# runner regardless of which layer opted the repo in, so any of them can supply the winning value.
validate_runner_shim_values() {
  local rc=0 f r k rblk blk raw inline
  resolve_candidate_runners
  # Nothing delegates this run, so there is no runner dimension to iterate and nothing to judge.
  # Explicit rather than left to the inner `for r in $CANDIDATE_RUNNERS` no-op: with the `runners:`
  # read hoisted per layer (below), an empty candidate set would otherwise still parse every config
  # file once to answer a question no one asked — the common case in a repo that delegates nothing.
  [ -n "$CANDIDATE_RUNNERS" ] || return 0
  for f in "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"; do
    [ -f "$f" ] || continue
    # AGENTS.md: never `producer | early-exiting-consumer` under `set -o pipefail`. section_body's
    # awk `exit`s the moment the block ends, so piping one into the next would leave the producer
    # taking a SIGPIPE. Capture, then feed in — the same shape runner_key uses.
    #
    # Hoisted out of the runner loop: the top-level `runners:` body depends on the LAYER only, so
    # re-reading it per runner re-parsed the same file once per candidate.
    rblk="$(section_body runners < "$f")"
    [ -n "$rblk" ] || continue
    for r in $CANDIDATE_RUNNERS; do
      # FLOW STYLE IS REFUSED, NOT IGNORED (0269 whole-branch review, finding 6). section_body
      # recognizes only a BARE `<runner>:` header, so `codex: { shim_model: haiku }` yields no
      # block, no key, no gate hit and a silent fallback to inherit/low — the user configured a pin
      # and nothing anywhere told them it was dropped. That style is not exotic: the `agents:`
      # entries configured directly above it in the same file are written exactly that way.
      # Refusing is deliberate over parsing it: reading flow style here would make this gate a
      # SECOND reader of the block, disagreeing with the emitter's
      # (LEARNINGS: duplicated-gate-copies-the-whole-predicate) — unifying the two `runners:`
      # parsers is change #0256's scope, and a flow-map class belongs there if it belongs anywhere.
      #
      # The value tail comes from the SAME primitive the block reader uses, so there is one spelling
      # of "the key line and its value": a bare header returns the empty string (rc 0), a runner
      # absent from this layer returns rc 1, and only a non-empty tail is an offender. Scoped to
      # CANDIDATE_RUNNERS with the rest of the gate — a flow-style block for a runner nothing
      # delegates to is inert config, and hard-failing on it is the machine-wide refusal the
      # scoping above exists to prevent.
      if inline="$(runner_block_value "$r" <<<"$rblk")" && [ -n "$inline" ]; then
        log "runners.$r must be a block mapping — write shim_model/shim_effort as indented lines under 'runners.$r:', not inline flow style ($f)"
        rc=1
        continue
      fi
      blk="$(section_body "$r" <<<"$rblk")"
      [ -n "$blk" ] || continue
      for k in shim_model shim_effort; do
        # rc 1 = the key is absent from this block, which is normal — both knobs are optional in
        # every layer. Present-but-empty returns 0 with an empty value and IS an offender below.
        raw="$(runner_block_value "$k" <<<"$blk")" || continue
        if [ -z "$raw" ]; then
          log "runners.$r.$k is present but has no value ($f)"
          rc=1
          continue
        fi
        # AN ALLOWLIST, NOT A LIST OF BAD SHAPES (0269 whole-branch review, finding 5). The earlier
        # form named three rejected shapes — whitespace-bearing, leading-quote, and the empty case
        # above — and passed everything else into the emitted `model:` pin VERBATIM while promising
        # a bare scalar in its diagnostic. `>` and `|` (block-scalar indicators), `[a]` and `{m:x}`
        # (flow collections), `*ref` and `&anchor` (alias and anchor) and a value quoted only on the
        # RIGHT all survived it. A blocklist can only ever cover the shapes someone thought of
        # (LEARNINGS: byte-pattern-guard-matches-a-spelling), so the test is inverted: the value
        # must be spellable as the diagnostic's own remedy — unquoted and space-free.
        #
        # The class is deliberately WIDER than [A-Za-z0-9._-]: `/` and `:` are first-class in real
        # model IDs (`anthropic/claude-opus-5`, `openrouter:vendor/model` — change 0173 made exactly
        # that the rule for the child pin, and the shim pin takes an ID of the same shape), so
        # narrowing to word characters would trade this fail-open for a fail-CLOSED on legitimate
        # configuration. It still admits none of the YAML indicators above, and it covers `inherit`,
        # `auto` and every effort word.
        case "$raw" in *[!A-Za-z0-9._:/-]*)
          log "runners.$r.$k value '$raw' is not a bare scalar — write shim_model/shim_effort values unquoted and space-free ($f)"
          rc=1
          continue
          ;;
        esac
      done
    done
  done
  return $rc
}

# Log an accumulated gate diagnostic AT MOST ONCE per exact string (change 0220). Reads and appends
# to the caller's `seen` (bash dynamic scoping; bash-3.2-safe — no associative arrays).
#
# Keyed on the RENDERED DIAGNOSTIC, not on the (harness, agent) triple: a bad runner: in the GLOBAL
# layer is visible to both gate legs and produces two byte-identical lines, while two genuinely
# different offenders that happen to share a harness and agent produce two different lines and must
# both survive. Deduping by layer provenance was rejected — the gate's loops deliberately do not do
# provenance, and it would suppress a project diagnostic that merely happens to read identically.
# Suppressing a repeat never changes the caller's rc: the caller sets rc=1 unconditionally.
report_runner_error_once(){  # $1=diagnostic ; requires a caller-scoped `seen`
  # `if ... ; then ... fi`, never `grep ... && return 0`: `set -e` is active, and a failing grep as
  # the last element of an && list aborts the whole run.
  if grep -F -x -q -- "$1" <<<"$seen"; then return 0; fi
  log "ERROR $1"
  seen="$seen$1"$'\n'
  return 0
}

# Gate 3's per-triple judgement, as a for_each_candidate_triple callback. Reads and writes the
# caller-scoped `rc` and `seen` (bash dynamic scoping; bash-3.2-safe — no associative arrays), the
# same convention report_runner_error_once already uses.
check_triple_runner_config(){  # $1=harness $2=agent ; requires caller-scoped `rc` and `seen`
  local err
  if ! err="$(runner_config_error "$1" "$2" "$RES_RUNNER" "$(user_flag_model)")"; then
    report_runner_error_once "$err"; rc=1
  fi
  return 0
}

# Gate 3 (change 0207): every `runner:` rule, checked across every candidate triple, BEFORE the
# first wrapper write. Wrapper generation is atomic — a run regenerates every WRAPPER or changes no
# wrapper on disk, so a configuration error leaves the previously generated wrappers in place on
# the assumption that what was already there was working (nginx -t / nginx -s reload).
#
# "No wrapper" is the exact claim, not "nothing" (change 0220): migrate_legacy_global runs ABOVE
# this gate and has two disk effects a failing run does not undo — it renames the user's legacy
# ~/.config/docket/agents.yaml to .migrated, and it APPENDS an indented agents: block to the user's
# live global config.yml (adding a trailing newline first if that file lacked one). Nothing else on
# the failure path writes: the .gitignore write, migrate_tracked_wrappers and prune_orphans all sit
# below a passing gate.
#
# PLACEMENT IS BOUNDED ON BOTH SIDES. It must stay BELOW resolve_global_agent_harnesses —
# USER_TARGETS is not computable until the POST-migration $GLOBAL_CFG has been read, and any triple
# this gate fails to see trips emit_wrapper's assertion mid-loop, which is the original bug. It must
# stay ABOVE user_level_pass — the first `mkdir -p` or emit_wrapper redirection past this point is
# already a partial generation. (Gates 1 and 2 sit above migrate_legacy_global, so the three are
# deliberately not contiguous.)
#
# ACCUMULATES rather than short-circuits: one run names every offender, so the fix is a single edit
# and a re-run. It walks every candidate triple (for_each_candidate_triple) and lets
# runner_config_error decide applicability — narrowing to `claude` here would put the rule's scope
# in a second place, and the day that scope moves this gate would silently under-enumerate.
#
# The walk itself lives in for_each_candidate_triple because validate_runner_shim_values must judge
# the SAME population; see that function's header for why it is shared rather than copied.
validate_runner_config() {
  local rc=0 seen=""
  for_each_candidate_triple check_triple_runner_config
  return $rc
}

# --- emit a resolved wrapper to stdout ---------------------------------------
# Model/effort are the FINAL resolved values (change 0168): agents/harness-defaults.yml, not the
# source frontmatter, is the default store. This STRIPS any model:/effort: line the source still
# carries and INSERTS the resolved pair before the closing fence, so it is idempotent whether or
# not the source carries a pin — which is what lets the source cleanup later in this change be a
# pure deletion — and can never emit a duplicated key. An empty model, and an empty (or `auto`)
# effort, omit their field entirely; the harness then applies its own default.
# The pair therefore lands at the END of the frontmatter block (below any `skills:` line) rather
# than at the source's original position. YAML mapping order is not significant and no consumer
# reads these files positionally; `tests/test_sync_agents*.sh` assert the fields, not the order.
#
# `model: inherit` passes through VERBATIM here, unlike in emit_cursor_md/emit_codex_toml. It is
# not a docket sentinel on this harness: Claude Code documents `inherit` as a real frontmatter
# value meaning "run this subagent on the parent conversation's model", which is a DIFFERENT
# runtime outcome from omitting the key (Claude Code's own subagent default). Cursor and Codex
# have no such value, so their emitters normalize it to "no pin" — that asymmetry is deliberate,
# and folding it into this shared emitter silently changed Claude's resolution (0168 whole-branch
# review, IMPORTANT 2). Pinned by the `inherit:` asserts in tests/test_sync_agents.sh.
emit() {  # $1=src file  $2=model  $3=effort
  local m="$2" e="$3"
  [ "$e" = "auto" ] && e=""
  awk -v model="$m" -v effort="$e" '
    /^---[[:space:]]*$/ {
      d++
      if (d==1) { print; infm=1; next }
      if (d==2 && infm) {                             # closing fence: insert the resolved pair
        if (model!="")  print "model: " model
        if (effort!="") print "effort: " effort
        infm=0; print; next
      }
      print; next
    }
    infm && $0 ~ /^model[[:space:]]*:/  { next }      # drop any pin the source still carries
    infm && $0 ~ /^effort[[:space:]]*:/ { next }
    { print }
  ' "$1"
}

# --- shared wrapper-source parse (change 0245) -------------------------------
# The three named emitters (codex/cursor/opencode) each re-derived the same four values from the
# wrapper source with byte-identical sed/awk. A parse fix that reached one and missed its twins is
# exactly the defect class this removes (learnings: escape-ere-metacharacters-in-key).
#
# Scope is deliberately SOURCE-DERIVED FIELDS ONLY. Three things stay per-emitter and must not
# migrate here:
#   * serialization (TOML vs YAML frontmatter),
#   * the skills-preamble sentence, which differs by one phrase per harness
#     (learnings: consolidation-flattens-caller-variance — templating it flattens real variance),
#   * the `inherit`/`auto` sentinel handling, which is ASYMMETRIC BY DESIGN: codex tests
#     `!= "inherit"` at emit position, cursor/opencode normalize to empty up front, and claude's
#     emit() passes `inherit` through verbatim (0168 whole-branch review, IMPORTANT 2). Folding it
#     in here is the regression that review caught.
# emit() itself is untouched: it is a stream transform and parses no fields.
#
# Result convention is fixed globals (the RES_*/resolve_agent_layers house pattern), not stdout
# key=value (a subshell per call, and escaping a multi-line body is the fragility being removed)
# and not namerefs (bash 4.3+; docket's floor is 3.2).
parse_wrapper_source(){  # $1=src md -> sets WSRC_NAME WSRC_DESC WSRC_SKILLS_CSV WSRC_BODY
  local src="$1"
  WSRC_NAME="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$WSRC_NAME" ] || WSRC_NAME="docket-$(short_name "$src")"
  WSRC_DESC="$(agent_description "$src")"
  WSRC_SKILLS_CSV="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  WSRC_BODY="$(awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$src" | awk 'NF{p=1} p{print}')"
}

# --- per-harness emitter registry (change 0077) ------------------------------
# Map a harness token to the on-disk extension docket generates for it.
harness_ext(){ case "$1" in codex) printf 'toml';; *) printf 'md';; esac; }

# Does this harness token have a NAMED emitter, or does it fall through to the generic
# Claude-shaped one? Named once, used by both consumers — emit_for_harness's `*)` arm and
# check_project_level's advisory leg — so the two cannot drift into disagreeing about which
# tokens are supported (learnings: duplicated-gate-copies-the-whole-predicate).
harness_has_named_emitter(){  # $1=harness
  case "$1" in claude|codex|cursor|opencode) return 0;; *) return 1;; esac
}

# Space-padded list of harness tokens already warned about in THIS run. A run generates one wrapper
# per agent (16+), so a per-wrapper warn would bury the message under its own repetition; the
# emitters run in the main shell, not subshells, so a plain global is sufficient state.
WARNED_UNMAPPED=" "
warn_unmapped_harness(){  # $1=harness
  case "$WARNED_UNMAPPED" in *" $1 "*) return 0;; esac
  WARNED_UNMAPPED="$WARNED_UNMAPPED$1 "
  log "WARN harness '$1' has no named emitter — its wrappers are Claude-shaped and unverified for '$1', so the model and effort docket reports may never be honored (ADR-0060). Give '$1' its own emitter, or accept the unverified shape."
}

# Dispatch to the harness-appropriate emitter. MODEL/EFFORT are the FINAL resolved values
# (change 0168: shipped sidecar ⊕ user layers; empty => emit no pin), identical in meaning to
# emit()'s args.
emit_for_harness(){  # $1=src md  $2=harness  $3=model  $4=effort
  case "$2" in
    codex)    emit_codex_toml "$1" "$3" "$4";;
    cursor)   emit_cursor_md  "$1" "$3" "$4";;
    opencode) emit_opencode_md "$1" "$3" "$4";;
    claude)   emit            "$1" "$3" "$4";;
    # The generic Claude-shaped wrapper. A harness reaching this branch has NO verified contract
    # mapping — its wrapper is a best guess, not a supported shape. Adding a harness token here
    # without a named emitter is how the Cursor defect (change 0135) shipped: the token inherited
    # Claude's frontmatter, and docket reported pins the harness never read. Give a new harness its
    # own emitter, or accept that its wrapper is unverified.
    *)        warn_unmapped_harness "$2"; emit "$1" "$3" "$4";;
  esac
}

# Escape a value for a TOML basic (double-quoted) string: backslash then double-quote.
toml_escape_basic(){ printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'; }

# Transform a built-in markdown wrapper into a Codex TOML agent document on stdout.
# Field mapping (ADR-0015 verbatim passthrough for model/effort):
#   frontmatter name:        -> name
#   frontmatter description: -> description
#   resolved model           -> model                  (omit if empty/inherit)
#   resolved effort          -> model_reasoning_effort  (omit if empty/auto)
#   skills: preload + body   -> developer_instructions  (multi-line basic string)
emit_codex_toml(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local name desc model effort skills_csv body dev esc
  parse_wrapper_source "$src"
  name="$WSRC_NAME"
  desc="$WSRC_DESC"
  skills_csv="$WSRC_SKILLS_CSV"
  body="$WSRC_BODY"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so there is nothing to fall back to: an unresolved field means the
  # wrapper is honestly UNPINNED and Codex applies its own default.
  model="$mo"
  effort="$eo"
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

# Transform a built-in markdown wrapper into a Cursor custom-agent document on stdout (change 0135).
# Cursor documents exactly five frontmatter fields — name, description, model, readonly,
# is_background — and encodes reasoning effort INSIDE the model value (`<id>[effort=<e>]`). It has
# no standalone `effort:` field and no `skills:` preload, so the generic Claude-shaped emitter
# silently dropped all three. Field mapping (ADR-0015 verbatim passthrough for model/effort):
#   frontmatter name:        -> name
#   frontmatter description: -> description
#   resolved model + effort  -> model: <model>[effort=<effort>]   (see the table below)
#   skills: preload + body   -> a body preamble + the body verbatim
# readonly/is_background are deliberately NOT emitted: their Cursor defaults already match every
# docket agent (agents commit and push; every docket dispatch is foreground), and emitting them
# would assert a policy docket does not have.
#
#   model            effort           emitted
#   claude-opus-5    medium           model: claude-opus-5[effort=medium]
#   claude-opus-5    unset|auto       model: claude-opus-5
#   unset|inherit    unset|auto       (no model: line)
#   unset|inherit    xhigh            (no model: line) + a generation-time WARN
#
# Docket keeps NO allowlist of Cursor model IDs and NO allowlist of effort tokens: Cursor's own
# compatible-model fallback handles anything it does not recognize, and a committed table of a
# vendor's internals goes stale silently (ADR-0015; ADR-0059's rejection of vendor-internal tables).
emit_cursor_md(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local name desc model effort skills_csv body
  parse_wrapper_source "$src"
  name="$WSRC_NAME"
  desc="$WSRC_DESC"
  skills_csv="$WSRC_SKILLS_CSV"
  body="$WSRC_BODY"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so there is nothing to fall back to. An agent with no cursor entry
  # in agents/harness-defaults.yml and no user override is emitted UNPINNED — which is the point:
  # falling back to the source frontmatter is exactly how a Claude model ID leaked into a Cursor
  # wrapper that could never honor it.
  model="$mo"
  effort="$eo"
  # Normalize the two "no pin" sentinels to empty, so the emit logic below has one shape to test.
  [ "$model" = "inherit" ] && model=""
  [ "$effort" = "auto" ] && effort=""
  printf -- '---\n'
  printf 'name: %s\n' "$name"
  printf 'description: %s\n' "$desc"
  if [ -n "$model" ]; then
    if [ -n "$effort" ]; then
      printf 'model: %s[effort=%s]\n' "$model" "$effort"
    else
      printf 'model: %s\n' "$model"
    fi
  elif [ -n "$effort" ]; then
    log "WARN cursor/$name: effort '$effort' dropped — Cursor encodes effort inside the model value, and no model is resolved (either none is configured or it is the 'inherit' sentinel). Set an explicit model to pin effort on Cursor."
  fi
  printf -- '---\n\n'
  if [ -n "$skills_csv" ]; then
    printf 'Before acting, load these docket skills from your Cursor skills directory: %s.\n\n' "$skills_csv"
  fi
  printf '%s\n' "$body"
}

# Transform a built-in markdown wrapper into an opencode agent definition on stdout (change 0192).
# opencode agents are markdown with YAML frontmatter under .opencode/agents/; the FILENAME is the
# agent identifier, so no `name:` field is emitted. Field mapping (ADR-0015 verbatim passthrough):
#   frontmatter description: -> description
#   (constant)               -> mode: subagent      every docket agent is dispatched, never primary
#   resolved model           -> model               (omit if empty/inherit)
#   resolved effort          -> reasoningEffort     (omit if empty/auto)
#   skills: preload + body   -> a body preamble + the body verbatim
#
# opencode has NO first-class reasoning-effort field. It forwards unrecognized agent-frontmatter
# keys to the provider as model options, so `reasoningEffort` is a real per-agent effort rather
# than a decorative key. Re-verified against opencode 1.18.14: `opencode debug agent docket-status`
# reported `options: {reasoningEffort: "low"}` alongside
# `model: {providerID: "openrouter", modelID: "deepseek/deepseek-v4-flash-0731"}`.
#
# Effort is dropped when no model resolves. That is DOCKET's choice, and the reason is positive:
# docket refuses to pin an effort it cannot attribute to a resolved model, so a generated file never
# carries an effort whose target is unnamed. It is NOT a workaround for an opencode limitation — the
# same 1.18.14 probe showed a hand-written agent with `reasoningEffort: high` and no `model:` still
# reporting `options: {reasoningEffort: "high"}`, i.e. opencode would have honored the orphan effort.
# The drop is pinned by tests/test_sync_agents_opencode.sh's effort-drop asserts.
#
# Docket keeps NO allowlist of opencode model IDs or effort tokens (ADR-0015). IDs reached through
# OpenRouter are double-prefixed (`openrouter/<vendor>/<model>`); opencode splits that into a
# providerID and a modelID itself, so docket passes the whole string through untouched.
emit_opencode_md(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local desc model effort skills_csv body
  parse_wrapper_source "$src"
  desc="$WSRC_DESC"
  skills_csv="$WSRC_SKILLS_CSV"
  body="$WSRC_BODY"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so an unresolved field means the wrapper is honestly UNPINNED and
  # opencode applies its own default.
  model="$mo"
  effort="$eo"
  # Normalize the two "no pin" sentinels to empty, so the emit logic below has one shape to test.
  # `inherit` is a real Claude Code frontmatter value with no opencode equivalent, so it normalizes
  # here exactly as it does in emit_cursor_md/emit_codex_toml rather than passing through.
  [ "$model" = "inherit" ] && model=""
  [ "$effort" = "auto" ] && effort=""
  printf -- '---\n'
  printf 'description: %s\n' "$desc"
  printf 'mode: subagent\n'
  if [ -n "$model" ]; then
    printf 'model: %s\n' "$model"
    [ -n "$effort" ] && printf 'reasoningEffort: %s\n' "$effort"
  elif [ -n "$effort" ]; then
    log "WARN opencode/docket-$(short_name "$src"): effort '$effort' dropped — opencode carries effort as a provider model option, and no model is resolved (either none is configured or it is the 'inherit' sentinel). Set an explicit model to pin effort on opencode."
  fi
  printf -- '---\n\n'
  if [ -n "$skills_csv" ]; then
    printf 'Before acting, load these docket skills from your opencode skills directory: %s.\n\n' "$skills_csv"
  fi
  printf '%s\n' "$body"
}

# The provenance-filtered model (change 0168), read from the RES_* globals resolve_agent_layers just
# set. ONLY a user-configured value may become a child-runner flag, so a shipped
# agents/harness-defaults.yml default must read as absent here. NOT spelled once (change 0220):
# emit_wrapper deliberately keeps its own copy over positional $2, because $2 is also the
# frontmatter pin and rerouting only the flag would split the two. What keeps them from drifting is
# emit_wrapper's $2 == $RES_MODEL assertion, not a shared call — the two spellings must still agree
# exactly, or the gate passes a triple the assertion then kills.
user_flag_model(){ [ "${RES_MODEL_FROM_USER:-0}" = "1" ] && printf '%s' "${RES_MODEL:-}"; return 0; }

# The single source of truth for both `runner:` rules, their diagnostics, and their ORDER
# (registration before required-model). Emits ONE diagnostic on stdout and returns 1, or returns 0
# silently. Callers capture stdout — never let it reach emit_wrapper's stdout, which is redirected
# into the wrapper file.
#
# Scope lives HERE, not in callers: an empty runner and a non-claude harness both return "no error".
# `runner:` under a non-claude harness is currently reserved (warned and ignored, emitting native),
# which implies a future where that scope moves; keeping the test in one place means the gate and
# the assertion cannot drift apart when it does.
#
# $4 is the PROVENANCE-FILTERED model (change 0168): a shipped agents/harness-defaults.yml default
# is not a user model, so it must arrive here empty. `inherit` is docket's own no-pin sentinel —
# every adapter normalizes it to "no flag", so accepting it would leave a one-word bypass.
runner_config_error(){  # $1=harness $2=agent $3=runner $4=flag_model  (diagnostic on stdout)
  local harness="$1" agent="$2" runner="$3" flag_model="$4"
  [ -n "$runner" ] || return 0
  [ "$harness" = "claude" ] || return 0
  # Registration FIRST: an unregistered AND model-less runner must report the more specific
  # failure. tests/test_sync_agents.sh pins this with its "ORDERING FENCE" fixture.
  if ! is_registered_runner "$runner"; then
    printf '%s\n' "$harness/docket-$agent: runner '$runner' is not a registered runner (registered: $REGISTERED_RUNNERS)"
    return 1
  fi
  if [ -z "$flag_model" ] || [ "$flag_model" = "inherit" ]; then
    printf '%s\n' "$harness/docket-$agent: runner '$runner' requires an explicit model — add a 'model:' to the agents.$harness.$agent entry in a config layer, then re-run. docket never forwards its own shipped default to another harness (that ID means nothing to the child), so without one the run would silently use $runner's own default model, of unknown identity and cost."
    return 1
  fi
  return 0
}

# Emit either the native wrapper (via emit_for_harness — harness-aware, change 0077) or,
# when a runner resolved for the claude harness, the runner-delegation shim body under the
# native frontmatter (change 0079). Non-claude harness + runner => warn (reserved) and emit
# native. Both error rules for a claude runner — unregistered runner, and a registered runner
# with no USER-configured model — live in runner_config_error and are gated up front by
# validate_runner_config; the call below is only a can't-happen assertion.
#
# CALLING CONTRACT (change 0220, amended by change 0269): $2 MUST be the RES_MODEL that
# resolve_agent_layers just resolved for this exact (harness, agent) pair, and $3 the matching
# RES_EFFORT. On the native path they are the wrapper's pin directly; on the delegated path they
# reach the child ONLY as the baked --model/--effort flags, provenance-filtered through
# RES_MODEL_FROM_USER. Either way a caller that passes a post-processed model sends the wrong
# identity to the harness that runs the work, which is what the assertion at the top of the body
# exists to prevent.
#
# Change 0269 removed $2's SECOND use: the delegated shim's frontmatter pin now comes from
# `runners.<name>.shim_model`, not from $2. Both halves of a delegated wrapper are still resolved
# here, but they now answer two different questions — "what can the PARENT harness run this relay
# on" (the frontmatter, via runner_key) and "what should the CHILD run the work on" (the baked
# flag, via $2). That is why the provenance filter stays a second spelling of user_flag_model's
# rather than a call to it: the two values are no longer the same value wearing two hats, and the
# filter belongs to the flag alone.
emit_wrapper(){  # $1=src $2=model $3=effort $4=runner $5=harness $6=agent-name  (stdout)
  # Enforce the calling contract stated in the header above. ABOVE the `[ -z "$runner" ]`
  # short-circuit deliberately: the header states the contract for EVERY call, so enforcing it only
  # on the delegated path would leave the documented rule unenforced on the native one.
  if [ "$2" != "${RES_MODEL:-}" ]; then
    log "ERROR emit_wrapper called for $5/docket-$6 with model '$2', which is not the resolved RES_MODEL '${RES_MODEL:-}' — see emit_wrapper's calling contract. This is a can't-happen assertion; the run aborts here and wrappers already written this run are left in place."
    exit 1
  fi
  local runner="$4"
  if [ -z "$runner" ]; then emit_for_harness "$1" "$5" "$2" "$3"; return 0; fi
  if [ "$5" != "claude" ]; then
    log "WARN $5/docket-$6: runner: $runner is reserved for the claude parent — ignored (native dispatch)"
    emit_for_harness "$1" "$5" "$2" "$3"; return 0
  fi
  # change 0168: ONLY a user-configured value may become a child-runner flag. `runner:` hands this
  # agent to a DIFFERENT harness's CLI, so the baked flags are read by that child. A shipped
  # agents/harness-defaults.yml entry is a default for THIS harness; it is not evidence that the
  # same ID means anything to a Codex or Cursor child, and baking it would send e.g. a Claude model
  # ID to a Codex process. A runner-only override therefore bakes no flag and lets the child pick
  # its own default. The provenance flags are set by resolve_agent_layers, which every emit_wrapper
  # call site invokes for this same (harness, agent) immediately beforehand.
  local flag_model="" flag_effort=""
  [ "${RES_MODEL_FROM_USER:-0}" = "1" ]  && flag_model="$2"
  [ "${RES_EFFORT_FROM_USER:-0}" = "1" ] && flag_effort="$3"
  # change 0205: a delegated agent MUST carry a user-configured model. Runner-wide, not per-adapter
  # — "is a model required?" must not be an adapter-by-adapter fact a user learns twice. Because a
  # shipped default is never forwarded (the provenance rule directly above), a model-less
  # delegation ran on the CHILD's own default: unknown identity, and on a pay-per-token backend
  # like OpenRouter unknown cost, with the failure surfacing on the bill rather than in the run.
  # `inherit` is docket's own no-pin sentinel — every adapter normalizes it to "no flag", so
  # accepting it here would leave a one-word bypass. Raised at generation time, where the config
  # was just written; the ordering now lives in runner_config_error.
  #
  # Can't-happen assertion, not the user-facing mechanism: validate_runner_config gates every
  # triple before the first wrapper write. This covers a FUTURE call site added without that gate —
  # there are three today (user_level_pass, project_level_pass, check_project_level's leg (c)) and
  # nothing structurally prevents a fourth. The three are gated consistently BECAUSE the gate's
  # project-level leg and leg (c) share project_wrappers_generated (change 0220) — not as a
  # coincidence of two predicates that happen to agree. A fourth call site must adopt that
  # predicate too, or re-open the gap 0220 closed. Reaching it means the gate under-enumerated, so
  # it dies loudly rather than emitting a wrapper the config says must not exist.
  local rc_err
  if ! rc_err="$(runner_config_error "$5" "$6" "$runner" "$flag_model")"; then
    log "ERROR $rc_err"
    exit 1
  fi
  # change 0269: $2/$3 are the CHILD's resolved pin and reach the shim only as the baked flags
  # above. The shim's own frontmatter gets the parent-side knobs, defaulting to `inherit`/`low`.
  # `inherit` is deliberate: emit() passes it through VERBATIM on the claude harness (Claude Code
  # documents it as "run on the parent conversation's model", a real value distinct from omitting
  # the key), so every currently-broken wrapper is repaired by regeneration alone with no config
  # edit. The knob is a cost optimization on top, never a prerequisite.
  # Both pins (and both defaults) come from resolve_shim_pins, which memoizes them per runner for
  # the run; see its header. It sets SHIM_MODEL/SHIM_EFFORT in the CALLER's shell, so it must not be
  # wrapped in a command substitution here.
  resolve_shim_pins "$runner"
  emit_shim "$1" "$SHIM_MODEL" "$SHIM_EFFORT" "$runner" "$6" "$flag_model" "$flag_effort"
}

# The shim: native frontmatter carrying the SHIM'S OWN pin (change 0269 — this agent runs in the
# claude parent and drives the facade plus a stdout relay, so its pin must name
# something the PARENT can resolve; the pin for the delegated work is the baked --model argument),
# body = the launch-then-observe facade loop (change 0271) + relay + verify rules. Both the
# `--launch` and the `--observe` calls are the same seam, so ADR-0038's chokepoint is intact:
# two invocations, one facade, still no inline fallback and no silent retry.
# The baked flags come from $6/$7, which
# carry USER-configured values only (change 0168); an empty one bakes NO flag, so the child harness
# applies its own default rather than inheriting a default that was only ever meant for this
# harness. This function stays a pure emitter — its caller resolves both pins and hands them down.
# The brief slot is baked UNBRACKETED and carries its own emphatic rule, exactly like the
# `<feature worktree>` slot below. Both are required inputs the model must fill from its caller,
# and both fail SILENTLY when omitted — a task-less child improvises from the worktree and the
# dispatch still looks successful. The earlier `[-- <caller args>]` spelling read as optional and
# was in fact dropped on live dispatches, so an optional-looking rendering of a required slot is a
# defect here, not a style choice (change 0271).
# Change 0277: the brief travels as a FILE, not as shell argv. The old form asked the model to
# perform a lossy, unverified transformation — quote a multi-line brief into one shell argument,
# every time, correctly — and the adapters then joined multiple arguments on whitespace. A
# quoted-delimiter heredoc removes the entire quoting burden, and the facade refuses a payload-less
# `build-*` dispatch outright, so the omission mode is now partly mechanical rather than only
# narrated. ONE path is taught: two would let the model pick the lossy one.
#
# THE WRITE AND THE LAUNCH ARE ONE BASH CALL, and that is load-bearing, not layout (0277 review).
# Harness Bash calls do not share shell state, and mktemp's suffix is random, so a recipe split
# across two calls never surfaces the path at all: the launch expands an unset $BRIEF to the empty
# string and the facade dies on `--brief-file requires a path` — a total failure of the SOLE taught
# channel. Keeping them together also removes the substitution itself: `--brief-file "$BRIEF"` is
# not a slot the model fills, so there is no per-dispatch transformation left on the brief argument
# to get wrong, which is this change's whole thesis applied to the path as well as the text. The
# angle-bracket slots that remain (the task text, and `<feature worktree>` on a build shim) are
# inputs only the CALLER can supply — those cannot be mechanized away, so they stay unbracketed-
# looking-required and emphatic per 0271. Guarded end-to-end in tests/test_sync_agents_runners.sh,
# which executes the emitted recipe against a stub facade and asserts a readable brief arrives.
emit_shim(){  # $1=src $2=shim-model $3=shim-effort $4=runner $5=agent-name $6=flag-model $7=flag-effort  (stdout)
  emit "$1" "$2" "$3" | awk '/^---[[:space:]]*$/{d++; print; next} d<2{print}'
  local flags="--runner $4 --agent $5"
  [ -n "${6:-}" ] && flags="$flags --model $6"
  [ -n "${7:-}" ] && [ "${7:-}" != "auto" ] && flags="$flags --effort $7"
  # change 0206, generalized by change 0208: a FEATURE-SCOPED worker must run INSIDE the worktree it
  # serves, on that branch. Keyed on the source's DECLARED `worktree-scope:` ($1 is the source file)
  # rather than on a `build-*` name shape — `rebase-resolver`, `integration-repair` and the three
  # `review-*` rungs are equally feature-scoped and match no build shape, and a second name list
  # here would be the twin of the facade's that drifts
  # (LEARNINGS: duplicated-gate-copies-the-whole-predicate). A metadata-scoped shim stays
  # byte-identical, which keeps 0206's bidirectional guard intact — now scope-keyed.
  local wt_slot="" wt_rule=""
  case "$(agent_worktree_scope "$1")" in
    feature)
      wt_slot=" --worktree <feature worktree>"
      wt_rule="This agent is FEATURE-SCOPED: it must run INSIDE the feature worktree it serves, on
that worktree's branch — never the main tree on the integration branch. Replace \`<feature worktree>\`
with the absolute path of the feature worktree your caller named (drop the angle brackets). If your
caller named no worktree, abort-and-report — never guess a path, and never omit the flag."
      ;;
  esac
  cat <<SHIM
This agent is DELEGATED to the \`$4\` runner (cross-harness runner delegation, change 0079).
Do NOT execute the skill inline and do NOT load its skills yourself.

The delegated run MAY OUTLIVE the call that starts it, so this is a launch-then-observe
dispatch, not a single blocking call (change 0271). Both steps go through the same facade —
one dispatch seam, no inline fallback, no silent retry.

STEP 1 — write the brief and launch. ONE Bash call containing BOTH commands below, in this
order. They must travel together: Bash calls do not share shell state, so a launch sent as a
call of its own expands \$BRIEF to nothing and the dispatch is refused.

    BRIEF="\$(mktemp "\${TMPDIR:-/tmp}/docket-brief.XXXXXX")"
    cat > "\$BRIEF" <<'DOCKET_BRIEF_EOF'
<THE TASK TEXT YOUR CALLER GAVE YOU>
DOCKET_BRIEF_EOF
    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch --launch $flags$wt_slot --brief-file "\$BRIEF"

Fill in the angle-bracket slots (drop the brackets) and copy every other character verbatim.
\`--brief-file "\$BRIEF"\` is already complete — it is not a slot, you never write out a brief
path yourself, and the variable resolves only because the launch rides in the same call as the
write above.

The quoted delimiter makes every character between the two DOCKET_BRIEF_EOF lines literal:
nothing is expanded, nothing needs escaping, and no quote inside the text needs any handling
at all. Paste your caller's task text IN FULL — every plan task, change id, path, and resume
note — copied verbatim, never summarized, never trimmed, and never reworded to make quoting
easier. If the text itself contains a line reading exactly DOCKET_BRIEF_EOF, pick a different
delimiter; never trim the text to avoid it.

The brief file is the ONLY way the child learns what to do. It inherits no conversation, no
plan, and no task from you: what is in that file is all it will ever see. Drop the
\`--brief-file\` argument ONLY when your caller handed you no task text at all — and for a
build worker the facade will refuse that dispatch outright. Getting this wrong FAILS SILENTLY:
a child launched with no task does not error, it improvises from whatever it can see in the
worktree and the dispatch still looks successful. Before you send the call, re-read it and
confirm the heredoc holds your caller's full task text.

The launch detaches the child and returns immediately, printing a DISPATCH KEY on stdout. A
non-zero exit here is a failed launch: abort-and-report its stderr diagnostic.

STEP 2 — observe. Using that key, make repeated SHORT foreground Bash calls:

    "\${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch --observe <key> $flags$wt_slot

Read the EXIT CODE, not the prose. The observe call's own STDOUT is the child's relayed
output, verbatim; every diagnostic is on stderr, and a still-running observation relays
nothing:
  - 4 — still running. This is NOT a failure. Observe again; keep going until another code.
  - 0 — the run completed. Relay that observe call's stdout as your result, and verify its
        contract exactly as a native caller would: git state on origin/docket for
        state-contract agents (status, adr); the relayed report for in-context-report agents.
  - any other non-zero — the run failed, halted for a human, or its result is unavailable.
        Abort-and-report its stderr diagnostic, plus any relayed stdout it printed.

PACE THE OBSERVATIONS. A delegated run takes minutes to tens of minutes, so put a BLOCKING
WAIT of roughly a minute before every observe after the first — whichever blocking wait your
harness offers (a sleep command if it permits one, otherwise its wait-for-condition helper).
Never yield to your caller for it. Back-to-back observe calls burn your context on identical
"still running" answers and tell you nothing a paced pass would not.

YOUR OWN BOUND, independent of the facade's. The facade enforces the observation budget
whenever it can measure elapsed time, and when it cannot — an unreadable clock, an
unreadable launch record — it says so on stderr as \`budget not enforced this pass\` and
gives up on its own after a few consecutive such passes. That is a narrow guarantee, not a
promise that every loop ends: if you have seen roughly 60 observations, or 5 in a row
carrying a \`budget not enforced\` diagnostic, STOP and abort-and-report the last stderr
diagnostic plus the dispatch key. Never loop indefinitely on 4.

Block on each observe call and never yield between them — never hand control back to your
caller mid-run.
Never retry a failed dispatch silently, and never run the skill inline on this harness
as a fallback.
SHIM
  # `if … fi`, never `[ -n "$wt_rule" ] && printf …`: as the function's last command the && form
  # returns 1 for a non-build shim, and emit_wrapper returns emit_shim's status. Emitted OUTSIDE
  # the heredoc so a non-build shim gains no trailing blank line (byte-identical output).
  if [ -n "$wt_rule" ]; then printf '\n%s\n' "$wt_rule"; fi
}

# The caller-side run gate (change 0242). ONE source — cursor-rules/run-gate.md — rendered verbatim
# into every parent-facing surface: the Cursor rule, the committed AGENTS.md block, and (change
# 0242) the Claude surface, which inherits it through the AGENTS.md block assembler. The gate is a
# whole predicate, not a threshold: a second hand-written copy would agree on the ordinary case and
# diverge on exactly the halted/incomplete states it exists to distinguish (LEARNINGS
# duplicated-gate-copies-the-whole-predicate). Printed with no surrounding blank line; each caller
# owns its own spacing.
assemble_run_gate() { cat "$CURSOR_RULES_SRC/run-gate.md"; }

# Assemble the Cursor dispatch rule to stdout: static head + the run gate + one subsection per
# built-in agent (glob order). A built-in agent with a fragment uses it verbatim; one without gets a
# minimal auto-block derived from its description + a warning (a new agent is never silently
# un-dispatched).
assemble_dispatch_rule() {
  cat "$CURSOR_RULES_SRC/dispatch.head.md"
  printf '\n'
  assemble_run_gate
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
      printf 'When this applies, do NOT run the skill inline. Dispatch to the subagent `docket-%s` using this mode'"'"'s subagent-launch mechanism, foreground, and relay its result.\n' "$name"
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
# Machine-neutral: agent names + delegation prose only, NO model IDs (pins live in each harness's
# own generated agent definitions).
#
# This block is COMMITTED into consumer repos and checked by `--check`, so a false claim here ships
# rather than merely displaying. It is SHARED by every harness in AGENTS_MD_DISPATCH_HARNESSES
# (codex and opencode), so its prose names no harness's artifact path and no harness's model
# vocabulary — a claim true for Codex only would be false in an opencode repo. The default store is
# agents/harness-defaults.yml, whose blocks are complete for every shipped harness, so the head
# states the pinned truth — and names that roster by interpolating HD_SHIPPED_HARNESSES, so a fifth
# shipped harness cannot leave a stale hand-list behind. The dispatch is required either way — the
# agent carries the skill's contract and preload, not just a model. Guarded, against the sidecar rather than a literal, in
# tests/test_sync_agents_codex_dispatch.sh and tests/test_sync_agents_opencode.sh.
assemble_agents_md_dispatch(){
  printf '%s\n' "$DISPATCH_START"
  # The shipped-harness roster is DERIVED from HD_SHIPPED_HARNESSES, never hand-listed: this head is
  # committed into consumer repos and --check-enforced, so a literal would ship a false claim the day
  # a fifth harness starts shipping defaults. Rendered as the lowercase harness tokens themselves —
  # a capitalized restatement alongside it would be exactly the literal this avoids. The heredoc is
  # unquoted so this interpolates; its body carries no $, backtick, or backslash.
  local shipped_list
  shipped_list="$(printf '%s' "$HD_SHIPPED_HARNESSES" | sed 's/ /, /g')"
  cat <<HEAD
## Docket agents — dispatch, don't run inline

Docket generates an agent definition per docket skill in your harness's own agents directory. When
you are asked to run one of the docket skills below, run the matching **agent** instead of executing
the skill inline at the session model: the agent carries that skill's dispatch contract, its skill
preload, and whatever model and reasoning effort your config layers pin for it. Docket ships a
validated model and reasoning effort for every one of these agents on the harnesses it ships
defaults for — $shipped_list — so they are pinned out of the box there; your
config layers override either field per agent, and set them for any other harness. Dispatch through
the hosting harness's native named-agent dispatch either way — the pin is not the only reason, since
the agent also carries the skill's dispatch contract and preload. Pass the request through
unchanged, including any change or ADR id.
HEAD
  printf '\n'
  # The roster follows its OWN head, and the gate comes after both. This is a markdown document
  # with headings, so order is structure: with `## Run gate` spliced between the head and the
  # bullets, the head's "run one of the docket skills below" pointed across an unrelated section and
  # a reader sectioning the file read the agent roster as part of the gate (change 0242 review,
  # finding 10). The Cursor assembler keeps the opposite order on purpose — there each agent
  # fragment carries its own `## docket-<name>` heading, so the gate is not sitting on top of an
  # unheaded list. The two surroundings genuinely differ; flattening them into one shared assembler
  # is what caused an earlier finding on this branch.
  local src name desc
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    printf '%s\n' "$src"
  done | LC_ALL=C sort | while IFS= read -r src; do
    name="$(short_name "$src")"
    desc="$(agent_description "$src")"
    printf -- '- **docket-%s** — %s Delegate to the `docket-%s` agent.\n' "$name" "$desc" "$name"
  done
  printf '\n'
  assemble_run_gate
  printf '%s\n' "$DISPATCH_END"
}

# Write the managed dispatch block into every parent-facing surface this repo targets — the
# committed AGENTS.md for codex/opencode (change 0077) and the Claude surface (change 0242) — ONCE
# PER DISTINCT PHYSICAL FILE, then strip it from every surface no harness targets any more.
#
# The physical-path set is the whole design. A CLAUDE.md symlinked to AGENTS.md is ONE file wearing
# two names, so it must be decided once and acted on once (LEARNINGS
# decide-and-act-on-the-same-copy). Both passes consult the SAME `seen` set, which is what keeps the
# strip half safe: a name that is merely an ALIAS of a live surface — a user's own CLAUDE.md ->
# AGENTS.md link in a codex-only repo — resolves into `seen` and is skipped, instead of being
# stripped straight through the link and deleting the live block from the file it points at.
# The strip pass only ever removes; `remove_managed_block` no-ops on a file with no block, so a
# surface docket does not own is read and left exactly as it was.
sync_dispatch_surfaces(){
  local block targets="" seen="" f phys status root
  block="$(assemble_agents_md_dispatch)"
  root="$(repo_physical_root)"

  if repo_wants_agents_md_dispatch; then targets="$REPO/AGENTS.md"; fi
  if repo_wants_claude_surface; then
    # claude_surface_target is called for its SIDE EFFECT — creating the surface when absent — and
    # its resolved answer is discarded in favour of the literal spelling, which the loop below
    # resolves identically (resolution is idempotent). Two reasons: the diagnostics then name the
    # file the user actually has rather than printing "X resolves to X", and the target list becomes
    # the same pair of literals the --check twin walks.
    claude_surface_target >/dev/null && targets="$targets${targets:+$'\n'}$REPO/CLAUDE.md"
  fi

  # Write pass — deduped by physical path.
  if [ -n "$targets" ]; then
    while IFS= read -r f; do
      [ -n "$f" ] || continue
      phys="$(resolve_physical_path "$f")"
      case "$seen" in *"|$phys|"*) continue;; esac
      # `seen` records every DECIDED path, refusals included, so a refusal warns once and the strip
      # pass below does not revisit — let alone strip — a surface this pass declined to write.
      seen="$seen|$phys|"
      if ! path_inside_repo "$phys" "$root"; then
        log "WARN $f resolves to $phys, outside this repository ($root) — SKIPPED: docket writes its dispatch block only inside the checkout it was run in. Repoint or remove that symlink to get the block."
        continue
      fi
      status="$(ensure_managed_block "$phys" "$DISPATCH_START" "$DISPATCH_END" "$block")"
      case "$status" in
        wrote)   log "wrote/updated the docket dispatch block in $phys — COMMIT THIS (machine-neutral; no model IDs).";;
        refused) log "WARN $phys has a malformed docket:dispatch block — refusing to rewrite; repair the markers by hand and re-run.";;
        # The write was ATTEMPTED and the shell could not open the target (read-only mount,
        # permission denied, a link resolving onto a directory, a dangling link whose parent dir is
        # missing — all reachable now that this pass follows symlinks). Announcing the write here
        # would send the user to commit bytes that were never written, so the failure is reported as
        # itself. The shell's own "cannot create" diagnostic has already gone to stderr above.
        failed)  log "WARN could not write the docket dispatch block to $phys — the target is not writable (see the shell diagnostic above); this run changed nothing there.";;
      esac
    done <<<"$targets"
  fi

  # Strip pass — a surface whose harness is no longer targeted loses its block. Existing files only:
  # this pass must never bring a surface into being.
  for f in "$REPO/AGENTS.md" "$REPO/CLAUDE.md"; do
    [ -e "$f" ] || continue
    phys="$(resolve_physical_path "$f")"
    case "$seen" in *"|$phys|"*) continue;; esac
    seen="$seen|$phys|"
    if ! path_inside_repo "$phys" "$root"; then
      log "WARN $f resolves to $phys, outside this repository ($root) — SKIPPED: docket strips its dispatch block only inside the checkout it was run in. Remove the block by hand if that file should not carry one."
      continue
    fi
    status="$(remove_managed_block "$phys" "$DISPATCH_START" "$DISPATCH_END")"
    case "$status" in
      removed) log "removed the docket dispatch block from $phys (no dispatch harness targets it) — COMMIT THIS."
               # A surface docket itself seeded (claude_surface_target's "neither surface existed"
               # arm creates a real, empty CLAUDE.md) holds nothing but the block, so the strip
               # leaves a lone newline: a file docket created and now nobody owns, invisible to
               # tracked_docket_files because it must stay committable. ADVISE, never delete: at
               # strip time there is no record of who created the file, and the same empty shape is
               # reachable for a file the USER created and emptied — deleting on a guess destroys
               # bytes docket does not own, which is the one failure this whole pass is built to
               # avoid (change 0242 review, finding 9). Whitespace-only, not `! -s`: the strip
               # writes a newline, so the byte count is 1 rather than 0.
               # Spelled as an `if`, never `test && log`: this is the last command in the arm, so a
               # false test would make the whole case exit 1 and trip the script's errexit.
               if [ -z "$(tr -d '[:space:]' < "$phys")" ]; then
                 log "note: $phys is now EMPTY — the dispatch block was all it held. Docket does not delete it (it cannot tell a file it seeded from one you emptied); delete it by hand if you do not want it."
               fi
               ;;
      refused) log "WARN $phys has a malformed docket:dispatch block — refusing to strip; repair the markers by hand.";;
    esac
  done
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

# Non-fatal footgun warning for a NON-claude harness wrapper with no harness-specific value —
# neither a shipped agents/harness-defaults.yml entry nor an agents.<harness> override. Since
# change 0168 that pair is generated UNPINNED (the source frontmatter is no longer a default store,
# so there is nothing left to leak); or, if only agents.default supplied a model, it carries an ID
# that may be meaningless to that harness (ADR-0015: some harnesses silently run their house
# default on an unknown model). Never an error; sync still succeeds. Scoped to non-claude — the
# claude sidecar values ARE Claude IDs.
warn_fallback_model(){  # $1=harness $2=agent ; consumes RES_MODEL_FROM_HARNESS / _FROM_SIDECAR / RES_MODEL
  [ "$1" = "claude" ] && return 0
  [ "$RES_MODEL_FROM_HARNESS" = "1" ] && return 0
  # Silence requires that the SIDECAR SUPPLIED the value, not merely that it holds an entry for the
  # pair. Testing existence instead silences the warning exactly when it is most needed: a user
  # `agents.default` line outranks the sidecar, so the wrapper is emitted with the foreign ID while
  # the guard reports the shipped default that never applied. Latent in change 0168 (it could only
  # bite cursor's three build workers); live once a harness ships a complete block.
  [ "$RES_MODEL_FROM_SIDECAR" = "1" ] && return 0
  if [ -z "${RES_MODEL:-}" ]; then
    log "WARN $1/docket-$2: no harness-specific model — generated unpinned; harness '$1' will apply its own default. Set agents.$1.$2.model to pin it."
  else
    log "WARN $1/docket-$2: model '$RES_MODEL' came from agents.default; may not be a valid model ID for harness '$1'."
  fi
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
      emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$dir/docket-$name.$(harness_ext "$harness")"
    done
  done
  # Cursor-only dispatch rule, user-level, for each targeted dispatch-rule harness.
  local drh
  for drh in $HARNESS_HAS_DISPATCH_RULES; do
    case " $USER_TARGETS " in *" $drh "*) write_dispatch_rule "$HARNESS_ROOT/.$drh" ;; esac
  done
}

project_level_pass() {  # built-in ⊕ local ⊕ committed ⊕ global -> <repo>/.<H>/agents for each H in HARNESSES
  if ! project_wrappers_generated; then
    # #0082: a user-level agent_harnesses cannot drive per-repo generation — per-repo targeting is
    # deliberately repo-owned so the committed artifacts stay deterministic across every clone
    # (ADR-0019's coordination-key fence; change 0050). What was wrong was the SILENCE: the user
    # set a knob, ran the tool, and got neither wrappers nor a word. Generation path only — one
    # authoritative copy of the hint, at the moment the user acted and the no-op bit.
    if [ "${USER_HARNESSES_SET:-0}" = "1" ]; then
      log "global agent_harnesses is set ($GLOBAL_CFG) but this repo has not opted in, so no per-repo wrappers were generated. To opt in, add 'agent_harnesses:' to .docket.local.yml (machine-local) or .docket.yml (committed)."
    fi
    return 0
  fi
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
      emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$dir/docket-$name.$(harness_ext "$harness")"
    done
  done
  # Cursor-only dispatch rule, per-repo (committed) when cursor is a targeted harness.
  local h
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    write_dispatch_rule "$REPO/.$h"
  done
  # Every parent-facing dispatch surface: the committed AGENTS.md shared by every
  # AGENTS_MD_DISPATCH_HARNESSES harness (changes 0077, 0192) and the Claude surface (change 0242),
  # written once per distinct physical file and stripped from whatever no harness targets.
  sync_dispatch_surfaces
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
  # Dispatch-surface currency (changes 0077, 0242) — CI-meaningful, symmetric with the .gitignore
  # leg. The blocks are committed (exempt from the tracked-file leg).
  #
  # This is a CORRESPONDENCE guard, so it is written as the read-only twin of sync_dispatch_surfaces
  # and mirrors BOTH of its halves against one shared `seen` set: a write half (every targeted
  # surface must carry the current block) and a strip half (every OTHER existing surface must carry
  # none). Mirroring only the first half would certify a repo whose Claude surface had drifted, and
  # mirroring the second half only when *no* harness is targeted would miss the commonest de-list —
  # dropping `claude` from a repo that still targets codex, where the write pass strips a distinct
  # CLAUDE.md that this leg would never have looked at (LEARNINGS correspondence-guard-runs-one-way).
  #
  # Read-only by construction: the targets are spelled as literal paths and resolved with
  # resolve_physical_path, never via claude_surface_target — that resolver CREATES the surface as a
  # side effect, and `--check` must not write. The cost is exact: a claude-targeted repo with no
  # CLAUDE.md reads as an empty current block, which is correctly stale rather than silently healed.
  #
  # Gated on project_wrappers_generated — the SAME predicate project_level_pass writes the surfaces
  # under, for the same reason leg (c) is. gitignore_block_wanted (checked above) is strictly
  # weaker: a docket branch alone satisfies it, and in such a repo HARNESSES falls back to its
  # default, which contains `claude` — so an ungated leg would demand a CLAUDE.md from every
  # non-opted-in repo while the write pass returns before creating one.
  #
  # Containment is mirrored too, for the same reason the halves are: a surface resolving OUTSIDE the
  # checkout is one sync_dispatch_surfaces refuses to touch, so reporting it stale would demand a
  # write that `bash sync-agents.sh` will never perform — a red CI leg with no green path out.
  local am_want am_have phys targets="" seen="" f root
  root="$(repo_physical_root)"
  if project_wrappers_generated; then
    am_want="$(assemble_agents_md_dispatch)"
    repo_wants_agents_md_dispatch && targets="$REPO/AGENTS.md"
    repo_wants_claude_surface && targets="$targets${targets:+$'\n'}$REPO/CLAUDE.md"
    # Write half — every targeted surface carries the current block, once per physical file.
    if [ -n "$targets" ]; then
      while IFS= read -r f; do
        [ -n "$f" ] || continue
        phys="$(resolve_physical_path "$f")"
        case "$seen" in *"|$phys|"*) continue;; esac
        seen="$seen|$phys|"
        if ! path_inside_repo "$phys" "$root"; then
          log "check: $f resolves to $phys, outside this repository ($root) — not checked; the sync refuses to write there."
          continue
        fi
        if [ "$am_want" != "$(_docket_gi_current_block "$phys" "$DISPATCH_START" "$DISPATCH_END")" ]; then
          log "check: the docket dispatch block in $phys is missing or stale — run: bash sync-agents.sh and commit it"
          rc=1
        fi
      done <<<"$targets"
    fi
    # Strip half — every OTHER existing surface carries none. `seen` spares a mere ALIAS of a live
    # surface (a user's CLAUDE.md -> AGENTS.md link in a codex-only repo), exactly as the write pass
    # does, so this reports only what a real run would actually change.
    for f in "$REPO/AGENTS.md" "$REPO/CLAUDE.md"; do
      [ -e "$f" ] || continue
      phys="$(resolve_physical_path "$f")"
      case "$seen" in *"|$phys|"*) continue;; esac
      seen="$seen|$phys|"
      if ! path_inside_repo "$phys" "$root"; then
        log "check: $f resolves to $phys, outside this repository ($root) — not checked; the sync refuses to strip there."
        continue
      fi
      am_have="$(_docket_gi_current_block "$phys" "$DISPATCH_START" "$DISPATCH_END")"
      if [ -n "$am_have" ]; then
        log "check: $phys carries a docket dispatch block but no harness in agent_harnesses targets it as a dispatch surface (claude, $(printf '%s' "$AGENTS_MD_DISPATCH_HARNESSES" | sed 's/ /, /g')) — run: bash sync-agents.sh and commit it"
        rc=1
      fi
    done
  fi
  # committed-config shape (the committed .docket.yml is CI-visible): legacy bare agent keys.
  legacy="$(legacy_agent_keys "$DOCKET_YML" 1)"
  if [ -n "$legacy" ]; then
    log "check: legacy bare-agent-key agents: shape ($(printf '%s' "$legacy" | tr '\n' ' ')) — reshape to agents.default.<agent> (run: bash sync-agents.sh)"
    rc=1
  fi
  # Unmapped-harness advisory (change 0245). ADVISORY: reported, never fails CI — existing repos
  # that list such a token today keep working, and hard refusal is deliberately out of scope. Same
  # substance as the generation-time WARN, same predicate; a token may surface twice on this path
  # (leg (c)'s emit_wrapper reaches the `*)` arm's own once-per-harness WARN), which is accepted —
  # each is deduped, and suppressing one would couple the legs.
  local uh
  for uh in $HARNESSES; do
    harness_has_named_emitter "$uh" && continue
    log "advisory: harness '$uh' has no named emitter — its wrappers are Claude-shaped and unverified for '$uh' (ADR-0060). Not a check failure."
  done
  # leg (c) — local staleness (ADVISORY: reported, never fails CI; vacuous on a fresh clone).
  # Gated on project_wrappers_generated, the SAME predicate project_level_pass writes under. Two
  # reasons, one predicate: (1) this loop calls emit_wrapper, so a triple the gate skipped would
  # die here on the can't-happen assertion (change 0220); (2) diffing against wrappers this repo
  # never generates produced a "not generated on this machine" advisory for every agent, which was
  # simply false. A repo that WAS opted in, generated wrappers, then dropped its key keeps those
  # wrappers and stops having them diffed — accepted, because prune_orphans' per-repo legs are
  # gated on this same predicate, so leg (c) was the last survivor of a boundary drawn elsewhere.
  if ! project_wrappers_generated; then
    return $rc
  fi
  local src name got tmp d harness
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/sync-agents.XXXXXX")"
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent_layers "$harness" "$name" "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"
      local ext; ext="$(harness_ext "$harness")"
      emit_wrapper "$src" "$RES_MODEL" "$RES_EFFORT" "$RES_RUNNER" "$harness" "$name" > "$tmp/docket-$name.$ext"
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
  rule_tmp="$(mktemp "${TMPDIR:-/tmp}/sync-agents.XXXXXX")"
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
  if project_wrappers_generated; then
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
  if project_wrappers_generated; then
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
    # The sidecar is validated on this path too (0168 whole-branch review, MINOR 1). --check
    # returned before the gate below, so CI could pass against a sidecar the next real run would
    # refuse — the one place a repo would rather learn about it. It costs one pass over a shipped
    # file docket owns; a valid sidecar changes nothing about --check's outcome.
    if ! validate_harness_defaults "$HARNESS_DEFAULTS" "$AGENTS_SRC"; then
      log "check: agents/harness-defaults.yml is invalid — a real run would refuse to write wrappers."
      exit 1
    fi
    if ! validate_agent_scopes "$AGENTS_SRC"; then
      log "check: an agent source declares no valid worktree-scope — a real run would refuse to write wrappers."
      exit 1
    fi
    if ! validate_user_agent_values; then
      log "check: user agent config has unconsumable values — a real run would refuse to write wrappers."
      exit 1
    fi
    # These two legs read PRE-migration config — the same asymmetry the two gates above already
    # have. They match what check_project_level's leg (c) drift loop resolves, and since change 0220
    # they are gated by the SAME predicate (project_wrappers_generated), so neither gate can see
    # fewer triples than that loop later emits. Before 0220 the gate used per_repo_opted_in while
    # leg (c) used the strictly weaker gitignore_block_wanted, and a global agent_harnesses: list
    # omitting claude let a bad claude runner: reach leg (c)'s emit_wrapper unchecked.
    #
    # ORDER (change 0269 review): validate_runner_config first, so its walk is the one that
    # populates CANDIDATE_RUNNERS and the shim gate does not repeat it. Performance only — see
    # resolve_candidate_runners; either order reaches the same verdicts.
    if ! validate_runner_config; then
      log "check: runner configuration is invalid — a real run would refuse to write wrappers."
      exit 1
    fi
    if ! validate_runner_shim_values; then
      log "check: runner shim-pin configuration is invalid — a real run would refuse to write wrappers."
      exit 1
    fi
    if check_project_level; then exit 0; else exit 1; fi
  fi

  # The sidecar is required program data (change 0168). Validate BEFORE writing any wrapper, so a
  # malformed file cannot leave a half-regenerated agent directory behind.
  if ! validate_harness_defaults "$HARNESS_DEFAULTS" "$AGENTS_SRC"; then
    log "ERROR agents/harness-defaults.yml is missing or invalid — no wrappers were written."
    exit 1
  fi

  if ! validate_agent_scopes "$AGENTS_SRC"; then
    log "ERROR an agent source declares no valid worktree-scope — no wrappers were written."
    exit 1
  fi

  # Same gate for USER config (change 0173): validate before writing any wrapper. This must stay
  # ABOVE migrate_legacy_global/user_level_pass — the first `mkdir -p` or emit_wrapper redirection
  # past this point is already a partial generation.
  if ! validate_user_agent_values; then
    log "ERROR user agent config has unconsumable values — no wrappers were written."
    exit 1
  fi

  migrate_legacy_global
  resolve_global_agent_harnesses
  # Gate 3 — see validate_runner_config. Must stay BELOW resolve_global_agent_harnesses (USER_TARGETS
  # needs the post-migration $GLOBAL_CFG) and ABOVE user_level_pass (the first mkdir -p or
  # emit_wrapper redirection past this point is already a partial generation). The shim-pin gate
  # shares both bounds — it walks the same candidate triples — and runs SECOND so that Gate 3's walk
  # is the one that populates CANDIDATE_RUNNERS (performance only; see resolve_candidate_runners).
  if ! validate_runner_config; then
    log "ERROR runner configuration is invalid — no wrappers were written."
    exit 1
  fi
  if ! validate_runner_shim_values; then
    log "ERROR runner shim-pin configuration is invalid — no wrappers were written."
    exit 1
  fi
  user_level_pass
  migrate_tracked_wrappers
  if gitignore_block_wanted; then ensure_docket_gitignore_block "$REPO"; fi
  project_level_pass
  prune_orphans all
  log "done"
fi
