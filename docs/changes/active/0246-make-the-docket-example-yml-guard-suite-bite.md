---
id: 246
slug: make-the-docket-example-yml-guard-suite-bite
title: 'Make the docket-example-yml guard suite bite'
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [178]
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

Consolidates #0178, #0187, and #0121 (2026-08-07 triage): three changes all editing `tests/test_docket_example_yml.sh`'s guard family. They must land together and in order, because #0178's defect means half the file's asserts — including the region #0187 hardens — may not execute at all on BSD-grep-first machines.

Verified 2026-08-07:

- **BSD-grep truncation (#0178).** The reported failure (a runtime parse error truncating the run to ~half its asserts while still reporting success) is PATH-dependent and needs a live reproduction; `bash -n` parses clean. Concrete leads: three ERE `\b` patterns at :374, :407, :568 — a class `AGENTS.md` and `tests/test_grep_portability.sh:50` explicitly ban (BSD/git-grep ERE returns zero silently); the portability guard only checks repetition bounds, so `\b` is unguarded. Reproduce under `/usr/bin/grep`-first PATH, fix, and extend the portability guard to the `\b` class.
- **Mirror guards one-directional (#0187), all three legs confirmed and one grown:** (1) the agents-mirror loop at :951-963 iterates the sidecar only, proving `sidecar ⊆ example` — a stale example row passes; the example file still claims equality. (2) The resolver round-trip slice terminator at :980 stops at the cursor `finalize-change` row, so the cursor build rows are excluded — and since 0192 the **entire opencode block** (through example line ~440) is also outside the slice; the ":990-992 all thirty-nine rows" comment is wrong. (3) The slice terminator is prefix-weak: `build-max:.*claude-opus-5` matches cursor's `claude-opus-5-high` row too (harness-defaults.yml:38 vs :68).
- **`elsewhere:` proves a word, not a read (#0121).** The :407 arm is a bare `grep -qE "\b$leaf_k\b"` over the consumer file — English prose in a SKILL.md or comment satisfies it. The consumer set has since grown (now includes `scripts/runners/opencode.sh`), widening the vocabulary surface. Note: killed #0147's analysis confirmed `(2b)`'s per-key checks are strictly stronger than `(2c)`'s union grep — the fix belongs here, on the `elsewhere:` arm, not on `(2c)`.

## What changes

1. First: reproduce and fix the BSD-grep truncation; convert the `\b` patterns to portable EREs; extend `test_grep_portability.sh` to guard the class.
2. Then: make the agents mirror bidirectional (example ⊆ sidecar too, or an explicit documented asymmetry); fix the round-trip slice to cover cursor build rows and the opencode block; make slice terminators non-prefix-matchable; correct the row-count comment.
3. Tighten the `elsewhere:` arm to evidence a code-shaped read (assignment/call shape, comment/prose excluded), per one of #0121's options — default to the shape-tightened grep rather than a new consumer-header convention.

## Out of scope

- The suite-wide toolchain pin/report decision — that is #0150 (re-scoped separately).
- `(2c)` orphan-key nested-key extension — #0147 killed as subsumed by `(2b)`.
