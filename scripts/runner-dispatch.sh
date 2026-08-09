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
# return the adapter's own code. That synchronous seam now serves HAND invocations: every generated
# shim launches, so the same gate follows the delegated run onto the `--observe` seam below.
# Registration IS the adapter file's existence. Unknown runner => loud nonzero (abort-and-report).
# Change 0271 added the `--launch` verb: the same validated request, but the adapter is started in
# its OWN process group with every stream redirected into a durable per-dispatch dir, and the call
# returns at once with a dispatch key instead of blocking for the child's whole run. Its other half
# is `--observe <key>`: ONE short, idempotent look at that dispatch, synthesizing 0 (complete),
# 1 (failed or unavailable), 3 (a delegated implement-next run HALTED — the synchronous gate's own
# code, reached from the seam a delegated run actually returns through) or 4 (still running — NOT a
# failure, observe again), and killing the detached PROCESS GROUP when the observation budget is
# spent rather than orphaning the adapter.
# On that seam sits the DISAGREEMENT RULE: a sentinel claiming success with no matching git evidence
# is a failure. For a `build-*` agent the facade reads verify-run's build verdict; for
# `implement-next` it runs change 0237's run gate against the before-snapshot `--launch` recorded.
# BOTH report and stop — an observation never re-dispatches: a build task may have left partial
# commits, and re-launching a detached child out of a repeated short read would race the very run
# being observed.
# Contract: scripts/runner-dispatch.md.
# Mock seams: RUNNERS_DIR, GIT.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNNERS_DIR="${RUNNERS_DIR:-$SELF_DIR/runners}"
# The git seam. lib/docket-root.sh reads the same variable; naming it here makes it a seam of this
# script too, which the change-0271 launch verb uses directly for the dispatch-time SHA.
GIT="${GIT:-git}"
# Run-gate seams (change 0237): the disposition reader, and the facade used for the metadata
# re-syncs that make BOTH snapshots — "before" and "after" — reads of fresh origin state.
VERIFY_RUN="${VERIFY_RUN:-$SELF_DIR/verify-run.sh}"
DOCKET_FACADE="${DOCKET_FACADE:-$SELF_DIR/docket.sh}"
# shellcheck source=lib/docket-root.sh
. "$SELF_DIR/lib/docket-root.sh"
# shellcheck source=lib/docket-dispatch-dir.sh
. "$SELF_DIR/lib/docket-dispatch-dir.sh"

die(){ printf 'runner-dispatch: %s\n' "$*" >&2; exit 1; }

# The OS's own start time for a pid, as an OPAQUE identity token — captured at launch, compared at
# observation. It exists because `pgid` and `child_pid` are both REUSABLE names: an hour after a
# child died, neither one alone can still prove that the recorded group is the launched tree rather
# than something the OS handed that id to since (see the budget kill's identity check).
# Whitespace-normalized and then compared as an EXACT STRING, never parsed into a date: the
# `ps -o lstart=` rendering is platform- and locale-dependent, and both sides of the comparison come
# from the same `ps` on the same machine, so parsing it would add a failure mode and buy nothing.
ps_lstart(){  # $1 = pid -> normalized start-time token, empty when the pid is gone or unreadable
  local s
  s="$(ps -o lstart= -p "${1:-0}" 2>/dev/null)"
  s="$(tr -s '[:space:]' ' ' <<<"$s")"
  s="${s# }"
  printf '%s' "${s% }"
}

RUNNER=""; AGENT=""; MODEL=""; EFFORT=""; WORKTREE=""
# Verb selection (change 0271). Empty = the legacy synchronous call-and-return, which stays the
# default so no currently-shipped caller changes behavior. `--observe` is parsed HERE, with
# `--launch`, so the two verbs are validated together even though its branch lands with Task 4.
VERB=""; OBSERVE_KEY=""
while [ $# -gt 0 ]; do
  case "$1" in
    --runner) RUNNER="${2:-}"; shift 2 ;;
    --agent)  AGENT="${2:-}";  shift 2 ;;
    --model)  MODEL="${2:-}";  shift 2 ;;
    --effort) EFFORT="${2:-}"; shift 2 ;;
    --worktree) WORKTREE="${2:-}"; shift 2 ;;
    --launch)  VERB="launch"; shift ;;
    # `shift 2` is this parser's house form, but bash's `shift` FAILS rather than truncating when
    # the flag is the last argument, and this loop has no trailing shift — so a value-taking flag
    # in final position spins here forever. For `--observe` that would make its own
    # "--observe requires a dispatch key" refusal unreachable, i.e. decoration. Shift the flag,
    # then the value only if a value is actually there.
    --observe) VERB="observe"; OBSERVE_KEY="${2:-}"; shift; [ $# -gt 0 ] && shift ;;
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
case "$MODEL" in inherit) MODEL="" ;; esac
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

