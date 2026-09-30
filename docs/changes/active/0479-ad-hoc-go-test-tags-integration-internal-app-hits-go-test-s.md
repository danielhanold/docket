---
id: 479
slug: 'ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s'
title: 'Ad hoc `go test -tags integration ./internal/app/` hits go test''s default 10-minute timeout'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-09-30'
updated: '2026-09-30'
depends_on: []
stacked_on:
related: [333, 434, 465]
discovered_from: [472]
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

During change 0472's build, the Task 3 integration gate ran `go test -tags integration ./internal/app/` and FAILED because the package exceeded go test's default 10-minute per-package timeout. The continuation worker had to run a filtered integration check with a 25-minute timeout to get a pass. Changes 0333 (partition internal/app), 0362 and 0465 (race-gate timeout) and 0434 (load-sensitive tests) each handled the timeout for another lane and ruled timeout increases out of scope, so nothing covers the integration-tag run of internal/app. Only tests/test_go_race.sh sets an explicit -timeout (its RACE_TIMEOUT backstop). It is not yet known whether the normal suite-runner gate for the integration lane is affected, or only ad hoc and gate-drive invocations.

## What changes

Trace how the integration lane of internal/app is invoked by the suite runner and by the gate drive, and determine whether any invocation relies on go test's default 10-minute timeout. If one does, either partition the integration-tagged internal/app tests so the package fits the default, or give those invocations an explicit -timeout following change 0465's backstop precedent. If only unsupported ad hoc runs are affected, document the required -timeout where the integration lane is described and close.

## Out of scope

Changes to the race gate's timeout (0465), weakening or skipping integration tests, and the closeout budget breach (0475).
