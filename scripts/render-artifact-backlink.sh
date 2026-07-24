#!/usr/bin/env bash
# scripts/render-artifact-backlink.sh — deterministic, idempotent renderer for the `docket:backlink`
# block stamped at the TOP of an artifact (spec, plan, or results) pointing HOME to its change file
# on metadata_branch (change 0136). The reciprocal of render-change-links.sh's forward `## Artifacts`
# block, sharing its idioms. Frontmatter (id, title) + the change-file path are the single source of
# truth; this script is the SOLE writer of the block (ADR-0012 script-vs-model boundary). Offline
# (no gh, no network); does NOT commit (the calling skill/script commits). Same inputs =>
# byte-identical file.
#
# Usage: render-artifact-backlink.sh --artifact-file FILE --change-file CHANGE [--repo OWNER/REPO]
#   --artifact-file  the spec/plan/results markdown file to update in place.
#   --change-file    the change file at its CURRENT canonical path (active/… while live, archive/…
#                    once terminal). id + title are read from its frontmatter; the URL path is
#                    derived from this path (so terminal_publish never changes the link TARGET).
#   --repo           build GitHub blob URLs; default derives OWNER/REPO from the artifact file's
#                    origin remote. Absent/non-GitHub remote => bare-path fallback.
#   Mock seams: GIT="${GIT:-git}", DOCKET_CONFIG="${DOCKET_CONFIG:-<scriptdir>/docket-config.sh}".
set -uo pipefail

START_MARKER='<!-- docket:backlink:start (generated — do not hand-edit) -->'
END_MARKER='<!-- docket:backlink:end -->'

GIT="${GIT:-git}"
SCRIPTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -n "${DOCKET_CONFIG:-}" ]; then DOCKET_CONFIG_EXPLICIT=1; else DOCKET_CONFIG_EXPLICIT=0; DOCKET_CONFIG="$SCRIPTDIR/docket-config.sh"; fi
ARTIFACT_FILE=""
CHANGE_FILE=""
REPO=""
REPO_EXPLICIT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --artifact-file) ARTIFACT_FILE="$2"; shift ;;
    --change-file) CHANGE_FILE="$2"; shift ;;
    --repo) REPO="$2"; REPO_EXPLICIT=1; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-artifact-backlink: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$ARTIFACT_FILE" ] || { printf 'render-artifact-backlink: missing --artifact-file\n' >&2; exit 2; }
[ -f "$ARTIFACT_FILE" ] || { printf 'render-artifact-backlink: artifact file not found: %s\n' "$ARTIFACT_FILE" >&2; exit 2; }
[ -n "$CHANGE_FILE" ]   || { printf 'render-artifact-backlink: missing --change-file\n' >&2; exit 2; }
[ -f "$CHANGE_FILE" ]   || { printf 'render-artifact-backlink: change file not found: %s\n' "$CHANGE_FILE" >&2; exit 2; }

# shellcheck source=/dev/null
source "$SCRIPTDIR/lib/docket-frontmatter.sh"

# Resolve config (metadata_branch + changes_dir). Mockable via DOCKET_CONFIG.
if [ "$DOCKET_CONFIG_EXPLICIT" -eq 1 ]; then
  cfg="$("$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-artifact-backlink: config resolution failed\n' >&2; exit 1; }
else
  cfg="$("${DOCKET_BASH_PATH:?run docket/install.sh}" "$DOCKET_CONFIG" --export 2>/dev/null)" || { printf 'render-artifact-backlink: config resolution failed\n' >&2; exit 1; }
fi
eval "$cfg"
METADATA_BRANCH="${METADATA_BRANCH:-docket}"
CHANGES_DIR="${CHANGES_DIR:-docs/changes}"

# Derive OWNER/REPO + GitHub mode from the artifact file's origin remote (render-change-links pattern),
# unless --repo is explicit.
GITHUB=0
if [ "$REPO_EXPLICIT" = 1 ]; then
  GITHUB=1
else
  url="$("$GIT" -C "$(dirname "$ARTIFACT_FILE")" remote get-url origin 2>/dev/null || true)"
  case "$url" in
    git@github.com:*|https://github.com/*|ssh://git@github.com/*)
      REPO="${url%.git}"
      REPO="${REPO#git@github.com:}"; REPO="${REPO#https://github.com/}"; REPO="${REPO#ssh://git@github.com/}"
      GITHUB=1 ;;
    *) GITHUB=0 ;;
  esac
fi

# Read id + title from the change frontmatter. fm_field is FRONTMATTER-SCOPED (first ---…--- block
# only): id/title are mandatory keys, but in a repo whose subject matter IS field names a body line
# opening `title:`/`id:` must never win (LEARNINGS frontmatter-anchored-read).
id="$(fm_field "$CHANGE_FILE" id)"
title="$(fm_field "$CHANGE_FILE" title)"
padded="$(printf '%04d' "$id" 2>/dev/null)" || padded="$id"

# Canonical repo-relative path of the change file, derived from the path the caller passed (its
# CURRENT canonical location — active/… or archive/…). Deterministic + offline.
sub="$(basename "$(dirname "$CHANGE_FILE")")"     # active | archive
relpath="$CHANGES_DIR/$sub/$(basename "$CHANGE_FILE")"

# Assemble the marker-bounded block into a temp file. The model-authored title is written with
# printf '%s' — VERBATIM, never a sed/string interpolation (which reinterprets & and \1 in a real
# title); the awk step below inserts the block bytes literally (LEARNINGS
# model-authored-values-are-untrusted-input). fm_field returns a single line => no newline injection.
block_file="$(mktemp)"; trap 'rm -f "$block_file"' EXIT
{
  printf '%s\n' "$START_MARKER"
  if [ "$GITHUB" = 1 ]; then
    printf '> ↩ **[Change %s — %s](https://github.com/%s/blob/%s/%s)**\n' "$padded" "$title" "$REPO" "$METADATA_BRANCH" "$relpath"
  else
    printf '> ↩ **Change %s — %s** — `%s`\n' "$padded" "$title" "$relpath"
  fi
  printf '%s\n' "$END_MARKER"
} > "$block_file"

out="$(mktemp)"
if grep -qF "$START_MARKER" "$ARTIFACT_FILE"; then
  # Replace the inclusive marker region in place (fixed-string match via index()).
  awk -v startm="$START_MARKER" -v endm="$END_MARKER" -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) block = block line ORS }
    index($0, startm) { printf "%s", block; inblk=1; next }
    inblk && index($0, endm) { inblk=0; next }
    !inblk { print }
  ' "$ARTIFACT_FILE" > "$out"
else
  # Insert the block as the very first lines, then one blank line, then the original content.
  awk -v blk="$block_file" '
    BEGIN { while ((getline line < blk) > 0) printf "%s\n", line; print "" }
    { print }
  ' "$ARTIFACT_FILE" > "$out"
fi
mv "$out" "$ARTIFACT_FILE"