# --- run-gate primitives, shared by all three verbs ---------------------------------
# Defined HERE, above every verb, because the same three reads serve all of them: the synchronous
# gate at the bottom of this file brackets its handoff with them, `--launch` captures the
# ATTRIBUTION inputs with them, and `--observe` re-reads with them. ONE reader — verify-run stays
# the single owner of frontmatter reading for the gate (its own header says so), and a second
# reader growing beside this one is exactly what that ownership exists to prevent.
in_progress_ids(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids 2>/dev/null; }
in_progress_claims(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids --with-claimed-at 2>/dev/null; }
# Best-effort metadata re-sync, used on BOTH sides of every hand-off — the synchronous gate's, and
# the launch/observe pair's. Both snapshots must be reads of FRESH ORIGIN state
# (LEARNINGS: cas-re-read-fresh-origin); an asymmetric pair attributes an ABANDONED claim from an
# earlier session to this run. A failure degrades the gate's freshness; it never fails a dispatch.
resync_metadata(){
  "$DOCKET_BASH_PATH" "$DOCKET_FACADE" preflight >/dev/null 2>&1 \
    || { printf 'runner-dispatch: run gate — metadata re-sync failed; verifying against local state\n' >&2; return 1; }
}

# --- verb: --launch (change 0271) ---------------------------------------------------
# Detach the adapter so the delegated run can OUTLIVE this call, then return at once with the
# dispatch key. The posture and its six required capabilities are cited, never restated:
# skills/docket-build/references/gate-execution.md, plus the adapter-launch verdicts in
# skills/docket-build/references/delegation-execution.md.
#
# This sits AFTER the anchor gates and the runners.<name>: resolution so `--launch` inherits every
# gate and every exported config value the legacy path already gets — a launch is the same
# validated request, differing only in how the adapter is started.
if [ "$VERB" = "launch" ]; then
  DROOT="$(docket_dispatch_root "$ANCHOR")" || die "cannot resolve the dispatch root for $ANCHOR"
  mkdir -p "$DROOT" || die "cannot create the dispatch root $DROOT"
  KEY="$(docket_dispatch_mint "$DROOT" "$AGENT")" || die "cannot mint a dispatch key under $DROOT"
  DDIR="$DROOT/$KEY"
  STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  # The dispatch-time SHA: the direct analogue of DISPATCH_EPOCH, captured BEFORE the child can
  # commit anything, so a commit landing in the gap is excluded either way. Empty on a repo with
  # no commits — the build verdict then reports unknown-since-sha rather than guessing.
  SINCE_SHA="$("$GIT" -C "$ANCHOR" rev-parse HEAD 2>/dev/null || true)"
  # The dispatch-time BRANCH, captured for the same reason and at the same instant: the build
  # verdict's `branch` conjunct asks whether the child ENDED where it was sent, which is only
  # answerable against a value recorded BEFORE the child could move HEAD. Read back at observation
  # time it compares the anchor's HEAD to itself and can never be unmet.
  # A DETACHED anchor records NOTHING rather than the literal `HEAD` that `--abbrev-ref` prints:
  # `HEAD` is not a branch name, and recording it would make the conjunct hold for any other
  # detached state. Absent is the honest answer, and `--observe` reports it as unverifiable.
  SINCE_BRANCH="$("$GIT" -C "$ANCHOR" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  [ "$SINCE_BRANCH" != "HEAD" ] || SINCE_BRANCH=""

  # THE RUN GATE'S ATTRIBUTION INPUTS, captured here because they are only knowable BEFORE the child
  # runs. Change 0237's gate identifies a claimant by diffing the in-progress set across the
  # hand-off; under detachment the two halves of that diff land in two different processes, so the
  # "before" half is recorded durably for `--observe` to read.
  #
  # Scoped to implement-next for exactly the reason the synchronous fence is: a build-* delegation
  # leaves its change in-progress BY DESIGN, and a build task's terminal state is a commit, read on
  # the observe seam by the build verdict family instead.
  #
  # The discipline is the synchronous gate's, unchanged: re-sync FIRST so the before-read is of
  # fresh origin state, then stamp the clock AFTER that read, so a claim landing in the gap is
  # either already in the before-set or stamped before the window and is excluded either way. The
  # observe half re-syncs again, which is what keeps the pair SYMMETRIC — an asymmetric pair
  # attributes an abandoned claim from an earlier session to this run
  # (LEARNINGS: cas-re-read-fresh-origin).
  #
  # Tolerant, like every other gate read: a failed snapshot or clock read leaves the gate UNARMED
  # and the dispatch untouched, never failed. `dispatch_epoch` in the launch record plus the
  # `gate-before` file ARE the arming signal — with either absent, `--observe` falls back to the
  # sentinel-only disposition rather than guessing at an attribution.
  GATE_BEFORE=""; GATE_EPOCH=""; GATE_ARMED=0
  if [ "$AGENT" = "implement-next" ]; then
    resync_metadata || :
    if GATE_BEFORE="$(in_progress_ids)"; then
      GATE_EPOCH="$(date -u +%s 2>/dev/null)"
      case "$GATE_EPOCH" in
        ''|*[!0-9]*) printf 'runner-dispatch: run gate disabled — could not read the clock\n' >&2 ;;
        *) GATE_ARMED=1 ;;
      esac
    else
      printf 'runner-dispatch: run gate disabled — could not read the in-progress set\n' >&2
    fi
  fi

  # DETACHMENT, measured (2026-08-09): `set -m` makes a background job a PROCESS-GROUP LEADER,
  # so it survives the harness's teardown of THIS call's process group — capability 1's stronger
  # reading. Measured with one variable changed between two arms of a single run: the set -m
  # child survived a group-directed TERM, the non-set-m child did not. `setsid` is ABSENT on
  # macOS, which is why job control rather than a new session is the mechanism.
  # Every stream is redirected to the durable dir and stdin is closed, so nothing remains
  # attached to the initiating call (capability 2).
  # THE WRAPPER WRITES THE SENTINEL, NEVER THE AGENT: "done" must not be a claim by the party
  # being judged. The write is atomic — a temp file BESIDE its destination (the one licensed
  # exception to templating temp files into TMPDIR, because the rename must be same-filesystem)
  # then `mv -f`; a reader therefore never sees a half-written sentinel.
  set -m
  {
    "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
    ec=$?
    printf 'exit_code=%s\nstarted_at=%s\nfinished_at=%s\npid=%s\ndispatch_key=%s\n' \
      "$ec" "$STARTED_AT" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$$" "$KEY" > "$DDIR/done.partial"
    mv -f "$DDIR/done.partial" "$DDIR/done"
  } >"$DDIR/stdout.log" 2>"$DDIR/stderr.log" </dev/null &
  CHILD_PID=$!
  set +m

  # RACE PRECONDITION (0223's measured one): the new group must be fully established before this
  # call returns, or the harness's teardown wins the race. Poll briefly and FAIL CLOSED rather
  # than return a key for a child that is still in our group and about to be reaped with us.
  MY_PGID="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  CHILD_PGID=""
  for _ in $(seq 1 50); do
    CHILD_PGID="$(ps -o pgid= -p "$CHILD_PID" 2>/dev/null | tr -d ' ')"
    # The child may already have finished — that is establishment too, and the sentinel proves it.
    [ -f "$DDIR/done" ] && break
    [ -n "$CHILD_PGID" ] && [ "$CHILD_PGID" != "$MY_PGID" ] && break
    sleep 0.1
  done
  if [ -z "$CHILD_PGID" ] && [ ! -f "$DDIR/done" ]; then
    die "launch failed — the detached child never appeared (key $KEY)"
  fi
  if [ -n "$CHILD_PGID" ] && [ "$CHILD_PGID" = "$MY_PGID" ] && [ ! -f "$DDIR/done" ]; then
    kill -TERM "$CHILD_PID" 2>/dev/null
    die "launch failed — the child did not separate into its own process group; refusing to report a dispatch that a teardown would kill (key $KEY)"
  fi

  # THE IDENTITY TOKEN for the observer's kill guard, captured from the same live process the group
  # was just measured on. `pgid` and `child_pid` are reusable names, so the observer needs one value
  # that a RECYCLED pid cannot reproduce: a pid that the OS later hands to an unrelated process
  # leading a group of the same id matches both of those and only differs here. Empty when the child
  # already finished (the `ps` above saw nothing) — the observer then degrades to the pgid check
  # alone and says so, rather than treating an absent token as a match.
  CHILD_LSTART="$(ps_lstart "$CHILD_PID")"

  # `pgid` is what an observer must signal to reach the whole detached tree, so it is recorded from
  # the MEASUREMENT wherever the measurement exists. The fallback fires only when `ps` could not see
  # the process at all — which, given the refusals above, means the child already finished — and it
  # is sound rather than a guess: under `set -m` a background job is created as a process-group
  # LEADER, so its pgid IS its pid by construction. Note the corollary a reader must not miss: with
  # `set -m` removed the fallback would record a pid that is no group's leader, which is exactly why
  # tests/test_runner_dispatch_detach.sh reads the group from the live process rather than from this
  # record ("the child is in its OWN process group, not the test's").
  # The before-snapshot lands BEFORE the record that advertises it, so an observer that sees a
  # readable `dispatch_epoch` can rely on the file being there. It is written even when EMPTY —
  # "nothing was claimed at the handoff" is a real answer, and the arming signal is the record's
  # epoch, never this file's size. `mv -f` for the reason it is used everywhere else here: BSD `mv`
  # onto an unwritable destination with a tty prompts, self-answers `n`, and exits 0.
  if [ "$GATE_ARMED" = 1 ]; then
    printf '%s\n' "$GATE_BEFORE" > "$DDIR/gate-before.partial"
    mv -f "$DDIR/gate-before.partial" "$DDIR/gate-before" || {
      printf 'runner-dispatch: run gate disabled — could not record the before-snapshot in %s\n' "$DDIR" >&2
      GATE_EPOCH=""
    }
  fi
  printf 'pgid=%s\nchild_pid=%s\nchild_lstart=%s\nstarted_at=%s\nagent=%s\nrunner=%s\nworktree=%s\nsince_sha=%s\nbranch=%s\ndispatch_epoch=%s\n' \
    "${CHILD_PGID:-$CHILD_PID}" "$CHILD_PID" "${CHILD_LSTART:-}" "$STARTED_AT" "$AGENT" "$RUNNER" \
    "$ANCHOR" "${SINCE_SHA:-}" "${SINCE_BRANCH:-}" "${GATE_EPOCH:-}" \
    > "$DDIR/launch.partial"
  mv -f "$DDIR/launch.partial" "$DDIR/launch"

  printf '%s\n' "$KEY"
  exit 0
