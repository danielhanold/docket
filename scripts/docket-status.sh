#!/usr/bin/env bash
# scripts/docket-status.sh — deterministic orchestrator for the docket-status pass (change 0058).
# Sequences the shared docket scripts in one process; emits one line-oriented report on stdout.
# The report is self-evidencing: it always states what it did (`board off` when the board is
# disabled, the backlog digest, `pass ok` on completion), so stdout is never empty (change 0069).
#
# Usage: docket-status.sh [--board-only] [--digest-only] [--must-land] [--repo OWNER/REPO]
#                          [--type TYPE|untyped|all] [--priority PRIORITY|all]
#                          [--project OWNER/NUMBER] [--auto-create-project] [--project-owner OWNER]
#   --board-only           only regenerate the board surfaces; skip sweep/health passes
#   --digest-only          WRITE-FREE READ (change 0094): resolve config, emit the backlog digest
#                          (rollups + `change` lines + the `ready` queue line) and exit. No worktree
#                          sync, no sweep, no health checks, no board render, no commit, no push,
#                          and no `board …` line. Mutually exclusive with --board-only AND with
#                          --must-land (which has nothing to retry without a board pass) — both
#                          gates reject in either flag order; see docket-status.md.
#   --must-land            (with --board-only) retry a push-failed board write in-script and
#                          map the outcome to the exit code (0 = board landed); see docket-status.md
#   --repo OWNER/REPO      GitHub repo for PR-link resolution (defaults to origin remote)
#   --type TYPE            REPORT-ONLY: narrow the backlog digest to changes of TYPE. `untyped`
#                          selects changes carrying no type:; `all` (the default) selects every
#                          change and is exactly equivalent to omitting the flag. Never narrows
#                          the board, sweep, harvesting, archiving, health checks, or reclaim.
#   --priority PRIORITY    REPORT-ONLY: narrow the backlog digest to changes of PRIORITY. `all`
#                          (the default) selects every change. Same read-only scope as --type.
#   --project OWNER/NUMBER GitHub Project to sync (later task)
#   --auto-create-project  create the GitHub Project if --project doesn't resolve (later task)
#   --project-owner OWNER  owner to create the project under (later task)
#
# Contract: scripts/docket-status.md.
# Mock seams: GIT="${GIT:-git}", GH="${GH:-gh}", NOW="${NOW:-$(date +%s)}" (staleness clock),
# CONFIG_EXPORT_CMD (config export override).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GIT="${GIT:-git}"
GH="${GH:-gh}"
# Staleness clock, spelled exactly as board-checks.sh's NOW seam so both suites drive their clocks
# the same way. Read by detect_orphan_pr's idle floor (change 0219).
NOW="${NOW:-$(date +%s)}"
SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"
# shellcheck source=lib/docket-frontmatter.sh
. "$SELF_DIR"/lib/docket-frontmatter.sh
# shellcheck source=lib/docket-preflight.sh
. "$SELF_DIR"/lib/docket-preflight.sh

BOARD_ONLY=0 DIGEST_ONLY=0 MUST_LAND=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
TYPE_FLAG="" PRIORITY_FLAG="" TYPE_SET=0 PRIORITY_SET=0
# The range must end on the LAST documented flag. tests/test_docket_status.sh pins that the final
# row (--project-owner) survives, because a header edit that does not bump this bound truncates
# --help silently — which is exactly how the two rows above went missing once already.
usage(){ sed -n '2,28p' "${BASH_SOURCE[0]}"; }
while [ $# -gt 0 ]; do
  case "$1" in
    --board-only) BOARD_ONLY=1 ;;
    --digest-only) DIGEST_ONLY=1 ;;
    --type|--priority)
      # Guard the arity: without it a trailing `--type` reads an unset $2 under `set -u` and dies
      # with a raw "unbound variable" trace instead of the documented exit-2 argument error.
      [ $# -ge 2 ] || { echo "docket-status: $1 requires a value" >&2; exit 2; }
      case "$1" in
        --type)     TYPE_FLAG="$2"; TYPE_SET=1 ;;
        --priority) PRIORITY_FLAG="$2"; PRIORITY_SET=1 ;;
      esac
      shift ;;
    --must-land) MUST_LAND=1 ;;
    --repo) REPO_FLAG="$2"; shift ;;
    --project) PROJECT_FLAG="$2"; shift ;;
    --auto-create-project) AUTO_CREATE_PROJECT=1 ;;
    --project-owner) PROJECT_OWNER="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "docket-status: unknown argument: $1" >&2; exit 2 ;;
  esac; shift
done
# Opposite postures: --digest-only is a write-free READ, --board-only commits and pushes BOARD.md.
# Rejected in BOTH orders — a gate that only closes one way is not a gate.
# Filter values are validated HERE, before any pass runs, so an invalid value fails closed
# IDENTICALLY in every mode. It cannot be left to render-board's own check: backlog_pass is
# best-effort by design, so a rejected filter would be swallowed on the full pass and
# `--board-only --type Bogus` would print a board while silently omitting the backlog the caller
# asked to filter. Keying on render-board's exit 2 instead does not work either — it also spends
# that code on "changes dir not found", a condition the full pass legitimately tolerates.
# This is a second CALL SITE of the shared predicates, not a second RULE: the grammar lives in
# lib/docket-frontmatter.sh, which this script already sources, so the two checks cannot drift.
# Gate on "the flag was PROVIDED", never on "the value is non-empty". Empty-as-unset made
# `--type ""` skip validation AND skip forwarding, silently returning the COMPLETE backlog, while
# render-board.sh rejects that same value with exit 2 — so a caller writing `--type "$T"` with an
# unset variable got the exact opposite of the fail-closed behaviour this block documents.
if [ "$TYPE_SET" = 1 ] && [ "$TYPE_FLAG" != all ] && [ "$TYPE_FLAG" != untyped ]; then
  if ! docket_change_type_is_wellformed "$TYPE_FLAG"; then
    echo "docket-status: unknown --type value: $TYPE_FLAG (expected all, untyped, or a [a-z][a-z0-9-]* token)" >&2
    exit 2
  fi
fi
if [ "$PRIORITY_SET" = 1 ] && [ "$PRIORITY_FLAG" != all ]; then
  if ! docket_priority_is_member "$PRIORITY_FLAG"; then
    echo "docket-status: unknown --priority value: $PRIORITY_FLAG (expected all, ${DOCKET_PRIORITIES[*]})" >&2
    exit 2
  fi
fi
if [ "$DIGEST_ONLY" = 1 ] && [ "$BOARD_ONLY" = 1 ]; then
  echo "docket-status: --digest-only and --board-only are mutually exclusive (a write-free read vs. a committing board write)" >&2
  exit 2
