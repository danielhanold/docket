---
id: 124
slug: backlog-triage-pass
title: Backlog triage pass — kill, defer, or arm each needs-brainstorm stub
status: killed
priority: medium
created: 2026-07-21
updated: 2026-07-26
depends_on: []
related: []
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
type: chore
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The needs-brainstorm queue is growing faster than grooming drains it. Measured 2026-07-21:
**17 stubs**, of which **6 were created that day** and **10 of 17 were auto-captured**
(`discovered_from:` set) rather than filed by hand. Four date to 2026-06-11 and have never been
touched.

Grooming them one at a time is the wrong instrument. Most of that queue does not need a design
conversation — it needs a *verdict*. Several stubs are plausibly dead (superseded by work that has
since merged, or duplicated by a sibling), and several others are mechanically obvious enough that
`docket-auto-groom` could carry them to build-ready with no human at all. Both outcomes are
cheaper than a grooming session, and both shrink the queue rather than merely advancing it.

`auto_groom` is `false` repo-wide, so today every stub waits on the maintainer personally. That is
the actual bottleneck, and it is a configuration choice rather than a fact.

## What changes

A single pass over every needs-brainstorm stub in `active/`, reaching one of three verdicts each:

- **Kill** — obsolete, superseded, or a duplicate. Drives the proposed-kill sub-path.
- **Defer** — right idea, wrong time. `status: deferred` plus a `## Why deferred`.
- **Arm** — mechanically clear enough to design without a human: commit `auto_groomable: true` so
  `docket-auto-groom` drains it.

Anything genuinely needing design judgment keeps its current state and stays in the human queue.

Input is `docket-status --digest-only` (shipped by #0094) — a write-free read that already emits
every stub with its status, readiness, and slug in selection order. No tooling is needed.

Arming is deliberate, not broad: `auto_groomable: true` must be **committed** before dispatch, and
arming N stubs drains all N in one autonomous run.

## Out of scope

- Changing the `auto_capture` knob or its materiality bar. The inflow rate is a real and separate
  question; this change addresses the standing queue, not the tap.
- Flipping `auto_groom` repo-wide. Per-stub arming is the deliberate unit here.
- Actually designing any stub. Arming hands that to `docket-auto-groom`; it does not do it.

## Open questions

- Which stubs are genuinely dead? The four dating to 2026-06-11 are the first candidates, but
  each needs a read against what has merged since.
- Is there a size threshold past which a repo-wide `auto_groom: true` beats per-stub arming?

## Notes

**This change is metadata-only and is NOT for `docket-implement-next`.** It produces no code and
no feature branch — its deliverable is a set of frontmatter and status edits on the metadata
branch. Sent through the autonomous implementer it would cut a `feat/` branch, find nothing to
build, and open an empty PR. It is deliberately left needs-brainstorm with `auto_groomable: false`
so that neither the implementer nor `docket-auto-groom` can pick it up; it is executed
interactively by a human, and reaches `killed` (not `done`) when the pass is complete.

Filed at the maintainer's explicit direction after the alternative — running the triage inline in
the grooming session that surfaced it — was offered and declined in favour of scheduling it.

## Why killed

Triage pass executed 2026-07-26. Killed on completion as the change specified — its deliverable was a set of verdicts, not code.

**Queue: 24 needs-brainstorm stubs in, 12 left for the human.** The 17 measured at filing had grown to 24.

**Method note worth keeping:** every verdict was checked against current code rather than read off the stub. That was not ceremony — **two stubs (0129, 0131) were already fixed** and would have been groomed and built otherwise. Both were fixed incidentally by change 0132 (`2e3789ca`) on 2026-07-22, the same day they were filed. A stub's own text is evidence about the past, not the present.

The inverse trap appeared too. **0130 looks fixed and is not:** its test passes on the maintainer's machine because PATH resolves `grep` to `ugrep`, which accepts the `{0,600}` bound that `/usr/bin/grep` rejects. Running the suite would have 'proven' the bug gone. Recorded in that stub so grooming does not repeat the mistake.

**Verdicts.**

- *Killed (2):* 0129, 0131 — already fixed, verified by grep and by running the suites.
- *Deferred (4):* 0007, 0008, 0009, 0010 — the 2026-06-11 competitive-review cohort. None obsolete, none with a forcing case. 0008 is half-shipped (0088) and gated on 0110; 0009's shape shipped twice as the auto-groom/finalize blocked markers; 0010 now sits against 0093's deliberate board-shrink and is flagged to kill on next review.
- *Armed (6):* 0018, 0119, 0120, 0122, 0126, 0130 — `auto_groomable: true`, committed before dispatch. 0018 was re-scoped first: the adopt-yq question is closed by conduct (ADR-0057/0058, centralized readers, zero yq invocations), leaving only its unmet 'if no, write the ADR' deliverable.
- *Human queue (12):* 0019, 0082, 0100, 0103, 0110, 0113, 0118, 0121, 0123, 0125, 0134, 0135. Four gained dated triage notes recording what changed under them — 0100 (narrowed by 0128; the residual is a security-posture reversal), 0103 (sibling 0102 shipped, so its 'groom as a sweep' framing is moot), 0125 (0114/ADR-0054 resolved the coordination it was waiting on), 0134 (named by ADR-0057 as its tracked follow-up).

**On the open question — is there a size threshold past which repo-wide `auto_groom: true` beats per-stub arming?** This pass says no, and the reason is not queue size. Arming was refused for 12 of 18 stubs on grounds a blanket flag cannot express: a live design fork (0121, 0123, 0134), a posture reversal needing a human (0100), or an unanswered empirical question (0118). Repo-wide arming would have sent all of those to a default-biased groom that should abstain on them — and abstain writes `auto_groomable: false`, so the queue would come back anyway, one wasted run later. Per-stub arming is not a scaling workaround; it is the judgment. Recommend keeping `auto_groom: false` and re-running this pass periodically instead.

**Two things this pass did not do.** The inflow tap (`auto_capture`) stayed out of scope as specified, and it is still the live question: 6 of the 24 stubs were filed within one day of the measurement, and 10 were auto-captured. Second, four stubs were annotated but not re-scoped where re-scoping is plainly warranted (0008 to fan-out-only, 0100 to its two residuals) — deferred to their own grooming so this pass did not quietly become a design session.
