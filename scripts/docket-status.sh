#!/usr/bin/env bash
# scripts/docket-status.sh — deterministic orchestrator for the docket-status pass (change 0058).
# Sequences the shared docket scripts in one process; emits one line-oriented report on stdout.
# The report is self-evidencing: it always states what it did (`board off` when the board is
# disabled, the backlog digest, `pass ok` on completion), so stdout is never empty (change 0069).
#
# Usage: docket-status.sh [--board-only] [--repo OWNER/REPO] [--project OWNER/NUMBER]
#                          [--auto-create-project] [--project-owner OWNER]
#   --board-only           only regenerate the board surfaces; skip sweep/health passes
#   --repo OWNER/REPO      GitHub repo for PR-link resolution (defaults to origin remote)
#   --project OWNER/NUMBER GitHub Project to sync (later task)
#   --auto-create-project  create the GitHub Project if --project doesn't resolve (later task)
#   --project-owner OWNER  owner to create the project under (later task)
#
# Contract: scripts/docket-status.md.
# Mock seams: GIT="${GIT:-git}", GH="${GH:-gh}", CONFIG_EXPORT_CMD (config export override).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT="${GIT:-git}"
GH="${GH:-gh}"
SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"
# shellcheck source=lib/docket-frontmatter.sh
. "$SELF_DIR"/lib/docket-frontmatter.sh

BOARD_ONLY=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
usage(){ sed -n '2,12p' "${BASH_SOURCE[0]}"; }
while [ $# -gt 0 ]; do
  case "$1" in
    --board-only) BOARD_ONLY=1 ;;
    --repo) REPO_FLAG="$2"; shift ;;
    --project) PROJECT_FLAG="$2"; shift ;;
    --auto-create-project) AUTO_CREATE_PROJECT=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "docket-status: unknown argument: $1" >&2; exit 2 ;;
  esac; shift
done

# Config export mock seam: CONFIG_EXPORT_CMD lets tests inject a stub export.
config_export(){ ${CONFIG_EXPORT_CMD:-"$SELF_DIR"/docket-config.sh --export}; }

ensure_and_sync_worktree(){
  if [ "${DOCKET_MODE:-}" = docket ]; then
    local wt="${METADATA_WORKTREE:-.docket}"
    if [ ! -d "$wt" ]; then
      "$GIT" worktree add "$wt" "$METADATA_BRANCH" >&2 2>/dev/null \
        || "$GIT" worktree add "$wt" "origin/$METADATA_BRANCH" >&2 \
        || { echo "docket-status: cannot create metadata worktree $wt" >&2; exit 1; }
    fi
    # Change 0063: skip the repo's shared git hooks on the metadata worktree (idempotent; self-heals
    # existing installs). Best-effort — a failure here must not block the status pass.
    "$SELF_DIR"/disable-worktree-hooks.sh --worktree "$wt" >&2 \
      || echo "docket-status: warning — could not disable hooks on $wt (continuing)" >&2
    "$GIT" -C "$wt" fetch origin "$METADATA_BRANCH" >&2 \
      && "$GIT" -C "$wt" pull --rebase origin "$METADATA_BRANCH" >&2 \
      || { echo "docket-status: metadata worktree sync failed" >&2; exit 1; }
  else
    "$GIT" pull --rebase >&2 || { echo "docket-status: metadata sync failed" >&2; exit 1; }
  fi
}

board_pass(){
  local surfaces="${BOARD_SURFACES:-}"
  # Change 0069: silence is not evidence. With no surfaces configured the board pass is a
  # deliberate no-op — SAY SO, so "exit 0 + empty stdout" can never again be read as "the script
  # silently did nothing" and send an agent hunting for a BOARD.md the config forbids.
  [ -n "$surfaces" ] || { echo "board off"; return 0; }
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"
  local tok
  for tok in $surfaces; do
    case "$tok" in
      inline) board_pass_inline "$mw" "$cd_dir" ;;
      github) board_pass_github "$cd_dir" ;;
      *) echo "docket-status: unknown board surface '$tok'" >&2 ;;
    esac
  done
}

