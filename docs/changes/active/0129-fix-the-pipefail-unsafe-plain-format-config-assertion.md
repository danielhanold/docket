---
id: 129
slug: fix-the-pipefail-unsafe-plain-format-config-assertion
title: Fix the pipefail-unsafe plain-format config assertion
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: []
discovered_from: [116]
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
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_docket_config.sh` asserts that `FINALIZE_REQUIRE_PR_APPROVAL` appears in plain-format
output with `rung … | grep -q` while the file runs under `set -o pipefail`. Once `grep -q` finds the
key it exits early, the config producer can receive SIGPIPE, and the otherwise-correct assertion
fails intermittently as exit 141. This violates the repository's promoted shell rule and currently
prevents a clean full-suite baseline for change 0116.

## What changes

Capture the producer output first, then test the captured value with a here-string. Mutation-test
the assertion so removing the exported key makes it red without reintroducing a pipefail-sensitive
producer/early-consumer pipeline.

## Out of scope

- Any change to `docket-config.sh` output or key ordering.
- Other configuration resolver behavior.

## Open questions

- None.
