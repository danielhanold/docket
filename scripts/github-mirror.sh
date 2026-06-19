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
#                    [--auto-create-project [--project-owner OWNER]]
#
# Projects v2 (optional half of the `github` surface):
#   --project OWNER/NUMBER   sync items into an existing board.
#   --auto-create-project    when --project is unset, mint a PRIVATE board under the repo owner
#                            (or --project-owner), seed a "Docket Status" single-select field, and
#                            print `project-minted <owner> <number>` for the caller to write back
#                            into .docket.yml on the default branch (this script does no git writes).
#                            Auto-create is opt-in so a bare/ad-hoc run never silently mints a board.
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
AUTOCREATE=0                 # --auto-create-project: mint a board when PROJECT is unset
PROJECT_OWNER=""             # --project-owner: override the auto-create owner (default: REPO owner)
PROJECT_TITLE="docket backlog"
# Distinct from the ProjectV2 default "Status" field so auto-create never collides with it —
# same namespacing instinct as the docket: label set. Options are the five non-terminal statuses
# (terminal done/killed are expressed by closing the issue, not a column).
STATUS_FIELD_NAME="Docket Status"
STATUS_OPTIONS="proposed,in-progress,blocked,deferred,implemented"

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
    --auto-create-project) AUTOCREATE=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) log "unknown argument: $1"; exit 2 ;;
  esac
  shift
done

[ -n "$CHANGES_DIR" ] || { log "missing --changes-dir"; exit 2; }
[ -d "$CHANGES_DIR" ] || { log "changes dir not found: $CHANGES_DIR"; exit 0; }  # best-effort no-op

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

# Wrong-tree guard (best-effort, never aborts): an empty active/ next to a populated archive/ is
# the signature of the pruned integration-branch checkout — the live backlog lives only on the
# metadata branch (the .docket/ worktree in docket-mode). Mirroring that tree would create/refresh
# issues for archived changes only and miss every active one, so warn loudly and continue.
_n_active=$(find "$CHANGES_DIR/active"  -maxdepth 1 -name '*.md' 2>/dev/null | wc -l | tr -d ' ')
_n_archive=$(find "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | wc -l | tr -d ' ')
if [ "$_n_active" = 0 ] && [ "$_n_archive" != 0 ]; then
  log "WARNING: '$CHANGES_DIR/active' is empty but archive/ has $_n_archive change(s) — this looks"
  log "         like an integration-branch checkout, not the metadata worktree. The live backlog"
  log "         lives on the '$META_BRANCH' branch (the .docket/ worktree). Point --changes-dir there."
fi

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

# --- pass 1: dependency resolution + issue index ------------------------------
resolve_deps "$CHANGES_DIR"     # populates STATUS_OF / DEP_STATE / DEP_REASON / DEP_ON
declare -A ISSUE_NUM            # id -> issue number; seeded from issue:, updated on a fresh mint
mapfile -t FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  iss="$(field "$f" issue)"; [ -n "$iss" ] && ISSUE_NUM["$id"]="$iss"
done

# --- href builders ------------------------------------------------------------
blob(){ # blob BRANCH PATH
  [ -n "$REPO" ] || { printf '%s' "$2"; return; }
  printf 'https://github.com/%s/blob/%s/%s' "$REPO" "$1" "$2"
}

# --- readiness label (maps the shared readiness token to the docket: label) ---
readiness_label(){
  local f="$1" status="$2" id tok
  [ "$status" = "proposed" ] || return 0
  id="$(field "$f" id)"; tok="$(readiness "$f")"
  case "$tok" in
    waiting)
      local r="${DEP_REASON[$id]:-not yet built}"
      printf 'docket:waiting/%s' "${r// /-}" ;;     # "needs your merge" -> needs-your-merge
    auto-groom-blocked) printf 'docket:readiness/auto-groom-blocked' ;;
    needs-brainstorm)   printf 'docket:readiness/needs-brainstorm' ;;
    build-ready)        printf 'docket:readiness/build-ready' ;;
  esac
}

