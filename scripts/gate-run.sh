#!/usr/bin/env bash
# scripts/gate-run.sh — detached launch / liveness-keyed observation / identity-checked stop
# for one long-running child process. Contract: scripts/gate-run.md
#
# ORDERING RULE (spec assumption 14, load-bearing): the user's command is NEVER exec'd before
# the pid/pgid/identity record is durably in the run dir. The wrapper records ITSELF after
# detaching, then forks the command. A wedge before the record can strand plumbing at worst,
# never an unaddressable command process.
#
# THE TWO RECORD WRITES ARE ORDERED, AND THE ORDER IS THE POINT. `launch` lands FIRST because it
# is the only thing that makes the detached group ADDRESSABLE: from the instant it exists, the
# launcher's failure path can name the group and kill it. `identity` lands SECOND and is what
# declares establishment COMPLETE — it is the handshake's last conjunct, so a run that got its
# address but never finished establishing is reported `launch-failed` and stopped, never handed
# back as a live handle. Both writes still precede the fork, so the ordering rule above holds.
#
# STDOUT IS THE PROTOCOL: exactly one machine-readable line per verb. Every diagnostic goes to
# stderr. `--launch` prints the absolute run-directory path on success and the single slash-free
# token `launch-failed` on failure — one failure shape, never a taxonomy.
set -euo pipefail

SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/$(basename "${BASH_SOURCE[0]}")"
# The runtime the wrapper is re-invoked under. `$BASH` is the absolute path of the interpreter
# already running this file, so the wrapper inherits the same Bash 4+ the caller resolved.
BASH_BIN="${DOCKET_BASH_PATH:-${BASH:-bash}}"

die() { printf 'gate-run: %s\n' "$*" >&2; }   # diagnostics: stderr, always
report() { printf '%s\n' "$1"; }              # the ONE stdout protocol line

LAUNCH_ESTABLISH_SECS="${GATE_RUN_ESTABLISH_SECS:-10}"
case "$LAUNCH_ESTABLISH_SECS" in ''|*[!0-9]*|0) LAUNCH_ESTABLISH_SECS=10 ;; esac

# --- identity ---------------------------------------------------------------------
# The runner-dispatch.sh `ps_lstart` shape: process start time as identity. A recycled pgid has a
# different start time, so it can never impersonate this run. Whitespace-normalized and compared as
# an EXACT STRING, never parsed into a date — the `ps -o lstart=` rendering is platform- and
# locale-dependent, and both sides of every comparison come from the same `ps` on the same machine.
# Always returns 0: an absent pid is an empty token, not an error, so a caller under `set -e` can
# assign from it.
identity_of() {  # $1 = pid -> normalized start-time token, empty when the pid is gone
  local s
  s="$(ps -o lstart= -p "${1:-0}" 2>/dev/null || true)"
  s="$(tr -s '[:space:]' ' ' <<<"$s")"
  s="${s# }"
  printf '%s' "${s% }"
}

# Read one field from a KEY=value record. Deliberately NOT `sed … | head -n1`: a producer piped into
# a consumer that may exit early takes SIGPIPE under `pipefail` (AGENTS.md, "Shell"), so the
# first-line trim is a parameter expansion on a captured value instead.
record_field() {  # $1 = file, $2 = key -> first value, empty when absent or unreadable
  local raw
  raw="$(sed -n "s/^$2=//p" "$1" 2>/dev/null || true)"
  printf '%s' "${raw%%$'\n'*}"
}

atomic_write() {  # atomic_write <dest> <content>   — temp BESIDE dest, then mv -f
  local dest="$1" content="$2" tmp
  tmp="$(mktemp "${dest}.XXXXXX")"
  printf '%s\n' "$content" >"$tmp"
  mv -f "$tmp" "$dest"
}

# Test-only crash-window injector. ENV-GATED AND INERT BY DEFAULT: unset means a no-op at full
# speed, so this hook can never itself become a hang site in production. Bounded even when armed —
# a fixture that forgot to release must fail the run, not wedge the machine.
wedge() {  # $1 = the point this call site names
  [ "${GATE_RUN_TEST_WEDGE:-}" = "$1" ] || return 0
  local waited=0
  while [ "$waited" -lt 600 ]; do sleep 0.1; waited=$(( waited + 1 )); done
}