fi
# --must-land is meaningful only alongside --board-only: it retries/maps the exit code of the
# board pass, which --digest-only never runs. main() short-circuits --digest-only BEFORE
# MUST_LAND is ever read, so without this gate `--digest-only --must-land` would silently drop
# --must-land and exit 0 with the digest — the same silent-flag-drop shape the gate above exists
# to prevent for --board-only. Extend the SAME gate rather than inventing a second mechanism;
# both orders, matching the discipline above.
if [ "$DIGEST_ONLY" = 1 ] && [ "$MUST_LAND" = 1 ]; then
  echo "docket-status: --digest-only and --must-land are mutually exclusive (--must-land only applies to --board-only; --digest-only is a write-free read)" >&2
  exit 2
fi
# --repo/--project/--auto-create-project/--project-owner are consumed only by board_pass and
# github-mirror.sh — machinery --digest-only never reaches (it short-circuits in main() before
# either runs). Passing any of them alongside --digest-only is the same silently-dropped-flag shape
# --must-land's gate above exists to prevent; extend the SAME gate, one check per flag, so the
# error names the actual offending flag rather than a generic complaint.
if [ "$DIGEST_ONLY" = 1 ]; then
  if [ -n "$REPO_FLAG" ]; then
    echo "docket-status: --digest-only and --repo are mutually exclusive (--repo only resolves PR links in a board pass; --digest-only never runs one)" >&2
    exit 2
  fi
  if [ -n "$PROJECT_FLAG" ]; then
    echo "docket-status: --digest-only and --project are mutually exclusive (--project only drives the GitHub Project sync; --digest-only never runs one)" >&2
    exit 2
  fi
  if [ "$AUTO_CREATE_PROJECT" = 1 ]; then
    echo "docket-status: --digest-only and --auto-create-project are mutually exclusive (--auto-create-project only drives the GitHub Project sync; --digest-only never runs one)" >&2
    exit 2
  fi
  if [ -n "$PROJECT_OWNER" ]; then
    echo "docket-status: --digest-only and --project-owner are mutually exclusive (--project-owner only drives the GitHub Project sync; --digest-only never runs one)" >&2
    exit 2
  fi
fi

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
      # Change 0247. The catch-all below already maps this to `failed`, which is the halt the token
      # wants — but inheriting the right verdict from a catch-all is not the same as documenting it,
      # and the arm makes the NOT-retryable half of the decision reviewable at the classifier.
      # Retrying is pure latency here: a rebase or merge in progress clears only when a human ends it.
      "board inline blocked-wedged-tree") has_failed=1 ;;
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
# Echoes exactly one of: clean | changed-pushed | changed-push-failed | blocked-wedged-tree
#
# blocked-wedged-tree (change 0247): the shared metadata worktree has a rebase or merge already in
# progress — another agent mid-sync — so NOTHING is committed and NOTHING is pushed. The probe is
# the FIRST statement of the body, ahead of even the nothing-to-commit check, because every path
# below it is unsafe in that state: the push/rebase retry loop's own `rebase --abort` would
# destroy that other agent's in-flight rebase, which is the one failure here nothing can walk back.
#
# That first probe is POINT-IN-TIME, so it is not the whole guarantee: the render->commit->push
# window is seconds wide, and a wedge that opens inside it reaches the retry loop. The loop
# therefore carries a SECOND probe of its own (see `re-probe HERE` below), which emits the same
# token — with one difference this contract has to state rather than imply: on that second probe a
# local commit may ALREADY exist. Nothing is ever pushed under this token, but "nothing is
# committed" is the first probe's promise alone. Both spellings mean the same thing to a caller:
# NOT LANDED, and a human clears it.
#
# Committing is unsafe too, and the `--` pathspec this change adds to the commit makes it MORE so,
# not less. Measured on git 2.55: mid-rebase-with-conflicts, `git commit -m … -- "$rel"` exits 0
# and writes a commit onto the rebase's detached HEAD, where the old pathspec-less form was refused
# ("Committing is not possible because you have unmerged files"). Scoping the commit therefore
# REMOVES an accidental protection, so this gate replaces it deliberately rather than inheriting it.
#
# A distinct token, never an overload of changed-push-failed: `--must-land` must read this as
# not-landed and halt (a human finishes or aborts the operation), while changed-push-failed keeps
# its exact prior meaning as the sole RETRYABLE board outcome.
commit_and_push_generated(){
  local mw="$1" rel="$2" commit_msg="$3" regen_fn="$4" regen_arg="$5"

  if _docket_tree_wedged "$GIT" "$mw"; then
    printf 'blocked-wedged-tree\n'
    return 0
  fi

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
    # `--` on BOTH (change 0247): the metadata worktree is SHARED, so a pathspec-less commit stages
    # and commits whatever another agent had in the index at that instant, under this run's message.
    # The mark also guards a $rel that could begin with a dash (the #0083 mark-path idiom).
    "$GIT" -C "$mw" add -- "$rel" >&2
    "$GIT" -C "$mw" commit -q -m "$commit_msg" -- "$rel" >&2 || true
  fi

  local attempt=0 pushed=0 blocked=0
  while [ $attempt -lt 5 ]; do
    attempt=$((attempt + 1))
    if "$GIT" -C "$mw" push >&2 2>&1; then
      pushed=1
      break
    fi
    # The top-of-function probe answered for the instant the function STARTED; re-probe HERE, in
    # front of the only statement below that can start a rebase. A rebase or merge in progress at
    # THIS point is never one this function started, and that is what makes the placement — not
    # merely the probe — load-bearing. The loop only ever arrives here before its first
    # `pull --rebase`, after a `pull --rebase` that returned 0, or after a `rebase --continue` that
    # returned 0, and each of those leaves no operation in progress. Moved DOWN beside the
    # `rebase --abort` calls instead, the same probe would refuse to abort the rebase this loop
    # itself just started on a conflict, stranding a wedge this function owns.
    #
    # It NARROWS the window rather than closing it: a rebase another agent starts between this
    # probe and git's own start-up inside `pull --rebase` is still indistinguishable from ours —
    # both rebase the same shared branch onto the same remote, so nothing in the rebase state names
    # an owner. What it removes is the seconds-wide render->commit->push window, which is where the
    # loss actually happens.
    if _docket_tree_wedged "$GIT" "$mw"; then
      blocked=1
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
        "$GIT" -C "$mw" add -- "$rel" >&2
        "$GIT" -C "$mw" rebase --continue >&2 2>&1 || { "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true; pushed=-1; break; }
      else
        "$GIT" -C "$mw" rebase --abort >&2 2>/dev/null || true
        break
      fi
    fi
  done
  if [ $blocked -eq 1 ]; then
    printf 'blocked-wedged-tree\n'
  elif [ $pushed -eq 1 ]; then
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
  "$DOCKET_BASH_PATH" "$SELF_DIR"/board-refresh.sh --changes-dir "$cd_dir" --surfaces inline ${REPO_FLAG:+--repo "$REPO_FLAG"} >&2 2>&2
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
  # The blocked-wedged-tree arm is NOT redundant with the catch-all below it: the catch-all prints
  # the RETRYABLE push-failed token, so without an explicit arm the new return value would be
  # silently relabelled retryable one layer up — turning a --must-land halt into a retry (change
  # 0247, spec Assumption 16(a)).
  case "$result" in
    clean)               echo "board inline clean" ;;
    changed-pushed)      echo "board inline changed pushed" ;;
    blocked-wedged-tree) echo "board inline blocked-wedged-tree" ;;
    *)                   echo "board inline changed push-failed" ;;
  esac
}

