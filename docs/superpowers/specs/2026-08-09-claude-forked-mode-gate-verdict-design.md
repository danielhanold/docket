<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0264 — Measure the claude harness's forked-mode gate verdict and pin a surviving launch shape](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0264-measure-the-claude-harness-s-forked-mode-gate-verdict-and-pi.md)**
<!-- docket:backlink:end -->

# Measure the claude harness's forked-mode gate verdict and pin a surviving launch shape — design

**Change:** 0264 · **Type:** docs · **Deliverables:** an updated claude row in
`skills/docket-build/references/gate-execution.md`, a forked-mode section in
`skills/docket-build/references/gate-execution-evidence.md`, and nothing else.

## Problem

`gate-execution.md` records the claude harness verdict as `supported — interactive session, two
foreground calls; forked mode unmeasured`. Docket's default execution path is exactly the unmeasured
mode: `docket-build` runs inside a dispatched/forked `docket-implement-next`, so every real gate on
this harness starts in a mode with no verdict and no known-surviving launch shape. Change 0200's
build hit this live: the reference's recommended mitigation, run literally from a dispatched call,
produced a gate that never started (inconclusive by the reference's own rule, not `incompatible`).

This change designs and performs the forked-mode probe and records the result. It is a
measurement-with-docs change: the "build" is running the probe protocol below and editing the two
reference files. **This spec designs the protocol and the recording rules; it does not perform the
measurement** — that happens at build time, because the verdict must be taken in the exact mode and
at the version current when the probe runs (learning: harness behavior is mode-and-version-scoped).

## Definitions

**Forked/dispatched mode**, for this probe, means: a subagent dispatched from a Claude Code session
(the same dispatch family `docket-implement-next` and the `docket-build-*` workers run in), issuing
the launch from its own Bash tool call. This is the mode docket actually runs, so it is the mode
whose verdict matters. The previously-attempted variant — a non-interactive `claude -p` child with
Bash granted — was already established unobtainable on this machine (permission classifier denial;
see the existing claude evidence section) and is explicitly out of scope: its absence is recorded
evidence, not a probe target.

**Probe fixture** — reuse change 0223's stand-in gate unchanged in its load-bearing properties:

1. emits progress, records that it started (a start marker file, written first),
2. sleeps 180s past any plausible turn/timeout boundary,
3. writes its terminal sentinel **last**,
4. the launch call's wall-clock duration is measured, and survival is credited only when the launch
   call exited before the sentinel was written,
5. the deciding observation happens **from outside the dispatched agent** — the parent session (or a
   later, separate call) reads the durable artifacts after the child has returned; the child's own
   report is never the evidence (learnings: capability-absence-needs-a-failed-attempt,
   yielded-worker-return-closes-every-door).

All probe scratch files live under a durable absolute path outside any worktree that could be torn
down (e.g. the session scratchpad or `${TMPDIR:-/tmp}` with an explicit template).

## Probe protocol — the one-variable-per-run ladder

Rung 0 exists because of a live hypothesis from the 0200 trigger: macOS ships no `setsid(1)`, so the
literal `nohup setsid <runner> … &` shape fails at exec with command-not-found — every stream
already redirected to /dev/null — which alone fully explains "the gate never started, no output file
was ever created." The ladder must separate "launcher never executed" from "harness killed the
child," or the whole probe repeats 0200's inconclusive run.

- **Rung 0 — launcher liveness (control).** From the dispatched agent, run the launch helper with a
  1-second stand-in gate and streams captured to a durable log. Confirms the helper itself executes
  in this mode before any survival claim is attempted. A failure here is a launcher bug, not a
  harness verdict.
- **Rung 1 — the standard race-free shape.** The exact shape that produced all four existing
  verdicts: a `nohup`'d, fully-redirected, backgrounded helper that forks, calls `POSIX::setsid(2)`
  in the child (fork mandatory — a process-group leader gets `EPERM`), reopens all three streams
  onto a durable log, and does not let the parent return until the child has finished detaching.
  Full 180s gate. The dispatched agent then **returns without waiting**; the parent observes the
  artifacts after the child's return and again after 180s+.
- **Rung 2 — disambiguation, only if rung 1 fails.** One variable at a time, mirroring the
  codex/cursor ladder: (a) racy detach (parent returns immediately after fork) — distinguishes a
  teardown race; (b) redirection kept, new-session removed — distinguishes whether the new session
  is the operative variable in this mode too. Run rung 2 only on a rung-1 failure; a surviving
  rung 1 pins the verdict and rung 2 adds nothing the reference needs.
- **Rung 3 — harness-native background launch (secondary, always run).** The 0200 gate ultimately
  completed via the harness's own background mechanism (Bash `run_in_background`). Measure it with
  the same fixture and the same outside-the-agent observation. If it survives, it is recorded as an
  additional named surviving shape for this harness — this evidence file is exactly the quarantine
  where product-specific mechanisms belong; the skill body and the capabilities stay
  harness-neutral and unchanged.
- **In-mode blocking observation.** Separately confirm that a dispatched agent can perform the
  posture-clause-4 discipline — launch, then block-observe the sentinel inside its own call budget —
  using a short (≤60s) gate. This is what a real `docket-build` worker does; it exercises
  capability 4's forked-mode analogue and is recorded as its own evidence line, scoped to what it
  measured.

Each rung is one dispatched agent per run (a fresh child per measurement — no rung reuses a child
whose teardown behavior a prior rung already triggered). The **ladder driver** — the agent that
dispatches each probe child, collects durations, and performs every deciding observation — is the
highest agent in the run that can resolve subagent dispatch, per the convention's
*Dispatch-capability resolution*; "the parent session" is the common case, not a guarantee. When
0264 is built autonomously, the plan places the driver at whatever tier resolves dispatch (the
implement-next agent itself, driving probe children directly rather than through a build worker),
with Tier C's authorized-or-halt posture as the backstop if no tier can dispatch.

## Recording rules

- **Verdict line** (`gate-execution.md`, claude section): rewrite the scope suffix from "forked mode
  unmeasured" to what the probe established, following the codex row's precedent of naming the
  surviving condition on the verdict line. Expected shapes:
  - rung 1 survives → the dispatched-mode scope is **added** to the verdict line, never merged with
    the interactive scope: the existing interactive suffix ("interactive session, two foreground
    calls" — the clause pinning capability 4 to that mode) is preserved verbatim, and the
    dispatched-mode clause is appended with its own launch-shape condition. If the CLI version at
    probe time differs from `2.1.223`, the dispatched clause carries its own version on the line
    (per the reference's "verdicts are version-scoped" and narrower-scope-on-the-line rules) — two
    separately-scoped measurements on one row, never one fused claim.
  - rung 1 fails, rung 3 survives → `supported` scoped to the native-background shape in dispatched
    mode, with the detaching shape recorded as not surviving that mode.
  - a gate that starts and is killed under every shape → `incompatible — dispatched mode` for the
    probed shapes, follow-up stub minted per the reference's own rule.
  - the gate never starts under any shape (including rung 0) → **inconclusive**: the verdict line
    keeps "forked mode unmeasured", and the evidence file gains a dated inconclusive-run record so
    the next prober doesn't repeat it. Never write `incompatible` from a never-started run.
- **Version scoping:** record `claude --version` at probe time in both files. If the version has
  moved past `2.1.223`, the forked-mode section states its own version and does **not** silently
  refresh the interactive row — the interactive verdict stays pinned to the version that measured
  it. Re-measuring interactive mode is out of scope.
- **Evidence file:** a new `#### forked/dispatched mode` block under the claude section — probe
  design deltas from the standard method (the dispatch vehicle, rung 0's rationale), measured launch
  durations per rung, and the narrative. The existing method section is not rewritten; the ladder
  above extends it.
- **No contract changes:** the six capabilities, the posture clauses in `docket-build/SKILL.md`,
  and the cursor/codex/opencode rows are untouched, per the stub's boundary.

## Out of scope

- Re-probing cursor, codex, opencode, or claude's interactive mode.
- The `claude -p` headless variant (established unobtainable; recorded, not retried).
- Any change to `docket-build/SKILL.md`, the capability list, or the observation-budget machinery.
- Capability 5's four-state distinction and capability 6 (unmeasured everywhere; stays so).

## Build shape

Single small change on a feature branch: run the ladder (the probe artifacts themselves are
scratch, not committed), then edit the two reference files. Tests: the repo suite must stay green;
no new tests — the deliverable is a measurement record, and no in-repo test can be its oracle
(learning: external-truth-needs-a-human-checkpoint) — the close-out carries a human verification
item naming the version pin and the re-probe trigger, mirroring change 0223's results file.

## Assumptions

1. **"Forked mode" is operationalized as a dispatched subagent inside a live Claude Code session,
   not a headless `claude -p` child.**
   Chosen because it is the mode docket actually runs (`docket-implement-next` and the build workers
   are session-dispatched agents) and because the headless variant is already recorded as
   unobtainable on this machine. Rejected: probing headless anyway (blocked by a measured classifier
   denial; retrying it is 0062-family territory — arguing with a guard instead of measuring the real
   path). Rejected: treating the interactive verdict as covering dispatch (the exact unscoped-
   inheritance error the learnings ledger warns about).
2. **The probe fixture is 0223's stand-in gate, unchanged.**
   Chosen for comparability — a verdict measured with a different fixture couldn't sit in the same
   table. Rejected: using the real `run-tests.sh` suite (couples the verdict to suite runtime and
   machine load; the stand-in's sentinel-last property is the load-bearing part).
3. **Rung 0 (launcher liveness control) is added ahead of the standard ladder.**
   Chosen because the 0200 trigger is fully explainable by macOS's missing `setsid(1)` with all
   streams discarded — a launcher exec failure, not a harness kill — and the reference's own rule
   makes a never-started run prove nothing. Without rung 0 the probe risks reproducing the same
   inconclusive outcome. Rejected: assuming the 0200 report already establishes a harness kill (it
   establishes nothing; one variable was never isolated).
4. **The harness-native background mechanism is measured (rung 3) and, if surviving, named in the
   evidence file as a surviving shape.**
   Chosen because 0200's gate actually completed through it, and the evidence file is the designated
   quarantine for product-specific mechanisms — the skill body stays harness-neutral either way.
   Rejected: excluding it as "not the standard shape" (it is the only shape with an existing
   survival observation in this mode); rejected: promoting it into the skill body or capabilities
   (contract change, out of the stub's boundary).
5. **A never-started outcome keeps the verdict at "forked mode unmeasured" with a dated
   inconclusive record; only a started-then-killed outcome across all shapes may write
   `incompatible`.**
   Direct application of the reference's own rule; the alternative (writing `incompatible` on any
   failure) would weaken the common contract from a probe that proved nothing.
6. **The interactive row is not refreshed even if the CLI version has moved.**
   Chosen to keep the change at its stated boundary; the forked-mode section carries its own version
   pin. Rejected: opportunistically re-measuring interactive mode (scope creep past the stub's
   boundary; verdicts are version-scoped observations, and the interactive one keeps its own).
7. **The groom performs no live measurement; the build does.**
   The verdict must be taken at the version current at probe time, in the exact mode, by the agent
   that records it — a groom-time measurement would be stale by build time and would also exceed
   grooming's markdown-only write contract in spirit (spawning probe processes).
8. **No new automated tests; a human verification item carries the version pin.**
   The measured fact lives outside the repo, so an in-repo assert could only ever pass
   (external-truth-needs-a-human-checkpoint). Mirrors 0223's close-out precedent.
9. **No dependencies or file couplings recorded.**
   A whole-active-set grep shows 0264 is the only change touching `gate-execution.md` /
   `gate-execution-evidence.md`; 0200 and 0223 are both done (archived). `discovered_from: [200]`
   already carries provenance; `depends_on`/`related` stay empty.
