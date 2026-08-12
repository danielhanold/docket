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
if [ -n "${DOCKET_CONFIG:-}" ]; then DOCKET_CONFIG_EXPLICIT=1; else DOCKET_CONFIG_EXPLICIT=0; DOCKET_CONFIG="$SCRIPTDIR/docket-config.sh"; fi
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
if [ "$DOCKET_CONFIG_EXPLICIT" -eq 1 ]; then
  cfg="$("$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-change-links: config resolution failed\n' >&2; exit 1; }
else
  cfg="$("${DOCKET_BASH_PATH:?run docket/install.sh}" "$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-change-links: config resolution failed\n' >&2; exit 1; }
fi
eval "$cfg"
METADATA_BRANCH="${METADATA_BRANCH:-docket}"
INTEGRATION_BRANCH="${INTEGRATION_BRANCH:-main}"
ADRS_DIR="${ADRS_DIR:-docs/adrs}"          # repo-relative, for URLs
CHANGES_DIR="${CHANGES_DIR:-docs/changes}" # repo-relative, for the derived Stacked children links
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
# The five optional keys take the ANCHORED read (ADR-0057; the rule table in
# lib/docket-frontmatter.sh). Absent from frontmatter, an unanchored field() runs past the closing
# --- and returns body prose — and this renderer stamps what it reads into specs, plans, results
# files and PR bodies.
branch="$(fm_field "$CHANGE_FILE" branch)"
spec="$(fm_field "$CHANGE_FILE" spec)"
plan="$(fm_field "$CHANGE_FILE" plan)"
results="$(fm_field "$CHANGE_FILE" results)"
pr="$(fm_field "$CHANGE_FILE" pr)"
adrs="$(list_field "$CHANGE_FILE" adrs)"   # space-separated ids, "" when [] / unset

# plan/results ref: integration branch once done, else the feature branch.
#
# The test is `= done` and deliberately NOT "is terminal" / "is not active" (change 0298). Those
# three used to coincide; `stacked-merged` splits them. A stacked-merged change has merged into its
# PARENT's branch, not into the integration branch, so its plan and results are not reachable from
# `$INTEGRATION_BRANCH` yet — the feature branch is still the only ref that resolves them, and it
# stays alive precisely because the stack root needs the code. Widening this condition to any
# non-active or non-`done` status would silently produce 404 links for the whole stack.
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

# --- Stacked children — DERIVED at render time, never stored (change 0298) -----------------------
# The parent side of a `stacked_on:` link has no field of its own. Denormalizing it would store one
# fact in two files and let them disagree, and the disagreement would stay invisible until a branch
# was cut from the wrong base. So the row is a scan of the changes directory, recomputed on every
# render, and a child that moves or is killed simply stops appearing.
#
# The directory to scan is derived from THIS change file's own location — a change lives at
# `<changes>/active/NNNN-slug.md` or `<changes>/archive/DATE-NNNN-slug.md`. A caller pointing at a
# file outside that shape finds no children rather than walking somewhere unrelated.
cf_dir="$(cd "$(dirname "$CHANGE_FILE")" && pwd)"
case "$(basename "$cf_dir")" in
  active|archive) changes_root="$(dirname "$cf_dir")" ;;
  *)              changes_root="$cf_dir" ;;
esac
self_id="$(field "$CHANGE_FILE" id)"
child_lines=""
case "$self_id" in
  ''|*[!0-9]*) ;;
  *)
    self_id=$(( 10#$self_id ))
    cand=()
    for c in "$changes_root"/active/*.md "$changes_root"/archive/*.md; do
      [ -f "$c" ] && cand+=("$c")
    done
    # PREFILTER, never the decision. One grep replaces one `fm_field` subprocess per change file,
    # which at real-repo scale (hundreds of changes, and this renderer runs on every frontmatter
    # write) is the difference between a millisecond and a second per render. It is keyed on the
    # SHAPE `fm_field` itself requires — a line whose first characters are `stacked_on:` — so it is
    # a strict SUPERSET of the files fm_field can answer non-empty for, and it deliberately does not
    # look at the VALUE: `stacked_on: 0030  # note` and `stacked_on: 30` must both survive it, and
    # so must the body-prose false positives the anchored read below is here to reject.
    stacked_files=""
    if [ "${#cand[@]}" -gt 0 ]; then
      stacked_files="$(grep -l -E -e '^stacked_on:' -- "${cand[@]}" || true)"
    fi
    while IFS= read -r c; do
      [ -n "$c" ] || continue
      # ANCHORED read — this is the decision. `stacked_on:` is optional — every change file minted
      # before change 0298 lacks the line — and this repo's change bodies discuss the field name in
      # ordinary prose. An unanchored read runs past the closing `---` and invents children out of
      # that prose, which is exactly what the prefilter above hands it.
      c_parent="$(fm_field "$c" stacked_on)"
      [ -n "$c_parent" ] || continue
      case "$c_parent" in (*[!0-9]*) continue ;; esac
      [ "$(( 10#$c_parent ))" = "$self_id" ] || continue
      c_id="$(field "$c" id)"
      case "$c_id" in ''|*[!0-9]*) continue ;; esac
      c_id=$(( 10#$c_id ))
      # A change stacked on itself is a cycle for the health checks to name, never a child here.
      [ "$c_id" != "$self_id" ] || continue
      c_padded="$(printf '%04d' "$c_id")"
      c_rel="$CHANGES_DIR/$(basename "$(dirname "$c")")/$(basename "$c")"
      if [ "$GITHUB" = 1 ]; then
        c_cell="[#$c_padded]($(blob "$METADATA_BRANCH" "$c_rel")) $(field "$c" title) ($(field "$c" status))"
      else
        c_cell="#$c_padded $(field "$c" title) ($(field "$c" status))"
      fi
      child_lines+="$c_padded"$'\t'"$c_cell"$'\n'
    done <<<"$stacked_files"
    ;;
esac
if [ -n "$child_lines" ]; then
  # Sort by padded id: `active/` globs in id order but `archive/` globs by DATE, so the two halves
  # would otherwise interleave by accident of merge dates and the block would not be deterministic.
  child_sorted="$(printf '%s' "$child_lines" | sort)"
  child_cell=""
  while IFS=$'\t' read -r _c_pad c_text; do
    [ -n "$c_text" ] || continue
    if [ -n "$child_cell" ]; then child_cell+=", $c_text"; else child_cell="$c_text"; fi
  done <<<"$child_sorted"
  # Emit nothing at all — not even the label — for an empty set. A labelled row with no names is a
  # different defect from an omitted row: it asserts the change HAS children and lost them.
  [ -n "$child_cell" ] && rows+="| Stacked children | $child_cell |"$'\n'
fi

# build_row may emit an empty line (killed + no pr). Strip blank lines from rows.
rows="$(printf '%s' "$rows" | sed '/^$/d')"
[ -n "$rows" ] && rows="$rows"$'\n'

# Assemble the marker-bounded block into a temp file.
block_file="$(mktemp "${TMPDIR:-/tmp}/render-change-links.XXXXXX")"; trap 'rm -f "$block_file"' EXIT
{
  printf '%s\n' "$START_MARKER"
  if [ -n "$rows" ]; then printf '| Artifact | Link |\n|---|---|\n'; printf '%s' "$rows"; fi
  printf '%s\n' "$END_MARKER"
} > "$block_file"

out="$(mktemp "${TMPDIR:-/tmp}/render-change-links.XXXXXX")"
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
mv -f "$out" "$CHANGE_FILE"
