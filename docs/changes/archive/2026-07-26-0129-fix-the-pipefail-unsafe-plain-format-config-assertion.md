---
id: 129
slug: fix-the-pipefail-unsafe-plain-format-config-assertion
title: Fix the pipefail-unsafe plain-format config assertion
status: killed
priority: medium
created: 2026-07-22
updated: 2026-07-26
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

## Why killed

Already fixed by change 0132 (`2e3789ca`, "Install configured Bash 4+ runtime", merged 2026-07-22 — the same day this stub was filed from 0116's results).

The stub described `rung … | grep -q` running under `set -o pipefail`, where `grep -q`'s early exit can SIGPIPE the producer and surface as an intermittent exit 141. 0132 rewrote those assertions onto here-strings, which is precisely the remedy this change proposed ("capture the producer output first, then test the captured value with a here-string").

Verified 2026-07-26: `tests/test_docket_config.sh:1336,1342` now read `grep -q "^FINALIZE_REQUIRE_PR_APPROVAL=" <<<\"\$out7\"`, and a scan for any surviving producer-into-`grep -q` pipeline in that file returns nothing.

Killed rather than closed as done: no work was performed under this id, and the fix landed incidentally under another change's number. Recorded here so the coincidence is legible rather than looking like an unexplained disappearance.
