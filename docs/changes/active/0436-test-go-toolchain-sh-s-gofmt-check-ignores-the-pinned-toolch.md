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
related: [304, 317, 370, 373]
discovered_from: [434]
adrs: [50, 108]
spec: 'docs/superpowers/specs/2026-09-18-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-09-18-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-18-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch-design.md) |
| ADRs | [ADR-0050](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0050-backstop-checks-must-compute-not-reenumerate.md), [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

The existing formatting check uses gofmt from PATH, so a developer and CI can disagree about whether identical source is formatted. Read-only probes on main reproduced this: ambient Go 1.27.1 reports clean formatting, while the formatter shipped with go.mod's declared go1.26.5 toolchain flags internal/githubcli/comment_integration_test.go. Automatic Go toolchain selection retains a newer installed version; plain go env GOROOT therefore does not resolve this mismatch.

CI selects the Go 1.26 release family, not an exact 1.26.5 patch. Selecting the declared formatter explicitly within the shared check gives local and CI runs the same formatting rule without changing their broader toolchain behavior.

## What changes

- Extend Check 1 in tests/test_go_toolchain.sh to resolve and run the formatter belonging to the toolchain declared in go.mod, using Go's existing toolchain selection and cache machinery.
- Fail clearly on unusable toolchain resolution or formatter failure, preserving stderr diagnostics, module-derived package discovery, and the existing four-check reporting contract.
- Apply formatting-only corrections to the files reported by the corrected check; the current affected file is internal/githubcli/comment_integration_test.go.
- Add focused behavioral regression coverage and document a formatting remedy that derives the version from go.mod.

## Out of scope

No new configuration, toolchain service, helper framework, downloader, suite lane, retries, timeout or budget changes. Do not change go.mod's go/toolchain directives, CI's go-version setting, the toolchains used by the other Go checks, cache policy, or concurrency limits. The separate integration-shard registration issue on change 0368 remains outside this change.
