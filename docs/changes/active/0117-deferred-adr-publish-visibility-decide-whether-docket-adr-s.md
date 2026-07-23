---
id: 117
slug: deferred-adr-publish-visibility-decide-whether-docket-adr-s
title: Deferred ADR-publish visibility — detect an unpublished ADR with a computed board-checks finding
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: [83, 118]
discovered_from: [83]
adrs: [51]
spec: docs/superpowers/specs/2026-07-21-unpublished-adr-check-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: feat
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-21-unpublished-adr-check-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-21-unpublished-adr-check-design.md) |
| ADRs | [ADR-0051](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0051-publish-deferred-marker-not-branch-diff-detector.md) |
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

**Detect, don't mark.** Add one computed health check — `adr-unpublished` — to
`scripts/board-checks.sh`, which runs on every `docket-status` pass. It compares the ADR set on
the metadata branch against the integration branch and reports what should have been published
but was not. No marker, no writer, no removal path.

- **Due rule.** An ADR is expected on the integration branch once its publish trigger has fired:
  a standalone `Accepted` ADR immediately; a change-tied ADR when its change reaches `done` or
  `killed`; and any ADR already present there must keep matching bytes, whatever its status.
- **Two arms**, both `git cat-file` against local branch refs — no network: **missing** (due but
  absent) and **stale** (present on both, bytes differ). One check-id, two messages, per the
  `stale-in-progress` precedent.
- **Gated** on `terminal_publish: true` AND docket-mode; `board-checks.sh` gains `--adrs-dir` and
  `--terminal-publish`, passed through by `docket-status.sh`.
- **Registered** in the four-site closed check-id vocabulary that `tests/test_board_checks.sh`
  pins.

The marker shape (extending `mark-publish-deferred.sh` to the ADR body) was considered and
rejected: it fires only if the failing run noticed it failed, it has no seam to hang on (ADRs are
never moved and are immutable once `Accepted`), and it cannot catch stale bytes from an
un-re-published status flip. Design rationale and the ADR-0051 boundary are in the spec.

## Out of scope

- A set-diff or audit over **change** records — #0083's decline stands; #0118 owns the adjacent
  skip-publish question.
- Any healer, re-publisher, or auto-fix. Report only.
- Publishing ADR-0023 (the one ADR absent from `main`) — its change #0044 is `blocked`, so under
  the due rule it is correctly absent and the check stays silent about it.
- Wiring `adr-checks.sh` into the `docket-status` health pass — considered and **declined**, not
  deferred (spec §4.1). It already runs under `docket-adr` on every ADR create and supersede,
  which is when its three checks could newly break. No stub minted.
- The `terminal_publish` knob's semantics, and the classifier / branch-protection / `--admin`
  policy — not docket's to change.

## Open questions

<!-- Resolved during grooming (2026-07-21). Reachability is settled: the ADR-only publish path
     is live and has failed twice. The immutability question is moot — the design writes no
     marker. "Decline and document" was weighed and rejected. One build-time call remains, in
     spec §5: which value the finding's <change-id> column carries for a standalone ADR, under
     ADR-0049's validated-values rule. -->

