---
id: 513
slug: 'release-v1-0-0-alpha-3-prove-and-publish-opencode-support'
title: 'Full OpenCode support after v1.0.0: prove and publish the OpenCode harness'
status: 'proposed'
priority: 'high'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-08'
depends_on: [544]
stacked_on:
related: [366, 511, 512, 544]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
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

The Go pre-releases were planned one harness at a time: Claude Code in alpha.1 (change 0366), Cursor in alpha.2 (change 0512), then OpenCode in an alpha.3.

On 2026-10-08 the human moved OpenCode out of the `v1.0.0` path. The release candidate (change 0544) and `v1.0.0` ship the OpenCode files marked **untested**, not unsupported. Full, human-tested OpenCode support lands in a release after `v1.0.0`.

Until then, OpenCode users have no tested release and no upgrade steps for their OpenCode files.

## What changes

- **Upgrade guide.** Add the OpenCode section to the Bash upgrade guide from change 0511, and add the OpenCode assertions to its test. The saved v0.9.2 and v0.9.3 cases already hold the OpenCode files.
- **Human test.** Run the full lifecycle in a fresh OpenCode process, as 0366 does for Claude Code.
- **Release.** Package, verify and publish the first post-`v1.0.0` release that claims OpenCode as tested, with the same lean protocol the alphas and the release candidate settled. Pick the version number at grooming.

## Out of scope

- Codex (officially unsupported).
- Re-proving Claude Code or Cursor beyond what the shared protocol already re-runs.
