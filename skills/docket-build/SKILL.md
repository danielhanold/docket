---
name: docket-build
description: Use as docket's build role (skills.build) — executes an implementation plan task-by-task by routing each task to a named economy/standard/premium/max profile agent running the docket-build-task contract, with one bounded escalation per task, no per-task review, and a single full-suite gate at the end.
---

# docket-build — profile-routed plan execution

docket's build role, bound by `skills.build`. You are already running inside
`docket-implement-next` Step 5 with the plan written and the feature worktree cut. You read the
plan, route each task to a profile, dispatch one fresh worker per task, apply the escalation
protocol, and run the build gate. Then you stop — review is not yours.

You are not a router subagent: routing is a decision you make in this context. Each selected task
gets exactly one fresh worker dispatch unless that worker requests its single allowed escalation.

## Inputs

- The **plan** `docket-implement-next` Step 4 wrote, at the path recorded in the change's `plan:`
  field and committed on the feature branch.
- The **feature branch and worktree** already cut for this change, plus that repo's own
  instruction files (`AGENTS.md`, `CLAUDE.md`, nested equivalents).
- The plan's `### Task N` headings, which are the **unit of dispatch** — one heading, one worker,
  one commit. The routing rubric below assumes that granularity; a plan whose tasks are not
  separable at that boundary is a planning defect, not something to re-cut here.

## Profiles

Four named agents, all preloading the same `docket-build-task` worker skill and differing only in
model and effort:

| Agent | Use |
|---|---|
| `docket-build-economy` | fully specified, pattern-following, no consequential risk, no cross-file reasoning |
| `docket-build-standard` | normal feature, integration, refactor, and debugging work — the default |
| `docket-build-premium` | consequential but correctable risk, or a risk the plan names |
| `docket-build-max` | unresolved architecture, or an irreversible data change |

A higher rung means greater reasoning investment, **not** a stronger correctness guarantee — every
profile carries identical testing and completion obligations. Model and effort resolve through
docket's ordinary generated-agent layer — the shipped `agents/harness-defaults.yml` under whatever
the global, repo-committed, and repo-local layers set; a harness the sidecar does not map for a
profile runs it unpinned. Never restate literal model IDs or effort tiers in your dispatch prose.

## Routing

**Explicit override wins.** A plan task may carry a line of the form:

```markdown
**Build profile:** economy
```

A valid value (`economy`, `standard`, `premium`, `max`) is authoritative; record its use in that task's
routing line. An **invalid** value is a plan contract error: **halt** per *Halting conditions* and
surface it — never silently fall back to a default.

**Otherwise classify**, with a deliberate asymmetry — `economy` must be *positively* established,
named risk selects upward, and uncertainty defaults to `standard`.

The `max`/`premium` boundary has an organizing principle, not just a list: **`max` is for mistakes this
build's own correction machinery cannot walk back.** An auth bug is serious but patch-correctable and
caught at the suite gate or in review; destroyed data cannot be un-destroyed by a retry, and a wrong
architectural call shapes every task after it. Resolve edge cases by applying that test, not by
extending the lists below.

- **`max`** — **unresolved architecture** or an **irreversible data change** (a destructive
  migration, a backfill, anything that cannot be rolled back). Nothing else classifies here.
  Irreversibility is the test: a reversible or purely additive migration is *not* `max` — it is
  `premium`, or `standard` if it carries no consequential risk at all.
- **`premium`** — authentication or security boundaries, concurrency or locking, release
  infrastructure, or any consequential risk **explicitly named in the plan or spec text**. That last
  door is honored, not inferred: never articulate a new risk on your own — your classification is
  this closed list, so uncertainty still sinks to `standard`.
- **`standard`** — everything remaining; the default and the uncertainty sink. Deliberately including
  hard-but-safe work: difficulty without consequence stays here, because the plan override covers
  difficulty known at plan time and the `standard -> premium` escalation covers difficulty discovered at
  build time.
- **`economy`** — *only when* the task is fully specified, follows an established pattern, carries no
  consequential risk, and requires **no cross-file reasoning** — either localized to a couple of
  implementation files (tests do not count against locality), or a mechanical, pattern-identical
  edit repeated across many files whose instances do not interact and where a missed instance fails
  loudly (a grep, a validator) rather than silently. All four conditions must hold; doubt about any
  one of them means `standard`.

`max` is rare by construction: the two-item rubric above, an explicit plan override, and a `premium`
escalation are its only three doors.

