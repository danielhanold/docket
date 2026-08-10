# gate-run.sh — launch, observe, and stop one long-running child

## Purpose

A three-verb helper for a child process that outlives the call that started it: a test-suite gate,
a build, any run whose duration exceeds a single foreground call. It starts the child **detached
and durable**, and it answers the only question a waiting caller actually has — *is this run still
going, and if not, how did it end?* — from **process liveness plus a terminal record**, never from
a success marker appearing in a log.

That distinction is the whole reason the helper exists. A marker-keyed wait cannot tell "still
running" from "died": the two look identical (no marker) until the caller's budget runs out, so a
child killed at minute one is reported at minute forty, with nothing diagnostic to show for it. A
liveness-keyed wait detects the same death on the **next observation**.

Three properties follow from that, and every caller may rely on them:

- **`died` is never `failed`.** A child killed by a signal never finished, so it never produced a
  verdict. `failed` — the child ran and went red — is the only state that may feed repair work.
- **The caller owns the polling loop and its budget.** This helper never polls internally; every
  verb is a short call that returns.
- **Only `running` is retryable.** The other five states are terminal.

## Usage

```
gate-run.sh --launch [--root <dir>] [--run-name <name>] -- <command…>
gate-run.sh --observe <run-dir>
gate-run.sh --stop <run-dir> [--reason <text>]
```

Reached through the facade as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-run …`
(the `WRAPPED_OPS` list in `scripts/docket.sh`), which is how every docket call site invokes it.

- `--launch` — start the command detached and return a **run-directory handle**. Does not block for
  the child's duration.
  - `--root <dir>` (optional) — the directory the run dir is minted **beneath**; created if absent.
    Default is a fresh `mktemp -d "${TMPDIR:-/tmp}/gate-run.XXXXXX"`.
  - `--run-name <name>` (optional) — a determinism hook for fixtures. Shape-checked to
    `[A-Za-z0-9._-]` and refused for `.` / `..` / empty, because a name carrying a slash would place
    the run dir outside the root. The helper **always mints** the run dir, so two callers cannot
    collide on one; an existing dir is **refused, never reused**.
  - `-- <command…>` — required. Everything after `--` is the command, run with `stdout.log` /
    `stderr.log` attached and stdin from `/dev/null`.
- `--observe <run-dir>` — one short, idempotent look. Prints `state=<state>` (plus `cause=<cause>`
  for `died`) and returns.
- `--stop <run-dir> [--reason <text>]` — terminate the run, identity-checked before anything is
  signalled. `--reason` is free text recorded in the intent and stop markers; it is flattened to one
  line, because every reader of those records is line-oriented.
- `--__wrap` — **internal**. The wrapper the launcher re-invokes itself as; never called by hand.

Environment seams:

- `GATE_RUN_ESTABLISH_SECS` — the `--launch` handshake bound in seconds (default `10`; a
  non-numeric or zero value falls back to the default).
- `DOCKET_BASH_PATH` — the Bash 4+ runtime the wrapper is re-invoked under, per the repo floor.
- `GATE_RUN_TEST_WEDGE`, `GATE_RUN_TEST_BARRIER`, `GATE_RUN_TEST_BARRIER_FILE` — **test-only**
  synchronization hooks, **inert by default**. Unset means a no-op at full speed, so neither hook
  can become a hang site in production; both are bounded even when armed. See *Invariants*.

**stdout is the protocol.** Each verb prints **exactly one machine-readable line** on stdout, and
every diagnostic — including a failing run's log tail — goes to stderr.

| Verb | stdout payload |
|---|---|
| `--launch` | the absolute run-directory path, or the single slash-free token `launch-failed` |
| `--observe` | `state=<state>`, or `state=died cause=<signal\|vanished>` |
| `--stop` | one of `stopped`, `already-terminal`, `unavailable` |

`launch-failed` is one shape and never a taxonomy: it carries no slash, so a caller can tell it from
a handle by shape alone without parsing.

## Run-directory layout

```
<run-dir>/                        # minted by --launch beneath --root, umask 077 (0700)
  launch                          # KEY=value: pid, pgid, identity, cmd, created
  identity                        # opaque identity token (start time of the pgid leader)
  stdout.log                      # child stdout, durable, unmerged
  stderr.log                      # child stderr, durable, unmerged
  terminal                        # written ONLY by the wrapper, ONLY on child exit; atomic
                                  #   kind=exit code=<n>   |   kind=signal signal=<n>
  stop-intent                     # written by --stop, immediately BEFORE the signal
  stopped                         # completed stop marker; written ONLY after termination verified
