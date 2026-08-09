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
# change 0127: the authoritative change-type vocabulary (DOCKET_CHANGE_TYPES_DEFAULT,
# DOCKET_CHANGE_TYPE_RESERVED, and the three docket_change_type_* predicates). Sourced rather than
# restated so the resolver and every consumer pin the SAME array — ADR-0055's single-authoritative-
# array rule. Definitions only; no source-time side effects.
. "$SELF_DIR/lib/docket-frontmatter.sh"
# change 0133: the single implementation of runtime.bash parsing, counting, and Bash 4+
# validation, shared with install.sh and scripts/ensure-global-config.sh. Definitions only.
. "$SELF_DIR/lib/docket-runtime.sh"

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

# Snapshot-backed readers for the documented scalar/block YAML subset. The three fixed arrays are
# populated once after each layer's established readability policy; no configuration text is ever
# evaluated or used as an array name. `runtime.bash` deliberately keeps its separate file reader.
declare -a CONFIG_LINES_COMMITTED=()
declare -a CONFIG_LINES_GLOBAL=()
declare -a CONFIG_LINES_LOCAL=()

config_trim() {  # config_trim VALUE -> CONFIG_TRIMMED
  CONFIG_TRIMMED="$1"
  CONFIG_TRIMMED="${CONFIG_TRIMMED#"${CONFIG_TRIMMED%%[![:space:]]*}"}"
  CONFIG_TRIMMED="${CONFIG_TRIMMED%"${CONFIG_TRIMMED##*[![:space:]]}"}"
}

config_normalize_scalar() {  # config_normalize_scalar RAW -> normalized scalar
  local value
  config_trim "${1%%#*}"
  value="$CONFIG_TRIMMED"
  if [ "${#value}" -ge 2 ]; then
    case "$value" in
      \"*\") value="${value:1:${#value}-2}" ;;
      \'*\') value="${value:1:${#value}-2}" ;;
    esac
  fi
  printf '%s' "$value"
}

config_line_scalar_get() {  # config_line_scalar_get KEY LINE -> value; 1 when key differs
  local key="$1" line="$2" body candidate
  body="${line%%#*}"
  body="${body#"${body%%[![:space:]]*}"}"
  [[ "$body" == *:* ]] || return 1
  config_trim "${body%%:*}"
  candidate="$CONFIG_TRIMMED"
  [ "$candidate" = "$key" ] || return 1
  config_normalize_scalar "${body#*:}"
}

config_scalar_from_lines() {  # config_scalar_from_lines KEY LINE... -> first scalar or empty
  local key="$1" line
  shift
  for line in "$@"; do
    if config_line_scalar_get "$key" "$line"; then
      return 0
    fi
  done
  return 0
}

config_layer_load() {  # config_layer_load committed|global|local FILE
  local slot="$1" file="$2" line
  [ -f "$file" ] || return 0
  case "$slot" in
    committed) CONFIG_LINES_COMMITTED=()
               while IFS= read -r line || [ -n "$line" ]; do CONFIG_LINES_COMMITTED+=("$line"); done <"$file" ;;
    global)    CONFIG_LINES_GLOBAL=()
               while IFS= read -r line || [ -n "$line" ]; do CONFIG_LINES_GLOBAL+=("$line"); done <"$file" ;;
    local)     CONFIG_LINES_LOCAL=()
               while IFS= read -r line || [ -n "$line" ]; do CONFIG_LINES_LOCAL+=("$line"); done <"$file" ;;
    *) die "internal error: unknown config layer $slot" ;;
  esac
}

config_scalar_get() {  # config_scalar_get committed|global|local KEY -> first scalar or empty
  case "$1" in
    committed) config_scalar_from_lines "$2" "${CONFIG_LINES_COMMITTED[@]}" ;;
    global)    config_scalar_from_lines "$2" "${CONFIG_LINES_GLOBAL[@]}" ;;
    local)     config_scalar_from_lines "$2" "${CONFIG_LINES_LOCAL[@]}" ;;
    *) die "internal error: unknown config layer $1" ;;
  esac
}

config_block_header() {  # config_block_header LINE BLOCK
  local line="$1" block="$2" candidate rest
  [[ "$line" != [[:space:]]* && "$line" == *:* ]] || return 1
  config_trim "${line%%:*}"
  candidate="$CONFIG_TRIMMED"
  config_trim "${line#*:}"
  rest="$CONFIG_TRIMMED"
  [ "$candidate" = "$block" ] && [ -z "$rest" ]
}

config_block_get_from_lines() {  # config_block_get_from_lines BLOCK LEAF LINE... -> first leaf
  local block="$1" leaf="$2" line body in_block=0
  shift 2
  for line in "$@"; do
    body="${line%%#*}"
    if config_block_header "$body" "$block"; then
      in_block=1
      continue
    fi
    [ "$in_block" -eq 1 ] || continue
    if [[ "$body" =~ ^[^[:space:]] ]]; then
      in_block=0
      continue
    fi
    if config_line_scalar_get "$leaf" "$line"; then
      return 0
    fi
  done
  return 0
}

