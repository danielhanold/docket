#!/usr/bin/env bash
# scripts/lib/docket-liveness.sh — process-group liveness, IDENTITY-CHECKED (change 0284).
# Sourced by scripts/gate-run.sh and scripts/runner-dispatch.sh; never executed directly.
#
# ONE PREDICATE, TWO CONSUMERS. Before this lib each script carried its own copy: gate-run.sh's
# `group_alive_and_ours` and runner-dispatch.sh's inline ladder inside `terminate_dispatch`. Two
# copies of a predicate that must AGREE is the drift the `duplicated-gate-copies-the-whole-predicate`
# learning describes, and the copies had already diverged on one leg (an empty recorded token: the
# gate failed closed, the dispatch SKIPPED the conjunct). This file is the single definition.
#
# IT TAKES VALUES, NEVER RUN DIRS. The two consumers store their records in incompatible layouts —
# gate-run.sh keys `pid`/`pgid`/`identity` across `$rd/launch` plus a separate `$rd/identity` file,
# runner-dispatch.sh keys `pgid`/`child_pid`/`child_lstart` in `$DDIR/launch`. Each keeps its own
# reader and passes the extracted values in. A layout-aware lib would have to know both, which is
# exactly the coupling change 0282's assumption 1 rejected.
#
# WHY A PGID IS NOT ENOUGH: a pgid is a REUSABLE NAME. An hour after a child died the OS may have
# handed that id to an unrelated tree, and `kill -0 -<pgid>` would answer for the stranger. So the
# conjunction is: the group exists AND the process leading it started at the instant the caller
# recorded. Identity is the process START TIME, compared as an EXACT STRING and never parsed into a
# date — the `ps -o lstart=` rendering is platform- and locale-dependent, and both sides of every
# comparison come from the same `ps` on the same machine, so parsing it would add a failure mode
# and buy nothing.
#
# EVERY LEG FAILS CLOSED — an absent pgid, a non-numeric one, `0` or `1`, an empty recorded token,
# an empty live token. The asymmetry is the whole justification: a false `dead` costs one wasted
# observation, while a false `alive` costs the caller its ENTIRE budget on a run that is not there.
#
# BUT "NOT ALIVE" IS NOT ONE FACT, and the second consumer is why (change 0284 review, finding 1).
# In gate-run.sh a false `dead` costs one bounded relaunch; on runner-dispatch.sh's `--observe` seam
# it writes a TERMINAL marker and ends the caller's polling loop — and because git decides that
# leg's exit code, it can return `0` ("the work landed") for a child that is STILL RUNNING. Only
# `kill -0 -<pgid>` failing is POSITIVE EVIDENCE of death; an unreadable `ps`, an empty token on
# either side and a mismatch say only that the question could not be answered this pass. So every
# non-zero return also carries a CLASS, and a consumer that cannot walk a wrong answer back routes
# on it. `DOCKET_LIVENESS_WHY` says what happened; `DOCKET_LIVENESS_CLASS` says whether it is
# evidence. Both are additive: a consumer reading only the exit status is unaffected.

# The normalized start-time token for a pid; empty when the pid is gone or `ps` cannot be read.
# ALWAYS RETURNS 0: an absent pid is an empty token, not an error, so a caller under `set -e` can
# assign from it (gate-run.sh does, at `SPAWN_IDENT=`).
#
# THE RENDERING IS PINNED, not merely whitespace-normalized. `ps -o lstart=` formats through the
# CALLER's environment — `TZ` moves the clock and `LC_TIME` moves the weekday and month names — so
# an unpinned reader hands two processes asking about the SAME pid two different tokens, and every
# comparison downstream reads that difference as "not the recorded process". A `--launch` under one
# environment and an `--observe` under another would make a healthy long-running child unprovable,
# which on the observe seam is a terminal verdict. `TZ` is UNSET rather than forced to `UTC`: the
# goal is that the caller's environment cannot move the rendering, and unsetting it lands on the
# machine's own zone — which is what an ambient reader on the same machine already produces, so a
# token recorded by an earlier build still compares equal. Forcing `UTC` would instead re-render
# every one of them (and redden gate-run.sh's own recorded-vs-ambient assert on any machine not
# already in UTC). `localtime()` resolves an absolute start time under the zone rules in force at
# THAT instant, so a DST transition does not move an already-recorded token either.
docket_identity_of(){  # $1 = pid -> normalized `ps -o lstart=` token, or empty
  local s
  s="$(unset TZ; LC_ALL=C ps -o lstart= -p "${1:-0}" 2>/dev/null || true)"
  s="$(tr -s '[:space:]' ' ' <<<"$s")"
  s="${s# }"
  printf '%s' "${s% }"
}

