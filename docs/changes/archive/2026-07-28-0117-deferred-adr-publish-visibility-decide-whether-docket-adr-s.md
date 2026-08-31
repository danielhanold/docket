---
id: 117
slug: deferred-adr-publish-visibility-decide-whether-docket-adr-s
title: Deferred ADR-publish visibility — detect an unpublished ADR with a computed board-checks finding
status: done
priority: medium
created: 2026-07-21
updated: 2026-07-28
depends_on: []
related: [83, 118]
discovered_from: [83]
adrs: [51, 61]
spec: docs/superpowers/specs/2026-07-21-unpublished-adr-check-design.md
plan: docs/superpowers/plans/2026-07-27-unpublished-adr-check-plan.md
results: docs/results/2026-07-27-deferred-adr-publish-visibility-results.md
trivial: false
auto_groomable:
branch: feat/deferred-adr-publish-visibility-decide-whether-docket-adr-s
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/129
blocked_by:
reconciled: true
type: feat
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-21-unpublished-adr-check-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-21-unpublished-adr-check-design.md) |
| Plan | [2026-07-27-unpublished-adr-check-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-27-unpublished-adr-check-plan.md) |
| Results | [2026-07-27-deferred-adr-publish-visibility-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-27-deferred-adr-publish-visibility-results.md) |
| ADRs | [ADR-0051](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0051-publish-deferred-marker-not-branch-diff-detector.md), [ADR-0061](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0061-detect-vs-mark-a-missing-terminal-record.md) |
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
- Publishing the ADRs currently absent from `main` — ADR-0023 (change #0044, `blocked`) and
  ADR-0060 (change #0135, `implemented`, PR still open). Under the due rule both are correctly
  absent and the check stays silent about them.
- Wiring `adr-checks.sh` into the `docket-status` health pass — considered and **declined**, not
  deferred (spec §4.1). It already runs under `docket-adr` on every ADR create and supersede,
  which is when its three checks could newly break. No stub minted.
- The `terminal_publish` knob's semantics, and the classifier / branch-protection / `--admin`
  policy — not docket's to change.

## Reconcile log

### 2026-07-27 — claimed for build

Re-read against current `origin/docket` + `origin/main` and the current code. Design holds; no
scope change beyond the refreshed measurements below.

- **Ledger re-measured** (spec §1b was taken 2026-07-21 at 53/52). Today: **60 ADRs on `docket`,
  58 on `main`**; **zero** byte drift among the 58 present on both. **Two** ADRs are absent from
  `main`, not one — ADR-0023 (`change: 44`, `blocked`) and the newly-added **ADR-0060**
  (`change: 135`, `implemented`, PR open). Both are correctly not-due under §4.2's due rule, so
  the check still yields **zero findings** on this repo today. ADR-0060 is a *better* negative
  fixture than ADR-0023: it exercises the `implemented` (built, unmerged) shape rather than the
  `blocked` (never built) one, and the two together cover both non-terminal arms.
- **Registration surfaces confirmed** at their current shape: `BOARD_CHECK_IDS` in
  `scripts/lib/docket-frontmatter.sh` (12 entries), `board-checks.sh`'s `check-id ∈ {…}` header,
  `scripts/board-checks.md`'s `**`<id>`**` sections, `scripts/docket-status.md`'s single
  `check <check-id>` row. Spec §4.5 asked whether change #0111's guard covers a new id for free —
  **it does not, entirely**: the set-compares are enumerated and cover it, but
  `tests/test_board_checks.sh` also carries a hardcoded `[ "${#BOARD_CHECK_IDS[@]}" = 12 ]`
  count assert that must be bumped to 13. That is the one hand-edit the guard does not absorb.
- **Call site** is `docket-status.sh`'s `health_checks()`, which already passes
  `--changes-dir/--metadata-branch/--integration-branch/--lease-ttl-hours`; `--adrs-dir` and
  `--terminal-publish` join that list. Note `--integration-branch` is passed as
  `origin/$INTEGRATION_BRANCH` there while `--metadata-branch` is the bare branch name — the new
  check must use the args verbatim, as the existing link checks do.
- **§5 open question** (the `<change-id>` column) remains a build-time call; ADR-0049 read and
  its `?` precedent for an unusable id is the leading candidate, with the ADR reference carried
  in the message column. Settled during the build, with an ADR.

## Open questions

<!-- Resolved during grooming (2026-07-21). Reachability is settled: the ADR-only publish path
     is live and has failed twice. The immutability question is moot — the design writes no
     marker. "Decline and document" was weighed and rejected. One build-time call remains, in
     spec §5: which value the finding's <change-id> column carries for a standalone ADR, under
     ADR-0049's validated-values rule. -->