board_pass_github(){
  local cd_dir="$1"
  local out rc
  out="$("$DOCKET_BASH_PATH" "$SELF_DIR"/github-mirror.sh --changes-dir "$cd_dir" ${REPO_FLAG:+--repo "$REPO_FLAG"} ${PROJECT_FLAG:+--project "$PROJECT_FLAG"} $([ "$AUTO_CREATE_PROJECT" = 1 ] && echo --auto-create-project) ${PROJECT_OWNER:+--project-owner "$PROJECT_OWNER"} 2>&2)"
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
  # change 0127: the report filters reach the DIGEST projection only. They are deliberately NOT
  # forwarded to board_pass's board-refresh.sh call — that writer must always emit a COMPLETE
  # BOARD.md, or a filtered --board-only run would commit a truncated board.
  local -a filt=()
  [ "$TYPE_SET" = 1 ]     && filt+=(--type "$TYPE_FLAG")
  [ "$PRIORITY_SET" = 1 ] && filt+=(--priority "$PRIORITY_FLAG")
  if ! out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest ${filt[@]+"${filt[@]}"} 2>&2)"; then
    echo "docket-status: backlog digest failed; continuing without it" >&2
    return 0
  fi
  [ -n "$out" ] && printf '%s\n' "$out"
  return 0
}

# digest_only_pass — the write-free selection read (change 0094). docket-implement-next Step 1
# acquires its ordered candidate set here, so this path must be a READ in the strict sense: it
# resolves config and runs the backlog pass, and does nothing else. Unlike every other caller of
# backlog_pass (the full pass, --board-only), this path treats the digest as NON-BEST-EFFORT: it
# captures backlog_pass's output and fails closed (non-zero exit, diagnostic on stderr, nothing on
# stdout) when that output is empty — for ANY reason, not only the missing-changes-dir case gated
# below. That is deliberate and narrow to this path alone: the digest is the entire deliverable
# here, so there is nothing else in the pass for a caller to fall back to, unlike the full pass or
# --board-only where a digest failure is one line among many and must not abort the rest.
#
# It deliberately does NOT call docket_preflight. Preflight FETCHES AND `pull --rebase`s the
# metadata worktree — a working-tree mutation that can move HEAD, which would make a "read" a
# write. That costs nothing here: the calling skill runs this AFTER its own Step-0 preflight, so
# the tree is already freshly synced and a second sync would be pure redundancy. The digest is a
# snapshot of the change files as it finds them, which is exactly the contract Step 1 wants.
#
# The bootstrap verdict is still enforced FAIL-CLOSED. A repo that was never migrated has no
# metadata worktree, and reporting a cheerful empty backlog for it would hand the selector a
# `ready` line meaning "nothing is ready" when the truth is "this repo is not set up" — the exact
# two-cases-one-signal collapse the always-emitted ready line exists to prevent.
digest_only_pass(){
  local cfg
  if [ -n "${CONFIG_EXPORT_CMD:-}" ]; then
    cfg="$($CONFIG_EXPORT_CMD)" || { echo "docket-status: config export failed" >&2; return 1; }
  else
    cfg="$("${DOCKET_BASH_PATH:?run docket/install.sh}" "$SCRIPTS_DIR"/docket-config.sh --export)" \
      || { echo "docket-status: config export failed" >&2; return 1; }
  fi
  eval "$cfg"
  export DOCKET_BASH_PATH
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  echo "docket-status: repo not migrated — run migrate-to-docket.sh" >&2; return 1 ;;
    CREATE_ORPHAN) echo "docket-status: fresh repo — run docket.sh bootstrap to create the docket branch" >&2; return 1 ;;
    *) echo "docket-status: unknown bootstrap verdict '${BOOTSTRAP:-}'" >&2; return 1 ;;
  esac
  # Fail-closed existence check (review fix, still change 0094) — needed ONLY on this path. The
  # full pass and --board-only both run docket_preflight first, which CREATES the metadata
  # worktree when it's missing, so every OTHER caller of backlog_pass gets that guarantee for
  # free. --digest-only deliberately skips preflight (see the header comment above), so nothing
  # else guarantees .docket/ exists here. Reachable scenario: a fresh clone of an
  # already-migrated repo — origin/docket exists (BOOTSTRAP=PROCEED), but .docket/ is gitignored
  # and was never materialized locally. Without this check, backlog_pass's render-board.sh call
  # fails, logs to stderr, and best-effort `return 0`s — exit 0 with empty stdout, the exact
  # two-cases-one-signal collapse the always-emitted `ready` line exists to prevent. Resolve the
  # changes dir the SAME way backlog_pass does, so the two can never diverge.
  local mw cd_dir
  mw="$(docket_metadata_worktree)"
  cd_dir="$mw/$CHANGES_DIR"
  if [ ! -d "$cd_dir" ]; then
    echo "docket-status: metadata worktree not found at $cd_dir — run docket.sh preflight (or a docket skill) to materialize it" >&2
    return 1
  fi
  # Review fix (still change 0094) — backlog_pass is BEST-EFFORT BY DESIGN (a render failure logs
  # to stderr and `return 0`s), which is the right posture for the full pass and --board-only: a
  # digest failure there must never abort the board/sweep/health work around it. But on THIS path
  # the digest IS the entire deliverable — there is nothing else in the pass for a caller to fall
  # back to — so best-effort here reopens exactly the "exit 0, empty stdout" collapse the
  # existence check above exists to prevent, just one call later: a partially-installed or
  # non-executable render-board.sh (any render failure other than the missing-changes-dir case
  # already gated above) would otherwise still reach `exit 0` with nothing on stdout. Capture
  # backlog_pass's output and require it non-empty — never call it for its side effect (echoing to
  # stdout) and trust the return code, since a best-effort function's return code says nothing.
  local out
  out="$(backlog_pass)"
  if [ -z "$out" ]; then
    echo "docket-status: digest render produced no output — the selection queue is unavailable" >&2
    return 1
  fi
  printf '%s\n' "$out"
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
      # --repo "$repo" is what SPENDS the resolution above. Without it gh infers the repository
      # from the process CWD, so a pass invoked with --repo would query one repository here and a
      # different one in board_pass / github-mirror.sh, which both forward the flag. Unconditional,
      # not ${repo:+...}: the early returns above guarantee $repo is resolved and shape-valid by
      # the time this arm runs. Same shape as detect_orphan_pr's single batched call.
      pl_json="$("$GH" pr list --repo "$repo" --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
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

