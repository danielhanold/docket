---
id: 106
slug: pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma
title: Pin the finalize.test_command auto sentinel's cross-layer masking with a two-layer fixture
status: in-progress
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: [103]
discovered_from: [101]
adrs: []
spec: docs/superpowers/specs/2026-07-20-auto-sentinel-cross-layer-masking-design.md
plan: docs/superpowers/plans/2026-07-20-auto-sentinel-cross-layer-masking.md
results:
trivial: false
auto_groomable:
branch: feat/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma
claimed_at: 2026-07-20T13:35:50Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-auto-sentinel-cross-layer-masking-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-auto-sentinel-cross-layer-masking-design.md) |
| Plan | [2026-07-20-auto-sentinel-cross-layer-masking.md](https://github.com/danielhanold/docket/blob/feat/pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma/docs/superpowers/plans/2026-07-20-auto-sentinel-cross-layer-masking.md) |
<!-- docket:artifacts:end -->

## Why

Change 0101 introduced the `auto` sentinel (≡ unset) for `finalize.test_command` so the shipped
default can be stated explicitly in `.docket.yml.example` rather than left as a blank key. Its
correctness rests on *where* the sentinel is collapsed to "unset": after layer resolution, not
during it. That placement is what buys the actually-useful property — a **higher** layer writing
`test_command: auto` masks a **lower** layer's real command, exactly as an explicit re-statement of
the default should.

`tests/test_docket_config.sh` section S covers the sentinel in three single-layer fixtures — each
writes only a committed `.docket.yml`, so no fixture ever has two rungs populated at once. The
cross-layer property is asserted by the comment at `scripts/docket-config.sh:195-199` and nowhere
else. A comment is a claim, not a guard (the repo's `verify-the-claim` and `guards-are-code`
findings both name this shape), and this particular comment has already been wrong once: `a9da1e2`
shipped it describing the property backwards, and only the 0101 review caught it (`dab12b0`). A
future refactor that collapses the sentinel per-layer instead of after the chain would silently
invert the masking behavior with the whole suite green.

## What changes

Add three fixtures to section S of `tests/test_docket_config.sh`, using the existing `mkrepo` and
`rung` helpers (`rung` roots the global layer at a per-fixture temp dir, so every case stays
hermetic):

- **Forward, `lcl()` path** — committed `.docket.yml` sets a real command, `.docket.local.yml` sets
  `auto`; assert the resolved export is empty.
- **Forward, `gbl()` path** — global `config.yml` sets a real command, committed `.docket.yml` sets
  `auto`; assert the resolved export is empty.
- **Reverse** — committed `.docket.yml` sets a real command, global `config.yml` sets `auto`; assert
  the real command survives.

Each direction gets its own mutation run against the real `scripts/docket-config.sh`: collapsing the
sentinel per-layer must redden the two forward cases, and a blanket "any layer says `auto` ⇒ unset"
scan must redden the reverse case. The reverse case does not redden under the forward mutation,
which is why it needs its own — see the spec.

Also fix this change's own citation: the load-bearing anchors are `docket-config.sh:194` (the
resolution chain) and `:201` (the collapse), not `:197`, which lands mid-comment.

## Out of scope

- Any change to the sentinel's semantics or its resolution placement — this pins current behavior.
- Extending the `auto` sentinel to `github_project`, which is unwired end to end (change 0103).
- The third layer ordering (local `auto` over global real) — it reuses the two helpers the forward
  cases already cover and adds no distinct code path.

## Reconcile log

### 2026-07-20 — reconciled against `origin/main`, no scope change

Every load-bearing anchor in the change and its spec was re-verified against current
`origin/main`; all matched exactly, so the design stands as written and the scope is unchanged.

- **`scripts/docket-config.sh`** — `:194` is still the three-rung `lcl` → committed → `gbl`
  resolution chain for `finalize.test_command`; `:201` is still the post-chain collapse
  `[ "$FINALIZE_TEST_COMMAND" = auto ] && FINALIZE_TEST_COMMAND=""`; `:195-199` is still the
  comment that carries the cross-layer masking claim. The stub's `:197` citation does indeed land
  mid-comment — the correction stands.
- **`tests/test_docket_config.sh`** — section S still ends the file with exactly three fixtures
  (`$tmp/s`, `s2`, `s3`), each writing only a committed `.docket.yml`. Confirmed: no existing
  fixture populates two rungs for this key, so the cross-layer property is untested today.
- **Helpers** — `mkrepo` (`:13`) and `rung` (`:34`) are present as the spec describes, and the
  suite still pins `XDG_CONFIG_HOME` at a void (`:31`), so the new fixtures are hermetic. The
  global-layer write pattern (`mkdir -p "$tmp/<n>.xdg/docket"` + `config.yml`) and the
  machine-local pattern (`.docket.local.yml`) both have working precedents in sections K/L and
  0051-L2 respectively — L2 already resolves `finalize.test_command` from the local layer, so the
  new `s4` reuses a proven read path.

No work has been done elsewhere that overlaps this change; nothing was dropped or added. Related
change 0103 (`github_project` unwired) remains correctly out of scope.