# The caller-printable reason for the most recent non-zero `docket_group_alive_and_ours`.
#
# THIS VARIABLE IS WHAT MAKES THIS ONE PREDICATE RATHER THAN TWO. runner-dispatch.sh's
# `terminate_dispatch` needs the REASON its identity check failed, for its "NOT signalling process
# group …" diagnostic; the new --observe leg needs only the boolean. A lib returning only a boolean
# would have forced `terminate_dispatch` to keep a private reason-producing copy — a second
# predicate wearing the first one's answer, which is the drift this file exists to prevent.
DOCKET_LIVENESS_WHY=""

# The CLASS of that same non-zero return — `gone` or `unprovable`, and nothing else.
#
#   gone       — `kill -0 -<pgid>` was ISSUED and FAILED. No process in that group answers: not the
#                leader, and not anything it spawned, because group membership survives its leader.
#                This is the only leg that is evidence, and the only one a consumer may dispose on.
#   unprovable — the question could not be answered: a record naming no usable group (nothing is
#                probed at all), a token missing on either side, or two tokens that disagree. A
#                disagreement is NOT the third kind of evidence it looks like: the group is
#                demonstrably alive on that leg, and a token rendered by an older, unpinned build of
#                `docket_identity_of` differs for a reason that has nothing to do with the process.
#
# A caller for whom a false `dead` is cheap (gate-run.sh: one bounded relaunch) may ignore this
# entirely and read the exit status alone — which is what keeps the field additive.
DOCKET_LIVENESS_CLASS=""

# 0 when the group exists AND is still the one the caller recorded; non-zero otherwise.
# Sets DOCKET_LIVENESS_WHY and DOCKET_LIVENESS_CLASS on every non-zero return, and clears both on 0.
docket_group_alive_and_ours(){  # $1 = pgid, $2 = expected identity token
  local pgid="${1:-}" want="${2:-}" have
  DOCKET_LIVENESS_WHY=""
  DOCKET_LIVENESS_CLASS=""

  # 1. THE SYNTACTIC FLOOR, and it comes FIRST so that a record naming nothing probes nothing.
  #    `kill … -0` means THIS caller's own process group and `kill … -1` means every process the
  #    user can signal — as a probe each answers for a bystander, and at the signalling call sites
  #    downstream each would take the caller (or the machine) down with it. Neither may ever stand
  #    in for a recorded run's group, so both are refused here rather than at each call site.
  #    NOTHING IS PROBED HERE, so nothing is learned: every leg of this floor is `unprovable`.
  case "$pgid" in
    ''|*[!0-9]*)
      DOCKET_LIVENESS_WHY="the record names no usable process group (got '${pgid}')"
      DOCKET_LIVENESS_CLASS="unprovable"
      return 1 ;;
  esac
  if [ "$pgid" -le 1 ]; then
    DOCKET_LIVENESS_WHY="process group '$pgid' is not a recorded run's group ('0' means this process's own group and '1' means every process this user can signal)"
    DOCKET_LIVENESS_CLASS="unprovable"
    return 1
  fi

  # 2. LIVENESS. Cheap, and answers for whoever holds the name NOW — which is why it is never the
  #    last word. THE ONE LEG THAT IS EVIDENCE: this failing means no process in that group answers.
  kill -0 -"$pgid" 2>/dev/null || {
    DOCKET_LIVENESS_WHY="process group $pgid is gone"
    DOCKET_LIVENESS_CLASS="gone"
    return 1
  }

  # 3. IDENTITY. Fails CLOSED on either token being empty: nothing to compare is not agreement.
  #    EVERY LEG BELOW IS `unprovable`, INCLUDING THE MISMATCH. The group is alive on all three —
  #    what failed is the comparison, and a comparison can fail for reasons that are not about the
  #    process at all (an unreadable `ps`, a record written before the rendering was pinned). The
  #    signalling call sites already treat that as "do not touch it"; a consumer that DISPOSES must
  #    treat it as "not established", never as a death.
  [ -n "$want" ] || {
    DOCKET_LIVENESS_WHY="the record carries no identity token for group $pgid, so the live group cannot be proven to be its own"
    DOCKET_LIVENESS_CLASS="unprovable"
    return 1
  }
  have="$(docket_identity_of "$pgid")"
  [ -n "$have" ] || {
    DOCKET_LIVENESS_WHY="group $pgid has no readable start time, so it cannot be proven to be the recorded one"
    DOCKET_LIVENESS_CLASS="unprovable"
    return 1
  }
  [ "$have" = "$want" ] || {
    DOCKET_LIVENESS_WHY="group $pgid started at '$have', not at the recorded '$want' — the id was recycled or the token was rendered differently"
    DOCKET_LIVENESS_CLASS="unprovable"
    return 1
  }
  return 0
}
