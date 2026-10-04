---
name: docket-build
description: Use as docket's build role — executes an implementation plan task-by-task by routing each task to a named economy/standard/premium/max tier agent running the docket-build-task contract, with one bounded escalation per task, no per-task review, and a single full-suite gate at the end.
---

# docket-build — tier-routed plan execution

docket's build role. You run inside `docket-implement-next` Step 5 with
the plan written and the worktree cut: read the plan, route each task to a tier, dispatch one
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

## Tiers

Four named agents, all preloading the same `docket-build-task` worker skill and differing only in
model and effort:

| Agent | Use |
|---|---|
| `docket-build-economy` | fully specified, pattern-following, no consequential risk, no cross-file reasoning |
| `docket-build-standard` | normal feature, integration, refactor, and debugging work — the default |
| `docket-build-premium` | consequential but correctable risk, or a risk the plan names |
| `docket-build-max` | unresolved architecture, or an irreversible data change |

A higher tier means greater reasoning investment, **not** a stronger correctness guarantee — every
tier carries identical testing and completion obligations. Model and effort resolve through
docket's ordinary generated-agent layer over the shipped `agents/harness-defaults.yml`; an
unmapped harness/tier pair runs unpinned. Never restate literal model IDs or effort tiers in
your dispatch prose.

## Routing

**Explicit override wins.** A plan task may carry a line of the form:

```markdown
**Build tier:** economy
```

A valid value (`economy`, `standard`, `premium`, `max`) is authoritative; record its use in that task's
routing line. An **invalid** value is a plan contract error: **halt** per *Halting conditions* and
surface it — never silently fall back to a default.

**Otherwise classify** using the shared character→tier rubric in
[`references/task-routing.md`](references/task-routing.md) — the same rubric
`docket-implement-next`'s Step 6 fix pass reads, which is why it lives in a file rather than here.
**Read it now (blocking) before routing your first task.** It carries the deliberate asymmetry
(`economy` positively established, uncertainty sinking to `standard`), the `max`/`premium`
organizing principle, and the four tier bullets. Never restate it in this file or in your dispatch
prose.

For docket-build specifically, `max` has exactly three doors: the rubric's two-item direct
classification, an explicit plan override, and a `premium` escalation.

Emit one concise routing line per task naming both the tier and its reason.

## Dispatching a task

<!-- docket:feature-dispatch:start targets=docket-build-economy,docket-build-max,docket-build-premium,docket-build-standard -->
Dispatch the selected tier agent **by name** — one of `docket-build-economy`,
`docket-build-standard`, `docket-build-premium`, or `docket-build-max` — foreground, one task at a
time; later tasks build on earlier task commits and share the worktree, so workers are strictly
sequential. Its dispatch payload contains:
Feature worktree: <absolute canonical feature-worktree root>
It also gives the worker the branch name, the plan task text, applicable repository instructions, selected
tier and routing reason, and the return schema. No run context or capability goes into a
worker prompt: the worker runs its tests directly and calls no gate operation. Never dispatch a task reviewer, and
never dispatch two workers concurrently — that binds a controller who *believes the first worker
is gone* exactly as it binds one dispatching deliberately. Never preload a review skill either —
for a **named** agent the wrapper's own skills list is the operative protection, so what it
forbids is bolting a review skill or instruction onto the dispatch prompt.
<!-- docket:feature-dispatch:end -->

A tier agent that cannot be dispatched — established only per the convention's
*Dispatch-capability resolution*, **never from a tool name** — is the **`halt`** posture: halt per
*Halting conditions*, and never execute tasks inline.

A tier agent **not registered on this machine** is the same `halt` condition reached
differently: the harness rejected a dispatch naming `docket-build-economy` — a concrete rejection of a
named agent, never an inference from a missing tool name, so the rule above stands. The cause is a
stale install (the `install` operation installs the wrappers; a harness registers them only at session start):
halt, naming a re-run of the `install` operation plus a fresh session as the remedy.

