---
id: 182
slug: facade-tests-read-the-developer-s-real-global-config-instead
title: Facade tests read the developer's real global config instead of a sandbox
status: killed
priority: medium
type: fix
created: 2026-07-31
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [173]
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

`tests/test_runner_dispatch.sh` unsets `XDG_CONFIG_HOME` at the top for hermeticity, but the
facade resolves its global layer as
`${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml`. With
`XDG_CONFIG_HOME` unset and `DOCKET_HARNESS_ROOT` not passed, that falls through to `$HOME` —
so the pre-existing facade sections read the **developer's real**
`~/.config/docket/config.yml` during the test run.

Surfaced while building change 0173: the new value-class asserts had to pass
`DOCKET_HARNESS_ROOT="$SBX"` explicitly to stay hermetic, which made the omission in the
surrounding pre-existing sections visible. Today it is latent — those sections assert on
`runners.<name>` keys that a real global config is unlikely to set — but it is a test that
reads machine state it does not control, so it can pass or fail for reasons unrelated to the
code, and differently on CI than on a developer's laptop.

## What changes

- Pass `DOCKET_HARNESS_ROOT` (or pin `XDG_CONFIG_HOME` into the sandbox) in the pre-existing
  facade invocations in `tests/test_runner_dispatch.sh`.
- Sweep the other suites for the same fall-through: any invocation that unsets
  `XDG_CONFIG_HOME` without also pinning `DOCKET_HARNESS_ROOT` has the identical leak.
- Consider a shared fixture helper so the pairing cannot be forgotten again.

## Out of scope

- The value-class behavior itself (change 0173).

## Why killed

Consolidated into #0252 at the 2026-08-07 backlog triage: the hermeticity pin and cross-suite sweep land with the shared fixture helper that standardizes the pattern.
