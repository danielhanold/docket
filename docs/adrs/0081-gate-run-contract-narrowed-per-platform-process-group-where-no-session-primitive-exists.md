---
id: 81
slug: gate-run-contract-narrowed-per-platform-process-group-where-no-session-primitive-exists
title: "gate-run's detachment contract is narrowed per platform: own process group where no session primitive exists"
status: Superseded by ADR-95
date: 2026-08-10
supersedes: []
reverses: []
relates_to: [80]
change: 282
---

## Context

Change 0282 ships `scripts/gate-run.sh`, whose `--launch` verb must start a long-running child that
survives the harness tearing down the call that started it. `docket-build`'s
`references/gate-execution.md` states capability 1 as a **new session**, not merely a new process
group, because on two of four measured harnesses a launch that satisfied "run it in the background"
returned success while the gate was already dead.

The spec (assumption 15) required the plan to establish which primitive delivers a genuine new
session on the supported platforms, and pre-authorized an honest narrowing plus escalation if none
could be found without taking a new dependency. Measured at plan time on darwin 25.6.0, one variable
per run:

- **`setsid(1)` is absent on macOS.** Already recorded in `runner-dispatch.md` and in
  `gate-execution-evidence.md`'s clause "*`setsid(1)` is not installed on macOS*".
- **`script(1)` — the platform-shipped pty candidate the spec named — is present but fails the
  required capability set**, on two independent grounds.
  `/usr/bin/script -q /dev/null /bin/bash -c 'echo alive' </dev/null` produced the bytes
  `^D \b \b a l i v e \r \n`: a typescript marker plus pty CRLF translation, so the durable log
  would not hold the child's own bytes. And a pty **merges stdout and stderr**, which the contract's
  stdout-is-the-protocol rule forbids. It is additionally fragile: with stdin on a socket it fails
  outright with `script: tcgetattr/ioctl: Operation not supported on socket`.
- **The ladder is therefore exhausted on macOS without taking a new dependency.**

## Decision

**The gate-run contract is honestly narrowed per platform. It delivers own-process-group detachment
plus the detachment handshake where no session primitive exists, and it never claims a new session
it does not deliver.**

`scripts/gate-run.sh` **probes at runtime — never by platform name**:

- Where `command -v setsid` succeeds, the wrapper is started under `setsid(1)` and the child gets a
  **genuine new session**.
- Where the ladder is exhausted, the wrapper is backgrounded under Bash job control (`set -m`),
  making it a **process-group leader**, with the detachment handshake unchanged. On such a platform
  the contract delivers own process group plus the handshake, and **nothing about a session**.
  `scripts/gate-run.md`'s *Per-platform capability note* records this.

ADR-0080 (change 0271) is the direct precedent: it measured the same `set -m` technique for the
delegation boundary and stated its limit in the terms this decision adopts — "*the child gets its
own process GROUP, not a new SESSION — it remains in the launcher's session, so session-scoped
teardown was not tested and is not claimed.*"

**The escalation this ADR carries, because it is a human's to settle.** `/usr/bin/perl` is present
on macOS and `POSIX::setsid` works — and `gate-execution-evidence.md`'s own four harness verdicts
were measured with exactly that shape ("*a `nohup`'d, fully-redirected, backgrounded helper that
forks, calls `setsid(2)` in the child*"). Accepting a perl dependency would close the gap and let
the contract claim a new session everywhere, on the same ground the evidence already stands on.
Docket's recorded policy is that it takes **no perl dependency**. This is presented as a **live
option a human may take to supersede the narrowing**, not as a settled rejection; the implementation
deliberately recorded and continued rather than stalling, and no task in change 0282 may decide it.

**Recorded as context, not as a second decision:** the launch wrapper carries `trap '' TERM`. It was
measured that an untrapped wrapper dies alongside its own child on a group-directed `TERM`, which
made the `kind=signal` terminal record unreachable and degraded every signal death to "no record at
all". No *handler* is installed — only the ignore disposition — so `wait` returning the command's
own status remains the single code path that writes the terminal record. Documented in
`scripts/gate-run.md`.

## Consequences

- On a no-primitive platform the gate survives teardown of the initiating call's **process group**
  (ADR-0080 measured this, one run with two arms and one variable changed), which is the property
  the gate-execution capabilities actually require. **Session-scoped teardown is untested and
  unclaimed** there.
- The narrowing is **per-platform and runtime-probed**, so a machine that has `setsid` gets the
  stronger session guarantee from the same code, with no platform branch to maintain and nothing to
  re-decide when a platform gains the primitive.
- The contract's stated capability is now a floor a reader can trust rather than an aspiration: a
  caller reasoning about teardown reads the *Per-platform capability note* instead of inferring a
  session that may not exist.
- The perl option stays open and is cheap to exercise later; superseding this ADR is the mechanism.
  Until then the residual is carried in `gate-run.md`'s *Named residuals*.
- **No harness verdict in `gate-execution.md` was rewritten or re-probed by change 0282** —
  capability 5 gained only a pointer to `gate-run.md`'s state vocabulary.
