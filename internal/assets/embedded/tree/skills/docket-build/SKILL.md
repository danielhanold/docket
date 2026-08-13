---
name: docket-build
description: Use as docket's build role (skills.build) — executes an implementation plan task-by-task by routing each task to a named economy/standard/premium/max profile agent running the docket-build-task contract, with one bounded escalation per task, no per-task review, and a single full-suite gate at the end.
---

# docket-build — profile-routed plan execution

docket's build role, bound by `skills.build`. You run inside `docket-implement-next` Step 5 with
the plan written and the worktree cut: read the plan, route each task to a profile, dispatch one
fresh worker per task, apply the escalation protocol, run the build gate. Then you stop — review
is not yours.

**Scope of this stop:** if you invoked this skill yourself, this stop ends only the build role —
you continue to your own next step; only an agent whose entire assignment is this role ends its
turn here.

You are not a router subagent: routing is a decision you make in this context. Each selected task
gets exactly one fresh worker dispatch unless that worker requests its single allowed escalation.

## Inputs

- The **plan** `docket-implement-next` Step 4 wrote, at the path recorded in the change's `plan:`
  field and committed on the feature branch.
- The **feature branch and worktree** already cut for this change, plus that repo's own
  instruction files (`AGENTS.md`, `CLAUDE.md`, nested equivalents).
- The plan's `### Task N` headings, the **unit of dispatch** — one heading, one worker, one
  commit; a plan whose tasks are not separable at that boundary is a planning defect, not
  something to re-cut here.

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
docket's ordinary generated-agent layer over the shipped `agents/harness-defaults.yml`; an
unmapped harness/profile pair runs unpinned. Never restate literal model IDs or effort tiers in
your dispatch prose.

## Routing

**Explicit override wins.** A plan task may carry a line of the form:

```markdown
**Build profile:** economy
```

A valid value (`economy`, `standard`, `premium`, `max`) is authoritative; record its use in that task's
routing line. An **invalid** value is a plan contract error: **halt** per *Halting conditions* and
surface it — never silently fall back to a default.

**Otherwise classify** using the shared character→profile rubric in
[`references/task-routing.md`](references/task-routing.md) — the same rubric
`docket-implement-next`'s Step 6 fix loop reads, which is why it lives in a file rather than here.
**Read it now (blocking) before routing your first task.** It carries the deliberate asymmetry
(`economy` positively established, uncertainty sinking to `standard`), the `max`/`premium`
organizing principle, and the four tier bullets. Never restate it in this file or in your dispatch
prose.

For docket-build specifically, `max` has exactly three doors: the rubric's two-item direct
classification, an explicit plan override, and a `premium` escalation.

Emit one concise routing line per task naming both the profile and its reason.

## Dispatching a task

Dispatch the profile agent **by name**, foreground, one task at a time — later tasks build on
earlier task commits and share the worktree, so workers are strictly sequential. Give the worker:
the plan task text, the branch and worktree, the applicable repository instructions, the selected
profile and routing reason, and the completion schema. Never dispatch a task reviewer, and
never dispatch two workers concurrently — that binds a controller who *believes the first worker
is gone* exactly as it binds one dispatching deliberately. Never preload a review skill either —
for a **named** agent the wrapper's own `skills:` frontmatter is the operative protection, so what
this rule actually forbids is bolting a review skill or a review instruction onto the dispatch
prompt.
A worker reached through a runner delegation receives its worktree through the facade's
`--worktree` flag, not through the prompt body alone.

If profile dispatch is genuinely unavailable — established only per the convention's
*Dispatch-capability resolution*, **never from a tool name** — this role is
**Tier C, authorized-or-halt**: only an explicitly configured `skills.build: auto` authorizes
inline execution. Selecting `docket-build` is not implicit authorization to discard its isolation
or its model/effort contract, so halt per *Halting conditions* instead.