board_pass_inline(){
  local mw="$1" cd_dir="$2"
  # $rel is BOARD.md's path RELATIVE TO $mw (the metadata worktree) — the form git -C "$mw"
  # accepts (verified: a full "$mw/.../BOARD.md" pathspec fatals under git -C "$mw").
  local rel="$CHANGES_DIR/BOARD.md"
  # Render through the single gated inline-board primitive (board-refresh.sh, change 0059): it
  # owns the atomic, truncation-safe write of BOARD.md (render to temp -> chmod 644 -> rename),
  # so render-board.sh is reached ONLY via this helper. board_pass already gated on the `inline`
  # token, so pass it verbatim; a render failure leaves the prior BOARD.md untouched.
  if ! "$SELF_DIR"/board-refresh.sh --changes-dir "$cd_dir" --surfaces inline ${REPO_FLAG:+--repo "$REPO_FLAG"} >&2 2>&2; then
    echo "docket-status: board render failed; keeping existing BOARD.md" >&2
    return 0
  fi
  # board-refresh.sh wrote BOARD.md in place; commit + push only if it actually changed.
  if [ -z "$("$GIT" -C "$mw" status --porcelain -- "$rel" 2>/dev/null)" ]; then
    echo "board inline clean"
    return 0
  fi
  "$GIT" -C "$mw" add "$rel" >&2
  "$GIT" -C "$mw" commit -q -m "docket: board refresh" >&2 || true

  local attempt=0 pushed=0
  while [ $attempt -lt 5 ]; do
    attempt=$((attempt + 1))
    if "$GIT" -C "$mw" push >&2 2>&1; then
      pushed=1
      break
    fi
    if ! "$GIT" -C "$mw" pull --rebase >&2 2>&1; then
      if "$GIT" -C "$mw" status --porcelain 2>/dev/null | grep -q "BOARD.md"; then
        # Regenerate through the same gated primitive (never a raw redirect) so a rebase never
        # leaves conflict markers or an empty/truncated board.
        if ! "$SELF_DIR"/board-refresh.sh --changes-dir "$cd_dir" --surfaces inline ${REPO_FLAG:+--repo "$REPO_FLAG"} >&2 2>&2; then
          echo "docket-status: board regeneration during rebase failed; aborting rebase" >&2
          "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true
          pushed=-1
          break
        fi
        "$GIT" -C "$mw" add "$rel" >&2
        "$GIT" -C "$mw" rebase --continue >&2 2>&1 || { "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true; pushed=-1; break; }
      else
        "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true
        break
      fi
    fi
  done
  if [ $pushed -eq 1 ]; then
    echo "board inline changed pushed"
  else
    echo "board inline changed push-failed"
  fi
}

board_pass_github(){
  local cd_dir="$1"
  local out rc
  out="$("$SELF_DIR"/github-mirror.sh --changes-dir "$cd_dir" ${REPO_FLAG:+--repo "$REPO_FLAG"} ${PROJECT_FLAG:+--project "$PROJECT_FLAG"} $([ "$AUTO_CREATE_PROJECT" = 1 ] && echo --auto-create-project) ${PROJECT_OWNER:+--project-owner "$PROJECT_OWNER"} 2>&2)"
  rc=$?
  echo "$out" | while IFS= read -r line; do
    case "$line" in
      "issue-minted "*) set -- $line; echo "minted issue $2 $3" ;;
      "project-minted "*) set -- $line; echo "minted project $2 $3" ;;
    esac
  done
  if [ $rc -eq 0 ]; then
    echo "board github ok"
  else
    echo "board github failed"
  fi
}