fi

# Read one field from a dispatch record. Deliberately NOT `sed … | sed -n 1p`: a producer piped
# into a consumer that may exit early takes SIGPIPE under `pipefail` (AGENTS.md, "Shell"), so the
# first-line trim is a parameter expansion on a captured value instead.
launch_field(){  # $1 = dispatch dir, $2 = field -> first value, empty when absent or unreadable
  local raw
  raw="$(sed -n "s/^$2=//p" "$1/launch" 2>/dev/null)"
  printf '%s' "${raw%%$'\n'*}"
}

# --- verb: --observe (change 0271) --------------------------------------------------
# ONE short, idempotent look. Never a long foreground call — that ceiling is the defect this
# change removes, so re-introducing it here would defeat the whole design.
#
# LIVENESS comes from the sentinel, and from nothing else; CORRECTNESS comes from git. A sentinel
# claiming success with no matching git evidence is a FAILURE — correctness wins, so the child's
# own exit code is never the last word about the work. Keeping the two sources apart is what lets
# the facade observe a LIVE child without ever reading liveness out of git state.
if [ "$VERB" = "observe" ]; then
  [ -n "$OBSERVE_KEY" ] || die "--observe requires a dispatch key"
  # The key becomes a path component, exactly as `--runner` does above, so it earns the same
  # shape-keyed refusal. `..` is the case that matters: it names a directory that EXISTS, so
  # without this gate a typo is observed as "still running" forever — a verdict manufactured out
  # of nothing rather than read off a dispatch.
  case "$OBSERVE_KEY" in
    *[!A-Za-z0-9._-]*|*..*) die "invalid dispatch key '$OBSERVE_KEY'" ;;
  esac
  DROOT="$(docket_dispatch_root "$ANCHOR")" || die "cannot resolve the dispatch root for $ANCHOR"
  DDIR="$(docket_dispatch_dir "$DROOT" "$OBSERVE_KEY")" \
    || die "unknown dispatch key '$OBSERVE_KEY' (no result directory under $DROOT)"

  # The budget, in minutes. The caller's environment wins — that is how a shim hands one down —
  # and otherwise it is resolved once, HERE: no other verb needs it, and resolving config a verb
  # does not use is how a config failure becomes a dispatch failure.
  if [ -z "${DELEGATION_OBSERVATION_BUDGET:-}" ]; then
    _cfg="$("$DOCKET_BASH_PATH" "$SELF_DIR/docket-config.sh" --export 2>/dev/null)" \
      && eval "$(grep -E '^DELEGATION_OBSERVATION_BUDGET=' <<<"$_cfg")"
    DELEGATION_OBSERVATION_BUDGET="${DELEGATION_OBSERVATION_BUDGET:-60}"
  fi
  # An unparseable budget is NOT a spent one. `[ N -lt abc ]` is a shell ERROR, whose non-zero
  # status is indistinguishable from "budget exceeded" at the `if` below — so a typo in an
  # environment variable would kill a healthy child. Normalize to the shipped default instead;
  # docket-config.sh already refuses a malformed configured value at its own boundary.
  case "$DELEGATION_OBSERVATION_BUDGET" in
    ''|*[!0-9]*) DELEGATION_OBSERVATION_BUDGET=60 ;;
  esac

  LPGID="$(launch_field "$DDIR" pgid)"
  LSTART="$(launch_field "$DDIR" started_at)"

  # THE RELAY. `--launch` redirects the adapter's stdout into `$DDIR/stdout.log`, so this function
  # is the ONLY channel by which a delegated agent's result reaches its caller: the generated shim
  # is told to "Relay that observe call's stdout as your result" (sync-agents.sh's emit_shim), an
  # instruction satisfiable only if `--observe` puts that captured stdout on ITS OWN stdout, so the
  # two are one contract and move together. The adapters call the child's stdout the relay
  # (codex.sh `cat`s the last message; opencode.sh and cursor.sh pass the formatted stdout through),
  # so it is emitted VERBATIM — never summarized, prefixed, or reformatted; a caller parses it.
  # Every diagnostic in this branch stays on stderr, which is what keeps the two from interleaving.
  #
  # Fired ONLY where the `done` sentinel exists — the child is finished, so its stdout is COMPLETE.
  # The still-running (4) path deliberately emits nothing: the shim observes repeatedly, and a
  # partial relay per pass would hand the caller the same prefix over and over. The budget-kill and
  # own-group-refusal paths emit nothing either — there the run has no result to relay (and in the
  # latter case it is still live and still writing).
  relay_child_stdout(){
    if [ -s "$DDIR/stdout.log" ]; then cat "$DDIR/stdout.log"; fi
    return 0
  }

  # THE RUN GATE ON THE OBSERVE SEAM — change 0237's disposition, reached through detachment.
  # The generated shim always launches, so the synchronous fence at the bottom of this file is
  # unreachable for every delegated implement-next run; without this leg such a run gets the
  # SENTINEL-ONLY disposition, and one that HALTED or stopped before its PR exits 0 at the adapter
  # and observes as "complete (child exited 0)". That is the prose-level failure change 0237 was
  # built to eliminate, so the disposition follows the run onto whichever seam it actually returns
  # through.
  #
  # It mirrors the synchronous gate's ATTRIBUTION exactly — the same symmetric fresh-origin
  # re-sync, the same three filters, the same more-than-one-candidate stand-down, the same posture
  # of acting only on a POSITIVE finding — and differs in exactly one respect, deliberately:
  #
  #   NO AUTO RE-DISPATCH. The synchronous gate's ONE bounded re-dispatch is not recreated here.
  #   Re-launching a detached child out of an OBSERVATION is a different lifecycle: an observation
  #   is a short, idempotent read that a shim makes repeatedly, so a re-dispatch on this path would
  #   race the very run being observed and could mint a fresh detached child per pass. A
  #   `run-incomplete` therefore reports 1 and the caller acts. This is a decision, not an omission
  #   (scripts/runner-dispatch.md records it as one).
  #
  # The verdict outranks the child's own exit code in BOTH directions, which is why this runs
  # before the exit_code split below rather than inside its zero leg: a halt is terminal whatever
  # the adapter returned, and a positively green verdict describes a run that reached its PR
  # despite a noisy adapter. Same rule as the disagreement rule — correctness outranks the
  # self-report of the party being judged.
  #
  # EXITS on a positive finding; RETURNS (falling through to the sentinel-only disposition) on
  # anything it could not establish.
  observe_implement_next(){
    local before after epoch verdict nid nclaimed
    local new_ids=()
    epoch="$(launch_field "$DDIR" dispatch_epoch)"
    # No epoch = the gate was never armed at launch (an older dispatch, a non-implement-next one,
    # or a snapshot/clock read that failed there and already said so). Silent: the launch side
    # announced it, and a second warning per observation pass would be noise in a polling loop.
    case "$epoch" in ''|*[!0-9]*) return 0 ;; esac
    # The before-snapshot and the epoch are written TOGETHER at launch, so a readable epoch with no
    # snapshot file is a half-written record: unarmed, never a before-set guessed as empty.
    [ -f "$DDIR/gate-before" ] || {
      printf 'runner-dispatch: observe %s — run gate disabled: the launch recorded no before-snapshot\n' "$OBSERVE_KEY" >&2
      return 0
    }
    before="$(cat "$DDIR/gate-before" 2>/dev/null)"
    # The AFTER read must come from FRESH ORIGIN state, symmetric with the launch-side read.
    resync_metadata || :
    after="$(in_progress_claims)" || {
      printf 'runner-dispatch: observe %s — run gate disabled: could not re-read the in-progress set\n' "$OBSERVE_KEY" >&2
      return 0
    }
    while IFS=' ' read -r nid nclaimed; do
      [ -n "$nid" ] || continue
      grep -qxF "$nid" <<<"$before" && continue
      case "${nclaimed:-}" in
        ''|*[!0-9]*)
          printf 'runner-dispatch: observe %s — run gate: ignoring change %s: no readable claimed_at\n' "$OBSERVE_KEY" "$nid" >&2
          continue ;;
      esac
      [ "$nclaimed" -ge "$epoch" ] || {
        printf 'runner-dispatch: observe %s — run gate: ignoring change %s: claimed before this dispatch started\n' "$OBSERVE_KEY" "$nid" >&2
        continue
      }
      new_ids+=("$nid")
    done <<<"$after"

    if [ "${#new_ids[@]}" -gt 1 ]; then
      printf 'runner-dispatch: observe %s — run gate disabled: %s changes were claimed during this dispatch (%s); this run claims at most one, so none can be attributed to it\n' \
        "$OBSERVE_KEY" "${#new_ids[@]}" "${new_ids[*]}" >&2
      return 0
    fi
    # No candidate at all: drained, a lost claim race, or a run that finished and left its change
    # `implemented` (which is not in-progress and so never appears in the after-set). All three are
    # no-ops here, exactly as the empty diff is on the synchronous path.
    [ "${#new_ids[@]}" -eq 1 ] || return 0
    nid="${new_ids[0]}"
    verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
    printf 'runner-dispatch: observe %s — run gate: %s\n' "$OBSERVE_KEY" "${verdict:-run-unverifiable $nid}" >&2
    case "$verdict" in
      run-halted*)
        # STOP + SURFACE with its own code, the same 3 the synchronous gate returns and the code
        # the design spec's synthesized-exit table pins normatively under detachment. Never 0: a
        # driver told 0 draws the next change, on a disposition that means a human is needed.
        printf 'runner-dispatch: observe %s — RUN HALTED (%s); the delegated implement-next run stopped and needs a human — read the change file'"'"'s "## Run halted" section\n' \
          "$OBSERVE_KEY" "$verdict" >&2
        relay_child_stdout
        exit 3 ;;
      run-incomplete*)
        printf 'runner-dispatch: observe %s — FAILED (%s); the delegated implement-next run did not reach its PR. No re-dispatch is made from an observation — the change stays in-progress with its claim intact, and board-checks'"'"' aborted-run leg remains the backstop\n' \
          "$OBSERVE_KEY" "$verdict" >&2
        relay_child_stdout
        exit 1 ;;
      run-complete*|run-unclaimed*)
        printf 'runner-dispatch: observe %s — complete (%s)\n' "$OBSERVE_KEY" "$verdict" >&2
        relay_child_stdout
        exit 0 ;;
    esac
    # Empty or unparseable: no positive finding, so the sentinel-only disposition stands.
    return 0
  }

  # 1. A prior budget kill is TERMINAL and re-reports identically forever (idempotence). The
  #    marker's `reason` distinguishes the two ways that state is reached — a group actually
  #    terminated, or one that could not be confirmed as the launched child's and so was left
  #    alone (see the identity check below). Both are unavailable and both exit 1; reporting them
  #    with one wording would tell a reader a kill happened when none did.
  if [ -f "$DDIR/killed" ]; then
    KREASON="$(sed -n 's/^reason=//p' "$DDIR/killed" 2>/dev/null)"
    case "${KREASON%%$'\n'*}" in
      group-already-gone)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the budget was exhausted and the recorded process group could not be confirmed as the launched child'"'"'s, so nothing was signalled)\n' "$OBSERVE_KEY" >&2 ;;
      *)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the detached run was killed at budget exhaustion)\n' "$OBSERVE_KEY" >&2 ;;
    esac
    exit 1
  fi

  # 2. A sentinel means the child is DONE. Well-formed vs malformed is the difference between a
  #    clean adapter exit and a wrapper crash — the latter is `unavailable`, NEVER an exit code
  #    read out of garbage.
  if [ -f "$DDIR/done" ]; then
    SEC="$(docket_sentinel_field "$DDIR" exit_code)"
    case "$SEC" in
      ''|*[!0-9]*)
        printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the sentinel is malformed; the launcher did not finish cleanly)\n' "$OBSERVE_KEY" >&2
        # The child is finished either way, so whatever it managed to say is the only evidence left;
        # relaying it costs nothing and is the difference between a diagnosable failure and a silent
        # one. The verdict still comes from the exit code, never from the relayed text.
        relay_child_stdout
        exit 1 ;;
    esac
    # The child is finished and its sentinel parses, which is the precondition for every git-read
    # disposition on this seam. implement-next's runs FIRST and for both exit-code classes — see
    # observe_implement_next's header for why the verdict outranks the child's own code in both
    # directions. It exits on a positive finding and returns otherwise, leaving the sentinel-only
    # disposition below in charge.
    case "$AGENT" in
      implement-next) observe_implement_next ;;
    esac
    if [ "$SEC" = "0" ]; then
      # LIVENESS said done; now CORRECTNESS decides. A sentinel claiming success with no matching
      # git evidence is a FAILURE — the delegated run is the party being judged, so its own exit
      # code can never be the last word. Change 0258's stranded +64 lines exited 0 at the adapter.
      GITV=""
      case "$AGENT" in
        build-*)
          LSINCE="$(launch_field "$DDIR" since_sha)"
          # THE BRANCH COMES FROM THE LAUNCH RECORD, never from the anchor now. Re-reading it here
          # compares the anchor's HEAD to itself, which made the verdict's `branch` conjunct
          # structurally vacuous on its only consumer: a child that ended on the WRONG branch or on
          # a DETACHED HEAD (a bad rebase, a stray `git checkout`) still satisfied `tip` and `tree`
          # and was reported `task-committed`.
          LBRANCH="$(launch_field "$DDIR" branch)"
          if [ -z "$LBRANCH" ]; then
            # An older dispatch, or a detached anchor at launch. Falling back to the observation-time
            # branch would reinstate exactly the vacuity above, so this is NO POSITIVE EVIDENCE and
            # is surfaced as such — the same posture the empty-verdict case below already takes.
            GITV="task-unverifiable launch-branch-missing"
          else
            GITV="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --build --worktree "$ANCHOR" \
                      --branch "$LBRANCH" --since "${LSINCE:-}" 2>/dev/null)"
          fi
          case "$GITV" in
            task-committed*) : ;;
            # OBSERVE-ONLY for build-*: never a re-dispatch. A build task may have left partial
            # commits, and re-running on top of them is docket-build's "never escalate onto a
            # stray commit" hazard. Report and stop. An empty verdict lands here too — a check
            # that could not run is not evidence of success.
            *) printf 'runner-dispatch: observe %s — FAILED (the child exited 0 but git disagrees: %s); work left in %s for inspection\n' \
                 "$OBSERVE_KEY" "${GITV:-no-verdict}" "$ANCHOR" >&2
               relay_child_stdout
               exit 1 ;;
          esac ;;
      esac
      printf 'runner-dispatch: observe %s — complete (child exited 0%s)\n' \
        "$OBSERVE_KEY" "${GITV:+; $GITV}" >&2
      relay_child_stdout
      exit 0
    fi
    printf 'runner-dispatch: observe %s — FAILED (child exited %s); see %s/stderr.log\n' \
      "$OBSERVE_KEY" "$SEC" "$DDIR" >&2
    relay_child_stdout
    exit 1
  fi

  # 3. No sentinel: still running, unless the budget is spent. `0` is legal and buys exactly ONE
  #    observation — this one — so the comparison is `>=`, evaluated AFTER the sentinel read
  #    above, which is what makes that single observation a real one.
  NOW="$(date -u +%s 2>/dev/null)"
  START_EPOCH=""
  [ -n "$LSTART" ] && START_EPOCH="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" --iso-to-epoch "$LSTART" 2>/dev/null)"
  # Neither clock read is positive evidence that the budget is spent, so an unreadable one keeps
  # observing rather than killing a healthy child on a guess. Same posture, twice.
  case "${NOW:-}" in
    ''|*[!0-9]*)
      printf 'runner-dispatch: observe %s — still running (the clock could not be read; budget not enforced this pass)\n' "$OBSERVE_KEY" >&2
      exit 4 ;;
  esac
  case "${START_EPOCH:-}" in
    ''|*[!0-9]*)
      printf 'runner-dispatch: observe %s — still running (start time unreadable; budget not enforced this pass)\n' "$OBSERVE_KEY" >&2
      exit 4 ;;
  esac
  ELAPSED_MIN=$(( (NOW - START_EPOCH) / 60 ))
  if [ "$ELAPSED_MIN" -lt "$DELEGATION_OBSERVATION_BUDGET" ]; then
    printf 'runner-dispatch: observe %s — still running (%sm of %sm budget)\n' \
      "$OBSERVE_KEY" "$ELAPSED_MIN" "$DELEGATION_OBSERVATION_BUDGET" >&2
    exit 4
  fi

  # 4. Budget exhausted: KILL THE WHOLE GROUP, never a single pid. A single-PID kill reaps the
  #    launcher shell and ORPHANS the adapter and its children — precisely the half-dead state
  #    this change exists to eliminate. Honors change 0231: no presumed-dead worker wakes to race
  #    its replacement. Partial work stays in the worktree for a human.
  #
  #    NEVER our own group, though. `--launch` fails closed when the child did not separate, so a
  #    record it wrote can only name a foreign group — but a record can be wrong anyway (hand
  #    edited, or a pgid reused after that group died), and a group-directed signal aimed at the
  #    observer's own group takes down the harness that ran it. That is the one failure this
  #    facade must not cause, so the impossible state is refused loudly instead of signalled.
  MY_PGID="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  if [ -n "$LPGID" ] && [ -n "$MY_PGID" ] && [ "$LPGID" = "$MY_PGID" ]; then
    printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (the launch record names THIS observation'"'"'s own process group (%s); refusing to signal it, so the dispatch was left running — inspect %s)\n' \
      "$OBSERVE_KEY" "$LPGID" "$DDIR" >&2
    exit 1
  fi
  #    IDENTITY BEFORE SIGNAL, and the own-group refusal above is not it. A pgid is a REUSABLE
  #    NAME, and this path is reached ONLY when no sentinel exists — which includes the child that
  #    was killed externally, or died without the wrapper writing `done`, an hour before this
  #    observation. By then the OS may have handed that id to an unrelated tree, and `kill -TERM
  #    -<pgid>` reaches every process in it. Refusing to signal our OWN group defends the harness;
  #    it defends nobody else. So the group is signalled only when it is provably STILL THE
  #    LAUNCHED ONE, in two conjuncts against the launch record:
  #      1. the recorded `child_pid` must STILL LEAD the recorded group. The child is the group
  #         leader by construction (`set -m` creates a background job as one), so any other answer
  #         — the pid is gone, or it now sits in a different group — means the launched group is no
  #         longer reachable under that name.
  #      2. that pid's START TIME must still be the token measured at launch, because conjunct 1
  #         alone is satisfied by a RECYCLED pid that coincidentally leads a group of the same id
  #         (pid reuse is what makes the whole hazard reachable in the first place, and a recycled
  #         pid that calls setpgid(0,0) is an ordinary background job, not an exotic state).
  #         Skipped only when the launch recorded no token, which it does only for a child that had
  #         already finished.
  #    Failing either conjunct is NOT an error — it is the ordinary "that group is already gone"
  #    outcome. The kill is skipped, the terminal `killed` marker is STILL recorded (with a reason
  #    that says nothing was signalled, so the dispatch stays terminal and re-observes identically),
  #    and the verdict is RESULT UNAVAILABLE either way: the run outran its budget with no sentinel,
  #    so there is no result to report whether or not a signal was sent.
  #    THE RESIDUAL THIS ACCEPTS, deliberately: a group whose LEADER died while processes it spawned
  #    keep running is not signalled, so those orphans outlive the budget. Killing them would mean
  #    signalling a name we cannot prove is still theirs — an unrelated process group dying is the
  #    worse of the two failures, and it is unrecoverable, while an orphan is visible and reapable.
  #    An unreadable `ps` lands on the same side for the same reason.
  KILL_REASON="budget-exhausted"
  SIGNAL_GROUP=0
  IDENTITY_WHY=""
  LCHILD="$(launch_field "$DDIR" child_pid)"
  LCHILD_LSTART="$(launch_field "$DDIR" child_lstart)"
  NOW_PGID=""
  NOW_LSTART=""
  if [ -z "$LPGID" ]; then
    IDENTITY_WHY="the launch record names no process group"
  else
    case "$LCHILD" in
      ''|*[!0-9]*) IDENTITY_WHY="the launch record names no usable child pid" ;;
      *)
        NOW_PGID="$(ps -o pgid= -p "$LCHILD" 2>/dev/null | tr -d ' ')"
        NOW_LSTART="$(ps_lstart "$LCHILD")"
        if [ -z "$NOW_PGID" ]; then
          IDENTITY_WHY="the launched child (pid $LCHILD) is gone, so the group is no longer provably its own"
        elif [ "$NOW_PGID" != "$LPGID" ]; then
          IDENTITY_WHY="pid $LCHILD now leads group $NOW_PGID, not the recorded $LPGID"
        elif [ -n "$LCHILD_LSTART" ] && [ "$NOW_LSTART" != "$LCHILD_LSTART" ]; then
          IDENTITY_WHY="pid $LCHILD started at '$NOW_LSTART', not at the launch's '$LCHILD_LSTART' — the pid was recycled"
        else
          SIGNAL_GROUP=1
        fi ;;
    esac
  fi

  if [ "$SIGNAL_GROUP" = 1 ]; then
    kill -TERM -"$LPGID" 2>/dev/null
    for _ in $(seq 1 20); do kill -0 -"$LPGID" 2>/dev/null || break; sleep 0.5; done
    kill -KILL -"$LPGID" 2>/dev/null
  else
    KILL_REASON="group-already-gone"
    printf 'runner-dispatch: observe %s — NOT signalling process group %s: %s. A pgid is a reusable name, so an unconfirmed one may now belong to an unrelated process group; recording the dispatch as terminal without a kill\n' \
      "$OBSERVE_KEY" "${LPGID:-<none>}" "$IDENTITY_WHY" >&2
  fi
  # `mv -f`, not `mv`: BSD `mv` onto an unwritable destination with a tty PROMPTS, self-answers
  # `n` at EOF, and exits 0 — the marker would be silently lost and the kill would re-fire on the
  # next observation.
  printf 'killed_at=%s\nreason=%s\nbudget_minutes=%s\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$KILL_REASON" "$DELEGATION_OBSERVATION_BUDGET" > "$DDIR/killed.partial"
  mv -f "$DDIR/killed.partial" "$DDIR/killed" || die "could not record the kill marker in $DDIR"
  if [ "$SIGNAL_GROUP" = 1 ]; then
    printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (budget of %sm exhausted; the detached process group was terminated)\n' \
      "$OBSERVE_KEY" "$DELEGATION_OBSERVATION_BUDGET" >&2
  else
    printf 'runner-dispatch: observe %s — RESULT UNAVAILABLE (budget of %sm exhausted; the recorded process group could not be confirmed as the launched child'"'"'s, so nothing was signalled — inspect %s)\n' \
      "$OBSERVE_KEY" "$DELEGATION_OBSERVATION_BUDGET" "$DDIR" >&2
  fi
  exit 1
fi

# --- run gate (change 0237), part 1: the "before" snapshot --------------------------
# Engages ONLY for an implement-next delegation. That scoping is load-bearing, not an
# optimization: a build-* delegation leaves its change `in-progress` BY DESIGN (the build role
# does not reach Step 7), so gating one would fire on every healthy build. status / adr /
# review-* / finalize-change / auto-groom are likewise out of scope, and an unrecognised agent is
# a no-op — never a guess.
#
# This SYNCHRONOUS path's gate stays scoped to implement-next. The build disposition is NOT bolted
# on here: it lives on the `--observe` seam (change 0271), where a detached child's terminal state
# is actually knowable and where the subject is a COMMIT on a feature branch rather than a change's
# metadata status. The two never overlap, which is why this fence never grew a build leg.
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
#
# THIS FENCE IS FOR THE SYNCHRONOUS PATH ONLY, and it is still reachable: a hand invocation with no
# `--launch` still blocks here. The DELEGATED path no longer passes through it at all — the
# generated shim always launches — so the same disposition is carried on the `--observe` seam by
# `observe_implement_next`, which mirrors this attribution exactly.
#
# `in_progress_ids`, `in_progress_claims` and `resync_metadata` are defined above the verbs so both
# halves share one reader.
GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1

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
