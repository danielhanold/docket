---
id: 330
slug: 'post-merge-results-appending-has-no-home-in-the-go-runtime-f'
title: 'Optional closeout notes preserve post-merge verification without rewriting frozen results'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-08-19'
updated: '2026-08-21'
depends_on: [316]
stacked_on:
related: [316, 331]
discovered_from: [316]
adrs: []
spec: 'docs/superpowers/specs/2026-08-21-terminal-closeout-notes-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/post-merge-results-appending-has-no-home-in-the-go-runtime-f'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-21T02:39:04Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-21-terminal-closeout-notes-design.md` |
<!-- docket:artifacts:end -->

## Why

The Bash finalize workflow once told the agent to append interactive-verification outcomes and late
findings to `results:` after merge. Change 0316's Go rewrite dropped that instruction, exposing that
the information had no typed owner. Restoring the instruction literally would now be wrong: the
repository's later frozen-artifact rule makes a merged results file a point-in-time build record that
authored closeout prose must not rewrite.

Finalize still needs a safe place for verification outcomes or late findings already supplied when
the human invokes closeout. Without one, that context is discarded or invites an unowned edit to a
merged artifact. The terminal change record is the durable home: it can distinguish what the build
knew from what closeout learned while preserving both.

## What changes

Extend `docket finalize closeout` with an optional structured request containing exactly two lists:
`verification_outcomes` and `late_findings`. Go renders non-empty lists under `## Closeout notes`
with `### Verification` and `### Late findings`, then lands those notes in the same transaction as
the explicit change's terminal closeout. Empty input preserves today's closeout byte-for-byte.

Keep finalize single-step. The finalize skill teaches callers to include already-known notes in the
invocation and routes supplied context into the request; it never pauses after merge or adds a human
checkpoint. The request participates in closeout's idempotency receipt so a response-loss retry cannot
duplicate notes and a later request cannot rewrite the frozen terminal record.

Document the terminal section in the convention, leave the merged-results freeze rule unchanged,
replace the obsolete skipped append assertion with semantic Go coverage plus a mutation-proven skill
handoff guard, and regenerate the embedded skill assets mechanically.

## Out of scope

Editing or redesigning `results:`, changing `attach-results`, adding free-form closeout Markdown or a
third category, adding a post-merge pause or lifecycle state, automatically creating follow-ups or
harvesting learnings, and the capabilities 0316 deliberately deferred: terminal publishing,
CI/combined gates, results-only skips, skill rebinding, and Bash fallback.
