---
id: 126
slug: apply-the-poison-value-prelude-uniformly-to-every-resolver-e
title: Apply the poison-value prelude uniformly to every resolver eval in the config suite
status: in-progress
priority: medium
created: 2026-07-22
updated: 2026-07-28
depends_on: []
related: [125]
discovered_from: [112]
adrs: []
spec: docs/superpowers/specs/2026-07-27-poison-prelude-uniformity-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/apply-the-poison-value-prelude-uniformly-to-every-resolver-e
claimed_at: 2026-07-28T02:15:02Z
pr:
blocked_by:
reconciled: false
type: chore
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-27-poison-prelude-uniformity-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-27-poison-prelude-uniformity-design.md) |
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

Settled at grooming (2026-07-27) — full design and assumptions in the linked spec.

- **Scope: `tests/test_docket_config.sh` only.** A whole-repo grep finds no sibling suite evaling
  command output into asserted variables. Roughly 50 of ~64 eval sites carry no clearing prelude.
- **Per-fixture clearing, using the poison-assignment idiom** (`VAR=__poison__`): clear exactly the
  variables the following asserts read, not a blanket line. The existing `unset`-idiom blocks stay
  byte-untouched. Sites whose asserts read only `$out`/`$err` text are exempt **by derivation**.
- **Mutation demonstration at the O→P `AUTO_GROOM` coincidence** (`tests/test_docket_config.sh:509–520`),
  not at `:500` — at `:500` the stale value is `none`, so the assert reddens rather than passing
  vacuously. Blocks O and P already leave and read the same `AUTO_GROOM=false` with nothing between,
  so aborting P's resolver run demonstrates the vacuous pass on the unmodified file.
- **A correspondence guard, not a presence guard**: it extracts the asserted variable names per
  segment, intersects them with the resolver's live-derived exported key set, and requires each to be
  cleared. A presence-only guard is green on exactly the failure it exists to stop. If correspondence
  proves infeasible at build time, ship **no** guard and record why.

## Open questions

Resolved at grooming; none remain open. The one judgment a human may want to revisit is whether the
enforcement guard belongs here at all — the spec's assumption 6 records why it was kept.

## Out of scope

- Rewriting the fixtures' structure or extracting shared helpers; section S's per-fixture shape was
  deliberately preserved by changes 0106 and 0112.
- Section S's own fixtures `s4`-`s9`, which already carry the prelude on every `eval`.

## Triage note (2026-07-26, change 0124)

Confirmed still live. The named `L2` gap is at `tests/test_docket_config.sh:501`:

```sh
out="$(rung "$tmp/n.xdg" "$tmp/n2" --export 2>/dev/null)"; eval "$out"
assert "0050 N: per-repo github honored" '[ "$BOARD_SURFACES" = "inline github" ]'
```

No poison prelude, and the assert reads `BOARD_SURFACES` immediately after — so the hazard shape is
present as described. Note the variable at risk here is `BOARD_SURFACES`, not
`FINALIZE_TEST_COMMAND`: the poison line must clear *the variable the following assert reads*, so
the audit is per-fixture, not a blanket copy of section S's line.
