---
id: 117
slug: deferred-adr-publish-visibility-decide-whether-docket-adr-s
title: Deferred ADR-publish visibility — decide whether docket-adr's publish path needs the Publish deferred marker
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

`docket-adr`'s publish-onto-integration path sits behind the same protected-`main` wall that
change #0083 addresses for terminal *change* records — but #0083 deliberately does not wire it
into the `## Publish deferred` marker.

#0083's spec calls the omission out explicitly (§5, "Notes carried from the investigation"):

> **`docket-adr`'s publish path** sits behind the same protected-`main` wall. It is *not* wired
> into the marker here (its records are ADRs, not change files, and it has no archive seam); if
> the maintainer wants deferred-ADR-publish visibility too, that is a follow-on. Called out so
> the omission is deliberate, not forgotten.

So after #0083 lands, a deferred or blocked **change** publish is durable and visible (a marker
at the change file plus a `publish-deferred` health-check finding), while a deferred or blocked
**ADR** publish is still invisible — it lives only in a chat thread, exactly the failure mode
#0043 demonstrated over eight days. The asymmetry is intentional but not obviously correct, and
it is the kind of gap that is cheap to close deliberately and expensive to rediscover.

## What changes

To be designed. The shape is genuinely open, and at least one honest outcome is "decline":

- ADRs have **no archive seam** — an ADR file is never moved, so there is no obvious per-record
  place to hang presence-encoded state the way an archived change file offers one.
- An ADR's frontmatter is **immutable once `Accepted`** except its `status:` line, which rules
  out a frontmatter field and makes a body marker awkward against that immutability rule.
- Candidate shapes to weigh: a body marker on the ADR (tension with immutability); the marker
  living on the **change** that produced the ADR (`change:` back-link) rather than the ADR
  itself; an `adr-checks.sh` finding computed some other way; or deciding the ADR case is rare
  enough that the change-record marker plus the existing loud `terminal-publish.sh` warning
  suffices.

## Out of scope

- Re-opening #0083's declined branch-diff detector/healer, or the `terminal_publish` knob's
  semantics.
- The classifier / branch-protection / `--admin` policy itself — not docket's to change.

## Open questions

- Is a deferred ADR publish actually reachable in practice, given `terminal-publish.sh`'s ADR
  mode is driven by `docket-adr` rather than an interactive close-out where a human defers?
- Does the ADR immutability rule permit a generated body marker, or does that force the marker
  onto the producing change file?
- Is the honest answer "decline and document", mirroring #0083's own refusal to build machinery
  around a maintainer-owned wall?
