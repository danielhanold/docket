---
id: 5
slug: convention-extraction-skill
title: Extract the shared convention into a docket-convention skill — reference-loaded, not embedded
status: implemented
priority: medium
created: 2026-06-10
updated: 2026-06-10
depends_on: []
related: [4]
adrs: [3]
spec: docs/superpowers/specs/2026-06-10-convention-extraction-skill-design.md
plan: docs/superpowers/plans/2026-06-10-convention-extraction-skill.md
results: docs/results/2026-06-10-convention-extraction-skill-results.md
trivial: false
branch: feat/convention-extraction-skill
pr: https://github.com/danielhanold/docket/pull/6
blocked_by:
reconciled: true
---

## Why

Every docket skill embeds the shared `## Convention` block — 146 lines, 52–72% of each skill's body — kept byte-identical across the five skills by `sync-convention.sh`. The duplication was deliberate (each skill self-contained, no centralized template dependency), but it makes every convention edit a 5-file change mediated by a sync script, a dedicated test, and sync-check steps in three other tests.

The brainstorm (2026-06-10) evaluated the risks of replacing embedding with a sixth, pure-reference skill, `docket-convention`, that the operating skills load at startup. Verdict: the mid-flow skill-invocation mechanism is proven (docket skills already chain superpowers skills and each other); the real failure mode — the model skipping the load because it "already knows" the convention — is mitigated by a blocking Step-0 instruction plus an undefined-terms forcing function (slimmed skills keep using convention vocabulary without redefining it, so skipping the load is self-defeating). Install coupling (five skills without the sixth) is accepted as negligible. Full analysis in the spec §2.

## What changes

- New pure-reference skill `skills/docket-convention/SKILL.md` — the convention block verbatim, now the single source; sync markers dropped.
- The five operating skills each replace their embedded 146-line block with a ~6-line blocking "Convention (load first)" Step 0; restatements outside the old markers are swept (reference, never restate).
- `sync-convention.sh` and `tests/test_sync_convention.sh` retired; sync-check steps removed from the three other tests; new `tests/test_convention_extraction.sh` (skill exists, no copies remain, every operating skill carries the Step-0 line).
- One ADR minted at build time: reference-loading over embedding, distilling the risk analysis.

## Out of scope

- Change 0004's board/source drift tripwire in `docket-status` (unrelated to convention sync; stays as-is).
- Editing historical records (archived changes, old specs/plans/results) that mention `sync-convention.sh`.
- Any change to what the convention *says* — this moves the text, it does not revise the contract.

## Open questions

None — both brainstorm-time questions (the `docket-convention` frontmatter `description` wording, and the anti-copy grep sentinels) were settled 2026-06-10 and recorded in spec §3 and §5.

## Reconcile log

- **2026-06-10** — Reconciled same-day as the brainstorm; `origin/main` unmoved since (tip `56840df`, change 0004's terminal publish). Verified the spec's one untested assumption: `link-skills.sh` globs `skills/*/` (no hardcoded skill list), so the sixth skill links automatically. Sentinel collision scan and 146-line block measurement were already run against current skills during the brainstorm. No scope changes; spec stands as written.
