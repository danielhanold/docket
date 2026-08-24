#!/usr/bin/env bash
# tests/test_gate_caller_loop.sh — guards the published caller loop in
# skills/docket-build/references/gate-caller-loop.md, executed byte-unmodified against scripted and
# real protocol-v1 documents; relocated from tests/test_gate_run.sh by change 0339.
#
# WHAT THIS FILE IS FOR: the caller-loop fence in gate-caller-loop.md § The caller's loop is
# agent-executed markdown (learnings: agent-executed-markdown-is-code), so it is CODE and must be
# tested as code. These asserts extract the fence byte-for-byte and RUN it — first against a
# scripted stub `docket` that hands back chosen protocol-v1 documents (the arms), then against the
# real native gate's own documents (the documents are real).
#
# It deliberately does NOT source tests/lib/gate_run_common.sh: that prologue and its barrier
# machinery belong to the retired gate-run shell facade (deleted by change 0339), not to this loop.
# The prologue below is self-contained.
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
# Whitespace-flatten a hard-wrapped markdown slice: grep matches within a line, so a phrase-spanning
# pattern over wrapped prose otherwise doubles as a re-flow guard.
flat(){ tr -s '[:space:]' ' ' <<<"$1"; }

# Hermetic sandbox. mktemp REQUIRES a template on macOS (AGENTS.md § Shell — a bare `mktemp -d`
# ignores TMPDIR). The real-gate leg below launches native supervised children under this dir; the
# EXIT trap kills any group a run's own manifest recorded before removing the sandbox, so a leg that
# dies on a failing assert cannot leave a detached child on the machine.
SBX="$(mktemp -d "${TMPDIR:-/tmp}/gate-caller-loop.XXXXXX")"; SBX="$(cd "$SBX" && pwd -P)"
caller_loop_cleanup(){
  local mf pg mine
  mine="$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')"
  while IFS= read -r mf; do
    pg="$(jq -r '.pgid // empty' "$mf" 2>/dev/null || true)"
    case "$pg" in ''|*[!0-9]*) continue ;; esac
    [ "$pg" != "$mine" ] || continue        # never a group-directed kill at our own group
    kill -KILL -"$pg" 2>/dev/null || true
  done < <(find "$SBX" -type f -name manifest.json 2>/dev/null)
  rm -rf "$SBX"
}
trap caller_loop_cleanup EXIT

# ---- THE CALLER'S LOOP IS A TAUGHT, EXECUTABLE SURFACE (0286 shape, 0338 serialization) --------
# The loop parses the native gate's protocol-v1 JSON with jq. An agent runs the fence's bytes
# verbatim (learnings: agent-executed-markdown-is-code), so these asserts EXECUTE the fence. The
# fence source is the new reference file; the section is sliced from its own `## The caller's loop`
# heading to the NAMED next heading `## State vocabulary and retryability`, whose presence is
# asserted so the slice cannot run to EOF (learnings: section-slice-needs-a-named-terminator).
LOOP_REF="$REPO/skills/docket-build/references/gate-caller-loop.md"
assert "the caller-loop reference exists" '[ -f "$LOOP_REF" ]'
ref="$(cat "$LOOP_REF" 2>/dev/null || true)"
# Population floor FIRST: an unreadable page has to redden here, or every assert below passes by
# default and reports a section it never actually looked for.
assert "the reference is non-vacuous (>= 40 lines)" '[ "$(grep -c . <<<"$ref")" -ge 40 ]'
assert "the section carries its own heading" \
  'grep -qxF -- "## The caller'"'"'s loop" <<<"$ref"'
# The NAMED terminator: without it the section slice runs to EOF and swallows the rest of the page.
assert "the section's named terminator heading is present" \
  'grep -qxF -- "## State vocabulary and retryability" <<<"$ref"'
loop_sec="$(awk '
  /^## The caller'"'"'s loop$/ {f=1; next}
  f && /^## State vocabulary and retryability$/ {f=0}
  f' <<<"$ref")"
assert "the caller-loop section was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$loop_sec")" -ge 20 ]'
loop_fence="$(awk '/^```bash$/ {inf=1; next}  inf && /^```$/ {inf=0; next}  inf {print}' <<<"$loop_sec")"
assert "the canonical loop fence was located (non-vacuity anchor)" \
  '[ "$(grep -c . <<<"$loop_fence")" -ge 15 ]'
loop_flat="$(flat "$loop_sec")"
# jq is a DOCUMENTED required dependency, bound to the loop rather than merely mentioned
# (learnings: prose-guard-binds-phrase-to-claim). Mutation: drop the dependency sentence.
assert "the section documents jq as a required dependency of the loop" \
  'grep -qiE "jq[^.]{0,80}required dependency" <<<"$loop_flat"'