A profile agent that is **not registered on this machine** is the same authorized-or-halt
condition, reached differently: the harness rejected a dispatch naming `docket-build-economy` — a
concrete rejection of a named agent, never an inference about dispatch capability from a missing
tool name, so the rule above stands unchanged. The cause is a stale install: `install.sh`
generates the profile wrappers, and a harness registers them only at session start. Halt, naming
a re-run of `install.sh` plus a fresh session as the remedy.

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
uncommitted changes the weaker worker left — revise, never blindly discard. A successful
escalation continues this run automatically.

A failed attempt that left a **commit** — not merely a dirty tree — is different, and it is the
one state that cancels the escalation. The worker was told never to commit on `NEEDS_ESCALATION` or
`BLOCKED`, but a crashed or truncated one still can; the escalated worker is separately forbidden
to rewrite earlier task commits, so it would inherit state it cannot clean up, and this task's
exactly-one-commit accounting is already contaminated. **Do not escalate onto a stray commit** —
halt per *Halting conditions*, naming the stray SHA so a human can inspect, keep, or drop it.

## Halting conditions

Every halt is the same disposition: stop, return `halted` — a build outcome, not
`docket-implement-next`'s run disposition of the same name — the change
stays `in-progress` and the worktree is preserved for inspection or resume.

**Scope of this halt:** if you invoked this skill yourself, it ends only the build role — you
continue to your own next step, `halted` in hand; only an agent whose entire assignment is this
role ends its turn here.

Report which condition below fired with its evidence (task, profile, SHA, command, or harness
message). Never improvise past one, never substitute a weaker path, and never invoke review. The
rules elsewhere in this file name their condition and point here rather than restating the
disposition.

- **Profile routing is un-dispatchable**, established per the convention's *Dispatch-capability
  resolution* and never from a tool name, and `skills.build: auto` was not explicitly configured.
- **A profile agent is not registered on this machine** — the harness rejected a dispatch naming
  it. Remedy: re-run `install.sh`, then start a fresh session.
- **An explicit plan `Build profile:` value is invalid** — a plan contract error; never fall back
  to a default.
- **A worker return is malformed or unverifiable** — a missing or unparsable outcome, a `COMPLETE`
  whose commit is absent, unresolvable, or not an ancestor of the branch tip, or a
  `NEEDS_ESCALATION` with no concrete reason. Never re-dispatch a task to repair its own return,
  and never discard the worktree and dispatch a fresh worker for that task either: a worker you
  did not observe return cleanly may still be running, and it wakes into the same worktree its
  replacement is writing. Name the task and the worktree.
- **A task's escalation allowance is exhausted** — an initial `max` worker requests escalation,
  or an escalated worker still cannot finish.
- **A failed attempt left a commit** — name the stray SHA; do not escalate onto it.
- **No suite is detectable** — no `FINALIZE_TEST_COMMAND` and nothing finalize's auto-detection
  recognizes. Remedy: set `finalize.test_command`. Never convert this into a repair task.
- **The observation budget is exhausted with no terminal gate result** — `GATE_OBSERVATION_BUDGET`
  ran out and no durable result artifact reports a terminal state. Fail closed: an unfinished run is
  not a failing suite, so never convert this into a repair task and never infer success.
- **The suite is still red after the max repair** — there is no second repair round.
- **Continuation is unsafe** — a worker's `BLOCKED`: contradictory requirements, missing authority,
  or an absent dependency.

## The build gate

Workers run focused tests only. After every plan task has committed, run the **whole suite once**:

1. Use the already-resolved `FINALIZE_TEST_COMMAND` when it is non-empty.
2. Otherwise reuse finalize's existing suite **auto-detection**.
3. **Neither** — no `FINALIZE_TEST_COMMAND` *and* nothing the auto-detection recognizes — is a
   **configuration gap, not a red suite**. A repo with no matching test files leaves the detection
   glob literal and exits non-zero; reading that as RED would manufacture a repair task. Finalize
   itself aborts here rather than repairing, and so do you: halt per *Halting conditions*, naming
   `finalize.test_command` as the remedy.

The command boundary is the one finalize already publishes — its `configured-bash-finalize` marker
block in `skills/docket-finalize-change/SKILL.md` is the single source, and the awkward `finalize`
namespace is deliberately kept rather than introducing a second, driftable test command. Do not
copy that fragment into this file.