Emit one concise routing line per task naming both the profile and its reason.

## Dispatching a task

Dispatch the profile agent **by name**, foreground, one task at a time — later tasks build on
earlier task commits and share the worktree, so workers are strictly sequential. Give the worker:
the plan task text, the branch and worktree, the applicable repository instructions, the selected
profile and routing reason, and the completion schema. Never dispatch a task reviewer, and
never dispatch two workers concurrently. Never preload a review skill either — though for a
**named** agent the operative protection is the wrapper's own `skills:` frontmatter, which you
cannot change from here, so what this rule actually forbids is bolting a review skill or a review
instruction onto the dispatch prompt.

If profile dispatch is genuinely unavailable — established only per the convention's
*Dispatch-capability resolution*, **never from a tool name** — this role is
**Tier C, authorized-or-halt**: only an explicitly configured `skills.build: auto` authorizes
inline execution. Selecting `docket-build` is not implicit authorization to discard its isolation
or its model/effort contract, so halt per *Halting conditions* instead.

A profile agent that is **not registered on this machine** is the same authorized-or-halt
condition, reached differently: the harness rejected a dispatch naming `docket-build-economy` — a
concrete rejection of a named agent, never an inference about dispatch capability from a missing
tool name, so the rule above stands unchanged. The cause is a stale install: `install.sh` generates
the profile wrappers and links the build skills, and a harness registers them only at session
start. Halt, naming a re-run of `install.sh` plus a fresh session as the remedy.

