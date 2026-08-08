#!/usr/bin/env bash
# scripts/runner-dispatch.sh — the runner-neutral delegation facade (change 0079), behind
# `docket.sh runner-dispatch`. Validates arguments, anchors the repo root (ADR-0034),
# resolves the runners.<name>: config block across layers (repo-local > repo-committed >
# global; per-key), exports it as DOCKET_RUNNER_CFG_<KEY>, and CALLS the named adapter
# scripts/runners/<name>.sh in the foreground, returning with its verbatim exit code (change 0237
# replaced the original `exec` so the facade regains control at that seam). On that seam sits the
# RUN GATE: for an `implement-next` delegation only, an unfinished run gets ONE re-dispatch and a
# second strike exits 1, a halted run stops the caller with 3, and a re-dispatch that drove the run
# to completion exits 0 over a stale first code — the only three paths where the facade does not
# return the adapter's own code.
# Registration IS the adapter file's existence. Unknown runner => loud nonzero (abort-and-report).
# Contract: scripts/runner-dispatch.md.
# Mock seams: RUNNERS_DIR, GIT (via lib/docket-root.sh).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNERS_DIR="${RUNNERS_DIR:-$SELF_DIR/runners}"
# Run-gate seams (change 0237): the disposition reader, and the facade used for the metadata
# re-syncs that make BOTH snapshots — "before" and "after" — reads of fresh origin state.
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
# `inherit` is DOCKET'S OWN no-pin sentinel (never a vendor model ID), normalized to "no model"
# HERE — the one layer every adapter is dispatched through — so no adapter re-decides it. The
# handoff below already gates the flag on `[ -n "$MODEL" ]`, which makes the sentinel
# indistinguishable from "no model supplied": exactly the model-less hand path every adapter
# contract documents. NORMALIZE, not reject — ADR-0067 already rejects the sentinel at GENERATION
# time (sync-agents.sh's runner_config_error), so a dispatch-time `inherit` is a hand invocation,
# and the hand contract is tolerant. This is sentinel normalization, NOT model-ID validation
# (ADR-0015): no vendor value is inspected and no allowlist is introduced.
[ "$MODEL" = "inherit" ] && MODEL=""
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
#
# BOTH reads must come from FRESH ORIGIN state (LEARNINGS: cas-re-read-fresh-origin), which is why
# the re-sync below is symmetric with the one after the handoff. An asymmetric pair is not merely
# imprecise, it is actively wrong: a change that is `in-progress` on origin/docket but not yet in
# the local .docket worktree is absent from a stale BEFORE and present in the freshly-synced AFTER,
# so an ABANDONED claim from an earlier session (exactly what board-checks' `aborted-run` leg
# exists for) would be attributed to this run and spend a whole agent run being re-dispatched.
GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1

in_progress_ids(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids 2>/dev/null; }
in_progress_claims(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids --with-claimed-at 2>/dev/null; }
# Best-effort metadata re-sync, used on BOTH sides of the handoff. A failure degrades the gate's
# freshness; it never fails a dispatch.
resync_metadata(){
  "$DOCKET_BASH_PATH" "$DOCKET_FACADE" preflight >/dev/null 2>&1 \
    || { printf 'runner-dispatch: run gate — metadata re-sync failed; verifying against local state\n' >&2; return 1; }
}

BEFORE=""; DISPATCH_EPOCH=""
if [ "$GATE" = 1 ]; then
  resync_metadata || :
  if ! BEFORE="$(in_progress_ids)"; then
    printf 'runner-dispatch: run gate disabled — could not read the in-progress set\n' >&2
    GATE=0
  fi
  # The claim window opens here — AFTER the before-read, so a claim landing in the gap between the
  # two is either already in BEFORE or stamped before the window, and is excluded either way.
  # `date -u +%s` is UTC epoch seconds on GNU and BSD alike, and is the ONLY timestamp work done
  # here: the ISO->epoch direction (which is where the two `date` dialects actually diverge) stays
  # in verify-run, on the shared `iso_to_epoch`. The claim writer stamps UTC ISO-8601
  # (`date -u +%Y-%m-%dT%H:%M:%SZ`), so the two ends meet on the same clock.
  DISPATCH_EPOCH="$(date -u +%s 2>/dev/null)"
  case "$DISPATCH_EPOCH" in
    ''|*[!0-9]*)
      printf 'runner-dispatch: run gate disabled — could not read the clock\n' >&2
      GATE=0 ;;
  esac
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
resync_metadata || :

AFTER="$(in_progress_claims)" || {
  printf 'runner-dispatch: run gate disabled — could not re-read the in-progress set\n' >&2
  exit "$rc"
}