# backlog_pass — the backlog digest (change 0069). UNGATED: it runs on BOTH paths regardless of
# BOARD_SURFACES, because the digest is REPORT OUTPUT, NOT A BOARD SURFACE. It persists
# nothing, commits nothing, pushes nothing, and never touches BOARD.md — which is exactly what
# lets `board_surfaces: []` keep meaning "no board is rendered or committed" while backlog state
# still reaches the report. Delegates to render-board.sh (--format digest), so readiness keeps
# exactly one owner and this orchestrator does not reimplement resolution. Best-effort: a render
# failure logs to stderr, emits no digest lines, and never aborts the pass.
#
# It is called ONCE PER PATH, not once globally, and the placement is load-bearing: the digest is
# a snapshot of the change files AT THE MOMENT IT RUNS. Under --board-only (no sweep) it is the
# "state as-is" projection and runs before the early exit; on a full pass it runs AFTER the sweep,
# so it projects the state the pass actually LEFT BEHIND. Running it before the sweep would make
# the report contradict itself — a change swept to `done` in the same pass would still be reported
# as `implemented`, and since the digest is the sole backlog channel that staleness has no
# corrective path.
backlog_pass(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"
  local out
  if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then
    echo "docket-status: backlog digest failed; continuing without it" >&2
    return 0
  fi
  [ -n "$out" ] && printf '%s\n' "$out"
  return 0
}