config_block_get() {  # config_block_get committed|global|local BLOCK LEAF -> first block leaf
  case "$1" in
    committed) config_block_get_from_lines "$2" "$3" "${CONFIG_LINES_COMMITTED[@]}" ;;
    global)    config_block_get_from_lines "$2" "$3" "${CONFIG_LINES_GLOBAL[@]}" ;;
    local)     config_block_get_from_lines "$2" "$3" "${CONFIG_LINES_LOCAL[@]}" ;;
    *) die "internal error: unknown config layer $1" ;;
  esac
}

config_block_keys_from_lines() {  # config_block_keys_from_lines BLOCK LINE... -> scalar keys
  local block="$1" line body candidate in_block=0
  shift
  for line in "$@"; do
    body="${line%%#*}"
    if config_block_header "$body" "$block"; then
      in_block=1
      continue
    fi
    [ "$in_block" -eq 1 ] || continue
    if [[ "$body" =~ ^[^[:space:]] ]]; then
      in_block=0
      continue
    fi
    body="${body#"${body%%[![:space:]]*}"}"
    [[ "$body" == *:* ]] || continue
    config_trim "${body%%:*}"
    candidate="$CONFIG_TRIMMED"
    [[ "$candidate" =~ ^[[:alnum:]_-]+$ ]] && printf '%s\n' "$candidate"
  done
}

config_block_keys() {  # config_block_keys committed|global|local BLOCK -> scalar keys
  case "$1" in
    committed) config_block_keys_from_lines "$2" "${CONFIG_LINES_COMMITTED[@]}" ;;
    global)    config_block_keys_from_lines "$2" "${CONFIG_LINES_GLOBAL[@]}" ;;
    local)     config_block_keys_from_lines "$2" "${CONFIG_LINES_LOCAL[@]}" ;;
    *) die "internal error: unknown config layer $1" ;;
  esac
}

# Read one scalar from one top-level block without treating `#` inside a quoted value as a
# comment. Duplicate leaves (including two separate runtime blocks) are an ambiguity, not
# precedence: callers require exactly one authority per layer. The resolver passes NO markers —
# it has no managed block; only the installer excludes one.
runtime_get() { # runtime_get <file>
  docket_runtime_unique "$1"
}

# Count runtime.bash declarations without parsing their values. The committed layer is fenced by
# key presence, so even empty, malformed, or duplicate committed values are warning-only and can
# never block a valid machine-local fallback.
runtime_count() { # runtime_count <file>
  docket_runtime_count "$1"
}

# --- Stage 1: resolve origin/HEAD + default branch (keyed on fetch/set-head rc) ---
CFG=""
FETCH_ERR=""
trap 'rm -f "$CFG" "$FETCH_ERR"' EXIT
g rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repo: $REPO_DIR"
FETCH_ERR="$(mktemp "${TMPDIR:-/tmp}/docket-config.XXXXXX")" || die "could not create git-fetch diagnostic file"
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
CFG="$(mktemp "${TMPDIR:-/tmp}/docket-config.XXXXXX")"
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
gbl(){ config_scalar_get global "$1"; }   # global-layer scalar read (empty when absent)

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
lcl(){ config_scalar_get local "$1"; }   # local-layer scalar read (empty when absent)

# parse_inline_list RAW -> normalized space-separated tokens on stdout.
# Strips one enclosing `[ ]`, treats commas as separators, collapses whitespace runs, trims the
# ends. `[]` and `[ ]` normalize to the empty string, which every caller checks for.
#
# Single source for all three inline-list keys (board_surfaces, change_types, auto_capture.types):
# the earlier per-key copies each spelled the same bracket/comma surgery inline, so a fix to one
# never reached the others.
#
# It normalizes WITHOUT word-splitting, deliberately. The previous spelling ended in an unquoted
# `$(echo $body)`; with no `set -f` in effect that pathname-expands, so `change_types: [f*]`
# resolved the taxonomy from FILENAMES IN THE RESOLVER'S CWD — the same committed config produced
# a different taxonomy on different machines, silently, because every expanded lowercase filename
# passes the well-formedness check downstream. sync-agents.sh already disables globbing for
# externally-sourced tokens for exactly this reason; using `tr` instead of a split means there is
# no expansion step to guard in the first place. Callers iterate via `read -r -a`, which also
# never globs.
parse_inline_list(){ # parse_inline_list RAW -> normalized tokens
  local body="${1#[}"; body="${body%]}"; body="${body//,/ }"
  body="$(printf '%s' "$body" | tr -s '[:space:]' ' ')"
  body="${body# }"; body="${body% }"
  printf '%s' "$body"
}

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

# Read each permitted source exactly once. From this point general resolution is snapshot-backed;
# runtime.bash intentionally remains file-backed through docket-runtime.sh below.
config_layer_load committed "$CFG"
config_layer_load global "$GCFG"
config_layer_load local "$LCFG"

