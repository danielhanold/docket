# Gate caller loop — the caller-side contract for driving the native gate

This reference is the **caller-side contract for driving the native gate**: the loop a caller runs,
the state vocabulary that loop resolves into, the stop mapping its dispositions key on, and the
measurement record behind the launch shape. It is a **caller contract, not a harness quarantine** —
that axis separates it from [`gate-execution.md`](gate-execution.md), which holds measured
per-harness verdicts and mechanism detail read once, ahead of the act. Change 0271 drew the same
line when it created `references/delegation-execution.md` rather than folding caller-facing content
into `gate-execution.md`.

## The caller's loop

The caller drives the **native gate** directly: this loop does not poll or observe on the caller's
behalf, and since change 0338 an observation has exactly one serialization — the gate's protocol-v1
JSON, parsed with **jq, a required dependency of this loop** (jq is already a docket dependency
elsewhere). A missing jq is a loud terminal diagnostic, never a silent spin. Copy this loop
verbatim; `tests/test_gate_caller_loop.sh` extracts this fence and executes it against scripted
documents and against the real gate.

```bash
# `run_dir` is the run directory `docket gate launch` reported. GATE_OBSERVATION_BUDGET is the
# docket execution policy from the Step-0 config export, in minutes; 0 is legal and buys exactly
# one observation. The `:?` is load-bearing: bash arithmetic reads an unset name as 0, so a bare
# read would make a MISSING export look like a configured 0 and halt a healthy run one
# observation in.
deadline=$(( $(date +%s) + ${GATE_OBSERVATION_BUDGET:?from the Step-0 config export} * 60 ))
state="" cause=""
while :; do
  # The loop's one hard dependency, checked where it is used: without jq no document can be
  # read, so the only honest answer is a LOUD terminal unavailable — never a poll-again.
  if ! command -v jq >/dev/null 2>&1; then
    printf '%s\n' "jq not found — the gate observe loop requires it" >&2
    state=unavailable; break
  fi
  # Capture, THEN parse. The `|| true` is load-bearing: observe exits non-zero for real
  # verdicts too (failed, and every interrupted state), and the rule is that callers key on the
  # document, never the exit code — without it an errexit caller dies before any arm runs.
  doc="$(docket gate observe "$run_dir" --json)" || true
  st="$(jq -r '.state // empty' <<<"$doc" 2>/dev/null)" || st=""
  case "$st" in
    running) : ;;                                  # the only retryable state
    passed|failed|stopped) state="$st"; break ;;
    signaled|vanished)                             # the JSON spellings of a death: the child
      state=died                                   # never finished, so this is never `failed`
      cause="$(jq -r '.cause // empty' <<<"$doc" 2>/dev/null)" || cause=""
      break ;;
    *)                                             # empty state, garbled document, jq failure:
      printf '%s\n' "gate observe returned no recognizable state; failing closed as unavailable" >&2
      state=unavailable; break ;;                  # fail closed, NEVER a retry arm
  esac
  [ "$(date +%s)" -lt "$deadline" ] || break       # budget spent; `state` stays empty
  sleep 10
done
# An empty `state` means the budget ran out with the run still `running` — the fail-closed case,
# not a verdict about the child. The child was last seen live, so a caller abandoning here calls
# `docket gate stop "$run_dir"` before it reports (`skills/docket-build/SKILL.md`
# § *Gate execution posture*, *Abandoning a live child*).
```

**Never re-derive the state by hand from the document.** A grep or `cut` over the JSON re-creates
exactly the parser drift this loop's jq extraction retired; the document is parsed, or the arm is
the fail-closed one. The **unknown-document arm is terminal, never a retry**: a document outside
the vocabulary means the invocation or the environment is wrong, so the loop stops polling and
disposes it as `unavailable`. A retry there is precisely the shape that never terminates — it is
the 0337 incident.

The loop RESOLVES the native spellings into the caller's disposition vocabulary: `signaled` and
`vanished` both resolve to `died` (with `cause` carrying the document's own qualifier, possibly
empty), because a signalled or vanished child **never finished** — `died` is never `failed`, and
only `failed` may feed repair work. What to *do* with each resolved state is the caller's policy:
dispositions are stated in `skills/docket-build/SKILL.md` § *Gate execution posture*.

## State vocabulary and retryability

The observed states are read from `internal/app/gate.go` at implementation time: `GateResult`'s
`State` field is a `GateState`, whose spellings mirror `internal/process`'s `State` constants —
`running`, `passed`, `failed`, `signaled`, `stopped`, and `vanished`. The loop resolves them into
the caller's disposition vocabulary under these rules:

- `signaled` and `vanished` both resolve to `died`, with `cause` carrying the document's own
  qualifier (possibly empty).
- A `died` run **never finished**, so it is never `failed`. Only `failed` — the child ran and went
  red — may feed repair work.
- **Only `running` is retryable.** Every other arm is terminal, including the fail-closed
  unknown-document arm: a document outside the vocabulary stops the loop as `unavailable`, never a
  retry.

## The caller's verbs

