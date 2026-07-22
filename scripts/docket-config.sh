#!/usr/bin/env bash
# scripts/docket-config.sh — deterministic resolver for docket's startup config + bootstrap
# guard (change 0026). Emits eval-able KEY=value lines a skill consumes in one turn:
#   eval "$(scripts/docket-config.sh --export)"
# Read-only by default (only the benign git fetch + set-head); the lone write — create+push
# the empty orphan `docket` on a fresh repo — is opt-in (--bootstrap), guarded to the
# ¬DOCKET ∧ ¬LIVE cell. Fail-closed: non-zero + stderr diagnostic on a hard error
# (unreachable origin, unresolvable origin/HEAD, ref-absent integration branch, bad
# metadata_branch). Abort keys on the fetch/set-head return code, NEVER on git show
# (a cached origin/HEAD lets git show succeed with stale bytes). Semantics are ADR-0002 +
# the convention's Configuration / Bootstrap guard, implemented verbatim — no new ADR.
# Four config layers resolve per-key (change 0051 adds the local rung): repo-local
# (<repo>/.docket.local.yml, gitignored, machine-AND-repo-scoped) > repo-committed
# (.docket.yml) > global (${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml) > built-in.
# `runtime.bash` is the machine-local exception: repo-local > global; committed is ignored.
# The coordination-key fence (ADR-0019) applies to both machine-scoped layers alike.
#
# Usage: docket-config.sh [--export] [--format plain|shell] [--bootstrap] [--repo-dir DIR]
#   --export        emit resolved KEY=value lines (default mode)
#   --format FMT    shell (default) — %q-quoted, eval-able, unchanged; plain — raw KEY=value,
#                   no quoting, no `export ` prefix, METADATA_WORKTREE absolutized (change 0068)
#   --bootstrap     additionally perform the CREATE_ORPHAN write when the verdict is
#                   CREATE_ORPHAN (fresh repo); a no-op in every other cell
#   --repo-dir DIR  operate on the git repo at DIR (default, change 0075: the MAIN worktree of
#                   the repo containing CWD, not CWD itself) — the test/mock seam
#   -h, --help      print this header
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-gitignore-block.sh"
# shellcheck source=/dev/null
. "$SELF_DIR/lib/docket-root.sh"

GIT="${GIT:-git}"
MODE=export
FORMAT=shell
DO_BOOTSTRAP=0
REPO_DIR=""   # empty => the MAIN worktree of the repo containing CWD (resolved after arg parsing)
while [ $# -gt 0 ]; do
  case "$1" in
    --export)    MODE=export ;;
    --format)    [ $# -ge 2 ] || { printf 'docket-config: --format requires an argument\n' >&2; exit 2; }
                 case "$2" in plain|shell) FORMAT="$2" ;; *) printf 'docket-config: --format must be plain or shell, got %s\n' "$2" >&2; exit 2 ;; esac
                 shift ;;
    --bootstrap) DO_BOOTSTRAP=1 ;;
    --repo-dir)  [ $# -ge 2 ] || { printf 'docket-config: --repo-dir requires an argument\n' >&2; exit 2; }
                 REPO_DIR="$2"; shift ;;
    -h|--help)   grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'docket-config: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

# --- repo anchor (change 0075) -----------------------------------------------
# The default repo is the MAIN worktree of the repo containing CWD — never CWD itself. A script
# invoked from the .docket/ metadata worktree, a .worktrees/<slug> feature worktree, or any
# subdirectory must resolve the SAME primary root as one invoked from the top; `cd "$REPO_DIR" &&
# pwd -P` (below) would otherwise absolutize the LINKED worktree, which is what mints a nested
# <repo>/.docket/.docket (D2). `--repo-dir` still overrides verbatim. Not a git repo => fall back
# to CWD so the is-inside-work-tree gate below emits its standard "not a git repo" error.
if [ -z "$REPO_DIR" ]; then
  REPO_DIR="$(docket_main_worktree)"
  [ -n "$REPO_DIR" ] || REPO_DIR="."
fi

die() { printf 'docket-config: %s\n' "$*" >&2; exit 1; }
g()   { "$GIT" -C "$REPO_DIR" "$@"; }
emit(){   # emit KEY VALUE — presentation keyed on $FORMAT
  case "$FORMAT" in
    plain) printf '%s=%s\n'  "$1" "$2" ;;   # raw, model-facing (never eval'd)
    *)     printf '%s=%q\n'  "$1" "$2" ;;   # shell-eval-able (default; unchanged)
  esac
}

