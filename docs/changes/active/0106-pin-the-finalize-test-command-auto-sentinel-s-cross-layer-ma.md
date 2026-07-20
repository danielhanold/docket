---
id: 106
slug: pin-the-finalize-test-command-auto-sentinel-s-cross-layer-ma
title: Pin the finalize.test_command auto sentinel's cross-layer masking with a two-layer fixture
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [101]
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

Change 0101 introduced the `auto` sentinel (≡ unset) for `finalize.test_command` so the shipped
default can be stated explicitly in `.docket.yml.example` rather than left as a blank key. Its
correctness rests on *where* the sentinel is collapsed to "unset": after layer resolution, not
during it. That placement is what buys the actually-useful property — a **higher** layer writing
`test_command: auto` masks a **lower** layer's real command, exactly as an explicit re-statement of
the default should.

`tests/test_docket_config.sh` section S covers the committed layer in isolation, and
`scripts/docket-config.sh:197` carries a corrected comment describing the cross-layer behavior. No
fixture pins it. So the one property the placement decision was made *for* is asserted by a comment
only — and a comment is a claim, not a guard (the repo's `verify-the-claim` and `guards-are-code`
findings both name this shape). A future refactor that collapses the sentinel earlier — during the
per-field merge instead of after it — would silently invert the masking behavior with the whole
suite green.

## What changes

Add a two-layer fixture to `tests/test_docket_config.sh` that pins the masking property directly:
a lower layer (repo-committed `.docket.yml`) setting a real `finalize.test_command`, a higher layer
(`.docket.local.yml` or the global config) setting `test_command: auto`, and an assert that the
resolved export is the **unset default**, not the lower layer's command. Mutation-test it by
collapsing the sentinel before resolution and confirming the new assert reddens.

Worth checking in the same pass whether the reverse direction deserves a companion assert (a lower
layer's `auto` must NOT mask a higher layer's real command).

## Out of scope

- Any change to the sentinel's semantics or its resolution placement — this pins current behavior.
- Extending the `auto` sentinel to `github_project`, which is unwired end to end (see the separate
  follow-up change on that key).

## Open questions

- Which higher layer makes the sharper fixture: `.docket.local.yml` or the global config? The global
  layer exercises one more hop, but `.docket.local.yml` keeps the fixture hermetic without touching
  a real `~/.config` path — a hazard this repo has already been bitten by
  (`config-layer-write-and-read-hazards`).
