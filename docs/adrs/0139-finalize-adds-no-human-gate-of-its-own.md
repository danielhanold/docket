---
id: 139
slug: 'finalize-adds-no-human-gate-of-its-own'
title: 'Finalize adds no human gate of its own'
status: 'Accepted'
date: '2026-10-04'
supersedes: []
reverses: []
relates_to: [10, 11, 43, 8]
change: 515
---

## Context

ADR-0010 split finalize's gate into a rebase resolver and an integration repair agent and added a sign-off rule: a repair that turned the rebased suite green still waited for a human before merging, recorded as a `## Finalize blocked` note. In practice the sign-off became a stop that needed a human `finalize clear-block` for every repair, even in a single-maintainer repository with zero required approvals. The convention also documented that auto-detect finalize runs skip changes carrying `## Finalize blocked`, but the binary never implemented that skip, and its blocked-note code was never active. Repositories that want humans to review repairs already have a mechanism: GitHub branch protection with required approvals and dismissal of stale approvals on new commits. Change 515 retires the sign-off.

## Decision

Finalize adds no human gate of its own.

- A repair that turns the rebased suite green publishes and merges. The repair is recorded in the run report and in the archived record's closeout notes. This reverses ADR-0010's sign-off rule only. ADR-0010 stays Accepted, because its resolver/repair split stands, and gets a dated `## Update` note pointing here. ADR-0008's update that mentions the sign-off likewise gets a dated `## Update` pointer.
- When a repository requires PR approvals and dismisses stale approvals on new commits, the repair push removes the approval, and the merge waits for a fresh one (GitHub refuses the merge; with `finalize.require_pr_approval: true` auto-detect skips the PR as `approval-required`). Human review of repairs is repository policy set in GitHub, not a docket gate. That GitHub setting is off by default; with it off, the earlier approval stands and the repair merges like any other green change.
- `## Finalize blocked` is a visible note that never stops selection or merge. The documented rule that auto-detect skips marked changes is dropped rather than implemented. The binary's never-active blocked-note code is removed.

## Consequences

A green repair lands without a human round-trip, and the single-maintainer setup (zero required approvals, `require_pr_approval: false`) never stops for a repair. Teams that want repairs re-reviewed get it by turning on GitHub's stale-approval dismissal, so the policy lives in one place, the repository settings. Finalize keeps retrying marked changes, so transient failures (a flaky test, a busy worktree, a moved base) can heal on their own. Given up: a docket-enforced review of repair diffs in repositories that do not configure one in GitHub; the repair stays visible only through the run report, closeout notes, and the PR history. The `finalize clear-block` human step for repairs goes away.

## Alternatives considered

Keep the sign-off and make finalize honor the blocked note (the change's original premise): rejected, because it adds a human gate to every repair even where the repository requires no approvals, against the no-new-blocking-gates posture. Implement the documented auto-detect skip of marked changes: rejected, because skipping ends the retries that let transient failures heal, and the human already sees the note on the board. Re-request approval from docket itself after a repair: rejected, because GitHub's dismiss-stale-approvals setting already does this for repositories that want it, and a second mechanism would duplicate repository policy.
