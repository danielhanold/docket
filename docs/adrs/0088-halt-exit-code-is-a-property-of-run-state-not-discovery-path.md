---
id: 88
slug: halt-exit-code-is-a-property-of-run-state-not-discovery-path
title: "A halt's exit code is a property of the run's state, not of how the facade learned it"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [87]
change: 284
---

## Context

`runner-dispatch.sh --observe` reports a delegated run's outcome through exit codes. Before change
0284 the only way it learned a run had stopped was the child's own `done` sentinel;
`observe_implement_next` then read the run gate's verdict from `verify-run` and, on `run-halted`,
exited **`3`**.

`3` is load-bearing, not arbitrary. It is the code change 0271's synthesized-exit table pins
normatively for a halt under detachment, and it is the code the parent-facing run gate keys on to
decide **never to re-dispatch** — a halt means a human is needed, and a driver told `1` treats it as
an ordinary failure while a driver told `0` draws the next change.

Change 0284 added a second way to learn a run stopped: an identity-checked liveness probe. When the
child is found dead with no sentinel, git still decides the disposition (a delegated run can commit,
push and open its PR and *then* be killed before the wrapper's `mv -f` lands). So the same
`verify-run` verdict — including `run-halted` — is now reachable through a path where **no exit code
was ever read from the child**.

Change 0284's spec §3 tabulated that new path as `run-halted` ⇒ exit **`1`**. But the same table row
names `observe_implement_next` as the reader and says its wording is "preserved" — and that function
exits `3`. The spec row was therefore **internally inconsistent**, and the implementation had to
choose. (The spec's assumption 6, "no new exit code", is not in tension: `3` already exists in this
file; nothing new is minted.)

## Decision

A halt discovered through the liveness probe exits **`3`**, exactly as a halt discovered through the
sentinel does.

The reasoning is that the two paths differ in **how the facade learned** the run stopped, never in
**what the run's state is**. `verify-run` returning `run-halted` means the change carries a committed
`## Run halted` section and a human is needed — a fact about the run's own recorded state,
established from git, entirely independent of whether the child exited cleanly, was killed, or
vanished. Collapsing that into the generic failure code `1` because of the discovery route would tell
a driver "ordinary failure, carry on" about a run that has explicitly stopped for a human. That is
precisely the prose-level failure change 0237 exists to eliminate, reintroduced one seam over.

Stated as a rule: **an exit code that encodes a run's disposition is a function of the verdict, never
of the channel the verdict arrived through.** Where a new discovery path reaches an existing verdict,
it inherits that verdict's existing code.

The whole-branch reviewer was asked to form an independent view on this deviation and reached the
same conclusion.

## Consequences

- Enables: a driver's halt handling is uniform — it keys on `3` and never re-dispatches, regardless
  of whether the delegated agent died or exited cleanly. No caller needs to know which seam the run
  returned through.
- Enables: the same rule settles future discovery paths without re-litigating each one.
- Costs: change 0284's spec text is superseded on this row. Recorded here rather than by editing the
  spec, which is a point-in-time record.
- Costs: the `child-vanished` terminal marker must persist the decided exit code (it carries
  `disposition=`, replayed by the step-2 reader) so that a re-observation reproduces `3` without a
  second `verify-run` call. A marker written by an older build, carrying no `disposition`, replays
  fail-closed as `1` — the safe direction, but it means a dispatch disposed across an upgrade
  boundary can transition `3` → `1`.
- Requires: `scripts/runner-dispatch.md`'s `## Exit codes` / `### --observe` table is the single
  place a caller reads these codes, so it names `3` for both discovery paths.
