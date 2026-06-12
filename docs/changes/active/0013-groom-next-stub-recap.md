---
id: 13
slug: groom-next-stub-recap
title: Groom-next recap — introduce the selected stub before the brainstorm starts
status: in-progress
priority: medium
created: 2026-06-12
updated: 2026-06-12
depends_on: []
related: [12]
adrs: []
spec:
plan:
results:
trivial: true
branch: feat/groom-next-stub-recap
pr:
blocked_by:
reconciled: true
---

## Why

`docket-groom-next` (#12) works as designed: it selects the next needs-brainstorm stub
correctly, then runs `superpowers:brainstorming` with the human. But in real use it jumps
straight into the brainstorm's question-and-answer session with no introduction to what the
chosen change is actually about. That is fine when the human just wrote the stub, but the
skill's whole premise (per #12's Why) is that stubs are captured on the go and groomed later —
from a phone, or from a fresh empty session with zero context. A cold-start human cannot answer
design questions about a change they have not been reminded of. Step 1 already requires stating
dependency statuses at session start; nothing requires recapping the stub itself.

## What changes

In `skills/docket-groom-next/SKILL.md`, between selection and the brainstorm: present a
**recap of the selected stub** before invoking `superpowers:brainstorming` — written for a
reader with no prior context. The recap covers:

- What was selected and why: id, title, priority (and that it was the deterministic pick, or
  the explicitly requested id).
- A PM-altitude summary of the stub — its `## Why` and `## What changes` distilled into a few
  sentences a phone reader can absorb.
- The dependency statuses Step 1 already requires, folded into the same recap.
- The stub's `## Open questions`, framed as the agenda the brainstorm is about to work through.

Then proceed **directly** into the brainstorm Q&A — the recap is an introduction, not a
confirmation gate; the human can redirect in the brainstorm itself.

## Out of scope

- Any change to selection logic, the four exits, the no-claim concurrency model (ADR-4), or
  the stop-at-the-spec rule.
- A confirmation prompt before brainstorming — the recap flows straight into the Q&A.
- Recaps in other skills (e.g. `docket-implement-next` is autonomous; no human to recap for).
- New `.docket.yml` knobs, frontmatter fields, or lifecycle statuses.

## Reconcile log

- 2026-06-12 — Reconciled same-day as proposal; codebase unmoved since #12 merged. Verified:
  `skills/docket-groom-next/SKILL.md` on `origin/main` is byte-identical to the installed copy
  the proposal was written against; Step 1 requires stating dependency statuses but no recap of
  the stub itself; Step 3 enters the brainstorm directly. Per the #12 learning, plumbing needs
  no edits — `link-skills.sh` globs, and the three test files only assert skill-inventory
  membership plus a `LEARNINGS.md` read line, none of which this edit touches. Scope holds:
  single-file edit to the skill text.
