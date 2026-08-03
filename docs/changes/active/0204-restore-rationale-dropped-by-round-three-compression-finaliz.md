---
id: 204
slug: restore-rationale-dropped-by-round-three-compression-finaliz
title: Restore rationale dropped by round-three compression (finalize named-id override, auto-capture mint-site loop)
status: proposed
priority: medium
type: docs
created: 2026-08-03
updated: 2026-08-03
depends_on: []
related: []
discovered_from: [201]
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

Change 0201's deep-rung review left two minor findings unfixed at merge time, both the same
defect: the compression round preserved a **rule** in its SKILL.md stub while dropping the
rule's **rationale** instead of relocating it into the new reference file. The rules still
work; the "why" is now stated nowhere, so the next reader (human or agent) who wants to
weaken or route around either rule has no argument to weigh against.

1. `docket-finalize-change`'s `## Finalize blocked` stub kept "an explicitly named id
   overrides the auto-detect skip" but dropped the anti-deadlock reason it exists — without
   the override a marked change can never be finalized, so the clearing rule could never
   fire.
2. `docket-convention`'s auto-capture summary kept "`docket-auto-groom` is never a mint site"
   and the provable-termination invariant, but dropped its concrete consequence: minted stubs
   are themselves autonomous-eligible, making `auto_groom` × `auto_capture` a backlog-growth
   loop.

Both were left for merge-time judgment per the no-auto-fix triage rule and are now merged as-is.

## What changes

- Append the anti-deadlock rationale to the marker section of
  `skills/docket-finalize-change/references/gate-failure.md`.
- Restore the backlog-growth-loop consequence to
  `skills/docket-convention/references/auto-capture.md`.
- Sweep the other three files 0201 touched for the same defect class (a rule whose rationale
  was dropped rather than relocated) — per the learnings finding
  `fix-reintroduces-its-own-defect-class`, the change's own additions are the likeliest place
  for its defect class to reappear.

## Out of scope

- Any further size reduction of the Big 4; budgets stay at 0201's ratcheted values (the
  restored sentences are small, but re-measure rather than assuming headroom).
- Re-litigating what 0201 chose to extract.

## Open questions

- Whether the sweep finds enough further instances to justify a grep-able guard, or whether
  two fixes plus a manual read is the whole of it.
