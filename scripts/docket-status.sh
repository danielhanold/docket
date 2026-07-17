#!/usr/bin/env bash
# scripts/docket-status.sh — deterministic orchestrator for the docket-status pass (change 0058).
# Sequences the shared docket scripts in one process; emits one line-oriented report on stdout.
# The report is self-evidencing: it always states what it did (`board off` when the board is
# disabled, the backlog digest, `pass ok` on completion), so stdout is never empty (change 0069).
#
# Usage: docket-status.sh [--board-only] [--must-land] [--repo OWNER/REPO] [--project OWNER/NUMBER]
#                          [--auto-create-project] [--project-owner OWNER]
#   --board-only           only regenerate the board surfaces; skip sweep/health passes
#   --must-land            (with --board-only) retry a push-failed board write in-script and
#                          map the outcome to the exit code (0 = board landed); see docket-status.md
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
# shellcheck source=lib/docket-preflight.sh
. "$SELF_DIR"/lib/docket-preflight.sh

BOARD_ONLY=0 MUST_LAND=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
usage(){ sed -n '2,15p' "${BASH_SOURCE[0]}"; }
while [ $# -gt 0 ]; do
  case "$1" in
    --board-only) BOARD_ONLY=1 ;;
    --must-land) MUST_LAND=1 ;;
    --repo) REPO_FLAG="$2"; shift ;;
    --project) PROJECT_FLAG="$2"; shift ;;
    --auto-create-project) AUTO_CREATE_PROJECT=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "docket-status: unknown argument: $1" >&2; exit 2 ;;
  esac; shift
done

board_pass(){
  local surfaces="${BOARD_SURFACES:-}"
  # Change 0071 — the polarity reversal, at its reference implementation. This guard used to read
  # `[ -n "$surfaces" ] || { echo "board off"; return 0; }` — i.e. an UNRESOLVED config produced
  # the DISABLED behavior, silently, with a success exit code. That is the bug. docket-config.sh
  # now never emits an empty BOARD_SURFACES (the off-state is the positive token `none`), so an
  # empty value here means exactly one thing: nobody resolved this. Fail closed and loudly —
  # main() runs board_pass FIRST, so a hard exit here never reaches `pass ok`.
  if [ -z "$surfaces" ]; then
    echo "docket-status: BOARD_SURFACES is empty — config was never resolved (a wiring bug). The deliberate off-state is 'none'." >&2
    exit 2
  fi
  # Change 0071 review, finding 6 — defence-in-depth: a whitespace-only value (e.g. " ") passes the
  # `-z` check above but tokenizes to zero words below, the same "nobody resolved this" hole with a
  # byte of padding. Not reachable from docket-config.sh today (its own `echo $bs` word-splitting
  # already collapses whitespace to true-empty), but treat it identically on principle — the same
  # failure shape finding 1 closes ("no line at all") must not have a second door.
  set -- $surfaces
  if [ $# -eq 0 ]; then
    echo "docket-status: BOARD_SURFACES is empty — config was never resolved (a wiring bug). The deliberate off-state is 'none'." >&2
    exit 2
  fi
  # `none` is the reserved, EXCLUSIVE off-token: it disables every surface. Its report line is
  # byte-identical to the pre-0071 `board off` — a disabled repo's output must not change.
  local tok
  for tok in $surfaces; do
    if [ "$tok" = none ]; then
      if [ "$surfaces" != none ]; then
        echo "docket-status: 'none' is exclusive — it cannot be combined with other surfaces: $surfaces" >&2
        exit 2
      fi
      echo "board off"
      return 0
    fi
  done
  local mw
  # ABSOLUTE (change 0075), via the one owner of root resolution (lib/docket-root.sh, reachable
  # because lib/docket-preflight.sh sources it). A RELATIVE mw resolves against the CALLER's CWD —
  # which misresolves from a linked worktree, and is what left the artifacts-refresh block in
  # sweep_execute_one dead (its `git -C "$mw"` pathspec carried the same `.docket/` prefix the -C
  # had already entered, so it matched nothing). Every mw resolution site in this file uses this.
  mw="$(docket_metadata_worktree)"
  local cd_dir="$mw/$CHANGES_DIR"
  for tok in $surfaces; do
    case "$tok" in
      inline) board_pass_inline "$mw" "$cd_dir" ;;
      github) board_pass_github "$cd_dir" ;;
      # Change 0071 review, finding 1 — a typo'd/unknown token used to warn on stderr only, which
      # left the report-line channel with a silent exit-0 gap: a must-land caller keying on the
      # stdout report line (never the exit code, per the convention) saw no line at all and had no
      # way to distinguish "the board landed" from "this token was silently ignored". Emit a
      # positive stdout line alongside the stderr warning so the channel stays total — closing the
      # exact hole the report-line contract exists to prevent.
      *) echo "docket-status: unknown board surface '$tok'" >&2; echo "board $tok unknown" ;;
    esac
  done
}