# Test-only synchronization point, and a DIFFERENT SHAPE from `wedge` above — the two are siblings
# on purpose, not an accident of naming, and a third hook should not be added without reading why:
#
#   * `wedge` is a ONE-WAY STALL with no counterpart. Its fixtures never release it; they arm it and
#     then let the launcher's own establishment timeout abandon the run, because what they are
#     testing is a CRASH WINDOW — what the world looks like when this process never comes back.
#   * `barrier` is a TWO-WAY RENDEZVOUS. It announces its arrival (`<file>.reached`) so the fixture
#     knows the process is held at exactly this point and nowhere else, then waits to be let go
#     (`<file>.release`). That handshake is what makes an INTERLEAVING deterministic instead of a
#     sleep-tuned guess, and a wedge cannot express it: without the arrival signal the fixture has
#     to guess when the held process got there.
#
# ENV-GATED AND INERT BY DEFAULT, and it is the POINT variable that arms it: with
# `GATE_RUN_TEST_BARRIER` unset this is a no-op at full speed no matter what else is in the
# environment, so the hook can never itself become a hang site in production. The match is on the
# point NAME, so arming one rendezvous cannot silently hold every other call site as well.
#
# BOUNDED even when armed. A fixture that forgets to release must fail its own bounded wait and
# leave a red assert behind, never hang the suite: the ceiling here is deliberately longer than the
# fixture-side wait in `tests/lib/gate_run_common.sh`'s `wait_for_file`, so the timeout that fires
# first — and therefore the diagnostic the author reads — is the fixture's, not this one's.
barrier() {  # $1 = the point this call site names
  [ "${GATE_RUN_TEST_BARRIER:-}" = "$1" ] || return 0
  local f="${GATE_RUN_TEST_BARRIER_FILE:?barrier point '$1' armed without GATE_RUN_TEST_BARRIER_FILE}"
  : >"$f.reached"
  local waited=0
  while [ ! -e "$f.release" ] && [ "$waited" -lt 300 ]; do
    sleep 0.1; waited=$(( waited + 1 ))
  done
  return 0
}

# --- session primitive ladder (resolved at plan time; probed at runtime, never by uname) ---
# Rung 1: setsid(1) where present -> a genuine new SESSION.
# Rung 2 (script(1)) was REJECTED at plan time: injected typescript framing + CRLF, and a pty
#         merges stdout/stderr. See scripts/gate-run.md § Per-platform capability note.
# Rung 3: own PROCESS GROUP via Bash job control (`set -m`) + the detachment handshake.
#         The contract is honestly narrowed on such platforms; it never claims "new session".
detach_mode() { command -v setsid >/dev/null 2>&1 && echo session || echo group; }

my_pgid() { local p; p="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ' || true)"; printf '%s' "$p"; }

numeric_or_empty() { case "${1:-}" in ''|*[!0-9]*) printf '' ;; *) printf '%s' "$1" ;; esac; }

