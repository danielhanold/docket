---
id: 145
slug: docket-status-skill-md-restates-a-stale-check-count-and-list
title: docket-status SKILL.md restates a stale check count and list the 0111 guard does not pin
status: done
priority: medium
type: docs
created: 2026-07-27
updated: 2026-07-28
depends_on: []
related: [117, 144]
discovered_from: [117]
adrs: []
spec: docs/superpowers/specs/2026-07-28-status-skill-stale-check-restatement-design.md
plan: docs/superpowers/plans/2026-07-28-status-skill-stale-check-restatement-plan.md
results: docs/results/2026-07-28-docket-status-skill-md-restates-a-stale-check-count-and-list-results.md
trivial: false
auto_groomable: true
branch: feat/docket-status-skill-md-restates-a-stale-check-count-and-list
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/135
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-status-skill-stale-check-restatement-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-status-skill-stale-check-restatement-design.md) |
| Plan | [2026-07-28-status-skill-stale-check-restatement-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-status-skill-stale-check-restatement-plan.md) |
| Results | [2026-07-28-docket-status-skill-md-restates-a-stale-check-count-and-list-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-28-docket-status-skill-md-restates-a-stale-check-count-and-list-results.md) |
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

## Reconcile log

### 2026-07-28

Reconciled at claim against `origin/main` (`f804c7b2`) and `origin/docket`. Scope unchanged; three
facts the spec stated as *pending* are now *settled*, all in this change's favour.

- **0117 (PR #129) has MERGED.** `BOARD_CHECK_IDS` on `main` now holds **thirteen** ids
  (`adr-unpublished board-row-dropped broken-plan-results broken-spec dep-cycle field-domain
  malformed-id merge-gate-stall merged-orphan publish-deferred stale-finalize-blocked
  stale-in-progress unknown-commit-ref`), and change 0117 is archived. Consequences: (a) the spec's
  "12 on `main`, 13 after #129" (Assumption 9) resolves to a flat **13** — still not load-bearing,
  since the fix *removes* the count rather than corrects it; (b) **Assumption 6's file-collision
  risk is retired** — 0117's two hunks in `tests/test_board_checks.sh` are already on `main`, so
  there is no concurrent rewrite to place around. The spec's end-of-file placement rule is kept
  anyway: it is independently correct (the epilogue is the stable anchor) and it keeps the guard
  out of the count-assert region that 0117 just churned.
- **SKILL.md is unchanged and the spec's structural claims re-verify on today's `main`.**
  `### Health checks` is still the file's **last** section (lines 92–107 of 107), so the extractor's
  **EOF arm is the live path**, exactly as Assumption 4 requires — a two-heading extractor would be
  vacuous from birth. Line 86 (`## Merge sweep` sweep-posture) remains the file's **only** check-id
  occurrence outside the target section (`publish-deferred`), so the negative assert must stay
  **section-scoped**, not file-wide.
- **`$emitted` is still derived in `tests/test_board_checks.sh`** (line 1447, from `board-checks.sh`'s
  `emit <id> "` sites) and the file still ends with the `PASS`/`exit "$fail"` epilogue — so the new
  assert can consume the real emitted set and sit immediately before that epilogue as designed.

**Still open, unchanged:** Assumption 7's soft spot — change 0144 is still `proposed` with **no
spec** (`auto_groomable: false`, `## Auto-groom blocked` present), and if it lands a distinguishable
`board-checks failed <exit>` diagnostic line it will likely edit `## Read the report` in this same
SKILL.md. Different section, same file; whoever builds second re-checks. No `depends_on` warranted.

No scope change, no work dropped, no new constraint folded in.
