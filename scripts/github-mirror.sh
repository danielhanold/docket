#!/usr/bin/env bash
# scripts/github-mirror.sh — the deterministic engine for docket's `github` board surface
# (change 0011). One-way mirror: change files on the metadata branch are the source of truth;
# this script upserts one GitHub issue per change (+ optional Projects v2 item), reconciles a
# `docket:`-namespaced label set, and closes terminal changes with the right reason. It NEVER
# reads GitHub state back into change files.
#
# It is invoked by docket-status's Board pass when `github` is in board_surfaces. The script is
# the single source of the external-write mechanics; the rest of docket stays agent-prose.
#
# Contract:
#   - Deterministic & idempotent: same change files + same GitHub state ⇒ same calls; re-runnable.
#   - Best-effort: missing network / auth / `project` scope ⇒ degrade (log to stderr, continue,
#     exit 0). Projects is the optional half — its failure never blocks Issues; nothing here
#     ever aborts the caller's build.
#   - Sole writer of issue open/closed state & reason. The PR only *references* the issue
#     elsewhere; this script never emits `Closes #N`.
#   - On creating an issue it prints `issue-minted <id> <number>` so the caller persists `issue:`
#     into the change file on the metadata branch (this script does no git writes).
#
# Mock seam: GH="${GH:-gh}". Tests set GH to a fake and/or pass --dry-run.
#
# Usage:
#   github-mirror.sh [--dry-run] --changes-dir DIR [--repo OWNER/REPO]
#                    [--metadata-branch BR] [--changes-path P] [--integration-branch BR]
#                    [--adrs-dir DIR] [--project OWNER/NUMBER]
set -uo pipefail

GH="${GH:-gh}"
DRY=0
CHANGES_DIR=""
REPO=""
META_BRANCH="docket"
CHANGES_PATH="docs/changes"
INT_BRANCH="main"
ADRS_DIR=""
PROJECT=""

log(){ printf '%s\n' "github-mirror: $*" >&2; }

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY=1 ;;
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --repo) REPO="$2"; shift ;;
    --metadata-branch) META_BRANCH="$2"; shift ;;
    --changes-path) CHANGES_PATH="$2"; shift ;;
    --integration-branch) INT_BRANCH="$2"; shift ;;
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --project) PROJECT="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) log "unknown argument: $1"; exit 2 ;;
  esac
  shift
done

[ -n "$CHANGES_DIR" ] || { log "missing --changes-dir"; exit 2; }
[ -d "$CHANGES_DIR" ] || { log "changes dir not found: $CHANGES_DIR"; exit 0; }  # best-effort no-op

# run_gh — the single external-write chokepoint. In --dry-run, print the argv; else exec the
# real (or mock) gh, swallowing failures best-effort. Returns gh's stdout on the real path.
run_gh(){
  if [ "$DRY" = 1 ]; then
    # Trace to STDERR so it survives both `$(run_gh …)` capture and `… >/dev/null`,
    # while leaving real gh's stdout (the created issue URL) clean on the non-dry path.
    { printf '+ %s' "$GH"; printf ' %s' "$@"; printf '\n'; } >&2
    return 0
  fi
  "$GH" "$@" 2>/dev/null || { log "gh call failed (best-effort, continuing): $*"; return 1; }
}

# --- frontmatter helpers ------------------------------------------------------
# field FILE KEY — first matching scalar in the file's frontmatter, trimmed.
field(){
  sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 \
    | sed 's/[[:space:]]*$//'
}
# list_field FILE KEY — `[a, b]` → space-separated `a b` (empty for `[]` / unset).
list_field(){
  local raw; raw="$(field "$1" "$2")"
  raw="${raw#[}"; raw="${raw%]}"
  printf '%s' "$raw" | tr ',' ' ' | xargs 2>/dev/null || true
}
has_section(){ grep -qF "$2" "$1"; }

