---
id: 9
slug: human-escalation-loop
title: Human escalation loop — structured questions-for-you in the change file, answered asynchronously in git
status: proposed
priority: medium
created: 2026-06-11
updated: 2026-06-11
depends_on: []
related: [8]
adrs: []
spec:
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
type: feat
---

## Why

Synthesized from the AgentRQ competitive review (2026-06-11). AgentRQ's core loop is escalation:
an agent that hits a decision it can't make creates a task for the human (its docs codify an
"Exception Escalation" pattern — `BLOCKER:` title prefix, pause until the human replies), the
human answers from a dashboard, and the answer flows back so the agent continues. That whole loop
rides on a server with push notifications — excluded for docket — but the *shape* (structured
question out, structured answer back, work resumes) translates cleanly to async markdown in git.

docket's current escape hatch is blunt: `docket-implement-next` is non-interactive, so when it
hits a genuinely human decision mid-reconcile or mid-build it can only set `status: blocked` with
free-text `blocked_by:`, and nothing defines how the human responds or how the next run consumes
the response. The human-facing half of the loop is undefined.

## What changes

- A structured `## Questions for you` convention in the change file: numbered questions written by
  the implementer (with enough context to answer cold), each with an empty **Answer:** slot the
  human fills in by editing the file on the metadata branch.
- A defined blocked→resume protocol: implementer sets `status: blocked` +
  `blocked_by: questions` (or similar marker), pushes, stops cleanly; the human answers and flips
  the status back; the next `docket-implement-next` run consumes the answers during reconcile and
  logs them in the `## Reconcile log`.
- Board surfacing: `BOARD.md` gets a "needs you" treatment for question-blocked changes, distinct
  from externally-blocked ones — complementing the existing "needs your merge" dependency reason
  and the health checks' merge-gate stall flag.
- Convention additions in `docket-convention` defining the section format and the protocol.

## Out of scope

- Real-time anything: no notifications, no chat thread, no dashboard. The "inbox" is the board
  plus the change file diff on GitHub; latency is whenever the human next looks.
- Permission/approval gating of individual tool calls (AgentRQ's allow/deny verdicts) — that is
  the agent harness's domain, not docket's.
- Changing the merge gate itself — this covers pre-PR decisions, not PR review.

## Open questions

- Does answering flip status back to `proposed` (re-selected naturally) or straight to
  `in-progress` (the original claim survives)? What happens to a half-built feature branch while
  blocked?
- Should questions also be mirror-posted as a PR/issue comment for visibility, or stay
  metadata-branch-only?
- Threshold guidance: when should the implementer self-answer-and-log (reconcile's current
  default) vs. escalate and block?

## Reconcile log
