---
id: 83
slug: terminal-publish-gap-detection
title: A terminal record can silently never reach the integration branch — investigate #0043's gap and decide on detection
status: proposed
priority: medium
created: 2026-07-16
updated: 2026-07-16
depends_on: []
related: [33, 43, 64]
adrs: [1]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md) |
<!-- docket:artifacts:end -->

## Why

Change **#0043**'s terminal record never reached `main`. It was killed on 2026-07-08
(`0ea9fd2`), archived correctly on `docket`, and the board was refreshed — but neither
the change file nor its `spec:` was ever published onto the integration branch. Nothing
noticed, and nothing healed it for the eight days until it was found by hand.

> **Symptom repaired 2026-07-16, cause still unknown.** #0043's record (change file +
> spec) was backfilled onto `main` with human approval (`c0d6c04`), and the archive sets
> now match at 71/71. The repair does **not** close this change: it removes the *symptom*
> that made the gap visible while leaving the *mechanism* that produced it untouched and
> unexplained. Evidence is preserved in git history — the kill commit `0ea9fd2` on
> `docket` has no accompanying publish commit on `main` from 2026-07-08, which is the
> artifact the investigation reads.

It surfaced only incidentally, while killing #0033 on 2026-07-16: comparing the archive
sets across branches showed `main` at 69 records and `docket` at 71. The two-record gap
was #0033 (mid-close-out at the time) and #0043 (silently missing since 2026-07-08). A
whole-branch set comparison found it; no docket health check did.

**Why this is worth a look now:** during #0033's close-out, `terminal-publish` was
**denied by the auto-mode classifier** — it flags the direct push to `main` as a route
around the repo's required-review protection, and a docket-level instruction ("kill
change 33") does not clear it. Only explicit human approval does; an autonomous run has
no one to ask and hard-stops. So there is a *live, reproducible* mechanism by which the
publish step fails while every `docket`-side step (archive, artifacts re-render, board)
succeeds — leaving the backlog looking closed and correct while `main` quietly lacks the
record. #0043 may be exactly that failure, already realized once.

The close-out sequence's own posture makes the gap plausible rather than surprising: the
skip-publish guard runs *forward* (a failed step 1 skips 2–3), but nothing runs backward
— a failed step 3 leaves no marker in the change file, no retry queue, and no state a
later pass could detect. The `docket-status` sweep self-heals `done` transitions; it has
no notion of "archived here, never published there."

## What changes

Two parts, in order:

1. **Root-cause #0043 specifically.** Its record is already repaired (see the note in
   *Why*), so this is now purely an investigation — why did the publish never run? The
   answer decides part 2; it is not a foregone conclusion (see Open questions).
2. **Decide whether the gap warrants tooling** — a detector (a `docket-status` health
   check comparing the archived set on `metadata_branch` against the published set on
   `integration_branch`), a healer (re-publish what is missing), both, or neither if the
   right answer is that a human-approved step is *allowed* to be deferred and the
   real fix is only that it must be visible.

Whatever is chosen must respect `terminal_publish: false` (change 0064): in a repo that
deliberately suppresses the publish, every archived record is legitimately absent from
the integration branch, so a naive set-difference check would fire on every change,
forever.

## Out of scope

- **The classifier's behavior itself** — that it gates a direct push to a protected
  `main` is correct and not something docket gets to change. This change is about docket
  noticing and surviving the resulting gap, not about removing the gate.
- **Branch protection / the `--admin` bypass policy** on `danielhanold/docket`.
- The `terminal_publish` knob's semantics — consulted as a constraint, not redesigned.

## Open questions

- **What actually happened to #0043?** Candidate causes, none yet confirmed: (a) the
  classifier blocked the publish and the run stopped there; (b) the kill was driven
  by hand and the publish step was simply skipped; (c) a bug in the kill path at the
  time. Note (c) is *not* well supported by a "the kill path did not publish back then"
  theory — #0043 was killed 2026-07-08, but #0028 was killed 2026-06-20, **earlier**,
  and its record **is** on `main`. The kill-publish demonstrably worked before #0043.
  (Changes 0054/0055, which rewired the kill callers onto the shared close-out
  reference, landed 2026-07-11 — *after* #0043's kill, so #0043 ran the pre-rewiring
  path. Whether that path had a defect is unestablished.)
- Is #0043 the only gap, or the only one *found*? The set comparison that surfaced it
  was ad hoc; a first pass should check the full history, including whether `spec:` and
  ADR sub-records can go missing independently of their change file.
- Detector, healer, or both — and where does it live? A `docket-status` health check is
  the obvious home (it already reports stale claims and broken links), but the sweep is
  best-effort and log-and-continue; a check that can only ever say "a human must approve
  a push" may belong somewhere the human actually reads.
- Should a blocked publish leave a **durable marker** (frontmatter field, or a body
  section like `## Auto-groom blocked`) so the gap is self-describing at the change,
  rather than only derivable by diffing two branches? This is the cheapest fix and may
  make a detector unnecessary.
- Does this interact with the `docket-adr` publish path, which the same classifier
  blocks in the same way — one shared mechanism, or two?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
