#!/usr/bin/env bash
# scripts/runner-dispatch.sh — the runner-neutral delegation facade (change 0079), behind
# `docket.sh runner-dispatch`. Validates arguments, anchors the repo root (ADR-0034),
# resolves the runners.<name>: config block across layers (repo-local > repo-committed >
# global; per-key), exports it as DOCKET_RUNNER_CFG_<KEY>, and CALLS the named adapter
# scripts/runners/<name>.sh in the foreground, returning with its verbatim exit code (change 0237
# replaced the original `exec` so the facade regains control at that seam). On that seam sits the
# RUN GATE: for an `implement-next` delegation only, an unfinished run gets ONE re-dispatch and a
# second strike exits 1 — the sole path where the facade does not return the adapter's own code.
# Registration IS the adapter file's existence. Unknown runner => loud nonzero (abort-and-report).
# Contract: scripts/runner-dispatch.md.
# Mock seams: RUNNERS_DIR, GIT (via lib/docket-root.sh).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNERS_DIR="${RUNNERS_DIR:-$SELF_DIR/runners}"
# Run-gate seams (change 0237): the disposition reader, and the facade used for the post-return
# metadata re-sync that makes the "after" snapshot a read of fresh origin state.
VERIFY_RUN="${VERIFY_RUN:-$SELF_DIR/verify-run.sh}"
DOCKET_FACADE="${DOCKET_FACADE:-$SELF_DIR/docket.sh}"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR/lib/docket-root.sh"

die(){ printf 'runner-dispatch: %s\n' "$*" >&2; exit 1; }

RUNNER=""; AGENT=""; MODEL=""; EFFORT=""; WORKTREE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --runner) RUNNER="${2:-}"; shift 2 ;;
    --agent)  AGENT="${2:-}";  shift 2 ;;
    --model)  MODEL="${2:-}";  shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --worktree) WORKTREE="${2:-}"; shift 2 ;;
    --) shift; break ;;
    *) die "unknown argument: $1" ;;
  esac
done
[ -n "$RUNNER" ] || die "--runner is required"
[ -n "$AGENT" ]  || die "--agent is required"
# The runner name becomes a path component below — reject anything that could traverse out
# of RUNNERS_DIR (the facade family is a finite table, never an escape hatch).
case "$RUNNER" in
  *[!A-Za-z0-9._-]*|*..*) die "invalid runner name '$RUNNER'" ;;
esac
ADAPTER="$RUNNERS_DIR/$RUNNER.sh"
if [ ! -f "$ADAPTER" ]; then
  registered="$(ls "$RUNNERS_DIR" 2>/dev/null | sed -n 's/\.sh$//p' | tr '\n' ' ')"
  die "unknown runner '$RUNNER' — no adapter at $ADAPTER (registered runners: ${registered:-<none>})"
fi

# --- anchor: an explicit argument defaulting to the main worktree (change 0206) -----
# ADR-0034 is UNAMENDED. Routing --worktree through docket_anchor_path rather than using it raw
# is the whole point: a relative value joins to the MAIN worktree, so it resolves identically from
# any cwd, and the new argument inherits ADR-0034's cwd-independence instead of quietly
# reintroducing the hazard ADR-0034 was written against. Absent --worktree the expression is
# docket_anchor_path "." — the main worktree — so every currently-shipped shim is unaffected.
REPO_ROOT="$(docket_main_worktree)"
[ -n "$REPO_ROOT" ] || die "not inside a git repository"
# Gate 1 — a build worker must run INSIDE its feature worktree. This is the one piece of
# agent-family knowledge the facade gains; it is a RUNTIME requirement (the path is runtime data),
# so sync-agents.sh's generation-time slot cannot substitute for it. Loud, matching the facade's
# posture for an unknown --runner rather than its tolerant posture for a runners.<name>: value:
# that tolerance exists so a cosmetic config typo cannot fail a live dispatch, whereas this is a
# request the facade cannot serve correctly.
case "$AGENT" in
  build-*) [ -n "$WORKTREE" ] || die "--worktree is required for build-* agents (a build worker must run in its feature worktree, not the main tree)" ;;
esac
ANCHOR="$(docket_anchor_path "${WORKTREE:-.}")"
# Gate 2 — the resolved anchor must exist as a directory.
[ -d "$ANCHOR" ] || die "--worktree $ANCHOR is not a directory"
# Gate 3 — and belong to THIS repo's worktree set, so a child harness running under an
# auto-approve permission grant is never handed a tree docket does not own. A non-repo path makes
# docket_main_worktree print nothing, so the not-a-repo case falls out of this same comparison.
[ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ] || die "--worktree $ANCHOR is not a worktree of this repository"
export DOCKET_REPO_ROOT="$ANCHOR"

