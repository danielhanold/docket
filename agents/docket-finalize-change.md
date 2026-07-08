---
name: docket-finalize-change
description: Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change.
model: claude-sonnet-5
effort: medium
skills: [docket-finalize-change, docket-convention]
---
Execute docket-finalize-change to close out the change. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition (PR not actually approved, merge conflict, dirty worktree) or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