# ==================================================================================
# THE WRAPPER — runs detached, in its own process group, with its streams already
# redirected into the run dir by the launcher. Records ITSELF, then forks the command.
# ==================================================================================
do_wrap() {
  local rd="${1:-}"; shift || true
  [ -n "$rd" ] && [ -d "$rd" ] || { die "wrap: missing or unreadable run dir"; exit 2; }
  [ "${1:-}" = "--" ] || { die "wrap: expected -- before the command"; exit 2; }
  shift

  local pid pgid ident cmd_line created rec
  pid="$BASHPID"
  pgid="$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"

  # HARD PRECONDITION, and it is a SIGNAL-SAFETY one, not a tidiness one. Every later verb signals
  # the RECORDED pgid, and the identity token is the start time of the process that LEADS it. If
  # this wrapper is not that leader, the recorded group is somebody else's — in the failure mode
  # that matters, the launcher's own — and a `--stop` would take the caller down with it. Refuse to
  # record at all rather than record a group we do not lead: the launcher's handshake then times
  # out and stops us, which is the fail-closed direction.
  [ -n "$pgid" ] && [ "$pgid" = "$pid" ] || {
    die "wrap: refusing to record — pid $pid does not lead its own process group (pgid ${pgid:-unknown}); a stop keyed on that group would signal a bystander"
    exit 1
  }

  ident="$(identity_of "$pgid")"
  [ -n "$ident" ] || { die "wrap: refusing to record — no identity token for pgid $pgid"; exit 1; }

  # The command line, flattened to one line: the record is KEY=value and every reader is a
  # line-oriented `sed -n 's/^key=//p'`, so an embedded newline would forge a field.
  cmd_line="$*"
  cmd_line="${cmd_line//$'\n'/ }"
  cmd_line="${cmd_line//$'\r'/ }"
  created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

  wedge pre-record
  rec="$(printf 'pid=%s\npgid=%s\nidentity=%s\ncmd=%s\ncreated=%s' \
    "$pid" "$pgid" "$ident" "$cmd_line" "$created")"
  atomic_write "$rd/launch" "$rec"
  wedge post-record
  atomic_write "$rd/identity" "$ident"

  # ---- and ONLY NOW the user's command -------------------------------------------
  # Forked, not exec'd: this process must outlive the command to write the terminal record, which
  # is what makes "the child completed" evidence rather than a claim. Stdout, stderr and stdin are
  # inherited from the launcher's redirect, so the command's bytes land unmerged in the durable
  # logs with no primitive-injected framing.
  #
  # THE WRAPPER IGNORES TERM; THE COMMAND DOES NOT. No handler is installed — there is no code path
  # anywhere that can write `terminal` other than the `wait` below returning the command's own
  # status, which is the property "untrapped" exists to protect. The disposition is set to IGNORE,
  # and only for the wrapper, because every teardown in this contract is GROUP-directed: the
  # wrapper leads the recorded group, so `kill -TERM -$pgid` reaches the wrapper too. MEASURED: with
  # the default disposition the wrapper dies alongside the command and `terminal` is NEVER written,
  # so `kind=signal` would be unreachable and every signal death would degrade to `cause=vanished`.
  # An ignored signal is inherited across fork and exec, so the subshell RESETS TERM to its default
  # before exec'ing — the command must still be killable by the very signal the wrapper survives.
  # SIGKILL is deliberately not survivable: a KILLed group leaves no record, which is exactly the
  # `cause=vanished` reading it should get.
  trap '' TERM
  ( trap - TERM; exec "$@" ) &
  local cmd_pid=$! rc=0
  wait "$cmd_pid" || rc=$?

  # The POSIX-shell floor and which way it is biased (spec assumption 16, NAMED RESIDUAL):
  # a shell sees only $?, which conflates a genuine `exit 143` with death by signal 15. A code
  # in 129..192 is therefore recorded kind=signal. The two errors are NOT symmetric: reading a
  # signal death as `failed` mints integration-repair work for tests that never ran, which
  # assumption 3 forbids; reading a genuine `exit 143` as `died` costs one relaunch that
  # reproduces the same code and then halts. Recorded in scripts/gate-run.md § Named residuals.
  if [ "$rc" -ge 129 ] && [ "$rc" -le 192 ]; then
    atomic_write "$rd/terminal" "kind=signal signal=$(( rc - 128 ))"
  else
    atomic_write "$rd/terminal" "kind=exit code=$rc"
  fi
  exit "$rc"
}

# ==================================================================================
# --launch
# ==================================================================================
RUN_DIR=""
SPAWN_PID=""

