---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and reporting configuration, record, and artifact-link findings.
skills: [docket-status, docket-convention]
worktree-scope: metadata
---
Execute docket-status to refresh docket state, run the sweep, and report its findings. Follow the skill exactly. A thin report is the success case — do not go looking for artifacts the repo's configuration disables.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
