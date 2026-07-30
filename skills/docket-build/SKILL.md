---
name: docket-build
description: Use as docket's build role (skills.build) — executes an implementation plan task-by-task by routing each task to a named economy/standard/premium Claude profile agent running the docket-build-task contract, with one bounded escalation per task, no per-task review, and a single full-suite gate at the end.
---

# docket-build — profile-routed plan execution

The lean alternative to `superpowers:subagent-driven-development`. You are already running inside
`docket-implement-next` Step 5 with the plan written and the feature worktree cut. You read the
plan, route each task to a profile, dispatch one fresh worker per task, apply the escalation
protocol, and run the build gate. Then you stop — review is not yours.

You are not a router subagent: routing is a decision you make in this context. Each selected task
gets exactly one fresh worker dispatch unless that worker requests its single allowed escalation.

## Profiles

Three named agents, all preloading the same `docket-build-task` worker skill and differing only in
model and effort:

| Agent | Use |
|---|---|
| `docket-build-economy` | fully specified, localized, pattern-following, no consequential risk |
| `docket-build-standard` | normal feature, integration, refactor, and debugging work |
| `docket-build-premium` | named risk, or unresolved architecture |

`premium` means greater reasoning investment, **not** a stronger correctness guarantee — every
profile carries identical testing and completion obligations. Model and effort resolve through
docket's ordinary generated-agent layer and may be overridden at the global, repo-committed, or
repo-local layer; never restate literal model IDs or effort tiers in your dispatch prose.

## Routing

**Explicit override wins.** A plan task may carry a line of the form:

```markdown
**Build profile:** economy
```

A valid value (`economy`, `standard`, `premium`) is authoritative; record its use in that task's
routing line. An **invalid** value is a plan contract error: **halt** and surface it — never
silently fall back to a default.

**Otherwise classify**, with a deliberate asymmetry — `economy` must be *positively* established,
a named risk selects `premium`, and uncertainty defaults to `standard`:

- **`premium`** when the task involves authentication or security boundaries, migrations or
  irreversible data changes, concurrency or locking, release infrastructure, or unresolved
  architecture.
- **`economy`** *only when* the task is fully specified, localized to roughly one or two files,
  follows an established pattern, and carries no consequential risk.
- **`standard`** for everything remaining.

Emit one concise routing line per task naming both the profile and its reason.

## Dispatching a task

Dispatch the profile agent **by name**, foreground, one task at a time — later tasks build on
earlier task commits and share the worktree, so workers are strictly sequential. Give the worker:
the plan task text, the branch and worktree, the applicable repository instructions, the selected
profile and routing reason, and the completion schema. Never preload a review skill, never dispatch
a task reviewer, and never dispatch two workers concurrently.

If profile dispatch is genuinely unavailable — established only per the convention's
*Dispatch-capability resolution*, **never from a tool name** — this role is
**Tier C, authorized-or-halt**: only an explicitly configured `skills.build: auto` authorizes
inline execution. Selecting `docket-build` is not implicit authorization to discard its isolation
or its model/effort contract, so abort-and-report instead, leaving the change `in-progress`.

A profile agent that is **not registered on this machine** is the same authorized-or-halt
condition, reached differently: the harness rejected a dispatch naming `docket-build-economy` — a
concrete rejection of a named agent, never an inference about dispatch capability from a missing
tool name, so the rule above stands unchanged. The cause is a stale install: `install.sh` generates
the profile wrappers and links the build skills, and a harness registers them only at session
start. Abort and report, naming a re-run of `install.sh` plus a fresh session as the remedy.

## Reading a worker's return

Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`. A
**missing or malformed outcome halts** the build. Never infer success from a child merely
reporting that it finished — a `COMPLETE` claim must come with the focused verification result
and a commit SHA, and a task without a commit is not complete.

## Escalation

Each task may escalate automatically **at most once**:

```text
initial economy  -> one standard retry
initial standard -> one premium retry
initial premium  -> halt
```

The retry consumes that task's whole escalation allowance: a task that started at `economy` and
whose `standard` retry still cannot complete **halts** — it does not climb again to `premium`.

Escalate only on a concrete reason that the task is materially more complex or riskier than the
assigned profile. An expected RED test, ordinary debugging, or a single failed test run is not an
escalation condition; a worker returning `NEEDS_ESCALATION` without such a reason is a malformed
return.

The stronger worker continues in the **same worktree** and must inspect and account for any
uncommitted changes the weaker worker left — revising them is allowed, discarding them blindly is
not. A successful escalation continues this run automatically.

Return `halted` — change still `in-progress`, worktree preserved for inspection or resume — when a
premium worker requests escalation, an escalated worker still cannot finish, requirements
contradict, authority or dependencies are missing, or continuation is unsafe.

## The build gate

Workers run focused tests only. After every plan task has committed, run the **whole suite once**:

1. Use the already-resolved `FINALIZE_TEST_COMMAND` when it is non-empty.
2. Otherwise reuse finalize's existing suite **auto-detection**.

The command boundary is the one finalize already publishes — its `configured-bash-finalize` marker
block in `skills/docket-finalize-change/SKILL.md` is the single source, and the awkward `finalize`
namespace is deliberately kept rather than introducing a second, driftable test command. Do not
copy that fragment into this file.

**Green** → the build is done; `docket-implement-next` Step 6 runs the resolved `skills.review`
role once over the whole branch.

**Red** → the build **never invokes review**. Turn the failure into exactly one synthetic
integration-repair task, run through the same worker contract on the ladder
`standard -> premium -> halt`. The repair worker diagnoses the cross-task failure, adds regression
coverage where appropriate, fixes it, re-runs the full suite, and commits the repair. There is no
repeated repair/review loop; failure after the premium repair path returns `halted`.

## Review boundary

This build performs **no per-task independent review** and **no final review of its own**. The
worker's self-review is part of implementation, not a second agent or an adversarial gate. Docket's
single independent whole-branch review remains `docket-implement-next` Step 6's `skills.review`
role, which stays separately configurable.

## Checkpointing

Read `BUILD_CHECKPOINT` from the Step-0 config export.

**`false` (default)** — persist nothing. Completed work is durable through the per-task code
commits; keep only the compact in-context worker returns; write no `.superpowers/docket-build/`
files. A resumed run reconstructs progress conservatively from the plan, the commits, the code, and
the tests rather than trusting a formal receipt.

**`true`** — write a compact ledger to `.superpowers/docket-build/<change-id>/progress.md` (covered
by the committed `.superpowers/` ignore rule) recording branch, plan path and blob hash, task
identity and status, profile and reason, escalation, TDD evidence or exception, verification, and
commit SHA. It is a state ledger, not a prose task report. On resume, skip a task **only** when its
ledger entry is `COMPLETE`, the plan hash still matches, and its commit is an **ancestor** of the
current branch — missing, stale, malformed, or contradictory state never marks a task complete.

## Output

Emit concise, stable lines and nothing more: task-to-profile selection and reason; escalation and
reason; worker outcome and commit; focused verification; full-suite command and result; the
terminal build disposition. Write no verbose task artifact unless `BUILD_CHECKPOINT` is `true`.
Material TDD exceptions and residual risks flow into the PR description or the results artifact,
not into per-task files.