# --- issue body ---------------------------------------------------------------
why_distilled(){ # first non-empty lines under "## Why", capped at two sentences
  awk '/^## Why/{f=1;next} f&&/^## /{exit} f&&NF{print}' "$1" \
    | tr '\n' ' ' | sed 's/  */ /g' \
    | grep -oE '^([^.]*\.){1,2}' | head -n1 | sed 's/^ *//'
}

build_body(){ # build_body FILE SUBDIR FILENAME
  local f="$1" subdir="$2" fname="$3"
  local id title status priority spec plan results
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
  local id status issue subdir fname
  id="$(field "$f" id)"; status="$(field "$f" status)"
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

  # eff_issue — the number we act on for close-state below: the existing issue: if set, else the
  # one we mint here. Keying close on this (not just the pre-existing field) lets a change that is
  # ALREADY terminal on its first sync (e.g. mirroring a backlog with done/killed history) close in
  # the same pass, instead of being created open and only closing on a later run.
  local eff_issue="$issue"
  if [ -z "$issue" ]; then
    # CREATE — capture the new number, emit a mint line for the caller to persist.
    local created num
    created="$(run_gh issue create ${REPO:+-R "$REPO"} --title "$title" --body "$body" "${label_args[@]}")"
    num="$(printf '%s' "$created" | grep -oE '[0-9]+$' | tail -n1)"
    if [ "$DRY" = 1 ]; then
      printf 'issue-minted %s (dry-run)\n' "$id"          # number unknown without a real create
    elif [ -n "$num" ]; then
      printf 'issue-minted %s %s\n' "$id" "$num"          # caller persists this into issue:
      ISSUE_NUM["$id"]="$num"                              # so Projects can link it this same pass
      eff_issue="$num"                                     # and so close-state can act this pass
    else
      log "issue create returned no number for #$id (best-effort) — issue: not minted this pass"
    fi
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
  if [ -n "$eff_issue" ]; then
    case "$status" in
      done)   run_gh issue close "$eff_issue" ${REPO:+-R "$REPO"} --reason completed >/dev/null ;;
      killed) run_gh issue close "$eff_issue" ${REPO:+-R "$REPO"} --reason "not planned" >/dev/null ;;
    esac
  fi
}

# --- Projects v2 (optional half of the github surface) ------------------------
# Best-effort and idempotent: gated on a configured (--project) or auto-created
# (--auto-create-project) board. Built on native `gh project` subcommands (themselves GraphQL
# under the hood, but far more robust than hand-rolled mutations). Any failure — missing `project`
# scope, network, an owner we can't resolve — logs and returns 0, leaving Issues fully mirrored;
# Projects never blocks Issues and `github` never blocks a build.
#
# Auto-create is opt-in (the caller decides owner/visibility, never the bare script): when
# --project is unset and --auto-create-project is given, mint a PRIVATE board under the repo
# owner (or --project-owner), seed the "Docket Status" single-select field, and print
# `project-minted <owner> <number>` for the Board pass to write back into .docket.yml. The script
# itself does no git writes — same contract as `issue-minted`.

# proj_field_id OWNER NUMBER — node id of our Status field (placeholder under --dry-run).
proj_field_id(){
  [ "$DRY" = 1 ] && { printf 'DRYFIELD'; return; }
  run_gh project field-list "$2" --owner "$1" --format json \
    --jq ".fields[]|select(.name==\"$STATUS_FIELD_NAME\")|.id" 2>/dev/null | head -n1
}
# proj_option_id OWNER NUMBER STATUS — option id for one status value within our field.
proj_option_id(){
  [ "$DRY" = 1 ] && { printf 'DRYOPT'; return; }
  run_gh project field-list "$2" --owner "$1" --format json \
    --jq ".fields[]|select(.name==\"$STATUS_FIELD_NAME\").options[]|select(.name==\"$3\")|.id" 2>/dev/null | head -n1
}
# proj_node_id OWNER NUMBER — the project's own node id (needed by item-edit).
proj_node_id(){
  [ "$DRY" = 1 ] && { printf 'DRYPID'; return; }
  run_gh project view "$2" --owner "$1" --format json --jq '.id' 2>/dev/null | head -n1
}

