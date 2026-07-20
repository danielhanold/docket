# Pin the `finalize.test_command` auto sentinel's cross-layer masking

**Change:** 0106
**Date:** 2026-07-20
**Status:** design settled

## Problem

Change 0101 added the `auto` sentinel for `finalize.test_command` (`auto` ≡ unset), so
`.docket.yml.example` can ship the default as an active value rather than a commented note. The
sentinel's correctness rests on *where* it is collapsed to "unset".

`scripts/docket-config.sh:194` resolves the key across three rungs, through two distinct helpers:

```sh
FINALIZE_TEST_COMMAND="$(lcl test_command)"                        # .docket.local.yml   (lcl, :153)
FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(yaml_get "$CFG" test_command)}"   # committed .docket.yml
FINALIZE_TEST_COMMAND="${FINALIZE_TEST_COMMAND:-$(gbl test_command)}"               # global config.yml (gbl, :140)
[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""    # :201 — AFTER the chain
```

The collapse at `:201` runs **after** the chain, which is what makes a **higher** layer's `auto`
mask a **lower** layer's real command — the correct reading of an explicit re-statement of the
default. Collapse per-layer instead and the behavior silently inverts: the higher `auto` becomes
empty, the `:-` chain falls through, and the lower layer's command resurfaces.

Nothing tests this. `tests/test_docket_config.sh` section S (`:987`) covers the sentinel in three
single-layer fixtures (`$tmp/s`, `s2`, `s3`) — each writes only a committed `.docket.yml`, so no
fixture ever has two rungs populated at once. The cross-layer property is asserted by the comment at
`:195-199` and nowhere else.

That comment has already been wrong once. Commit `a9da1e2` shipped it describing the property
backwards ("a lower layer's `auto` cannot resurrect a higher layer's real command"); `dab12b0`, the
0101 review-findings commit, corrected it to the higher-masks-lower statement. A property that a
careful reader misdescribed once, and that only a human review caught, is exactly the shape
[[verify-the-claim]] and [[guards-are-code]] name: a comment is a claim, not a guard.

## Design

Three fixtures appended to section S of `tests/test_docket_config.sh`, using the existing `mkrepo`
and `rung` helpers. `rung` (`:33`) roots the global layer at a per-fixture temp dir and the suite
pins `XDG_CONFIG_HOME` at a void (`:31`), so every fixture is hermetic — no fixture reads or writes
a real `~/.config`. This closes the stub's open question: the hermeticity hazard that argued against
the global layer was fixed by change 0050, so both higher layers are equally safe to exercise.

| Case | Lower layer | Higher layer | Assert |
|---|---|---|---|
| `s4` | committed `.docket.yml`: `make test` | `.docket.local.yml`: `auto` | `FINALIZE_TEST_COMMAND` is empty |
| `s5` | global `config.yml`: `make global` | committed `.docket.yml`: `auto` | `FINALIZE_TEST_COMMAND` is empty |
| `s6` | global `config.yml`: `auto` | committed `.docket.yml`: `make test` | `FINALIZE_TEST_COMMAND` == `make test` |

`s4` exercises the `lcl()` read path; `s5` exercises `gbl()`. The third possible ordering
(local `auto` over global real) is deliberately omitted — it reuses the two helpers `s4` and `s5`
already cover and adds no distinct code path.

`s6` is the reverse direction, required by the two-sided-proof rule [[guards-are-code]] harvested
from change 0091: a guard that can fail by being too loose **and** by being too tight must be
mutation-tested in both directions, because a one-sided test blesses whichever error it does not
probe. Here the too-tight defect is a lower layer's `auto` wrongly wiping out a higher layer's real
command.

## Mutation testing

Both mutations are applied to the real `scripts/docket-config.sh`, not to a fixture copy — a fixture
battery only samples the shapes already thought of ([[guards-are-code]], item g).

**Mutation 1 — collapse per-layer instead of after the chain.** Replace the `:201` collapse with a
per-rung one, e.g. `_l="$(lcl test_command)"; [ "$_l" = auto ] && _l=""` and equivalently for the
committed rung. Expected: `s4` falls through to `make test` and `s5` to `make global`; both go
**red**. This is the refactor the change exists to prevent.

**Mutation 2 — blanket "any layer says `auto` ⇒ unset".** Replace the collapse with a scan that
clears the value when *any* rung holds the sentinel, regardless of precedence. Expected: `s6` goes
**red** (its `make test` is wrongly cleared by the global `auto`).

`s6` does **not** redden under Mutation 1, and this is stated explicitly rather than left implied:
each fixture must be shown to redden under some mutation, or it is decoration. [[guards-are-code]]
item (k) documents the failure mode where an assert ceases to exist rather than failing, and a
pass/fail-only reading of the suite calls that green.

**Completion bar:** read the suite's `ok` count as part of the contract on every mutation run. A
mutation that lowers the count while producing zero `NOT OK` is a vacuous guard announcing itself.
Record the before/after counts.

## Out of scope

- Any change to the sentinel's semantics or the placement of the collapse. This change pins current
  behavior; it does not alter it.
- Extending the `auto` sentinel to `github_project`, which is unwired end to end — that key belongs
  to change 0103.
- The third layer ordering (local `auto` over global real), per the rationale above.

## Notes

The stub cited `scripts/docket-config.sh:197`, which lands mid-comment. The load-bearing anchors are
`:194` (the resolution chain) and `:201` (the collapse); cite that range instead.