# The bounded stop on the failure path. Called only when establishment did NOT complete, so the
# handle is never returned and nothing else will ever come back to clean this up.
launch_failure_stop() {
  local rec_pgid spawn_pgid probe_pgid="" mine
  mine="$(my_pgid)"
  rec_pgid="$(numeric_or_empty "$(record_field "$RUN_DIR/launch" pgid)")"
  spawn_pgid="$(numeric_or_empty "$(ps -o pgid= -p "$SPAWN_PID" 2>/dev/null | tr -d ' ' || true)")"

  if [ -n "$rec_pgid" ]; then
    # RECORD PRESENT ⇒ the group is ADDRESSABLE, so the whole tree gets the signal.
    probe_pgid="$rec_pgid"
    if [ -n "$mine" ] && [ "$rec_pgid" = "$mine" ]; then
      die "refusing to signal process group $rec_pgid — it is this launcher's own group"
    else
      kill -TERM -"$rec_pgid" 2>/dev/null || true
      local w=0
      while kill -0 -"$rec_pgid" 2>/dev/null && [ "$w" -lt 20 ]; do sleep 0.1; w=$(( w + 1 )); done
      kill -0 -"$rec_pgid" 2>/dev/null && { kill -KILL -"$rec_pgid" 2>/dev/null || true; }
    fi
  else
    # NO RECORD ⇒ by the ORDERING RULE there is nothing here but the plumbing process, so the stop
    # is pid-directed at the direct spawn child and the group is only PROBED. Deliberately not a
    # group kill: with no record, a group kill would quietly clean up after a violation of the
    # ordering rule instead of leaving it observable, and this failure path is the only thing
    # standing between that violation and an unaddressable command process running unattended.
    probe_pgid="$spawn_pgid"
  fi

  # The direct spawn child is OUR child and has not been waited on, so its pid cannot have been
  # recycled: a pid-directed signal here can only ever reach the process we started. Reaping it is
  # what makes the verification below honest — a zombie still answers `kill -0`.
  kill -TERM "$SPAWN_PID" 2>/dev/null || true
  local t=0
  while kill -0 "$SPAWN_PID" 2>/dev/null && [ "$t" -lt 20 ]; do sleep 0.1; t=$(( t + 1 )); done
  kill -KILL "$SPAWN_PID" 2>/dev/null || true
  wait "$SPAWN_PID" 2>/dev/null || true

  local leaked=0
  if [ -n "$probe_pgid" ]; then
    local v=0
    while kill -0 -"$probe_pgid" 2>/dev/null && [ "$v" -lt 20 ]; do sleep 0.1; v=$(( v + 1 )); done
    kill -0 -"$probe_pgid" 2>/dev/null && leaked=1
  fi
  if [ "$leaked" = 1 ]; then
    die "LAUNCH FAILED AND UNVERIFIED — process group $probe_pgid is still alive. A child of this run may be running unattended; inspect and dispose of it manually. Run dir: $RUN_DIR"
  else
    die "launch failed — establishment did not complete within ${LAUNCH_ESTABLISH_SECS}s; the run's processes were terminated and verified gone. Run dir: $RUN_DIR"
  fi
}

launch_failed() { die "$*"; report launch-failed; exit 1; }

