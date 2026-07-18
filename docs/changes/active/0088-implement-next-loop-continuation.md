---
id: 88
slug: implement-next-loop-continuation
title: Loop continuation — implement-next chains into the next ready change instead of stopping
status: in-progress
priority: medium
created: 2026-07-17
updated: 2026-07-18
depends_on: []
related: [8, 87]
adrs: [1]
spec: docs/superpowers/specs/2026-07-17-implement-next-loop-continuation-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/implement-next-loop-continuation
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-17-implement-next-loop-continuation-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-implement-next-loop-continuation-design.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads' loop
continuation primitive is `bd close --claim-next`: closing an issue atomically chains into claiming
the next ready one, so an agent drains a queue without an orchestrator re-dispatching it per item.
docket already has the atomic-claim *safety* (the compare-and-swap claim push, ADR-0001 territory);
what it lacks is the *continuation*. `docket-implement-next` "runs solo per change" — after opening
a PR it stops, so a backlog of N independent build-ready changes needs N separate human
invocations.

## What changes

The continuation is a **driver-agnostic re-invocation contract** on `docket-implement-next` — docket
builds **no loop primitive and no new entry surface**. Because implement-next is already a forked
subagent (`context: fork`), each iteration of a generic driver forks a fresh run: the heavy build
lives in the fork, the driver context stays minimal. Two prose additions to the skill (no scripts):

- **Terminal disposition report** — every run ends declaring one of four outcomes a driver keys on:
  `advanced` (built → PR), `contended` (lost the claim race, nothing built), `drained` (no
  build-ready change in scope), `halted` (stopped, needs a human). A driver continues on
  `advanced`/`contended` and stops on `drained`/`halted`. This mostly names exits that already
  exist; the one new exit is a clean empty-queue `drained` report. The final report enumerates what
  was built, what was skipped and why, and which disposition ended the run.
- **Id-set scoping** — generalize "accept an explicit id" to an id allowlist
  (`docket-implement-next 90,92,94`): drain only those, in deterministic order within the set, unset
  ⇒ the whole build-ready backlog as today. A scoped member that is not build-ready (needs-brainstorm
  / in-progress / dependency-blocked) is skipped with its reason.

The recommended driver is the built-in **`/loop`** — `/loop docket-implement-next` drains the whole
backlog, `/loop docket-implement-next 90,92,94` a named set — self-paced, stopping on `drained`.
Budget/iteration caps are `/loop`'s own mechanism; docket does not rebuild them. A documentation
section records the pattern. Full design, disposition mapping, and the build-time `/loop` spike are
in the linked spec.

## Out of scope

- **loser-picks-next / in-run re-selection on a lost claim race** — owned by **#0008**. With a driver
  re-selecting each iteration, a race loser aborts cheaply before building (`contended`) and the next
  iteration picks another, so the in-run optimization is unnecessary here; #0088 only guarantees the
  lost race reports `contended`, not `halted`.
- Concurrent/parallel fan-out of multiple implementers (#0008 owns that).
- Merging PRs mid-loop — finalize stays a separate skill; the human merge gate is untouched.
- Any cross-skill orchestrator chaining groom → implement → finalize — this change only makes the
  implement stage self-continuing.
- A bespoke `docket-drain` skill or new loop primitive — `/loop` is the driver.

## Open questions

_Resolved during grooming (2026-07-17) — see the linked spec §5. The only deferred item is a
build-time harness spike confirming `/loop` cleanly drives the forked skill and stops on `drained`
(spec §6); it degrades gracefully and is recorded here at build, not a groom-time blocker._

## Reconcile log

### 2026-07-18 — build-time reconcile (docket-implement-next)

Freshened against current reality before planning. Findings:

- **Related changes hold.** #0008 (parallel backlog drain) and #0087 (headless finalize driver) are
  both still `proposed`. The spec's three-way partition — #0088 = serial self-continuation, #0008 =
  concurrent fan-out, #0087 = single headless finalize — stands unchanged; no scope overlap to fold
  in. ADR-0001 (metadata-branch / CAS-claim model) remains the relevant citation.
- **Size-budget guard is live and binding.** Change 0085 shipped `tests/test_skill_size_budgets.sh`;
  the row for `skills/docket-implement-next/SKILL.md` is 119 lines / 2451 words, and the file is
  currently 108 / 2228. The disposition-report + id-set-scoping prose will exceed the line budget, so
  the build RAISES that budget row **in the same diff** (the guard explicitly permits an in-diff
  raise). Spec §7 already anticipated the size-budget guard.
- **§6 `/loop` spike degraded (spec-authorized).** The live harness spike — confirming `/loop`
  forks implement-next per iteration and continues on `advanced`/`contended`, stops on
  `drained`/`halted` — cannot be run inside this autonomous forked build: driving `/loop
  docket-implement-next` live would uncontrolledly claim and build real backlog changes, and a forked
  subagent has no TUI to drive `/loop` or observe its per-iteration disposition handling. Per the
  spec's own §6 degrade path, the build ships the **driver-agnostic contract** (which stands
  regardless of driver) and documents `/loop` as the **recommended** drain pattern, framed as a
  recommended pattern rather than a hard verified-supported guarantee, with a follow-up filed for the
  live spike. Recorded in the results file.
- **Self-referential build.** This change edits the very skill (`docket-implement-next`) executing
  the build. Safe: edits land on the feature branch's repo source; the running installed copy is
  untouched mid-run, and the feature branch never touches docket metadata.

No obsolescence, no fundamental invalidation — the design is intact; only the §6 driver-verification
scope degrades, exactly as the spec pre-authorized. Proceeding to plan.