## Reading a worker's return

Valid outcomes are `COMPLETE`, `NEEDS_ESCALATION`, and `BLOCKED`; any other token, or a **missing or
malformed outcome, halts** the build. Never infer success from a child reporting it finished: a completion report
is unreliable in **both** directions, so every claim is settled against git state, never the return's
prose — a SHA-shaped string somewhere in the text is not a commit.

**Malformed is wider than an unparsable token.** Before accepting a `COMPLETE`, verify the claimed
commit: the SHA must resolve in this repository *and* be an ancestor of the branch tip
(`git merge-base --is-ancestor <sha> HEAD`). A `COMPLETE` whose commit is absent, unresolvable, or
not on this branch is a malformed return — halt per *Halting conditions*, and never re-dispatch the
task to "fix" its own return. A `COMPLETE` must equally carry the focused verification result, and a
task without a commit is not complete; a `NEEDS_ESCALATION` with no concrete reason is malformed the
same way (see *Escalation*).

**Time-limit audit — visibility only.** For every return, check each test command in
`VERIFICATION` for the `timeout --kill-after=10s 10m` wrapper (or `gtimeout`).
- A command without it, or a `VERIFICATION` line too unclear to tell, becomes one informational
  line in the results file's `## Verification performed` section, through the worker-surfaced
  findings path (*Build-findings checkpoint*): "task N: `<command>` ran without the 10-minute
  `timeout` wrapper — informational, no action required; the full-suite gate certified the branch."
- The audit is never a halting condition and never makes a return malformed. It never
  re-dispatches, escalates, or re-runs a test, and never casts doubt on the task's commit; it adds
  no operation, state, transaction, or commit of its own and rides on the results checkpoint the
  coordinator already writes.
- It catches an honest omission. It cannot catch a worker misreporting what it ran.

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
assigned tier. An expected RED test, ordinary debugging, or a single failed test run is not an
escalation condition; a `NEEDS_ESCALATION` without such a reason is a **malformed return**, and a
malformed return halts — never a free escalation.

The stronger worker continues in the **same worktree** and must inspect and account for any
uncommitted changes the weaker worker left — revise, never blindly discard. A successful
escalation continues this run automatically.

A failed attempt that left a **commit** — not merely a dirty tree — is the one state that cancels
the escalation: a crashed or truncated worker can commit despite the ban, and the escalated worker,
forbidden to rewrite earlier task commits, would inherit state it cannot clean up while this task's
exactly-one-commit accounting is already contaminated. **Do not escalate onto a stray commit** — halt
per *Halting conditions*, naming the stray SHA so a human can inspect, keep, or drop it.

## Halting conditions

Every halt is the same disposition: stop, return `halted` — a build outcome, not
`docket-implement-next`'s run disposition of the same name — the change
stays `in-progress` and the worktree is preserved for inspection or resume.

**Scope of this halt:** if you invoked this skill yourself, it ends only the build role — you
continue to your own next step, `halted` in hand; only an agent whose entire assignment is this
role ends its turn here.

Report which condition below fired with its evidence (task, tier, SHA, command, or harness
message). Never improvise past one, never substitute a weaker path, and never invoke review. The
rules elsewhere in this file name their condition and point here rather than restating the
disposition.

- **Tier routing is un-dispatchable**, established per the convention's *Dispatch-capability
  resolution* and never from a tool name.
- **A tier agent is not registered on this machine** — the harness rejected a dispatch naming
  it. Remedy: re-run the `install` operation, then start a fresh session.
- **An explicit plan `Build tier:` value is invalid** — a plan contract error; never fall back
  to a default.
