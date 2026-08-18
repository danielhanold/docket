---
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
mode: subagent
---

Before acting, load these docket skills from your opencode skills directory: docket-status, docket-convention.

Execute docket-status to refresh docket state and run the sweep + health checks. Follow the skill exactly. A thin report is the success case — do not go looking for artifacts the repo's configuration disables.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
