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
#
# Usage: docket-config.sh [--export] [--bootstrap] [--repo-dir DIR]
#   --export        emit resolved KEY=value lines (default mode)
#   --bootstrap     additionally perform the CREATE_ORPHAN write when the verdict is
#                   CREATE_ORPHAN (fresh repo); a no-op in every other cell
#   --repo-dir DIR  operate on the git repo at DIR (default: .) — the test/mock seam
#   -h, --help      print this header
# Mock seam: GIT="${GIT:-git}".
set -uo pipefail

GIT="${GIT:-git}"
MODE=export
DO_BOOTSTRAP=0
REPO_DIR="."
while [ $# -gt 0 ]; do
  case "$1" in
    --export)    MODE=export ;;
    --bootstrap) DO_BOOTSTRAP=1 ;;
    --repo-dir)  [ $# -ge 2 ] || { printf 'docket-config: --repo-dir requires an argument\n' >&2; exit 2; }
                 REPO_DIR="$2"; shift ;;
    -h|--help)   grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'docket-config: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

die() { printf 'docket-config: %s\n' "$*" >&2; exit 1; }
g()   { "$GIT" -C "$REPO_DIR" "$@"; }
emit(){ printf '%s=%q\n' "$1" "$2"; }

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
# unintended lines. Nested finalize.gate / finalize.test_command are read by their unique
# leaf-key name. NOTE: a value may not contain a literal '#' — it is treated as the start of an
# inline comment and truncated (fine for the current enum / path / empty values).
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

# --- Stage 1: resolve origin/HEAD + default branch (keyed on fetch/set-head rc) ---
g rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "not a git repo: $REPO_DIR"
g fetch --quiet origin 2>/dev/null || die "cannot reach origin (git fetch failed) — check the remote/network"
g remote set-head origin -a >/dev/null 2>&1 || die "cannot resolve origin/HEAD (git remote set-head failed)"
DEFAULT_BRANCH="$(g symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || true)"
DEFAULT_BRANCH="${DEFAULT_BRANCH#origin/}"
[ -n "$DEFAULT_BRANCH" ] || die "origin/HEAD is unresolvable after set-head"

# --- Stage 2: read + resolve .docket.yml (authoritative via git show origin/HEAD) ---
CFG="$(mktemp)"; trap 'rm -f "$CFG"' EXIT
g show "origin/HEAD:.docket.yml" >"$CFG" 2>/dev/null || : >"$CFG"   # absent file => defaults (NOT an error)

# --- Stage 2b: global config layer (change 0050) ------------------------------
# ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — the full .docket.yml schema,
# resolved PER-KEY: per-repo > global > built-in (map-valued skills: merges field-by-field).
# Read from the LOCAL filesystem — the file is per-machine by definition, so there is no
# authoritative-ref concern as with .docket.yml's origin/HEAD read. Coordination keys are
# fenced (warned-and-ignored) in Stage 2c below.
GCFG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/docket"
GCFG="$GCFG_DIR/config.yml"
gbl(){ yaml_get "$GCFG" "$1"; }   # global-layer scalar read (empty when absent)

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
FINALIZE_GATE="$(yaml_get "$CFG" gate)";      FINALIZE_GATE="${FINALIZE_GATE:-$(gbl gate)}"; FINALIZE_GATE="${FINALIZE_GATE:-local}"
FINALIZE_TEST_COMMAND="$(yaml_get "$CFG" test_command)"; FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"
AUTO_GROOM="$(yaml_get "$CFG" auto_groom)";   AUTO_GROOM="${AUTO_GROOM:-$(gbl auto_groom)}"; AUTO_GROOM="${AUTO_GROOM:-false}"

bs_raw="$(yaml_get "$CFG" board_surfaces)"; bs_from_global=0
if [ -z "$bs_raw" ]; then
  bs_raw="$(gbl board_surfaces)"
  [ -n "$bs_raw" ] && bs_from_global=1
fi
if [ -z "$bs_raw" ]; then
  BOARD_SURFACES="inline"                                  # unset in both layers => default [inline]
else
  bs="${bs_raw#[}"; bs="${bs%]}"; bs="${bs//,/ }"
  BOARD_SURFACES="$(echo $bs)"                             # trim/collapse; "[]" => ""
fi

# --- skills: role-keyed pluggable workflow skills (change 0049 + 0050 global layer) ---
# Nested block; each leaf read within the block only. Per-key precedence:
# per-repo leaf > global leaf > the superpowers default.
SKILLS_BLK="$(mktemp)";  yaml_block_body "$CFG"  skills >"$SKILLS_BLK"
GSKILLS_BLK="$(mktemp)"; yaml_block_body "$GCFG" skills >"$GSKILLS_BLK"
skill_role(){  # skill_role <role> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$SKILLS_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GSKILLS_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
SKILL_BRAINSTORM="$(skill_role brainstorm superpowers:brainstorming)"
SKILL_PLAN="$(skill_role plan superpowers:writing-plans)"
SKILL_BUILD="$(skill_role build superpowers:subagent-driven-development)"
SKILL_REVIEW="$(skill_role review superpowers:requesting-code-review)"
SKILL_FINISH="$(skill_role finish superpowers:finishing-a-development-branch)"
# Unknown role keys in EITHER layer: warn-and-ignore (a typo must never abort).
for _blk in "$SKILLS_BLK" "$GSKILLS_BLK"; do
  while IFS= read -r _role; do
    [ -n "$_role" ] || continue
    case " brainstorm plan build review finish " in
      *" $_role "*) ;;
      *) printf 'docket-config: warning: unknown skills role %s — ignored\n' "$_role" >&2 ;;
    esac
  done < <(sed -n -E 's/^[[:space:]]*([[:alnum:]_-]+)[[:space:]]*:.*/\1/p' "$_blk")
done
rm -f "$SKILLS_BLK" "$GSKILLS_BLK"

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
    BOOTSTRAP=PROCEED   # the repo is now migrated; the caller may proceed
  fi
fi

# --- emit ---
if [ "$MODE" = export ]; then
  emit DOCKET_MODE "$DOCKET_MODE"
  emit DEFAULT_BRANCH "$DEFAULT_BRANCH"
  emit METADATA_BRANCH "$METADATA_BRANCH"
  emit INTEGRATION_BRANCH "$INTEGRATION_BRANCH"
  emit METADATA_WORKTREE "$METADATA_WORKTREE"
  emit CHANGES_DIR "$CHANGES_DIR"
  emit ADRS_DIR "$ADRS_DIR"
  emit RESULTS_DIR "$RESULTS_DIR"
  emit FINALIZE_GATE "$FINALIZE_GATE"
  emit FINALIZE_TEST_COMMAND "$FINALIZE_TEST_COMMAND"
  emit BOARD_SURFACES "$BOARD_SURFACES"
  emit AUTO_GROOM "$AUTO_GROOM"
  emit SKILL_BRAINSTORM "$SKILL_BRAINSTORM"
  emit SKILL_PLAN "$SKILL_PLAN"
  emit SKILL_BUILD "$SKILL_BUILD"
  emit SKILL_REVIEW "$SKILL_REVIEW"
  emit SKILL_FINISH "$SKILL_FINISH"
  emit BOOTSTRAP "$BOOTSTRAP"
fi