# Create an empty orphan `docket` and push to origin. Worktree-free (empty-tree root
# commit via plumbing) and leaves NO local branch: we push the commit straight to
# origin's refs/heads/docket, then fetch so refs/remotes/origin/docket is populated.
create_orphan() {
  local tree commit
  tree="$(g mktree </dev/null)" || die "mktree failed"
  commit="$(g commit-tree "$tree" -m 'docket: initialize empty orphan metadata branch')" \
    || die "commit-tree failed — is git user.name/email set?"
  g push origin "$commit:refs/heads/docket" >/dev/null 2>&1 \
    || die "could not push orphan docket to origin"
  g fetch --quiet origin docket 2>/dev/null || true
}

# Minimal flat scalar reader for `key: value` (strips inline #comments, quotes, whitespace).
# Adapted from migrate-to-docket.sh's reader (.docket.yml is intentionally a flat scalar file,
# no yq); migrate's identical copy is out of this change's scope and left as-is. The key is
# escaped before it enters the regex so a metacharacter in any future key can't match
# unintended lines. Nested finalize.gate / finalize.test_command / finalize.require_pr_approval
# are read by their unique leaf-key name. NOTE: a value may not contain a literal '#' — it is
# treated as the start of an inline comment and truncated (fine for the current enum / path /
# empty values).
yaml_get() {  # yaml_get <file> <key>  -> value on stdout (empty if key absent)
  [ -f "$1" ] || return 1
  local key_re
  key_re="$(printf '%s' "$2" | sed 's#[^[:alnum:]_]#\\&#g')"   # escape ERE metachars in the key
  sed -n -E "s/^[[:space:]]*$key_re[[:space:]]*:[[:space:]]*([^#]*).*/\1/p" "$1" \
    | head -n1 | sed -E 's/[[:space:]]+$//; s/^"(.*)"$/\1/; s/^'\''(.*)'\''$/\1/'
}

