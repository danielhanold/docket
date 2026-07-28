---
id: 145
slug: docket-status-skill-md-restates-a-stale-check-count-and-list
title: docket-status SKILL.md restates a stale check count and list the 0111 guard does not pin
status: in-progress
priority: medium
type: docs
created: 2026-07-27
updated: 2026-07-28
depends_on: []
related: [117, 144]
discovered_from: [117]
adrs: []
spec: docs/superpowers/specs/2026-07-28-status-skill-stale-check-restatement-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/docket-status-skill-md-restates-a-stale-check-count-and-list
claimed_at: 2026-07-28T11:15:48Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-status-skill-stale-check-restatement-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-status-skill-stale-check-restatement-design.md) |
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

Remove the restatement from `skills/docket-status/SKILL.md`'s `### Health checks` section rather
than add a fifth surface to change 0111's correspondence guard — removal makes the drift impossible
instead of merely detected.

- **Drop** the count word ("Five"), the five-item check-id list, and the hand-run
  `docket.sh board-checks` invocation block (the skill never runs the checker directly — it invokes
  `docket.sh docket-status`, and SKILL.md already delegates mechanics to `scripts/docket-status.md`).
- **Keep** what the skill actually owns: the warn-only/git-only/never-auto-fix posture, the existing
  one-line cross-reference to `## Judgment follow-ups`, and a one-line characterization of what the
  checks are about.
- **Point** at the authoritative enumeration instead: `scripts/board-checks.md`'s per-check sections
  and `scripts/docket-status.md`'s `check <check-id>` row.
- **Guard** the removal so it cannot silently return: one assert in `tests/test_board_checks.sh`,
  placed immediately before the `PASS`/`exit` epilogue, that extracts the `### Health checks` section
  (terminator: next `^#{1,3} ` heading or EOF — the section is currently file-final) and asserts no
  emitted check-id appears in it by word boundary, with a positive non-vacuity anchor so a heading
  rename reddens rather than passing silently.

Design, rejected alternatives, and the guard's named limitation are in the linked spec.

## Out of scope

- Changing the check-id vocabulary itself or any check's behavior.
- The four already-pinned surfaces, which are correct and guarded.
- Auditing other skills for the same restatement class — real, but a separate sweep.
