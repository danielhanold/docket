---
id: 141
slug: factor-the-shared-wrapper-source-parse-out-of-the-named-harn
title: Factor the shared wrapper-source parse out of the named harness emitters
status: proposed
priority: medium
type: refactor
created: 2026-07-27
updated: 2026-07-27
depends_on: []
related: []
discovered_from: [135]
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

Change 0135 added `emit_cursor_md()` to `sync-agents.sh` alongside the existing `emit_codex_toml()`.
The two are roughly 90% textually identical: both extract `name`, `description`, the built-in
`model`/`effort`, the `skills:` list, and the body from the same built-in wrapper source, using the
same `sed`/`awk` incantations, before diverging only at the point where they serialize into their
target harness's shape.

Two named emitters sharing a parse is tolerable duplication. A third would not be — and ADR-0060
(recorded by 0135) makes a third likely: it establishes that every harness docket claims to support
gets a named emitter conforming to that harness's own documented contract, and that the generic
`*)` arm is a documented gap rather than a supported mapping. `agents`, `kiro`, and `windsurf` are
already accepted tokens sitting on that arm.

The duplication is also the kind that rots asymmetrically: a fix to one parse (a quoting edge case,
a frontmatter shape change in the built-in sources) can silently miss its twin. This repo already
carries a learning about exactly that failure mode for duplicated helpers.

## What changes

Extract the shared wrapper-source parse into one helper — something like
`parse_wrapper_source()` — that both named emitters call, leaving each emitter responsible only for
serializing into its own target shape.

The extraction must be behavior-preserving: the Claude, Cursor, and Codex generation tests are the
gate, and each should stay byte-identical across the refactor. Prefer a shape that makes the NEXT
emitter cheap to add, since that is the whole reason to do it.

Worth deciding as part of this: whether the helper returns values by convention (setting a fixed
set of variables) or by emitting parseable output, given Bash 3.2 has no associative arrays and no
`declare -n`.

## Out of scope

- Adding a new harness emitter. This is the enabling refactor, not the next harness.
- Changing any emitted wrapper's bytes. A diff in generated output means the refactor is wrong.
