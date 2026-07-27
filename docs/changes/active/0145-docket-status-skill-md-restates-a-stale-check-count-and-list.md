---
id: 145
slug: docket-status-skill-md-restates-a-stale-check-count-and-list
title: docket-status SKILL.md restates a stale check count and list the 0111 guard does not pin
status: proposed
priority: medium
type: docs
created: 2026-07-27
updated: 2026-07-27
depends_on: []
related: []
discovered_from: [117]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`skills/docket-status/SKILL.md` describes the mechanical health checks as "Five mechanical
checks" and enumerates five check-ids. The real vocabulary is now thirteen (`BOARD_CHECK_IDS` in
`scripts/lib/docket-frontmatter.sh`), and the SKILL.md text has been stale for several changes —
it was already wrong before change 0117 added `adr-unpublished`. The documented
`docket.sh board-checks` invocation in the same file also omits flags the script has since gained.

The staleness is structural, not a one-off typo: change 0111 built a correspondence guard that
pins the check-id vocabulary against FOUR surfaces (`BOARD_CHECK_IDS`, `board-checks.sh`'s header,
`scripts/board-checks.md`, `scripts/docket-status.md`), and `skills/docket-status/SKILL.md` is not
one of them. So every future check-id will drift there too, silently, while the guard stays green —
the exact `correspondence-guard-runs-one-way` failure the repo has already recorded once.

## What changes

Refresh the SKILL.md prose to stop restating a count and a list it does not own, and decide how it
should be kept honest. Two candidate shapes: point the skill at the authoritative enumeration
instead of restating it (the cheapest durable fix), or add SKILL.md as a fifth pinned surface to the
change-0111 guard. Prefer whichever removes the restatement rather than adding a fifth place to
maintain it.

## Out of scope

- Changing the check-id vocabulary itself or any check's behavior.
- The four already-pinned surfaces, which are correct and guarded.
