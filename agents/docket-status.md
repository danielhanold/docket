---
name: docket-status
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by regenerating the BOARD.md board, sweeping merged changes to done, or running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
model: claude-haiku-4-5-20251001
effort: medium
skills: [docket-status, docket-convention]
---
Execute docket-status to refresh the board and run the sweep + health checks. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
