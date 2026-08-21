---
id: 334
slug: claude-md-global-and-project-copies-out-of-sync-on-finalize-a
title: CLAUDE.md global and project copies drift on finalize-agent descriptions
status: proposed
priority: medium
type: docs
created: 2026-08-21
updated: 2026-08-21
depends_on: []
stacked_on:
related: []
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

The global `~/.claude/CLAUDE.md` and the in-repo project `CLAUDE.md` carry divergent
descriptions for the finalize-related docket agents (`docket-integration-repair`,
`docket-rebase-resolver`). The global copy still has the OLD wording — e.g.
"reports an authored repair the dispatcher gates behind sign-off" and
"returns a structured report; never runs Git rebase mechanics" — while the project copy
has the NEW wording — "returns a structured repair report the sequencer gates" and
"reconciles each conflicted hunk … continues the rebase to completion".

Two copies of the same agent roster describing the same agents differently is a latent
correctness hazard: whichever copy a session loads shapes what the agent is understood to
do, and the two now disagree about the finalize merge-gate architecture. Discovered
2026-08-21 while re-installing docket agents after an ownership-conflict surfaced the same
old-vs-new split in the on-disk wrapper files.

## What changes

Reconcile the two CLAUDE.md copies to a single source of truth so the finalize-agent
descriptions agree, and establish how they stay in sync going forward (whoever owns the
generation path — likely `sync-agents.sh` / the install roster — should be the single
writer rather than two hand-maintained copies).

## Out of scope

- Rewriting the finalize agents themselves or their behavior.
- Any change to the merge-gate architecture the descriptions describe.

## Open questions

- Is the global CLAUDE.md meant to be generated/synced from the same roster the project
  copy comes from, or is it hand-maintained? The fix differs (regenerate vs. reconcile +
  guard against future drift).
- Which wording is canonical — is the "sequencer/controller" language the current design
  the global copy should adopt?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