```

Every record write is a `mktemp` **beside** its destination followed by `mv -f`, so no reader ever
sees a half-written record. (That is the one licensed exception to templating temp files into
`TMPDIR`: the rename must be same-filesystem.)

**`launch` is written first, `identity` second, and the order carries meaning.** `launch` is what
makes the detached group **addressable** — from the instant it exists the launcher's failure path
can name the group and kill it. `identity` is the handshake's last conjunct and declares
establishment **complete**. So on an establishment that crashed between the two writes, the
`identity` file may be **absent** while `launch` is present; `launch` also carries the same value in
its own `identity=` field, and both readers accept either source. They cannot disagree — one
`identity_of` call produced both. Both writes precede the fork of the user's command, so the
ordering rule below holds regardless.

**The invariant every verb rests on:** the wrapper is the **only** writer of `terminal`, so a
`terminal` file visible *after* a liveness probe was necessarily written by a child that completed.
That is what makes `--observe`'s step-3 re-read and `--stop`'s steps 3 and 6 correct rather than
merely defensive.

## Behavior

### `--launch`

1. Parse arguments. Any argument error reports `launch-failed`.
2. `umask 077`; resolve or mint the root; **mint** the run dir beneath it. An existing run dir, an
   unwritable root, or a root that cannot be created are all `launch-failed`.
3. Spawn the **wrapper** detached (see *Per-platform capability note* for the primitive), with its
   stdin from `/dev/null` and its streams redirected into the run dir.
4. **The wrapper, in the child**, refuses to record at all unless it **leads its own process
   group** — every later verb signals the *recorded* pgid, so recording a group we do not lead
   would point a later `--stop` at a bystander, in the worst case the launcher itself. It then
   computes the identity token, writes `launch`, writes `identity`, and **only then** forks the
   user's command.
   - **ORDERING RULE (load-bearing):** the command is never started before the pid/pgid/identity
     record is durably in the run dir. A wedge before the record can strand plumbing at worst,
     never an unaddressable command process.
5. **The handshake.** The launcher waits, bounded by `GATE_RUN_ESTABLISH_SECS`, for `launch` **and**
   `identity` to both exist and be non-empty, and then verifies the recorded pgid is **not the
   launcher's own group** — a handle naming the caller's group would be reaped by the caller's own
   teardown. Only then does it print the run-dir path.
6. **The failure path is a bounded stop.** On timeout or a failed separation check: with a record
   present the recorded group gets `TERM`, a bounded grace, then `KILL`; with no record, only the
   direct spawn child is signalled by pid and the group is merely probed — a group kill there would
   quietly clean up after a violation of the ordering rule instead of leaving it observable.
   Anything left unverified is reported **loudly on stderr with the run-dir path for manual
   disposal**. Either way the token is `launch-failed`.

### The terminal record, and the wrapper's TERM disposition

The wrapper forks the command rather than `exec`ing it, because it must outlive the command to
write the record — that is what makes "the child completed" evidence rather than a claim. On the
command's exit it writes exactly one of:

```
kind=exit code=<n>
kind=signal signal=<n>
```

**The wrapper carries `trap '' TERM`.** No *handler* is installed — the disposition is set to
**ignore** — so there is still exactly **one** code path anywhere that can write `terminal`: the
`wait` returning the command's own status. That is the property "untrapped" was protecting, and it
is preserved. The ignore is necessary because every teardown in this contract is **group**-directed
and the wrapper leads the recorded group, so `kill -TERM -$pgid` reaches the wrapper too.
**MEASURED:** with the default disposition the wrapper died alongside the command and `terminal` was
never written, which made `kind=signal` unreachable and degraded every signal death to
`cause=vanished`. An ignored signal is inherited across fork and exec, so the command's subshell
**resets TERM to its default** before exec — the command must still be killable by the very signal
the wrapper survives. **SIGKILL is deliberately not survivable:** a KILLed group leaves no record,
which is exactly the `cause=vanished` reading it should get.

### `--observe`

The read order **is** the contract:

1. **Terminal record first — it always wins.** The child's own verdict outranks any probe of the
   group it used to lead.
2. **Liveness, identity-checked** — never a bare `kill -0`. The conjunction is: the recorded group
   exists **and** the process leading it started at the instant this run recorded. A pgid is a
   reusable name; without the second conjunct a recycled group reads as `running` and the caller
   waits out its whole budget on a run that is not there. Every leg fails **closed**.
3. **Dead or identity-mismatched ⇒ re-read the record.** Atomicity prevents partial reads, not stale
   ones: the step-1 read was a snapshot of a moment that has passed, and the child had every chance
   to finish since. Without this re-read the sequence "no record → child completes → dead probe"
   turns a run that **passed** into a `died`, and a call site keyed on `died` relaunches it.
4. A `stopped` marker with no record ⇒ `state=stopped`.
5. Otherwise the recorded group is gone and nothing survives that could ever write a verdict:
   `state=died cause=vanished`, with the last lines of the run's own streams on **stderr**.

The six states:

| State | Meaning | Retryable |
|---|---|---|
| `running` | the recorded group is alive and its leader's identity matches | **yes** |
| `passed` | `kind=exit code=0` — the child ran and finished green | no |
| `failed` | `kind=exit` with a non-zero code — the child ran and went red | no |
| `died cause=signal` | `kind=signal`, with no stop of ours recorded — killed by something else | no |
| `died cause=vanished` | the recorded group is gone and no record was ever written | no |
| `stopped` | a `--stop` of ours: `stopped` or `stop-intent` exists beside a `kind=signal` record, or `stopped` exists with no record | no |
| `unavailable` | no verdict could be read: an unreadable run dir, a malformed `terminal`, or a usage error | no |

**Only `running` is retryable.** The other five are terminal, and a caller that keeps polling past
one of them is polling a decided run.

A malformed `terminal` is `unavailable` rather than a guess: a record that does not parse says the
**supervisor** did not finish cleanly, which is not a verdict about the child, and a verdict read
out of garbage is fabricated.

### `--stop`

Seven steps, and two properties hold across all of them.

**The record outranks the stop** — steps 1, 3 and 6. A stop entered off a stale "no record" read
kills a run that had **already succeeded** and then reports it as terminated.

**Identity is checked before anything is signalled** — steps 2 and 4. The single bare `kill -0` in
the whole script is step 1's orphan probe, where the leader is known dead (so no identity match is
possible) and an alive result can only move the outcome **fail-closed**.

1. **Record present ⇒ probe the recorded group before reporting.** The leader is dead, so ownership
   of any survivor is unprovable — but *detection* is possible where safe signalling is not, and
   relaunching over live orphans is the double-run state this contract exists to avoid. Live
   members ⇒ `unavailable`, nothing signalled. Otherwise ⇒ `already-terminal`.
2. **Validate identity.** An **absent** group is not an identity failure — `unavailable` requires a
   group that exists and cannot be proven ours — so absence falls through to step 4's branch.
3. **Re-read the record immediately before signalling.** Nothing but the intent write and the kill
   separates the test from the act. See *Named residuals* for this step's honest testability status.
4. **Probe with both conjuncts, on the kill's side of the fence.** Neither `kill -0` nor the
   identity check is carried down from step 2: a group recycled inside the stop window is invisible
   to a value captured before the window, and that value is the one a `TERM` would be aimed at.
   - group absent ⇒ `already-terminal`, and **nothing is written**: no signal was sent, so there is
     nothing to claim, and a marker here would make a vanished death read `stopped`, which no caller
     ever relaunches.
   - alive but mismatched ⇒ `unavailable`, nothing signalled.
   - alive and ours ⇒ write `stop-intent` (claiming only that a signal is **imminent**), then
     `TERM` the group, wait out a bounded grace, then `KILL` the survivors.
   - **The KILL escalation deliberately does not re-check identity.** Ownership was proven a moment
     earlier at this same step, and this group has already taken our `TERM`; by then the **leader**
     is usually the first thing gone, so requiring a live matching leader would refuse to reap
     exactly the orphaned stragglers the escalation exists for. A recycle inside that window would
     additionally require the pgid to be freed and reassigned between two probes microseconds apart,
     which POSIX pgid-reuse rules forbid while any member of the group still exists. This is a
     stated, reasoned non-check, not an omission.
5. **Verify the group is gone.** A signal returns once it is queued, not once the group is reaped,
   and nothing may be claimed off a queued signal. Survival of both `TERM` and `KILL` ⇒
   `unavailable`, and **no marker is written**.
6. **Re-read after the kill and before any marker.** A `kind=signal` record found here is our own
   step-4 `TERM`, reaped and recorded by the wrapper: **annotate** it with the `stopped` marker, or a
   deliberately cancelled run reads `died` forever and an idempotent site relaunches a cancellation.
   A `kind=exit` record is the child's **own** verdict, reached despite the signal, and gets
   nothing. Either way the token is `already-terminal`.
7. Only now — the group having actually been signalled and verified gone, with no record of the
   child's own having appeared — write `stopped` and report `stopped`.

`--stop` writes `terminal` **never**, on any path. Synthesizing one would report a run that never
finished as one that did.

**Which leg produces which token — and the ordinary case is not the obvious one.** Stopping an
**ordinary live child** reports **`already-terminal`**, not `stopped`. The wrapper leads the recorded
group and survives the `TERM` (it ignores it), so it reaps the command, writes `terminal`, and
exits; step 5 cannot verify the group empty until that has happened, so by the time step 6 reads,
the record exists. `stopped` is produced on the **KILL escalation** leg — a command that ignores
`TERM` outlives the grace, the `KILL` takes the whole group including the wrapper, and no record can
exist — and on the narrow leg where a group vanishes to our `TERM` without the wrapper recording.
The tokens below are stated against the legs that actually produce them:

| Token | Produced when |
|---|---|
| `stopped` | the group was signalled, verified gone, and **no** terminal record exists — in practice the `KILL`-escalation leg |
| `already-terminal` | a record exists at step 1, 3 or 6 (the ordinary live-child stop lands here, annotated), **or** the recorded group was already absent at step 4 |
| `unavailable` | an unreadable run dir, an unusable record, live orphans under a record, an unprovable identity, or termination that could not be verified |

## Exit codes

**Callers key on the stdout report line, never on the exit code.** The mapping is documented here
for scripting completeness only, and it is deliberately coarse — a taxonomy of exit codes invites a
caller to branch on it, which is the coupling this section exists to discourage.

`--observe` and `--stop` agree on the same two-value mapping:

| Exit | Meaning |
|---|---|
| `0` | an outcome or verdict was determined — **including** `died`, `failed`, and `stopped`, which are answers, not errors |
| `1` | `unavailable`, and nothing else — no verdict could be determined |

Argument errors (a missing run dir, an unknown flag, more than one run dir, a `--reason` with no
value) report `unavailable` and exit `1`; there is no separate usage code, because a caller reading
the report line must not have to distinguish one.

`--launch` is the one verb with a different pair, because its payload is a handle rather than a
verdict: `0` with the absolute run-dir path on stdout, non-zero with the token `launch-failed`.

## Per-platform capability note

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
narrowing is carried as a design finding in *Named residuals* below.

## Named residuals

Each of these is a property this helper does **not** have. A residual that is not written down is
indistinguishable from a bug nobody has hit yet.

**1. The `129..192` shell floor conflates a high exit code with a signal death.** A POSIX shell sees
only `$?`, which renders "killed by signal 15" and a genuine `exit 143` identically. A code in
**129..192** is therefore recorded `kind=signal`, and the bias is chosen rather than incidental: the
two errors are **not** symmetric. Reading a signal death as `failed` mints integration-repair work
for tests that never ran, which this contract forbids outright; reading a genuine `exit 143` as
`died` costs one bounded relaunch that reproduces the same code and then halts. **No portable
POSIX-shell way exists to recover the true wait status** — the distinction is simply not in `$?`, and
no arrangement of shell builtins puts it there. A genuine fix needs a **non-shell helper** to read
the raw wait status, which is a new dependency and therefore a **human-gated decision**, not
something this helper may take on its own.

**2. A child that escaped the recorded group survives `--stop`.** Every teardown here is directed at
the pgid recorded at launch. A grandchild that double-forks, calls `setsid`, or otherwise leaves that
group is unreachable by the stop, will not be reaped, and — being outside the group — does not keep
the group alive either, so the stop still verifies and reports success. Signalling more broadly is
not the remedy: a signal aimed at a name we cannot prove is still ours can reach an unrelated
process group, which is both the worse failure and the unrecoverable one.

**3. An external signal landing after the stop-intent is recorded as deliberate.** `stop-intent` is
written immediately **before** the `TERM` and says only that a signal is imminent. If something else
kills the child inside that window, the resulting `kind=signal` record is read as `stopped` rather
than `died cause=signal`, because the intent file cannot distinguish our signal from a stranger's.
The bias is deliberate and matches residual 1's: over-reading a death as deliberate costs a run that
is not relaunched (and is visible as a `stopped` in the caller's report), while under-reading a
cancellation as `died` makes an idempotent call site relaunch a run a human deliberately cancelled.

**4. The macOS session-primitive narrowing.** On a platform at rung 3 this contract delivers own
process group plus the handshake, and no session — see *Per-platform capability note*. This is
carried as a **design finding for a human to accept or supersede**, and the supersede option is
already named: `/usr/bin/perl` is present on macOS and `POSIX::setsid` works. Notably,
`gate-execution-evidence.md`'s own four harness verdicts were measured with exactly that shape —
*"a `nohup`'d, fully-redirected, backgrounded helper that forks, calls `setsid(2)` in the child"* —
so accepting a perl dependency would close the gap on the same ground the evidence already stands on.
Docket's recorded policy is that it takes **no perl dependency**, and **only a human may change
that**; no task in this change may decide it.

**5. `--stop` step 3's pre-signal re-read reddens no assert, and is defense in depth rather than
distinct behavior.** Measured: step 3 and step 4's group-absent branch emit the **identical** token
(`already-terminal`) and write the **identical nothing**, so no observable outcome separates them,
and the window in which a record could arrive between the two is sub-scheduler-tick. Deleting step 3
therefore leaves every assert in `tests/test_gate_run_stop.sh` green. It is kept because the
cost is one `[ -f ]` and the shape it protects — read the record as late as possible before acting on
its absence — is the same shape steps 1 and 6 are built on. Stated honestly rather than claimed as
covered: the suite does not have a test for it, and it is not testable through the shipped surface.

## Invariants

- **stdout is the protocol and it is exactly one line wide.** Every verb prints one report line and
  nothing else; every diagnostic, including a failing run's log tail, goes to stderr. The trim is
  structural, applied once where the report is emitted, so no branch can widen the channel.
- **The wrapper is the only writer of `terminal`, and it writes it only on the command's exit.** No
  verb ever synthesizes one. This is the invariant every re-read in `--observe` and `--stop` rests
  on: a record visible after a liveness probe was necessarily written by a child that completed.
- **No liveness probe is a bare `kill -0`, with exactly one scoped exception** — `--stop` step 1's
  orphan probe, where the leader is known dead so no identity match is possible and an alive result
  can only move the outcome fail-closed. Everywhere else liveness is the conjunction of group
  existence and a matching identity token, and every leg fails closed.
- **A recorded pgid of `0` or `1` is refused, and so is the caller's own group.** `kill … -0` means
  the caller's own process group and `kill … -1` means everything the user can signal; as a probe
  each answers for a bystander, and as a signal each takes the caller or the machine down.
- **The command is never started before its record is durable** (the ordering rule), and the handle
  is never returned before establishment completes (the handshake). A run that got its address but
  never finished establishing is reported `launch-failed` and stopped, never handed back live.
- **Every record write is atomic** — `mktemp` beside the destination, then `mv -f`. A reader never
  sees a partial record.
- **`--stop` writes `stopped` only after termination is verified *and* only where a signal actually
  went out.** A marker written earlier lets a half-dead stop leave a false claim of termination that
  a later idempotent call no-ops on while the child runs; a marker written without a signal mints a
  `stopped` for a run that vanished on its own, which makes the vanished-death relaunch leg
  unreachable.
- **`--observe` and `--stop` are idempotent and mutate nothing a caller can observe** — except the
  stop's own `stop-intent` and `stopped` markers, which exist so a cancellation is read identically
  forever after.
- **The test hooks are env-gated and inert by default.** `GATE_RUN_TEST_WEDGE` is a one-way stall
  (fixtures arm it and let the launcher's own establishment timeout abandon the run — it models a
  crash window); `GATE_RUN_TEST_BARRIER` is a two-way rendezvous that announces arrival and waits for
  release (it makes an interleaving deterministic instead of sleep-tuned). It is the **point**
  variable that arms a barrier, matched by name, so arming one rendezvous never holds another. Both
  are bounded even when armed: a fixture that forgets to release must fail its own bounded wait and
  leave a red assert, never hang the suite.
- **The helper never polls for the caller.** Every verb is a short call; the wait loop and its budget
  belong to the call site, whose posture is stated in `skills/docket-build/SKILL.md`
  § *Gate execution posture*.

## Tests

`tests/test_gate_run.sh` (launch, the records, the terminal record, `--observe`, and this contract's
own shape) and `tests/test_gate_run_stop.sh` (`--stop` and its deterministic interleaving fixtures),
sharing the prologue in `tests/lib/gate_run_common.sh`. Every assert is keyed to a mutation that
reddens it, with the one exception named in *Named residuals* 5.
