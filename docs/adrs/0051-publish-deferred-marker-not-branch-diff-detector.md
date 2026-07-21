---
id: 51
slug: publish-deferred-marker-not-branch-diff-detector
title: Make a deferred terminal publish visible with a presence-encoded marker, not a branch-diff detector
status: Accepted
date: 2026-07-21
supersedes: []
reverses: []
relates_to: [1]
change: 83
---

## Context

docket publishes a change's archived terminal record from the `docket` metadata branch onto the
integration branch at close-out. In a real incident (change #0043, 2026-07-08) that publish was
consciously DEFERRED pending a human approval that never arrived. The record stayed on `docket`
only, and nothing noticed for eight days — `board-checks.sh` had no terminal-record check at all
and reported the tree clean the entire time. The deferral existed only as a line in a chat
thread. Investigation established the publish script was never executed: the agent planned it,
recommended against publishing a never-shipped proposal, asked twice, and the thread moved on.
The failure was not a broken mechanism — it was a **visibility** failure around a legitimate
human-gated decision.

## Decision

Make the deferral durable and legible at the change file, rather than detecting the gap by
comparing branches:

1. A `## Publish deferred` body section — presence-encoded state — written by a dedicated
   deterministic script (`mark-publish-deferred.sh`), replacing rather than appending on a
   re-mark, and removed automatically by `terminal-publish.sh` on a successful publish (so a
   later backfill self-heals for free). Never written when the publish is legitimately
   suppressed (`terminal_publish: false`, or `main`-mode), where a skipped publish is success
   rather than a deferral.
2. A `publish-deferred` check in `board-checks.sh` that surfaces the marker as a finding, so a
   pending deferral can never again be certified clean. It reads the marker in the change file —
   preserving the checker's git-only/offline invariant.

Explicitly **declined**: a standing detector/healer that diffs the archived set on the metadata
branch against the integration branch and re-publishes what is missing. Two reasons:

- The realized gap was a **conscious human deferral, not a fault to auto-heal** — a healer would
  have reversed a choice the agent recommended and the maintainer never overrode.
- The direct publish push sits behind a **maintainer-controlled branch-protection wall**, and
  this repo has already spent three changes (0015/0021/0062) building machinery to route around
  that same class of wall when the real fix was one console setting — the
  relax-the-policy-before-building-the-workaround learning. Building a detector to survive a wall
  the maintainer owns is that anti-pattern again.

## Consequences

Gains: a pending deferral is visible where a human reads it; the checker stops lying;
presence-encoded state self-heals on the next successful publish; the offline/git-only invariant
of `board-checks.sh` is preserved; no machinery is added around a policy wall.

Costs / accepted residual: **the write side is a rule for drivers, not an enforced code path.**
The check reads a marker only a compliant driver writes, so a terminal record that goes missing
via a path that writes NO marker (a hard crash between archive and publish) is still not caught.
This is the accepted cost of "mark, don't detect"; a general "every terminal record must be on
the integration branch" audit remains a separate, deliberately-unbuilt change.

Also declined for now: wiring `docket-adr`'s own publish path (which sits behind the same wall)
into the marker — tracked as change #0117.
