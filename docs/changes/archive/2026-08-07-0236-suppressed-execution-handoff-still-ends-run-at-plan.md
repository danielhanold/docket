---
id: 236
slug: suppressed-execution-handoff-still-ends-run-at-plan
title: A suppressed execution hand-off still ends the run at the plan — 0113 recurrence
status: killed
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [96, 113, 109, 219]
discovered_from: [113]
adrs: [24, 44]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0044](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md) |
<!-- docket:artifacts:end -->

## Why

Change 0096 stopped an autonomous run from *asking* the plan skill's interactive
execution-hand-off question. Change 0113 was supposed to stop a *suppressed* hand-off from
silently ending the run, by making step completion verifiable rather than narrated. Both are
`done` and merged (PR #102, PR #154). The failure recurred anyway.

On 2026-08-07, `/docket-implement-next 231` was dispatched as a forked subagent
(agentId `a735ab380e777c9e1`). It wrote the plan and returned, verbatim:

> Plan complete and saved. Per the directive I'm stopping at the plan file rather than offering
> an execution choice — execution is already resolved.

It then reported task-completed with no build commits, no build on the branch, and no PR. So the
suppression half worked — the hand-off question was never posed — but the agent read suppression
as authorization to **end the autonomous run at the plan**, which is precisely the mode 0113 set
out to make impossible. The run only continued after a human resumed the agent with an explicit
"execute the plan" message; that resume then built normally, which suggests nothing was blocking
the build itself.

This is the third instance in the 0096 → 0113 family (0109 is the fourth data point), so it is a
recurrence of a fixed bug rather than a new class. It matters because it converts an autonomous
drainer into a half-run that reports success: the caller sees `completed`, the board shows the
change still `in-progress`, and only a human reading the transcript notices the gap.

## What changes

Close the plan→build boundary so that a run which suppresses the hand-off cannot also stop there.
Whatever verifiable-step-completion machinery 0113 installed needs to either cover this boundary
or actually detect this stop as a false completion — grooming should determine which.

## Out of scope

- Re-litigating 0096's suppression design; suppression worked correctly here.
- Patching the vendored `superpowers:writing-plans` plugin (established non-goal in this family).
- Change 231's own content — it is unrelated and was merely the carrier for this observation.

## Open questions

- Does 0113's verifiable-step-completion gate simply not cover the plan→build boundary, or does it
  cover it and fail to fire?
- Is the fork's stop observable to the caller at all, given a forked skill's completion report is
  unreliable in both directions? If not, the fix may belong on the caller's verification side
  rather than in the worker's step ritual.
- Do the three prior fixes share a root cause that a fourth point-fix would also miss?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

## Auto-groom blocked

### 2026-08-07

Autonomous grooming drafted a full design, the critic returned two `wrong but fixable` verdicts and
three design defects, the one bounded revision round was spent, and the re-check returned **four
still-unresolved defects**. No spec was emitted. The design work is not wasted — the reconstructed
evidence and the settled findings below are the useful output, and a human groom should start from
them rather than from a blank page.

**Settled during the attempt — carry these forward, they do not need re-deriving.**

The 2026-08-07 abort on change 0231 has a recoverable git signature, and it reshapes the change's
premise:

| Fact | Evidence |
|---|---|
| Plan committed on the feature branch | `71d64d33`, 11:35:37 -0400 |
| `plan:` landed on `metadata_branch` | `2107f884`, 11:36:02 -0400 |
| Build commits at the moment of return | none — next branch commit `20b70b62` at 11:43:01, after the human's resume |
| Branch pushed / `pr:` | no / unset |
| Disposition declared in the fork's report | **none** |

So **Step 4's postcondition held in full, on both trees.** The run stopped on a *satisfied
intermediate row* — the stop `docket-implement-next` already forbids twice in prose ("a satisfied
intermediate row is never licence to stop"; the invented/absent-disposition rule). Both rules were
live. Both were violated in one report.

This answers the third open question above: **yes, the family shares a root cause.** Every
prevention lever docket has shipped (0096's ADR-0044 call-site pre-specification, 0113's split §5
sentence, the *Step postconditions* table, the terminal-disposition obligation) is prose addressed
to the agent that is failing. The one non-prose lever, `aborted-run`, is the only one that could
ever catch an instance without a suspicious human — and it could not catch this one:

- **leg A** (artifact committed, field unset) is blind — `plan:` *was* set.
- **leg B** (12h claim age) — the claim was 15 minutes old.
- **leg C** (commits ahead, unpushed, `pr:` unset) — gated on a 2h branch-idle floor; the human
  resumed after ~8 minutes.

The structural gap, stated once: **the terminal-disposition contract has a producer and no
consumer.** `advanced` is defined as claimable only when Step 7's postcondition holds — a
git-checkable statement nothing checks.

The candidate design was a pure-reader, zero-floor, single-change verifier
(`docket.sh verify-run <id> <declared-disposition>`) plus README wiring, adding no fourth prose
rule. The critic kept the script and holed the wiring.

**What a human needs to decide.**

1. **Is a self-declared `halted` trustworthy?** The verifier's whole premise is that the failing
   agent violates prose contracts it has read. `halted` and `contended` leave *identical* git state
   to an abort, so an aborting agent that declares `halted` passes the oracle silently — the same
   producer-with-no-consumer gap one level up. Either both are `unverifiable`, or someone must name
   a git discriminator. This is the load-bearing call and it is a policy judgement about how much
   the worker's self-report may be trusted, not something to default.
2. **Does the fix stop at an attended verifier?** A verifier nobody is obliged to run reduces cost
   for an already-suspicious human; it does not close the unattended path. Reaching that path means
   reaching the dispatch mechanism, which docket does not own. That is a scope call about docket's
   boundary.
3. **Ordering against change 0219.** 0219 (`in-progress`, `high`) is adding a time-free **leg D** to
   `aborted-run` keyed on `status: in-progress` + `pr:` set, enriching leg C, and rewriting
   `board-checks.md`'s `## Not covered` paragraph — the same oracle, the same failure family, an
   overlapping Step-7 predicate. `related: 219` has been recorded here, but `related:` is never a
   readiness gate: decide whether this needs `depends_on: [219]` (noting 0219 itself hangs on
   `depends_on: [211]`) or merely a "expect a rebase" note.

**Recommendation: keep it, do not kill or defer.** The bug is real, reproducible, and this is its
fourth instance. But it wants a human groom, and the human should resist a fourth point-fix — the
evidence above is that another prose rule will be violated exactly as the first three were.

## Why killed

Absorbed into change 0237. The two stubs were filed the same day as two step boundaries (Step 4/5 and Step 5/6) of one failure family, and each independently concluded the family shares a root cause: every remedy docket has shipped is prose addressed to the agent that is failing. A per-boundary split reproduces that family's own failure mode — each fix misses the boundary it did not name. 0237 carries the settled evidence from this stub's auto-groom abstain (the reconstructed 0231 git signature, the producer-with-no-consumer statement, and the three aborted-run legs' blindness) and grooms it to a design covering every boundary at once.
