---
id: 86
slug: in-context-gating-dispatch-carved-out-of-the-tier-taxonomy
title: "An in-context-gating dispatch sits outside the dispatch-capability tier taxonomy by carve-out, not as a fourth tier"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [59, 85]
change: 260
---

## Context

ADR-0059 (change 0137) established that dispatch capability is resolved, never inferred from a tool
name, and that unavailability is **tiered**: Tier A deterministic (the `docket-status` /
`docket-adr` composition dispatches — run inline, a first-class equivalent path, because their
contract is git state on `metadata_branch`), Tier B adversarial (the `docket-auto-groom-critic`
gate — abstain), Tier C discipline (the `build` and `review` role skills plus the in-branch fix
workers — authorized-or-halt, where an explicitly configured `skills.build: auto` is the human's
authorization to run inline).

`docket-finalize-change`'s two merge-gate dispatches — `docket-rebase-resolver` (rebase-conflict
reconciliation) and `docket-integration-repair` (red suite after the rebase lands) — matched no row.
The gap was deliberately deferred and machine-pinned rather than improvised:
`tests/test_dispatch_capability.sh` carried a `PENDING_TIER` variable asserting exactly those two
knowingly-untiered sites, with a comment stating it MUST SHRINK TO EMPTY when a follow-up change
tiered them. Nothing was unsafe in the meantime — `gate-failure.md`'s abort-and-report set already
covered both situations. Change 0139 was killed into 0260 with halt/carve-out as its own stated
conclusion.

## Decision

These two dispatches sit **outside** the A/B/C taxonomy by an explicit carve-out paragraph
immediately after the tier table, not in a fourth row. The reason is their return channel: their
contract is an **in-context report gating the merge**, not git state on `metadata_branch`. Neither
tier posture can be borrowed — Tier A's first-class-equivalent inline path presupposes a git-state
transition to reproduce, and Tier C's authorized-or-halt presupposes a `skills:` role whose resolved
value could carry a human's `auto` authorization; these dispatches have neither.

When dispatch is genuinely unavailable for either (established per the resolution rule, never from a
tool name) the posture is finalize's own pre-existing **abort-and-report**: the gate stops, the PR
stays open, the change stays `implemented`. Inline substitution is **forbidden** for both —
reconciling the conflict, or authoring the repair, inside the very agent that would then merge that
work is the same self-approval shape Tier B rejects for the critic. The posture is also a **named
member** of `gate-failure.md`'s abort-and-report enumeration, so the carve-out's pointer resolves to
a listed reason rather than an implied one. The canonical site marker lives only in
`gate-failure.md`, which `SKILL.md` blocking-loads at both dispatch moments — deliberately not
copy-pinned into `SKILL.md`'s dispatch sentences.

**Alternatives rejected** (from the spec's audited assumptions):

- **A Tier D table row** — implies a new posture *kind* when the posture is finalize's pre-existing
  one, and forces the cross-file coherence guard to treat an outside-the-taxonomy case as a taxonomy
  member.
- **Inline resolution/repair with sign-off** — the self-approval shape change 0139's own body argued
  against; the agent would merge its own repair.
- **A new `halted`-flavour status or field** — same reasoning the `## Finalize blocked` design
  already records: an eighth status flattens distinct reasons and touches the board renderer, the
  GitHub mirror's seven-state mapping, and the health checks.
- **Deleting the `PENDING_TIER` mechanism once emptied** — it stays empty-pinned at count 0,
  preserving "a knowingly-untiered dispatch site is an in-diff decision, never a silent one".

## Consequences

The taxonomy now has a stated **shape rule** rather than an implicit assumption that every dispatch
is tierable; a future dispatch whose contract is an in-context return has a home and a precedent.
The cross-file coherence guard keys on posture-label **shape** (`Tier <letter>` versus anything
else) and dispatches each recognised class to its own derived check, failing outright on an
unrecognised label.

Cost: two classes of dispatch posture to reason about instead of one uniform table, and the
carve-out's agreement with its wired sites is guarded by a paragraph-scoped derivation rather than a
table-row lookup.

Relates to ADR-0059 — the taxonomy this carves out of; its three tiers stand unchanged, and this is
neither a supersession nor a reversal. ADR-0085 (a critic verdict travels on exactly one channel,
the foreground return) reinforces the in-context-return reasoning here.
