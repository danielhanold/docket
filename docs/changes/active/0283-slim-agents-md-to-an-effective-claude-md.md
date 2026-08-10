---
id: 283
slug: slim-agents-md-to-an-effective-claude-md
title: 'Slim AGENTS.md to an effective, lean always-in-context file'
status: proposed
priority: medium
type: docs
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: [263, 154]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

AGENTS.md is the always-in-context rules file, so every line of it taxes every session's context
window. Many of the learnings it carries have since been codified elsewhere — enforced by shell
scripts, tests (e.g. change 0263's guards over the shell rules), or docket skills — so restating
them in the always-loaded file buys nothing and dilutes the rules that genuinely must fire
unprompted. Claude Code's guidance on writing an effective CLAUDE.md
(https://code.claude.com/docs/en/best-practices#write-an-effective-claude-md) recommends keeping
the file lean: only universally applicable, non-derivable rules belong there.

## What changes

- Review every AGENTS.md section against the tiering criterion ("will the agent know to search for
  this?") AND against whether the rule is now mechanically enforced (test/guard/script) or lives in
  a skill that loads on demand.
- Remove or demote anything codified elsewhere; keep only rules that must fire unprompted and are
  not otherwise enforced or discoverable.
- Follow the linked best-practices guidance for structure and brevity.
- Demoted content that is still a useful war story returns to (or stays in)
  `docs/changes/learnings/`; nothing is simply deleted without a home unless it is obsolete.

## Out of scope

- Adding new rules or new guards; this is a pruning pass only.
- Changing the promotion mechanics or the learnings ledger itself.

## Open questions

- Which shell rules are now fully guard-enforced (0263) vs. still needing the prose backstop?
- Does the "Docket agents — dispatch, don't run inline" table and the "Run gate" section stay
  verbatim, get compressed, or move behind a pointer?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
