#!/usr/bin/env bash
# migrate-to-docket.sh — one-shot, idempotent migration of a single-branch (`main`-mode)
# repo to docket-mode: a dedicated orphan `docket` branch becomes the authoritative working
# surface for ALL planning state (changes, board, ADRs, specs), while the integration branch
# (main | develop) keeps only code, build artifacts, and PUBLISHED TERMINAL RECORDS.
#
#   bash migrate-to-docket.sh
#
# What it does (spec §9):
#   1. Resolve config (.docket.yml or defaults) and print it.
#   2. Verify preconditions: clean tree; the live planning surface present on the integration
#      branch; and NO `docket` branch already on origin (already-migrated repos are adopted via
#      the §6 worktree path, not re-orphaned here).
#   3. Seed an orphan `docket` branch from the current planning dirs AS WHOLE DIRECTORIES
#      (<changes_dir>/, <adrs_dir>/, docs/superpowers/specs/, BOARD.md) — NOT <results_dir>/
#      or docs/superpowers/plans/ (those are feature-branch build artifacts). Commit + push.
#   4. Prune the live surface (<changes_dir>/active/, the changes README, BOARD.md) from the
#      integration branch; KEEP terminal records (<changes_dir>/archive/, <adrs_dir>/ + its
#      index README) and build artifacts (<results_dir>/, docs/superpowers/plans/). Commit + push.
#   5. Extend .gitignore with .docket/ + .worktrees/ (idempotent).
#   6. Print next steps.
#
# This is a ONE-TIME, per-repo operation, but it is fully IDEMPOTENT and interrupted-run safe:
# re-running it from any partial state converges. Each step is a mutation guarded on its own
# LOCAL filesystem/branch postcondition, plus a separately-guarded push (push only when
# origin/<branch> differs from the local tip). Removes are tolerant (git rm --ignore-unmatch).
# No history rewrite, no force-push, no rm -rf outside a temp dir this script created.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# ---------------------------------------------------------------------------
# Output helpers
# ---------------------------------------------------------------------------
say()  { printf '%s\n' "$*"; }
step() { printf '\n==> %s\n' "$*"; }
die()  { printf 'migrate-to-docket: %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 1. Resolve config — .docket.yml if present, else defaults.
# ---------------------------------------------------------------------------
# Minimal scalar reader for `key: value` (strips inline `# comments`, quotes, whitespace).
# We intentionally avoid a YAML dependency; .docket.yml is a flat scalar file.
yaml_get() {  # yaml_get <file> <key>
  [ -f "$1" ] || return 1
  sed -n -E "s/^[[:space:]]*$2[[:space:]]*:[[:space:]]*([^#]*).*/\1/p" "$1" \
    | head -n1 \
    | sed -E 's/[[:space:]]+$//; s/^"(.*)"$/\1/; s/^'\''(.*)'\''$/\1/'
}

default_integration_branch() {
  # origin/HEAD → strip leading "origin/"; fall back to main if undetectable.
  local head
  head="$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null || true)"
  head="${head#origin/}"
  printf '%s\n' "${head:-main}"
}

CONFIG_FILE=".docket.yml"
INTEGRATION_BRANCH="$(yaml_get "$CONFIG_FILE" integration_branch || true)"
CHANGES_DIR="$(yaml_get "$CONFIG_FILE" changes_dir || true)"
ADRS_DIR="$(yaml_get "$CONFIG_FILE" adrs_dir || true)"
RESULTS_DIR="$(yaml_get "$CONFIG_FILE" results_dir || true)"

# `auto` (or absent) → resolve to origin/HEAD; an explicit value is used verbatim.
if [ -z "${INTEGRATION_BRANCH:-}" ] || [ "$INTEGRATION_BRANCH" = "auto" ]; then
  INTEGRATION_BRANCH="$(default_integration_branch)"
fi
CHANGES_DIR="${CHANGES_DIR:-docs/changes}"
ADRS_DIR="${ADRS_DIR:-docs/adrs}"
RESULTS_DIR="${RESULTS_DIR:-docs/results}"

SPECS_DIR="docs/superpowers/specs"
PLANS_DIR="docs/superpowers/plans"
INTEGRATION_REF="origin/$INTEGRATION_BRANCH"

# BOARD.md lives under <changes_dir>/ in the documented layout; tolerate a repo-root BOARD.md.
if [ -f "$CHANGES_DIR/BOARD.md" ]; then
  BOARD="$CHANGES_DIR/BOARD.md"
else
  BOARD="BOARD.md"
fi
CHANGES_README="$CHANGES_DIR/README.md"

step "Resolved configuration"
say  "  integration_branch : $INTEGRATION_BRANCH  (code lands here; ref $INTEGRATION_REF)"
say  "  changes_dir        : $CHANGES_DIR"
say  "  adrs_dir           : $ADRS_DIR"
say  "  results_dir        : $RESULTS_DIR  (stays on integration — NOT seeded onto docket)"
say  "  specs_dir          : $SPECS_DIR"
say  "  board              : $BOARD"
if [ -f "$CONFIG_FILE" ]; then
  say "  (source: $CONFIG_FILE)"
else
  say "  (source: defaults — no $CONFIG_FILE present)"
fi

# ---------------------------------------------------------------------------
# Probes — ALWAYS via `git ls-tree <ref> -- <paths>` (never bare <ref>:<path>,
# which misreports against the working dir). ls-tree exit≠0 ⇒ the ref itself is
# absent ⇒ a hard config error, NOT "path absent".
# ---------------------------------------------------------------------------
require_ref() {  # require_ref <ref>  — abort if the ref does not resolve
  git rev-parse --verify --quiet "$1^{commit}" >/dev/null \
    || die "ref '$1' does not resolve. Set integration_branch correctly in $CONFIG_FILE and ensure the branch is pushed to origin."
}

tree_has() {  # tree_has <ref> -- <paths...>  : 0 if any path present on <ref>; ref-absent ⇒ die
  local ref="$1"; shift
  [ "$1" = "--" ] && shift
  local out rc
  out="$(git ls-tree "$ref" -- "$@" 2>/dev/null)"; rc=$?
  if [ "$rc" -ne 0 ]; then
    die "git ls-tree could not read '$ref' (exit $rc) — the ref is absent/unreadable. This is a config error (check integration_branch), not an empty path."
  fi
  [ -n "$out" ]
}

# ---------------------------------------------------------------------------
# 2. Preconditions
# ---------------------------------------------------------------------------
step "Checking preconditions"

# Clean working tree (no staged or unstaged changes).
if [ -n "$(git status --porcelain)" ]; then
  die "working tree is not clean — commit or stash changes first, then re-run."
fi

# The integration ref must resolve (hard config error otherwise).
require_ref "$INTEGRATION_REF"

# The live planning surface must be present on the integration branch. The probe set EQUALS
# the prune set below (active/, changes README, board) — NOT the seed set — so a correctly
# migrated repo (which keeps archive/ + adrs/ + specs on integration) does not read LIVE.
if ! tree_has "$INTEGRATION_REF" -- "$CHANGES_DIR/active" "$CHANGES_README" "$BOARD"; then
  say "  The live planning surface (active/, changes README, board) is already absent from"
  say "  $INTEGRATION_REF — this repo appears to be migrated (or never had a live surface)."
  # Not necessarily an error: a half-migrated repo (docket exists locally, prune done) can
  # reach here on re-run. We continue; the guarded steps below are all no-ops in that case.
else
  say "  Live planning surface present on $INTEGRATION_REF — ok."
fi

# Abort if origin/docket already exists: the repo is already migrated. Adopt the existing
# branch via the §6 "branch on origin, not local" worktree path — do NOT create a divergent
# orphan here.
git fetch --quiet origin docket 2>/dev/null || true
if git rev-parse --verify --quiet refs/remotes/origin/docket >/dev/null; then
  cat >&2 <<EOF
migrate-to-docket: origin/docket already exists — this repo is already migrated.
Do NOT re-run migration (it would create a divergent orphan). To start working in
docket-mode in this (or any) clone, adopt the existing remote branch:

    git fetch origin docket
    git worktree add .docket --track -b docket origin/docket

(The docket skills do this automatically at startup — see spec §6, "branch on origin, not local".)
EOF
  exit 1
fi
say "  origin/docket does not exist — ok to seed."

# ---------------------------------------------------------------------------
# Local-state helpers for the idempotency split.
#   MUTATION guards probe the LOCAL tree/branch.
#   PUSH guards probe origin/<branch> vs the local tip.
# ---------------------------------------------------------------------------
local_branch_exists() { git rev-parse --verify --quiet "refs/heads/$1^{commit}" >/dev/null; }

# Transient worktrees live in a temp dir OUTSIDE the repo (no .gitignore entry, no
# .worktrees/ slug-collision or prune hazard). Track + clean them up on exit.
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/migrate-to-docket.XXXXXX")"
cleanup() {
  # Best-effort teardown of any worktree we registered, then the temp dir.
  if [ -n "${DOCKET_WT:-}" ] && [ -d "$DOCKET_WT" ]; then
    git worktree remove --force "$DOCKET_WT" 2>/dev/null || true
  fi
  if [ -n "${PRUNE_WT:-}" ] && [ -d "$PRUNE_WT" ]; then
    git worktree remove --force "$PRUNE_WT" 2>/dev/null || true
  fi
  git worktree prune 2>/dev/null || true
  rm -rf "$TMP_ROOT" 2>/dev/null || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# 3. Seed the orphan `docket` branch (MUTATION guarded on the LOCAL branch/paths).
# ---------------------------------------------------------------------------
step "Seeding the orphan 'docket' branch"

# Seed set: whole planning directories + the board. NOT results_dir / plans (build artifacts).
# Only include paths that actually exist on the integration ref (a fresh repo may lack adrs/).
SEED_CANDIDATES=("$CHANGES_DIR" "$ADRS_DIR" "$SPECS_DIR" "$BOARD")
seed_paths=()
for p in "${SEED_CANDIDATES[@]}"; do
  if tree_has "$INTEGRATION_REF" -- "$p"; then
    seed_paths+=("$p")
  else
    say "  (skip seed: '$p' not on $INTEGRATION_REF)"
  fi
done
[ "${#seed_paths[@]}" -gt 0 ] || die "nothing to seed — no planning dirs found on $INTEGRATION_REF."

DOCKET_WT="$TMP_ROOT/docket"

if ! local_branch_exists docket; then
  # Branch on neither local nor origin → create the orphan and seed everything in one commit.
  say "  Local 'docket' branch absent — creating orphan and seeding from $INTEGRATION_REF."
  git worktree prune 2>/dev/null || true
  git worktree add --orphan -b docket "$DOCKET_WT" >/dev/null
  # The orphan starts empty; populate its index+tree from the integration ref.
  git -C "$DOCKET_WT" checkout "$INTEGRATION_REF" -- "${seed_paths[@]}"
  git -C "$DOCKET_WT" commit --quiet -m "docket: seed metadata branch from $INTEGRATION_BRANCH (migrate-to-docket.sh)"
  say "  Seeded docket with: ${seed_paths[*]}"
else
  # Local 'docket' already exists (interrupted prior run) — top up only MISSING seed paths.
  say "  Local 'docket' branch exists — verifying seed paths (top up only what is missing)."
  git worktree prune 2>/dev/null || true
  git worktree add "$DOCKET_WT" docket >/dev/null
  missing=()
  for p in "${seed_paths[@]}"; do
    if ! tree_has refs/heads/docket -- "$p"; then
      missing+=("$p")
    fi
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    say "  Topping up missing seed paths: ${missing[*]}"
    git -C "$DOCKET_WT" checkout "$INTEGRATION_REF" -- "${missing[@]}"
    git -C "$DOCKET_WT" diff --cached --quiet \
      || git -C "$DOCKET_WT" commit --quiet -m "docket: top up seeded metadata (migrate-to-docket.sh)"
  else
    say "  All seed paths already present on docket — no seeding needed."
  fi
fi

# PUSH guard: push docket only if origin/docket differs from (or is absent vs) the local tip.
local_docket="$(git rev-parse --verify --quiet refs/heads/docket || true)"
origin_docket="$(git rev-parse --verify --quiet refs/remotes/origin/docket || true)"
if [ -n "$local_docket" ] && [ "$local_docket" != "$origin_docket" ]; then
  say "  Pushing docket → origin/docket."
  git -C "$DOCKET_WT" push -u origin docket
else
  say "  origin/docket already matches local docket — no push needed."
fi

# Done with the docket worktree; remove it now (cleanup() also covers the failure path).
git worktree remove --force "$DOCKET_WT" 2>/dev/null || true
DOCKET_WT=""

# ---------------------------------------------------------------------------
# 4. Prune the live planning surface from the integration branch (MUTATION guarded on
#    the LOCAL integration tree; tolerant `git rm` so a redundant re-run is a no-op).
# ---------------------------------------------------------------------------
step "Pruning the live planning surface from '$INTEGRATION_BRANCH'"

# Prune set: active/, the changes README (links to the now-docket-only board), and the board.
# KEEP: archive/, adrs/ (+ its index README), results_dir/, plans/ (terminal records + artifacts).
PRUNE_PATHS=("$CHANGES_DIR/active" "$CHANGES_README" "$BOARD")

# We must operate on the integration branch's working tree. Provision a transient worktree on
# a throwaway local branch tracking the integration ref — the main tree never switches branches.
PRUNE_WT="$TMP_ROOT/prune"
git worktree prune 2>/dev/null || true
git worktree add -B "migrate-prune-$INTEGRATION_BRANCH" "$PRUNE_WT" "$INTEGRATION_REF" >/dev/null

# Probe the LOCAL (worktree) tree: only paths still present there need removing. Probing origin
# would re-`rm` an already-pushed-but-locally-pruned path on re-run (a hard error) — hence local.
prune_targets=()
for p in "${PRUNE_PATHS[@]}"; do
  if tree_has "migrate-prune-$INTEGRATION_BRANCH" -- "$p"; then
    prune_targets+=("$p")
  fi
done

if [ "${#prune_targets[@]}" -gt 0 ]; then
  say "  Removing from $INTEGRATION_BRANCH: ${prune_targets[*]}"
  # Tolerant remove: --ignore-unmatch makes a redundant re-run a no-op, not a failure.
  git -C "$PRUNE_WT" rm -r --quiet --ignore-unmatch -- "${prune_targets[@]}" >/dev/null
  if ! git -C "$PRUNE_WT" diff --cached --quiet; then
    git -C "$PRUNE_WT" commit --quiet -m "docket: prune live planning surface (moved to docket branch; migrate-to-docket.sh)"
  fi
  say "  Kept on $INTEGRATION_BRANCH: $CHANGES_DIR/archive/, $ADRS_DIR/, $RESULTS_DIR/, $PLANS_DIR/"
else
  say "  Live planning surface already pruned from $INTEGRATION_BRANCH — nothing to remove."
fi

# ---------------------------------------------------------------------------
# 5. Extend .gitignore on the integration branch (idempotent — add only if absent).
# ---------------------------------------------------------------------------
step "Ensuring .gitignore ignores .docket/ and .worktrees/"
GITIGNORE="$PRUNE_WT/.gitignore"
touch "$GITIGNORE"
added_ignore=0
for entry in ".docket/" ".worktrees/"; do
  # Match an existing line with or without the trailing slash (e.g. ".docket" or ".docket/").
  pat="^${entry%/}/?$"
  if ! grep -qE "$pat" "$GITIGNORE"; then
    printf '%s\n' "$entry" >> "$GITIGNORE"
    say "  Added '$entry' to .gitignore"
    added_ignore=1
  else
    say "  '$entry' already in .gitignore"
  fi
done
if [ "$added_ignore" -eq 1 ]; then
  git -C "$PRUNE_WT" add .gitignore
  if ! git -C "$PRUNE_WT" diff --cached --quiet; then
    git -C "$PRUNE_WT" commit --quiet -m "docket: gitignore .docket/ and .worktrees/ (migrate-to-docket.sh)"
  fi
fi

# PUSH guard for the integration branch: push only if origin/<integration_branch> differs from
# the local prune-branch tip. (Guarding on "did the mutation run" would strand an unpushed
# prune/gitignore commit; guarding on the SHA converges from a mutate-but-not-pushed state.)
local_integration="$(git -C "$PRUNE_WT" rev-parse @)"
origin_integration="$(git rev-parse --verify --quiet "refs/remotes/$INTEGRATION_REF" || true)"
if [ "$local_integration" != "$origin_integration" ]; then
  step "Pushing '$INTEGRATION_BRANCH' → $INTEGRATION_REF"
  # Push HEAD explicitly to the integration branch (the worktree's local branch is the throwaway
  # migrate-prune-* ref, not <integration_branch>).
  git -C "$PRUNE_WT" push origin "HEAD:$INTEGRATION_BRANCH"
else
  step "Integration branch already up to date"
  say  "  $INTEGRATION_REF already matches the pruned/gitignored tree — no push needed."
fi

# Teardown of the prune worktree + its throwaway branch (cleanup() also covers failures).
git worktree remove --force "$PRUNE_WT" 2>/dev/null || true
git branch -D "migrate-prune-$INTEGRATION_BRANCH" >/dev/null 2>&1 || true
PRUNE_WT=""

# ---------------------------------------------------------------------------
# 6. Next steps
# ---------------------------------------------------------------------------
step "Migration complete"
cat <<EOF
  This repo is now in docket-mode:
    - origin/docket holds the live planning surface (changes, board, ADRs, specs).
    - $INTEGRATION_REF keeps code, build artifacts, and published terminal records.

  Next steps:
    - The docket skills now operate in docket-mode automatically — they ensure a persistent
      .docket/ worktree parked on the docket branch at startup (spec §6) and push every
      metadata commit to origin/docket.
    - A .docket.yml is OPTIONAL. Defaults are docket-mode with integration_branch=auto
      (resolved to origin/HEAD). Pin metadata_branch: main only to opt back out to
      single-branch mode.
    - Re-running this script is safe: every step is idempotent and converges from any
      partial state.
EOF