# --- runners.<name>: config, per-key across layers (local > committed > global) -----
# Same nested-section awk shape as sync-agents.sh's section_body (kept self-contained
# here; sync-agents.sh has the twin — tracked divergence, see LEARNINGS on twins).
yaml_section(){  # $1=key ; reads stdin -> the dedented body under <key>:, '' when absent
  awk -v key="$1" '
    function ind(s,   m){ m=match(s, /[^[:space:]]/); return (m==0 ? length(s) : m-1) }
    { nc=$0; sub(/#.*/,"",nc) }
    !inb { if (nc ~ ("^[[:space:]]*" key "[[:space:]]*:[[:space:]]*$")) { inb=1; kin=ind(nc) } next }
    nc ~ /[^[:space:]]/ && ind(nc) <= kin { exit }
    { if (!haveBase && nc ~ /[^[:space:]]/) { base=ind($0); haveBase=1 }
      if (haveBase) print substr($0, base+1); else print }
  '
}
runner_block(){  # $1=file -> the dedented body under runners.<RUNNER>., '' when absent
  [ -f "$1" ] || return 0
  yaml_section runners < "$1" | yaml_section "$RUNNER"
}

GLOBAL_CFG="${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml"
seen_keys=" "
for f in "$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"; do
  blk="$(runner_block "$f")"
  [ -n "$blk" ] || continue
  while IFS= read -r line; do
    k="$(sed -nE 's/^[[:space:]]*([A-Za-z0-9._-]+)[[:space:]]*:.*/\1/p' <<<"$line")"
    [ -n "$k" ] || continue
    case "$seen_keys" in *" $k "*) continue ;; esac   # first (highest-precedence) layer wins per key
    seen_keys="$seen_keys$k "                          # claim the key for THIS layer before parsing its
                                                       # value, so a malformed high-precedence value still
                                                       # masks lower layers (precedence is per-key, not per-value)
    # Value class (change 0173): rest of the line, trailing `# comment` stripped, whitespace
    # trimmed. This is a BLOCK mapping (one key per line), not sync-agents.sh's flow map, so the
    # flow-map class would be wrong here — it would admit a slash-bearing path only by luck. The
    # pre-0173 class ([A-Za-z0-9._-]+) truncated `https://host/v1` to `https` and dropped a
    # leading-slash path entirely. Comment detection requires leading whitespace, per YAML, so a
    # value containing `#` (a URL fragment) survives. A COMMENT-ONLY value needs its own leg: the
    # capture's `[[:space:]]*` is greedy and eats the space before the `#`, so the whitespace-
    # preceded strip below can never fire on it — without this the comment TEXT would be exported
    # as the value, which is worse than the truncation this change removes (change 0173 review).
    # A YAML plain scalar can never begin with `#`, so stripping a leading-`#` value is safe.
    # Posture stays TOLERANT — an unparseable value `continue`s rather than dying. This path runs
    # mid-handoff to a child process, where dying converts a cosmetic config typo into a failed
    # dispatch. sync-agents.sh is loud instead; the asymmetry is deliberate (change 0173).
    v="$(sed -nE 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*(.*)$/\1/p' <<<"$line")"
    v="${v%%$'\n'*}"
    v="$(sed -E -e 's/^#.*$//' -e 's/[[:space:]]+#.*$//' -e 's/[[:space:]]+$//' <<<"$v")"
    [ -n "$v" ] || continue
    uk="$(tr '[:lower:]' '[:upper:]' <<<"$k" | tr '.-' '__')"
    export "DOCKET_RUNNER_CFG_$uk=$v"
  done <<<"$blk"
done

# --- handoff: foreground, adapter owns everything child-specific --------------------
args=( --agent "$AGENT" )
[ -n "$MODEL" ]  && args+=( --model "$MODEL" )
[ -n "$EFFORT" ] && args+=( --effort "$EFFORT" )

# --- run gate (change 0237), part 1: the "before" snapshot --------------------------
# Engages ONLY for an implement-next delegation. That scoping is load-bearing, not an
# optimization: a build-* delegation leaves its change `in-progress` BY DESIGN (the build role
# does not reach Step 7), so gating one would fire on every healthy build. status / adr /
# review-* / finalize-change / auto-groom are likewise out of scope, and an unrecognised agent is
# a no-op — never a guess.
#
# The snapshot must be taken BEFORE the handoff: the gate's subject is what THIS run claimed, and
# that is only knowable as a diff across the hand-off. A snapshot that cannot be read disables the
# gate with a warning and leaves the dispatch untouched — the facade's standing tolerant posture on
# this live path (the same reason an unparseable runners.<name>: value `continue`s rather than
# dying): the gate must never convert a healthy dispatch into a failure.
GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1

in_progress_ids(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids 2>/dev/null; }

BEFORE=""
if [ "$GATE" = 1 ]; then
  if ! BEFORE="$(in_progress_ids)"; then
    printf 'runner-dispatch: run gate disabled — could not read the in-progress set\n' >&2
    GATE=0
  fi
fi

# CALL-AND-RETURN, not exec (change 0237). The facade must regain control so the run gate can read
# what the delegated run actually left in git — that seam is the whole point, and `exec` made it
# unreachable by replacing this process with the adapter's image. Removing `exec` shifts process
# semantics: the adapter is now a CHILD rather than a replacement image, so it gets its own pid and
# this process stays alive as its parent. Two consequences are deliberate. (1) The adapter's exit
# code is captured and propagated VERBATIM on every path where the gate takes no action, so no
# existing caller observes a behavior change.
#
# (2) SIGNAL DELIVERY, measured rather than assumed (spec Risk 2; change 0237 build). The facade
# installs NO traps, so behavior splits by how the signal is addressed:
#   - GROUP-directed (what a terminal Ctrl-C does — it signals the whole foreground process group):
#     UNCHANGED. The adapter is in that group and gets the signal directly, its own trap fires
#     immediately, and the facade dies of the same signal (130 on INT, 143 on TERM). This is the
#     interactive path, and it is the reason no trap is added here.
#   - PID-directed (a supervisor doing `kill -INT <facade pid>`, e.g. `timeout`): CHANGED, because
#     the facade is now a separate process that absorbs the signal instead of being the adapter.
#     A pid-directed INT is deferred while this shell waits on its child and is then discarded
#     (exit 0); a pid-directed TERM kills this shell at once and ORPHANS the still-running adapter.
#     Under `exec` neither was possible — there was only one process to signal. Nothing in docket
#     signals the facade by pid today; forwarding traps are deliberately NOT added here because
#     that is a design decision for the run gate's own error posture, not a silent side effect of
#     this refactor.
"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
rc=$?

# --- run gate, part 2: the "after" snapshot, the diff, and the verdicts -------------
[ "$GATE" = 1 ] || exit "$rc"

# The "after" read must come from FRESH ORIGIN state, never from the local tree the child just
# wrote (LEARNINGS: cas-re-read-fresh-origin). Best-effort: a failed re-sync degrades the gate's
# freshness, it does not fail a dispatch that may well have succeeded.
"$DOCKET_BASH_PATH" "$DOCKET_FACADE" preflight >/dev/null 2>&1 \
  || printf 'runner-dispatch: run gate — metadata re-sync failed; verifying against local state\n' >&2

AFTER="$(in_progress_ids)" || {
  printf 'runner-dispatch: run gate disabled — could not re-read the in-progress set\n' >&2
  exit "$rc"
}

# This run's claim = any id in AFTER that was not in BEFORE. A change another agent already held
# was in BEFORE and is ignored, so concurrent runs never cross-fire; a run that claimed nothing
# (drained, or contended where the CAS was lost) yields an empty diff and the gate is a no-op.
NEW_IDS=()
while IFS= read -r nid; do
  [ -n "$nid" ] || continue
  grep -qxF "$nid" <<<"$BEFORE" || NEW_IDS+=("$nid")
done <<<"$AFTER"

STILL_INCOMPLETE=()
for nid in "${NEW_IDS[@]:-}"; do
  [ -n "$nid" ] || continue
  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) : ;;
    # run-halted NEVER re-dispatches: a halt means a human is needed, and spending a second full
    # agent run on it is waste. run-complete and run-unclaimed need nothing. An empty/unparseable
    # verdict falls here too — the gate acts only on a POSITIVE finding, never on a guess.
    *) continue ;;
  esac

  # ONE bounded re-dispatch — docket-build's one-escalation-per-task rule, applied at this seam.
  # One re-dispatch is a real cost (a false `run-incomplete` spends a full agent run), which is
  # exactly why the bound is one and not a loop: the second strike stops and tells a human.
  unmet="${verdict#run-incomplete "$nid" }"
  retry_ctx="docket-implement-next $nid — the previous run left Step 7 unmet (${unmet}); resume that change and finish it: push the branch, open the PR, and write status: implemented + pr:. If it genuinely cannot proceed, write a '## Run halted' section into the change file and commit it — the heading must be bare and undated (the reader matches the whole line), so put the date inside the body."
  printf 'runner-dispatch: run gate — re-dispatching once for change %s (%s)\n' "$nid" "$unmet" >&2
  "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@" "$retry_ctx"

  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — after re-dispatch: %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) STILL_INCOMPLETE+=("$verdict") ;;
  esac
done

if [ "${#STILL_INCOMPLETE[@]}" -gt 0 ]; then
  # Abort-and-report. The change stays `in-progress` with its claim intact; board-checks'
  # `aborted-run` remains the standing backstop. This is the only NEW non-zero this change
  # introduces, and it is on a path that is presently silent. The retry's own exit code is
  # deliberately NOT propagated: `$rc` (the first adapter's code) is what every no-action path
  # returns, and this abort supersedes it with 1 rather than reporting a second, different code.
  printf 'runner-dispatch: RUN GATE FAILED after one re-dispatch — a delegated implement-next run did not reach its PR:\n' >&2
  for v in "${STILL_INCOMPLETE[@]}"; do printf '  %s\n' "$v" >&2; done
  exit 1
fi

exit "$rc"
