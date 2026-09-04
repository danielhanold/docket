# Build: Building without supervision

By the end of this page you can hand a designed piece of work to an autonomous loop and get back an
open pull request, without babysitting it — and you will understand what the loop checks before it
writes a line of code, how it decides how hard to work on each part, and where it stops and waits
for you. The one thing it never does is merge; that stays your call.

## Why hand work to the loop at all

If you already run coding agents in your repos, you have probably felt the gap this fills.

Some agent workflows give you excellent execution but no memory. A brainstorm-to-merge run works
well, but everything happens inside a single session: there is no tracked backlog, and no "done"
state that outlives the session. Every session starts from a blank slate. Heavier tools close that
gap by adding a full living-spec lifecycle, but at the cost of a command-line dependency and a rigid
markdown contract not every project wants to adopt.

Docket sits in between. It adds a thin lifecycle layer — plain markdown files in your repo, a
handful of skills (a **skill** is a named, reusable instruction set an agent loads for one job), no
extra command-line tool to install — and by default hands each execution step back to whatever
workflow engine you already use. The code stays the single source of truth about current state:
docket keeps no living-spec layer and never tries to mirror your codebase in prose. The decisions
behind the code are recorded separately, as ADRs (an **ADR** is an architecture decision record: one
file per decision, immutable once accepted).

But the thin lifecycle is not the real reason to use the loop. The reconcile step is.

## Reconcile: killing stale work before any code

This is the most valuable and least obvious part of the loop.

**The problem.** A change (one unit of planned work, roughly one pull request, tracked as one
markdown file) is drafted against a *snapshot* of the world — the codebase, the recorded decisions,
and the other in-flight work as they stood the day you wrote it. In a durable backlog, the loop may
not pick that work up for a week or a month. By then another change may have already shipped what
this one planned to add; a decision may have settled an open question in the opposite direction; or
a dependency may have changed the interface this design assumed.

Most backlog-driven systems build the ticket as written and let the worker discover the mismatch
halfway through. The classic results: re-implementing something already done elsewhere, or building
something a later decision has invalidated.

