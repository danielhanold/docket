---
id: 526
slug: 'repository-init-writes-gate-off-and-configure-tests-then-ref'
title: 'repository init writes gate: off and configure-tests then refuses to set test commands'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 352, 374]
discovered_from: [366]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

On the alpha.1 acceptance fixture (change 0366), `docket repository init` ran on a repository whose only test was an executable `test.sh`. It wrote `.docket.yml` with `build.gate: "off"` and `finalize.gate: "off"` and no test commands. The release spec expected `init` to leave the policy pending so that `repository configure-tests` could fill it in. Instead, `configure-tests` returned `no-op (healthy): the test policy is already configured; nothing to write`, because it treats an explicit `off` as a decision already made. The docs describe `configure-tests` as the command that writes the build and finalize test commands. There was no command path to turn the gates on, so the operator edited `.docket.yml` by hand to set `gate: local` and `test_command: sh ./test.sh` for both gates.

## What changes

Give a freshly initialized repository a command path to local gates. Options:
- `init` leaves the policy pending, instead of `off`, when it can't infer a command;
- `configure-tests` can replace an `off` policy that `init` wrote (or takes a flag to);
- `configure-tests` takes the test command as input.

Grooming picks one. Check whether `init` should recognize a root `test.sh` at all. Align the docs with whatever is chosen.

## Out of scope

Inferring test commands for specific language ecosystems beyond what grooming settles. Changing gate semantics.
