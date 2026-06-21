---
id: 37
slug: skill-fallback-progressive-disclosure
title: Slim skills — move the per-skill manual-fallback / script-contract prose into on-demand sibling files
status: proposed
priority: medium
created: 2026-06-21
updated: 2026-06-21
depends_on: [34]
related: [34]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Why

Each docket skill body carries detailed prose describing what its helper-script operations
do — e.g. `docket-new-change` / `docket-status` spell out the `archive-change.sh` /
`terminal-publish.sh` steps; the convention asserts "the prose is the contract the script
implements verbatim." That prose serves two readers: (a) a human/agent who wants to
understand what a step does (reference), and (b) the historical "hand-work it if the script
is absent" fallback. It is valuable, but it lives **in the always-loaded `SKILL.md`** of
every skill, so it inflates every skill's context cost on every invocation.

Change #34 makes the scripts reliably reachable and switches the skills to **fail loud**
(`${DOCKET_SCRIPTS_DIR:?…}`) rather than silently hand-work — which removes reader (b)'s
*urgency* but not the prose. So the prose is now mostly reader (a): the script's
**operations/contract reference**. It should be **kept** (it is the human-readable spec of
each script) but **moved off the hot path**.

## What changes

Relocate the per-skill manual-fallback / script-contract prose out of each `SKILL.md` into
an **on-demand sibling file**, linked from the skill via **progressive disclosure** — the
same pattern docket already uses for `docket-convention/github-board-mirror.md` ("read it
when `board_surfaces` includes `github`") and the `*-template.md` siblings.

- Each skill's `SKILL.md` keeps a short pointer ("if a script is unreachable / you need the
  operation's contract, see `<sibling>.md`") instead of the inline prose.
- The detailed operations prose moves to the sibling, loaded only when needed.
- Net effect: every `SKILL.md` shrinks; no prose is lost.

## Out of scope

- The script-reachability fix itself (`DOCKET_SCRIPTS_DIR`, install-time injection,
  fail-loud) — that is **#34**, which this builds on.
- Rewriting the scripts or their behaviour.

## Open questions

- **Framing/naming of the sibling.** Post-#34 the prose is really the script's
  *operations/contract reference*, not a "re-implement by hand" path. Name and frame it as
  such (e.g. `operations.md` / `<skill>-contract.md`) rather than "manual-fallback", so it
  reads as authoritative reference, not an encouraged escape hatch?
- **Granularity.** One sibling per skill, or one shared reference per *script* that multiple
  skills link (several skills invoke the same scripts — `archive-change.sh`,
  `terminal-publish.sh`, `render-board.sh`)? A per-script contract referenced by many skills
  avoids restating the same prose in N siblings.
- **Drift discipline.** The script stays the source of truth; the sibling must not silently
  drift from it. Add a check (mirroring #34's CI drift-guard) that the contract reference
  stays in sync — or keep the convention's "prose is the contract" assertion as the binding
  rule and audit at review.
- **Which skills.** All eight bodies, or only those with substantial inline operations prose?
