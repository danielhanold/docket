---
id: 499
slug: 'a-cancelled-publish-s-git-push-or-gh-child-can-still-land-af'
title: 'A cancelled publish''s git push or gh child can still land after cancel'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: [494]
discovered_from: [494]
adrs: [137]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0137](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0137-the-publish-journal-blocks-only-on-a-publisher-that-may-stil.md) |
<!-- docket:artifacts:end -->

## Why

Change 0494 made the publish journal stop blocking once the publisher (`pr.publish` / `workspace.publish`) is provably gone. Killing the docket publisher process, though, does not kill the `git push` or `gh` child it started. That child can still push the branch or edit the PR after `run.cancel` has reported `cancelled`, so a cancelled run can still change the remote. 0494 documented this in the glossary and ADR-0137 but did not fix it in code.

## What changes

Make sure a publisher's external child processes (`git push`, `gh`) cannot outlive a cancel or a publisher death unnoticed. For example, run them in the publisher's process group so cancel's teardown reaches them, or have the abandonment check also confirm the children are gone. Then `cancelled` and `mutation-abandoned` mean no publish mutation can still land.

## Out of scope

Rolling back a mutation that already landed on the remote. Changing the journal's blocking rule from 0494/ADR-0137 except where needed to account for the children. Other operations' child processes outside the two publish operations.