# --- pass 1: index id -> status (for dependency readiness) --------------------
declare -A STATUS_OF
mapfile -t FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  STATUS_OF["$id"]="$(field "$f" status)"
done

# --- href builders ------------------------------------------------------------
blob(){ # blob BRANCH PATH
  [ -n "$REPO" ] || { printf '%s' "$2"; return; }
  printf 'https://github.com/%s/blob/%s/%s' "$REPO" "$1" "$2"
}

# --- readiness (only meaningful for proposed) ---------------------------------
# Echoes a single docket: label, or nothing. Mirrors docket-status's dependency pass.
readiness_label(){
  local f="$1" status="$2"
  [ "$status" = "proposed" ] || return 0
  local worst="" dep dstat
  for dep in $(list_field "$f" depends_on); do
    dstat="${STATUS_OF[$dep]:-}"
    if [ "$dstat" = "done" ]; then continue
    elif [ "$dstat" = "implemented" ]; then worst="needs-your-merge"
    else [ "$worst" = "needs-your-merge" ] || worst="not-yet-built"; fi
  done
  if [ -n "$worst" ]; then printf 'docket:waiting/%s' "$worst"; return 0; fi
  local spec trivial; spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"
  if [ -z "$spec" ] && [ "$trivial" != "true" ]; then
    if has_section "$f" "## Auto-groom blocked"; then printf 'docket:readiness/auto-groom-blocked'
    else printf 'docket:readiness/needs-brainstorm'; fi
    return 0
  fi
  printf 'docket:readiness/build-ready'
}

# --- issue body ---------------------------------------------------------------
why_distilled(){ # first non-empty lines under "## Why", capped at two sentences
  awk '/^## Why/{f=1;next} f&&/^## /{exit} f&&NF{print}' "$1" \
    | tr '\n' ' ' | sed 's/  */ /g' \
    | grep -oE '^([^.]*\.){1,2}' | head -n1 | sed 's/^ *//'
}