# board_classify BOARD_OUT — reduces captured board-pass stdout to one verdict (change 0085):
#   failed    — any non-retryable board failure line, or NO board line at all (sole-channel:
#               "no line" is never success)
#   retryable — at least one `board inline changed push-failed` and no non-retryable failure
#   success   — every `board …` line is a terminal success line
# Precedence: failed > retryable > success. Non-`board ` lines (minted …, digest) are ignored.
board_classify(){
  local out="$1" line has_retryable=0 has_failed=0 has_board=0
  while IFS= read -r line; do
    case "$line" in
      "board "*) has_board=1 ;;
      *) continue ;;
    esac
    case "$line" in
      "board inline changed pushed"|"board inline clean"|"board off"|"board github ok") ;;
      "board inline changed push-failed") has_retryable=1 ;;
      *) has_failed=1 ;;   # board inline failed | board github failed | board <tok> unknown | anything else
    esac
  done <<<"$out"
  if [ "$has_board" -eq 0 ] || [ "$has_failed" -eq 1 ]; then echo failed
  elif [ "$has_retryable" -eq 1 ]; then echo retryable
  else echo success; fi
}

# board_pass_must_land — the --must-land wrapper (change 0085). Runs board_pass; on the SOLE
# retryable outcome (`board inline changed push-failed`) re-syncs the metadata worktree and
# re-renders, up to 3 attempts total. Returns 0 iff every emitted `board …` line is a terminal
# success line; prints the report line(s) each attempt and returns non-zero on any other terminal
# line or on retry exhaustion. board_pass's fail-closed `exit 2` (unresolved config) is captured
# via the command substitution's exit status and propagated verbatim. Flagless callers never reach
# this — main() invokes board_pass directly, byte for byte as before.
board_pass_must_land(){
  local mw board_out rc attempt=0 verdict
  mw="$(docket_metadata_worktree)"
  while :; do
    attempt=$((attempt + 1))
    board_out="$(board_pass)"; rc=$?
    [ -n "$board_out" ] && printf '%s\n' "$board_out"
    [ "$rc" -ne 0 ] && exit "$rc"   # board_pass hard-failed (fail-closed) — propagate verbatim
    verdict="$(board_classify "$board_out")"
    case "$verdict" in
      success) return 0 ;;
      failed)  return 1 ;;
      retryable)
        [ "$attempt" -ge 3 ] && return 1   # exhausted — the push-failed line is already printed
        "$GIT" -C "$mw" pull --rebase >&2 2>&1 || true
        ;;
    esac
  done
}

