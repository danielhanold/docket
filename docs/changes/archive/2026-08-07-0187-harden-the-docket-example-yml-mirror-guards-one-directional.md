---
id: 187
slug: harden-the-docket-example-yml-mirror-guards-one-directional
title: Harden the .docket.example.yml mirror guards — one-directional coverage, an unexercised round-trip slice, and a prefix-weak terminator
status: killed
priority: medium
type: chore
created: 2026-08-01
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [184]
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

Change 0184's whole-branch review probed `tests/test_docket_example_yml.sh`'s mirror
guards by mutation and found three weaknesses, all **pre-existing** — 0184 renamed the
rows the guards check but did not create any of these gaps, and fixing them was out of
its scope.

1. **The mirror runs one direction only.** The loop iterates the sidecar and asserts every
   shipped row appears in the example, proving `sidecar ⊆ example`. Nothing proves the
   converse, so a stale example row naming an agent that no longer exists goes undetected.
   `.docket.example.yml` claims the test "enforces the equality", which is currently false.
   This is a mirror, not a proper subset — both blocks claim to be complete — so per the
   learnings ledger's `correspondence-guard-runs-one-way` finding the reverse loop is
   mandatory rather than optional. 0184 added an orphan check for *retired profile names*
   only, which is a special case, not the general property.

2. **The round-trip never exercises the cursor build rows.** The `agents_block` slice
   terminates on the cursor block's `finalize-change` row, which sits above the cursor build
   rows. Its comment says the strip uncomments "all thirty-nine rows"; the real number is
   35. So the cursor build entries get mirror-equality coverage but never resolver
   round-trip coverage, and the comment misdescribes what is covered.

3. **The slice terminator is prefix-weak.** Each harness block's slice is anchored on its
   last build row plus that row's model ID. The claude anchor's ID (`claude-opus-5`) is a
   strict prefix of the cursor block's (`claude-opus-5-high`), so deleting the claude
   terminator row would let the claude slice run into the cursor block while the terminator
   guard still passed. Mutation showed the downstream mirror asserts do catch it today, so
   it is not exploitable — but the terminator guard is weaker than its comment claims.

Worth doing together: all three live in one file, and (1) and (3) share a root cause — the
guard trusts positional extraction where it could compare sets.

## What changes

To be designed. Sketch: add the reverse loop (example rows -> sidecar) as a set compare with
its own arity assert, mutation-proven in both directions; correct or derive the row-count
comment rather than restating it; and anchor each slice terminator on something that cannot
be a prefix of a neighbouring block's value.

## Out of scope

- Any change to the shipped pins themselves.
- The `agents/harness-defaults.yml` validator, which already checks its own correspondence
  in both directions.

## Why killed

Consolidated into #0246 at the 2026-08-07 backlog triage: all three legs verified (one-directional mirror, round-trip slice now also missing the opencode block, prefix-weak terminator); lands after the truncation fix in the same file.