# runtime.bash is machine-local by definition: repo-local > global, while a committed value is
# loudly ignored. Read every `bash:` leaf WITHIN its `runtime:` block so an unrelated bare leaf
# cannot shadow it. The temporary block bodies are removed before validation can die.
_runtime_local="$(runtime_get "$LCFG")"; _runtime_local_rc=$?
case "$_runtime_local_rc" in
  0) ;;
  3) die "runtime.bash must be nested exactly one level under \`runtime:\`; found it deeper in .docket.local.yml" ;;
  *) die ".docket.local.yml contains multiple runtime.bash declarations; keep exactly one" ;;
esac
_runtime_committed_count="$(runtime_count "$CFG")"
_runtime_global="$(runtime_get "$GCFG")"; _runtime_global_rc=$?
case "$_runtime_global_rc" in
  0) ;;
  3) die "runtime.bash must be nested exactly one level under \`runtime:\`; found it deeper in global config.yml" ;;
  *) die "global config.yml contains multiple runtime.bash declarations; keep exactly one" ;;
esac

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
docket_runtime_serializable "$DOCKET_BASH_PATH" \
  || die "runtime.bash must not contain carriage returns or newlines — $_runtime_remedy"
# The shared validator returns a reason token; the five diagnostics below are resolver-owned
# policy and are why it returns a token instead of printing a message. The `printf x` guard keeps
# an empty version line from collapsing the two-line payload.
_runtime_probe="$(docket_runtime_validate_bash "$DOCKET_BASH_PATH"; printf 'x')"
_runtime_probe="${_runtime_probe%x}"
_runtime_reason="${_runtime_probe%%$'\n'*}"
_runtime_probe="${_runtime_probe#*$'\n'}"
_runtime_first_line="${_runtime_probe%$'\n'}"
case "$_runtime_reason" in
  ok) ;;
  not-absolute)
    die "runtime.bash must be an absolute path, got '$DOCKET_BASH_PATH' — $_runtime_remedy" ;;
  not-executable)
    die "runtime.bash is not an executable file: $DOCKET_BASH_PATH — $_runtime_remedy" ;;
  no-version)
    die "runtime.bash could not report its version: $DOCKET_BASH_PATH — $_runtime_remedy" ;;
  not-gnu-bash)
    die "runtime.bash did not identify itself as GNU Bash: $DOCKET_BASH_PATH reported '${_runtime_first_line:-no version}' — $_runtime_remedy" ;;
  old-major)
    die "runtime.bash must be Bash 4 or newer, got '${_runtime_first_line:-unknown version}' from $DOCKET_BASH_PATH — $_runtime_remedy" ;;
  *)
    die "runtime.bash validation returned an unrecognized result '$_runtime_reason' for $DOCKET_BASH_PATH — $_runtime_remedy" ;;
esac

# Coordination-key fence: a key whose effect writes SHARED state (commits on shared
# branches, committed generated files, external GitHub objects) is per-repo-only; a global
# value is loudly warned-and-ignored — never honored, never fatal. (ADR records the rule.)
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project terminal_publish skip_results_only_delta; do
  if [ -n "$(config_scalar_get global "$_fkey")" ]; then
    printf "docket-config: warning: global config key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
  if [ -n "$(config_scalar_get local "$_fkey")" ]; then
    printf "docket-config: warning: .docket.local.yml key %s is per-repo-only — set it in the repo's committed .docket.yml; ignored\n" "$_fkey" >&2
  fi
done

METADATA_BRANCH="$(config_scalar_get committed metadata_branch)"; METADATA_BRANCH="${METADATA_BRANCH:-docket}"
case "$METADATA_BRANCH" in
  docket) DOCKET_MODE=docket; METADATA_WORKTREE=.docket ;;
  main)   DOCKET_MODE=main;   METADATA_WORKTREE=. ;;
  *) die "unparseable .docket.yml: metadata_branch must be 'docket' or 'main', got '$METADATA_BRANCH'" ;;
esac

INTEGRATION_BRANCH="$(config_scalar_get committed integration_branch)"
if [ -z "$INTEGRATION_BRANCH" ] || [ "$INTEGRATION_BRANCH" = auto ]; then
  INTEGRATION_BRANCH="$DEFAULT_BRANCH"
fi

CHANGES_DIR="$(config_scalar_get committed changes_dir)"; CHANGES_DIR="${CHANGES_DIR:-docs/changes}"
ADRS_DIR="$(config_scalar_get committed adrs_dir)";       ADRS_DIR="${ADRS_DIR:-docs/adrs}"
RESULTS_DIR="$(config_scalar_get committed results_dir)"; RESULTS_DIR="${RESULTS_DIR:-docs/results}"
FINALIZE_GATE="$(lcl gate)"; FINALIZE_GATE="${FINALIZE_GATE:-$(config_scalar_get committed gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-$(gbl gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-local}"
FINALIZE_TEST_COMMAND="$(lcl test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(config_scalar_get committed test_command)}"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
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
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(config_scalar_get committed require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-$(gbl require_pr_approval)}"
FINALIZE_REQUIRE_PR_APPROVAL="${FINALIZE_REQUIRE_PR_APPROVAL:-false}"
case "$FINALIZE_REQUIRE_PR_APPROVAL" in
  true|false) ;;
  *) die "unparseable config: finalize.require_pr_approval must be 'true' or 'false', got '$FINALIZE_REQUIRE_PR_APPROVAL'" ;;
