---
id: 510
slug: 'match-reported-follow-ups-against-proposed-and-deferred-chan'
title: 'Match reported follow-ups against proposed and deferred changes'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [509]
stacked_on:
related: [302]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-match-reported-follow-ups-against-proposed-and-deferred-chan-design.md'
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
| Spec | [2026-10-04-match-reported-follow-ups-against-proposed-and-deferred-chan-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-match-reported-follow-ups-against-proposed-and-deferred-chan-design.md) |
<!-- docket:artifacts:end -->

## Why

When `docket-implement-next` finishes a change, it records issues it found but did not fix under the results file's `## Known issues and follow-ups` section and lists them again in its final report for a human to capture. The rule there says to "link an existing change when one is known", but nothing tells the run to look. It links one only when that change already happens to be in its context. Of the last dozen results files, two linked an existing change (0496 named #502, 0493 named #492) and the rest left every follow-up unlinked, so the human has to remember whether the backlog already covers each one before capturing it, and a missed match turns into a duplicate change.

The retired auto-mint dedup failed on the same problem from the other side (#302): it matched by title, and a change stating the general form of a problem rarely shares wording with a follow-up describing one instance of it. A useful check has to read what each candidate change is for, not just its title.

## What changes

At final results consolidation, implement-next checks every reported follow-up against the backlog's `proposed` and `deferred` changes and gives each one a verdict: it fits an existing change, it is related to one but needs its own change, or no existing change fits (with the number of changes checked). Each Known-issues entry and the final report's follow-up list carry that verdict and a next action matched to the target's state: edit a proposed change, revise a groomed change's spec through grooming, or revive a deferred change and then edit it.

The check only recommends. A human decides whether to fold the follow-up in.

## Out of scope

Editing, creating, reviving, or killing any change from the run; the check never writes outside the run's own results file and final report. Matching against `in-progress`, `blocked`, `implemented`, `stacked-merged`, or archived changes. finalize's late findings in closeout notes. Any new gate, halt, or operation: a failed backlog read degrades to today's wording with a note, and never stops the run. The general dedup redesign tracked in #302.
