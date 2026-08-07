# Gate execution — required capabilities and per-harness evidence

Reference for `docket-build` § *Gate execution posture*. The skill body states the posture by
capability and stays harness-neutral; every product-specific name, setting, and observed figure is
quarantined here. That quarantine is what lets the rule stay actionable without the contract naming
a tool.

## The six required capabilities

These are required **capabilities**, not required mechanisms — each harness may satisfy them
differently.

1. **Start a gate whose execution continues beyond the lifetime or timeout of the foreground call
   that initiated it** — including the harness's teardown of that call's **process group**, not
   merely the exit of its immediate parent. The weaker reading is not sufficient: on two of the four
   harnesses below, a launch that satisfied "run it in the background" returned success while the
   gate was already dead.
2. **Preserve gate output in a durable location**, with **every stream redirected away from the
   initiating call.** Output left attached to the initiating call is discarded when that call is
   torn down, and durability is what makes a terminal result readable afterwards. (The design spec
   additionally claimed an attached stream *blocks* the initiating call on at least one supported
   harness. That was probed directly and **not reproduced** at the versions below — see the cursor
   evidence. The redirection requirement stands on durability alone.)
3. **Record an unambiguous terminal result** — a state a later look can distinguish from partial
   output.
4. **Perform subsequent short-lived observations of that result.**
5. **Distinguish *still running* from *completed successfully*, *completed unsuccessfully*, and
   *result unavailable*.** Four states, not two.
6. **Enforce the observation budget without depending on a single long-lived foreground call.**

One mitigation satisfied all of them on every harness measured below: **detach into a new session
and redirect every stream to a durable location.** That is the same act that produces the durable
result artifact — one discipline, three payoffs. It carries one non-obvious precondition, measured
and recorded under *Method* below: the new session must be **fully established before the initiating
call returns**, or the harness's teardown wins the race.

## Reading a verdict

`supported` — measured, with the evidence and version recorded. `unverified` — not measured, or
measured inconclusively; treat as unknown, never as working. `incompatible` — measured and
established as unable to meet a required capability.

**A verdict covers only what § *Method* measured — capabilities 1, 2 and 3** (survival past the
initiating call; every stream redirected to a durable location; an unambiguous terminal sentinel) —
**plus any further capability its own section names.** It is not a claim about the other three: the
standard probe leaves them unmeasured, so read them as `unverified` in the sense above unless a
section records otherwise.

- **Capability 4** is *not* measured by the standard probe — § *Method* observes from **outside**
  the harness after it exited, which establishes that the result is durable, not that the harness
  can look again. Credit it only where a section records a re-observation the harness itself
  performed.
- **Capability 5**'s four-state distinction was never produced: the stand-in gate always succeeds,
  so *completed unsuccessfully* and *result unavailable* went unobserved on every harness.
- **Capability 6** was not probed at all.

Where a row's evidence is narrower still — a mode, launch shape, or variant left unmeasured — that
limit belongs **on its verdict line**, written as ` — <scope>` after the token. A bare token claims
no more than the bound above.

Verdicts are **version-scoped**. A verdict is an observation about the version named in its section,
not a property of the product. Re-probe when the version moves; never inherit a row on faith. Docket
does not weaken the common contract to preserve nominal support for a harness that cannot meet it —
an `incompatible` finding is recorded with its evidence and a follow-up stub is minted.

A probe that changes two variables at once proves nothing about either, and a run in which the gate
never started is **inconclusive**, not `incompatible`.

## Method

Every verdict below was measured on macOS 26.6.1 (arm64) with a stand-in gate that emits progress,
records that it started, sleeps 180s past any plausible turn boundary, and writes its terminal
sentinel **last**. Three properties of the probe are load-bearing, each having caught a false
verdict:

- **The sentinel is written last.** A gate that records its result first proves nothing about
  survival.
- **Observation happens from outside the harness**, after the harness process has exited. The
  harness's own report is never the evidence: on the failing runs below, every harness reported the
  launch as succeeding.
- **The launch call's duration is measured**, not only whether the artifact appears. A launch that
  blocks for the gate's full duration is a contract failure even though every artifact assertion
  passes. Survival is credited only when the launch call exited *before* the gate wrote its
  sentinel — a fixed threshold would not distinguish survival from a gate that merely finished
  first.

