---
id: 126
slug: apply-the-poison-value-prelude-uniformly-to-every-resolver-e
title: Apply the poison-value prelude uniformly to every resolver eval in the config suite
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: []
discovered_from: [112]
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

Section S of `tests/test_docket_config.sh` establishes a convention that every `eval` of the
resolver's `--export` output is preceded by a literal `FINALIZE_TEST_COMMAND=__poison__` line. The
poison line is load-bearing, not decoration: an aborted resolver run emits nothing, and a bare
`eval ""` would silently leave the **previous fixture's value** standing — so an assert could pass
by reading a stale value rather than the one its own fixture produced.

The whole-branch review of change 0112 audited poison coverage across the suite and found the
convention is **not applied uniformly**: the `L2` fixture at `tests/test_docket_config.sh:500`
evaluates the resolver's output with no poison line, unlike the section-S fixtures. It is
pre-existing (outside 0112's diff) and was correctly left alone by that change.

This is the `guards-are-code` rule the ledger already carries — "any test that `eval`s a command's
output must clear the variables it asserts on first." The gap is latent rather than currently
failing, which is exactly the shape that survives until someone looks.

## What changes

Audit every `eval` of resolver output in `tests/test_docket_config.sh` (and any sibling suite that
uses the same idiom), and apply the poison-value prelude uniformly wherever an assert reads an
exported variable afterwards.

Prove the fix rather than asserting it: for at least the `L2` case, demonstrate the hazard is real
by making the resolver abort for that fixture and showing the assert passes on the stale value
without the poison line, then reddens with it. That mutation is the completion bar — a poison line
added without a demonstrated hazard is decoration.

Consider whether the convention is better enforced than remembered: a guard asserting that every
`eval "$out"` in the file is immediately preceded by a poison assignment would keep the next
fixture author honest. Weigh that against the enumerated-floor risk before building it.

## Out of scope

- Rewriting the fixtures' structure or extracting shared helpers; section S's per-fixture shape was
  deliberately preserved by changes 0106 and 0112.
- Section S's own fixtures `s4`-`s9`, which already carry the prelude on every `eval`.
