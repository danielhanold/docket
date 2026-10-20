---
id: 575
slug: 'retire-claim-reclaim-and-its-lease'
title: 'Retire claim reclaim and its lease'
status: 'proposed'
priority: 'medium'
type: 'refactor'
deliverable: 'pr'
created: '2026-10-20'
updated: '2026-10-20'
depends_on: [360]
stacked_on:
related: [89, 316, 368, 268]
discovered_from: [360]
adrs: []
spec:
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Claim reclaim returns an abandoned `in-progress` change to `proposed` when three things hold: its `claimed_at` is older than `reclaim.lease_ttl` (72h by default), no feature branch exists locally or remotely, and no live workspace or gate run exists. It runs when typed (`docket change reclaim`), or from the maintenance sweep when `reclaim.auto: true` (off by default). It arrived with #89 (2026-07-17), borrowed from a competitor's lease-and-reclaim feature to self-heal crashed implement-next runs, and was ported to Go in #316.

It has never fired. Traced while grooming #360 (2026-10-20):

- No change record has ever carried a `## Reclaim log`: not in docket since #89, and not in the six other local repositories using docket. Every match is a spec, a doc, or a test fixture.
- No saved session ever ran a real `docket change reclaim` (transcripts go back to 2026-09-18).
- `reclaim.auto` is set nowhere: repo, local, or global config.

Its one window is tiny and already covered. On the building clone, the local branch blocks reclaim from `workspace.prepare` onward, minutes after the claim. A run that dies in those minutes is recovered by resuming the change by id (which works without a workspace since #368), with the run tracker's verdict driving any retry.

From another clone it is a risk rather than a help. A live run's branch is invisible there until publish, so a reclaim after the TTL could hand back a change that is still being built (#360 records this as an accepted risk). The guide also describes it wrongly: it says "crashed before pushing a branch", but reclaim needs no local branch either.

Removing it simplifies docket. One lifecycle edge goes, along with one operation, a sweep pass, two config keys, a body section, and their docs and tests.

## What changes

Retire claim reclaim end to end (the design is settled at grooming):

- Remove `change.reclaim` (CLI command, capability-catalog entry, schema), the maintenance sweep's reclaim pass with its reasons and findings, and the domain reclaim and lease-evaluation code only reclaim uses.
- Remove the `in-progress → proposed` reclaim edge, the *Reclaim edge* rule, and the `## Reclaim log` body section from the convention and the lifecycle docs.
- Retire the `reclaim.lease_ttl` and `reclaim.auto` config keys, so existing config files that still carry them keep loading.
- Update the guides (`keeping-the-backlog-honest.md`, `capturing-work.md`, and the install/config docs) so a stranded run points only at resume and the run tracker.
- Decide what happens to `claimed_at`, which would then have no reader.

## Out of scope

- Resume, `change.resume-halted`, the run tracker, and `## Run halted`: these are the recovery paths that stay.
- #360's own edits (deleting `change.refresh-claim`, and the claim-time wording in implement-next).
- `docket install collect`'s unrelated use of the word "reclaim".
- Rewriting archived records, specs, plans, or results that mention reclaim.