- **A worker return is malformed or unverifiable** — a missing, unparsable, or unknown outcome, a `COMPLETE`
  whose commit is absent, unresolvable, or not an ancestor of the branch tip, or a
  `NEEDS_ESCALATION` with no concrete reason. Never re-dispatch a task to repair its own return,
  and never discard the worktree and dispatch a fresh worker for that task either: a worker you
  did not observe return cleanly may still be running, and it wakes into the same worktree its
  replacement is writing. Name the task and the worktree.
- **A task's escalation allowance is exhausted** — an initial `max` worker requests escalation,
  or an escalated worker still cannot finish.
- **A failed attempt left a commit** — name the stray SHA; do not escalate onto it.
- **The build gate has no command** — `build_gate` is `local` but `build_test_command` is empty. A
  configuration gap, not a red suite. Remedy: `docket repository configure-tests`. Never convert this
  into a repair task.
- **The driver's observation budget is exhausted with no terminal gate result** — the gate driver
  spent its budget and no durable result artifact reports a terminal state. Fail closed: an unfinished run is
  not a failing suite, so never convert this into a repair task and never infer success.
- **The suite-attempt budget is exhausted and the suite is still red** — the last permitted
  full-suite run failed, or `gate.drive.start` refuses with `suite-attempts-exhausted`; there is no
  repair round beyond the budget.
- **Continuation is unsafe** — a worker's `BLOCKED`: contradictory requirements, missing authority,
  or an absent dependency.

## The build gate

Workers run focused tests only. After every plan task has committed, apply this role's own gate
policy. A worker's focused tests do **not** substitute for this final full-suite gate: they cover
only what each worker ran, and this is the one run that certifies the branch. On a final
`PASSED`, the drive's raw run directory feeds the existing evidence operation. The policy arrives in
the implementation context as `build_gate`, `build_test_command`, and `build_max_attempts` —
authoritative config the build role reads, never a command it invents:

1. **`build_gate: off`** — the repo declares no build gate. Run **nothing**: mint truthful
   **skipped** evidence via the `evidence.record` operation (no run dir) — `result: skipped` /
   `reason: build-gate-off` at the current head — and proceed to review. Nothing to run or repair.
2. **`build_gate: local`, non-empty `build_test_command`** — drive it through the native gate
   **driver**: the `gate.drive.start` operation with `--owner build --change-id <id>
   --run-context <token> --json` (`--run-context` only when your prompt carried it; `--change-id` always — it is what charges `build_max_attempts` and lets the run
   tracker find the drive) — capture that first response into `gate_reply` (its exit
   code, if needed, into `gate_rc`; never a zsh read-only special parameter such as
   `status`) and read the drive id and owner generation from it — then `gate.drive.advance` operation slices,
   exactly as *Gate run posture* describes. `--owner build` resolves the build-owned command
   from config; the caller passes no suite argv.
3. **`build_gate: local`, empty `build_test_command`** — a **configuration gap, not a red suite**:
   nothing to run, no failure to repair, and reading an empty command as RED would manufacture a
   repair task. Halt per *Halting conditions*, remedy `docket repository configure-tests`.

The verdict is an **exit status, never output text**. A run is **green if and only if the resolved
suite command exits zero**; any non-zero status is not green. A `PASS`/`FAIL` line, a summary count,
or a progress ticker is **diagnostic only** — a gate that reads its verdict out of the output is not
a gate. The deciding status is the one recorded in the **terminal result artifact** that *Gate
run posture* requires: **completed successfully** means that artifact records a zero status.
*Still running* and *result unavailable* are not verdicts, so they stay budget halts and are
never red. Nor is every non-zero status red: a completed run whose recorded status the resolved
runner defines as a **non-failure** outcome is a halt per *Halting conditions*, the same refusal the
configuration gap gets — neither has a failure to repair. **Red** is a completed run that is neither
green nor one of those halts. When the resolved command is a **sequence of per-file commands**, the
deciding status is the **aggregate** the sequence exits with, never any individual file's. This rule
binds every full-suite run this role performs, including every post-repair attempt below.

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

