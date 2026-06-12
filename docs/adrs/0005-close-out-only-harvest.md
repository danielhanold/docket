---
id: 5
slug: close-out-only-harvest
title: Learnings are harvested only at close-out — one writer, one moment, ledger unpublished
status: Accepted
date: 2026-06-12
supersedes: []
reverses: []
relates_to: [1, 3]
change: 6
---

## Context

Change 0006 added the learnings ledger (`<changes_dir>/LEARNINGS.md`) — build-loop lessons
carried across changes. The design space for *when* entries get written ranged from
write-anywhere (any skill appends a lesson the moment it learns one) to a single harvest moment.
Mid-build appends are tempting: lessons are freshest mid-build, and `docket-implement-next`
already writes metadata during a build. The counterweights: build-time findings already have a
durable home (the results file, change 0001), and a ledger that every skill can append to
degrades from curated memory into a chat log nobody reads. Separately: should the ledger publish
to the integration branch like ADRs and terminal records do?

## Decision

Entries are written **only by the harvest at close-out** — when a change reaches `done` — by a
procedure single-sourced in `docket-finalize-change` (step 2.5) and invoked by reference from
`docket-status`'s sweep (best-effort), with an entry-cites-`(#<id>` idempotency probe so the two
drivers never double-write. Zero entries for a change is normal. Kills are not harvested. The
ledger lives on `metadata_branch` only and is **never published to the integration branch**: it
is working memory for the loop, not a durable record of the code — ADRs hold those. Readers are
exactly the design and build moments: `docket-groom-next` before a brainstorm,
`docket-implement-next` at plan time and review.

## Consequences

- Lessons arrive once, distilled from the full close-out picture (PR review + merge gate +
  results), rather than dribbling in raw — the ledger stays short enough to actually be read.
- A lesson discovered mid-build waits in the results file until close-out; if a change never
  merges, its lessons die with it. Accepted — an unmerged change's lessons are unvalidated.
- The integration branch carries no trace of the ledger; a consumer reading only `main` sees
  ADRs and terminal records, not the loop's working memory. Accepted as the same split the
  board already has.
- If mid-build appends ever prove necessary, that is a new ADR superseding this one — the
  single-writer probe logic is the part that would need redesign.