esac
# change 0190: skip_results_only_delta — the ARMING switch for the second limb of finalize's
# post-rebase suite-skip predicate (skip when the evidence head_sha is a strict ancestor of HEAD
# and the whole delta lies under <results_dir>/). Default `false`, so a repo that never sets it
# keeps change 0170's equality-only predicate unchanged.
# Coordination-fenced (per-repo-only), UNLIKE its two finalize siblings, and deliberately so.
# `gate` and `require_pr_approval` express a POLICY a maintainer may legitimately hold differently
# per machine. This key instead asserts a FACT about the repo's own test suite — that no executable
# suite component reads the results tree as a content source, which is the ONLY reason skipping the
# suite over a results-only delta is safe. A machine-scoped value asserts that fact for every repo
# the machine touches (global config) or for a repo where no collaborator can see it was claimed
# (.docket.local.yml, gitignored), i.e. arms a merge-gating skip on a property nobody verified
# there. ADR-0019's own rule reaches the same classification: the armed key's effect is a merge
# pushed onto the shared integration branch on trust, which is shared state that is not
# deterministically re-derivable. So: committed layer only, no lcl/gbl rungs — a machine-scoped
# value is warned-and-ignored by the Stage 2c fence above.
# Fails CLOSED on a non-boolean, require_pr_approval's argument inverted: defaulting a typo to
# `true` would ARM a gate-weakening skip the repo never opted into.
FINALIZE_SKIP_RESULTS_ONLY_DELTA="$(config_scalar_get committed skip_results_only_delta)"
FINALIZE_SKIP_RESULTS_ONLY_DELTA="${FINALIZE_SKIP_RESULTS_ONLY_DELTA:-false}"
case "$FINALIZE_SKIP_RESULTS_ONLY_DELTA" in
  true|false) ;;
  *) die "unparseable .docket.yml: finalize.skip_results_only_delta must be 'true' or 'false', got '$FINALIZE_SKIP_RESULTS_ONLY_DELTA'" ;;
esac
AUTO_GROOM="$(lcl auto_groom)"; AUTO_GROOM="${AUTO_GROOM:-$(config_scalar_get committed auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-$(gbl auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-false}"
# change 0127: auto_capture became a MAP (change_types + the nested block are resolved together,
# after the reclaim: block below — auto_capture.types validates against the effective change_types,
# so the list must resolve first). The scalar form this key had from change 0091 is now a hard
# error with no compatibility shim; see the legacy guard there.
# change 0064: coordination-key fenced — repo-committed .docket.yml ONLY (no lcl/gbl rungs; a
# machine-scoped value is warned-and-ignored by the Stage 2c fence above). Fail closed on garbage:
# silently defaulting a typo to `true` would publish onto the integration branch against intent.
# change 0084: the default is `false` — publishing onto the integration branch is opt-in. A repo
# that never set the key must never get direct machine commits on its code line.
TERMINAL_PUBLISH="$(config_scalar_get committed terminal_publish)"; TERMINAL_PUBLISH="${TERMINAL_PUBLISH:-false}"
case "$TERMINAL_PUBLISH" in
  true|false) ;;
  *) die "unparseable .docket.yml: terminal_publish must be 'true' or 'false', got '$TERMINAL_PUBLISH'" ;;
esac

bs_raw="$(lcl board_surfaces)"; bs_machine=0
[ -n "$bs_raw" ] && bs_machine=1                            # local = machine-scoped
if [ -z "$bs_raw" ]; then bs_raw="$(config_scalar_get committed board_surfaces)"; fi
if [ -z "$bs_raw" ]; then
  bs_raw="$(gbl board_surfaces)"
  [ -n "$bs_raw" ] && bs_machine=1                          # global = machine-scoped
fi
if [ -z "$bs_raw" ]; then
  BOARD_SURFACES="inline"                                  # unset in all layers => default [inline]
