# Build: Building without supervision

By the end of this page you can hand a designed piece of work to implement-next and get back an
open pull request, without babysitting it — and you will understand what a build run checks before
it writes a line of code, how it decides how hard to work on each part, and where it stops and waits
for you. The one thing it never does is merge; that stays your call.

## Why hand work to implement-next at all

If you already run coding agents in your repos, you have probably felt the gap this fills.

Some agent workflows give you excellent execution but no memory. A brainstorm-to-merge run works
well, but everything happens inside a single session: there is no tracked backlog, and no "done"
state that outlives the session. Every session starts from a blank slate. Heavier tools close that
gap by adding a full living-spec lifecycle, but at the cost of a command-line dependency and a rigid
markdown contract not every project wants to adopt.

Docket sits in between. It adds a thin lifecycle layer — plain markdown files in your repo, a
handful of skills (a **skill** is a named, reusable instruction set an agent loads for one job), and
its own `docket` binary, which the skills call for every bookkeeping step. Brainstorming, planning,
and finishing a branch use the superpowers skills; building and reviewing use docket's own
`docket-build` and `docket-review` skills. The code stays the single source of truth about current state:
docket keeps no living-spec layer and never tries to mirror your codebase in prose. The decisions
behind the code are recorded separately, as ADRs (an **ADR** is an architecture decision record: one
file per decision, immutable once accepted).

But the thin lifecycle is not the real reason to use docket. The reconcile step is.

## Reconcile: killing stale work before any code

This is the most valuable and least obvious part of a build run.

**The problem.** A change (one unit of planned work, roughly one pull request, tracked as one
markdown file) is drafted against a *snapshot* of the world — the codebase, the recorded decisions,
and the other in-flight work as they stood the day you wrote it. In a durable backlog, a build run may
not pick that work up for a week or a month. By then another change may have already shipped what
this one planned to add; a decision may have settled an open question in the opposite direction; or
a dependency may have changed the interface this design assumed.

Most backlog-driven systems build the ticket as written and let the worker discover the mismatch
halfway through. The classic results: re-implementing something already done elsewhere, or building
something a later decision has invalidated.

**What a build run does instead.** Every run includes a **reconcile** step (a check at build time that
the change is still worth doing and its assumptions still hold, before any code is written). It runs
at the last responsible moment — *after* the change is claimed, so it belongs to this run, but
*before* the working copy and the plan are created, so no build effort is wasted if the scope
shifts. The reconcile pass re-reads the change and its spec (the design document a change links to,
written before building) against related and recently archived changes (to find work already done),
cited and recent decisions (to find new constraints), and the current code (to find interface
drift). It then rewrites the change to what is true now: it drops work done elsewhere, adjusts
scope, and folds in new constraints, leaving a dated reconcile-log entry and a `reconciled: true`
mark as an audit trail.

Two escape hatches handle what a rewrite cannot. If the change is now entirely obsolete, it is
killed and the run moves on to the next one. If the design is fundamentally invalidated — it needs
re-thinking, not just scope-trimming — the run stops and escalates to you, because re-designing
needs a human and a build run will not do it alone.

The stance: **plans rot; refresh them just-in-time, and never trust a stale backlog.** If an
interrupted run resumes with `reconciled` still `false`, the full reconcile pass runs again before
any work continues.

## What one unattended run does, end to end

A single implement-next run walks a fixed path and then stops:

1. **Pick.** It selects the next **build-ready** change (a proposed change that has a spec or is
   marked trivial and whose dependencies are all `done`). It never touches work that is not ready.
2. **Claim.** It takes a **claim** on that change (the moment a change is picked up for building; it
   records which branch will carry the work and when it was taken) with a conflict-checked write on the
   change's status, so two runs in parallel can never grab the same change. The claim carries a
   **claim lease** (a timestamp on a claim; when it expires with no branch behind it, the change
   goes back to the queue), which is what lets a crashed run's change self-heal instead of sitting
   stuck — see [Keeping the backlog honest](./keeping-the-backlog-honest.md).