# Emit the indented child lines of a top-level block key (block-style YAML only). Used for the
# nested `skills:` map (change 0049) so each leaf is read WITHIN the block via yaml_get — never as
# a bare top-level key, which a future top-level `build:`/`review:` could otherwise shadow.
# Comment-strips each line (matching yaml_get semantics) so a trailing comment on `skills:` or a
# full-line comment inside the block can't fool block detection. `[[:space:]]` => tab OR space.
yaml_block_body() {  # yaml_block_body <file> <top-level-key>  -> child lines on stdout
  [ -f "$1" ] || return 0
  awk -v parent="$2" '
    { line=$0; sub(/[[:space:]]*#.*/, "", line) }
    line ~ ("^" parent "[[:space:]]*:[[:space:]]*$") { inblk=1; next }
    inblk && line ~ /^[^[:space:]]/ { inblk=0 }
    inblk { print }
  ' "$1"
}

# Read one scalar from one top-level block without treating `#` inside a quoted value as a
# comment. Duplicate leaves (including two separate runtime blocks) are an ambiguity, not
# precedence: callers require exactly one authority per layer.
runtime_get() { # runtime_get <file>
  [ -f "$1" ] || return 0
  awk '
    function scalar(value, sq,out,i,ch,rest) {
      sq=sprintf("%c", 39)
      if (substr(value,1,1) == sq) {
        out=""
        for (i=2; i<=length(value); i++) {
          ch=substr(value,i,1)
          if (ch == sq) {
            if (substr(value,i+1,1) == sq) { out=out sq; i++; continue }
            rest=substr(value,i+1)
            if (rest ~ /^[[:space:]]*(#.*)?$/) return out
            return value
          }
          out=out ch
        }
        return value
      }
      if (value ~ /^"[^"]*"[[:space:]]*(#.*)?$/) {
        sub(/^"/, "", value); sub(/"[[:space:]]*(#.*)?$/, "", value)
      } else {
        sub(/[[:space:]]*#.*/, "", value); sub(/[[:space:]]+$/, "", value)
      }
      return value
    }
    { raw=$0; structural=$0; sub(/[[:space:]]*#.*/, "", structural) }
    structural ~ /^runtime[[:space:]]*:[[:space:]]*$/ { in_runtime=1; next }
    in_runtime && structural ~ /^[^[:space:]]/ { in_runtime=0 }
    in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/ {
      count++
      value=raw; sub(/^[[:space:]]+bash[[:space:]]*:[[:space:]]*/, "", value)
      found=scalar(value)
    }
    END { if (count > 1) exit 2; if (count == 1) print found }
  ' "$1"
}

# Count runtime.bash declarations without parsing their values. The committed layer is fenced by
# key presence, so even empty, malformed, or duplicate committed values are warning-only and can
# never block a valid machine-local fallback.
runtime_count() { # runtime_count <file>
  [ -f "$1" ] || { printf '0\n'; return; }
  awk '
    { structural=$0; sub(/[[:space:]]*#.*/, "", structural) }
    structural ~ /^runtime[[:space:]]*:[[:space:]]*$/ { in_runtime=1; next }
    in_runtime && structural ~ /^[^[:space:]]/ { in_runtime=0 }
    in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/ { count++ }
    END { print count+0 }
  ' "$1"
}

# --- Stage 1: resolve origin/HEAD + default branch (keyed on fetch/set-head rc) ---
CFG=""
FETCH_ERR=""
trap 'rm -f "$CFG" "$FETCH_ERR"' EXIT
g rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repo: $REPO_DIR"
FETCH_ERR="$(mktemp)" || die "could not create git-fetch diagnostic file"
if ! g fetch --quiet origin 2>"$FETCH_ERR"; then
  printf 'docket-config: git fetch origin failed\n' >&2
  cat "$FETCH_ERR" >&2
  exit 1
fi
rm -f "$FETCH_ERR"
FETCH_ERR=""
g remote set-head origin -a >/dev/null 2>&1 || die "cannot resolve origin/HEAD (git remote set-head failed)"
DEFAULT_BRANCH="$(g symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || true)"
DEFAULT_BRANCH="${DEFAULT_BRANCH#origin/}"
[ -n "$DEFAULT_BRANCH" ] || die "origin/HEAD is unresolvable after set-head"

# --- Stage 2: read + resolve .docket.yml (authoritative via git show origin/HEAD) ---
CFG="$(mktemp)"
g show "origin/HEAD:.docket.yml" >"$CFG" 2>/dev/null || : >"$CFG"   # absent file => defaults (NOT an error)

# --- Stage 2b: global config layer (change 0050) ------------------------------
# ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — the full .docket.yml schema,
# resolved PER-KEY: repo-local > repo-committed > global > built-in (map-valued skills:
# merges field-by-field).
# Read from the LOCAL filesystem — the file is per-machine by definition, so there is no
# authoritative-ref concern as with .docket.yml's origin/HEAD read. Coordination keys are
# fenced (warned-and-ignored) in Stage 2c below.
GCFG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/docket"
GCFG="$GCFG_DIR/config.yml"
gbl(){ yaml_get "$GCFG" "$1"; }   # global-layer scalar read (empty when absent)

# --- Stage 2b': machine-local layer (change 0051) ------------------------------
# <repo>/.docket.local.yml — machine-AND-repo-scoped overrides for exactly the
# global-able key set (the file is machine-scoped, so the ADR-0019 fence applies
# verbatim). Read from the WORKING TREE — the origin/HEAD-authoritative read applies
# only to the committed .docket.yml. Precedence per field (the .env pattern):
# repo-local > repo-committed > global > built-in.
LCFG="$REPO_DIR/.docket.local.yml"
if [ -e "$LCFG" ] && { [ ! -f "$LCFG" ] || [ ! -r "$LCFG" ]; }; then
  printf 'docket-config: warning: %s is not a readable regular file — machine-local config layer ignored\n' "$LCFG" >&2
  LCFG=/dev/null
fi
lcl(){ yaml_get "$LCFG" "$1"; }   # local-layer scalar read (empty when absent)

# --- Stage 2c: fail-loud guards + the coordination-key fence (change 0050) ----
# Misplacement: a global .docket.yml is NEVER read — the global file is config.yml.
if [ -e "$GCFG_DIR/.docket.yml" ]; then
  printf 'docket-config: warning: %s/.docket.yml is not read — global config is config.yml, not .docket.yml (did you mean %s?)\n' "$GCFG_DIR" "$GCFG" >&2
fi
# Malformed/unreadable: warn and fall back to built-ins for the GLOBAL layer only
# (a broken personal file must not brick every repo; per-repo config is still honored).
if [ -e "$GCFG" ] && { [ ! -f "$GCFG" ] || [ ! -r "$GCFG" ]; }; then
  printf 'docket-config: warning: %s is not a readable regular file — global config layer ignored\n' "$GCFG" >&2
  GCFG=/dev/null
fi

# runtime.bash is machine-local by definition: repo-local > global, while a committed value is
# loudly ignored. Read every `bash:` leaf WITHIN its `runtime:` block so an unrelated bare leaf
# cannot shadow it. The temporary block bodies are removed before validation can die.
_runtime_local="$(runtime_get "$LCFG")" \
  || die ".docket.local.yml contains multiple runtime.bash declarations; keep exactly one"
_runtime_committed_count="$(runtime_count "$CFG")"
_runtime_global="$(runtime_get "$GCFG")" \
  || die "global config.yml contains multiple runtime.bash declarations; keep exactly one"

if [ "$_runtime_committed_count" -gt 0 ]; then
  printf 'docket-config: warning: committed config key runtime.bash is machine-local — set it in .docket.local.yml or global config.yml; ignored\n' >&2
fi

DOCKET_BASH_PATH="$_runtime_local"
if [ -z "$DOCKET_BASH_PATH" ]; then
  DOCKET_BASH_PATH="$_runtime_global"
fi

_runtime_remedy='run docket/install.sh after installing Bash 4+ (on macOS: brew install bash)'
[ -n "$DOCKET_BASH_PATH" ] \
  || die "runtime.bash is not configured — $_runtime_remedy"
case "$DOCKET_BASH_PATH" in
  *$'\r'*|*$'\n'*) die "runtime.bash must not contain carriage returns or newlines — $_runtime_remedy" ;;
esac
[[ "$DOCKET_BASH_PATH" = /* ]] \
  || die "runtime.bash must be an absolute path, got '$DOCKET_BASH_PATH' — $_runtime_remedy"
[[ -x "$DOCKET_BASH_PATH" ]] \
  || die "runtime.bash is not an executable file: $DOCKET_BASH_PATH — $_runtime_remedy"
_runtime_version="$(LC_ALL=C "$DOCKET_BASH_PATH" --version 2>/dev/null)" \
  || die "runtime.bash could not report its version: $DOCKET_BASH_PATH — $_runtime_remedy"
_runtime_first_line="${_runtime_version%%$'\n'*}"
case "$_runtime_first_line" in
  'GNU bash, version '*) ;;
  *) die "runtime.bash did not identify itself as GNU Bash: $DOCKET_BASH_PATH reported '${_runtime_first_line:-no version}' — $_runtime_remedy" ;;
esac
_runtime_major="$(sed -nE 's/^GNU bash, version ([0-9]+)\..*/\1/p' <<<"$_runtime_first_line")"
[[ "$_runtime_major" =~ ^[0-9]+$ ]] && [ "$_runtime_major" -ge 4 ] \
  || die "runtime.bash must be Bash 4 or newer, got '${_runtime_first_line:-unknown version}' from $DOCKET_BASH_PATH — $_runtime_remedy"

# Coordination-key fence: a key whose effect writes SHARED state (commits on shared
# branches, committed generated files, external GitHub objects) is per-repo-only; a global
# value is loudly warned-and-ignored — never honored, never fatal. (ADR records the rule.)
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project terminal_publish; do
  if [ -n "$(yaml_get "$GCFG" "$_fkey")" ]; then
    printf "docket-config: warning: global config key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
  if [ -n "$(yaml_get "$LCFG" "$_fkey")" ]; then
    printf "docket-config: warning: .docket.local.yml key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
done

METADATA_BRANCH="$(yaml_get "$CFG" metadata_branch)"; METADATA_BRANCH="${METADATA_BRANCH:-docket}"
case "$METADATA_BRANCH" in
  docket) DOCKET_MODE=docket; METADATA_WORKTREE=.docket ;;
  main)   DOCKET_MODE=main;   METADATA_WORKTREE=. ;;
  *) die "unparseable .docket.yml: metadata_branch must be 'docket' or 'main', got '$METADATA_BRANCH'" ;;
esac

INTEGRATION_BRANCH="$(yaml_get "$CFG" integration_branch)"
if [ -z "$INTEGRATION_BRANCH" ] || [ "$INTEGRATION_BRANCH" = auto ]; then
  INTEGRATION_BRANCH="$DEFAULT_BRANCH"
fi

CHANGES_DIR="$(yaml_get "$CFG" changes_dir)"; CHANGES_DIR="${CHANGES_DIR:-docs/changes}"
ADRS_DIR="$(yaml_get "$CFG" adrs_dir)";       ADRS_DIR="${ADRS_DIR:-docs/adrs}"
RESULTS_DIR="$(yaml_get "$CFG" results_dir)"; RESULTS_DIR="${RESULTS_DIR:-docs/results}"
FINALIZE_GATE="$(lcl gate)"; FINALIZE_GATE="${FINALIZE_GATE:-$(yaml_get "$CFG" gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-$(gbl gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-local}"
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
# change 0101: `auto` ≡ unset — the sentinel that lets .docket.example.yml ship this default as an
# ACTIVE value instead of a commented "normally unset" note. Applied AFTER layer resolution, which
# is what makes a HIGHER layer's `auto` mask a LOWER layer's real command: converting per-layer
# would blank the higher value and let the `:-` chain fall through to the lower one instead.
# Literal lowercase only, matching the integration_branch precedent. Consumers must never see the
# sentinel: finalize would try to RUN `auto` as a shell command.
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""
# change 0102: require_pr_approval — the human-sign-off half of the merge gate (ADR-0011).
# Global-able, deliberately NOT coordination-fenced: `finalize.gate` — already global-able and
# gating the very same merge — is the governing precedent, and splitting the two halves of one
# merge gate across opposite scope classes would be the harder thing to explain. Per-machine
# divergence here is a policy the maintainer chose per machine, never a split backlog.
# Fails CLOSED on a non-boolean (the auto_capture / terminal_publish precedent): defaulting a
# typo to `false` would DISARM a gate the user believes is armed — the exact failure this change
# exists to eliminate.
FINALIZE_REQUIRE_PR_APPROVAL="$(lcl require_pr_approval)"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(yaml_get "$CFG" require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(gbl require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-false}"
case "$FINALIZE_REQUIRE_PR_APPROVAL" in
  true|false) ;;
  *) die "unparseable config: finalize.require_pr_approval must be 'true' or 'false', got '$FINALIZE_REQUIRE_PR_APPROVAL'" ;;
esac
AUTO_GROOM="$(lcl auto_groom)"; AUTO_GROOM="${AUTO_GROOM:-$(yaml_get "$CFG" auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-$(gbl auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-false}"
# change 0091: auto_capture — gates autonomous mid-run capture of discovered follow-up work into
# proposed needs-brainstorm stubs. Global-able (ADR-0019): like auto_groom it gates a LOCAL-RUN
# behavior producing ordinary backlog commits, never coordination state, so per-machine divergence
# is the benign "machine A captures, machine B does not" — never a split backlog. Unlike auto_groom
# it fails CLOSED on a non-boolean (the reclaim.auto / learnings.enabled precedent): defaulting a
# typo to `false` would silently stop capture in a repo that opted in, an invisible failure.
AUTO_CAPTURE="$(lcl auto_capture)"; AUTO_CAPTURE="${AUTO_CAPTURE:-$(yaml_get "$CFG" auto_capture)}"; AUTO_CAPTURE="${AUTO_CAPTURE:-$(gbl auto_capture)}"; AUTO_CAPTURE="${AUTO_CAPTURE:-false}"
case "$AUTO_CAPTURE" in
  true|false) ;;
  *) die "unparseable config: auto_capture must be 'true' or 'false', got '$AUTO_CAPTURE'" ;;
esac
# change 0064: coordination-key fenced — repo-committed .docket.yml ONLY (no lcl/gbl rungs; a
# machine-scoped value is warned-and-ignored by the Stage 2c fence above). Fail closed on garbage:
# silently defaulting a typo to `true` would publish onto the integration branch against intent.
# change 0084: the default is `false` — publishing onto the integration branch is opt-in. A repo
# that never set the key must never get direct machine commits on its code line.
TERMINAL_PUBLISH="$(yaml_get "$CFG" terminal_publish)"; TERMINAL_PUBLISH="${TERMINAL_PUBLISH:-false}"
case "$TERMINAL_PUBLISH" in
  true|false) ;;
  *) die "unparseable .docket.yml: terminal_publish must be 'true' or 'false', got '$TERMINAL_PUBLISH'" ;;
esac

bs_raw="$(lcl board_surfaces)"; bs_machine=0
[ -n "$bs_raw" ] && bs_machine=1                            # local = machine-scoped
if [ -z "$bs_raw" ]; then bs_raw="$(yaml_get "$CFG" board_surfaces)"; fi
if [ -z "$bs_raw" ]; then
  bs_raw="$(gbl board_surfaces)"
  [ -n "$bs_raw" ] && bs_machine=1                          # global = machine-scoped
fi
if [ -z "$bs_raw" ]; then
  BOARD_SURFACES="inline"                                  # unset in all layers => default [inline]
else
  bs="${bs_raw#[}"; bs="${bs%]}"; bs="${bs//,/ }"
  BOARD_SURFACES="$(echo $bs)"                             # trim/collapse; "[]" => ""
  # The github token is per-repo-only when it arrives from a MACHINE-scoped layer (local or
  # global): it mints issues + a Projects board (external objects, not self-healing). Per-repo
  # github is honored.
  if [ "$bs_machine" -eq 1 ] && [ -n "$BOARD_SURFACES" ]; then
    _filtered=""
    for _tok in $BOARD_SURFACES; do
      if [ "$_tok" = github ]; then
        printf 'docket-config: warning: board_surfaces token github is per-repo-only (mints external GitHub objects) — set it in the committed .docket.yml; ignored\n' >&2
      else
        _filtered="$_filtered $_tok"
      fi
    done
    BOARD_SURFACES="$(echo $_filtered)"
  fi
fi
# Change 0071 — the positive sentinel. BOARD_SURFACES is NEVER emitted empty. `board_surfaces: []`
# (and any layer combination whose tokens all get filtered out, e.g. a global `[github]` dropped by
# the machine-scope fence) resolves to the reserved token `none`. Empty therefore has exactly one
# meaning left downstream: *nobody resolved this* — a wiring bug, which board-refresh.sh and
# docket-status.sh now reject loudly instead of silently treating as "board disabled". `none` is
# reserved and exclusive; no real surface may ever be named `none`.
[ -n "$BOARD_SURFACES" ] || BOARD_SURFACES="none"

# --- skills: role-keyed pluggable workflow skills (change 0049 + 0050 global layer) ---
# Nested block; each leaf read within the block only. Per-key precedence:
# per-repo leaf > global leaf > the superpowers default.
SKILLS_BLK="$(mktemp)";  yaml_block_body "$CFG"  skills >"$SKILLS_BLK"
GSKILLS_BLK="$(mktemp)"; yaml_block_body "$GCFG" skills >"$GSKILLS_BLK"
LSKILLS_BLK="$(mktemp)"; yaml_block_body "$LCFG" skills >"$LSKILLS_BLK"
skill_role(){  # skill_role <role> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LSKILLS_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$SKILLS_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GSKILLS_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
SKILL_BRAINSTORM="$(skill_role brainstorm superpowers:brainstorming)"
SKILL_PLAN="$(skill_role plan superpowers:writing-plans)"
SKILL_BUILD="$(skill_role build superpowers:subagent-driven-development)"
SKILL_REVIEW="$(skill_role review superpowers:requesting-code-review)"
SKILL_FINISH="$(skill_role finish superpowers:finishing-a-development-branch)"
# Unknown role keys in EITHER layer: warn-and-ignore (a typo must never abort).
for _blk in "$LSKILLS_BLK" "$SKILLS_BLK" "$GSKILLS_BLK"; do
  while IFS= read -r _role; do
    [ -n "$_role" ] || continue
    case " brainstorm plan build review finish " in
      *" $_role "*) ;;
      *) printf 'docket-config: warning: unknown skills role %s — ignored\n' "$_role" >&2 ;;
    esac
  done < <(sed -n -E 's/^[[:space:]]*([[:alnum:]_-]+)[[:space:]]*:.*/\1/p' "$_blk")
done
rm -f "$SKILLS_BLK" "$GSKILLS_BLK" "$LSKILLS_BLK"

# --- learnings: the findings ledger subsystem (change 0067) --------------------
# Nested block, mirroring finalize:'s SHAPE but the skills: block's PARSING. Each leaf is read
# WITHIN the block via yaml_block_body — never as a bare top-level key. finalize.gate gets away
# with a bare leaf read because `gate`/`test_command` are unusual words; `enabled` and `cap` are
# generic, so a bare read would let ANY block's (or a future top-level) `enabled:` shadow this one.
# Per-key precedence: repo-local > repo-committed > global > built-in.
# ADR-0019 fence: BOTH keys are global-able. A machine-local disable only OMITS an enrichment
# write — it never writes conflicting state, so there is no "which ledger is authoritative"
# question, and the index self-heals on any enabled render.
LEARN_BLK="$(mktemp)";  yaml_block_body "$CFG"  learnings >"$LEARN_BLK"
GLEARN_BLK="$(mktemp)"; yaml_block_body "$GCFG" learnings >"$GLEARN_BLK"
LLEARN_BLK="$(mktemp)"; yaml_block_body "$LCFG" learnings >"$LLEARN_BLK"
# Re-issue the EXIT trap now that all four temp files are defined, so a die() below (or any
# later in the script) cleans up LEARN_BLK/GLEARN_BLK/LLEARN_BLK too — unlike the skills: block
# (which never dies), this block's own fail-closed guards can exit before an end-of-block
# explicit rm would run, so cleanup has to live in the trap, not after the last use.
trap 'rm -f "$CFG" "$LEARN_BLK" "$GLEARN_BLK" "$LLEARN_BLK"' EXIT
learn_key(){  # learn_key <leaf> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LLEARN_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$LEARN_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GLEARN_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
LEARNINGS_ENABLED="$(learn_key enabled true)"
LEARNINGS_CAP="$(learn_key cap 300)"
# Fail closed on garbage (the terminal_publish precedent): silently defaulting a typo would
# either tax every read or silently disable the subsystem — both against intent. `yes`/`no` are
# rejected deliberately (YAML-scalar family: they are boolean keywords under a real loader but
# arrive here as literal strings).
case "$LEARNINGS_ENABLED" in
  true|false) ;;
  *) die "unparseable config: learnings.enabled must be 'true' or 'false', got '$LEARNINGS_ENABLED'" ;;
esac
case "$LEARNINGS_CAP" in
  ''|*[!0-9]*) die "unparseable config: learnings.cap must be a non-negative integer, got '$LEARNINGS_CAP'" ;;