# commit_and_push_generated MW REL COMMIT_MSG REGEN_FN REGEN_ARG — the shared write-decision
# helper (change 0067), lifted out of board_pass_inline's own commit+push so a second generated
# artifact (the learnings index, learnings_pass below) reuses the EXACT same discipline rather
# than a second, parallel commit path: commit-only-if-changed, then push with a bounded
# rebase-retry loop that regenerates REL in place (never hand-merges) on a conflict touching it.
#
# Carries forward the hard-won subtlety from change 0071 review, finding 3: a clean working tree
# alone is NOT sufficient evidence REL reached the remote — a prior run may have committed REL
# locally and then failed to push, in which case a re-invocation renders the same bytes, finds
# nothing to commit, and must not report success without checking the remote. The no-op probe is
# therefore keyed on unpushed commits touching REL (`@{u}..HEAD`, count > 0; no upstream at all
# counts as nothing-to-push, not an error) — never on tree cleanliness alone.
#
# REL is MW-RELATIVE (the git -C "$mw" form the caller already resolved). The caller has ALREADY
# rendered REL's new bytes in place before calling this. REGEN_FN is the name of a function this
# file defines, taking REGEN_ARG as its sole positional argument, that re-renders REL in place —
# byte-identically to the caller's own initial render — invoked ONLY when a rebase conflict
# actually touches REL, so a conflict is regenerated through the same gated renderer rather than a
# hand 3-way-merge.
#
# Echoes exactly one of: clean | changed-pushed | changed-push-failed
commit_and_push_generated(){
  local mw="$1" rel="$2" commit_msg="$3" regen_fn="$4" regen_arg="$5"

  if [ -z "$("$GIT" -C "$mw" status --porcelain -- "$rel" 2>/dev/null)" ]; then
    local unpushed
    unpushed="$("$GIT" -C "$mw" rev-list --count '@{u}..HEAD' -- "$rel" 2>/dev/null)" || unpushed=0
    if [ "${unpushed:-0}" -eq 0 ]; then
      printf 'clean\n'
      return 0
    fi
    # Working tree is clean (nothing to commit) but an existing commit touching $rel has never
    # reached the remote — fall through into the push/rebase retry loop below without committing.
  else
    "$GIT" -C "$mw" add "$rel" >&2
    "$GIT" -C "$mw" commit -q -m "$commit_msg" >&2 || true
  fi

  local attempt=0 pushed=0
  while [ $attempt -lt 5 ]; do
    attempt=$((attempt + 1))
    if "$GIT" -C "$mw" push >&2 2>&1; then
      pushed=1
      break
    fi
    if ! "$GIT" -C "$mw" pull --rebase >&2 2>&1; then
      # Capture into a variable before grep -qF (never producer | early-exiting-consumer under
      # `set -o pipefail` — grep -q can exit before git finishes writing, and pipefail would then
      # surface git's SIGPIPE exit status instead of the match result).
      local porcelain
      porcelain="$("$GIT" -C "$mw" status --porcelain 2>/dev/null)"
      if grep -qF -- "$rel" <<<"$porcelain"; then
        # Regenerate through the same gated primitive (never a raw redirect) so a rebase never
        # leaves conflict markers or an empty/truncated file.
        if ! "$regen_fn" "$regen_arg"; then
          echo "docket-status: regeneration during rebase failed for $rel; aborting rebase" >&2
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
    printf 'changed-pushed\n'
  else
    printf 'changed-push-failed\n'
  fi
}

# board_regen_inline CD_DIR — re-renders BOARD.md in place through the single gated inline-board
# primitive (board-refresh.sh, change 0059): it owns the atomic, truncation-safe write of
# BOARD.md (render to temp -> chmod 644 -> rename), so render-board.sh is reached ONLY via this
# helper. Used both for board_pass_inline's initial render and as commit_and_push_generated's
# REGEN_FN callback on a rebase conflict — one render path, not two.
board_regen_inline(){
  local cd_dir="$1"
  "$SELF_DIR"/board-refresh.sh --changes-dir "$cd_dir" --surfaces inline ${REPO_FLAG:+--repo "$REPO_FLAG"} >&2 2>&2
}