else
  BOARD_SURFACES="$(parse_inline_list "$bs_raw")"          # trim/collapse; "[]" => ""
  # The github token is per-repo-only when it arrives from a MACHINE-scoped layer (local or
  # global): it mints issues + a Projects board (external objects, not self-healing). Per-repo
  # github is honored.
  if [ "$bs_machine" -eq 1 ] && [ -n "$BOARD_SURFACES" ]; then
    _filtered=""
    read -r -a _bs_arr <<< "$BOARD_SURFACES"
    for _tok in "${_bs_arr[@]}"; do
      if [ "$_tok" = github ]; then
        printf 'docket-config: warning: board_surfaces token github is per-repo-only (mints external GitHub objects) — set it in the committed .docket.yml; ignored\n' >&2
      else
        _filtered="$_filtered $_tok"
      fi
    done
    BOARD_SURFACES="$(parse_inline_list "$_filtered")"
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
# per-repo leaf > global leaf > the role's built-in default (superpowers for brainstorm/plan/
# finish, docket's own docket-build/docket-review for build/review since change 0193).
skill_role(){  # skill_role <role> <default> -> resolved value on stdout
  local v; v="$(config_block_get local skills "$1")"
  [ -n "$v" ] || v="$(config_block_get committed skills "$1")"
  [ -n "$v" ] || v="$(config_block_get global skills "$1")"
  printf '%s' "${v:-$2}"
}
SKILL_BRAINSTORM="$(skill_role brainstorm superpowers:brainstorming)"
SKILL_PLAN="$(skill_role plan superpowers:writing-plans)"
SKILL_BUILD="$(skill_role build docket-build)"
SKILL_REVIEW="$(skill_role review docket-review)"
SKILL_FINISH="$(skill_role finish superpowers:finishing-a-development-branch)"
# Unknown role keys in EITHER layer: warn-and-ignore (a typo must never abort).
for _slot in local committed global; do
  while IFS= read -r _role; do
    [ -n "$_role" ] || continue
    case " brainstorm plan build review finish " in
      *" $_role "*) ;;
      *) printf 'docket-config: warning: unknown skills role %s — ignored\n' "$_role" >&2 ;;
    esac
  done < <(config_block_keys "$_slot" skills)
done

# --- learnings: the findings ledger subsystem (change 0067) --------------------
# Nested block, mirroring finalize:'s SHAPE but the skills: block's PARSING. Each leaf is read
# WITHIN the block via config_block_get — never as a bare top-level key. finalize.gate gets away
# with a bare leaf read because `gate`/`test_command` are unusual words; `enabled` and `cap` are
# generic, so a bare read would let ANY block's (or a future top-level) `enabled:` shadow this one.
# Per-key precedence: repo-local > repo-committed > global > built-in.
# ADR-0019 fence: BOTH keys are global-able. A machine-local disable only OMITS an enrichment
# write — it never writes conflicting state, so there is no "which ledger is authoritative"
# question, and the index self-heals on any enabled render.
learn_key(){  # learn_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local learnings "$1")"
  [ -n "$v" ] || v="$(config_block_get committed learnings "$1")"
  [ -n "$v" ] || v="$(config_block_get global learnings "$1")"
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
# Nested block parsed exactly like learnings: — each leaf read WITHIN the block via config_block_get
# (never a bare top-level key: `auto` is a generic word a future block could shadow). BOTH keys are
# behavioral, NOT coordination-fenced (spec §7-H): they resolve through the full per-field layering
# repo-local > repo-committed > global > built-in, like learnings.* / auto_groom. lease_ttl is an
# integer number of HOURS (converted to seconds by the consumers); auto gates the ONLY mutating path.
reclaim_key(){  # reclaim_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local reclaim "$1")"
  [ -n "$v" ] || v="$(config_block_get committed reclaim "$1")"
  [ -n "$v" ] || v="$(config_block_get global reclaim "$1")"
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

# --- build: the build-role knobs (change 0167) -------------------------------
# Nested block parsed exactly like reclaim: — the leaf is read WITHIN the block via
# config_block_get, never as a bare top-level key: `checkpoint` is a generic word another block
# could shadow. Behavioral, NOT coordination-fenced: it resolves through the full per-field
# layering repo-local > repo-committed > global > built-in, like reclaim.* / learnings.*.
# checkpoint gates whether docket-build persists a resume ledger; false (the default) keeps the
# build's durability in the per-task code commits alone.
build_key(){  # build_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local build "$1")"
  [ -n "$v" ] || v="$(config_block_get committed build "$1")"
  [ -n "$v" ] || v="$(config_block_get global build "$1")"
  printf '%s' "${v:-$2}"
}
BUILD_CHECKPOINT="$(build_key checkpoint false)"
case "$BUILD_CHECKPOINT" in
  true|false) ;;
  *) die "unparseable config: build.checkpoint must be 'true' or 'false', got '$BUILD_CHECKPOINT'" ;;
esac