esac

# --- reclaim: the claim-lease self-heal subsystem (change 0089) ----------------
# Nested block parsed exactly like learnings: — each leaf read WITHIN the block via yaml_block_body
# (never a bare top-level key: `auto` is a generic word a future block could shadow). BOTH keys are
# behavioral, NOT coordination-fenced (spec §7-H): they resolve through the full per-field layering
# repo-local > repo-committed > global > built-in, like learnings.* / auto_groom. lease_ttl is an
# integer number of HOURS (converted to seconds by the consumers); auto gates the ONLY mutating path.
RECLAIM_BLK="$(mktemp)";  yaml_block_body "$CFG"  reclaim >"$RECLAIM_BLK"
GRECLAIM_BLK="$(mktemp)"; yaml_block_body "$GCFG" reclaim >"$GRECLAIM_BLK"
LRECLAIM_BLK="$(mktemp)"; yaml_block_body "$LCFG" reclaim >"$LRECLAIM_BLK"
trap 'rm -f "$CFG" "$LEARN_BLK" "$GLEARN_BLK" "$LLEARN_BLK" "$RECLAIM_BLK" "$GRECLAIM_BLK" "$LRECLAIM_BLK"' EXIT
reclaim_key(){  # reclaim_key <leaf> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LRECLAIM_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$RECLAIM_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GRECLAIM_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
RECLAIM_LEASE_TTL="$(reclaim_key lease_ttl 72)"
RECLAIM_AUTO="$(reclaim_key auto false)"
case "$RECLAIM_LEASE_TTL" in
  ''|*[!0-9]*) die "unparseable config: reclaim.lease_ttl must be a non-negative integer (hours), got '$RECLAIM_LEASE_TTL'" ;;