## Reading a worker's return

Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`. A
**missing or malformed outcome halts** the build. Never infer success from a child merely
reporting that it finished: a child's completion report is unreliable in **both** directions, so
every claim is settled against git state and never against the return's prose — a SHA-shaped string
appearing somewhere in the text is not a commit.

**Malformed is wider than an unparsable token.** Before accepting a `COMPLETE`, verify the claimed
commit: the SHA must resolve in this repository *and* be an ancestor of the branch tip
(`git merge-base --is-ancestor <sha> HEAD`). A `COMPLETE` whose commit is absent, unresolvable, or
not on this branch is a malformed return — halt per *Halting conditions*, and never re-dispatch the
task to "fix" its own return. A `COMPLETE` must equally carry the focused verification result, and
a task without a commit is not complete. A `NEEDS_ESCALATION` carrying no concrete reason is
malformed the same way (see *Escalation*).

## Escalation

Each task may escalate automatically **at most once**:

```text
initial economy  -> one standard retry
initial standard -> one premium retry
initial premium  -> one max retry
initial max      -> halt
```

The retry consumes that task's whole escalation allowance: a task that started at `economy` and
whose `standard` retry still cannot complete **halts** — it does not climb again to `premium`.

Escalate only on a concrete reason that the task is materially more complex or riskier than the
assigned profile. An expected RED test, ordinary debugging, or a single failed test run is not an
escalation condition; a worker returning `NEEDS_ESCALATION` without such a reason is a **malformed
return**, and a malformed return halts — it is never a free escalation.

The stronger worker continues in the **same worktree** and must inspect and account for any
uncommitted changes the weaker worker left — revising them is allowed, discarding them blindly is
not. A successful escalation continues this run automatically.

A failed attempt that left a **commit** — not merely a dirty tree — is different, and it is the
one state that cancels the escalation. The worker was told never to commit on `NEEDS_ESCALATION` or
`BLOCKED`, but a crashed or truncated one still can; the escalated worker is separately forbidden
to rewrite earlier task commits, so it would inherit state it cannot clean up, and this task's
exactly-one-commit accounting is already contaminated. **Do not escalate onto a stray commit** —
halt per *Halting conditions*, naming the stray SHA so a human can inspect, keep, or drop it.

## Halting conditions

Every halt is the same disposition: stop, return `halted` — the change stays `in-progress` and the
worktree is preserved for inspection or resume — and report which condition below fired with its
concrete evidence (task, profile, SHA, command, or harness message). Never improvise past one,
never substitute a weaker path, and never invoke review. The rules elsewhere in this file name
their condition and point here rather than restating the disposition.

- **Profile routing is un-dispatchable**, established per the convention's *Dispatch-capability
  resolution* and never from a tool name, and `skills.build: auto` was not explicitly configured.
- **A profile agent is not registered on this machine** — the harness rejected a dispatch naming
  it. Remedy: re-run `install.sh`, then start a fresh session.
- **An explicit plan `Build profile:` value is invalid** — a plan contract error; never fall back
  to a default.
- **A worker return is malformed or unverifiable** — a missing or unparsable outcome, a `COMPLETE`
  whose commit is absent, unresolvable, or not an ancestor of the branch tip, or a
  `NEEDS_ESCALATION` with no concrete reason. Never re-dispatch a task to repair its own return.
- **A task's escalation allowance is exhausted** — an initial `max` worker requests escalation,
  or an escalated worker still cannot finish.
- **A failed attempt left a commit** — name the stray SHA; do not escalate onto it.
- **No suite is detectable** — no `FINALIZE_TEST_COMMAND` and nothing finalize's auto-detection
  recognizes. Remedy: set `finalize.test_command`. Never convert this into a repair task.
- **The suite is still red after the max repair** — there is no second repair round.
- **Continuation is unsafe** — a worker's `BLOCKED`: contradictory requirements, missing authority,
  or an absent dependency.

## The build gate

Workers run focused tests only. After every plan task has committed, run the **whole suite once**:

1. Use the already-resolved `FINALIZE_TEST_COMMAND` when it is non-empty.
2. Otherwise reuse finalize's existing suite **auto-detection**.
3. **Neither** — no `FINALIZE_TEST_COMMAND` *and* nothing the auto-detection recognizes — is a
   **configuration gap, not a red suite**. A repo with no matching test files leaves the detection
   glob literal and exits non-zero; reading that as RED would manufacture a repair task and burn a
   whole ladder on a config problem. Finalize itself aborts here rather than repairing, and so do
   you: halt per *Halting conditions*, naming `finalize.test_command` as the remedy.

The command boundary is the one finalize already publishes — its `configured-bash-finalize` marker
block in `skills/docket-finalize-change/SKILL.md` is the single source, and the awkward `finalize`
namespace is deliberately kept rather than introducing a second, driftable test command. Do not
copy that fragment into this file.

**Green** → the build is done. Emit the **build-evidence** record — a marker-bounded block carrying
`command` (the exact full-suite command run), `result: green`, `head_sha` (the branch HEAD the run
tested, from `git rev-parse HEAD`), and `ran_at` (UTC ISO-8601):

```text
<!-- docket:build-evidence:start -->
command:  <full-suite command>
result:   green
head_sha: <40-char SHA>
ran_at:   <UTC ISO-8601>
<!-- docket:build-evidence:end -->
```

The record certifies the branch so the review step need not re-run the suite; `docket-implement-next`
Step 6 validates it and Step 7 writes it into the PR body, then runs the resolved `skills.review`
role once over the whole branch. Only a green run mints a record: a red suite mints no evidence
record at all, and enters the repair path below.

**Red** → the build **never invokes review**. Turn the failure into exactly one synthetic
integration-repair task, run through the same worker contract on the ladder
`premium -> max -> halt`. The repair worker diagnoses the cross-task failure, adds regression
coverage where appropriate, fixes it, re-runs the full suite, and commits the repair. That ladder
starts one rung above the default deliberately: repair is cross-task diagnosis, never routine work.
There is no repeated repair/review loop; failure after the max repair path halts per
*Halting conditions*.

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

**Plan checkboxes are not progress state.** Nobody ticks a plan's `- [ ]` boxes — not you, not a
worker — so a half-ticked plan means nothing, and a resumed run reads commits, code, and tests,
never checkbox marks. Treating a checkbox as evidence of a finished task is a misread docket has
already been burned by.

**`true`** — write a compact ledger to `.superpowers/docket-build/<change-id>/progress.md` (covered
by the committed `.superpowers/` ignore rule) recording branch, plan path and blob hash, task
identity and status, profile and reason, escalation, TDD evidence or exception, verification, and
commit SHA. It is a state ledger, not a prose task report. On resume, skip a task **only** when its
ledger entry is `COMPLETE`, the plan hash still matches, and its commit is an **ancestor** of the
current branch — missing, stale, malformed, or contradictory state never marks a task complete.

## Output

Emit concise, stable lines and nothing more: task-to-profile selection and reason; escalation and
reason; worker outcome and commit; focused verification; full-suite command and result; the
build-evidence record on green; the terminal build disposition. Write no verbose task artifact
unless `BUILD_CHECKPOINT` is `true`.
Material TDD exceptions and residual risks flow into the PR description or the results artifact,
not into per-task files.
