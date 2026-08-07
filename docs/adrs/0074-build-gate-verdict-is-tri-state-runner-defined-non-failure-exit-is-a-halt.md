---
id: 74
slug: build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt
title: The build gate's verdict is tri-state — a runner-defined non-failure exit is a halt
status: Accepted
date: 2026-08-07
supersedes: []
reverses: []
relates_to: []
change: 224
---

## Context

`skills/docket-build/SKILL.md` § *The build gate* stated what green and red MEAN but never what
DETERMINES which a run is. Change 0224 closed that gap with `green if and only if the resolved suite
command exits zero`. Its design spec
(`docs/superpowers/specs/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-design.md`,
assumption 4) deliberately refused an exit-code taxonomy and explicitly ACCEPTED a residual: under
`green iff zero`, `scripts/run-tests.sh`'s exit 3 — a harness failure that certified nothing — would
read as red and mint an integration-repair task against zero failing assertions. The spec called
that "fail-closed and identical to today's behavior".

Whole-branch review found that residual is not benign. `scripts/run-tests.md` documents exit 3 as
"every target that produced a result passed, but at least one produced no result at all" and warns
that "a caller that answers 1 by dispatching a repair agent to root-cause failing tests would find
none"; exit 4 is "the suite is green but something got slow". In this repo
`finalize.test_command: scripts/run-tests.sh` and finalize's `configured-bash-finalize` block
propagates that exit code verbatim, so the gate's only remaining branch was **Red → turn the failure
into exactly one synthetic integration-repair task**. That is the identical "manufacture a repair
task" harm the adjacent configuration-gap carve-out already exists to refuse — reachable from a run
with zero failing tests.

## Decision

The build gate's verdict space is **three-valued**, and the contract says so **without naming a
single exit code**:

- **Green** — a completed run whose recorded status is zero.
- **Halt** — a completed run whose recorded non-zero status **the resolved runner defines as a
  non-failure outcome**; it is refused under *Halting conditions*, the same refusal the
  configuration gap gets.
- **Red** — a completed run that is neither.

*still running* and *result unavailable* are not verdicts at all; they remain budget halts.

The no-taxonomy posture is preserved deliberately: an enumerated list of exit-code meanings is the
exact anti-pattern change 0224 exists to close one level up (`AGENTS.md`: key a guard on shape,
never an enumerated list of spellings). So the gate still reads a **bare non-zero** and **delegates**
the non-failure judgment to the resolved runner's own documented contract, rather than learning a
taxonomy itself.

## Consequences

**Enables.** A runner may signal a non-failure condition with a non-zero exit without triggering a
spurious repair chain. The gate's four-state capability requirement in
`skills/docket-build/references/gate-execution.md` (capability 5) finally has a definition of
*completed successfully*.

**Costs.** The gate's correctness now depends on the resolved runner documenting which of its
non-zero exits are non-failures — an obligation on runner contracts that docket does not enforce
mechanically.

**Gives up.** The simpler binary reading of the gate, and the spec's original accepted residual.

**Relates to.** The `docket:build-evidence` record (change 0190, still open) — its schema was
deliberately NOT changed here; and `scripts/run-tests.md`'s exit-code table, which already defers
exit-code semantics to change 0224.