esac
case "$RECLAIM_AUTO" in
  true|false) ;;
  *) die "unparseable config: reclaim.auto must be 'true' or 'false', got '$RECLAIM_AUTO'" ;;
esac

# --- Stage 3: bootstrap guard — evaluate the DOCKET/LIVE 2×2 (docket-mode only) ---
BOOTSTRAP=PROCEED
if [ "$DOCKET_MODE" = docket ]; then
  # DOCKET = the docket branch exists (origin OR local)
  if g rev-parse --verify --quiet refs/remotes/origin/docket >/dev/null 2>&1 \
     || g rev-parse --verify --quiet refs/heads/docket >/dev/null 2>&1; then
    DOCKET=1; else DOCKET=0; fi
  # LIVE = the pruned live planning surface still sits on the integration branch.
  # ls-tree exit≠0 => the ref is absent/unreadable => HARD config error, NOT ¬LIVE.
  live_out="$(g ls-tree "origin/$INTEGRATION_BRANCH" -- \
              "$CHANGES_DIR/active" "$CHANGES_DIR/README.md" "$CHANGES_DIR/BOARD.md" 2>/dev/null)"
  rc=$?
  [ "$rc" -eq 0 ] || die "cannot read origin/$INTEGRATION_BRANCH (git ls-tree exit $rc) — integration_branch ref absent/unreadable (config error, not ¬LIVE)"
  [ -n "$live_out" ] && LIVE=1 || LIVE=0
  if   [ "$DOCKET" -eq 1 ] && [ "$LIVE" -eq 0 ]; then BOOTSTRAP=PROCEED        # migrated
  elif [ "$DOCKET" -eq 0 ] && [ "$LIVE" -eq 0 ]; then BOOTSTRAP=CREATE_ORPHAN  # fresh
  else BOOTSTRAP=STOP_MIGRATE   # ¬DOCKET∧LIVE (single-branch) | DOCKET∧LIVE (half-migrated)
  fi
  if [ "$DO_BOOTSTRAP" -eq 1 ] && [ "$BOOTSTRAP" = CREATE_ORPHAN ]; then
    create_orphan
    # Seed the managed .gitignore block in the primary tree (closes the fresh-repo gap). We do
    # NOT auto-commit — bootstrap runs inside a skill's startup, and committing to the user's
    # integration branch from a config script crosses a write-scope line docket holds. --export
    # stays strictly read-only (this branch only runs under --bootstrap).
    ensure_docket_gitignore_block "$REPO_DIR"
    printf 'docket-config: seeded the managed .gitignore block in %s/.gitignore — COMMIT THIS so the .docket/ worktree and other docket-owned files stay untracked.\n' "$REPO_DIR" >&2
    BOOTSTRAP=PROCEED   # the repo is now migrated; the caller may proceed
  fi
