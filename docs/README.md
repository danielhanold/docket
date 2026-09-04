# Docket documentation

Docket's documentation is organised into three tiers, each answering a different kind of question.
The **guide** answers *how do I do the thing* — one goal per page, end to end; **concepts** explain
*what each piece is and why* it is shaped the way it is; and **reference** lists the *exact fields,
keys, and owners*, each pointing at the surface that holds the current value.

## Start here

Follow the daily loop, then read the page for the step you are on:

[The daily loop](guide/daily-loop.md) → [Capturing work](guide/capturing-work.md) →
[Building without supervision](guide/building-without-supervision.md) →
[Landing changes](guide/landing-changes.md)

## Guide — how do I

Index: [guide/README.md](guide/README.md)

- [The daily loop](guide/daily-loop.md) — the handful of steps you run by name in a day of docket
  work, and which page covers each one in full.
- [Capturing work that outlives the session](guide/capturing-work.md) — turn an idea into a tracked
  unit of work that survives the session it occurred to you in, so you (or the autonomous loop) can
  pick it up weeks later without re-explaining it.
- [Designing before building](guide/designing-before-building.md) — take a half-formed stub through
  the step between capturing and building, until an autonomous run can implement it without guessing.
- [Building without supervision](guide/building-without-supervision.md) — hand a designed piece of
  work to an autonomous loop and get back an open pull request, and learn what it checks, how hard it
  works on each part, and where it stops and waits for you.
- [Proving the build](guide/proving-the-build.md) — how a finished branch earns the right to be
  reviewed and merged: the test run that certifies it and the durable record that run leaves behind.
- [Reviewing before the human does](guide/reviewing-before-the-human.md) — what happens to a finished
  branch between its last build commit and the pull request you read, and who touches it on the way.
- [Landing changes safely](guide/landing-changes.md) — how an approved change gets from an open pull
  request into your mainline and out of your backlog, hands-off across a whole set of changes.
- [Keeping the backlog honest](guide/keeping-the-backlog-honest.md) — tell whether your backlog still
  reflects reality, and fix it when it does not: the routine sweep versus the checks that flag a
  human.
- [Remembering why](guide/remembering-why.md) — where docket keeps the decisions it made and the
  lessons it learned, why they are kept apart, and how a lesson becomes a rule the tools always
  follow.
- [Governing through configuration](guide/governing-through-configuration.md) — change docket's
  behavior without editing its code: where a setting can live, which one wins, and how to rebind any
  step.
- [Running on your harness](guide/running-on-your-harness.md) — install docket on your machine, keep
  it current, and make each unit of work run at its own model and effort under whichever agent tool
  you use.
- [Where the metadata lives](guide/where-the-metadata-lives.md) — where docket keeps its planning
  records and why they sit apart from your code, across the two branches it uses.

## Concepts — what is it and why

Index: [concepts/README.md](concepts/README.md)

- [Two branches and the metadata worktree](concepts/two-branches.md) — why planning and code share
  one repository but never the same branch.
- [The change lifecycle as a state machine](concepts/change-lifecycle.md) — the states a unit of work
  moves through, and the events that move it.
- [Skills, agents, and harness dispatch](concepts/skills-agents-dispatch.md) — how one set of
  instructions runs on any vendor's tool, at the right model for the job.
- [Config layers and the coordination fence](concepts/config-layers.md) — four ordered config layers,
  and the fence that keeps a shared setting from being overridden.
- [Reconcile](concepts/reconcile.md) — the build-time check that kills stale work before a line of
  code is written.
- [Build profiles and the test gate](concepts/build-profiles-and-gate.md) — routing each task to a
  worker sized for its risk, then proving the whole suite green once.
- [The run gate and attribution](concepts/run-gate.md) — the bookkeeping that decides whether a
  launched build really finished and may be retried.
- [Finalize as a sequencer](concepts/finalize-sequencer.md) — close-out as an ordered chain of gated
  steps: rebase, retest, merge, archive.
- [Learnings and ADRs as memory](concepts/memory.md) — the two records docket keeps of why a decision
  was made and what a build taught.

## Reference — exact fields and owners

Index: [reference/README.md](reference/README.md)

- [`cli.md`](reference/cli.md) — the `docket` commands by noun, each pointing at its `--help` and the
  capability catalog for the current verbs and flags.
- [`fields.md`](reference/fields.md) — the change-manifest and ADR fields, owned by the
  `docket-convention` skill's sections.
- [`config-keys.md`](reference/config-keys.md) — every top-level config block by purpose, pointing at
  `.docket.example.yml` for shape, defaults, and layer scope.
- [`outcomes.md`](reference/outcomes.md) — dispositions, finalize reason tokens, and status health
  codes, each with its owning surface.
- [`skills-and-agents.md`](reference/skills-and-agents.md) — the skills and agents inventory, derived
  from the `skills/` and `agents/` directories.
- [`harness/README.md`](reference/harness/README.md) — the harness runbooks and example files: the
  live validation checklists and permission/sandbox examples behind the harness setup prose.