The loop drives three of the native gate's verbs. Each returns a protocol-v1 result to the caller:

| Verb | What it returns to the caller |
|---|---|
| `launch` | the run-dir handle in a protocol-v1 JSON envelope, or a failure envelope |
| `observe` | one protocol-v1 JSON document per call |
| `stop` | a `GateResult` per the stop mapping table below |

`recover` and `cleanup` are **operator verbs, not caller-loop verbs**: the native CLI registers five
subcommands (`internal/cli/gate.go`), and only these three belong to the loop.

## The stop mapping table

Native `docket gate stop <run-dir>` returns a `GateResult` whose envelope `result` is `applied` or
`no-op` (or an error), with `state` preserved. `internal/app/gate.go`'s `GateStop` states it
directly: *"A performed termination is applied; an already-terminal no-op carries the preserved
state (consumers read state; the stop performed nothing)."* The table maps that two-axis answer onto
the three caller dispositions:

| Native stop outcome | What it means | Caller disposition |
|---|---|---|
| `no-op` — `state` preserved | the run was already finished; the stop performed nothing | **re-observe and key on the preserved `state`** — this is the ordinary outcome of stopping a live child |
| `applied` | we terminated it; the run produced no verdict of its own | **one relaunch only**, and only where the child is idempotent |
| `error` — any error result, or the run unreachable | nothing can be proven about what survives; `error` is the caller's label for the whole error family, which the envelope spells many ways | **abort and report loudly**; never relaunch |

`applied` and `no-op` are read from `internal/app/result.go` (`ResultApplied` / `ResultNoOp`); if a
spelling ever differs there, the source wins and this table is written to match. The mapping's
semantics — which outcome earns which disposition — are settled here.

## Per-platform capability note (shell-era measurement record)

The measurements in this section were taken against the retired `gate-run` shell facade's launch
shape — a shell supervisor, not the native gate — and are kept here as a historical record of what
that shape delivered per platform. The native launcher's guarantee is recorded in the carryover
paragraph that follows.

**The contract does not claim a new session on a platform where it does not deliver one.** What is
delivered is decided by a **runtime probe**, never by a hard-coded platform name.

The session-primitive ladder, resolved and measured at plan time on darwin 25.6.0:

- **Rung 1 — `setsid(1)`.** Where present (Linux, typically), the wrapper is started under it and
  the child gets a **genuine new session**. This is the preferred rung and it is taken whenever
  `command -v setsid` succeeds.
- **Rung 2 — `script(1)`, the macOS pty candidate — MEASURED AND REJECTED.** It is present at
  `/usr/bin/script` and it fails on two independent criteria. **Primitive-injected framing:**
  `/usr/bin/script -q /dev/null /bin/bash -c 'echo alive' </dev/null` produced the bytes
  `^D \b \b a l i v e \r \n` — a `^D` typescript marker and pty CRLF translation — so the durable
  log would not hold the child's bytes. **No stream separation:** a pty merges the child's stdout
  and stderr onto one stream, which the durable-unmerged-streams requirement and the
  stdout-is-the-protocol rule both forbid. It is additionally fragile: with stdin attached to a
  socket the same invocation failed outright with
  `script: tcgetattr/ioctl: Operation not supported on socket`.
- **Rung 3 — the honest narrowing.** Where no session primitive is available without taking a new
  dependency — **macOS today** — the wrapper is backgrounded under Bash job control (`set -m`), which
  makes it a **process-group leader**. On such a platform this contract delivers **own process group
  plus the unchanged detachment handshake**, and **nothing about a session**.

What rung 3 does and does not buy is quoted from the two places that measured it, verbatim rather
than by pointer:

- ADR-0080, on the `set -m` technique: *"the child gets its own process GROUP, not a new SESSION —
  it remains in the launcher's session, so session-scoped teardown was not tested and is not
  claimed."* Own-group detachment **was** measured there, one run with two arms and one variable
  changed: the `set -m` child survived a `TERM` directed at the launcher's whole process group and
  the non-`set -m` child did not.
- `skills/docket-build/references/gate-execution-evidence.md`, on the absent primitive:
  *"**`setsid(1)` is not installed on macOS**, so the launch shape cannot use it."*

So: on rung 1 the child survives teardown of the initiating call's session; on rung 3 it survives
teardown of the initiating call's process **group**, which is the property the gate-execution
capabilities actually require, and session-scoped teardown is neither tested nor claimed. The
narrowing is a shell-era finding; the carryover paragraph below records why the native launcher is
not bound by it.

The native launcher is **not** bound by that shell-era rung-3 narrowing. ADR-0095 records that the
native per-run supervisor establishes a genuine new session with `setsid` on **both Darwin and
Linux**, and the Go launcher's own `Launch` in `internal/process/launch.go` describes re-exec'ing
the binary *"as a Setsid session-leader supervisor with the live lock and a handshake pipe"*. That
guarantee is therefore **at least as strong** as the shell shape the per-harness verdicts above were
measured under, so those verdicts **carry over without re-probing** — recorded here rather than left
silent.