fi

# --- emit ---
if [ "$MODE" = export ]; then
  # METADATA_WORKTREE: relative for shell (eval'd by code running at the repo root); absolute for
  # plain (the model reads it as a cwd-independent literal). REPO_DIR is the resolver's repo.
  MW_EMIT="$METADATA_WORKTREE"
  if [ "$FORMAT" = plain ]; then
    REPO_ABS="$(cd "$REPO_DIR" && pwd -P)"
    case "$METADATA_WORKTREE" in
      .)  MW_EMIT="$REPO_ABS" ;;
      *)  MW_EMIT="$REPO_ABS/$METADATA_WORKTREE" ;;
    esac
  fi
  emit DOCKET_MODE "$DOCKET_MODE"
  emit DEFAULT_BRANCH "$DEFAULT_BRANCH"
  emit METADATA_BRANCH "$METADATA_BRANCH"
  emit INTEGRATION_BRANCH "$INTEGRATION_BRANCH"
  emit METADATA_WORKTREE "$MW_EMIT"
  # REPO_ROOT — PLAIN FORMAT ONLY (change 0075). The absolute main-worktree path; the literal
  # skills read from the `docket.sh preflight` block for a cwd-independent `cd`. It is deliberately
  # absent from the SHELL format: ensure-claude-settings.sh:24 sets its own REPO_ROOT and eval's
  # the shell export at :33, reading it at :38/:74 — emitting it there would silently capture that
  # name. (REPO_ABS is computed above, in the plain branch.)
  if [ "$FORMAT" = plain ]; then
    emit REPO_ROOT "$REPO_ABS"
  fi
  emit DOCKET_BASH_PATH "$DOCKET_BASH_PATH"
  emit CHANGES_DIR "$CHANGES_DIR"
  emit ADRS_DIR "$ADRS_DIR"
  emit RESULTS_DIR "$RESULTS_DIR"
  emit FINALIZE_GATE "$FINALIZE_GATE"
  emit FINALIZE_TEST_COMMAND "$FINALIZE_TEST_COMMAND"
  emit FINALIZE_REQUIRE_PR_APPROVAL "$FINALIZE_REQUIRE_PR_APPROVAL"
  emit LEARNINGS_ENABLED "$LEARNINGS_ENABLED"
  emit LEARNINGS_CAP "$LEARNINGS_CAP"
  emit BOARD_SURFACES "$BOARD_SURFACES"
  emit AUTO_GROOM "$AUTO_GROOM"
  emit AUTO_CAPTURE "$AUTO_CAPTURE"
  emit TERMINAL_PUBLISH "$TERMINAL_PUBLISH"
  emit RECLAIM_LEASE_TTL "$RECLAIM_LEASE_TTL"
  emit RECLAIM_AUTO "$RECLAIM_AUTO"
  emit SKILL_BRAINSTORM "$SKILL_BRAINSTORM"
  emit SKILL_PLAN "$SKILL_PLAN"
  emit SKILL_BUILD "$SKILL_BUILD"
  emit SKILL_REVIEW "$SKILL_REVIEW"
  emit SKILL_FINISH "$SKILL_FINISH"
  emit BOOTSTRAP "$BOOTSTRAP"
fi
