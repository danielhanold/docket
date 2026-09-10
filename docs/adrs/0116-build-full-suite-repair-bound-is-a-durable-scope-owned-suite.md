---
id: 116
slug: 'build-full-suite-repair-bound-is-a-durable-scope-owned-suite'
title: 'Build full-suite repair bound is a durable scope-owned suite-attempt reservation'
status: 'Accepted'
date: '2026-09-10'
supersedes: []
reverses: []
relates_to: [74, 102, 107, 115]
change: 421
---

## Context

Before change 0421 docket-build's full-suite repair bound was prose-only in skills/docket-build/SKILL.md (a single premium->max integration-repair path, then halt) with no durable Go accounting. internal/gatedrive's per-drive `Attempt` counter is not a repair budget: it is a one-relaunch recovery of the same logical run, so it cannot express "how many full-suite repair cycles has this change spent". Change 0421 makes the bound configurable via `build.max_attempts` (built-in default 4, positive integer, counting the initial full-suite run; the value 1 disables repair cycles), which requires accounting that survives process death, driver relaunch, handoff/claim continuation, and an outer-gate re-dispatch of the same change.

## Decision

Introduce a durable per-`(RepoIdentity, ChangeID, phase)` suite-attempt budget store (internal/gatedrive/suitebudget.go) under the git common dir, using the same flock + generation-CAS + atomic-write discipline as the existing scope store. The limit is snapshotted at first reservation (create time), so a config edit mid-phase never rewrites an owned budget and recovery/continuation preserve consumed budget. Enforcement lives at the app-layer GateDriveService.Start seam (internal/app/gate_drive.go): a build-owned start (owner == "build") carrying a non-empty ChangeID reserves one attempt keyed on the literal phase "build" BEFORE the drive record is created or any suite launches. Nothing else is charged — task-owned focused-test starts, finalize drives (a different owner), advance/observation, driver relaunch recovery, takeover, handoff/claim continuation, and scopeless (no-ChangeID) build starts. There are no refunds: a reservation whose later drive creation or launch fails still counts, so a lost launch can never overrun the bound (a bounded run of pre-launch HALTs during recovery can therefore spend attempts — the intended fail-safe direction). On exhaustion the start is refused with a stable `suite-attempts-exhausted` reason naming build.max_attempts and used/limit, which the build skill's halting conditions handle. The budget key carries no outer-attempt or epoch dimension, so it spans the change lifetime: an outer-gate `gate-retry-once` re-dispatch of the same change inherits the remaining build budget and can never acquire more build repairs.

## Consequences

The default (4) allows the initial full-suite run plus three repair-and-rerun cycles; a limit of 1 halts a red initial run with no repair. No repair worker can bypass the cap, because its post-fix full-suite rerun is itself a build-owned start through the same seam. Keeping enforcement in the app layer leaves the gatedrive engine owner-agnostic — the engine stores and CASes the budget, the app layer decides who is charged. The reserve-before-launch, no-refund ordering is the deliberate safety choice: it errs toward under-running the configured bound, never overrunning it, at the cost that a pre-launch failure consumes an attempt. Because the key omits any outer-attempt dimension, the build budget is a property of the change rather than of one dispatch, which is what makes the outer retry unable to buy extra repairs.

## Alternatives considered

Reuse internal/gatedrive's per-drive `Attempt` counter as the repair budget — rejected: it is a one-relaunch recovery counter for the same logical run and resets across drives, so it would silently grant unlimited repairs. Keep the bound as prose in the build skill — rejected: change 0421 exists because a prose bound has no durable accounting and cannot be configured or enforced. Enforce inside the gatedrive engine — rejected: it would make the engine owner-aware, coupling a generic drive primitive to the build role. Key the budget on the outer attempt or epoch — rejected: an outer gate-retry-once re-dispatch would then refill the build repair budget, defeating the bound. Refund a reservation whose launch fails — rejected: a lost launch would then be indistinguishable from a never-charged one, allowing overrun of the configured bound; under-running is the safe failure direction. Read the limit live on each reservation instead of snapshotting it — rejected: a mid-phase config edit would retroactively change an owned budget.