**`build_gate: off`** mints the **skipped** variant of the same marker-bounded block instead: drop
`command` (nothing ran) and swap in `result: skipped` plus `reason: build-gate-off`, keeping
`head_sha` and `ran_at`.

The record certifies the branch so the review step need not re-run the suite; `docket-implement-next`
Step 6 validates it and Step 7 writes it into the PR body, then runs the review role (`docket-review`)
once over the whole branch. Only a green run — or an explicit `build_gate: off` — mints a
record: a red suite mints no evidence record at all, and enters the repair path below.

**Red** → the build **never invokes review**. `build_max_attempts` (from the implementation context,
default 4) caps the full-suite runs this phase may spend, counting the initial run;
`build_max_attempts: 1` means a red initial run halts with no repair cycle. Otherwise:

1. Each red full-suite result becomes exactly one synthetic integration-repair task, run through
   the same worker contract on the ladder `premium -> max -> halt` — one tier above the default
   deliberately: repair is cross-task diagnosis, never routine work. The repair worker diagnoses
   the failure, adds regression coverage where appropriate, fixes it, re-runs the failing tests
   directly as its focused check, and commits; it never runs the full suite.
2. When the repair worker returns `COMPLETE`, you start the next counted attempt yourself with the
   same build-owned start — the `gate.drive.start` operation with `--owner build --change-id <id>
   --run-context <token> --json` (flags as above) — and drive it to a final result
   exactly as *Gate run posture* describes, so the facade charges it with no bypass.
3. **Green at any point ends the phase immediately; review is never invoked while red.** A red
   result becomes the next repair task while attempts remain. A refused start
   (`suite-attempts-exhausted`) or a red final permitted run halts per *Halting conditions* with
   the exhaustion reason naming `build.max_attempts` and used/limit.

`build_gate: off` runs no suite and spends no attempt, and an infrastructure, result-unavailable,
configuration-gap, or observation-budget halt is unchanged and is **not** a red result to repair.

### Gate run posture

The suite may take longer than the harness will hold a foreground call open, so the gate is
specified by capability rather than by mechanism. A harness's foreground-call timeout does **not**
define the maximum duration of the build gate.

1. Do **not** depend on a single foreground call remaining attached until the suite completes. Gate
   runs must be able to outlive any individual foreground call used to start or observe them.
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
5. Observation is **bounded** by a finite budget — never wait indefinitely. The gate driver enforces
   that budget itself and fails closed when it is spent; no skill reads or passes it. It is docket
   execution policy, distinct from any foreground-call timeout a particular harness imposes. The
   observation interval is an implementation detail; what the contract requires is that each
   observation is short-lived and the whole period finite. A budget of `0` is legal and is not a
   disabled gate: it buys exactly **one** observation of the artifact, taken once, before the
   budget is spent.
6. If no terminal result artifact exists when the budget is exhausted, **fail closed** — halt per
   *Halting conditions*. Under a `0` budget that verdict is reached after the single observation
   clause 5 grants, never before it. Never infer success, and never turn it into a red suite: an
   unfinished run is not a failing one, so it must **not** mint an integration-repair task — the same
   refusal the configuration-gap case above already gets.

**The shipped implementation of clauses 1–6** is the native gate **driver** — the `gate.drive`
operations (`start`, `advance`, `handoff`, `claim`), whose caller-side contract and disposition
vocabulary live in `references/gate-driver.md` (**read it now, blocking, before the gate**).
Drive the suite through short synchronous `gate.drive.start` then `gate.drive.advance` calls; the
driver composes the raw supervisor and owns the detached run, durable drive record, artifact-based
completion, and fail-closed budget. **Reuse the driver, never a hand-rolled polling script** — a hand-rolled
sleep-and-parse over `gate.observe` can spin a gate until a human resumes it; the raw `gate.launch`/`observe`/`stop` operations are primitives, never this role's workflow API.

