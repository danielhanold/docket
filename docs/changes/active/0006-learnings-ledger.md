---
id: 6
slug: learnings-ledger
title: Learnings ledger — an append-only per-repo memory that builds feed and future builds read
status: proposed
priority: high
created: 2026-06-11
updated: 2026-06-11
depends_on: []
related: [1]
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

- A `LEARNINGS.md` ledger on the metadata branch (location/knob to be decided in the brainstorm,
  e.g. `<changes_dir>/LEARNINGS.md`) — append-only dated entries, each a short lesson with its
  provenance (change id, PR review comment, reconcile finding).
- `docket-finalize-change` gains a harvest step: at close-out, distill merge-gate feedback and PR
  review comments into ledger entries.
- `docket-implement-next` reads the ledger at reconcile/plan time and again at its review step, so
  past lessons shape the build and the self-review checklist.
- Convention addition in `docket-convention` defining the ledger format and ownership rules.

## Out of scope

- Anything server-side or database-backed — the ledger is a markdown file in git.
- Replacing CLAUDE.md: the ledger is build-loop memory, not general project instructions.
- Automatic promotion of ledger entries into CLAUDE.md (could be a later change).

## Open questions

- One flat file vs. categorized sections (security / review-feedback / process)?
- Pruning/compaction policy — does the ledger ever get distilled, or grow forever?
- Should the harvest step also mine results files (change 0001) retroactively?

## Reconcile log
