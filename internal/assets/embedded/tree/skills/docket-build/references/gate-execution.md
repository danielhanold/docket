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
   *result unavailable*.** Four states, not two. This capability is mechanized
   **harness-independently**: the state vocabulary a caller keys on — and which of those states is
   retryable — is defined once in [`gate-caller-loop.md`](gate-caller-loop.md), which is
   why a per-harness capability list is the wrong owner for it. What a harness must supply is the
   ability to make the observations at all; it never defines states of its own.
6. **Enforce the observation budget without depending on a single long-lived foreground call.**

One mitigation satisfied all of them on every harness measured below: **detach into a new session
and redirect every stream to a durable location.** That is the same act that produces the durable
result artifact — one discipline, three payoffs. It carries one non-obvious precondition, measured
and recorded as evidence: the new session must be **fully established before the initiating
call returns**, or the harness's teardown wins the race. That precondition was measured, not
reasoned about; the measurement is in the evidence file linked at the end of this reference.
Docket ships that mitigation as the `gate.launch` operation, implemented in `internal/process/launch.go`:
it performs the detached launch, the durable unmerged streams, and the establishment handshake the
precondition names, so a call site satisfies these capabilities by using it rather than by
re-deriving a launch shape. **What its detachment delivers is uniform across every supported
platform** — ADR-0095 records that the native per-run supervisor establishes a genuine new session on
both Darwin and Linux, so the page carries no per-platform narrowing and claims the session guarantee
outright. The capability required above is met on every platform, not bounded by what a weaker
launch shape could provide.

## Reading a verdict

`supported` — measured, with the evidence and version recorded. `unverified` — not measured, or
measured inconclusively; treat as unknown, never as working. `incompatible` — measured and
established as unable to meet a required capability.

**A verdict covers only what the probe measured — capabilities 1, 2 and 3** (survival past the
initiating call; every stream redirected to a durable location; an unambiguous terminal sentinel) —
**plus any further capability its own section names.** It is not a claim about the other three: the
standard probe leaves them unmeasured, so read them as `unverified` in the sense above unless a
section records otherwise.

- **Capability 4** is *not* measured by the standard probe — the probe observes from **outside**
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

## Per-harness verdicts

### claude

`2.1.223 (Claude Code)`. Measured as two separate foreground calls of one **interactive** live
session, which is also the only row that establishes capability 4 — in that mode alone. Docket's own
default path is the **forked**/dispatched one, and that mode is **unmeasured** here; the verdict
claims nothing about it.

**Verdict:** `supported` — interactive session, two foreground calls; forked mode unmeasured

### cursor

`2026.08.04-aaa8809`, invoked with `--print --force --sandbox disabled`. Measured with the standard
race-free detaching launch shape; two further runs establish that this version does not block the
initiating call on an attached stream.

**Verdict:** `supported`

### codex

`codex-cli 0.146.1`, invoked with `exec --skip-git-repo-check --sandbox danger-full-access` (it
refuses to run outside a trusted directory otherwise). This is the harness that motivates capability
1's stronger reading: without a race-free new session the gate does not survive, so the verdict is
conditional on the launch shape rather than on the harness alone.

**Verdict:** `supported` — race-free new-session launch shape only

### opencode

`1.18.14`, invoked with `run`. Only the standard launch shape was probed, so whether opencode would
also kill an un-detached gate is unmeasured.

**Verdict:** `supported` — standard launch shape only; un-detached behavior unmeasured

## Change 0359 continuation/takeover acceptance — PENDING HUMAN RE-PROBE (pre-merge gate)

The four-harness continuation/takeover acceptance probes for change 0359 were carved out of the
autonomous build: they require driving four separately-installed harnesses and **must never be
fabricated**. They are recorded here as a required pre-merge human verification. The measured verdict
sections above are point-in-time records and are untouched by this gate.

### Pending rows

| Harness | Version | Paths to re-probe | Verdict |
| --- | --- | --- | --- |
| Claude Code | `2.1.251` | interactive AND forked/dispatched implement-next path | `unverified — 0359 re-probe pending (human, pre-merge)` |
| Cursor | `3.17.21` | registered named-agent + continuation dispatch | `unverified — 0359 re-probe pending (human, pre-merge)` |
| Codex | `0.150.1` | named dispatch, same-agent resume when available, fresh continuation fallback | `unverified — 0359 re-probe pending (human, pre-merge)` |
| OpenCode | `1.18.23` | named dispatch + continuation dispatch | `unverified — 0359 re-probe pending (human, pre-merge)` |

### The seven probe scenarios

Each harness must be observed against all seven:

1. a fast test that returns before 30 seconds;
2. a test spanning the first slice, with deterministic worker-to-controller handoff;
3. a worker that returns before handoff, followed by controller takeover of the same process;
4. an implement-next controller that returns, followed by top-parent continuation of the same
   process and same gate key;
5. no duplicate process, no new task, and no retry consumption while the drive is active;
6. terminal pass and terminal failure consumed by the correct resumed role;
7. explicit resume of an already-in-progress change remaining attributable.

### Standing rules

- A verdict is version-scoped: re-probe when the version moves; never inherit a row on faith.
- An interactive-only observation cannot stand in for a dispatched path.
- A harness that cannot supply the direct-child return event or an explicit continuation is
  unsupported on that path, and the gap is reported, never bridged with a timer.

This section is the outstanding merge-gate verification for change 0359: a human runs these probes
and records the evidence here before the PR merges. Do not merge on pending rows, and never write
probe evidence that was not observed.

## Evidence

How each verdict above was obtained — the probe design, the one-variable-per-run ladder, the
measured launch durations, and the per-harness narratives — is in
[`gate-execution-evidence.md`](gate-execution-evidence.md). That file is **not read before a gate
run**: an agent about to start a suite needs the capabilities, the mitigation, and the verdicts on
this page, and nothing on that one. Read it when re-probing a harness whose version has moved, or
when judging what a verdict is worth.