3. **Reconcile.** The step above — freshen the change against reality, or kill or escalate it.
4. **Plan.** It cuts a feature branch, creates an isolated working copy of the repo on that branch
   (a *worktree*), commits a copy of the change's spec as the branch's first commit, and authors a
   **plan** (the task-by-task breakdown a build follows). The plan lives with the backlog on the
   metadata branch, not with the code; so do the results file and the build evidence, so the pull
   request carries only the spec copy and the code.
5. **Build.** It works the plan task by task, test-first, committing as it goes (the next section
   covers how it decides how hard to work on each task).
6. **Review.** Before the pull request opens, a bounded reviewer reads the whole branch — see
   [Reviewing before the human does](./reviewing-before-the-human.md) — and its findings are fixed
   on the branch.
7. **Stop.** It opens the pull request and stops at the PR handoff. It never merges.

A change whose feature branch is cut from another change's *unmerged* branch is a **stacked change**
(a change built on another change's unmerged branch rather than on the integration branch); its
pull request targets that parent branch rather than your **integration branch** (the branch code
lands on, usually `main`). Everything else cuts straight from the integration branch.

The whole run is unattended between "pick" and "stop": your only required touch-point is reading and
merging the pull request it opens.

### A change that delivers findings, not code

Some work ends in findings rather than code: a measurement, a piece of research, preparation for a
larger effort. Such a change carries `deliverable: results-artifact` (the default is `pr`), and its
run takes a different ending after the claim, reconcile, and worktree steps. There is no plan, no
build, and no pull request. The run does the work the spec describes in the feature worktree, never
commits to or publishes the branch, and keeps the findings, numbers, and the commands that produced
them in the results file. A reviewer checks that the results answer what the spec asked, the run
fixes what it finds in the results, and then it runs `docket change complete`, which moves the change
straight to `done` and archives it, followed by `docket finalize cleanup` to remove the worktree.

If the work turns out to need a code change, or reaches a step only a person can do, the run halts
and says so. You finish such a change yourself: do the remaining steps, attach the results with
`docket change attach-results`, then run `docket change complete` and `docket finalize cleanup`.

## Build tiers and the one escalation

The build step does not treat every task the same. Each task in the plan is routed to one of four
**build tiers** (economy, standard, premium, max — the worker a plan task is
routed to by risk), which share one worker contract and differ only in the model and effort behind
them. Each tier is a separately launched **agent** (a separately launched worker with its own
context, pinned to a model and effort), and the build **dispatches** one per task (launching a named
agent to do a step and waiting for it to return).

The routing is deliberately asymmetric. The cheapest tier, `economy`, must be *positively*
established — the task is fully specified, follows an existing pattern, carries no consequential
risk, and needs no cross-file reasoning. Genuine uncertainty defaults to `standard`, the next tier
up, rather than dropping to the cheaper one: when in doubt, docket spends more, not less. The top
tier, `max`, is deliberately rare — reachable only for unresolved architecture or an irreversible
data change, by an explicit override on the task, or by escalation — so the tier meant for extreme
cases does not become normal. A plan task can override the routing outright with a build-tier
line on that task; an invalid value halts the build rather than silently falling back to a guess.

Each task carries **at most one automatic escalation**, and only ever one tier up: an `economy`
worker that cannot finish retries once at `standard`, a `standard` worker once at `premium`, a
`premium` worker once at `max`. There is never a second climb — a `max` worker that still cannot
finish halts the build for a human rather than climbing again. The concrete payoff of this shape is that a
cheap task costs a cheap worker, a risky task gets a strong one, and a task that turns out harder
than it looked gets exactly one shot at more capability before a human is asked.

Once every task has committed, the build runs the whole test suite once as its gate — that half of
the story, and how the result is certified, is [Proving the build](./proving-the-build.md). The
deeper mechanism behind tier routing and the gate verdict lives in
[Build tiers and the suite gate](../concepts/build-tiers-and-gate.md).

## Draining the queue hands-free

A single run builds one change. To drain a whole backlog you start runs one after another, and
whatever starts them keys on the outcome each run declares. Every run ends by declaring one of four outcomes:

- **advanced** — it built a change and opened a pull request, or completed a results-artifact
  change (done and archived, with its results attached).
- **contended** — it lost a claim race to another run and built nothing.
- **drained** — nothing in scope was build-ready.
- **halted** — something needs a human (an escalation, an invalidated design, a broken precondition).

A driver keys on these with one simple rule: **continue on `advanced` or `contended`, stop on
`drained` or `halted`** — except that a tracked loop carries on past `halted` when the run
tracker's verdict is `run-next` (see [When a change halts mid-drain](#when-a-change-halts-mid-drain)).
The contract is deliberately driver-agnostic — a human re-issuing the
plain request between runs satisfies it exactly as well as any automated driver, so nothing here
depends on a particular tool.

The recommended driver is the built-in `/loop`, which starts a fresh run each iteration so the heavy
build work stays isolated and the driver's own context stays small:

- `/loop docket-implement-next` — self-paced; drains the whole build-ready backlog, stopping on
  `drained`.
- `/loop docket-implement-next 90,92,94` — drains only that set of change ids, in a deterministic
  order. A named change that is not build-ready — still needs design, already in progress, or
  waiting on an unmerged dependency — is skipped this drain with its reason, not waited on.

A change on [hold](../reference/glossary.md#hold) is skipped by both drains; it is built only
when you name it alone.

Iteration caps belong to the driver, not to docket, which does not reimplement them; the one limit
docket keeps itself is the halt limit below. The one invariant the driver never breaks is the PR handoff: **a build run never merges.** A dependency
on a `pr` change therefore only clears between drains, once its pull request is merged outside the
drain — you clicking Merge, or a separate close-out drain
([Landing changes safely](./landing-changes.md)) — and the change reaches `done`. A dependency on a
results-artifact change clears as soon as that change is completed. Confirm the driver composes cleanly in your own setup before relying on it
unattended; `/loop` behavior is version- and mode-specific. The bookkeeping that decides whether a stopped run may be retried at all — who
launched it, whether it finished, whether a re-dispatch is allowed — is the run tracker, described in
[The run tracker and attribution](../concepts/run-tracker.md).

docket supports **one implement loop and one finalize loop at a time**; the two may run side by
side. A second implement loop is unsupported: both loops would share one halt count, and one loop's
clean builds would keep resetting the other's halts, so a broken setup would never reach the limit.

The run tracker follows a run only when the session starts it: with a plain request in the session
(for example "implement 412") or with `/loop docket-implement-next`. In Claude Code the
implement-next skill runs forked, so a typed command goes straight to the implementer and the
session never takes the turn that would start a tracked run.
A typed `/docket-implement-next` runs untracked.
It still builds to a PR, but if it stops partway there is no automatic retry, resume, or cancel, and
its outcome can only be read unattributed (`docket run verdict --unattributed`).

### When a change halts mid-drain

A halt usually concerns one change, such as a spec with a logical error that needs you. The change
is already marked as needing you on the board, so a tracked loop does not stop the whole drain for
it. When a run halts on a change it picked from the queue, the run tracker's verdict is
`run-next <key> run-halted <id>`: the halted change is never re-dispatched, and the loop starts a
fresh run for the next build-ready change.

A setup problem would halt every change the same way, so the run tracker keeps a **count of halts
in a row** and stops the loop at `run.max_consecutive_halts` (default 2). The halt that reaches the
limit prints `run-stop <key> run-halted <id>` with a note, and the session asks you whether to reset
the count. It never resets the count on its own; on your yes it runs `docket run reset-halts`, which
you can also run yourself at any time. Set the limit to `1` to stop a loop at its first halt.

- **Which runs count.** A run that picked its change from the queue: no id, or a set of two or more
  ids. That includes a one-off "implement the next change". A named run (a single id) or a resume
  never reads or changes the count, and its halt stops as before.
- **What resets it.** A clean build (`run-done … run-complete`) sets the count back to 0, whether
  the change was named or picked, and so does `docket run reset-halts`. Nothing else does: a run
  that claimed nothing, including one that halted before its claim, leaves the count alone.
- **It outlives the loop.** The count is shared by every session and worktree of the repository.
  A loop stopped at the limit leaves the count there until a clean build or a reset, so the next
  loop stops at its first halt. A visibility switch moves the state folder, and the count moves with
  it.
- **Halted runs stay active.** Each halted run stays active so it can be resumed. A visibility
  switch refuses while any run is still active, and a loop can leave up to the limit's worth of
  halted runs behind; cancel each (`docket run cancel --key <key> --reason <why>`) before switching.
