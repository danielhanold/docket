#!/usr/bin/env bash
# scripts/render-change-links.sh — deterministic, idempotent renderer for the per-change
# `## Artifacts` link block (change 0035). Reads ONE change file's frontmatter + resolved
# config and rewrites the marker-bounded block in place. Frontmatter is the single source of
# truth; this script is the SOLE writer of the block (ADR-0012 script-vs-model boundary).
# Offline (no gh, no network); does NOT commit (the calling skill commits). Same inputs =>
# byte-identical file.
#
# Usage: render-change-links.sh --change-file FILE [--repo OWNER/REPO] [--adrs-dir DIR]
#   --repo      build GitHub blob/pull URLs; default derives OWNER/REPO from the origin remote
#               of the change file's repo. Absent/non-GitHub remote => fallback (bare paths).
#   --adrs-dir  LOCAL dir to resolve ADR slugs; default METADATA_WORKTREE/ADRS_DIR from config.
#   Mock seams: GIT="${GIT:-git}", DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}".
set -uo pipefail

START_MARKER='<!-- docket:artifacts:start (generated — do not hand-edit) -->'
END_MARKER='<!-- docket:artifacts:end -->'

GIT="${GIT:-git}"
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKET_CONFIG="${DOCKET_CONFIG:-$SCRIPTDIR/docket-config.sh}"
CHANGE_FILE=""
REPO=""
ADRS_DIR_LOCAL=""
REPO_EXPLICIT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --change-file) CHANGE_FILE="$2"; shift ;;
    --repo) REPO="$2"; REPO_EXPLICIT=1; shift ;;
    --adrs-dir) ADRS_DIR_LOCAL="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-change-links: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$CHANGE_FILE" ] || { printf 'render-change-links: missing --change-file\n' >&2; exit 2; }
[ -f "$CHANGE_FILE" ] || { printf 'render-change-links: change file not found: %s\n' "$CHANGE_FILE" >&2; exit 2; }

# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-root.sh"

# Resolve config (branches + adrs dir). Mockable via DOCKET_CONFIG.
cfg="$("$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-change-links: config resolution failed\n' >&2; exit 1; }
eval "$cfg"
METADATA_BRANCH="${METADATA_BRANCH:-docket}"
INTEGRATION_BRANCH="${INTEGRATION_BRANCH:-main}"
ADRS_DIR="${ADRS_DIR:-docs/adrs}"          # repo-relative, for URLs
METADATA_WORKTREE="${METADATA_WORKTREE:-}"

if [ -z "$ADRS_DIR_LOCAL" ]; then
  # change 0075: METADATA_WORKTREE arrives RELATIVE from the config export (".docket") and would
  # otherwise resolve against the CALLER's CWD — the same defect the $mw anchor closes in
  # docket-status.sh. Anchor it to the main worktree. (Every in-repo caller passes --adrs-dir
  # explicitly, so this is the fallback path only; audited in the same pass.)
  if [ -n "$METADATA_WORKTREE" ]; then
    ADRS_DIR_LOCAL="$(docket_anchor_path "$METADATA_WORKTREE")/$ADRS_DIR"
  else
    ADRS_DIR_LOCAL="$ADRS_DIR"
  fi
fi

# Derive OWNER/REPO + GitHub mode from the origin remote (render-board.sh pattern), unless --repo.
GITHUB=0
if [ "$REPO_EXPLICIT" = 1 ]; then
  GITHUB=1
else
  url="$("$GIT" -C "$(dirname "$CHANGE_FILE")" remote get-url origin 2>/dev/null || true)"
  case "$url" in
    git@github.com:*|https://github.com/*|ssh://git@github.com/*)
      REPO="${url%.git}"
      REPO="${REPO#git@github.com:}"; REPO="${REPO#https://github.com/}"; REPO="${REPO#ssh://git@github.com/}"
      GITHUB=1 ;;
    *) GITHUB=0 ;;
  esac
fi

blob(){ printf 'https://github.com/%s/blob/%s/%s' "$REPO" "$1" "$2"; }  # ref, repo-rel-path

# Read frontmatter (command substitution strips trailing newline — safe).
status="$(field "$CHANGE_FILE" status)"
branch="$(field "$CHANGE_FILE" branch)"
spec="$(field "$CHANGE_FILE" spec)"
plan="$(field "$CHANGE_FILE" plan)"
results="$(field "$CHANGE_FILE" results)"
pr="$(field "$CHANGE_FILE" pr)"
adrs="$(list_field "$CHANGE_FILE" adrs)"   # space-separated ids, "" when [] / unset

# plan/results ref: integration branch once done, else the feature branch.
build_ref="$branch"
[ "$status" = "done" ] && build_ref="$INTEGRATION_BRANCH"

