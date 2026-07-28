---
id: 61
slug: detect-vs-mark-a-missing-terminal-record
title: Detect a missing terminal record where there is no marker seam; mark where the failure mode is a conscious human deferral
status: Accepted
date: 2026-07-27
supersedes: []
reverses: []
relates_to: [49, 51]
change: 117
---

## Context

docket now has two shapes for the same class of problem — "a terminal record that should be
sitting on the integration branch is not there" — and they disagree on method.

ADR-0051 (change 0083) chose **marker, not branch-diff detector** for terminal *change*
records: `mark-publish-deferred.sh` writes a `## Publish deferred` section into the archived
change file, and `board-checks.sh` reports its presence. That decision explicitly declined a
detector-*and*-healer, on two grounds. First, the realized gap it was closing was a **conscious
human deferral** — the maintainer knew the publish had not happened — and a healer would have
silently reversed a decision a human had just made. Second, the
*relax-the-policy-before-building-the-workaround* learning: do not build machinery to route
around a wall the maintainer owns.

Change 0117 builds the opposite shape for the **ADR corpus**: a computed, read-only
`adr-unpublished` check that compares the set of ADRs on the metadata branch against the set on
the integration branch and reports the difference. Two shapes now coexist for one problem class,
and without a stated boundary a future author copies whichever precedent they saw first.

`board-checks.sh`'s inline comment had also given a third reason for rejecting a set-diff: that
it would break the script's git-only/offline invariant. That reason does not survive scrutiny —
the script already runs `git cat-file -e <ref>:<path>` for its link checks, and a presence probe
against a local branch ref is the same shape and needs no network.

## Decision

**Detect where there is no marker seam and no healer; mark where a conscious human deferral is
the failure mode.**

This is a **narrowing of ADR-0051, not a reversal**. ADR-0051 stands unchanged for terminal
change records. Three grounds make the marker shape the wrong fit for ADRs:

1. **A marker only fires if the failing run noticed it failed.** The failure mode being closed
   for the ADR corpus is precisely that *nobody noticed*. A computed check needs nothing from
   the run that went wrong. It additionally catches stale bytes from an un-re-published
   `status:` flip — a case a marker structurally cannot reach, because there was no failure
   event to mark.
2. **There is no seam to hang a marker on.** An ADR file is never moved — there is no archive
   moment to write into — and an `Accepted` ADR is immutable except its `status:` line, so a
   body marker would bend the repo's own rule. The `change:` back-link is not a fallback either:
   a standalone ADR has none, and for a `status:` flip the producing change is long since
   archived and already published.
3. **Cost.** A marker needs a writer script, a removal path wired into `terminal-publish.sh`'s
   success branch, and check-id registration at several sites. The computed check needs none of
   it and self-heals by construction — the finding disappears when the ADR is published.

A **read-only report is not "routing around the wall."** It is the visibility fix the
relax-the-policy learning explicitly endorses: it heals nothing, publishes nothing, and reverses
no decision a human made.

ADR-0051's own Consequences named the residual this closes — "a terminal record that goes
missing via a path that writes NO marker … is still not caught. This is the accepted cost of
'mark, don't detect'." For the ADR corpus, where the marker seam does not exist at all, that
cost is not worth accepting.

## Consequences

- The check is **warn-only and report-only**. It never publishes, never edits an ADR, and never
  changes a status.
- It is **gated** — `terminal_publish: true` and docket-mode — so it stays silent on repos that
  deliberately keep their records on the metadata branch only.
- **`terminal_publish` is not retroactive.** A repo flipping the knob from `false` to `true` may
  see a burst of findings for ADRs that were legitimately never published. Measured at **zero**
  on this repo, so **no baseline or suppression mechanism was built** — a knob for a problem no
  repo currently has would be speculative surface.
- One correction to the record: the third reason in `board-checks.sh`'s comment (a set-diff
  breaks the git-only/offline invariant) **does not hold** and is retired here. The other two
  were real; the second — a check that fires forever under `terminal_publish: false` — is
  handled by the gate above, not by declining to compute.
- **The two shapes now coexist deliberately.** A future author facing "a record is missing"
  must ask which situation they are in — is there a seam, and is a human deferral the failure
  mode? — rather than copying whichever precedent they saw first. That question is the cost of
  this decision, and it is the intended cost.
