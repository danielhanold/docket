---
id: 66
slug: auto-groom-critic-recheck-foreground
title: Auto-groom's critic re-check must be foreground — a forked skill that yields returns a half-done run to its caller
status: proposed
priority: high
created: 2026-07-12
updated: 2026-07-12
depends_on: []
related: [17, 61, 65]
adrs: [24]
spec: docs/superpowers/specs/2026-07-12-auto-groom-critic-recheck-foreground-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
issue:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-12-auto-groom-critic-recheck-foreground-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-12-auto-groom-critic-recheck-foreground-design.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

Observed live on 2026-07-12 grooming change #0065. `docket-auto-groom` (running as a fork) drafted the spec, dispatched its critic, got back a *wrong but fixable* verdict, and entered the bounded revision round. It then **dispatched the critic's re-check as a background task and yielded**, returning to its caller with:

> `Holding for the critic's re-check verdict — the monitor will notify me the moment it lands.`

To the parent, the Skill tool reported `completed (forked execution)` — the run looked **finished**. It was not: the agent was alive and mid-step-5, with a 22KB spec sitting **uncommitted and unlinked** in the shared `.docket` worktree and the stub still reading needs-brainstorm.

The parent then did the natural thing — inspected the "aftermath", concluded the fork had crashed, and committed and pushed the agent's working-tree files itself. The groom later woke, found its own work already committed under someone else's messages, and reported it as a concurrent loop racing it. Nothing was lost this time (the landed content was byte-for-byte the intended output), but that was luck, not design: two writers were live in one worktree, each believing the other was not.

**Two independent defects, and both are load-bearing for autonomous operation:**

1. **The skill's foreground rule doesn't cover the re-check.** SKILL.md §5 (line 44) explicitly qualifies the *initial* critic dispatch as **foreground**, then describes the revision round — "the critic re-checks only the revised items" — with **no foreground qualifier**. The agent read the gap literally and backgrounded the re-check. The convention's composition rule (dispatches are foreground; the parent suspends until the child returns) was never restated for the second round.

2. **A forked skill has no channel to receive a notification, so "wait for a notification" is a yield.** A fork cannot be resumed by the task-notification it is waiting on the way a main-loop session can — waiting on one hands control back to the caller. This is the same family as the known SDD hazard (an implementer that backgrounds the long test suite stalls the loop), and it generalizes beyond auto-groom: **any forked skill that awaits a background child returns a half-done run to a caller that will believe it finished.**

Left alone, an overnight drain silently hands build-ready-looking stubs (or uncommitted specs) to whatever runs next.

## What changes

Skill + convention wording, one guard test, and one dated ADR update note. No script, schema, or dispatch *mechanics* change — only the prose that governs how a dispatch is awaited.

**1. Close the wording gap in `docket-auto-groom` §3.** Qualify the critic's **re-check** round explicitly foreground, in the same sentence that bounds the revision round, and point it at the general rule — so no reader (human or agent) can take the `(foreground, …)` on the first dispatch as applying only there.

**2. State the general never-yield rule at the single contract source — `docket-convention`'s *Composition* paragraph** (which already declares dispatches foreground). A forked/dispatched skill's parent must **actively block** on the child; it may never background a child and *yield* to await a task-notification, because a fork has no channel to receive one — a "wait for the notification" hands control back to the caller and returns a half-done run that reads as `completed`. This one statement binds `docket-auto-groom` (the re-check), the two single-shot dispatchers, and any future multi-round dispatch — no per-skill duplication. Reciprocally, a caller must not read a bare `completed` as proof of completion: it verifies the child's git-state transition (the contract already in that paragraph) and never adopts a child's uncommitted working-tree files.

**3. Guard both** in `tests/test_composition_wiring.sh` (owner of the composition contract) with positive-anchor sentinels: the convention states the never-yield rule; auto-groom's re-check is foreground.

**4. A dated `## Update` note on ADR-0024** extends its "no channel to the human" to its corollary — no channel to a task-notification either — keeping the principle discoverable from the decision that spawns it. `adrs: [24]` carries the note to `main` at close-out.

## Out of scope

- Changing how the critic gate works, its verdict vocabulary, or the one-bounded-revision-round rule — the gate behaved correctly; only its *dispatch mechanics* are at fault.
- Removing `context: fork` from auto-groom or any other skill (0061's fork-exclusion principle stands; see #0065).
- Editing the two single-shot dispatchers' SKILL.md files — their dispatch sites are already foreground-qualified and single-shot; the general convention rule covers them by construction.
- Concurrency control / locking for the shared `.docket` worktree. The race here was a *consequence* of the premature return, not an independent defect. If locking is ever the real fix, that is a separate change.

## Open questions

Both resolved at groom (2026-07-12); the reasoning and rejected alternatives are audited in the spec's *Design decisions* block.

- **Same gap in the other composing skills?** → **No edits needed.** `docket-implement-next` (→ status/adr) and `docket-finalize-change` (→ rebase-resolver/integration-repair) dispatch single-shot children, each already `(foreground, …)`; there is no second round to under-qualify, and the general convention rule (change 2) binds them regardless (spec D1).
- **Caller-side guard advisory or hard?** → **Advisory.** Lean on the existing git-state dispatch contract — a caller verifies the child's transition and does not treat a bare `completed` as done — rather than a new enforced validator; the root-cause fix (never-yield) removes the hazard, and a hard guard is a clean follow-up only if premature returns recur (spec D2).

## Reconcile log
