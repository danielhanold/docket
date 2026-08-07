---
id: 237
slug: prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical
title: "Prose levers fail to hold the step boundary — consider a mechanical end-of-run gate"
status: proposed
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [96, 113, 212, 235, 236]
discovered_from: [235]
adrs: [69]
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
| ADRs | [ADR-0069](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0069-mode-conditioned-clause-discriminates-on-provenance.md) |
<!-- docket:artifacts:end -->

## Why

Change 0212 fixed "an inlined role skill's terminal stop ends the whole run." It merged on
2026-08-05 (PR #161, ADR-0069). On 2026-08-07 the failure recurred, unchanged, on the first live
run to exercise the fixed path.

`/docket-implement-next 235` (agentId `ac44ea6eb7832da86`) ran Steps 0–5 in full — six build
commits, full suite green (`files=87 passed=87 failed=0 asserts=6347`) — then ended its turn at
the **Step 5/6 boundary**, leaving the branch unpushed, no review, no PR, and the manifest still
`in-progress`. Its closing line was:

> Build disposition (role-scoped): **green**.

That is `docket-build`'s output vocabulary, not `docket-implement-next`'s.

**Both of 0212's levers were present, intact, and correctly worded at the time of failure:**

- `skills/docket-build/SKILL.md:13` — *"**Scope of this stop:** if you invoked this skill yourself,
  this stop ends only the build role…"* The fork **did** invoke the skill itself, via the Skill
  tool. That is precisely the provenance condition ADR-0069's mode-conditioned clause discriminates
  on, so the clause applied by its own terms. The run stopped anyway.
- `skills/docket-implement-next/SKILL.md:123-126` — the four run-disposition table
  (`advanced`/`contended`/`drained`/`halted`), plus line 102's *"a satisfied intermediate row is
  never licence to stop"* and *"A run that ends any other way ends on a disposition, not on a
  postcondition."* 0212 introduced the disposition-vocabulary tell precisely because it is
  independently checkable and would have caught all four prior incidents. Nothing checked it.

So this is not a sixth instance of the original bug. It is evidence about the **fix strategy**:
0212 diagnosed the class correctly and answered it with two prose levers, and on first re-test the
levers did not bind. The agent read the scoping clause and stopped regardless.

Incident count in the family: 0109, 0194 (×2), 0206, 0231 (the 0236 stub), and now 0235 — the
first after 0212 merged.

It matters because the failure mode is a half-run that reports success: the caller sees
`completed`, the board shows `in-progress`, the branch is unpushed, and only a human reading the
transcript notices.

## What changes

To be settled at groom time. The direction worth exploring — explicitly **not** decided here — is
to stop adding prose riders and add a **mechanical** end-of-run gate: something that reads git
state at turn end and refuses to terminate a run that holds a claimed change with an unpushed
branch and no PR. A check that does not depend on the agent having correctly read a paragraph
about which role it is currently playing.

Also in scope for the design conversation: whether the disposition-vocabulary tell can be
mechanically enforced rather than merely stated, given that it is already a reliable signal and
was already ignored once.

## Out of scope

Re-fixing the Step 5/6 boundary with more prose. That is what 0212 did, and this change exists
because it did not hold. A third prose rider needs to justify itself against that record.

## Open questions

- **Should this be merged into 0236?** 0236 ("A suppressed execution hand-off still ends the run
  at the plan — 0113 recurrence") was filed the same day, is `high`, and is the same meta-class at
  the Step 4/5 boundary in the 0096→0113 lineage. This stub is the Step 5/6 boundary in the 0212
  lineage. Two lineages, two boundaries, one underlying shape: *every prose fix in this family
  fails at whichever step boundary it did not name.* A groomer should decide deliberately whether
  these are one change or two — filing both was the safe default, not a considered verdict.
- Is a mechanical gate reachable from inside the agent's own turn, or does it need to live in the
  harness / a wrapper obligation the agent cannot narrate its way past?
- What is the correct posture when the gate fires — abort-and-report, or auto-continue to the next
  step?

## Reconcile log