The verdict is an **exit status, never output text**. A run is **green if and only if the resolved
suite command exits zero**; any non-zero status is not green. A `PASS`/`FAIL` line, a summary count,
or a progress ticker is **diagnostic only** — a gate that reads its verdict out of the output is not
a gate. The deciding status is the one recorded in the **terminal result artifact** that *Gate
execution posture* requires: **completed successfully** means that artifact records a zero status.
*Still running* and *result unavailable* are not verdicts, so they stay budget halts and are
never red. Nor is every non-zero status red: a completed run whose recorded status the resolved
runner defines as a **non-failure** outcome is a halt per *Halting conditions*, the same refusal the
configuration gap gets — neither has a failure to repair. **Red** is a completed run that is neither
green nor one of those halts. When the resolved command is a **loop over per-file commands** — the shape finalize's
`configured-bash-finalize` block takes when `FINALIZE_TEST_COMMAND` is unset — the deciding status
is the **aggregate** that block exits with, never any individual file's. This rule binds every
full-suite run this role performs, including the repair worker's post-fix re-run below.

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

### Gate execution posture

The suite may take longer than the harness will hold a foreground call open, so the gate is
specified by capability rather than by mechanism. A harness's foreground-call timeout does **not**
define the maximum duration of the build gate.

1. Do **not** depend on a single foreground call remaining attached until the suite completes. Gate
   execution must be able to outlive any individual foreground call used to start or observe it.
2. The gate writes its eventual outcome to a **durable result artifact** — readable after a yield,
   outside the committed tree, and non-colliding between concurrent gates. Where it lives is a
   per-harness decision, not a contract value.
3. Gate completion is established **from that artifact**, never from the caller-visible completion
   signal of the command that started the gate.
4. Whether you may **yield** while the gate runs is decided by *your own* dispatch posture, never by
   the gate's. Only a **top-level session agent**, able to receive a resumption signal, may yield and
   then make short observations of that artifact. A build role running as a **dispatched or forked
   child** has no such channel, so it may **never** yield: it observes by *blocking* instead —
   repeated short foreground reads of the artifact, control never handed back to its caller mid-gate.
5. Observation is **bounded** by a finite budget — never wait indefinitely. That budget is
   `GATE_OBSERVATION_BUDGET` (default 30, in minutes) from the Step-0 config export: docket
   execution policy, distinct from any foreground-call timeout a particular harness imposes. The
   observation interval is an implementation detail; what the contract requires is that each
   observation is short-lived and the whole period finite. A budget of `0` is legal and is not a
   disabled gate: it buys exactly **one** observation of the artifact, taken once, before the
   budget is spent.
6. If no terminal result artifact exists when the budget is exhausted, **fail closed** — halt per
   *Halting conditions*. Under a `0` budget that verdict is reached after the single observation
   clause 5 grants, never before it. Never infer success, and never turn it into a red suite: an
   unfinished run
   is not a failing one, so it must **not** mint an integration-repair task. Same refusal the
   configuration-gap case above already gets.

