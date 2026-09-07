---
id: 406
slug: 'flaky-testintegrationreleasepackagedeterministic-linux-arm64'
title: 'Flaky TestIntegrationReleasePackageDeterministic — linux_arm64 bundle nondeterminism reddens the suite'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-04'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [317, 366]
discovered_from: [403]
adrs: []
spec: 'docs/superpowers/specs/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-design.md'
plan: 'docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/flaky-testintegrationreleasepackagedeterministic-linux-arm64'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T01:59:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-design.md) |
| Plan | [2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64.md) |
<!-- docket:artifacts:end -->

## Why

internal/release's TestIntegrationReleasePackageDeterministic builds the release bundle twice and compares checksums.txt; the docket_v0.0.1-*_linux_arm64.tar.gz checksum intermittently differs between the two builds of the same source while the other three tuples (darwin_amd64/arm64, linux_amd64) and install.sh match. Observed live during change 0403's build gate (2026-09-03/04): it reddened the full suite on one gate run, then passed 3/3 on serial isolation re-runs — a nondeterminism flake, not a diff-related regression. Because the full suite runs at both the build gate and the finalize gate, this flake intermittently fails otherwise-green runs of unrelated changes, costing re-gates and eroding trust in the suite's red signal.

## What changes

Root-cause the nondeterminism in the linux_arm64 release tarball build and make the bundle byte-deterministic across repeated builds of identical source, so the two-build checksum comparison is stable. Likely suspects to investigate: gzip/tar embedding of mtimes or a non-fixed modification time, file-ordering nondeterminism in the arm64 packaging path, or build-metadata (paths, timestamps) leaking into the arm64 artifact but not the others. Fix at the packaging source so the determinism test passes reliably rather than weakening or skipping the test.

## Out of scope

Relaxing, skipping, or quarantining TestIntegrationReleasePackageDeterministic (the test is correct — the artifact is nondeterministic). Changes to the other three target tuples' packaging beyond what a shared fix requires. Suite-runner or gate changes to tolerate flakes generally. Any change to change 0403's diff (config diagnostics, internal/app) — this is unrelated follow-up.

## Reconcile log

### 2026-09-07

2026-09-06: Reconciled against current source at origin/main effc9a6d. internal/release/archive.go still pins every tar/gzip metadata field (ModTime=epoch, gzip OS=0xFF, empty Name/uname/gname, fixed mode/uid/gid), and internal/release/package.go builds every tuple through one loop with CGO_ENABLED=0, -trimpath, cleared GOFLAGS, and a fixed injected ldflags identity — matching the spec's stated current implementation. TestIntegrationReleasePackageDeterministic (internal/release/package_integration_test.go) is unchanged: two Package calls, byte-compare of checksums.txt. No dependency, relation, or scope change is required; related [317,366] and discovered_from [403] remain context-only, no depends_on, no stacked_on. The archive layer being fully pinned confirms the spec's framing that the differing bytes must originate in the compiled linux_arm64 binary or its packaging inputs rather than in archive metadata — the investigation-and-repair scope stands as groomed.