assert "and it names the unknown-document arm as terminal, never a retry" \
  'grep -qiE "unknown[^.]{0,140}(never a retry|stop[s]? polling)" <<<"$loop_flat"'
assert "the section defers disposition policy to the build skill's posture" \
  'grep -qF -- "Gate execution posture" <<<"$loop_sec"'
assert "the section names the mandatory stop on the abandon-while-running leg" \
  'grep -qiE "abandon[^.]{0,100}gate stop[^.]{0,60}before it reports" <<<"$loop_flat"'

LOOPBOX="$SBX/loopbox"; mkdir -p "$LOOPBOX/bin"
cat >"$LOOPBOX/bin/docket" <<'STUB'
#!/usr/bin/env bash
# Stub native gate: answers the Nth line of $OBS_SCRIPT as a whole protocol document, repeating
# the last line forever. Mirrors the real exit mapping (internal/app/result.go ExitCode +
# gate.go mapObservation): running/passed -> 0, everything else -> 1. Past the cap it answers a
# vocabulary-outside document so a mutated, never-terminating fence resolves to a comparable
# `unavailable|201` instead of hanging the suite (learnings: mutation-target-needs-a-forced-exit).
n=$(( $(cat "$OBS_COUNT") + 1 )); printf '%s' "$n" >"$OBS_COUNT"
[ "$n" -le 200 ] || { printf '{"protocol_version":1,"operation":"gate.observe","result":"internal-error","state":"LOOPCAP"}\n'; exit 1; }
line="$(sed -n "${n}p" "$OBS_SCRIPT")"
[ -n "$line" ] || line="$(sed -n '$p' "$OBS_SCRIPT")"
printf '%s\n' "$line"
case "$line" in
  *'"state":"running"'*|*'"state":"passed"'*) exit 0 ;;
  *) exit 1 ;;
esac
STUB
chmod +x "$LOOPBOX/bin/docket"
printf '%s\n' "$loop_fence" >"$LOOPBOX/loop.body"

# Runs the byte-unmodified fence under `set -euo pipefail` with a SIMULATED clock (sleep advances
# a counter; date reports it), exactly the 0286 harness shape. $2 selects the PATH: `jq` keeps the
# real PATH (jq present) behind the stub dir; `nojq` restricts PATH to the stub dir alone, which
# is the simulated jq-absent machine. Output: `<state>|<cause>|<count>`.
run_loop(){ # $1 = budget minutes or UNSET; $2 = jq|nojq; $3… = scripted observe documents
  local budget="$1" pathmode="$2"; shift 2
  printf '%s\n' "$@" >"$LOOPBOX/script"
  printf '0' >"$LOOPBOX/count"
  {
    if [ "$budget" = UNSET ]; then printf '%s\n' 'set -eo pipefail'
    else                           printf '%s\n' 'set -euo pipefail'; fi
    if [ "$pathmode" = nojq ]; then printf 'PATH=%q\n' "$LOOPBOX/bin"
    else                            printf 'PATH=%q\n' "$LOOPBOX/bin:$PATH"; fi
    printf '%s\n' '__now=0' \
      'date(){ printf "%s\n" "$__now"; }' \
      'sleep(){ __now=$(( __now + ${1:-0} )); }' \
      'run_dir=/nonexistent-run-dir'
    [ "$budget" = UNSET ] || printf '%s\n' "GATE_OBSERVATION_BUDGET=$budget"
    cat "$LOOPBOX/loop.body"
    printf '%s\n' 'printf "%s|%s" "${state}" "${cause:-}"'
  } >"$LOOPBOX/harness.sh"
  local st
  st="$(OBS_SCRIPT="$LOOPBOX/script" OBS_COUNT="$LOOPBOX/count" \
        "$DOCKET_BASH_PATH" "$LOOPBOX/harness.sh" 2>"$LOOPBOX/harness.err")" || st="ERRExit|"
  printf '%s|%s' "$st" "$(cat "$LOOPBOX/count")"
}
J='{"protocol_version":1,"operation":"gate.observe","result":"applied","state":"running"}'
P='{"protocol_version":1,"operation":"gate.observe","result":"applied","state":"passed"}'
F='{"protocol_version":1,"operation":"gate.observe","result":"gate-failed","state":"failed","exit_code":1}'
S='{"protocol_version":1,"operation":"gate.observe","result":"interrupted","state":"stopped"}'
G='{"protocol_version":1,"operation":"gate.observe","result":"interrupted","state":"signaled","cause":"terminated"}'
V='{"protocol_version":1,"operation":"gate.observe","result":"interrupted","state":"vanished"}'
E='{"protocol_version":1,"operation":"gate.observe","result":"invalid-state","reason":"observe: no run at that path"}'