**What the loop does instead.** Every run includes a **reconcile** step (a check at build time that
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
killed and the loop moves on to the next one. If the design is fundamentally invalidated — it needs
re-thinking, not just scope-trimming — the run stops and escalates to you, because re-designing
needs a human and the loop will not do it alone.

The stance: **plans rot; refresh them just-in-time, and never trust a stale backlog.** If an
interrupted run resumes with `reconciled` still `false`, the full reconcile pass runs again before
any work continues.

## What one unattended run does, end to end

A single run of the autonomous loop walks a fixed path and then stops:

1. **Pick.** It selects the next **build-ready** change (a proposed change that has a spec or is
   marked trivial and whose dependencies are all merged). It never touches work that is not ready.
2. **Claim.** It takes a **claim** on that change (the moment a change is picked up for building; it
   records which branch will carry the work and when it was taken) with a compare-and-swap on the
   change's status, so two runs in parallel can never grab the same change. The claim carries a
   **claim lease** (a timestamp on a claim; when it expires with no branch behind it, the change
   goes back to the queue), which is what lets a crashed run's change self-heal instead of sitting
   stuck — see [Keeping the backlog honest](./keeping-the-backlog-honest.md).
3. **Reconcile.** The step above — freshen the change against reality, or kill or escalate it.
4. **Plan.** It cuts a feature branch, creates an isolated working copy of the repo on that branch
   (a *worktree*), and authors a **plan** on that branch (the task-by-task breakdown a build
   follows, written on the feature branch). The plan lives with the code, not with the backlog.
5. **Build.** It works the plan task by task, test-first, committing as it goes (the next section
   covers how it decides how hard to work on each task).
6. **Review.** Before the pull request opens, a bounded reviewer reads the whole branch — see
   [Reviewing before the human does](./reviewing-before-the-human.md) — and its findings are fixed
   on the branch.
7. **Stop.** It opens the pull request and stops at the human merge gate. It never merges.

A change whose feature branch is cut from another change's *unmerged* branch is a **stacked change**
(a change built on another change's unmerged branch rather than on the integration branch); its
pull request targets that parent branch rather than your **integration branch** (the branch code
lands on, usually `main`). Everything else cuts straight from the integration branch.

The whole run is unattended between "pick" and "stop": your only required touch-point is reading and
merging the pull request it opens.

## Build profiles and the one escalation

The build step does not treat every task the same. Each task in the plan is routed to one of four
**build profiles** (one of four worker tiers — economy, standard, premium, max — a plan task is
routed to by risk), which share one worker contract and differ only in the model and effort behind
them. Each profile is a separately launched **agent** (a separately launched worker with its own
context, pinned to a model and effort), and the build **dispatches** one per task (launching a named
agent to do a step and waiting for it to return).

The routing is deliberately asymmetric. The cheapest tier, `economy`, must be *positively*
established — the task is fully specified, follows an existing pattern, carries no consequential
risk, and needs no cross-file reasoning. Genuine uncertainty defaults to `standard`, the next tier
up, rather than dropping to the cheaper one: when in doubt, docket spends more, not less. The top
tier, `max`, is deliberately rare — reachable only for unresolved architecture or an irreversible
data change, by an explicit override on the task, or by escalation — so the tier meant for extreme
cases does not become normal. A plan task can override the routing outright with a build-profile
line on that task; an invalid value halts the build rather than silently falling back to a guess.

Each task carries **at most one automatic escalation**, and only ever one rung up: an `economy`
worker that cannot finish retries once at `standard`, a `standard` worker once at `premium`, a
`premium` worker once at `max`. There is never a second climb — a `max` worker that still cannot
finish halts the build for a human rather than looping. The concrete payoff of this shape is that a
cheap task costs a cheap worker, a risky task gets a strong one, and a task that turns out harder
than it looked gets exactly one shot at more capability before a human is asked.

Once every task has committed, the build runs the whole test suite once as its gate — that half of
the story, and how the result is certified, is [Proving the build](./proving-the-build.md). The
deeper mechanism behind profile routing and the gate verdict lives in
[Build profiles and the test gate](../concepts/build-profiles-and-gate.md).

## Draining the queue hands-free

A single run builds one change. To drain a whole backlog you loop the run, and the loop keys on the
outcome each run declares. Every run ends by declaring one of four outcomes:

- **advanced** — it built a change and opened a pull request.
- **contended** — it lost a claim race to another run and built nothing.
- **drained** — nothing in scope was build-ready.
- **halted** — something needs a human (an escalation, an invalidated design, a broken precondition).

A driver keys on these with one simple rule: **continue on `advanced` or `contended`, stop on
`drained` or `halted`.** The contract is deliberately driver-agnostic — a human re-typing the
command between runs satisfies it exactly as well as any automated runner, so nothing here depends
on a particular tool.

The recommended driver is the built-in `/loop`, which starts a fresh run each iteration so the heavy
build work stays isolated and the driver's own context stays small:

- `/loop docket-implement-next` — self-paced; drains the whole build-ready backlog, stopping on
  `drained`.
- `/loop docket-implement-next 90,92,94` — drains only that set of change ids, in a deterministic
  order. A named change that is not build-ready — still needs design, already in progress, or
  waiting on an unmerged dependency — is skipped this drain with its reason, not waited on.

Budget and iteration caps belong to the driver, not to docket, which does not reimplement them. The
one invariant the driver never breaks is the merge gate: **the loop never merges.** A dependency
therefore only clears between drains when a merge happens outside the loop — you clicking Merge, or a
separate close-out drain ([Landing changes safely](./landing-changes.md)). Confirm the driver
composes cleanly in your own setup before relying on it unattended; loop behavior is version- and
mode-specific. The bookkeeping that decides whether a stopped run may be retried at all — who
launched it, whether it finished, whether a re-dispatch is allowed — is the run gate, described in
[The run gate and attribution](../concepts/run-gate.md).
