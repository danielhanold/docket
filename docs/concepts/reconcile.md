# Reconcile

## The problem it solves

A change can sit in the backlog for weeks between the day it is designed
and the day it is built. In that gap the ground moves: a related change
lands that already did half the work, an architecture decision reverses
the approach the design assumed, the code the plan targeted is refactored
out from under it. Build straight from the old design and you produce a
correct implementation of a plan that no longer makes sense — the most
expensive kind of wrong, because it passes its own tests and looks done.

**Reconcile** is a check at build time that the change — one unit of
planned work, roughly one pull request, tracked as one markdown file — is
still worth doing and its assumptions still hold, before any code is
written. It runs after the change is picked up and before the working
copy is cut, re-reading the design against current reality and then doing
one of three things: refreshing the scope, killing the change as
obsolete, or halting it as invalidated so a human can look. It spends a
few minutes of reading to avoid building the wrong thing.

## The moving parts

Reconcile sits between the **claim** — the moment a change is picked up
for building; it records which branch will carry the work and when it was
taken — and the first line of code.

```
  build-ready change
        │
     (claim taken)
        │
        ▼
   ┌─ reconcile ─────────────────────────────────────────┐
   │  re-read the change against:                         │
   │    - related and archived changes                    │
   │    - ADRs (architecture decision records)            │
   │    - the current code                                │
   │  then exactly one of:                                │
   │    ├─ refresh scope, record a Reconcile log          │
   │    ├─ kill  — the change is now obsolete             │
   │    └─ halt  — an assumption is invalidated; a human  │
   │              is needed                               │
   └──────────────────────────────────────────────────────┘
        │  (refreshed)
        ▼
   cut the worktree, plan, build
```

- The inputs are the change's own **spec** (the design document a change
  links to, written before building), the ADRs, and neighbouring changes
  both active and archived.
- The output that lets the build proceed is a refreshed scope plus a
  `## Reconcile log` written onto the change, so the reasoning is durable
  and not just in the builder's head.
- The refreshed spec is read from the **metadata branch** — the `docket`
  git branch where the backlog, specs, and decisions are stored, separate
  from the code — during the build, never carried on the feature branch.

Reconcile is docket's one addition to the standard build chain that other
AI-native workflows do not describe. The
[AI-native SDLC playbook comparison](../comparison/ai-native-sdlc-playbook.md)
records it under the row **Refresh a stale change before planning**.

## The invariants

- Reconcile runs after the claim and before any code is written; a build
  never skips it.
- Reconcile ends a change exactly one of three ways — refresh it, kill it
  as obsolete, or halt it as invalidated for a human — and never silently
  builds a design it found stale.
- A refreshed change carries a `## Reconcile log` on its change file, so
  the decision to proceed is auditable after the fact.
- Reconcile reads the design against related and archived changes, the
  ADRs, and the current code — never the design in isolation.

## Decided in

- [ADR-0001](../adrs/0001-docket-metadata-branch-model.md) — put the
  reconcile push and the reconciled spec on the metadata branch, read
  cross-tree during the build rather than carried on the feature branch.
- [ADR-0045](../adrs/0045-auto-capture-is-best-effort.md) — made the
  discovered-work capture that runs during the reconcile pass best-effort,
  so a failed stub mint never aborts the change in flight.
