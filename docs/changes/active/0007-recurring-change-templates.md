---
id: 7
slug: recurring-change-templates
title: Recurring change templates — scheduled maintenance work that spawns proposed instances
status: proposed
priority: medium
created: 2026-06-11
updated: 2026-06-11
depends_on: []
related: []
adrs: []
spec:
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
type: feat
---

## Why

Synthesized from the AgentRQ competitive review (2026-06-11). AgentRQ models recurring work as
cron-template tasks: a template row with a 5-field cron schedule spawns child task instances when
due, with a double-spawn guard (no new instance while a previous one is still open) and one-time
templates that self-delete after spawning. It's a small mechanism with high leverage for
maintenance work.

docket has no notion of recurring work. Dependency bumps, security audits, doc refreshes, license
sweeps — the kinds of changes that should re-enter the backlog on a cadence — must be re-proposed
by hand each time. A git-native translation is straightforward because docket already has a
periodic tick: every `docket-status` run (and the Step-0 status invocation of every
`docket-implement-next` run) is an opportunity to evaluate schedules and spawn due instances. No
daemon, no scheduler store — just template files and a spawn pass.

## What changes

- A template form of the change file (e.g. a `schedule:` frontmatter field, or a `templates/`
  sibling directory — brainstorm decides) holding a cadence plus the body to instantiate.
- A spawn pass in `docket-status`: when a template is due, mint a fresh `proposed` change from it
  (normal id allocation), back-linked to the template (`related:` or a `template:` field).
- Double-spawn guard: skip spawning while a previous instance of the same template is non-terminal.
- One-time scheduled changes ("propose this on 2026-09-01") that retire their template on spawn.
- Convention additions defining template format, due-evaluation semantics, and the guard.

## Out of scope

- Wall-clock daemons, cron jobs, or any push-based scheduler — spawning happens only when a docket
  skill runs (lazy tick). If nobody runs docket, nothing spawns; due templates spawn on next run.
- Scheduling *builds* (when implement-next runs) — only scheduling backlog *entry*.
- Sub-daily cadence (AgentRQ caps recurrence at hourly; docket's tick is far coarser anyway).

## Open questions

- Cadence syntax: full cron, or a simpler `every: 2 weeks` / `on: 2026-09-01` vocabulary?
- Are spawned instances `trivial: true` by default, template-defined, or always needs-brainstorm?
- Where do templates live so the board and id-scan don't confuse them with real changes?

## Reconcile log