# --- review: the review-role knobs (change 0218) ------------------------------
# Nested block parsed exactly like build: — the leaf is read WITHIN the block via config_block_get,
# never as a bare top-level key. Two reasons here, not one: `min_fix_severity` is a generic-ish
# word another block could shadow, AND the `skills:` block already carries a `review:` LEAF.
# config_block_header rejects `skills.review: docket-review` as this block's header for TWO
# independent reasons — the line is indented, and it has a value after the colon — so neither
# check alone is load-bearing for THAT spelling. The column-0 requirement IS load-bearing for an
# indented, valueless `review:` carrying a nested `min_fix_severity`. tests/test_docket_config.sh
# pins both: RMF-g1 the coexistence, RMF-g2 the column-0 invariant (mutation-verified — deleting
# the column-0 conjunct reddens RMF-g2 and nothing else).
# Behavioral, NOT coordination-fenced: it shapes BRANCH content (which findings get fixed in the
# diff a human reviews), never shared metadata, so it resolves through the full per-field layering
# repo-local > repo-committed > global > built-in, like build.checkpoint / reclaim.* / learnings.*.
# min_fix_severity is the MINIMUM finding severity that enters docket-implement-next's Step 6 fix
# loop. Blockers are always fixed regardless — a run cannot proceed past an unfixed blocker — so
# `blocker` means "fix nothing else", the pre-0218 record-everything behavior kept as a compat
# escape hatch. Fails CLOSED on anything else (the build.checkpoint / learnings.enabled
# precedent): silently defaulting a typo would either over-fix or under-fix a branch a human is
# about to merge, and both are invisible.
review_key(){  # review_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local review "$1")"
  [ -n "$v" ] || v="$(config_block_get committed review "$1")"
  [ -n "$v" ] || v="$(config_block_get global review "$1")"
  printf '%s' "${v:-$2}"
}
# max_fix_tasks is the MAXIMUM number of non-blocker fix TASKS that loop dispatches per run — the
# unit is the task as skills/docket-implement-next/references/fix-loop.md defines it, so a batched-
# minors task spends one slot. Blockers are never counted against it, for the same reason
# min_fix_severity cannot suppress them: a cap that counted blockers would disarm the gate the
# blocker floor exists to protect. Validated as a COUNT, on reclaim.lease_ttl's / learnings.cap's
# `''|*[!0-9]*` precedent rather than an enum: non-negative integer or abort. Fails CLOSED for the
# same reason min_fix_severity does.
# ZERO IS LEGAL, deliberately. It reads "fix nothing but blockers" — a state the config can ALREADY
# express through `min_fix_severity: blocker`, so rejecting 0 here would forbid a configuration that
# is reachable anyway by another key, which is arbitrary rather than protective. It also cannot fail
# open: blockers sit outside the count, so a 0 cap still fixes every blocker and still halts on one
# it cannot fix. The alternative reading — 0 means "unbounded", the way an empty list sometimes
# means "all" — was rejected: it would make the single most restrictive-looking value the single
# most permissive one, and this file's other counts (learnings.cap, reclaim.lease_ttl) give 0 no
# such magic. tests/test_docket_config.sh RMX-h ("0 is legal (fix nothing but blockers)", beside the
# aborts-nonzero asserts) pins the boundary in both directions.
REVIEW_MIN_FIX_SEVERITY="$(review_key min_fix_severity minor)"
REVIEW_MAX_FIX_TASKS="$(review_key max_fix_tasks 10)"
case "$REVIEW_MIN_FIX_SEVERITY" in
  minor|important|blocker) ;;
  *) die "unparseable config: review.min_fix_severity must be 'minor', 'important', or 'blocker', got '$REVIEW_MIN_FIX_SEVERITY'" ;;
esac
case "$REVIEW_MAX_FIX_TASKS" in
  ''|*[!0-9]*) die "unparseable config: review.max_fix_tasks must be a non-negative integer, got '$REVIEW_MAX_FIX_TASKS'" ;;
esac

# --- gate_observation_budget: the build gate's artifact-observation budget (change 0223) ------
# A FLAT top-level key, deliberately not nested. `finalize.gate_observation_budget` would be wrong
# because the key binds docket-build's gate too, and a new top-level `gate:` block would collide
# with `finalize.gate`, which already means the gate MODE — a permanent reading hazard for a key
# read under time pressure.
# Integer MINUTES. It bounds how long docket is willing to await a terminal durable gate result;
# it does NOT control the timeout of any individual harness operation, and no harness's foreground
# timeout may be encoded here.
# Global-able, NOT coordination-fenced: local execution timing is legitimately per-machine, so it
# resolves through the full chain repo-local > repo-committed > global > built-in, like auto_groom.
# Fail closed on garbage (the learnings.cap / review.max_fix_tasks precedent): a typo'd budget
# silently defaulting would make the fail-closed halt fire at a duration nobody chose. 0 is legal
# and carries no magic — it means "observe once, then fail closed".
# tests/test_docket_config.sh pins the chain, the fail-closed boundary, and the emit position
# (GOB-a … GOB-g).
GATE_OBSERVATION_BUDGET="$(lcl gate_observation_budget)"
GATE_OBSERVATION_BUDGET="${GATE_OBSERVATION_BUDGET:-$(config_scalar_get committed gate_observation_budget)}"
GATE_OBSERVATION_BUDGET="${GATE_OBSERVATION_BUDGET:-$(gbl gate_observation_budget)}"
GATE_OBSERVATION_BUDGET="${GATE_OBSERVATION_BUDGET:-30}"
case "$GATE_OBSERVATION_BUDGET" in
  ''|*[!0-9]*) die "unparseable config: gate_observation_budget must be a non-negative integer (minutes), got '$GATE_OBSERVATION_BUDGET'" ;;
esac

