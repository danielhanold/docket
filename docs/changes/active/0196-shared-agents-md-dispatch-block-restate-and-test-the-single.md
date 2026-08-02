---
id: 196
slug: shared-agents-md-dispatch-block-restate-and-test-the-single
title: Shared AGENTS.md dispatch block — restate and test the single-owner assumptions
status: proposed
priority: medium
type: fix
created: 2026-08-02
updated: 2026-08-02
depends_on: []
related: []
discovered_from: [192]
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

Change 0192 (PR #150) merged with four **Important** review findings and a set of Minor ones left
for merge-time judgment rather than auto-fixed (0 blockers). They all fall out of the same move:
the project-root `AGENTS.md` dispatch block became **shared** between Codex and opencode, and the
assumptions written when Codex was its sole owner were not all restated or tested. See the
learnings finding `shared-resource-keeps-first-owner-assumptions`.

## What changes

- `docs/codex/setup.md`'s de-list note is now false — it still says de-listing Codex removes the
  `AGENTS.md` block. With the block shared, de-listing Codex from a repo still targeting opencode
  leaves it in place. Give it the same "last dispatch harness" caveat `docs/opencode/setup.md` has.
- Add a **two-dispatch-harness fixture**. No test configures Codex and opencode at once, so the
  central shared-ownership property is unasserted: the block appears exactly once, and de-listing
  one harness while the other remains leaves it in place. Both fixtures added in 0192 pass against
  the old codex-only predicate, so the suite cannot currently detect a regression to it.
- The opencode emitter test asserts only frontmatter. If body extraction or the skills preamble
  regressed to empty, the definition would carry no prompt and the suite would stay green. Both
  sibling emitters guard this; match their coverage.
- The effort-drop-when-no-model rationale is stated as verified behavior in three places but the
  model-less case was never probed. Unlike Cursor (where effort is encoded inside the model string
  and must drop), opencode has an independent `reasoningEffort:` key. Probe it, then either keep
  the claim or correct all three sites.
- Minor cleanups from the same review: the shared block's head claims defaults "on every harness it
  supports", overreaching past the shipped set to the unpinned `agents`/`kiro`/`windsurf` tokens;
  two new guards key on enumerated spellings rather than shape and grep the whole file rather than
  the marker-bounded block; the `--check` diagnostic hand-lists "(codex, opencode)" instead of
  interpolating the variable; and the agent-layer table's opencode cell holds a full path where its
  three siblings hold a bare extension.

## Out of scope

The human merge-gate items recorded in 0192's results file (OpenRouter entitlement, live rung
certification, one real end-to-end dispatch) — those are verification acts, not code changes.
