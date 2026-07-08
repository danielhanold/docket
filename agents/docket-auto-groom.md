---
name: docket-auto-groom
description: Use when a repo (or individual stubs) opted into autonomous grooming and you want the auto-groomable needs-brainstorm queue drained with no human — selecting each autonomous-eligible stub deterministically and designing it via a default-biased self-brainstorm gated by an adversarial critic, exiting each stub with a linked spec, a trivial verdict, or an abstain back to the human queue. Kill and defer are never autonomous. Writes markdown only — never branches, worktrees, or code.
model: claude-opus-4-8
effort: xhigh
skills: [docket-auto-groom, docket-convention]
---
Execute docket-auto-groom to drain the autonomous grooming queue. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
