---
id: 302
slug: mint-stub-dedup-misses-a-general-form-parent-of-a-specific-d
title: 'mint-stub dedup misses a general-form parent of a specific discovery'
status: 'deferred'
priority: medium
type: fix
created: 2026-08-12
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [298]
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

**Trigger** — surfaced closing out #0298. That run minted #0299 as a stub for test-file resharding
work that #0280 already owned; `mint-stub.sh`'s dedup did not match, because #0280's title states
the general form of the problem and #0299's stated the specific instance the run had just hit.
#0299 was killed the same day and its terminal record published — a full mint, groom-queue entry,
kill, and publish spent on a duplicate.

**Opportunity** — dedup that can match a specific discovery against an existing general-form parent.
Today's check is title similarity, which is structurally blind to exactly this pair: the two titles
share little surface text precisely because one is the generalization of the other.

**Independent value** — every autonomous mint site pays this cost, and the failure is silent in the
direction that hurts: a missed match creates backlog, a false match suppresses real work. Worth
doing with stacked changes entirely reverted.

**Boundary** — `mint-stub.sh`'s dedup predicate and its contract, plus fixtures covering the
general/specific pair. It stops at the mint boundary: it does not touch grooming, the kill path, or
the auto-capture admission gates that decide whether a mint is attempted at all, and it must not
become a model call inside a deterministic script (ADR-0012).

**Reason for deferral** — the mint machinery is untouched by #0298's stacking work and shares no
files with it; folding a dedup redesign into that branch would expand its scope past the stacking
contract it exists to deliver.

## Why deferred

Backlog review 2026-09-02 (Bash→Go migration): the fix site is mint-stub's dedup predicate; automatic change capture / mint-stub is deferred from Go v1. The general-vs-specific title dedup problem survives the port and should revive with the Go auto-capture successor (ADR-0012 still applies: no model call inside a deterministic op).