do_launch() {
  local root="" run_name="" mode
  while [ $# -gt 0 ]; do
    case "$1" in
      # `shift 2` FAILS rather than truncating when the flag is the last argument, so each
      # value-taking flag checks its operand is present before consuming it.
      --root)     [ $# -ge 2 ] || launch_failed "launch: --root needs a directory"
                  root="$2"; shift 2 ;;
      --run-name) [ $# -ge 2 ] || launch_failed "launch: --run-name needs a name"
                  run_name="$2"; shift 2 ;;
      --)         shift; break ;;
      *)          launch_failed "launch: unknown argument: $1" ;;
    esac
  done
  [ $# -gt 0 ] || launch_failed "launch: no command after --"

  umask 077
  if [ -n "$root" ]; then
    mkdir -p "$root" 2>/dev/null || launch_failed "launch: cannot create root: $root"
  else
    root="$(mktemp -d "${TMPDIR:-/tmp}/gate-run.XXXXXX")" \
      || launch_failed "launch: cannot mint a default root under ${TMPDIR:-/tmp}"
  fi
  [ -d "$root" ] && [ -w "$root" ] || launch_failed "launch: root is not a writable directory: $root"
  root="$(cd "$root" && pwd -P)" || launch_failed "launch: cannot resolve root: $root"

  # The helper ALWAYS mints the run dir, so two callers cannot collide on one. An existing dir is
  # REFUSED, never reused: reuse would silently graft a new run onto another run's records.
  if [ -n "$run_name" ]; then
    # `--run-name` is a determinism hook for fixtures, so it is shape-checked rather than trusted:
    # a name carrying a slash or a `..` would place the run dir outside the root.
    case "$run_name" in
      *[!A-Za-z0-9._-]*|.|..|'') launch_failed "launch: invalid --run-name: $run_name" ;;
    esac
    RUN_DIR="$root/$run_name"
    mkdir "$RUN_DIR" 2>/dev/null \
      || launch_failed "launch: run dir already exists (or cannot be created), refusing to reuse it: $RUN_DIR"
  else
    RUN_DIR="$(mktemp -d "$root/run.XXXXXX")" || launch_failed "launch: cannot mint a run dir under $root"
  fi

  mode="$(detach_mode)"
  if [ "$mode" = session ]; then
    # Rung 1. Job control is deliberately OFF here: backgrounding under `set -m` would make setsid
    # itself a process-group leader, at which point setsid(1) forks and `$!` names a parent that
    # exits immediately instead of the wrapper. Without job control, setsid(2) succeeds in place and
    # the exec preserves the pid, so `$!` is the wrapper.
    set +m
    setsid "$BASH_BIN" "$SELF" --__wrap "$RUN_DIR" -- "$@" \
      >"$RUN_DIR/stdout.log" 2>"$RUN_DIR/stderr.log" </dev/null &
    SPAWN_PID=$!
  else
    # Rung 3, measured at plan time and again in ADR-0080: `set -m` makes a background job a
    # PROCESS-GROUP LEADER, so it survives a group-directed teardown of the launcher's own group.
    # This delivers own-process-group detachment plus the handshake below — NOT a new session.
    set -m
    "$BASH_BIN" "$SELF" --__wrap "$RUN_DIR" -- "$@" \
      >"$RUN_DIR/stdout.log" 2>"$RUN_DIR/stderr.log" </dev/null &
    SPAWN_PID=$!
    set +m
  fi

  # ---- THE HANDSHAKE -------------------------------------------------------------
  # Bounded by LAUNCH_ESTABLISH_SECS. Both conjuncts are required: `launch` (the group is
  # addressable) AND `identity` (establishment completed). A dead-or-reaped wrapper ends the wait
  # early, but never the decision — the records are re-read once more below, because a fast command
  # can finish and be reaped between two polls having recorded everything correctly.
  local ticks=$(( LAUNCH_ESTABLISH_SECS * 10 )) t=0 state
  while [ "$t" -lt "$ticks" ]; do
    { [ -s "$RUN_DIR/launch" ] && [ -s "$RUN_DIR/identity" ]; } && break
    state="$(ps -o state= -p "$SPAWN_PID" 2>/dev/null | tr -d ' ' || true)"
    case "$state" in ''|Z*) break ;; esac
    sleep 0.1; t=$(( t + 1 ))
  done

  if [ -s "$RUN_DIR/launch" ] && [ -s "$RUN_DIR/identity" ]; then
    # Establishment claims a separated group; verify the claim before handing back a handle. A
    # recorded pgid equal to ours means detachment did not happen, and the handle would name a
    # group that the caller's own teardown will reap.
    local rec_pgid mine
    rec_pgid="$(numeric_or_empty "$(record_field "$RUN_DIR/launch" pgid)")"
    mine="$(my_pgid)"
    if [ -z "$rec_pgid" ] || { [ -n "$mine" ] && [ "$rec_pgid" = "$mine" ]; }; then
      die "launch: the child did not separate into its own process group (recorded '${rec_pgid:-none}', launcher's '$mine')"
      launch_failure_stop
      report launch-failed
      exit 1
    fi
    report "$RUN_DIR"
    exit 0
  fi

  launch_failure_stop
  report launch-failed
  exit 1
}

# ==================================================================================
# --observe
# ==================================================================================

