---
id: 6
slug: learnings-ledger
title: Learnings ledger — an append-only per-repo memory that builds feed and future builds read
status: in-progress
priority: high
created: 2026-06-11
updated: 2026-06-12
depends_on: []
related: [1, 12]
adrs: [5]
spec: docs/superpowers/specs/2026-06-12-learnings-ledger-design.md
plan: docs/superpowers/plans/2026-06-12-learnings-ledger.md
results:
trivial: false
branch: feat/learnings-ledger
pr:
blocked_by:
reconciled: true
---

## Why

Synthesized from the AgentRQ competitive review (2026-06-11). AgentRQ keeps a per-workspace
`selfLearningLoopNote` — a free-text field agents append learned human preferences to, so every
later session starts already knowing them. Separately, the AgentRQ repo itself maintains
`.jules/sentinel.md`, an append-only "security lessons learned" ledger (path traversal, IDOR,
stored XSS, …) that a review bot consults and extends — a self-documenting regression memory.

docket has no equivalent. Each `docket-implement-next` run starts cold: PR review feedback, human
corrections at the merge gate, and recurring review findings (the kind captured in results files,
change 0001) evaporate once the change archives. The same class of mistake can recur build after
build because nothing carries it forward. CLAUDE.md covers durable project conventions, but there
is no low-ceremony, docket-owned place where the *build loop itself* deposits and consumes lessons.

## What changes

- `<changes_dir>/LEARNINGS.md` on the metadata branch — no new knob; never published to the
  integration branch. Flat dated entries, newest first, each with provenance (change id / PR)
  and an actionable phrasing; curated prose, never regenerated.
- Harvest single-sourced in `docket-finalize-change` as a close-out step (zero entries is fine);
  `docket-status`'s sweep invokes the same procedure by reference, best-effort, with an
  entry-per-change-id idempotency probe. Kills are not harvested.
- Readers: `docket-implement-next` at plan time and its review step; `docket-groom-next` in its
  scan-related-context step.
- Distill at a ~300-line soft cap: merge near-duplicates, drop entries promoted to CLAUDE.md or
  the convention (the boundary rule lives in the file header). Git history keeps what's dropped.
- Convention gains a *Learnings ledger* subsection (single source of the rules); the build
  retro-seeds the ledger from already-archived changes' results files so it ships non-empty.

## Out of scope

- Anything server-side or database-backed — the ledger is a markdown file in git.
- Replacing CLAUDE.md: the ledger is build-loop memory, not general project instructions.
- Automatic promotion of ledger entries into CLAUDE.md (could be a later change).
- Harvesting killed changes; mid-build appends; write access outside the harvest procedure.

## Reconcile log

- 2026-06-12 — Reconciled same-day as groom; codebase unmoved since. One correction: five results
  files exist on the integration branch for retro-seeding (0001, 0002, 0003, 0005, 0012), not the
  two the spec named; spec §6.6 updated, with the zero-entries-is-fine rule applied retroactively.
  Verified the read-site line numbers still hold in docket-implement-next / docket-groom-next /
  docket-finalize-change / docket-status as the spec assumes.