# detect_merged — batched sweep detection (change 0058, task 4). Prints TAB-separated
# "<id>\t<slug>\t<pr>\t<merged-date>" for every `implemented` change under $CD/active whose
# PR has merged, using ONE batched gh call (an aliased graphql query keyed by pr number, plus a
# per-change `gh pr list` fallback only for changes with no `pr:` set). merged-date is the UTC
# date portion of GitHub's mergedAt (already Zulu/UTC) — never now()/local time. Best-effort:
# any gh/network/parse failure emits "sweep-skipped <reason>" and returns 0 (never aborts the pass).
detect_merged(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"

  local -a files
  mapfile -t files < <(find "$cd_dir/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  [ ${#files[@]} -gt 0 ] || return 0

  local -a ids=() slugs=() prs=()
  local f id slug status pr
  for f in "${files[@]}"; do
    status="$(field "$f" status)"
    [ "$status" = implemented ] || continue
    id="$(int_field "$f" id)"
    [ -n "$id" ] || continue
    slug="$(field "$f" slug)"
    pr="$(int_field "$f" pr)"
    ids+=("$id"); slugs+=("$slug"); prs+=("$pr")
  done
  [ ${#ids[@]} -gt 0 ] || return 0

  local repo="${REPO_FLAG:-}"
  if [ -z "$repo" ]; then
    repo="$("$GH" repo view --json owner,name -q '(.owner.login)+"/"+(.name)' 2>/dev/null)" \
      || { echo "sweep-skipped gh-unavailable"; return 0; }
  fi
  local owner="${repo%%/*}" name="${repo#*/}"
  if [ -z "$owner" ] || [ -z "$name" ] || [ "$owner" = "$repo" ]; then
    echo "sweep-skipped repo-unresolved"
    return 0
  fi

  # Build one aliased graphql query for every change with a known pr: number.
  local query="query {" i has_pr=0
  for i in "${!ids[@]}"; do
    [ -n "${prs[$i]}" ] || continue
    query="$query p${ids[$i]}: repository(owner: \"$owner\", name: \"$name\") { pullRequest(number: ${prs[$i]}) { number mergedAt state } }"
    has_pr=1
  done
  query="$query }"

  local gql_json="" gql_rc=0
  if [ "$has_pr" -eq 1 ]; then
    gql_json="$("$GH" api graphql -f query="$query" 2>/dev/null)"; gql_rc=$?
    if [ $gql_rc -ne 0 ] || [ -z "$gql_json" ] || ! printf '%s' "$gql_json" | jq -e . >/dev/null 2>&1; then
      echo "sweep-skipped gh-unavailable"
      return 0
    fi
  fi

  local merged_at state date pl_json pl_num pl_merged
  for i in "${!ids[@]}"; do
    id="${ids[$i]}"; slug="${slugs[$i]}"; pr="${prs[$i]}"
    if [ -n "$pr" ]; then
      merged_at="$(printf '%s' "$gql_json" | jq -r ".data.p${id}.pullRequest.mergedAt // empty" 2>/dev/null)"
      state="$(printf '%s' "$gql_json" | jq -r ".data.p${id}.pullRequest.state // empty" 2>/dev/null)"
      if [ "$state" = MERGED ] && [ -n "$merged_at" ]; then
        date="${merged_at:0:10}"
        printf '%s\t%s\t%s\t%s\n' "$id" "$slug" "$pr" "$date"
      fi
    else
      pl_json="$("$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
      if [ $? -ne 0 ] || ! printf '%s' "$pl_json" | jq -e . >/dev/null 2>&1; then
        continue
      fi
      pl_num="$(printf '%s' "$pl_json" | jq -r '.[0].number // empty')"
      pl_merged="$(printf '%s' "$pl_json" | jq -r '.[0].mergedAt // empty')"
      if [ -n "$pl_num" ] && [ -n "$pl_merged" ]; then
        date="${pl_merged:0:10}"
        printf '%s\t%s\t%s\t%s\n' "$id" "$slug" "$pl_num" "$date"
      fi
    fi
  done
  return 0
}

# sweep_execute — chains the shared ADR-0035 close-out scripts (archive-change.sh →
# render-change-links.sh → terminal-publish.sh → cleanup-feature-branch.sh) for each merged
# change fed on stdin as TAB-separated "<id>\t<slug>\t<pr>\t<merged-date>" (detect_merged's
# format; pipe `detect_merged | sweep_execute`). Log-and-continue: any per-change step failure
# emits "sweep-failed <id> <step> <reason>" and abandons the REST of that change's close-out,
# but the loop always continues to the next change. Full success emits "swept <id> <date>" and
# "harvest <id> <archived-path>" (the archived file — a hook for the caller to harvest
# learnings). Idempotent: a change already done/archived is a silent no-op.
sweep_execute(){
  local mw cd_dir
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  cd_dir="$mw/$CHANGES_DIR"

  local id slug pr merged_date
  while IFS=$'\t' read -r id slug pr merged_date; do
    [ -n "$id" ] || continue
    # Not a valid close-out record (e.g. detect_merged's "sweep-skipped <reason>" line,
    # which carries no TAB fields) — pass it through verbatim so it reaches the report
    # instead of being silently dropped as a bogus change record.
    if ! [[ "$id" =~ ^[0-9]+$ ]]; then
      printf '%s\n' "$id"
      continue
    fi
    sweep_execute_one "$mw" "$cd_dir" "$id" "$slug" "$pr" "$merged_date"
  done
}

sweep_execute_one(){
  local mw="$1" cd_dir="$2" id="$3" slug="$4" pr="$5" merged_date="$6"
  local pad; pad="$(printf '%04d' "$id" 2>/dev/null)"
  [ -n "$pad" ] || pad="$id"

  if ! "$GIT" -C "$mw" pull --rebase >&2; then
    echo "sweep-failed $id sync pull-failed"
    return 0
  fi

  local active status
  active="$(find "$cd_dir/active" -maxdepth 1 -name "${pad}-*.md" 2>/dev/null | head -n1)"
  if [ -z "$active" ]; then
    return 0   # already archived — idempotent no-op
  fi
  status="$(field "$active" status)"
  case "$status" in
    done|killed) return 0 ;;   # already terminal — idempotent no-op
  esac

  if ! "$SCRIPTS_DIR"/archive-change.sh \
        --changes-dir "$cd_dir" --id "$id" --outcome done --date "$merged_date" \
        --message "docket($id): done — archived (status done, $merged_date)" >&2; then
    echo "sweep-failed $id archive script-error"
    return 0
  fi

  local archived
  archived="$(find "$cd_dir/archive" -maxdepth 1 -name "${merged_date}-${pad}-*.md" 2>/dev/null | head -n1)"
  if [ -z "$archived" ]; then
    echo "sweep-failed $id archive archived-file-not-found"
    return 0
  fi

  if ! "$SCRIPTS_DIR"/render-change-links.sh \
        --change-file "$archived" --adrs-dir "$mw/$ADRS_DIR" >&2; then
    echo "sweep-failed $id render-change-links skipped-publish"
    return 0
  fi
  if [ -n "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ]; then
    "$GIT" -C "$mw" add "$archived" >&2
    "$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links" >&2
    if ! "$GIT" -C "$mw" push >&2; then
      echo "sweep-failed $id render-change-links push-failed"
      return 0
    fi
  fi

  if ! "$SCRIPTS_DIR"/terminal-publish.sh \
        --id "$id" --outcome done --enabled "${TERMINAL_PUBLISH:-true}" \
        --integration-branch "$INTEGRATION_BRANCH" --metadata-branch "$METADATA_BRANCH" \
        --changes-dir "$CHANGES_DIR" --adrs-dir "$ADRS_DIR" \
        --message "docket($id): publish terminal record (done)" >&2; then
    echo "sweep-failed $id terminal-publish script-error"
    return 0
  fi

  if ! "$SCRIPTS_DIR"/cleanup-feature-branch.sh --slug "$slug" >&2; then
    echo "sweep-failed $id cleanup script-error"
  fi

  echo "swept $id $merged_date"
  echo "harvest $id $archived"
}

# health_checks — runs board-checks.sh (the mechanical git-only checks) over the current
# changes-dir and prefixes each TSV finding line as "check <check-id> <change-id> <message>".
# Best-effort: a clean tree (or a board-checks failure) prints nothing extra; never aborts the pass.
health_checks(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"
  local metadata_branch
  if [ "${DOCKET_MODE:-}" = docket ]; then metadata_branch="$METADATA_BRANCH"; else metadata_branch="$INTEGRATION_BRANCH"; fi
  "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" 2>&2 | \
  while IFS=$'\t' read -r check_id change_id message; do
    [ -n "$check_id" ] || continue
    echo "check $check_id $change_id $message"
  done
  return 0
}

# emit_judgment — one "judgment blocked <id> <blocked_by text>" line per `blocked` change under
# $CD/active. The judgment (whether the blocking reason still holds) is left to the caller/skill.
emit_judgment(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"

  local -a files
  mapfile -t files < <(find "$cd_dir/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  [ ${#files[@]} -gt 0 ] || return 0

  local f status id blocked_by
  for f in "${files[@]}"; do
    status="$(field "$f" status)"
    [ "$status" = blocked ] || continue
    id="$(field "$f" id)"
    blocked_by="$(field "$f" blocked_by)"
    echo "judgment blocked $id $blocked_by"
  done
  return 0
}

# integration_sync — best-effort FF-only sync of the invoking repo's integration-branch
# checkout, run once at the end of a pass that swept at least one change.
integration_sync(){
  "$SCRIPTS_DIR"/sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH" >&2 2>&1 || true
  return 0
}

main(){
  local cfg; cfg="$(config_export)" || { echo "docket-status: config export failed" >&2; exit 1; }
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE) echo "docket-status: repo not migrated — run migrate-to-docket.sh" >&2; exit 1 ;;
    CREATE_ORPHAN) echo "docket-status: fresh repo — bootstrap is opt-in; run a docket skill to create the docket branch" >&2; exit 1 ;;
    *) echo "docket-status: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; exit 1 ;;
  esac
  ensure_and_sync_worktree
  board_pass
  if [ "$BOARD_ONLY" = 1 ]; then
    # Change 0069: --board-only is the "just show me the backlog" path, and it runs no sweep — so
    # the digest here is the "state as-is" projection and belongs before the early exit. In a
    # board-off repo this path used to do literally nothing and return nothing; it now reports the
    # backlog in every configuration.
    backlog_pass
    echo "pass ok"
    exit 0
  fi

  local swept_count=0 line
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    printf '%s\n' "$line"
    case "$line" in
      "swept "*) swept_count=$((swept_count + 1)) ;;
    esac
  done < <(detect_merged | sweep_execute)

  health_checks
  emit_judgment
  # Change 0069: on the FULL path the digest runs AFTER the sweep, so it is the "state after the
  # pass" projection — a change swept to done this pass is reported as `done`, never as the
  # `implemented` it was when the pass began. The report must not contradict itself.
  backlog_pass
  [ "$swept_count" -gt 0 ] && integration_sync
  # Change 0069: stdout is NEVER empty on a completed pass. `pass ok` means "the orchestrator ran
  # to completion" — a hard error exits non-zero above and never reaches this line, so it stays a
  # reliable completion signal. A thin report is the success case, not a symptom.
  echo "pass ok"
  exit 0
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  main "$@"
fi
