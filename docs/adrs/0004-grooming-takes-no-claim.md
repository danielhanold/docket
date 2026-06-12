---
id: 4
slug: grooming-takes-no-claim
title: Grooming takes no claim — final-push CAS suffices for human-attended sessions
status: Accepted
date: 2026-06-12
supersedes: []
reverses: []
relates_to: [1]
change: 12
---

## Context

`docket-implement-next` opens with a compare-and-swap claim: it flips the selected change to
`in-progress` and pushes before doing any work, so two autonomous builders can run concurrently
without ever building the same change. When `docket-groom-next` was designed (change 0012), the
question was whether grooming needs the same protection. Grooming differs from building in two
ways: it is human-attended (a person is in the brainstorm, so two simultaneous groom sessions on
one backlog imply the same human in both), and its only writes land in a single final commit
(spec + change-file edit) rather than hours of accumulated build work that a late collision would
waste.

## Decision

`docket-groom-next` takes **no claim** and introduces no new lifecycle status or marker field.
The concurrency control is entirely at the final push on `metadata_branch`: on a non-fast-forward
rejection, `pull --rebase` and retry; if the rebase brought in commits touching the groomed
change's file, the skill MUST re-read it first and STOP (report, don't overwrite) if the change is
no longer needs-brainstorm. The arbiter is the re-read after rebase, exactly mirroring the spirit
of the implementer's claim loop — but applied once, at the end, where the work is cheap to discard.

The rule for future skills: an **autonomous, long-running** writer claims up front; a
**human-attended, single-commit** writer may rely on final-push CAS plus mandatory re-read. A
`grooming:` marker field and a status-based claim were considered and rejected — both add
machinery (a new field or an eighth status, plus stale-state cleanup in health checks) for a race
that the final-push CAS already resolves safely.

## Consequences

- No stale-claim cleanup burden for grooming: an abandoned groom session leaves no trace in the
  backlog (nothing was written until the end), so `docket-status` health checks need no new rule.
- A genuinely simultaneous groom of the same stub wastes at most one brainstorm session's
  conversation — accepted, since both sessions include the same human, who would notice.
- The board cannot show "being groomed" the way it shows `in-progress` builds; grooming is
  invisible until it lands. Accepted as the cost of zero machinery.
- If grooming ever becomes autonomous or batch (explicitly out of scope in change 0012), this
  decision must be revisited — that variant moves into the claim-up-front category, and a new ADR
  should supersede this one.
