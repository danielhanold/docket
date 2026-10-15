---
id: 554
slug: 'update-cursor-review-tier-defaults-to-grok-4-7'
title: 'Update Cursor review-tier defaults to Grok 4.7'
status: 'proposed'
priority: 'medium'
type: 'chore'
created: '2026-10-15'
updated: '2026-10-15'
depends_on: []
stacked_on:
related: [195]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Grok 4.7 has been released. Docket's shipped Cursor defaults should pin the review tiers to the newer model, not the previous generation.

## What changes

Update the Cursor block of the built-in model defaults (`agents/harness-defaults.yml`) so the three review tiers (`review-lean`, `review-standard`, `review-deep`) use Grok 4.7 model IDs. Mirror the change in the commented Cursor example in `.docket.example.yml`. Daniel will supply the exact Cursor model IDs at grooming.

State found at stub time (origin/main, 2026-10-15): the shipped Cursor defaults use `cursor-grok-4.5-*` (not 4.6). `review-lean` = `cursor-grok-4.5-medium`, `review-standard` = `cursor-grok-4.5-high`, and `review-deep` = `claude-opus-5-high` (not Grok). Grooming should confirm whether `review-deep` moves to Grok too and whether the other `cursor-grok-4.5-*` rows stay as they are.

## Out of scope

Cursor agents other than the review tiers, unless grooming widens the scope. Model defaults for other harnesses (Claude, Codex, OpenCode).
