---
id: 436
slug: 'test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch'
title: 'test_go_toolchain.sh''s gofmt check ignores the pinned toolchain, flip-flopping CI red'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-18'
updated: '2026-09-18'
depends_on: []
stacked_on:
related: []
discovered_from: [434]
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

tests/test_go_toolchain.sh:132 runs `gofmt -l $pkg_dirs` resolving `gofmt` from bare PATH, not from this module's pinned toolchain (go.mod: `go 1.26.0` / `toolchain go1.26.5`). CI's release-candidate.yml pins `go-version: '1.26'` via actions/setup-go, so CI's gofmt is 1.26's. A local Go newer than the toolchain line (e.g. 1.27.1) is NOT auto-downgraded by GOTOOLCHAIN=auto -- Go's toolchain directive is a floor, not an exact pin, so `go env GOROOT` on a machine with a newer system Go silently reports that newer Go's GOROOT, not the pinned 1.26.5 toolchain. Confirmed directly: go1.27's gofmt and go1.26.5's gofmt disagree on trailing-comment column alignment in internal/githubcli/comment_integration_test.go (a composite literal with per-element trailing comments) -- go1.26.5 wants heavy padding-aligned comments, go1.27 does not. Forcing `GOTOOLCHAIN=go1.26.5 go env GOROOT` and using that GOROOT's gofmt reproduces CI's exact failure locally. Net effect: whoever last gofmt's that file with a local Go newer than 1.26.5 makes it look clean locally while it stays red on CI's pinned 1.26 gate, and the reverse also holds -- a flip-flopping trap, not a one-off typo. This has been hitting `test_go_toolchain` across most recently observed open PRs (confirmed on change 434's PR #312, and PR #368's branch), not just one.

## What changes

Fix tests/test_go_toolchain.sh's Check 1 (the gofmt cleanliness check, around line 132) to resolve `gofmt` from this module's pinned toolchain rather than bare PATH -- e.g. read the `toolchain` line from go.mod and invoke `GOTOOLCHAIN=<that version> go env GOROOT` to locate the matching gofmt binary, so the check is deterministic regardless of the ambient system Go version and matches exactly what CI's pinned setup-go will see. Reformat internal/githubcli/comment_integration_test.go (and any other currently-drifted file the corrected check newly flags) with the pinned toolchain's gofmt so the suite is green under the fixed check.

## Out of scope

Do not change go.mod's `go`/`toolchain` lines or CI's pinned go-version -- this fixes the LOCAL check to match the existing pin, not the pin itself. Do not touch the unrelated test_go_integration_contract failure on PR #368's branch (three new integration tests there are missing a shard-runner registration) -- that is change 0368's own defect on its own branch, distinct from this toolchain-drift bug, and is being tracked/handled separately.
