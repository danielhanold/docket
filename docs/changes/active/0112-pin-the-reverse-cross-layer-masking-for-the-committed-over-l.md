---
id: 112
slug: pin-the-reverse-cross-layer-masking-for-the-committed-over-l
title: Complete the finalize.test_command cross-layer masking matrix (reverse committed-over-local + both skip-rung pairs)
status: implemented
priority: medium
created: 2026-07-20
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [106]
adrs: []
spec: docs/superpowers/specs/2026-07-20-reverse-cross-layer-masking-matrix-design.md
plan: docs/superpowers/plans/2026-07-21-reverse-cross-layer-masking-matrix.md
results: docs/results/2026-07-21-pin-the-reverse-cross-layer-masking-for-the-committed-over-l-results.md
trivial: false
auto_groomable: true
branch: feat/pin-the-reverse-cross-layer-masking-for-the-committed-over-l
claimed_at: 2026-07-22T00:20:47Z
pr: https://github.com/danielhanold/docket/pull/118
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-reverse-cross-layer-masking-matrix-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-reverse-cross-layer-masking-matrix-design.md) |
| Plan | [2026-07-21-reverse-cross-layer-masking-matrix.md](https://github.com/danielhanold/docket/blob/feat/pin-the-reverse-cross-layer-masking-for-the-committed-over-l/docs/superpowers/plans/2026-07-21-reverse-cross-layer-masking-matrix.md) |
| Results | [2026-07-21-pin-the-reverse-cross-layer-masking-for-the-committed-over-l-results.md](https://github.com/danielhanold/docket/blob/feat/pin-the-reverse-cross-layer-masking-for-the-committed-over-l/docs/results/2026-07-21-pin-the-reverse-cross-layer-masking-for-the-committed-over-l-results.md) |
| PR | [#118](https://github.com/danielhanold/docket/pull/118) |
<!-- docket:artifacts:end -->

## Why

Change 0106 pinned the `finalize.test_command` `auto` sentinel's cross-layer masking with three
fixtures in section S of `tests/test_docket_config.sh`: two forward cases (`s4`, `s5`) and one
reverse case (`s6`). The whole-branch review of that change found the reverse direction is proven
for only **one** of the two lower rungs.

`s6` covers *(lower = global `auto`, higher = committed real command)*. The pair
*(lower = committed `auto`, higher = local `.docket.local.yml` real command)* is untested — a gap on
the **precedence** axis.

The concrete refactor that would slip through: a committed-rung-specific clear appended after the
resolution chain, e.g.

```sh
[ "$(yaml_get "$CFG" test_command)" = auto ] && FINALIZE_TEST_COMMAND=""
```

Traced against all five of 0106's asserts, every one stays **green**:

- `s4` — the chain yields `auto` from the local rung; the committed rung holds `make test`, so the
  extra clear does not fire; `:202` collapses as normal. Green.
- `s5` — the committed rung holds `auto`, so the extra clear fires on a value that is already `""`.
  Green.
- `s6` — the committed rung holds `make test`, so the extra clear does not fire. Green.

Yet a real repo with `.docket.local.yml` setting `test_command: make local-test` over a committed
`test_command: auto` would **silently lose its local command** — finalize would fall back to
auto-detection with the whole suite green. That is exactly the class 0106 exists to prevent, one
rung over.

Note that 0106's spec explicitly declined a *forward* skip-rung case on the grounds that it "reuses
the two helpers `s4` and `s5` already cover and adds no distinct code path" — a claim about
**helpers**. That rationale does not reach *this* gap, which is about **precedence**: the mutation
above slips past all five existing asserts. It does, however, still hold for the skip-rung case
itself, and grooming confirmed it by mutation — see the spec.

## What changes

Complete the masking matrix. Three fixtures join `s4`/`s5`/`s6` in section S of
`tests/test_docket_config.sh`, following their established shape (per-fixture repo and `.xdg` root
via the `rung` helper, the `FINALIZE_TEST_COMMAND=__poison__` prelude before each `eval`):

- **`s7`** — reverse, committed `auto` under a local real command. Assert `make local-test`
  survives. This is the fixture the change exists for; it is the only one the
  committed-rung-specific clear reddens.
- **`s8`** — reverse, skip-rung: global `auto` under a local real command, committed key absent.
- **`s9`** — forward, skip-rung: local `auto` over a global real command, committed key absent.
  Expects an empty export, so it carries a control assert first.

Test-only, plus the section's comment header. Three mutation runs gate it — per-layer collapse,
blanket any-rung scan, and the committed-rung-specific clear — with a predicted redden/green cell
for every fixture, and the `ok` count read as part of the contract per the repo's
`guards-are-code` rule. `s7` is earned on unique discriminating power; `s8`/`s9` are
matrix-completeness witnesses that share the two mutations 0106 already used. The spec carries the
full table and the reasoning for each.

## Out of scope

- Any change to the sentinel's semantics or the placement of the collapse — like 0106, this pins
  current behavior.
- Re-running 0106's own two mutations as recorded in its results file; they are re-pointed at the
  new asserts only.
- A table-driven rewrite of section S, and any edit to 0106's archived record.

## Reconcile log

### 2026-07-21 — reconciled against `origin/main` @ `62a881b`

Scope holds unchanged: three test-only fixtures (`s7`/`s8`/`s9`) plus the section-S comment header.
Verified against current code rather than the design-time snapshot.

**Confirmed.** Section S of `tests/test_docket_config.sh` is present in exactly the shape the spec
assumes — `s4` (`:1043`), `s5` (`:1062`), `s6` (`:1087`), all six asserts intact, and the
`FINALIZE_TEST_COMMAND=__poison__` prelude before every `eval`. The helpers the design depends on
(`assert`, `mkrepo`, `run`, `rung`) are unchanged, and `assert` still emits `ok - <name>`, which is
what makes the `ok`-count read in the mutation protocol executable. The resolver still resolves
through a flat three-rung `:-` chain with the `auto` collapse applied *after* it — the behavior
being pinned is unchanged, so A7's test-only posture stands.

**Drift absorbed — the anchors moved.** A8 predicted concurrent work on the resolver as the only
realistic drift, and it happened: change 0102 (`finalize.require_pr_approval` layer resolution)
merged on 2026-07-21, after this spec was authored, inserting its resolution above the collapse.
The chain moved `:194 → :195` and the collapse `:201 → :202`. The spec has been corrected in both
places. Note for the build: 0102's follow-up commit `43b1aca` **already** re-anchored the section-S
header comment to the new numbers, so the header in the working tree is correct as it stands — the
edit here retitles it from `(S4/S5/S6)` to span `S4`–`S9` and must leave the `:202`/`:195`
citations alone rather than "restoring" the spec's old values.

**Adjacent, deliberately not folded in.** Change 0114 (`proposed`, build-ready) is the repo's
open question on whether line-number comment anchors are a supportable convention at all — this
reconcile is a live instance of exactly the fragility it exists to weigh, but 0114 is undecided,
so this change follows the established convention (cite the line numbers, as `s4`–`s6` already do)
and does not preempt it. ADR-0052 (Accepted, from 0102) was read and reinforces rather than
disturbs the premise: `finalize.test_command` is a `resolved:FINALIZE_TEST_COMMAND` key whose
cross-layer behavior is precisely what these fixtures pin.

No scope dropped, no new constraints folded in, nothing minted — the one gap found was internal
drift, which belongs in this log rather than in a new stub.
