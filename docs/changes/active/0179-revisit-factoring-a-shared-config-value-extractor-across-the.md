---
id: 179
slug: revisit-factoring-a-shared-config-value-extractor-across-the
title: Revisit factoring a shared config value extractor across the three readers
status: proposed
priority: medium
type: refactor
created: 2026-07-31
updated: 2026-07-31
depends_on: [175]
related: [168, 173]
discovered_from: [173]
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

Three config readers extract a value with their own copy of the same character class, and the class
has already been wrong in two of them. `harness-defaults.sh`'s `hd_field` was fixed by change 0168;
`sync-agents.sh`'s `field_of` and `scripts/runner-dispatch.sh`'s value read are fixed by change
0173. Each fix is a separate copy of the same rule, which is the duplication that produced the bug
class in the first place — the `fix-reintroduces-its-own-defect-class` learning applies directly.

Change 0173 considered factoring a shared extractor and deliberately rejected it, for reasons that
were true at the time and are worth re-testing rather than inheriting:

- `hd_field` is keyed on `(file, harness, agent, field)` and does its own line lookup, while
  `field_of` receives a line it was handed — same extraction, different signatures.
- The three readers parse three different YAML shapes (flow map, flow map with lookup, block
  mapping), and 0173 deliberately gives two of them *different* value classes. The duplication is
  narrower than a first read suggests.
- Change 0175 rewrites `field_of`'s internals for performance, so factoring beforehand is work done
  twice.

The third reason expires when 0175 lands, which is what makes this worth revisiting then rather
than never.

## What changes

- Re-examine whether a shared value extractor is warranted once 0175 has settled `field_of`'s
  implementation, or whether the three readers are genuinely different enough to stay separate.
- If shared: settle the signature (line-in vs lookup-in), where it lives, and how the
  block-mapping reader's different value class and tolerant posture are expressed through it.
- If not shared: record the decision so a fourth reader does not re-open it from scratch — a
  cross-referencing comment in each reader, or an ADR.

## Out of scope

- Changing any reader's behavior. This is a structural question; 0168 and 0173 own the semantics.
- Any vendor model allowlist (ADR-0015).

## Open questions

- Is the right artifact a shared function, or an ADR recording that the readers stay separate?
- Does `docket-frontmatter.sh`'s `field`/`field_raw` pair belong in the same consolidation, or is
  its quote-style split genuinely a different concern from the reader-capability split?
