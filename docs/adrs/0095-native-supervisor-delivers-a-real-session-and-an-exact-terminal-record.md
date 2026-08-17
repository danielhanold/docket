---
id: 95
slug: native-supervisor-delivers-a-real-session-and-an-exact-terminal-record
title: "The native per-run supervisor delivers a genuine session and an exact terminal record on every supported platform"
status: Accepted
date: 2026-08-17
supersedes: [81]
reverses: []
relates_to: [80, 87]
change: 314
---

## Context

ADR-0081 narrowed the gate-run contract per platform because no session primitive was reachable from
Bash on macOS: `setsid(1)` is absent there, `script(1)` fails the capability set (typescript markers,
pty CRLF translation, merged stdout/stderr), and the only remaining route — `POSIX::setsid` under
`/usr/bin/perl` — would have taken a dependency docket's recorded policy refuses. The honest outcome
was a floor: own process group plus the detachment handshake where the ladder was exhausted, and
nothing claimed about a session. ADR-0081 named superseding itself as the mechanism by which that
narrowing would be lifted, and left the choice to a human.

A second defect sat alongside the first and had the same root cause — the supervisor was Bash. A
shell reports a child's fate through `$?`, which folds a signal death into `128+N`. That encoding is
ambiguous by construction: a command that genuinely exits `137` is indistinguishable from one killed
by `SIGKILL`, so the terminal record could not state exit-versus-signal exactly. Both problems were
consequences of the supervising process being a shell rather than a program that can call the kernel
directly.

Change 0314 moves the supervisor into Go. The `docket` binary already exists and is already on the
path every gate run takes, so the same binary can re-exec itself as the per-run supervisor: no
interpreter has to be discovered, no second executable has to be shipped or located, and no daemon
has to be started or kept alive. From Go, `setsid` is a syscall on both Darwin and Linux, and the
child's raw wait status is available directly rather than through a shell's lossy `$?`.

## Decision

**The per-run supervisor is the `docket` binary re-exec'd as a supervisor, and it establishes a
genuine new session with `setsid` on both Darwin and Linux.** The per-platform narrowing ADR-0081
recorded is therefore lifted rather than re-stated: the gate-run contract claims a new session
everywhere it is supported, from one code path, with no platform branch and no runtime capability
probe standing in for a guarantee.

**The supervisor reads the raw child wait status directly and writes an exact terminal record** —
`kind=exit` with the true exit code, or `kind=signal` with the signal number — with no `128+N`
folding and therefore no ambiguity between a real exit code in that range and a signal death.

Two properties bound the supervisor's behavior:

- **It holds the run's live lock until the terminal record is durable**, and the record is written
  atomically. A reader that observes the lock released is guaranteed to find an exact terminal
  record, so "still running" and "finished, here is exactly how" are the only two states an observer
  can see — never a gap between them.
- **It signals only ownership-proven process groups.** The identity check that gates every signal is
  unchanged in kind; a session of its own simply makes the group it signals unambiguously its own.

**No discovered interpreter, no second executable, no daemon.** This is the property that makes the
supersession available at all: ADR-0081's ladder was exhausted only under the constraint of
supervising from Bash, and the perl escalation it carried is now moot — the guarantee is obtained
without it, so the dependency policy stands untouched.

**What this decision does not touch.** ADR-0080 governs the separate delegated-agent boundary and is
unchanged — its launch-then-observe posture and its own measured limits are a different boundary
with a different contract. ADR-0087's distinction between clean absence and unprovable liveness
remains applicable in full: the supervisor's observe, stop, and recover paths preserve it, and a
non-zero liveness answer is still not evidence of death. Neither ADR is superseded by this one.

## Consequences

- The gate-run contract's session guarantee is now uniform across supported platforms, so a caller
  reasoning about teardown reads one statement instead of a per-platform capability note. ADR-0081's
  *Per-platform capability note* and its named perl residual both retire with it.
- A terminal record is exact, which makes the exit-versus-signal distinction a fact a reader can act
  on rather than an inference from an encoding. Anything that previously had to hedge around
  `128+N` can stop hedging.
- The lock-until-durable ordering removes the observable window in which a run had ended but its
  record had not landed. Observers get a two-state answer, which is what makes recovery decidable.
- The cost is that the supervisor is now compiled code inside the `docket` binary rather than a
  shell script: changing it requires a rebuild, and its behavior is less inspectable by reading a
  file in the repo. That is accepted deliberately — the two guarantees above are reachable only from
  a process that can call the kernel and read a wait status directly.
- Re-exec'ing the same binary keeps deployment a non-issue: there is nothing extra to install,
  locate, or version-match against the caller, which is precisely why this route was available where
  the Bash-supervised ladder was not.