board_pass_inline(){
  local mw="$1" cd_dir="$2"
  # $rel is BOARD.md's path RELATIVE TO $mw (the metadata worktree) — the form git -C "$mw"
  # accepts (verified: a full "$mw/.../BOARD.md" pathspec fatals under git -C "$mw").
  local rel="$CHANGES_DIR/BOARD.md"
  # board_pass already gated on the `inline` token; a render failure leaves the prior BOARD.md
  # untouched.
  if ! board_regen_inline "$cd_dir"; then
    echo "docket-status: board render failed; keeping existing BOARD.md" >&2
    # Change 0071 review, finding 1 — this used to `return 0` with nothing on stdout: exit 0, no
    # `board …` line, no evidence at all. A must-land caller keying on the report line (never the
    # exit code) would see silence and proceed as if the board had landed — exactly the
    # silent-stale-board failure this whole change exists to kill, merely relocated here. The pass
    # itself still isn't fatal (best-effort; `return 0` stands), but the LINE now carries the
    # outcome so the report channel is never empty on this path. Terminal, not retryable — a
    # render failure is not fixed by retrying.
    echo "board inline failed"
    return 0
  fi
  # board-refresh.sh wrote BOARD.md in place; commit + push only if it actually changed — via the
  # shared write-decision helper (change 0067) so a second generated artifact (the learnings
  # index) reuses the identical discipline rather than a parallel commit path.
  local result
  result="$(commit_and_push_generated "$mw" "$rel" "docket: board refresh" board_regen_inline "$cd_dir")"
  case "$result" in
    clean)          echo "board inline clean" ;;
    changed-pushed) echo "board inline changed pushed" ;;
    *)               echo "board inline changed push-failed" ;;
  esac
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
  mw="$(docket_metadata_worktree)"
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
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
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
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass. This is the value
                                     # sweep_execute_one receives, and the one that makes its
                                     # artifacts-refresh pathspec match at all.
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
  # Change 0075 §5 — this block was DEAD before $mw was anchored: its pathspec carried the same
  # RELATIVE $mw that `git -C "$mw"` had already entered, so it matched nothing and the refreshed
  # ## Artifacts block was silently never committed. Anchoring brings it alive — and its old
  # `return 0` on a failed commit/push would then have ABANDONED terminal-publish AND cleanup.
  # That trade is upside down: a stale link block is COSMETIC (the record still publishes, one
  # table row out of date), while a skipped publish leaves the change archived-but-unpublished
  # (invisible to every future sweep, which only scans active/) and an orphaned worktree + remote
  # branch behind. So: report the failure on the report channel — the closed, line-oriented
  # contract callers key on — and CONTINUE the close-out.
  if [ -n "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ]; then
    if ! "$GIT" -C "$mw" add "$archived" >&2 \
      || ! "$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links" >&2; then
      echo "sweep-failed $id render-change-links commit-failed"
    elif ! "$GIT" -C "$mw" push >&2; then
      echo "sweep-failed $id render-change-links push-failed"
    fi
  fi

  if ! "$SCRIPTS_DIR"/terminal-publish.sh \
        --id "$id" --outcome done --enabled "${TERMINAL_PUBLISH:-false}" \
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
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
  local cd_dir="$mw/$CHANGES_DIR"
  local metadata_branch
  if [ "${DOCKET_MODE:-}" = docket ]; then metadata_branch="$METADATA_BRANCH"; else metadata_branch="$INTEGRATION_BRANCH"; fi
  "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" \
    --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" 2>&2 | \
  while IFS=$'\t' read -r check_id change_id message; do
    [ -n "$check_id" ] || continue
    echo "check $check_id $change_id $message"
  done
  return 0
}