# ATTRIBUTION. A candidate for "this run's claim" must clear THREE filters, because the set diff
# alone does not identify a claimant — it only identifies a change of state:
#
#   1. not in BEFORE — a claim already held at the handoff is another run's (or an earlier
#      session's), and both reads are of fresh origin so BEFORE really is the whole prior set;
#   2. `claimed_at` readable — an absent or unparseable stamp is NO POSITIVE EVIDENCE of ownership,
#      and the gate acts only on a positive finding, never on a guess;
#   3. `claimed_at` at or after DISPATCH_EPOCH — a claim stamped before this run started cannot be
#      ours however it came to be visible. This is what excludes the abandoned in-progress change
#      that the pre-handoff re-sync above pulled in for the first time, even on the path where that
#      re-sync FAILED and BEFORE is stale — belt and braces, deliberately.
#
# WHAT THIS DOES NOT ESTABLISH, stated plainly so no later reader trusts more than it holds: a
# timestamp cannot separate our claim from one a CONCURRENT loop made during our run, and
# `claimed_at` is re-stamped at every phase boundary, so a foreign live run's stamp is fresh too.
# The ambiguity is therefore handled by COUNTING rather than by the clock: an implement-next run
# claims at most ONE change, so two or more candidates means at least one is not ours and none can
# be told apart — the gate disables itself rather than re-dispatching onto a change another agent
# is holding. A run that claimed nothing (drained, or contended where the CAS was lost) yields an
# empty set and the gate is a no-op.
NEW_IDS=()
while IFS=' ' read -r nid nclaimed; do
  [ -n "$nid" ] || continue
  grep -qxF "$nid" <<<"$BEFORE" && continue
  case "${nclaimed:-}" in
    ''|*[!0-9]*)
      printf 'runner-dispatch: run gate — ignoring change %s: no readable claimed_at\n' "$nid" >&2
      continue ;;
  esac
  [ "$nclaimed" -ge "$DISPATCH_EPOCH" ] || {
    printf 'runner-dispatch: run gate — ignoring change %s: claimed before this dispatch started\n' "$nid" >&2
    continue
  }
  NEW_IDS+=("$nid")
done <<<"$AFTER"

if [ "${#NEW_IDS[@]}" -gt 1 ]; then
  printf 'runner-dispatch: run gate disabled — %s changes were claimed during this dispatch (%s); this run claims at most one, so none can be attributed to it\n' \
    "${#NEW_IDS[@]}" "${NEW_IDS[*]}" >&2
  exit "$rc"
fi

STILL_INCOMPLETE=(); HALTED=(); GATE_VERIFIED_DONE=0
for nid in "${NEW_IDS[@]:-}"; do
  [ -n "$nid" ] || continue
  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) : ;;
    # run-halted NEVER re-dispatches: a halt means a human is needed, and spending a second full
    # agent run on it is waste. But it is not a no-op either — docket-implement-next's disposition
    # table pins `halted` to STOP + SURFACE, and returning the adapter's (healthy) 0 here would tell
    # a driver to draw the next change, which is exactly the prose-level failure this change exists
    # to replace. It gets its OWN terminal code at this seam (see below).
    run-halted*) HALTED+=("$verdict"); continue ;;
    # run-complete and run-unclaimed need nothing. An empty/unparseable verdict falls here too —
    # the gate acts only on a POSITIVE finding, never on a guess.
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
    # A halt discovered on the SECOND verdict is the same disposition as one discovered on the
    # first: terminal, exit 3. The re-dispatched run stopping deliberately is not a success, so it
    # never reaches the override below.
    run-halted*) HALTED+=("$verdict") ;;
    # The re-dispatch DROVE THE RUN HOME. From here the gate's own git-read verdict is a stronger
    # fact than the first adapter's status: that first code very often accompanies a run that
    # stopped short, and returning it would report a now-verified-complete run as a failure. Only a
    # POSITIVE second verdict qualifies — an empty or unparseable one falls through to `$rc`, since
    # the gate never acts on a guess.
    run-complete*|run-unclaimed*) GATE_VERIFIED_DONE=1 ;;
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

if [ "${#HALTED[@]}" -gt 0 ]; then
  # STOP + SURFACE, with its own code. 3, not 1: the two-strikes abort above is a run that FAILED
  # to finish, whereas a halt is the delegated run deliberately stopping because a human is needed —
  # two different terminal outcomes deserve two different codes, and a driver that wants to
  # distinguish them can. The consumer that exists today is the generated shim wrapper
  # (sync-agents.sh's emit_shim), whose rule is bare-non-zero: "if the dispatch exits non-zero,
  # abort-and-report its stderr diagnostic — never retry silently". That reading is CORRECT for a
  # halt — abort-and-report IS stop + surface — so the new code costs nothing there and the stderr
  # lines below carry the distinction. This is the one place the facade returns non-zero for
  # something that is not a failure of the dispatch itself; it is deliberate, and it is why the code
  # is distinct rather than folded into 1.
  printf 'runner-dispatch: RUN HALTED — a delegated implement-next run stopped and needs a human:\n' >&2
  for v in "${HALTED[@]}"; do printf '  %s\n' "$v" >&2; done
  printf 'runner-dispatch: not continuing — read the change file'"'"'s "## Run halted" section.\n' >&2
  exit 3
fi

# A re-dispatch ran and the second verdict positively showed the run finished. `$rc` is the FIRST
# adapter's code and is stale here — it describes the attempt the gate just superseded, and a
# non-zero first code is a common accompaniment to a run that stopped short. This is the one path
# where the facade drops the adapter's code without aborting; every path where NO re-dispatch ran
# still returns it verbatim, which is why the override keys on GATE_VERIFIED_DONE (set only inside
# the retry loop) rather than on the verdict alone.
[ "$GATE_VERIFIED_DONE" = 1 ] && exit 0

exit "$rc"