# --- delegation_observation_budget: the delegation boundary's budget (change 0271) -----
# SIBLING of gate_observation_budget, deliberately a SEPARATE key rather than a reuse.
# The two bound different units: gate_observation_budget bounds awaiting one SUITE RUN
# started by an agent; this bounds awaiting a whole delegated AGENT RUN, which contains
# a plan task, its verification, and its commit. Folding them onto one number would force
# whichever unit is larger to set the ceiling for both.
# Integer MINUTES; default 60. Same fail-closed posture and the same full layering chain
# (repo-local > repo-committed > global > built-in) — local execution timing is
# legitimately per-machine, so it is global-able and NOT coordination-fenced.
# 0 is legal and carries no magic: "observe once, then fail closed".
# tests/test_docket_config.sh pins the chain and the boundary (DOB-a … DOB-f).
DELEGATION_OBSERVATION_BUDGET="$(lcl delegation_observation_budget)"
DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-$(config_scalar_get committed delegation_observation_budget)}"
DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-$(gbl delegation_observation_budget)}"
DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-60}"
case "$DELEGATION_OBSERVATION_BUDGET" in
  ''|*[!0-9]*) die "unparseable config: delegation_observation_budget must be a non-negative integer (minutes), got '$DELEGATION_OBSERVATION_BUDGET'" ;;
esac

# --- change_types + auto_capture: the typed-capture policy (change 0127) -------
# change_types is a LIST resolved with WHOLE-LIST REPLACEMENT: the first layer that sets it wins
# entirely. Merging would make a built-in value unremovable — a user could only ever add types,
# never drop one, which is the opposite of a configurable taxonomy. Inline flow style only
# (`[a, b]`), matching the board_surfaces / agent_harnesses precedent. Global-able: it governs
# what this machine CREATES, never coordination state.
ct_raw="$(lcl change_types)"
[ -n "$ct_raw" ] || ct_raw="$(config_scalar_get committed change_types)"
[ -n "$ct_raw" ] || ct_raw="$(gbl change_types)"
if [ -z "$ct_raw" ]; then
  CHANGE_TYPES="${DOCKET_CHANGE_TYPES_DEFAULT[*]}"
else
  CHANGE_TYPES="$(parse_inline_list "$ct_raw")"    # trim/collapse; "[]" => ""
  [ -n "$CHANGE_TYPES" ] \
    || die "unparseable config: change_types must be a non-empty list, got '$ct_raw'"
  read -r -a ct_arr <<< "$CHANGE_TYPES"
  for ct_tok in "${ct_arr[@]}"; do
    if docket_change_type_is_reserved "$ct_tok"; then
      die "unparseable config: change_types must not contain the reserved value '$ct_tok' (a selector/query pseudo-value, never a real type)"
    fi
    docket_change_type_is_wellformed "$ct_tok" \
      || die "unparseable config: change_types entry '$ct_tok' must match [a-z][a-z0-9-]*"
  done
  ct_dupes="$(printf '%s\n' "${ct_arr[@]}" | sort | uniq -d | tr '\n' ' ')"
  [ -z "${ct_dupes// /}" ] \
    || die "unparseable config: change_types has duplicate entries: ${ct_dupes% }"
fi

# auto_capture is a MAP (intentionally breaking, change 0127). The legacy scalar has NO shim: a
# top-level auto_capture carrying a non-empty scalar value is a hard error in EVERY layer, with a
# diagnostic that prints the nested replacement carrying the user's OWN value — a remedy has to be
# valid in the exact state that produced it (learning: printed-remedy-state-validity). A map header
# (`auto_capture:`) yields an empty scalar read, which is what discriminates it from the scalar.
for ac_slot in local committed global; do
  case "$ac_slot" in
    local) ac_layer="$LCFG" ;;
    committed) ac_layer="$CFG" ;;
    global) ac_layer="$GCFG" ;;
  esac
  ac_legacy="$(config_scalar_get "$ac_slot" auto_capture)"
  [ -n "$ac_legacy" ] || continue
  die "unparseable config: auto_capture is now a map, not a scalar (found 'auto_capture: $ac_legacy' in $ac_layer). Replace it with:

auto_capture:
  enabled: $ac_legacy
  types: all"
done
# Nested block parsed exactly like learnings:/reclaim: — each leaf read WITHIN the block, which is
# what gives PER-LEAF fallback (a high layer may override `enabled` while inheriting `types`) and
# what keeps `enabled` from colliding with learnings.enabled under the snapshot scalar reader.
ac_key(){  # ac_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local auto_capture "$1")"
  [ -n "$v" ] || v="$(config_block_get committed auto_capture "$1")"
  [ -n "$v" ] || v="$(config_block_get global auto_capture "$1")"
  printf '%s' "${v:-$2}"
}
# ct_effective_at LAYER -> the change_types an author writing AT that layer could see: their own
# layer when it sets the key, else the layers BELOW them in precedence, else the built-in default.
#
# The auto_capture.types membership check below is evaluated against THIS, not against the globally
# effective CHANGE_TYPES. The two keys resolve through independent chains — change_types by
# WHOLE-LIST replacement, types per-leaf inside the block — so a higher layer that narrows
# change_types without also restating types would otherwise invalidate a LOWER layer's perfectly
# valid block. That was not hypothetical: .docket.example.yml advertises whole-list replacement as
# the way to remove a built-in value, and separately advertises per-leaf inheritance of `types`;
# composing the two documented features aborted the resolver, and since every skill's Step 0 runs
# `docket.sh preflight`, one machine-local narrowing bricked docket on that machine entirely —
# even with auto_capture.enabled: false, where the leaf governs nothing. Validating each layer
# against what its own author could see keeps a genuine SAME-layer inconsistency an error while
# letting independently-valid layers compose.
ct_effective_at(){ # ct_effective_at local|committed|global -> normalized change_types
  local raw=""
  case "$1" in
    local)     raw="$(lcl change_types)"
               [ -n "$raw" ] || raw="$(config_scalar_get committed change_types)"
               [ -n "$raw" ] || raw="$(gbl change_types)" ;;
    committed) raw="$(config_scalar_get committed change_types)"
               [ -n "$raw" ] || raw="$(gbl change_types)" ;;
    global)    raw="$(gbl change_types)" ;;
  esac
  if [ -z "$raw" ]; then printf '%s' "${DOCKET_CHANGE_TYPES_DEFAULT[*]}"
  else parse_inline_list "$raw"; fi
}
AUTO_CAPTURE_ENABLED="$(ac_key enabled false)"
case "$AUTO_CAPTURE_ENABLED" in
  true|false) ;;
  *) die "unparseable config: auto_capture.enabled must be 'true' or 'false', got '$AUTO_CAPTURE_ENABLED'" ;;
