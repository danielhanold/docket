---
id: 83
slug: terminal-publish-gap-detection
title: A terminal record can silently never reach the integration branch — mark deferred publishes, stop the checker lying
status: proposed
priority: medium
created: 2026-07-16
updated: 2026-07-18
depends_on: []
related: [33, 43, 64, 95]
adrs: [1]
spec: docs/superpowers/specs/2026-07-18-terminal-publish-gap-detection-design.md
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-18-terminal-publish-gap-detection-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-18-terminal-publish-gap-detection-design.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md) |
<!-- docket:artifacts:end -->

## Why

Change **#0043**'s terminal record never reached `main`. It was killed on 2026-07-08
(`0ea9fd2`), archived correctly on `docket`, and the board was refreshed — but neither
the change file nor its `spec:` was ever published onto the integration branch. Nothing
noticed, and nothing healed it for the eight days until it was found by hand while killing
#0033 (an ad-hoc archive-set diff showed `main` at 69 records, `docket` at 71). No docket
health check caught it.

**Root cause (settled — see the spec).** The 2026-07-08 session survives: `terminal-publish.sh`
was **never executed**. The agent correctly planned the publish, then *deliberately deferred*
it pending approval — recommending `main` stay clean for a never-shipped proposal — and
**asked twice**. The reply *"43 is already published to main"* was not true; the agent
re-checked, re-asked, and the thread moved on unanswered. So this was a **conscious,
human-gated deferral that was never answered** — not a classifier denial, not a hand-skipped
step, not a kill-path bug (all three falsified in the spec). The record was legitimately
absent because the publish was legitimately deferred; what failed was **visibility** — the
deferral lived only in a chat thread, and `board-checks.sh` reported the tree clean while it
was pending.

The close-out sequence made that invisibility structural: the skip-publish guard runs
*forward* (a failed step 1 skips 2–3) but nothing runs backward — a deferred or blocked
step 3 leaves no marker in the change file, no state a later pass could read. And
`board-checks.sh` has **no** terminal-record check at all, so a pending deferral rides out a
"done/clean" report unseen.

## What changes

Two parts, per the 2026-07-18 groom decision (*mark the deferral, don't build a
branch-diff detector; and fix the checker that certified the gap clean*):

1. **A durable marker.** When a terminal close-out's publish step is *expected*
   (`terminal_publish: true`, docket-mode) but consciously deferred or blocked, append a
   dated `## Publish deferred` section to the change file — self-describing, at the change,
   mirroring `## Auto-groom blocked`. Written by a deterministic script on the defer path;
   **removed automatically when a later successful publish lands** (presence-encoded state).
   Never written when the publish is legitimately suppressed (`terminal_publish: false` or
   `main`-mode).
2. **Stop `board-checks.sh` lying.** Add a `publish-deferred` check that surfaces the marker
   as a finding, so a pending deferral can never again be certified clean. It reads the
   marker in the change file — git-only and offline, preserving the checker's invariant —
   **not** a branch-set diff.

Deliberately **not** built: a standing detector/healer that diffs the archived set on
`metadata_branch` against `integration_branch` and re-publishes what is missing. The
realized gap was a conscious deferral, not a fault to auto-heal, and building machinery to
route around a protected-`main` wall the maintainer controls is the exact anti-pattern the
`relax-the-policy-before-building-the-workaround` learning (from #0095) warns against. The
honest fix is to make the deferral *visible*, not to automate around the wall.

## Out of scope

- **The classifier / branch protection / `--admin` policy** on `danielhanold/docket` — the
  wall is not docket's to change; this change is about surviving the deferral visibly.
- **A branch-diff audit or a healer** over the full archive set — declined per the groom
  decision (see spec §1a, §3.3). A general "every terminal record must be on `main`" audit,
  if ever wanted, is a separate change.
- **The `terminal_publish` knob's semantics** — consulted as a constraint, not redesigned.
- **The 2026-07-16 backfill (`c0d6c04`)** — noted in the spec as possibly having reversed a
  deliberate choice; not reverted here.

## Open questions

<!-- Resolved during grooming; remaining build-time calls live in the spec's §6 (marker
     section name, writer-script boundary, reason-line format). -->

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
