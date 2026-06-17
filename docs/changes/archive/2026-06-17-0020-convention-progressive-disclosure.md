---
id: 20
slug: convention-progressive-disclosure
title: Split the docket-convention skill via progressive disclosure — extract the GitHub board mirror first
status: done
priority: medium
created: 2026-06-17
updated: 2026-06-17
depends_on: []
related: [5]
adrs: [3]
spec: docs/superpowers/specs/2026-06-17-convention-progressive-disclosure-design.md
plan: docs/superpowers/plans/2026-06-17-convention-progressive-disclosure.md
results: docs/results/2026-06-17-convention-progressive-disclosure-results.md
trivial: false
auto_groomable:
branch: feat/convention-progressive-disclosure
pr: https://github.com/danielhanold/docket/pull/33
blocked_by:
reconciled: true
---

## Why

Change 0005 extracted the shared convention into the single `docket-convention` reference skill,
loaded as a blocking Step 0 by every operating skill (ADR-0003). Since then it has grown — the
GitHub board mirror (0011) and the Agent layer (0016/0017) — to ~224 lines / 26.6 KB, the largest
skill in the repo. All seven operating skills load it up front, so every run pays that ~26 KB of
context.

This change starts paying that down via **progressive disclosure** — a concise core `SKILL.md` plus
on-demand sibling references — to (1) shrink the common-path context footprint and (2) bring the
convention into the skill-authoring shape good skills follow. The constraint is ADR-0003's vocabulary
guarantee: the Step-0 load must stay sufficient for the common path, so only detail that is genuinely
off that path may move out.

## What changes

- A durable extraction criterion (recorded in the spec): a section may move to a sibling only when it
  is **both** heavy **and** off the common read-path (opt-in, or its work is script-delegated).
- By that criterion, extract **only** the `### GitHub board mirror` section into a flat sibling
  `skills/docket-convention/github-board-mirror.md`. Core keeps a 2-line stub under the same heading
  (one-way · change-files-authoritative · script-owned · rides in the Board pass) plus a pointer:
  read the sibling when `board_surfaces` includes `github`.
- Retarget the single existing reference in `docket-status`'s Board pass at the sibling — the only
  skill that needs the mechanics; every other skill runs that Board pass by reference.
- Extend `tests/test_convention_extraction.sh` to guard the new structure (sibling exists and carries
  a mirror-distinctive phrase; that phrase is gone from `SKILL.md`; stub heading + pointer present;
  docket-status pointer present).

## Out of scope

- Extracting any other section (Agent layer, Configuration, Bootstrap guard, Branch model, …). The
  Agent layer specifically fails the criterion — its abort-and-report rule and composition dispatch
  contract are runtime, non-delegated, and apply on the common autonomous path.
- Introducing a `references/` subdirectory (not warranted for a single file).
- Any change to *what the mirror does* — this moves text, it does not revise the contract.
- No new ADR; this is a recursive application of ADR-0003.

## Open questions

None — scope, discovery mechanism, file location, and the no-ADR call were settled in the 2026-06-17
brainstorm (see spec §7).

## Reconcile log

- **2026-06-17** — Reconciled same-session as the brainstorm. `origin/main` at `f6c2253`
  (0015's terminal-publish; PR #32 merged the finalize rebase-retest gate) — none of it touches
  the convention mirror, the docket-status mirror reference, or `test_convention_extraction.sh`.
  Verified against `origin/main` (the build base): the `### GitHub board mirror` section is
  lines 206–218 of `skills/docket-convention/SKILL.md`; the `### Configuration` block references
  it by name twice (line 36, `see *GitHub board mirror*`), so the stub heading must survive;
  docket-status line 47 carries the single mirror reference to retarget; the mirror header/phrase
  is absent from the test's header + sentinel lists, so extraction breaks no existing assertion.
  No scope changes — spec stands as written.
