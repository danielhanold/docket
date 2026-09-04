# Concepts

What each piece of docket is, and why it is shaped the way it is. Reach for
these when the how-to guide says a mechanism runs deeper than a task needs.
Every page follows the same four sections: the problem it solves, the moving
parts, the invariants, and the decisions behind it.

- [Two branches and the metadata worktree](./two-branches.md) — why planning
  and code share one repository but never the same branch.
- [The change lifecycle as a state machine](./change-lifecycle.md) — the states
  a unit of work moves through, and the events that move it.
- [Skills, agents, and harness dispatch](./skills-agents-dispatch.md) — how one
  set of instructions runs on any vendor's tool, at the right model for the job.
- [Config layers and the coordination fence](./config-layers.md) — four ordered
  config layers, and the fence that keeps a shared setting from being overridden.
- [Reconcile](./reconcile.md) — the build-time check that kills stale work
  before a line of code is written.
- [Build profiles and the test gate](./build-profiles-and-gate.md) — routing
  each task to a worker sized for its risk, then proving the whole suite green
  once.
- [The run gate and attribution](./run-gate.md) — the bookkeeping that decides
  whether a launched build really finished and may be retried.
- [Finalize as a sequencer](./finalize-sequencer.md) — close-out as an ordered
  chain of gated steps: rebase, retest, merge, archive.
- [Learnings and ADRs as memory](./memory.md) — the two records docket keeps of
  why a decision was made and what a build taught.