**Keying on the disposition.** Key the wait on the typed disposition the driver returns
(`WAITING`/`PASSED`/`FAILED`/`HALTED`), never on a success marker in the log — a marker-keyed reading
cannot tell *still running* from a process death, the one moment the wait exists for. `WAITING` is
the only nonterminal disposition and the only one that advances again. Only `FAILED` — the suite ran
and went red — feeds repair; a process death, a changed worktree, uncertain ownership, deadline expiry, or
malformed observation is `HALTED`, **not** a red suite and it **never** mints repair work. The
caller never relaunches, stops a raw run, or composes the raw verbs; a gate whose supervisor dies
halts `supervisor-died`, never relaunched.

**Abandoning a live drive.** A caller that must stop while the drive is still `WAITING` — budget
exhausted, halt, or abort — performs an explicit `gate.drive.handoff` operation **before it reports**,
returning the drive id, phase, and single-use handoff token so the nearest live owner can `claim` and
continue — no suite stranded, backgrounded, or notification-waited. Every leg then halts per *Halting
conditions*; the leg where the handoff itself is **unavailable** halts **loudly**, the one leg where a
human inherits a live drive.

**The false-completion rule.** A caller-visible completion signal is never gate completion.
Reciprocally, a **stale pre-yield report is not evidence of a crashed run**: an observer seeing a
completion signal carrying pre-yield text resolves the run's state from git and the durable artifact
before concluding. *Reading a worker's return* states this for a worker's report; it holds for the
gate.

**This does not relax the never-yield rule for dispatched subagents (clause 4).** An external **gate
process** continuing while the responsible agent makes bounded observations of its durable result is
permitted; a **dispatched subagent** yielding control across execution phases is not. Which branch
applies is settled by *who observes*, not by what is running — the yield belongs to a **top-level
session agent** only, and docket's default path has none: this role runs inside
`docket-implement-next` Step 5, itself dispatched. Not hypothetical — dispatched build workers here
have yielded to await a gate completion event and gone unresumed.

### Build-findings checkpoint

When docket-build runs as the invoked build role for a coordinator that owns a results artifact,
the controller **may** perform the build-findings checkpoint on that coordinator's behalf, under
the explicit caller contract that grants it that authority — consolidating the build findings
available at this point (the material TDD exceptions, residual risks, and worker-surfaced findings
above) before the final full-suite gate where the ordering permits, so the gate certifies the
checkpoint-containing head. **Task workers never edit the results file**, there are **never
concurrent writers**, and **a checkpoint never independently launches tests** — it records what is
already known and moves no gate of its own. A custom build skill bound in this role's place owns
none of this: it returns its findings through its own contract, and the coordinator captures them at
the next safe boundary it controls.

## Review boundary

This build performs **no per-task independent review** and **no final review of its own**. The
worker's self-review is part of implementation, not a second agent or an adversarial gate. Docket's
single independent whole-branch review remains `docket-implement-next` Step 6's review role (`docket-review`).

## Checkpointing

Persist nothing. Completed work is durable through the per-task code commits; keep only the compact
in-context worker returns. A resumed run reconstructs progress conservatively from the plan,
commits, code, and tests.

**Plan checkboxes are not progress state.** Nobody ticks a plan's `- [ ]` boxes — not you, not a
worker — so a half-ticked plan means nothing; a resumed run reads commits, code, and tests, never
checkbox marks. Treating a checkbox as evidence of a finished task is a misread docket has been
burned by.

## Output

Emit concise, stable lines and nothing more: task-to-tier selection and reason; escalation and
reason; worker outcome and commit; focused verification; full-suite command and result; the
build-evidence record on green; the terminal build disposition (**role-scoped** — a build
disposition, never a run disposition). Write no verbose task artifact; material TDD exceptions,
residual risks, and worker-surfaced findings flow to the coordinator's results artifact (and the PR description where evidence belongs), not into per-task
files.
