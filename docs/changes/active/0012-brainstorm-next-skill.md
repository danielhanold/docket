---
id: 12
slug: brainstorm-next-skill
title: Brainstorm-next skill — pick the next needs-brainstorm stub and drive it to build-ready
status: proposed
priority: high
created: 2026-06-12
updated: 2026-06-12
depends_on: []
related: []
adrs: []
spec:
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
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

- A new skill (working name `docket-brainstorm-next` — naming is an open question) that:
  - Selects the next needs-brainstorm change — `proposed`, no `spec:`, not `trivial: true` —
    using the same deterministic order as implement-next: `priority` → age (`created`) →
    lowest `id`; an explicit id can be passed to override.
  - Runs `superpowers:brainstorming` WITH THE HUMAN on that change (selection is autonomous,
    the brainstorm is not), informed by the stub's body, related changes, and ADRs — same
    stop-at-the-spec rule as `docket-new-change`.
  - Writes the spec to the metadata branch, sets `spec:`, refreshes the stub's body from the
    brainstorm outcome, updates `updated:`, runs the Board pass, pushes — the change leaves the
    board as `needs-brainstorm` and becomes build-ready.
  - May conclude the stub should die instead — then it follows the existing proposed-kill
    sub-path rather than forcing a spec.
- Convention/`docket-new-change` touch-ups pointing "a later brainstorm pass" at the new skill.

## Out of scope

- Building anything — it stops at build-ready, exactly where `docket-implement-next` picks up.
- Autonomous (no-human) spec writing — the brainstorm stays interactive; the human is the point.
- Batch mode (brainstorming several stubs in one run) — one stub per invocation, like
  implement-next; loop by re-invoking.

## Open questions

- **Name.** Candidates: `docket-brainstorm-next` (mirrors implement-next; the board's
  `needs-brainstorm` label points straight at it), `docket-refine` (Scrum "backlog refinement"),
  `docket-spec-next` (names the output), `docket-ready-next` (names the exit state),
  `docket-groom` (older grooming term).
- Does selection require `depends_on` satisfied? Building needs deps `done`, but brainstorming a
  dependent change early is often fine (reconcile catches drift) — maybe deps-unsatisfied stubs
  are eligible but flagged.
- Does it claim? Implement-next CAS-claims to exclude concurrent builders; a brainstorm is
  human-attended, so collisions are unlikely — is a claim (or a lighter marker) worth it?
- Relationship to `docket-new-change`: separate sixth operating skill, or a mode of
  `docket-new-change` ("brainstorm an existing stub")?

## Reconcile log