esac
# `all` is preserved LITERALLY rather than expanded to the effective list, so a consumer can still
# distinguish "every type, including any a future layer adds" from "this explicit subset".
AUTO_CAPTURE_TYPES="$(ac_key types all)"
if [ "$AUTO_CAPTURE_TYPES" != all ]; then
  act_raw="$AUTO_CAPTURE_TYPES"
  AUTO_CAPTURE_TYPES="$(parse_inline_list "$act_raw")"
  [ -n "$AUTO_CAPTURE_TYPES" ] \
    || die "unparseable config: auto_capture.types must be 'all' or a non-empty list, got '$act_raw'"
  # Which layer supplied `types`? Same precedence ac_key resolved it with.
  ac_types_layer=local
  if   [ -n "$(config_block_get local auto_capture types)" ]; then ac_types_layer=local
  elif [ -n "$(config_block_get committed auto_capture types)" ]; then ac_types_layer=committed
  elif [ -n "$(config_block_get global auto_capture types)" ]; then ac_types_layer=global
  fi
  ct_visible="$(ct_effective_at "$ac_types_layer")"
  read -r -a ctv_arr <<< "$ct_visible"
  read -r -a act_arr <<< "$AUTO_CAPTURE_TYPES"
  for act_tok in "${act_arr[@]}"; do
    docket_change_type_is_member "$act_tok" "${ctv_arr[@]}" \
      || die "unparseable config: auto_capture.types entry '$act_tok' is not in the change_types visible to the $ac_types_layer layer ($ct_visible)"
  done
  act_dupes="$(printf '%s\n' "${act_arr[@]}" | sort | uniq -d | tr '\n' ' ')"
  [ -z "${act_dupes// /}" ] \
    || die "unparseable config: auto_capture.types has duplicate entries: ${act_dupes% }"
fi

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
  # absent from the SHELL format: ensure-claude-settings.sh sets its own REPO_ROOT (from
  # `rev-parse --show-toplevel`) and eval's the shell export, reading it back later —
  # emitting it there would silently capture that
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
  emit FINALIZE_SKIP_RESULTS_ONLY_DELTA "$FINALIZE_SKIP_RESULTS_ONLY_DELTA"
  emit LEARNINGS_ENABLED "$LEARNINGS_ENABLED"
  emit LEARNINGS_CAP "$LEARNINGS_CAP"
  emit BOARD_SURFACES "$BOARD_SURFACES"
  emit AUTO_GROOM "$AUTO_GROOM"
  emit CHANGE_TYPES "$CHANGE_TYPES"
  emit AUTO_CAPTURE_ENABLED "$AUTO_CAPTURE_ENABLED"
  emit AUTO_CAPTURE_TYPES "$AUTO_CAPTURE_TYPES"
  emit TERMINAL_PUBLISH "$TERMINAL_PUBLISH"
  emit RECLAIM_LEASE_TTL "$RECLAIM_LEASE_TTL"
  emit RECLAIM_AUTO "$RECLAIM_AUTO"
  emit BUILD_CHECKPOINT "$BUILD_CHECKPOINT"
  emit REVIEW_MIN_FIX_SEVERITY "$REVIEW_MIN_FIX_SEVERITY"
  emit REVIEW_MAX_FIX_TASKS "$REVIEW_MAX_FIX_TASKS"
  emit GATE_OBSERVATION_BUDGET "$GATE_OBSERVATION_BUDGET"
  emit DELEGATION_OBSERVATION_BUDGET "$DELEGATION_OBSERVATION_BUDGET"
  emit SKILL_BRAINSTORM "$SKILL_BRAINSTORM"
  emit SKILL_PLAN "$SKILL_PLAN"
  emit SKILL_BUILD "$SKILL_BUILD"
  emit SKILL_REVIEW "$SKILL_REVIEW"
  emit SKILL_FINISH "$SKILL_FINISH"
  emit BOOTSTRAP "$BOOTSTRAP"
fi