# 1 — terminal states dispose in one observation, in the loop's own vocabulary.
assert "the loop disposes a passed document as passed, in one observation" \
  '[ "$(run_loop 5 jq "$P")" = "passed||1" ]'
assert "the loop disposes a failed document as failed (despite its exit-1 transport)" \
  '[ "$(run_loop 5 jq "$F")" = "failed||1" ]'   # mutation key: drop the fence's `|| true` -> ERRExit
assert "the loop disposes a stopped document as stopped" \
  '[ "$(run_loop 5 jq "$S")" = "stopped||1" ]'
# 2 — running is the ONLY retryable state, and the retry actually happens.
assert "the loop retries running and takes the next document's verdict" \
  '[ "$(run_loop 5 jq "$J" "$J" "$F")" = "failed||3" ]'
# 3 — THE VOCABULARY CORRECTION THE SPEC'S SKETCH MISSED: the native gate spells a death
# signaled/vanished, never died. Both must resolve to the died disposition, cause carried —
# an arm that leaves them to `*)` reads every real signal death as unavailable.
assert "a signaled document resolves to the died disposition, cause extracted" \
  '[ "$(run_loop 5 jq "$G")" = "died|terminated|1" ]'
assert "a vanished document resolves to died too, with an empty cause" \
  '[ "$(run_loop 5 jq "$V")" = "died||1" ]'
# 4 — THE DEFECT THIS CHANGE EXISTS FOR, in each garbled shape: disposed in ONE observation,
# never polled. Mutation key: rewrite `*)` into a retry arm -> both halves invert (and the
# LOOPCAP stub bounds the mutated run at unavailable||201 instead of a hang).
assert "the loop fails closed on a stateless failure document, in exactly one observation" \
  '[ "$(run_loop 5 jq "$E")" = "unavailable||1" ]'
assert "the loop fails closed on a non-JSON line" \
  '[ "$(run_loop 5 jq "hello world")" = "unavailable||1" ]'
assert "the loop fails closed on an empty line" \
  '[ "$(run_loop 5 jq "")" = "unavailable||1" ]'
assert "the fail-closed diagnostic is loud, on stderr" \
  'run_loop 5 jq "$E" >/dev/null; grep -qiF -- "failing closed" "$LOOPBOX/harness.err"'
# 5 — jq ABSENT is a NAMED terminal diagnostic before any observation, never a silent spin.
# Mutation key: delete the fence's `command -v jq` check -> the count reads 1 (the doc was
# fetched) and the named diagnostic vanishes; both asserts redden.
assert "a jq-less PATH resolves unavailable with zero observations" \
  '[ "$(run_loop 5 nojq "$P")" = "unavailable||0" ]'
assert "and the diagnostic names jq by the contract's own words" \
  'run_loop 5 nojq "$P" >/dev/null; grep -qF -- "jq not found — the gate observe loop requires it" "$LOOPBOX/harness.err"'
# 6 — budget semantics, unchanged from 0286. Mutation keys: (a) drop the deadline check -> the
# running fixture resolves unavailable||201 off the stub cap; (b) drop the `:?` -> UNSET reads "||1".
assert "a zero budget buys one observation and reports no verdict" \
  '[ "$(run_loop 0 jq "$J")" = "||1" ]'
assert "a running run that never settles stops at the budget with no verdict" \
  '[ "$(run_loop 5 jq "$J")" = "||31" ]'
assert "an unset budget aborts the loop instead of passing for a configured zero" \
  '[ "$(run_loop UNSET jq "$J")" = "ERRExit||0" ]'

# ---- AND THE FENCE RUNS AGAINST THE REAL GATE'S OWN DOCUMENTS -------------------------------
# The scripted stub above proves the ARMS; this leg proves the documents are real — the fence's
# jq extraction against bytes internal/app/gate.go actually emits, through every terminal state.
# Built once, like tests/test_asset_bundle_drift.sh builds its comparator.
REALBIN="$SBX/realbin"; mkdir -p "$REALBIN"
build_out="$( (cd "$REPO" && go build -o "$REALBIN/docket" ./cmd/docket) 2>&1 )"
assert "the native gate binary builds" '[ -x "$REALBIN/docket" ] || { printf "%s\n" "$build_out" >&2; false; }'

