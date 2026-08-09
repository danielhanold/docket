---
id: 80
slug: detached-delegation-execution-posture-launch-then-observe
title: "Detached delegation execution posture — launch-then-observe"
status: Accepted
date: 2026-08-09
supersedes: []
reverses: []
relates_to: [38]
change: 271
---

## Context

Runner delegation bounded an entire delegated agent run inside ONE foreground call of the parent
harness, hard-capped at 600000 ms — the Claude Code Bash tool's maximum, not a tunable. Any
delegated task that outlasted the window was killed mid-work, and the kill was indistinguishable
from failure. Observed on change 0258: `runner-dispatch` exited 143 after roughly ten minutes, with
+64 lines of already-mutation-proved work stranded UNCOMMITTED in the feature worktree.

Three layers hard-coded that posture: the generated shim wrapper ("Make exactly ONE foreground Bash
call … never background it, never poll"), the facade (`scripts/runner-dispatch.sh` calling the
adapter synchronously), and each per-runner adapter.

Change 0223 had already solved the same shape one layer up for `docket-build`'s suite gate — six
required capabilities, quarantined in `skills/docket-build/references/gate-execution.md` — and change
0249 propagated it to the worker contract. Neither binds the DELEGATION BOUNDARY itself: a worker
that perfectly obeys 0249 still dies when the shim's window closes, because the whole worker is
already inside that window.

## Decision

Two parts.

**1. Detachment mechanism.** The facade launches the adapter in its OWN PROCESS GROUP using Bash job
control — `set -m` makes a background job a process-group leader — with every stream redirected into
a durable per-dispatch directory under `<git-common-dir>/docket/dispatch/<key>/` and stdin closed.
`setsid` was rejected because it is ABSENT on macOS; a `perl`-based `setsid` was rejected to avoid a
new runtime dependency.

This satisfies capability 1's stronger reading — survival of the harness's teardown of the
initiating call's PROCESS GROUP, not merely its parent's exit. MEASURED, one run with two arms and
one variable changed (2026-08-09, darwin 25.6.0, GNU Bash 5.x): a launcher started two children, one
under `set -m` and one not, then the launcher's whole process group received TERM; the `set -m`
child (own PGID) SURVIVED, the non-`set -m` child (launcher's PGID) was KILLED. Stated honestly: the
child gets its own process GROUP, not a new SESSION — it remains in the launcher's session, so
session-scoped teardown was not tested and is not claimed.

**2. Launch-then-observe, and a non-failure exit code.** The facade grows two verbs: `--launch`
(detaches, returns immediately with a dispatch key) and `--observe <key>` (one short, idempotent look
at a sentinel file plus repo git state).

- **Liveness** comes from the sentinel, written by the detachment WRAPPER as its last act — never by
  the delegated agent, since a sentinel written by the party being judged would make "done" a claim
  rather than evidence.
- **Correctness** comes from git via `verify-run.sh`, which stays a PURE READER. A sentinel claiming
  success with no matching git evidence is a FAILURE: correctness wins.
- `--observe` returns `0` complete / `1` failed-or-unavailable / `3` halted / **`4` still running**.
  Exit `4` is a NEW CALLER-VISIBLE NON-FAILURE code — exactly the hazard
  `LEARNINGS: exit-code-encodes-a-non-failure` names — so its only consumer, the generated shim
  wrapper, was changed in the same change to loop on `4` and abort-and-report on any other non-zero.

ADR-0038's chokepoint property survives: two verbs, still exactly ONE dispatch seam, no inline
fallback, no silent retry.

## Consequences

- A delegated run is no longer bounded by any harness's foreground ceiling.
- Observation is bounded instead by a new config key `delegation_observation_budget` (default 60
  minutes, sibling of `gate_observation_budget`). On exhaustion the facade terminates the whole
  detached process group before reporting failure, so no unwatched agent keeps working after its run
  was declared failed (honoring change 0231).
- Per-dispatch directories accumulate under `.git`; mitigated by a 7-day retention prune at launch.
- The caller must now run a polling loop rather than one blocking call.
- The per-harness verdicts for the ADAPTER launch shape ship as `unverified` in
  `skills/docket-build/references/delegation-execution.md`. The gate-execution verdicts were measured
  for a GATE launch and are explicitly version- and scope-scoped, so they do not transfer. Re-probing
  needs each child CLI installed and authenticated.