# Branch-idle floor for the GitHub enrichment leg (change 0219). This is leg C's OWN floor
# (board-checks.sh's ABORTED_RUN_IDLE_SECS), reused rather than re-tuned, and the reuse is
# load-bearing: it guarantees the enrichment never fires on a change leg C stayed silent about, so
# the git-only finding and its GitHub resolution always agree. Hardcoded with no config knob — the
# same precedent ABORTED_RUN_STALE_SECS and ABORTED_RUN_IDLE_SECS set; a second magic number would
# need its own justification and this one has none to offer. Kept in sync BY VALUE, not by import:
# the two scripts share no library, and board-checks.sh must stay independently runnable.
ORPHAN_PR_IDLE_SECS=$(( 2 * 3600 ))

# detect_orphan_pr — the GitHub enrichment leg for board-checks.sh's aborted-run leg C (change 0219).
#
# Leg C fires on "branch has commits, pr: is unset, tip idle > 2h" and can only tell a human to "verify
# the PR exists": board-checks.sh is git-only BY CONTRACT and shells no gh. Two very different
# situations produce that one finding — a PR that exists and merely went unrecorded (remedy: record
# it) versus a run that died before opening one (remedy: open it) — and today only a manual check
# distinguishes them. This leg asks GitHub which it is. It adds NO detection; it RESOLVES an
# ambiguity, which is why it lives here (where gh already lives) rather than widening leg C.
#
# The gate is leg C's own, so the two findings always agree and this can never fire on a change leg C
# stayed silent about. Advisory like every aborted-run leg: it flips no status, releases no claim, and
# writes no file.
#
# Network cost is O(1) per pass, VERBATIM detect_merged's discipline: ONE batched `gh pr list` for
# the whole candidate set (plus the shared `gh repo view` when --repo is unset), matched to the
# candidates locally by headRefName. See the query below for the --limit choice.
#
# Best-effort, VERBATIM detect_merged's POSTURE: any gh/network/parse failure emits
# "orphan-pr-skipped <reason>" and returns 0. That is what keeps board-checks.sh's offline guarantee
# intact — offline, the git-only check keeps emitting leg C's finding and only the enrichment goes
# quiet. Prints "check aborted-run <id> <message>" lines, matching health_checks' own render so
# consumers read one vocabulary.
#
# The posture is detect_merged's; the TOKEN deliberately is not. "sweep-skipped" is detect_merged's
# machine contract with sweep_execute (which recognizes those lines and passes them through) and it
# means "the MERGE SWEEP did not run". These lines are printed raw onto the same stdout by an
# advisory enrichment and mean "the PR-ORPHAN CHECK did not run" — two unrelated subsystems, so a
# reader of one pass log would have attributed an enrichment skip to the sweep. The reasons are
# shared verbatim; only the prefix separates them.
detect_orphan_pr(){
  local mw
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
  local cd_dir="$mw/$CHANGES_DIR"

  local -a files
  mapfile -t files < <(find "$cd_dir/active" -maxdepth 1 -name '*.md' 2>/dev/null | sort)
  [ ${#files[@]} -gt 0 ] || return 0

  # Collect candidates FIRST, and return before touching gh when there are none. A repo with no
  # candidate must pay nothing — not a `gh repo view`, not a subprocess.
  local -a ids=() branches=() idles=() pushes=()
  local f id status pr slug branch tip ref b pushed
  local -a bases
  for f in "${files[@]}"; do
    status="$(field "$f" status)"
    [ "$status" = in-progress ] || continue
    # ANCHORED reads (ADR-0057) for both OPTIONAL keys, matching board-checks.sh leg D verbatim —
    # the two legs share leg C's gate, so a disagreement here would break the "the two findings
    # always agree" property this block's header sells. field() takes the first match ANYWHERE in
    # the file, and this repo's change bodies routinely open a line with `pr:` or `branch:`: an
    # unanchored pr: read would skip such a change as already-recorded (a SILENT false negative),
    # and an unanchored branch: read would aim the tip probe at a ref named in prose.
    pr="$(fm_field "$f" pr)"
    [ -z "$pr" ] || continue          # pr: recorded is leg D's domain, never this one
    id="$(int_field "$f" id)"
    [ -n "$id" ] || continue
    slug="$(field "$f" slug)"
    branch="$(fm_field "$f" branch)"
    [ -n "$branch" ] || branch="feat/$slug"
    # Resolve the ref board-checks.sh's `branch_ref` way — refs/heads/<branch>, then
    # refs/remotes/origin/<branch>, each show-ref-verified. Full ref paths, never the bare name:
    # the bare name is a rev-parse spelling that resolves through the DWIM rules, and this leg has
    # to know WHICH of the two it got (see the pushed probe below). An unresolvable branch is
    # silence, never a finding — no positive evidence is the posture every aborted-run leg takes.
    ref=""
    if "$GIT" -C "$cd_dir" show-ref --verify --quiet "refs/heads/$branch"; then
      ref="refs/heads/$branch"
    elif "$GIT" -C "$cd_dir" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
      ref="refs/remotes/origin/$branch"
    fi
    [ -n "$ref" ] || continue
    tip="$("$GIT" -C "$cd_dir" log -1 --format=%ct "$ref" 2>/dev/null)"
    [ -n "$tip" ] || continue
    [ "$(( NOW - tip ))" -gt "$ORPHAN_PR_IDLE_SECS" ] || continue
    # Ahead of BOTH bases — leg C's gate, mirrored predicate for predicate because "the two
    # findings always agree" is this leg's whole premise, and an idle floor ALONE is not that gate.
    # A run that died before its first commit leaves a branch whose tip IS the base commit, and a
    # base commit is almost always older than the floor: without this, the leg fires "the run
    # stopped before opening one" on the NOTHING-BUILT signature (0109) that belongs to leg B and
    # that leg C deliberately stays silent about.
    #
    # Both bases, both show-ref-verified, for board-checks.sh's reason: feature branches are cut
    # from origin/<integration_branch> while INTEGRATION_BRANCH names the LOCAL ref, which
    # routinely lags origin (sync-integration-branch.sh is FF-only and best-effort), so the local
    # ref alone makes a freshly-cut branch look arbitrarily far ahead. INTEGRATION_BRANCH is a
    # config global (docket_preflight's `eval "$cfg"`); it is read defensively because
    # detect_orphan_pr is also called directly, and an unset one resolves NO base.
    #
    # The count gate comes FIRST and short-circuits: bash >= 4.4 expands an empty "${bases[@]}"
    # happily, so rev-list would exclude nothing, list the branch's WHOLE history, and fire on
    # "ahead of nothing" — and older bash raises an unbound-variable error under set -u. No base
    # resolving at all is SILENCE, the same posture leg C takes.
    bases=()
    if [ -n "${INTEGRATION_BRANCH:-}" ]; then
      for b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do
        "$GIT" -C "$cd_dir" show-ref --verify --quiet "$b" && bases+=( "$b" )
      done
    fi
    [ "${#bases[@]}" -gt 0 ] || continue
    [ -n "$("$GIT" -C "$cd_dir" rev-list -n 1 "$ref" --not "${bases[@]}" 2>/dev/null)" ] || continue
    # "Pushed" is a claim about the REMOTE, so it is keyed on the remote-tracking ref existing —
    # leg C splits its two messages on exactly this probe. Resolving refs/heads/<branch> above
    # says nothing about whether the branch was ever published, and asserting it anyway would send
    # a human to a GitHub branch that is not there. A STALE remote-tracking ref left by a
    # remote-side deletion reads as pushed; acceptable for an advisory whose remedy either way is
    # "go look at this run" — leg C makes the same trade.
    if "$GIT" -C "$cd_dir" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
      pushed=1
    else
      pushed=0
    fi
    ids+=("$id"); branches+=("$branch"); idles+=("$(( (NOW - tip) / 3600 ))"); pushes+=("$pushed")
  done
  [ ${#ids[@]} -gt 0 ] || return 0

  local repo="${REPO_FLAG:-}"
  if [ -z "$repo" ]; then
    repo="$("$GH" repo view --json owner,name -q '(.owner.login)+"/"+(.name)' 2>/dev/null)" \
      || { echo "orphan-pr-skipped gh-unavailable"; return 0; }
  fi
  # Validated detect_merged's way — split on the slash and reject the malformed shape. A bare
  # non-empty test is not enough: `gh repo view` can exit 0 and print something that is not
  # owner/name, and that string would then be handed straight to --repo below. Sharing the shape
  # check with detect_merged is what makes `repo-unresolved` mean the same thing in both legs.
  local owner="${repo%%/*}" name="${repo#*/}"
  if [ -z "$owner" ] || [ -z "$name" ] || [ "$owner" = "$repo" ]; then
    echo "orphan-pr-skipped repo-unresolved"
    return 0
  fi

  # ONE network call for the WHOLE candidate set, then matched locally by headRefName. detect_merged
  # is batched for exactly this reason — this leg sits on the same full-path pass, and a per-candidate
  # `gh pr list` made the pass's network cost O(candidates) during a backlog drain. The `pr list`
  # form is preferred over detect_merged's aliased graphql because this leg keys on BRANCH NAME, not
  # on a known PR number: a graphql alias per candidate would need one `pullRequests(headRefName:)`
  # connection each and buys nothing over a single open-PR listing.
  #
  # --limit 200: gh's default is 30, which a busy repo would silently truncate — and a truncated
  # listing does not read as "no PR", it reads as the WRONG message arm ("the run stopped before
  # opening one" for a PR that exists). 200 is two 100-item API pages inside the one invocation,
  # while a docket-managed repo's open-PR count is bounded by its in-flight changes (single digits
  # in practice), so it is two orders of magnitude of headroom at a fixed cost. The guard below
  # makes the choice safe rather than merely hopeful: at the ceiling the leg goes quiet instead of
  # guessing, because a missing match can no longer be distinguished from a truncated page.
  local pl_list_limit=200
  local pl_json pl_count
  # --repo "$repo" is what SPENDS the resolution above. Without it gh infers the repository from
  # the process CWD, so a pass invoked with --repo would query one repository here and a
  # different one in board_pass / github-mirror.sh, which both forward the flag.
  pl_json="$("$GH" pr list --repo "$repo" --state open --json number,headRefName --limit "$pl_list_limit" 2>/dev/null)" || {
    echo "orphan-pr-skipped gh-unavailable"
    return 0
  }
  # A gh that exits 0 and prints something jq cannot parse is a THIRD failure mode, distinct from
  # a non-zero exit and from an absent binary. With one batched response this failure is GLOBAL —
  # the one response is all the evidence there was — so it skips the whole leg rather than a single
  # change. The reason is unchanged; only its blast radius is, and it is documented that way in
  # scripts/docket-status.md.
  if ! printf '%s' "$pl_json" | jq -e . >/dev/null 2>&1; then
    echo "orphan-pr-skipped gh-unparseable"
    return 0
  fi
  pl_count="$(printf '%s' "$pl_json" | jq -r 'if type == "array" then length else 0 end' 2>/dev/null)"
  if [ "${pl_count:-0}" -ge "$pl_list_limit" ]; then
    echo "orphan-pr-skipped pr-list-truncated"
    return 0
  fi

  local i br pl_num
  for i in "${!ids[@]}"; do
    id="${ids[$i]}"; br="${branches[$i]}"
    pl_num="$(printf '%s' "$pl_json" | jq -r --arg b "$br" \
      'map(select(.headRefName == $b)) | .[0].number // empty' 2>/dev/null)"
    # Both messages HEDGE nothing about the PR's existence — unlike leg C, this leg has ASKED, so it
    # states what it found as fact. The remedy stays a bookkeeping act on the manifest, never a
    # push or a merge: acting on the branch would race a run that is merely between commits.
    if [ -n "$pl_num" ]; then
      echo "check aborted-run $id PR #$pl_num is open on $br but pr: is unset — record it"
    elif [ "${pushes[$i]}" = 1 ]; then
      echo "check aborted-run $id $br is pushed (last commit ${idles[$i]}h ago) but no PR on GitHub — the run stopped before opening one"
    else
      # Never pushed: GitHub cannot hold a PR for a branch it has never seen, so "no PR on GitHub"
      # is not the informative half here — the missing push is, and it names an EARLIER seam the
      # human has to act on first. Leg C splits the same way for the same reason.
      echo "check aborted-run $id $br was never pushed (last commit ${idles[$i]}h ago) and has no PR on GitHub — the run stopped before pushing it"
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

# sweep_mark_publish_deferred MW ARCHIVED ID DETAIL — write the durable `## Publish deferred`
# marker on an archived change whose expected publish did not complete, and land it on
# metadata_branch. THE SINGLE WRITER for both sweep paths that abandon an expected publish: the
# change-0083 terminal-publish failure, and (change 0118) the render-change-links skipped-publish
# failure. Extracted rather than inlined twice because the two blocks share an invariant, and a
# second copy is where that invariant would silently diverge.
#
# BEST-EFFORT toward the report stream: always returns 0, writes nothing to stdout or stderr, and
# no caller may branch on it. A failed mark must degrade to the pre-mark observable behavior
# EXACTLY — same report lines, same order, same control flow.
#
# TRANSACTIONAL toward the worktree, which bare `|| true` suppression is not (change 0118). The
# metadata worktree is SHARED: a marker that writes but fails to commit leaves the archived path
# dirty or staged, and every later pass's `pull --rebase` then fails for EVERY change — strictly
# worse than the unmarked gap this records. So recovery is defined per failure point:
#   - precondition, path not clean  -> skip entirely; never stack a marker onto a dirty state
#                                      some other actor left behind.
#   - precondition, tree wedged     -> skip entirely (change 0247's rule at the sweep's other
#                                      exposed commit). A commit into a mid-rebase tree writes
#                                      onto that rebase's DETACHED HEAD — and the restore below
#                                      would then resolve `HEAD` to that same detached commit and
#                                      corrupt the file it exists to repair.
#   - add or commit fails           -> restore the path to HEAD, index and worktree both.
#                                      Degraded outcome: unmarked gap, CLEAN worktree — exactly
#                                      today's behavior.
#   - commit succeeds, push fails   -> RETAIN the local commit. This is the correlated case: the
#                                      motivating renderer failure is a network blip, and the push
#                                      needs the same network. A clean unpushed commit is harmless
#                                      and self-heals — the next pass's `pull --rebase` carries it
#                                      and a later push from the shared worktree publishes it.
#                                      Never reset it; destroying it re-opens the gap.
#
# DETAIL must never contain the literal `terminal-publish.sh`: this invocation carries `--id` and
# no `--enabled`, and tests/test_closeout.sh's find_ungated_terminal_publish_call_sites scans
# JOINED logical lines for that literal regardless of quoting, so it would trip on this call site.
sweep_mark_publish_deferred(){
  local mw="$1" archived="$2" id="$3" detail="$4"
  [ -z "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ] || return 0
  ! _docket_tree_wedged "$GIT" "$mw" || return 0
  "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/mark-publish-deferred.sh --mode add --change-file "$archived" \
    --reason blocked --detail "$detail" \
    --integration-branch "$INTEGRATION_BRANCH" --id "$id" >/dev/null 2>&1 || return 0
  if "$GIT" -C "$mw" add -- "$archived" >/dev/null 2>&1 \
     && "$GIT" -C "$mw" commit -q -m "docket($id): mark terminal publish deferred (blocked)" -- "$archived" >/dev/null 2>&1; then
    "$GIT" -C "$mw" push >/dev/null 2>&1 || true
  else
    "$GIT" -C "$mw" checkout HEAD -- "$archived" >/dev/null 2>&1 || true
  fi
  return 0
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
  active="$(find "$cd_dir/active" -maxdepth 1 -name "${pad}-*.md" 2>/dev/null | sed -n 1p)"
  if [ -z "$active" ]; then
    return 0   # already archived — idempotent no-op
  fi
  status="$(field "$active" status)"
  docket_status_is_terminal "$status" && return 0   # already terminal — idempotent no-op

  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/archive-change.sh \
        --changes-dir "$cd_dir" --id "$id" --outcome done --date "$merged_date" \
        --message "docket($id): done — archived (status done, $merged_date)" >&2; then
    echo "sweep-failed $id archive script-error"
    return 0
  fi

  local archived
  archived="$(find "$cd_dir/archive" -maxdepth 1 -name "${merged_date}-${pad}-*.md" 2>/dev/null | sed -n 1p)"
  if [ -z "$archived" ]; then
    echo "sweep-failed $id archive archived-file-not-found"
    return 0
  fi

  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/render-change-links.sh \
        --change-file "$archived" --adrs-dir "$mw/$ADRS_DIR" >&2; then
    # Change 0118: mark, so this gap is not invisible. The pre-0118 rationale — "nothing published
    # means nothing was deferred yet" — does not survive the code: once archived the change leaves
    # active/, and the sweep scans active/ ONLY, so no later pass ever resumes it. The gap is
    # permanent until a human acts, which is byte-for-byte the state ADR-0051 exists to surface.
    # The marker cannot flap here either — only terminal-publish.sh's success path strips it, and
    # nothing retries an archived change — so even a TRANSIENTLY caused mark is stable, not noisy
    # (and the cause can be transient: render-change-links.sh resolves config through
    # docket-config.sh --export, which does a `git fetch`, so a network blip fires this branch).
    #
    # The gate is LOAD-BEARING here, and is the one structural difference from the change-0083
    # block below, which needs none: both of that branch's suppressions are exit-0 no-ops, so it is
    # unreachable under suppression, whereas a renderer failure fires regardless of the knob. Under
    # `terminal_publish: false` or in main-mode a skipped publish is SUCCESS, never a deferral
    # (ADR-0051) — the residual there stays what docket-status.md already documents: a stale
    # `## Artifacts` block, fixed by a manual re-render.
    if [ "${TERMINAL_PUBLISH:-false}" = true ] && [ "${DOCKET_MODE:-}" = docket ]; then
      sweep_mark_publish_deferred "$mw" "$archived" "$id" \
        "sweep: the artifacts re-render failed, so the publish was never attempted — re-render before publishing"
    fi
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
  # Change 0247: the same two rules as commit_and_push_generated, at the second exposed commit into
  # the shared worktree — probe for a rebase/merge in progress first (committing into one writes
  # onto its detached HEAD), and scope both the add and the commit with `--` so another agent's
  # staged work is not swept in under this run's message. The new reason keeps this block's
  # report-and-continue posture: like commit-failed it is COSMETIC here — terminal-publish.sh and
  # cleanup-feature-branch.sh still run, and the ## Artifacts block self-heals next pass.
  if [ -n "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ]; then
    if _docket_tree_wedged "$GIT" "$mw"; then
      echo "sweep-failed $id render-change-links blocked-wedged-tree"
    elif ! "$GIT" -C "$mw" add -- "$archived" >&2 \
      || ! "$GIT" -C "$mw" commit -q -m "docket($id): refresh artifacts links" -- "$archived" >&2; then
      echo "sweep-failed $id render-change-links commit-failed"
    elif ! "$GIT" -C "$mw" push >&2; then
      echo "sweep-failed $id render-change-links push-failed"
    fi
  fi

  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/terminal-publish.sh \
        --id "$id" --outcome done --enabled "${TERMINAL_PUBLISH:-false}" \
        --integration-branch "$INTEGRATION_BRANCH" --metadata-branch "$METADATA_BRANCH" \
        --changes-dir "$CHANGES_DIR" --adrs-dir "$ADRS_DIR" --metadata-worktree "$mw" \
        --message "docket($id): publish terminal record (done)" >&2; then
    # Change 0083: this is the HIGHEST-VOLUME path on which a publish does not complete, so it
    # marks itself rather than relying on a driver to notice a report line. Without the marker the
    # change is archived-but-unpublished and invisible — no later sweep resumes it (the sweep only
    # scans active/) and board-checks' `publish-deferred` reads a marker nothing wrote. This branch
    # only fires when terminal-publish.sh EXITS NON-ZERO, and neither main-mode nor `--enabled
    # false` can reach it (both are no-ops that exit 0), so the never-mark-under-suppression rule
    # holds without a second gate here.
    #
    # The mark's posture, its recovery on each failure point, and the --detail literal ban now
    # live once, on sweep_mark_publish_deferred above. Change 0118 also gave this path recovery it
    # did not have: the pre-0118 bare `&&` chain could leave the archived file dirty or staged in
    # the shared worktree when the marker wrote and `add`/`commit` failed.
    sweep_mark_publish_deferred "$mw" "$archived" "$id" "sweep: the publish step exited non-zero"
    echo "sweep-failed $id terminal-publish script-error"
    return 0
  fi

  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/cleanup-feature-branch.sh --slug "$slug" >&2; then
    echo "sweep-failed $id cleanup script-error"
  fi

  echo "swept $id $merged_date"
  echo "harvest $id $archived"
}

# health_checks — runs board-checks.sh (the mechanical git-only checks) over the current
# changes-dir and prefixes each TSV finding line as "check <check-id> <change-id> <message>".
# Best-effort: a clean tree prints nothing extra; a board-checks.sh failure now prints
# "health checks failed <exit>" (change 0144) — either way this never aborts the pass.
health_checks(){
  local mw
  mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
  local cd_dir="$mw/$CHANGES_DIR"
  local metadata_branch
  if [ "${DOCKET_MODE:-}" = docket ]; then metadata_branch="$METADATA_BRANCH"; else metadata_branch="$INTEGRATION_BRANCH"; fi
  # change 0117: the adr-unpublished check is opt-in on --adrs-dir, and its gate opens only when
  # terminal_publish is true AND we are in docket-mode. BOTH legs are resolved HERE, not in
  # board-checks.sh: mode is this script's knowledge, and in main-mode the metadata and integration
  # refs coincide so the comparison is vacuous. "${TERMINAL_PUBLISH:-false}" guards a stale or
  # mocked config export that does not emit the key (the change-0064 unbound-variable shape).
  local adr_args=()
  # change 0117 review: ADRS_DIR is never empty (docket-config.sh defaults it to docs/adrs and
  # always exports it), so gating on -n alone always passes --adrs-dir even when the directory
  # does not exist (a fresh repo that skipped seeding adrs/ — migrate-to-docket.sh's fresh-repo path). board-
  # checks.sh's exit-2-on-missing-dir rule is correct for a hand-run caller with a typo'd path,
  # but here it would exit the whole pipeline non-zero and silently drop EVERY health check
  # (broken-spec, dep-cycle, merged-orphan, board-row-dropped, ...), not just adr-unpublished.
  # Require the directory to actually exist before ever passing --adrs-dir.
  if [ -n "${ADRS_DIR:-}" ] && [ -d "$mw/$ADRS_DIR" ]; then
    adr_args+=(--adrs-dir "$mw/$ADRS_DIR")
    if [ "${TERMINAL_PUBLISH:-false}" = true ] && [ "${DOCKET_MODE:-}" = docket ]; then
      adr_args+=(--terminal-publish)
    fi
  fi
  # Guarded expansion: this repo's floor is Bash 4 (scripts/docket.sh, ensure-docket-env.sh), and
  # "${adr_args[@]}" on a declared-but-empty array throws "unbound variable" under set -u on bash
  # 4.0-4.3 (fixed upstream in 4.4) — the same change-0064 crash shape, one line lower.
  # Capture-then-consume (change 0144). board-checks.sh accumulates findings into $FINDINGS and
  # prints once at the END, so a validation failure (exit 2) emits ZERO TSV lines: piping it
  # straight into the read loop made a broken checker byte-indistinguishable from a clean tree.
  # This file's own idiom — reclaim_pass captures then greps a here-string, per change 0067's
  # no-pipefail-SIGPIPE rule. The prologue is `set -uo pipefail` (no -e), so `|| rc=$?` is safe.
  # The loop no longer runs in a pipeline subshell, so its three variables must be `local`.
  local out rc=0 check_id change_id message
  out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" \
    --results-dir "${RESULTS_DIR:-docs/results}" \
    --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" ${adr_args[@]+"${adr_args[@]}"} 2>&2)" || rc=$?
  # An empty $out yields one blank line from <<<, already swallowed by the [ -n ] guard below.
  while IFS=$'\t' read -r check_id change_id message; do
    [ -n "$check_id" ] || continue
    echo "check $check_id $change_id $message"
  done <<<"$out"
  # ADDITIVE, never a replacement: findings print first, then the diagnostic. board-checks.sh's
  # --strict path already prints $FINDINGS and THEN exits 1, so the emit-then-fail shape is one
  # flag away from this caller. The exit code is the whole payload — the CAUSE stays on stderr,
  # where board-checks.sh already writes it and this function passes it through untouched. Same
  # contract as `board inline failed`: the line signals WHICH STEP failed, not why.
  [ "$rc" -eq 0 ] || printf 'health checks failed %s\n' "$rc"
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
#
# SCOPED to stale-in-progress lines (change 0104 review). The marker is a MACHINE CONTRACT between
# board-checks and this gate, and this gate is mutating — so it must never be keyed on an unscoped
# substring search of the whole findings blob. Findings echo untrusted frontmatter verbatim by
# design (`field-domain` quotes `status`, `slug` and free-form `title` prose), so a change file
# carrying `title: Sneaky | thing [reclaimable]` would otherwise FORGE the marker: under
# reclaim.auto=true that triggers the reclaim sweep, and with auto off it prints a false remedy line
# and inflates the count. health_checks renders each finding as `check <check-id> <change-id>
# <message>`, so anchoring at ^ pins the check-id COLUMN — a marker appearing inside some other
# check's message can never satisfy it, because that line begins with that other check's id. The
# marker is additionally required at end-of-line, which is where board-checks stamps it.
RECLAIMABLE_LINE_RE='^check stale-in-progress [^ ]+ .*\[reclaimable\]$'
reclaim_pass(){
  local health_out="$1" mw cd_dir line n
  # grep -c prints 0 and exits 1 on no-match; capture it, then TEST — the count and the gate come
  # from ONE evaluation, so the remedy can never name a number the gate did not agree with.
  n="$(grep -cE "$RECLAIMABLE_LINE_RE" <<<"$health_out")" || n=0
  [ "${n:-0}" -gt 0 ] || return 0
  if [ "${RECLAIM_AUTO:-false}" = true ]; then
    mw="$(docket_metadata_worktree)"   # ABSOLUTE (change 0075) — see board_pass.
    cd_dir="$mw/$CHANGES_DIR"
    while IFS= read -r line; do
      [ -n "$line" ] && printf 'reclaim %s\n' "$line"
    done < <("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/reclaim-claims.sh --changes-dir "$cd_dir" --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}")
  else
    printf 'reclaim: %s expired-lease change(s) can self-heal — run: docket.sh reclaim-claims\n' "$n"
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
    # VERBATIM, not fm_field: blocked_by is free prose where a whitespace-preceded `#` is DATA
    # (`PR #69 is stale`), and fm_field's comment strip would truncate it to `PR`.
    blocked_by="$(fm_field_verbatim "$f" blocked_by)"
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
  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/render-learnings-index.sh --learnings-dir "$ldir" >"$tmp" 2>/dev/null; then
    rm -f "$tmp"
    return 1
  fi
  if [ ! -s "$tmp" ]; then
    rm -f "$tmp"
    return 1
  fi
  chmod 644 "$tmp"
  mv -f "$tmp" "$ldir/README.md"
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
    state="$(fm_field "$f" promotion_state)"   # anchored: optional key (ADR-0057)
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
  # Same explicit arm, for the same reason, as board_pass_inline's — this `case` carries the
  # identical retryable catch-all (change 0247, spec Assumption 16(a)).
  case "$result" in
    clean)               printf 'learnings index clean\n' ;;
    changed-pushed)      printf 'learnings index changed pushed\n' ;;
    blocked-wedged-tree) printf 'learnings index blocked-wedged-tree\n' ;;
    *)                   printf 'learnings index changed push-failed\n' ;;
  esac

  learnings_advisories "$ldir"
}

# integration_sync — best-effort FF-only sync of the invoking repo's integration-branch
# checkout, run once at the end of a pass that swept at least one change.
integration_sync(){
  "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/sync-integration-branch.sh --integration-branch "$INTEGRATION_BRANCH" >&2 2>&1 || true
  return 0
}

# preflight_wedged_report — maps a FAILED Step-0 preflight onto this script's OWN documented
# blocked-wedged-tree vocabulary. Returns 0 when it recognized the wedge and emitted the report
# line(s); 1 when it did not, leaving the caller's bare `exit 1` in force.
#
# Change 0247 made _docket_sync_metadata fail closed on a shared metadata worktree that already has
# a rebase or merge in progress — it used to green-light exactly that state, its fast path
# returning before the wedge was ever probed. Correct, but it moves the refusal AHEAD of both write
# paths: main() exits at Step 0, so neither commit_and_push_generated's own probe nor
# sweep_execute_one's step-6a probe is ever reached, and a PRE-EXISTING wedge would leave the
# report channel silent on the one condition this script documents a token for. Silence on a path
# that still exits 0 is the exact failure `board inline failed` was minted to close (change 0071
# review, finding 1) — and a token documented in docket-status.md that no run can emit is worse
# than no token at all.
#
# So the two layers split the job rather than duplicating it: preflight REFUSES TO SYNC a wedged
# tree, and this says WHY. The in-function probes keep the case no Step-0 check can ever catch —
# the TOCTOU window in which another agent starts its rebase AFTER preflight returned, mid-pass.
# tests/test_docket_status.sh exercises that window through the RENDER_BOARD seam, so neither probe
# is redundant with the other; deleting either one reddens a different assert.
#
# Only a tree this function RE-PROBES as wedged is mapped; every other preflight failure keeps the
# bare `exit 1`. `BOOTSTRAP=PROCEED` is a conjunct, not decoration: config export and the bootstrap
# gate return BEFORE the sync, and softening one of those fail-closed exits into a best-effort exit
# 0 because some unrelated rebase happens to be in flight would re-open a fail-open door right
# beside the one this change shut. A config that never resolved leaves BOOTSTRAP unset, which fails
# the same conjunct.
preflight_wedged_report(){
  [ "${BOOTSTRAP:-}" = PROCEED ] || return 1
  local mw
  mw="$(docket_metadata_worktree)"
  [ -n "$mw" ] || return 1
  _docket_tree_wedged "$GIT" "$mw" || return 1
  # ONE line per consumer that would actually have run and would actually have hit its own wedge
  # probe — never a blanket pair. `learnings index blocked-wedged-tree` under --board-only (which
  # never runs that pass) or in a learnings-off repo would be a report line for work that was never
  # scheduled, which is the same lie as silence with the polarity flipped. The board line is gated
  # on the `inline` surface for the same reason: it is the only surface that writes into the shared
  # worktree, and `none`/`github` reach no wedge.
  case " ${BOARD_SURFACES:-} " in
    *" inline "*) echo "board inline blocked-wedged-tree" ;;
  esac
  if [ "$BOARD_ONLY" != 1 ] && [ "${LEARNINGS_ENABLED:-true}" = true ] \
     && [ -d "$mw/${CHANGES_DIR:-}/learnings" ]; then
    printf 'learnings index blocked-wedged-tree\n'
  fi
  return 0
}

main(){
  # change 0094: the write-free read short-circuits BEFORE docket_preflight (which syncs).
  if [ "$DIGEST_ONLY" = 1 ]; then
    digest_only_pass || exit 1
    exit 0
  fi
  if ! docket_preflight "$SCRIPTS_DIR"; then
    preflight_wedged_report || exit 1
    # --must-land halts: a wedged tree is NOT LANDED, byte for byte the verdict board_classify
    # reaches on the same token when the pass does get to run. Flagless stays BEST-EFFORT (exit 0),
    # which is what docket-status.md promises this token's callers — and `pass ok` is deliberately
    # absent, because no pass ran to completion here and that marker means exactly that.
    if [ "$MUST_LAND" = 1 ]; then exit 1; fi
    exit 0
  fi
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
  # Change 0219: the GitHub enrichment for leg C, printed immediately after the git-only findings so
  # a leg-C finding and its resolution read together. Deliberately NOT folded into $health_out:
  # reclaim_pass keys a MUTATING gate on that blob (RECLAIMABLE_LINE_RE), and widening what feeds it
  # with network-derived lines would put a remote service inside a local mutation's trigger. This is
  # advisory output only. FULL PATH ONLY — never under --board-only, which early-exits above and is
  # invoked by many callers as a must-land board write.
  detect_orphan_pr
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