# reclaim_pass HEALTH_OUT — opt-in claim-lease reclaim OR a state-valid remedy line (change 0089).
# FULL PATH ONLY (main() never calls this under --board-only). Keys on the [reclaimable] marker
# board-checks (change 0089) stamps on the expired-lease-AND-no-branch finding — the one case reclaim
# is provably collision- and orphan-free. The MUTATION is gated behind BOTH a [reclaimable] finding
# AND reclaim.auto=true; when auto is off, it prints ONE remedy line instead and touches nothing.
#
# printed-remedy-state-validity: the remedy is keyed on the SAME condition that gates the write (a
# [reclaimable] finding exists), so the command it names is valid in exactly the state that produced
# it, and it is NEVER printed under reclaim.auto=true (reclaim just ran).
#
# Capture-then-grep on a HERE-STRING — never `health_checks | grep -q` (change 0067's no-pipefail-
# SIGPIPE rule): a grep -q that exits on its first match would, in a pipeline, leave the producer's
# SIGPIPE exit status to surface under `set -o pipefail` and mislabel a match as no-match. A
# here-string has no producer process, so no SIGPIPE. docket_metadata_worktree is the SAME resolver
# health_checks uses, so reclaim runs against the same metadata worktree the findings came from;
# single-clone safety comes from the guard's LOCAL refs/heads/feat/<slug> arm (always present in
# this clone) — docket_preflight fetches only origin/<metadata_branch>, never origin/feat/*. The
# genuine cross-machine unfetched-remote-ref case is the documented §7-H residual, contained by
# lease expiry plus reclaim.auto's default-off (see reclaim-claims.md).
reclaim_pass(){
  local health_out="$1" mw cd_dir line
  grep -qF "[reclaimable]" <<<"$health_out" || return 0
  if [ "${RECLAIM_AUTO:-false}" = true ]; then
    mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
    cd_dir="$mw/$CHANGES_DIR"
    while IFS= read -r line; do
      [ -n "$line" ] && printf 'reclaim %s\n' "$line"
    done < <("$SCRIPTS_DIR"/reclaim-claims.sh --changes-dir "$cd_dir" --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}")
  else
    printf 'reclaim: %s expired-lease change(s) can self-heal — run: docket.sh reclaim-claims\n' \
      "$(grep -cF "[reclaimable]" <<<"$health_out")"
  fi
}