**`setsid(1)` is not installed on macOS**, so the launch shape cannot use it. The equivalent is
`POSIX::setsid(2)` called after a `fork` — the fork is mandatory, because a process-group leader
gets `EPERM`, which is the exact failure that made two of grooming's Codex runs inconclusive. All
four verdicts were measured with one identical launch shape: a `nohup`'d, fully-redirected,
backgrounded helper that forks, calls `setsid(2)` in the child, reopens all three streams onto a
durable log, and **does not let the parent return until the child has finished detaching**.

That last clause is the operative variable, established by a one-variable-per-run ladder rather than
guessed:

- Same shape but with the parent returning immediately after the fork (a race): on Codex the gate
  **never started** — the child was killed within milliseconds, before `setsid(2)` could complete.
- Same shape with the new-session detach removed but the redirection kept: on Codex and on cursor
  the gate **started, then was killed mid-run**. Both harnesses reported success.
- Redirection additionally removed (streams left attached): on cursor, identical outcome — gate
  killed mid-run, and the launch call did **not** block (20s). This is the run that failed to
  reproduce the spec's blocking claim.

So on both Codex and cursor the operative variable is the **new session**, not the redirection:
redirection alone does not save the gate, and a race-free new session does.

### claude

`2.1.223 (Claude Code)`. Launch call returned in **0s**; the gate ran its full 180s and wrote the
sentinel, observed in a later, separate foreground call. Launch exited before the gate's terminal
write, and the gate ran as the leader of its own session, detached from the launching call.

Evidence grade, and the mode it is scoped to: the launch and the observation were performed as two
separate foreground calls of one **interactive live session**, so this row additionally measures
capability 4 — in that mode, and only there. Docket's own default path is **not** that mode: the
gate runs inside `docket-build`, which is invoked inline by the **forked** `docket-implement-next`,
and a forked or dispatched agent has no channel on which a resumption signal can arrive. That is why
`docket-build` § *Gate execution posture* clause 4 grants the yield to a top-level session agent
only and requires a dispatched child to observe by blocking — on this change, three dispatched build
workers backgrounded the suite and yielded, and none was resumed by the completion event. The
stricter variant that would have measured it — a non-interactive `claude -p` child observed from a
shell outside it — was **not** obtainable on this machine: the permission classifier denied granting
the child process Bash access (both `--allowedTools Bash` and the bypass flag), while a plain
`claude -p` with no tool grant runs fine. The forked/dispatched mode is therefore **unmeasured**, and
the verdict says nothing about it.

**Verdict:** `supported` — interactive session, two foreground calls; forked mode unmeasured

### cursor

`2026.08.04-aaa8809`, invoked with `--print --force --sandbox disabled`. Launch call returned in
**19s**; the gate ran its full 180s and wrote the sentinel; launch exited before the gate's terminal
write.

Two disambiguating runs are recorded under *Method*: without a new session the gate is started and
then killed mid-run whether streams are redirected (17s launch) or left attached (20s launch). The
attached-stream run is what establishes that this version does **not** block the initiating call on
an attached stream, correcting the inherited claim. Note this verdict supersedes the design spec's
`2026.01.23` row, which was not re-usable.

**Verdict:** `supported`

### codex

`codex-cli 0.146.1`, invoked with `exec --skip-git-repo-check --sandbox danger-full-access` (it
refuses to run outside a trusted directory otherwise). Launch call returned in **11s**; the gate ran
its full 180s and wrote the sentinel; launch exited before the gate's terminal write.

This is the harness that motivates capability 1's stronger reading. Codex runs the command under
`/bin/zsh -lc` and tears down that call's process group on return, reporting `succeeded in 0ms`
either way. With the racy detach the gate never started at all; with no detach it started and was
killed mid-run. Only the race-free new session survives — so a `supported` verdict here is
conditional on the launch shape, not on the harness alone.

**Verdict:** `supported`

### opencode

`1.18.14`, invoked with `run`. Launch call returned in **5s** — the fastest of the four; the gate ran
its full 180s and wrote the sentinel; launch exited before the gate's terminal write. No
disambiguating run was needed: the standard shape succeeded first time.

Only the standard shape was probed here, so this section does not establish whether opencode would
also kill an un-detached gate the way Codex and cursor do. That is unmeasured, and the verdict claims
nothing about it.

**Verdict:** `supported` — standard launch shape only; un-detached behavior unmeasured