# True only when $1 looks like a URL (has a scheme). Used to avoid emitting a broken
# markdown link from a malformed, non-URL `pr:` (e.g. a bare number); the convention sets a
# full URL, but the renderer never produces a broken link on bad input. Empty => false.
is_url(){ case "$1" in *://*) return 0 ;; *) return 1 ;; esac; }

# Emit one artifact row to stdout (nothing if it must be omitted). $1 label, $2 path.
build_row(){
  local label="$1" path="$2" text; text="$(basename "$path")"
  if [ "$GITHUB" != 1 ]; then printf '| %s | `%s` |\n' "$label" "$path"; return; fi
  if [ "$status" = "killed" ]; then
    # feature branch gone, not merged: link to the PR if it's a URL; a non-URL pr renders the
    # filename as plain text (no broken link); no pr at all => omit the row.
    if is_url "$pr"; then printf '| %s | [%s](%s) |\n' "$label" "$text" "$pr"
    elif [ -n "$pr" ]; then printf '| %s | %s |\n' "$label" "$text"; fi
    return
  fi
  printf '| %s | [%s](%s) |\n' "$label" "$text" "$(blob "$build_ref" "$path")"
}

rows=""
# Spec — always on METADATA_BRANCH.
if [ -n "$spec" ]; then
  if [ "$GITHUB" = 1 ]; then rows+="| Spec | [$(basename "$spec")]($(blob "$METADATA_BRANCH" "$spec")) |"$'\n'
  else rows+="| Spec | \`$spec\` |"$'\n'; fi
fi
# Plan / Results — lifecycle-pinned (build_row).
[ -n "$plan" ]    && rows+="$(build_row Plan "$plan")"$'\n'
[ -n "$results" ] && rows+="$(build_row Results "$results")"$'\n'
# PR — a URL renders as [#NN](url) in GitHub mode; anything else (non-GitHub mode, or a
# non-URL/malformed pr) renders verbatim, never a broken link.
if [ -n "$pr" ]; then
  if [ "$GITHUB" = 1 ] && is_url "$pr"; then
    num="${pr##*/}"; rows+="| PR | [#$num]($pr) |"$'\n'
  else
    rows+="| PR | $pr |"$'\n'
  fi
fi
# ADRs — each id on METADATA_BRANCH; slug resolved from the local ADR file; missing => dir link.
if [ -n "$adrs" ]; then
  adr_cell=""
  for id in $adrs; do
    padded="$(printf '%04d' "$id")"
    m=( "$ADRS_DIR_LOCAL"/"${padded}"-*.md )   # glob, not `ls | head` (pipefail-safe)
    if [ -e "${m[0]}" ]; then
      relpath="$ADRS_DIR/$(basename "${m[0]}")"
      if [ "$GITHUB" = 1 ]; then link="[ADR-$padded]($(blob "$METADATA_BRANCH" "$relpath"))"; else link="\`$relpath\`"; fi
    else
      if [ "$GITHUB" = 1 ]; then link="[ADR-$padded]($(blob "$METADATA_BRANCH" "$ADRS_DIR"))"; else link="ADR-$padded"; fi
    fi
    if [ -n "$adr_cell" ]; then adr_cell+=", $link"; else adr_cell="$link"; fi
  done
  rows+="| ADRs | $adr_cell |"$'\n'
fi

# build_row may emit an empty line (killed + no pr). Strip blank lines from rows.
rows="$(printf '%s' "$rows" | sed '/^$/d')"
[ -n "$rows" ] && rows="$rows"$'\n'

# Assemble the marker-bounded block into a temp file.
block_file="$(mktemp)"; trap 'rm -f "$block_file"' EXIT
{
  printf '%s\n' "$START_MARKER"
  if [ -n "$rows" ]; then printf '| Artifact | Link |\n|---|---|\n'; printf '%s' "$rows"; fi
  printf '%s\n' "$END_MARKER"
} > "$block_file"

out="$(mktemp)"
if grep -qF "$START_MARKER" "$CHANGE_FILE"; then
  # Replace inclusive marker block (fixed-string match via index()).
  awk -v startm="$START_MARKER" -v endm="$END_MARKER" -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) block = block line ORS }
    index($0, startm) { printf "%s", block; inblk=1; next }
    inblk && index($0, endm) { inblk=0; next }
    !inblk { print }
  ' "$CHANGE_FILE" > "$out"
else
  # Insert as the first body section, right after the frontmatter close (2nd ---).
  awk -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) block = block line ORS }
    { print }
    /^---[[:space:]]*$/ { n++; if (n==2) { print ""; print "## Artifacts"; print ""; printf "%s", block } }
  ' "$CHANGE_FILE" > "$out"
fi
mv "$out" "$CHANGE_FILE"