# emit_judgment — one "judgment blocked <id> <blocked_by text>" line per `blocked` change under
# $CD/active. The judgment (whether the blocking reason still holds) is left to the caller/skill.
emit_judgment(){
  local mw
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
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

# learnings_regen_index LDIR — re-renders <ldir>/README.md atomically: temp file on the same
# filesystem, non-empty check, chmod 644, then rename — mirroring board-refresh.sh's own
# atomic-write discipline for BOARD.md (the pure renderer, render-learnings-index.sh, only ever
# writes to stdout; this is the gated primitive that turns that stdout into an in-place file, so a
# render failure never truncates/corrupts the last-good index). Used both for learnings_pass's
# initial render and as commit_and_push_generated's REGEN_FN callback on a rebase conflict — one
# render path, not two.
learnings_regen_index(){
  local ldir="$1" tmp
  tmp="$(mktemp "$ldir/.learnings-index.XXXXXX")" || return 1
  if ! "$SCRIPTS_DIR"/render-learnings-index.sh --learnings-dir "$ldir" >"$tmp" 2>/dev/null; then
    rm -f "$tmp"
    return 1
  fi
  if [ ! -s "$tmp" ]; then
    rm -f "$tmp"
    return 1
  fi
  chmod 644 "$tmp"
  mv "$tmp" "$ldir/README.md"
}

# learnings_advisories LDIR — the two needs-you channels (ADR-0028's digest-is-a-report-channel
# pattern, applied to the learnings subsystem): over-cap and promotion-pending. The cap counts
# ACTIVE findings only — `retained` + `candidate`, never `promoted` — because a promoted finding
# is precisely what the shrink valve removes from the count (convention, "Capacity"). Read
# promotion_state through the frontmatter lib, keyed on shape — never a bare grep, which cannot
# tell a `promotion_state: candidate` line from a war-story sentence that happens to contain the
# same word.
learnings_advisories(){
  local ldir="$1" f state active=0 candidates=0
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    state="$(field "$f" promotion_state)"
    state="${state:-retained}"
    [ "$state" = "promoted" ] && continue
    active=$((active + 1))
    [ "$state" = "candidate" ] && candidates=$((candidates + 1))
  done < <(find "$ldir" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)

  if [ "$active" -gt "${LEARNINGS_CAP:-300}" ]; then
    printf 'learnings over-cap — needs curation (%d active, cap %d)\n' "$active" "${LEARNINGS_CAP:-300}"
  fi
  [ "$candidates" -gt 0 ] && printf 'learnings promotion-pending %d — needs you\n' "$candidates"
  return 0
}

# learnings_pass — the learnings-index self-heal + advisories (change 0067). Gated FIRST on
# learnings.enabled — the gate short-circuits BEFORE cap is ever consulted, and the renderer is
# NEVER invoked when disabled: a repo that turned learnings off gets zero reads and zero writes of
# learnings/, and existing finding files are left byte-untouched (a read/write gate, never a
# purge). The disabled note is deliberate positive evidence, not silence — the same "no line is
# indistinguishable from success" lesson change 0069/ADR-0028 already forced onto the backlog
# digest, applied here. Same write decision as the board pass, via the SAME shared helper
# (commit_and_push_generated): render in place, diff, commit only if changed, push with the
# bounded rebase-retry loop. The two needs-you advisories (learnings_advisories) fire on EVERY
# path that has finding files to look at — including a failed render — because they are computed
# from the finding files themselves, not from the render outcome; only the "enabled" gate and the
# "no learnings dir" (nothing to advise on) cases skip them (change 0067 review, finding 3).
learnings_pass(){
  if [ "${LEARNINGS_ENABLED:-true}" != "true" ]; then
    printf 'learnings disabled\n'
    return 0
  fi
  local mw
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
  local ldir="$mw/$CHANGES_DIR/learnings"
  [ -d "$ldir" ] || { printf 'learnings index skipped (no learnings dir)\n'; return 0; }

  if ! learnings_regen_index "$ldir"; then
    printf 'learnings index failed\n'
    # F3 (change 0067 review) — the two needs-you advisories below are computed from the finding
    # FILES, independent of whether the index render succeeded; a broken renderer must not also
    # mute the escalation channels precisely when something is already wrong. The "no learnings
    # dir" branch above is different in kind (there are no finding files to advise on, so no
    # advisories there is correct) — this is the one early-return that must still advise.
    learnings_advisories "$ldir"
    return 0
  fi

  local rel="$CHANGES_DIR/learnings/README.md"
  local result
  result="$(commit_and_push_generated "$mw" "$rel" "docket: learnings index refresh" learnings_regen_index "$ldir")"
  case "$result" in
    clean)          printf 'learnings index clean\n' ;;
    changed-pushed) printf 'learnings index changed pushed\n' ;;
    *)               printf 'learnings index changed push-failed\n' ;;
  esac

  learnings_advisories "$ldir"
}

# integration_sync — best-effort FF-only sync of the invoking repo's integration-branch
# checkout, run once at the end of a pass that swept at least one change.
integration_sync(){
  "$SCRIPTS_DIR"/sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH" >&2 2>&1 || true
  return 0
}

main(){
  docket_preflight "$SCRIPTS_DIR" || exit 1
  if [ "$MUST_LAND" = 1 ]; then
    board_pass_must_land || exit 1
  else
    board_pass
  fi
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

  # Change 0089: capture health output (never `health_checks | grep` — pipefail SIGPIPE rule), print
  # it, then let reclaim_pass key its opt-in mutation / state-valid remedy on the same [reclaimable]
  # findings. FULL PATH ONLY — reclaim_pass is never reached under --board-only (that early-exits above).
  local health_out
  health_out="$(health_checks)"
  [ -n "$health_out" ] && printf '%s\n' "$health_out"
  reclaim_pass "$health_out"
  emit_judgment
  # Change 0067: the learnings pass runs on the FULL path only — never under --board-only, which
  # is the board's own dedicated entry point and is invoked by many callers as a must-land board
  # write; adding unrelated learnings work to it would be wrong.
  learnings_pass
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
