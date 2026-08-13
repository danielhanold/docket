# Gate execution — probe evidence

**This file is evidence, not instruction, and is not read before a gate run.** The rules an agent
needs at gate time — the six required capabilities, the mitigation, and each harness's verdict —
are in [`gate-execution.md`](gate-execution.md). This file records how those verdicts were
obtained, so a reader can judge what they are worth and re-probe when a version moves.

The same version scoping and re-probe caveats are carried as human verification items in
`docs/results/2026-08-07-the-build-gate-contract-never-states-an-execution-posture-for-results.md`,
the close-out record of change 0223, which produced these measurements.

## Method

Every verdict was measured on macOS 26.6.1 (arm64) with a stand-in gate that emits progress,
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

That last clause is the operative variable — the non-obvious precondition the kept surface states
as instruction — and it was established by a one-variable-per-run ladder rather than guessed:

- Same shape but with the parent returning immediately after the fork (a race): on Codex the gate
  **never started** — the child was killed within milliseconds, before `setsid(2)` could complete.
- Same shape with the new-session detach removed but the redirection kept: on Codex and on cursor
  the gate **started, then was killed mid-run**. Both harnesses reported success.
- Redirection additionally removed (streams left attached): on cursor, identical outcome — gate
  killed mid-run, and the launch call did **not** block (20s). This is the run that failed to
  reproduce the design spec's blocking claim, which is why capability 2 rests on durability alone.

So on both Codex and cursor the operative variable is the **new session**, not the redirection:
redirection alone does not save the gate, and a race-free new session does.

## Per-harness evidence

### claude — measured

`2.1.223 (Claude Code)`. Launch call returned in **0s**; the gate ran its full 180s and wrote the
sentinel, observed in a later, separate foreground call. Launch exited before the gate's terminal
write, and the gate ran as the leader of its own session, detached from the launching call.

The launch and the observation were performed as two separate foreground calls of one **interactive
live session**, so this row additionally measures capability 4 — in that mode, and only there.
Docket's own default path is not that mode: the gate runs inside `docket-build`, which is invoked
inline by the forked `docket-implement-next`, and a forked or dispatched agent has no channel on
which a resumption signal can arrive. On change 0223 itself, three dispatched build workers
backgrounded the suite and yielded, and none was resumed by the completion event. That is why
`docket-build` § *Gate execution posture* clause 4 grants the yield to a top-level session agent
only and requires a dispatched child to observe by blocking.

The stricter variant that would have measured that mode — a non-interactive `claude -p` child
observed from a shell outside it — was **not obtainable on this machine**: the permission classifier
denied granting the child process Bash access (both `--allowedTools Bash` and the bypass flag),
while a plain `claude -p` with no tool grant runs fine. That denial is why the forked mode is
unmeasured.

### cursor — measured

`2026.08.04-aaa8809`, invoked with `--print --force --sandbox disabled`. Launch call returned in
**19s**; the gate ran its full 180s and wrote the sentinel; launch exited before the gate's terminal
write.

Two disambiguating runs: without a new session the gate is started and then killed mid-run whether
streams are redirected (17s launch) or left attached (20s launch). The attached-stream run is what
establishes that this version does **not** block the initiating call on an attached stream,
correcting the claim inherited from the design spec. This verdict supersedes the spec's `2026.01.23`
row, which was not re-usable.

### codex — measured

`codex-cli 0.146.1`, invoked with `exec --skip-git-repo-check --sandbox danger-full-access` (it
refuses to run outside a trusted directory otherwise). Launch call returned in **11s**; the gate ran
its full 180s and wrote the sentinel; launch exited before the gate's terminal write.

This is the harness that motivates capability 1's stronger reading. Codex runs the command under
`/bin/zsh -lc` and tears down that call's process group on return, reporting `succeeded in 0ms`
either way. With the racy detach the gate never started at all; with no detach it started and was
killed mid-run. Only the race-free new session survives.

### opencode — measured

`1.18.14`, invoked with `run`. Launch call returned in **5s** — the fastest of the four; the gate ran
its full 180s and wrote the sentinel; launch exited before the gate's terminal write. No
disambiguating run was needed: the standard shape succeeded first time, so nothing here establishes
whether opencode would also kill an un-detached gate the way Codex and cursor do.
