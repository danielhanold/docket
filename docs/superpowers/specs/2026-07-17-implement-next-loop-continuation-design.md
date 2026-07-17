# Design: loop continuation — implement-next as a driver-agnostic re-invocation contract (change 0088)

**Status:** design (interactively groomed 2026-07-17 by `docket-groom-next`, with the human)
**Change:** 0088 — Loop continuation — implement-next chains into the next ready change instead of stopping
**Depends on:** none (`depends_on: []`).
**Related:** 0008 (`proposed` — parallel backlog drain; owns concurrent fan-out **and** the in-run
loser-picks-next race optimization; complementary, not a dependency), 0087 (`proposed` — headless
finalize driver; a *single* hands-off finalize, **not** a loop — human-clarified, so it does not
contest this change's driver decision).

---

## 1. Context

`docket-implement-next` "runs solo per change": it selects one build-ready change, claims it (CAS
push on the metadata branch), builds it to an open PR, and **stops** at the human merge gate
(SKILL.md Step 7). A backlog of N independent build-ready changes therefore needs N separate human
invocations. Synthesized from the beads competitive review (2026-07-17): beads' `bd close
--claim-next` chains close→claim-next atomically, so one agent drains a queue without an
orchestrator re-dispatching it per item. docket already has the atomic-claim *safety* (the CAS
push, ADR-0001 territory); it lacks the *continuation*.

This change was auto-groom **abstained** (2026-07-17, critic verdict *sound*). The abstain was
correct: the titular value — continue-after-PR — cannot get a spec without settling the
continuation **architecture**, and neither horn was safely defaultable by an autonomous groomer:

- **In-context loop** (one forked run loops Step 7 → Step 1) accumulates N heavyweight builds
  (reconcile + plan + SDD + review) in a single forked context — an **unprobed harness/context-limit
  assumption** (learning [[harness-behavior-is-mode-and-version-scoped]]). A groomer writes markdown
  and cannot spike it.
- **Thin external driver** appeared to front-run the shared loop/driver primitive that #0008 and
  #0087 also circle — a backlog-partition call reserved to a human (learning [[moving-base]]).

**Both blockers are cleared by explicit human decisions in the grooming session:**

1. **Not in-context.** Each change builds via the usual forked subagent; the loop parent stays
   minimal. This removes the entire unprobed context-limit assumption — nothing heavyweight
   accumulates in the driver context.
2. **#0087 is not a loop** (single hands-off finalize). The partition resolves cleanly: **#0088 =
   serial self-continuation**, **#0008 = concurrent fan-out**, **#0087 = single headless finalize** —
   three distinct primitives, not one contested one.
3. **The driver is the built-in `/loop`, not a bespoke docket skill** (human-stated: "I would not
   want to rebuild that"). docket ships no loop primitive.

## 2. Decision

Ship a **driver-agnostic re-invocation contract** on `docket-implement-next`, plus documentation
naming `/loop` as the recommended driver. docket builds **no loop primitive** and **no new entry
surface**.

Because `docket-implement-next` is already `context: fork` (SKILL.md frontmatter), each `/loop`
iteration forks a fresh implement-next: the heavy build lives in the fork; the loop parent
accumulates only compact per-iteration disposition reports. The "parent stays minimal" property is
therefore satisfied *by construction* — it is not new machinery.

The contract is deliberately **driver-agnostic**: the same re-invocation semantics work for a human
re-typing the command, a cron/scheduled agent, or #0008's fan-out. `/loop` is *recommended*, not
*required*. This is the [[harness-behavior-is-mode-and-version-scoped]] discipline applied at design
time: the design does not rest on `/loop`'s internal fork-composition behavior; it makes
implement-next drivable and recommends `/loop`, with a build-time spike (§6) before docs promote
`/loop` as supported.

## 3. What ships

Two prose additions to `skills/docket-implement-next/SKILL.md` and one documentation section. **No
shell scripts change.**

### 3.1 Terminal disposition report (SKILL.md)

Every run ends by declaring exactly one of four **dispositions**, so any driver keys on the outcome
rather than parsing prose:

| Disposition | Meaning | Driver action |
|---|---|---|
| `advanced` | Built a change → PR opened (Step 7 reached). | continue |
| `contended` | Selected a change but lost the claim CAS; **nothing built**. | continue (re-select next iteration) |
| `drained` | No build-ready change in scope. | **stop** |
| `halted` | Stopped needing a human — fundamentally-invalidated design (Step 3) or hard error. | **stop + surface** |

The driver's decision is binary: **continue on `advanced`/`contended`, stop on `drained`/`halted`.**

This mostly *names* exits implement-next already has:
- Step 7 (PR opened, `status: implemented`, STOP) → **`advanced`**.
- Step 2 claim-race loss ("abort if no longer `proposed`") → **`contended`** — a normal, expected,
  continue-able outcome, explicitly **not** `halted`. (This is the *only* behavioral tightening of an
  existing exit: today's prose says "abort"; the contract classifies that abort as `contended` so a
  driver retries rather than treating it as a human-needed stop.)
- Step 3 FUNDAMENTALLY-invalidated / hard error → **`halted`**.

The one genuinely new exit is a clean **empty-queue `drained` report**: Step 1 today says "pick the
top" but does not spell out the no-build-ready-change case. The contract adds it — an explicit,
driver-recognizable "nothing to do in scope" terminal report.

The final report also **enumerates** what happened (the stub's reporting requirement): changes
built, changes skipped **with reasons** (needs-brainstorm / already in-progress / waiting on an
unmerged dependency), and which disposition ended the run.

### 3.2 Id-set scoping (SKILL.md — the human's added requirement)

Generalize Step 1's "accept an explicit id" from a single id to an **id allowlist**:

```
docket-implement-next 90,92,94        # drain only these, in deterministic order within the set
docket-implement-next 90              # unchanged: today's single-explicit-id behavior
docket-implement-next                 # unchanged: the whole build-ready backlog
```

Semantics:
- **Selection is restricted to the allowlist**; the convention's deterministic selection order
  (priority → age → lowest id) applies **within** the set. Unset ⇒ the whole build-ready backlog,
  byte-identical to today.
- A scoped member that is **not currently build-ready+claimable** — needs-brainstorm, already
  `in-progress` by another agent, or waiting on an unmerged `depends_on` — is **skipped with its
  reason** and never blocks the run. The loop reports `drained` once no member of the set is
  claimable.
- Scoping is an **allowlist, never a dependency override**: a scoped id whose `depends_on` is
  unsatisfied is skipped (reason: waiting on #M), not force-built. Build-readiness is unchanged.

### 3.3 The `/loop` drain pattern (documentation)

A documentation section (README under `docs/` and/or a `docket-convention` pointer) records the
supported drain pattern:

- `/loop docket-implement-next` — self-paced, drains the whole build-ready backlog.
- `/loop docket-implement-next 90,92,94` — self-paced, drains a named set.

Self-paced `/loop` continues on `advanced`/`contended` and calls `ScheduleWakeup stop:true` on
`drained`/`halted`. **Budget and iteration caps are `/loop`'s own mechanism** (its budget directive
and self-pacing) — docket leans on them and does not reimplement them. The doc states plainly that
the driver never merges (the human merge gate is untouched), so dependencies only clear between
drains via a human merge — a scoped change waiting on an unmerged dependency is *skipped this drain*,
not waited on.

## 4. Scope boundaries

**In scope:** the four-disposition terminal report; the empty-queue `drained` exit; id-set scoping;
the skipped-with-reasons enumeration; the `/loop` documentation.

**Out of scope (with owners):**
- **loser-picks-next / in-run re-selection on a lost race** — **NOT built here.** With `/loop`
  re-selecting each iteration, a race loser aborts cheaply *before building* (`contended`) and the
  next iteration picks another; the in-run optimization is unnecessary for continuation. It stays
  with **#0008**, which owns concurrent fan-out where racing actually matters. Per [[moving-base]],
  #0008 keeps this bullet; #0088 does not fold it in. #0088's only obligation is that a lost race
  reports `contended`, not `halted`.
- **Concurrent/parallel fan-out** — #0008.
- **Merging PRs mid-loop** — finalize stays a separate skill; the human merge gate is untouched.
- **Any cross-skill orchestrator** chaining groom → implement → finalize — this change only makes the
  implement stage self-continuing (honors the stub's original `## Out of scope`).
- **A bespoke `docket-drain` skill or new loop primitive** — explicitly rejected; `/loop` is the
  driver.

## 5. Open questions — resolved

- *In-context vs. external driver?* → **External, via `/loop`; each build a fresh fork** (§2). The
  in-context horn is rejected; the context-limit assumption is designed out.
- *Dependency interaction — stop vs. skip?* → **Skip with reason; the loop never merges, so a
  dependency cannot clear mid-drain.** The run drains all currently-build-ready work in scope, then
  reports `drained`. (§3.1, §3.3)
- *Trigger/entry surface (stub OQ3)?* → **Collapses — the trigger is `/loop docket-implement-next`;
  no new entry surface is built.** (§2, §3.3)
- *Relationship to #0008 — subsume/depend/absorb?* → **Neither. Complementary.** #0088 leans on
  `/loop` for serial drain; #0008 owns concurrent fan-out and the in-run race optimization. (§4)

## 6. Risk & build-time reconcile item

The design does not rest on `/loop`'s internals, but the docs promote `/loop docket-implement-next`
as *supported*. Before that promotion, the build (reconcile/verify) **must spike, in the actual
harness**, that `/loop`:
1. forks `docket-implement-next` per iteration (heavy build in the fork, parent minimal), and
2. reliably continues on `advanced`/`contended` and stops on `drained`/`halted`.

If `/loop` does **not** compose cleanly with the forked skill (see the session-memory caveats: forks
are not TUI-drillable and cannot receive task-notifications), **degrade**: document the contract +
manual/other-driver re-invocation, drop the `/loop`-is-supported claim, and file a follow-up. This
is a build-time reconcile item (recorded in the `## Reconcile log` at build), **not** a groom-time
blocker — the contract (§3.1, §3.2) stands regardless of which driver consumes it.

## 7. Testing / verification

- **Disposition mapping** — assert each of implement-next's terminal exits maps to exactly one
  disposition (Step 7→`advanced`, Step 2 race-loss→`contended`, empty queue→`drained`, Step 3
  invalidated/error→`halted`), and that the final report names it.
- **Id-set scoping** — a scoped run selects only allowlist members in deterministic order; a member
  that is needs-brainstorm / in-progress / dependency-blocked is skipped with the correct reason and
  does not abort the run; an all-unclaimable set reports `drained`.
- **`/loop` spike (§6)** — the build-time harness spike is the acceptance check for the docs claim;
  its outcome (supported vs. degraded) is recorded in the results file and `## Reconcile log`.
- The change touches skill prose + docs (no scripts), so the existing docket test suite (skill
  size-budget guards, sentinel greps) plus a whole-branch review carry it; any new sentinel guarding
  the disposition vocabulary rides with the build.
