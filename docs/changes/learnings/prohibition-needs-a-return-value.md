---
slug: prohibition-needs-a-return-value
hook: "A prohibition added to a contract with a CLOSED return vocabulary is incomplete until it names which return it maps to — otherwise the most likely response to a correctly-written rule is a malformed return."
topics: [contracts, subagents, design]
changes: [249]
created: 2026-08-08
updated: 2026-08-08
promotion_state: retained
promoted_to:
---

## Apply
When you add a "never do X" clause to a worker or agent contract whose outcomes are a fixed
enumeration (`COMPLETE` / `NEEDS_ESCALATION` / `BLOCKED`, or any closed set), finish the sentence:
say which enumerated value the worker returns *instead*. Check the fit against the controller that
consumes the return — a value that reads as "malformed" to the controller halts the run, so a rule
that leaves the worker to pick one is a rule whose likeliest outcome is a halt. Put the mapping in
the clause itself, at the point the worker is reading when it stops, not in the distant `## Outcomes`
enumeration: the rule has to bind at the moment of action.

## War story
- 2026-08-08 (#249, PR #178) — the new fail-closed clause in `skills/docket-build-task/SKILL.md`
  told a worker never to infer success from an unfinished verification run, but named no return.
  None of the three outcomes fits an unverified run on its face, and the controller reads a
  reasonless `NEEDS_ESCALATION` as malformed and halts the build — so a correctly-written
  prohibition's most likely effect was a halted build. Review caught it (finding 1); the fix named
  `BLOCKED` inside the clause (`0f7ad4c8`) rather than extending `## Outcomes`.
