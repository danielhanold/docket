---
id: 76
slug: quote-leg-rule-binds-by-role-not-reader-shape
title: ADR-0065's quote-leg rule binds by role, not by reader shape
status: Accepted
date: 2026-08-08
supersedes: []
reverses: []
relates_to: [65, 72]
change: 255
---

## Context

ADR-0065 established that every `field`/`field_raw` reader pair in docket needs an explicit quote
leg beside its raw-vs-consumed comparison. That comparison is a whitespace test only: a quoted but
space-free value (`{model: "claude-opus-5"}`) has consumed == raw, so the `!=` leg is structurally
blind to it and the quote characters ride into the emitted wrapper pin.

Change 0255 set out to apply that rule to `hd_validate` in `scripts/lib/harness-defaults.sh` — the
twin ADR-0065 named as still missing the leg. The spec's inventory of affected sites listed exactly
two validators: `hd_validate` and `validate_user_agent_values`.

The whole-branch review found a third site the inventory missed, and it was the one that matters
most: `validate_harness_defaults` in `sync-agents.sh`. That function short-circuits to `hd_validate`
only when `${BASH_VERSINFO[0]} -lt 4`; on Bash 4+ — the default on every developer machine, and both
call sites, the real run and `--check` — an awk single-pass validator runs instead. It computed
`consumed=raw; sub(/[[:space:]].*$/,"",consumed)` and compared: the same whitespace-only test, on the
primary execution path, validating docket's own shipped sidecar. A quoted pin passed and shipped.

It was missed because it does not *look* like a `field`/`field_raw` pair. It has no such functions —
it is one awk program that inlines the same two readings. ADR-0065's rule was written in terms of the
reader shape it was discovered on, so a site performing the same role in a different shape read as
out of scope to a careful reader working from the spec.

## Decision

The rule binds any code that decides whether a config value is consumable — by **role**, not by
whether it exposes a `field`/`field_raw` pair. Wherever docket compares a permissively-read value
against a narrowly-read one to judge well-formedness, that comparison needs the explicit quote leg
beside it, regardless of whether the two readings live in two named functions, one awk program, or
anything else. The same applies to the sibling `#`-inside-the-flow-map leg change 0255 added.

The practical corollary: docket now has three copies of the flow-map-comment predicate
(`_hd_flow_map_has_comment` in `harness-defaults.sh`, `flow_map_has_comment` in `sync-agents.sh`, and
the awk `flow_comment()` inside `validate_harness_defaults`) and three copies of the quote leg.
Duplication by value is the accepted design — `harness-defaults.sh`'s header forbids coupling the
shipped-data reader to the user-config readers, and extraction is deferred to change #0256 — so the
correspondence is pinned by a three-way table-driven parity test in
`tests/test_harness_defaults_flow_map.sh` rather than by shared code. That test is the regression net
#0256's extraction will be checked against.

## Consequences

- **Enables:** a future reader auditing the family greps for the role (a consumable-value judgement)
  rather than for a function-name pattern, so a fourth implementation in a fourth shape is findable.
- **Costs:** three copies to keep in step, with drift caught by a test rather than prevented by
  construction — the same accepted cost ADR-0072 recorded for its duplicated gate. Two validators
  that both hard-abort generation now exist on version-dependent paths, so a diagnostic difference
  between them is a real, user-visible defect class; the parity test covers verdicts, and change 0255
  also made the two emit byte-identical diagnostics for the flow-map case.
- **Gives up:** the simplicity of a rule stated in terms of a concrete code shape, which was easier
  to check mechanically but proved to under-cover.
- **A live reminder of the cost:** the spec-level inventory of "every `field`/`field_raw` validator
  pair" was written by careful reading and still missed the primary execution path. The finding was
  caught by whole-branch review, not by the suite — before the fix, every test in the repo passed
  with a quoted pin shipping on Bash 4+.
