---
id: 12
slug: groom-next-skill
title: Groom-next skill — pick the next needs-brainstorm stub and groom it to build-ready
status: done
priority: high
created: 2026-06-12
updated: 2026-06-12
depends_on: []
related: []
adrs: [4]
spec: docs/superpowers/specs/2026-06-12-groom-next-skill-design.md
plan: docs/superpowers/plans/2026-06-12-groom-next-skill.md
results: docs/results/2026-06-12-groom-next-skill-results.md
trivial: false
branch: feat/groom-next-skill
pr: https://github.com/danielhanold/docket/pull/7
blocked_by:
reconciled: true
---

## Why

Driven by actual usage: ideas get captured on the go (phone, on the road) as quick stubs, and the
design work happens later in a desk session. The convention already names the resulting state —
`needs-brainstorm` — and `docket-new-change`'s scan mode explicitly promises that "a later
brainstorm pass turns build-ready" stubs into real changes. But no skill owns that pass.
`docket-new-change` only mints *new* changes; nothing selects an *existing* stub and carries it to
build-ready. The gap is now concrete: the board currently holds six needs-brainstorm stubs
(0006–0011) with no skill whose job is to drain them.

The shape mirrors `docket-implement-next`: a "next" skill over a queue. The difference is the
queue (needs-brainstorm stubs instead of build-ready changes), the work (an interactive
brainstorm with the human instead of an autonomous build), and the exit state (build-ready
`proposed` with a `spec:` instead of `implemented` with a PR).

## What changes

- A new standalone operating skill, `docket-groom-next` (`skills/docket-groom-next/SKILL.md`,
  post-0005 reference-loading pattern, writes markdown only), that:
  - Selects the next needs-brainstorm change — `proposed`, no `spec:`, not `trivial: true` —
    using implement-next's deterministic order: `priority` → age (`created`) → lowest `id`;
    an explicit id overrides (an id that is not needs-brainstorm is an error, not a re-pick).
  - Treats unsatisfied `depends_on` as non-gating but states each dependency's status at
    session start — design ahead of builds; reconcile catches drift at build time.
  - Runs `superpowers:brainstorming` WITH THE HUMAN (selection is autonomous, the brainstorm is
    not), seeded with the stub's body and open questions, after the scan-related-context read —
    same stop-at-the-spec rule as `docket-new-change`.
  - Exits via one of four existing transitions, no new lifecycle status: spec written +
    `spec:` set (build-ready), `trivial: true` verdict (build-ready), the proposed-kill
    sub-path, or `deferred`.
  - Takes no claim: the final push's `pull --rebase`-and-retry loop is the CAS, with a re-read
    if the rebase touched the groomed change's file. Board pass as a separate must-land commit.
- Touch-ups: `docket-new-change` scan mode names `docket-groom-next` as its "later brainstorm
  pass"; `docket-convention`'s operating-skills enumeration grows to six; the hardcoded skill
  arrays in `tests/test_convention_extraction.sh` and `tests/test_docket_metadata_branch.sh`
  gain the new entry (`link-skills.sh` globs and needs no change).

Name settled 2026-06-12: `docket-groom-next`. "Grooming" is the Jira-lineage term for exactly
this transformation (stubbed-out item → ready to build), it keeps the `-next` symmetry with
`docket-implement-next`, and it avoids overloading "brainstorm", which the suite already uses
for designing a *new* change in `docket-new-change`. (Considered and rejected:
`docket-brainstorm-next` for that ambiguity; `docket-refine`, `docket-spec-next`,
`docket-ready-next` as less evocative.)

## Out of scope

- Building anything — it stops at build-ready, exactly where `docket-implement-next` picks up.
- Autonomous (no-human) spec writing — the brainstorm stays interactive; the human is the point.
- Batch mode (brainstorming several stubs in one run) — one stub per invocation, like
  implement-next; loop by re-invoking.
- Re-grooming changes that already have a spec — drift is the reconcile pass's job.
- New `.docket.yml` knobs, frontmatter fields, or lifecycle statuses — deliberately none.

## Reconcile log

- 2026-06-12 — Reconciled same-day as groom; codebase unmoved. One correction: `link-skills.sh`
  is glob-based (`skills/*/`) and needs no edit — the real inventory touch points are the
  hardcoded arrays in `tests/test_convention_extraction.sh` (`OPERATING=`) and
  `tests/test_docket_metadata_branch.sh` (`SKILLS=`). Spec §6 and the body updated to match.
  Verified: convention's five-skill enumeration and new-change's "later brainstorm pass" wording
  are as the spec assumes.