**The shipped implementation of clauses 1–3** is
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh gate-run` — `--launch` starts the suite
detached and durable, `--observe` is each short-lived look, `--stop` terminates one. **Key the wait
on the state each observation reports, never on a success marker appearing in the log.** The two
differ exactly when the child dies, which is the one moment the wait exists for: a marker-keyed loop
cannot tell *still running* from *died*, so it burns its whole budget before reporting a death a
state-keyed wait catches on the next observation. The six states and their retryability are
`gate-run.md`'s contract, and **only `running` is retryable**. **Reuse the canonical loop** in
`gate-run.md` § *The caller's loop* verbatim rather than authoring one, and key each `case` arm on
the full printed `state=<name>` line: a loop that re-tokenizes that line and matches bare state
names matches nothing, so it never terminates on a state — it polls a finished gate until the
budget is spent.

**On a failed launch.** `--launch` prints either the run dir's absolute path or the token
`launch-failed` — a **slash-free token rather than an absolute path**, which is the shape a caller
keys on instead of hand-rolling its own failure detection. `launch-failed` is
**abort-and-report** per *Halting conditions*: never a retry loop, and never observed, since no
handle exists to observe.

**On the died state.** The child never finished, so it never produced a verdict: `died` is **not** a
red suite and **never** mints repair work. Where the child is **idempotent** — the suite gate is —
the posture is `--stop`, then at most **one** bounded relaunch, gated on the token `--stop` reports.
Two vocabularies overlap here and **one spelling appears in both**: `stopped` is a `--stop` token
*and* an `--observe` state, with opposite dispositions — so each bullet below is a **token the verb
`--stop` reports**, and every state named inside a bullet is written `state=<name>` for the value
the verb `--observe` returns:

- `already-terminal` (stop token) — the **ordinary** outcome of stopping a live child, and also what
  an already-absent run reports; re-observe first and key on the state that comes back:
  `state=passed` or `state=failed` keep that verdict (the run finished after all), `state=died`
  takes the one relaunch, `state=stopped` and `state=unavailable` never relaunch.
- `stopped` (stop token) — the run was signalled and verified gone with no verdict of its own.
  Relaunch once.
- `unavailable` (stop token) — abort and report **without** relaunching: what survives could not be proven to be
  this run's, so a relaunch would race a suite that is still live.

A second `died` is abort-and-report, never a third attempt. Where the child is **non-idempotent**,
the relaunch is not licensed at all and the site keeps its existing failure posture — the permission
comes from idempotence, not from the state.

**Abandoning a live child.** A caller that stops observing while the state is still `running` —
budget exhausted, halt, or abort — calls `--stop` **before it reports**, so no suite outlives the run
a human is about to inspect. Every leg then halts per *Halting conditions*; the `unavailable` leg
halts **loudly**, because that is the one leg where the human inherits a live process.

**The false-completion rule.** A caller-visible completion signal is never gate completion.
Reciprocally, a **stale pre-yield report is not evidence of a crashed run**: an observer seeing a
completion signal that carries pre-yield text resolves the run's state from git and from the durable
artifact before concluding anything. *Reading a worker's return* states this for a worker's report;
it holds for the gate.

**This does not relax the never-yield rule for dispatched subagents.** Two boundaries are in play: a
**dispatched subagent** yielding control in violation of its execution contract, and an external
**gate process** continuing independently while the responsible agent performs bounded observations
of its durable result. Only the second is permitted here, and it is never permission for dispatched
agents to yield across execution phases. Which branch of clause 4 applies is therefore settled by
*who observes*, not by what is running: the yield belongs to a **top-level session agent** only, and
on docket's own default path there is none — this role is invoked inside `docket-implement-next`
Step 5, which is itself dispatched. Blocking observation is the norm here; the yield is the
exception. Not hypothetical: dispatched build workers here have yielded to await a gate completion
event and gone unresumed.

Which capabilities a harness must have to host such a gate, and the measured verdict for each
harness docket ships, are quarantined in
[`references/gate-execution.md`](references/gate-execution.md) — **read it now (blocking) before
starting the gate.**

## Review boundary

This build performs **no per-task independent review** and **no final review of its own**. The
worker's self-review is part of implementation, not a second agent or an adversarial gate. Docket's
single independent whole-branch review remains `docket-implement-next` Step 6's `skills.review`
role, which stays separately configurable.

## Checkpointing

Read `BUILD_CHECKPOINT` from the Step-0 config export.

**`false` (default)** — persist nothing. Completed work is durable through the per-task code
commits; keep only the compact in-context worker returns; write no `.superpowers/docket-build/`
files. A resumed run reconstructs progress conservatively from the plan, commits, code, and tests.

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
build-evidence record on green; the terminal build disposition (**role-scoped** — a build
disposition, never a run disposition). Write no verbose task artifact
unless `BUILD_CHECKPOINT` is `true`.
Material TDD exceptions and residual risks flow into the PR description or the results artifact,
not into per-task files.
