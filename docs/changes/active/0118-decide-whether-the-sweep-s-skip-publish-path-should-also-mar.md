---
id: 118
slug: decide-whether-the-sweep-s-skip-publish-path-should-also-mar
title: Decide whether the sweep's skip-publish path should also mark an unpublished terminal record
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [83]
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

Change #0083 made a deferred terminal publish visible: a `## Publish deferred` marker written on
the defer path, a `publish-deferred` health check that surfaces it, and automatic removal on a
successful publish. As part of that work, `docket-status.sh`'s sweep learned to mark itself on the
`sweep-failed <id> terminal-publish` branch, so the highest-volume automated blocked-publish path
records its own gap instead of emitting an ephemeral report line.

One sibling path was deliberately left unmarked, and the reasoning is the weakest link in the set.

`scripts/docket-status.md:186-188` documents the **`render-change-links` skip-publish** path: when
the `## Artifacts` re-render fails to commit/push, the close-out's skip-publish guard fires and
`terminal-publish.sh` is never invoked. The stated rationale for not marking is that *nothing
published means nothing was deferred yet*.

From a **detection** standpoint that distinction does not survive contact with the problem #0083
exists to solve. The change ends up archived on the metadata branch with its terminal record never
copied onto the integration branch — which is byte-for-byte the #0043 state that went unnoticed for
eight days. Whether the publish was *deferred* or *never reached* is a distinction about cause, not
about visibility, and the whole premise of the marker is that visibility is the thing that failed.

The counter-argument is real and is why this is a change rather than a bug: the skip-publish guard
exists precisely so a STALE `## Artifacts` block is never published, so the correct remedy there may
be "re-render and retry", not "mark and move on". Marking might also fire noisily on a transient
push failure that the next sweep self-heals — the marker would then be written and cleared
repeatedly, and a marker that appears and disappears on its own trains a human to ignore it.

## What changes

To be designed. The decision is whether the skip-publish path should also mark, and if so how the
marker distinguishes itself from a genuine deferral. Candidate shapes:

- **Mark with a distinct reason.** `mark-publish-deferred.sh` already takes `--reason
  deferred|blocked`; a third value (or a distinct `--detail` prefix) could record "publish never
  attempted — artifacts re-render did not land", keeping the finding honest about its cause.
- **Mark only after N consecutive failures**, so a transient push failure that self-heals on the
  next sweep never surfaces — at the cost of state the sweep does not currently keep.
- **Leave it unmarked and fix the doc instead.** Replace the "nothing was deferred yet" rationale
  with the honest version: this path is a known, accepted detection gap, and say why it is
  acceptable (the next sweep retries the re-render, so the window is one pass).
- **Decline entirely**, on the same reasoning #0083 used to decline a standing branch-diff
  detector — this is a fault to fix at its source, not a gap to instrument.

Whichever is chosen, `scripts/docket-status.md:186-188` should end up stating the real reason.

## Out of scope

- Re-opening #0083's declined branch-diff detector/healer, or the `terminal_publish` knob.
- The `## Publish deferred` marker's format, writer, or removal semantics (settled in #0083,
  recorded in ADR-0051).
- The skip-publish guard itself — that a failed artifacts re-render must not publish a stale block
  is correct and not in question here.

## Open questions

- How often does the artifacts re-render actually fail to commit/push in practice? If it is
  vanishingly rare, "fix the doc" is likely the whole answer.
- Does a marker whose cause is "never attempted" rather than "deferred" belong under the same
  heading at all, or does sharing the heading dilute what a `publish-deferred` finding means?
- Is there any other close-out path that can leave a change archived-but-unpublished without
  writing a marker? A short audit of the close-out sequence's failure branches would answer it
  once rather than one path at a time.