# Launch through the REAL binary; wait until the run is terminal (cheap raw polls, budget-bounded
# by the loop below at 60 real iterations of 0.5s); then run the byte-unmodified fence ONCE with
# a fresh simulated clock, so each terminal state is read by the fence in exactly one observation.
real_loop(){ # $1 = run dir -> prints `<state>|<cause>`
  local rd="$1"
  {
    printf '%s\n' 'set -euo pipefail' \
      "PATH=$REALBIN:\$PATH" \
      '__now=0' 'date(){ printf "%s\n" "$__now"; }' 'sleep(){ __now=$(( __now + ${1:-0} )); }' \
      "run_dir=$rd" 'GATE_OBSERVATION_BUDGET=5'
    cat "$LOOPBOX/loop.body"
    printf '%s\n' 'printf "%s|%s" "${state}" "${cause:-}"'
  } >"$LOOPBOX/real-harness.sh"
  "$DOCKET_BASH_PATH" "$LOOPBOX/real-harness.sh" 2>/dev/null
}
real_launch(){ # $@ = child command -> prints the run dir
  # `--json` is a PERSISTENT root flag and Cobra stops flag parsing at `--` — launch reads its child
  # argv via ArgsLenAtDash (internal/cli/gate.go), so a `--json` trailing the child argv is handed to
  # the CHILD and launch prints human text jq cannot parse. `--json` therefore precedes `--`. Fix is
  # in this helper, never the fence.
  PATH="$REALBIN:$PATH" docket gate launch --root "$SBX/native-runs" --cwd "$REPO" --json -- "$@" \
    | jq -r '.run_dir'
}
await_native_terminal(){ # $1 = run dir -> waits (real time) until observe stops saying running
  local i=0 st doc
  while [ "$i" -lt 60 ]; do
    # Capture THEN parse, exactly as the fence does: observe exits non-zero for every terminal
    # verdict (failed/interrupted -> exit 1), so under `set -o pipefail` a `docket | jq` capture
    # would inherit that non-zero, fire the `||`, and wipe the terminal state back to empty — a
    # poll that never sees the run settle and burns its whole 30s window (AGENTS.md § Shell).
    doc="$(PATH="$REALBIN:$PATH" docket gate observe "$1" --json 2>/dev/null)" || true
    st="$(jq -r '.state // empty' <<<"$doc" 2>/dev/null)" || st=""
    case "$st" in running|"") /bin/sleep 0.5; i=$(( i + 1 )) ;; *) return 0 ;; esac
  done
  return 1
}

# passed / failed — the child's own verdicts.
RDP="$(real_launch /bin/sh -c 'exit 0')"
assert "real launch handed back a run dir" '[ -d "$RDP" ]'
assert "the real run reached a terminal state" 'await_native_terminal "$RDP"'
assert "the fence reads the real passed document as passed" '[ "$(real_loop "$RDP")" = "passed|" ]'
RDF="$(real_launch /bin/sh -c 'exit 3')"
assert "the failed run reached a terminal state" 'await_native_terminal "$RDF"'
assert "the fence reads the real failed document as failed" '[ "$(real_loop "$RDF")" = "failed|" ]'
# stopped — the gate's own stop verb.
RDS="$(real_launch /bin/sh -c 'sleep 30')"
PATH="$REALBIN:$PATH" docket gate stop "$RDS" --json >/dev/null 2>&1 || true
assert "the stopped run reached a terminal state" 'await_native_terminal "$RDS"'
assert "the fence reads the real stopped document as stopped" '[ "$(real_loop "$RDS")" = "stopped|" ]'
# signaled — an external TERM of the real group, read from the run's own manifest (pgid; verified
# against internal/process/records.go's manifestRecord). Racy against the supervisor by design, so
# the assert keys on the RESOLVED disposition (died) only, never on the cause string.
RDG="$(real_launch /bin/sh -c 'sleep 30')"
sig_native_pgid="$(jq -r '.pgid' "$RDG/manifest.json")"
kill -TERM -"$sig_native_pgid" 2>/dev/null || true
assert "the signaled run reached a terminal state" 'await_native_terminal "$RDG"'
sig_read="$(real_loop "$RDG")"
assert "the fence resolves the real signaled document to died (got '$sig_read')" \
  '[ "${sig_read%%|*}" = "died" ]'
# vanished — KILL the whole group so no record can ever be written. Signaled and vanished both
# resolve to died, so keying on died holds whichever the supervisor race produces.
RDV="$(real_launch /bin/sh -c 'sleep 30')"
van_native_pgid="$(jq -r '.pgid' "$RDV/manifest.json")"
kill -KILL -"$van_native_pgid" 2>/dev/null || true
assert "the vanished run reached a terminal state" 'await_native_terminal "$RDV"'
van_read="$(real_loop "$RDV")"
assert "the fence resolves the real vanished document to died (got '$van_read')" \
  '[ "${van_read%%|*}" = "died" ]'

exit "$fail"
