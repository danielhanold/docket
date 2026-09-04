# Review: Reviewing before the human does

By the end of this page you will know what happens to a finished branch between its last build commit
and the pull request you read: who reviews it, what that reviewer is allowed to touch, how its
findings get fixed before they ever reach you, and why the tests already ran before the review began.

## The reviewer and its rungs

The review role — the step in the autonomous drainer that builds one change (one unit of planned
work, roughly one pull request, tracked as one markdown file) end to end, run just before the pull
request opens — reads the finished branch and hands back findings. It runs `docket-review`, one
read-only reviewer contract behind three pinned rung wrappers: `docket-review-lean`,
`docket-review-standard`, and `docket-review-deep`. These are three agents (a separately launched
worker with its own context, pinned to a model and effort) that share the one contract and differ
only in model and effort.

The reviewer reads the branch diff, its commit log, and the tree. It never writes, never commits,
never checks out another branch, never launches a sub-worker, and never runs the test suite. It gets
one shot at the rung it was dispatched at (dispatch being the act of launching a named agent to do a
step and waiting for it to return) — there is no reviewer escalation ladder.

`docket-review` is the shipped default. To opt back out to the general-purpose reviewer, set the role
in any config layer:

```yaml
skills:
  review: superpowers:requesting-code-review
```

## Choosing the rung

The rung is chosen **deterministically as one above the build** — not by the model's own judgment. The
drainer takes the highest **build profile** (one of four worker tiers — economy, standard, premium,
max — a plan task is routed to by risk) that any task routed or escalated to, and maps `economy` to
lean, `standard` to standard, and `premium` or `max` to deep. A whole-branch diff of more than 1500
changed lines bumps the rung one step, capped at deep.

The reason a cheap build earns a cheap review is that the build's own routing already answered how
hard the work was; the diff-size bump is the one signal independent of that self-assessment.

## From findings to fixes

Findings come back **severity-tiered**, and they are fixed on the branch rather than recorded and left
for you. After review returns and before the pull request opens, the drainer runs a bounded **fix
loop**: each finding becomes a task through the same worker contract that wrote the code, committed
into the same diff you were going to read anyway, so the merge gate does not move. The reviewer itself
is unchanged — it returns the finding list and a one-line verdict, and never fixes anything.

Two axes are kept deliberately apart:

- **Character** picks the model tier, using the same routing rubric the build applies to a **plan**
  (the task-by-task breakdown a build follows, written on the feature branch) task — so a subtle
  one-line fix is not handed to a cheap model just for being labelled minor.
- **Severity** picks only the *failure posture*: a blocker that cannot be fixed halts the run, while an
  important or minor that cannot be fixed falls back to a line in the pull request body. Fix routing
  **never reaches the `max` tier** at any severity — `max` means irreversible, and an irreversible act
  must not happen to a branch as an unplanned side-quest discovered at review time.

## The loop is bounded

One full-suite run happens after the fixes land. If it goes red, the non-blocker fix commits are
reverted and the suite runs once more — green proceeds with those findings recorded unfixed, still-red
halts. That is two suite runs at most, no second repair chain, and no re-review round, so the branch
can never end worse than the green build that entered the loop. Every finding's outcome reaches the
pull request body as a table: fixed (with its commit SHA), deferred, reverted, or recorded.

Two config knobs shape the loop, both settable in any layer:

- `review.min_fix_severity` (default `minor`) is the lowest severity that enters the loop. `important`
  records minors instead of fixing them; `blocker` restores the older record-only behavior as a
  compatibility escape hatch.
- `review.max_fix_tasks` (default `10`) caps how many non-blocker fix **tasks** one run dispatches —
  the task, not the finding, so a batch of minors sharing one routed tier spends a single slot.
  Overflow findings are deferred with the cap named as the reason, and `0` is legal, meaning "fix
  nothing but blockers."

**Blockers are fixed regardless of both knobs** — neither can disarm the one gate that must not be
disarmed, so blockers never count against the cap either. One consequence worth stating: a review
finding about the branch's own diff is no longer captured as a separate backlog stub at all. It is
fixed or it is recorded; only genuinely distinct, beyond-the-branch work still mints one.

## Why the suite already ran

The reviewer never runs the test suite because the full run happened earlier, in the **build gate**
(the full test-suite run at the end of a build that must be green before review). Four reasons make
that split deliberate, and they compound:

- The suite answers the *build's* question — "does what I assembled actually work together?" — while
  review asks a different one: "is this good?" Putting the first inside the second confuses two jobs.
- The repair machinery already lives on the build side. A suite inside a reviewer forbidden to fix
  anything would have to hand its failures back out and re-enter build machinery, recreating exactly
  the build → review → build loop this design exists to kill.
- Gate-first ordering is cheaper on failure. A red suite discovered *after* an expensive whole-branch
  read has wasted the read; discovered before, it costs one build task.
- The evidence chain then follows naturally, because the thing that last changed the branch is the
  thing that certifies it.

The mental model: **the suite is the boundary between building and reviewing**, owned by the side that
can fix a failure. It works like a continuous-integration status check on a pull request — the check
runs on the branch as it lands, and the reviewer is the human-style reader who trusts the green check
instead of re-running it.

That boundary is made durable by the **build evidence** (the committed record of that gate run, read
by the reviewer). On green, the gate emits the command it ran, the result, the exact branch head, and
a timestamp; the reviewer verifies the record is present, green, and pinned to the exact head it is
reviewing, and returns an `unverified-build-state` blocker if it is missing, malformed, or stale —
running the suite itself is never the remedy. How that record is minted and carried forward is
[Proving the build](./proving-the-build.md); the profile ladder and the gate verdict as a mechanism
are [Build profiles and the test gate](../concepts/build-profiles-and-gate.md).