# The identity token THIS RUN recorded — the value every liveness probe is checked against.
#
# TWO SOURCES, and the reason is the launch ordering: `launch` is written BEFORE `identity`, so an
# establishment that crashed between the two writes leaves the token file absent while the launch
# record still carries the same value in its `identity=` field. Either source answers, and they
# cannot disagree — one `identity_of` call produced both.
#
# An empty result is a real answer, not an error: it means this run never recorded who it was, and
# `group_alive_and_ours` fails the conjunct closed on it.
recorded_identity() {  # $1 = run dir -> the recorded token, empty when neither source holds one
  local tok=""
  if [ -s "$1/identity" ]; then
    tok="$(cat "$1/identity" 2>/dev/null || true)"
    tok="${tok%%$'\n'*}"
  fi
  [ -n "$tok" ] || tok="$(record_field "$1/launch" identity)"
  printf '%s' "$tok"
}

# LIVENESS, IDENTITY-CHECKED — never a bare `kill -0` (spec assumption 9). A pgid answers for
# whoever holds it NOW, and pgids are recycled; the run that recorded this one may be long dead and
# a stranger may lead the group today. The conjunction is: the group exists AND the process leading
# it started at the instant this run recorded. Every leg fails CLOSED — an absent pgid, a
# non-numeric one, an empty recorded token, an empty live token — because the only cost of a false
# `died` is one bounded relaunch, while a false `running` waits out the caller's whole budget on a
# run that is not there.
group_alive_and_ours() {  # $1 = run dir
  local rd="$1" pgid want have
  pgid="$(numeric_or_empty "$(record_field "$rd/launch" pgid)")"
  [ -n "$pgid" ] || return 1
  # `kill -0 -0` means THIS caller's own group and `kill -0 -1` means every process the user can
  # signal, so neither can ever be treated as a recorded run's group.
  [ "$pgid" -gt 1 ] || return 1
  kill -0 -"$pgid" 2>/dev/null || return 1
  want="$(recorded_identity "$rd")"
  [ -n "$want" ] || return 1
  have="$(identity_of "$pgid")"
  [ -n "$have" ] || return 1
  [ "$have" = "$want" ]
}

# Map the terminal record to a state line. Returns NON-ZERO when there is no record at all — that
# is the only "keep looking" answer; every other outcome is a verdict.
#
# `kind=signal` reads `stopped` when this run was CANCELLED on purpose: `--stop` writes `stop-intent`
# immediately before it signals and `stopped` once it has verified the group gone, so either file
# means the signal that killed the child was ours. Without that, a deliberately cancelled run would
# read `died` forever and an idempotent call site would relaunch a cancellation.
classify_record() {  # $1 = run dir -> prints one state line; non-zero when `terminal` is absent
  local rd="$1" rec payload
  [ -f "$rd/terminal" ] || return 1
  rec="$(cat "$rd/terminal" 2>/dev/null || true)"
  case "$rec" in
    'kind=exit code='*)
      payload="$(numeric_or_empty "${rec#kind=exit code=}")"
      if [ -z "$payload" ]; then :
      elif [ "$payload" = 0 ]; then printf 'state=passed\n'; return 0
      else printf 'state=failed\n'; return 0
      fi
      ;;
    'kind=signal signal='*)
      payload="$(numeric_or_empty "${rec#kind=signal signal=}")"
      if [ -n "$payload" ]; then
        if [ -f "$rd/stopped" ] || [ -f "$rd/stop-intent" ]; then printf 'state=stopped\n'
        else printf 'state=died cause=signal\n'
        fi
        return 0
      fi
      ;;
  esac
  # Anything unparseable. A malformed record says the SUPERVISOR did not finish cleanly, which is
  # not a verdict about the child — and a verdict read out of garbage is fabricated.
  die "malformed terminal record in $rd: ${rec//$'\n'/ }"
  printf 'state=unavailable\n'
}

# The last N lines of the run's own streams, to STDERR. Never stdout: a tail is multiline and
# arbitrary, and stdout is a protocol exactly one line wide.
log_tail_to_stderr() {  # $1 = run dir
  local rd="$1" stream body
  for stream in stderr.log stdout.log; do
    [ -s "$rd/$stream" ] || continue
    body="$(tail -n 20 "$rd/$stream" 2>/dev/null || true)"
    [ -n "$body" ] || continue
    printf 'gate-run: --- last 20 lines of %s ---\n%s\n' "$rd/$stream" "$body" >&2
  done
}

