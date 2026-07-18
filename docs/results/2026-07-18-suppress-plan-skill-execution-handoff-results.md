# Autonomous skill hand-off precedence — results

Change: #96 · Branch: feat/suppress-plan-skill-execution-handoff · PR: <url> · Plan: docs/superpowers/plans/2026-07-18-autonomous-skill-handoff-precedence.md · ADRs: 44

## Verify (human)

- [ ] **The fix cannot be verified by this suite — it needs live runs.** The defect is model judgment
      inside a fork at roughly 1-in-40, so no test asserts "no prompt surfaced" (an explicit scope
      boundary, see the change's *Out of scope*). Confidence comes from the next N autonomous
      `docket-implement-next` runs completing inside their fork. Watch for the conditional trace line
      (role + skill) — it means a hand-off was met and suppressed, which is the mechanism working,
      not a fault.
- [ ] **This build is itself one data point, and a favorable one.** The run that produced this branch
      invoked `$SKILL_REVIEW` and `$SKILL_FINISH` under the new directed phrasing and surfaced no
      choice; `superpowers:writing-plans` §Execution Handoff did not fire (the plan already existed
      from the interrupted session, so §4 was not re-entered — the site that actually broke at run 40
      was **not** exercised end-to-end here).

## Findings

**ADR-0044** — *Autonomy precedence is enforced by pre-specification at the call site.* Records the
two-part decision and, importantly, which part is load-bearing: the convention paragraph is
durability for future bindings, the call-site `DIRECTED to:` directions are the enforcement. Recorded
on `origin/docket`; publish-onto-`main` was deliberately skipped despite `terminal_publish: true`
(the auto-mode classifier denies that direct-to-main push mid-run — finalize publishes it at merge
from the `adrs:` list).

**The plan's self-review dropped a spec deliverable.** The spec's `## Expected ADR` section required
an ADR; the plan's *Self-review notes* §"Spec coverage" enumerated spec §1–§4 and silently omitted
it, so no task produced one. Caught by the whole-branch review, not by the plan. The lesson is about
coverage-checking a plan against *every* section of its spec, not only the numbered ones.

**A false parallel in prose the guard structurally could not catch.** The first pass at
`docket-finalize-change`'s exception paragraph asserted that on the autonomous path "finalize
pre-specifies its outcome exactly as `docket-implement-next` §7 does". It does not — finalize never
invokes `$SKILL_FINISH` on that path at all (it merges via `gh` and runs its own steps 1–6). The
sentence invited precisely the inverse drift: an autonomous finalize concluding it *should* invoke
the finish skill with a directed outcome. The guard cannot see this, because that line already
satisfies it through the human-present exception branch. This is the repo's documented
false-prose failure mode (`verify-the-claim`) reappearing inside the change that ships the rule.

**Two sites had the direction but not the trace.** §5 and §6 were directed but omitted the
suppression-log clause §4 carries, leaving the trace at those sites resting entirely on the
convention paragraph — the exact shape this change argues does not fire at the moment of invocation.
Self-inconsistency, fixed in the same pass.

**A silent site-discovery bypass, caught only by a floor.** The guard keyed on the bare `$SKILL_X`
sigil, so rewriting a site to `${SKILL_X}` dropped it from discovery with no assert reddening — only
the `checked >= 5` vacuity floor noticed, and a floor stops protecting the moment a legitimate 6th
site lands. Widened to match both spellings, with asserts pinning each. AGENTS.md already named this
class ("key a guard on shape, never a spelling"); it still slipped through.

**Accepted limitations of a token-presence guard**, documented in the test header rather than fixed:
the marker satisfies a line from any position (a parenthetical mention would pass), and `checked`
counts matching *lines*, so one line invoking two role skills is covered by a single marker. Both
need contrived prose to hit; realistic drift (a direction deleted or reflowed away) is caught. Two
inaccurate header comments were also corrected — run 40's evidence concerns the *wrapper's*
abort-and-report rule losing, not this change's new prose, and `agents/*.md` is the committed source
`sync-agents.sh` installs, not its output.

**Verify-the-claim fired against my own dispatch.** The ADR dispatch prompt cross-linked ADR-0015 for
unvalidated skill-name passthrough; that is ADR-0018. The `docket-adr` subagent checked the spec's
actual cross-links, followed those, and flagged the discrepancy instead of encoding it.

## Follow-ups

- **The §4 site remains unexercised end-to-end.** This build resumed from an interrupted session with
  the plan already written, so the one call site that actually failed at run 40 was never re-entered
  under the new phrasing. The first fresh autonomous build that plans from scratch is the real test.
- **Consider tightening the exception classifier.** Today, once a line in `docket-finalize-change`
  contains "human is present", the marker is never demanded on it again — which is exactly the line
  the false-parallel defect lived on. Requiring the exception line to *also* carry a shape assertion
  would close the one structural blind spot the review identified. Not done here: it is a guard
  redesign, out of scope for this change.
- **`checked >= 5` is doing double duty** as a vacuity floor and (accidentally) as a spelling guard.
  The spelling half is now handled by the widened pattern; the floor should be revisited if the site
  count changes, since it must track the real count to stay meaningful.