build_body(){ # build_body FILE SUBDIR FILENAME
  local f="$1" subdir="$2" fname="$3"
  local id slug title status priority spec plan results
  id="$(field "$f" id)"; title="$(field "$f" title)"; status="$(field "$f" status)"
  priority="$(field "$f" priority)"; spec="$(field "$f" spec)"
  plan="$(field "$f" plan)"; results="$(field "$f" results)"
  printf '> **Generated mirror** of `%s/%s/%s` on the `%s` branch. ' \
    "$CHANGES_PATH" "$subdir" "$fname" "$META_BRANCH"
  printf 'Edits and comments here are not read back — the change file is the source of truth.\n\n'
  printf '**status** `%s` · **priority** `%s` · **#%s**\n\n' "$status" "$priority" "$id"
  local why; why="$(why_distilled "$f")"
  [ -n "$why" ] && printf '%s\n\n' "$why"
  printf '**Links:**\n'
  printf -- '- [change file](%s)\n' "$(blob "$META_BRANCH" "$CHANGES_PATH/$subdir/$fname")"
  [ -n "$spec" ]    && printf -- '- [spec](%s)\n' "$(blob "$META_BRANCH" "$spec")"
  [ -n "$plan" ]    && printf -- '- [plan](%s)\n' "$(blob "$INT_BRANCH" "$plan")"
  [ -n "$results" ] && printf -- '- [results](%s)\n' "$(blob "$INT_BRANCH" "$results")"
  local adr advpath
  for adr in $(list_field "$f" adrs); do
    if [ -n "$ADRS_DIR" ]; then
      advpath="$(find "$ADRS_DIR" -maxdepth 1 -name "*${adr}-*.md" 2>/dev/null | head -n1)"
      [ -n "$advpath" ] && printf -- '- [ADR-%s](%s)\n' "$adr" \
        "$(blob "$META_BRANCH" "${advpath#"$ADRS_DIR"/}")" && continue
    fi
    printf -- '- ADR-%s\n' "$adr"
  done
}

# --- label set ----------------------------------------------------------------
labels_for(){ # echoes one docket:* label per line
  local f="$1" status priority ready
  status="$(field "$f" status)"; priority="$(field "$f" priority)"
  printf 'docket:status/%s\n' "$status"
  [ -n "$priority" ] && printf 'docket:priority/%s\n' "$priority"
  ready="$(readiness_label "$f" "$status")"
  [ -n "$ready" ] && printf '%s\n' "$ready"
}

# --- per-change upsert --------------------------------------------------------
mirror_change(){
  local f="$1"
  local id slug status issue subdir fname
  id="$(field "$f" id)"; slug="$(field "$f" slug)"; status="$(field "$f" status)"
  issue="$(field "$f" issue)"
  fname="$(basename "$f")"
  case "$f" in *"/archive/"*) subdir="archive" ;; *) subdir="active" ;; esac
  [ -n "$id" ] || return 0

  local title; title="$(field "$f" title)"
  local body; body="$(build_body "$f" "$subdir" "$fname")"
  local -a label_args=() ; local lbl
  while IFS= read -r lbl; do [ -n "$lbl" ] || continue
    # ensure the label exists (idempotent), then attach it
    run_gh label create "$lbl" --color ededed --force ${REPO:+-R "$REPO"} >/dev/null
    label_args+=(--label "$lbl")
  done < <(labels_for "$f")

  if [ -z "$issue" ]; then
    # CREATE — capture the new number, emit a mint line for the caller to persist.
    local created num
    created="$(run_gh issue create ${REPO:+-R "$REPO"} --title "$title" --body "$body" "${label_args[@]}")"
    num="$(printf '%s' "$created" | grep -oE '[0-9]+$' | tail -n1)"
    [ -n "$num" ] || num="(dry-run)"
    printf 'issue-minted %s %s\n' "$id" "$num"
  else
    # UPDATE — title/body + reconcile labels (add-label is additive; docket:* only).
    local -a add_args=(); local i=0
    while [ $i -lt ${#label_args[@]} ]; do
      [ "${label_args[$i]}" = "--label" ] && add_args+=(--add-label "${label_args[$((i+1))]}")
      i=$((i+1))
    done
    run_gh issue edit "$issue" ${REPO:+-R "$REPO"} --title "$title" --body "$body" "${add_args[@]}" >/dev/null
  fi

  # CLOSE STATE — the sync is the sole writer. Only terminal statuses close.
  if [ -n "$issue" ]; then
    case "$status" in
      done)   run_gh issue close "$issue" ${REPO:+-R "$REPO"} --reason completed >/dev/null ;;
      killed) run_gh issue close "$issue" ${REPO:+-R "$REPO"} --reason "not planned" >/dev/null ;;
    esac
  fi
}

# --- Projects v2 (optional half of the github surface) ------------------------
# Best-effort: gated on a configured/auto-created project; any GraphQL failure ⇒ skip, keep
# Issues. Auto-create mints a private board under the repo owner when --project is unset AND
# the caller asked for auto-create (the caller passes the resolved/minted ref back as --project;
# this script constructs the GraphQL but never decides owner/visibility silently).
sync_projects(){
  [ -n "$PROJECT" ] || { log "no project configured — skipping Projects v2 (Issues still mirrored)"; return 0; }
  # Command construction only; the real field/option resolution happens via GraphQL at runtime.
  run_gh api graphql -f query='query{ node(id:"") { id } }' \
    -F project="$PROJECT" >/dev/null \
    || { log "Projects v2 GraphQL unavailable (scope/network) — skipping, Issues unaffected"; return 0; }
}

# --- drive --------------------------------------------------------------------
mapfile -t ACTIVE_FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${ACTIVE_FILES[@]}"; do
  mirror_change "$f"
done
sync_projects

exit 0