sync_projects(){
  local owner number pid fid
  if [ -n "$PROJECT" ]; then
    owner="${PROJECT%%/*}"; number="${PROJECT##*/}"
  elif [ "$AUTOCREATE" = 1 ]; then
    owner="${PROJECT_OWNER:-${REPO%%/*}}"
    [ -n "$owner" ] || { log "Projects: --auto-create-project needs an owner (--repo or --project-owner) — skipping, Issues unaffected"; return 0; }
    local cj
    cj="$(run_gh project create --owner "$owner" --title "$PROJECT_TITLE" --format json)" \
      || { log "Projects: board create failed (scope/network) — skipping, Issues unaffected"; return 0; }
    if [ "$DRY" = 1 ]; then
      number="DRYNUM"
      printf 'project-minted %s (dry-run)\n' "$owner"     # number unknown without a real create
    else
      number="$(printf '%s' "$cj" | grep -oE '"number":[0-9]+' | head -n1 | grep -oE '[0-9]+')"
      [ -n "$number" ] || { log "Projects: board create returned no number — skipping this pass"; return 0; }
      printf 'project-minted %s %s\n' "$owner" "$number"  # caller writes github_project into .docket.yml
    fi
    # Seed the docket Status single-select field with the five active statuses.
    run_gh project field-create "$number" --owner "$owner" --name "$STATUS_FIELD_NAME" \
      --data-type SINGLE_SELECT --single-select-options "$STATUS_OPTIONS" --format json >/dev/null \
      || log "Projects: Status field create failed (best-effort) — items still added, status left unset"
  else
    log "no project configured — skipping Projects v2 (Issues still mirrored)"; return 0
  fi

  [ -n "$REPO" ] || { log "Projects: --repo missing (needed to build issue URLs) — board left empty this pass"; return 0; }
  fid="$(proj_field_id "$owner" "$number")"
  pid="$(proj_node_id "$owner" "$number")"

  # Add every mirrored issue as a board item and set its Status from the change's status.
  # Terminal changes (done/killed) are expressed by the closed issue, so they get no column value.
  local id num st url ij itemid oid
  for id in $(printf '%s\n' "${!ISSUE_NUM[@]}" | sort -n); do
    num="${ISSUE_NUM[$id]}"; [ -n "$num" ] || continue
    url="https://github.com/$REPO/issues/$num"
    ij="$(run_gh project item-add "$number" --owner "$owner" --url "$url" --format json)" || continue
    st="${STATUS_OF[$id]:-}"
    case "$st" in done|killed|"") continue ;; esac
    [ -n "$pid" ] && [ -n "$fid" ] || continue
    if [ "$DRY" = 1 ]; then itemid="DRYITEM"
    else itemid="$(printf '%s' "$ij" | grep -oE '"id":"[^"]+"' | head -n1 | sed 's/.*:"//;s/"$//')"; fi
    [ -n "$itemid" ] || continue
    oid="$(proj_option_id "$owner" "$number" "$st")"
    [ -n "$oid" ] || continue
    run_gh project item-edit --id "$itemid" --project-id "$pid" --field-id "$fid" \
      --single-select-option-id "$oid" --format json >/dev/null
  done
}

# --- drive --------------------------------------------------------------------
mapfile -t ACTIVE_FILES < <(find "$CHANGES_DIR/active" "$CHANGES_DIR/archive" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
for f in "${ACTIVE_FILES[@]}"; do
  mirror_change "$f"
done
sync_projects

exit 0
