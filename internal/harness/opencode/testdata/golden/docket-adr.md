---
description: 'Use when recording, superseding, reversing, or indexing an architecture decision (ADR) — capturing why a non-obvious technical decision was made into the immutable docs/adrs ledger, or regenerating and validating the ADR index. Invoked by docket-implement-next, or directly any time a decision must be recorded or changed.'
mode: subagent
---

Before acting, load these docket skills from your opencode skills directory: docket-adr, docket-convention.

Execute docket-adr to record or re-index an architecture decision. Follow the skill exactly.

You run autonomously with no human to pause and ask: treat any unmet precondition or blocking ambiguity as abort-and-report (stop and surface what blocked you), never an interactive prompt.