# THE READ ORDER IS THE CONTRACT. Prints the state line on stdout and nothing else; every
# diagnostic goes to stderr. `do_observe` captures this, so there is structurally no path by which
# a second line can reach the protocol channel.
observe_state() {  # $1 = run dir
  local rd="$1" rec

  { [ -d "$rd" ] && [ -r "$rd/launch" ]; } || {
    die "rundir-unreadable: $rd"
    printf 'state=unavailable\n'
    return 0
  }

  # 1. TERMINAL RECORD FIRST — it always wins. The child's own verdict outranks any probe of the
  #    group it used to lead, and the wrapper is the only writer of that record.
  if rec="$(classify_record "$rd")"; then printf '%s\n' "$rec"; return 0; fi

  # THE TOCTOU WINDOW, NAMED SO A FIXTURE CAN HOLD IT OPEN. Everything this function does between
  # the read above and the probe below is speculation about a world that may already have changed;
  # the re-read at step 3 is the only thing that makes the verdict honest, and nothing can tell the
  # two reads apart unless a test can stop the observer right here. Inert unless armed.
  barrier post-first-record

  # 2. Liveness — identity-checked (assumption 9), never a bare `kill -0`.
  if group_alive_and_ours "$rd"; then printf 'state=running\n'; return 0; fi

  # 3. Dead or identity-mismatched ⇒ RE-READ. THE WHOLE POINT: atomicity prevents partial reads,
  #    not stale ones. The record read in step 1 was a snapshot of a moment that has passed, and
  #    the child had every chance to finish since. Without this re-read the sequence
  #    "no record → child completes → dead probe" turns a run that PASSED into a `died`, and a
  #    call site keyed on `died` relaunches it. The re-read is sound rather than merely defensive
  #    because of the invariant the wrapper holds — it is the ONLY writer of `terminal`, so a
  #    record visible now was necessarily written by a child that completed.
  if rec="$(classify_record "$rd")"; then printf '%s\n' "$rec"; return 0; fi

  if [ -f "$rd/stopped" ]; then printf 'state=stopped\n'; return 0; fi

  # No record, and the group this run recorded is gone or is not ours. Nothing survives that could
  # ever write a verdict, so this is terminal — and it is detected on THIS observation rather than
  # at the far end of a caller's budget, which is the promptness the whole contract exists for.
  die "died: the recorded group is gone and no terminal record was ever written (run dir: $rd)"
  log_tail_to_stderr "$rd"
  printf 'state=died cause=vanished\n'
}

do_observe() {
  local rd="${1:-}" state
  [ $# -le 1 ] || { die "observe: expected exactly one run dir"; report "state=unavailable"; return 1; }
  [ -n "$rd" ] || { die "observe: missing run dir"; report "state=unavailable"; return 1; }
  state="$(observe_state "$rd")"
  # stdout is a protocol exactly one line wide; the trim makes that structural rather than a
  # convention every branch above has to remember. The exit status is keyed to the SAME trimmed
  # value the caller was handed, so the two can never disagree about what was reported.
  state="${state%%$'\n'*}"
  report "$state"
  # Callers key on the report line, never on this. Documented in scripts/gate-run.md § Exit codes:
  # non-zero says only "no verdict was available", which is `unavailable` and nothing else.
  case "$state" in state=unavailable) return 1 ;; esac
  return 0
}

usage() {
  printf '%s\n' \
    'usage: gate-run.sh --launch [--root <dir>] [--run-name <name>] -- <command…>' \
    '       gate-run.sh --observe <run-dir>' \
    'Contract: scripts/gate-run.md' >&2
}

VERB="${1:-}"
[ $# -gt 0 ] && shift || true
case "$VERB" in
  --launch)  do_launch "$@" ;;
  --observe) do_observe "$@" ;;
  --__wrap)  do_wrap "$@" ;;
  -h|--help) usage; exit 0 ;;
  *)         die "unknown verb: ${VERB:-<none>}"; usage; exit 2 ;;
esac
