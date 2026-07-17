---
id: 83
slug: terminal-publish-gap-detection
title: A terminal record can silently never reach the integration branch — investigate #0043's gap and decide on detection
status: proposed
priority: medium
created: 2026-07-16
updated: 2026-07-17
depends_on: []
related: [33, 43, 64]
adrs: [1]
spec:
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

## Auto-groom blocked

### 2026-07-17 — `docket-auto-groom` abstained (critic: needs human context)

**Part 1 is answered — and its answer is why part 2 cannot be auto-decided.** The root cause is
not any of the three candidates in *Open questions*, and it is not the classifier mechanism this
change's *Why* leans on. It is the one branch *What changes* reserved for you.

**What happened to #0043 (decisive evidence).** The kill session survives at
`~/.claude/projects/-Users-homer-dev-docket/01386d16-b488-4c1f-b76d-d4de7747717b.jsonl`:

- **01:34:42Z** — the agent wrote a **4-step** plan; step 4 was *"Terminal-publish 43's kill to
  `main` — this is outward-facing **and** would copy the abandoned tier-design spec onto the code
  line, so I'll **pause and confirm** before that push rather than assume it."*
- **01:37:44Z** — `archive-change.sh` ran (steps 1–3 completed on `docket`).
- **01:40:17Z** — the agent surfaced it: *"**One deferred step (your call):** by convention a kill
  also terminal-publishes to `main` … Since 43 was a never-shipped proposal, **I'd leave `main`
  clean** … Say the word if you'd rather publish it."*
- **12:20:54Z** — you replied *"43 is already published to main."* — which was **not true**.
- **12:24:41Z** — the agent re-checked, confirmed it absent, and **re-asked**: *"I deliberately
  didn't run the kill's terminal-publish (I'd asked you first) … Tell me if you specifically want
  the killed record published to main; otherwise it's done."* The thread moved on; it was never
  answered.

So: `terminal-publish.sh` was **never executed** (verified: 0 executions in that session) — not
because a step was dropped, but because it was **correctly planned, deliberately deferred pending
your approval, recommended against, and asked about twice.** Candidates (a) classifier denial and
(c) a kill-path bug are both **falsified** — the publish was never attempted, and the publish step
had been in `docket-new-change`'s proposed-kill path since `9d38434` (2026-06-19). A fourth
candidate not listed here — a stale local skill checkout (`~/.claude/skills/docket-*` symlinks
into the local `main` tree; #0028's kill record documents that biting once at 39 commits behind) —
is also falsified: local `main` fast-forwarded 12 minutes before the kill, and even the prior
stale tip postdated `9d38434`.

**Correcting this change's own reasoning** (for whoever grooms it): the *Open questions* argument
that (c) is unsupported "because #0028 was killed earlier and its record **is** on `main`" reaches
the right conclusion by invalid means — which code runs is set by which skill the operator invokes
and by local checkout freshness, not by date. Date-ordering across two differently-driven sessions
is not evidence about a code path.

**The undecidable decision.** *What changes* part 2 offers detector / healer / both / **neither —
"if the right answer is that a human-approved step is *allowed* to be deferred and the real fix is
only that it must be visible."** The evidence lands exactly there: this **was** a consciously
deferred, human-gated step. Choosing detector-vs-marker-vs-neither is therefore no longer a
technical default an agent can pick — it is a call about how your own unanswered approval should
be treated, and *defer* is never autonomous. A first-pass draft (detector-only, in
`board-checks.sh`) was rejected by the critic on exactly this ground: its rejection of the
marker rested on the false premise that the publish path never ran, when in fact
`archive-change.sh` **did** run and a marker written there would have fired.

**What we need from you — one question:** you were asked twice on 2026-07-08 whether to publish
#0043's kill to `main`, and the agent recommended *leaving `main` clean* for a never-shipped
proposal. **Do you want that class of deferral detected, marked, or simply accepted?**

**Recommendations (yours to decide, not this groom's):**

1. **A marker is now the strongest candidate**, not the weakest — the reverse of this change's
   guess that a detector might subsume it. The deferral was conscious; a durable marker at the
   change (written when a close-out step is deferred or blocked) makes it visible exactly where
   *Open questions* hoped, and `archive-change.sh` is a real seam that runs.
2. **One defect survives regardless of your answer, and is worth keeping in scope:**
   `board-checks.sh` was run at 01:39Z **with `--integration-branch main`** and reported **clean**
   while the record was absent from `main`. It did not merely fail to notice — it **certified the
   gap clean**, and that certification is what the "done" report at 01:40Z rested on. A checker
   that says clean when a terminal record is missing is wrong under *any* root cause. If you want
   a minimum viable change here, this is it.
3. **Re-examine the 2026-07-16 backfill (`c0d6c04`).** It published #0043's record to `main` as a
   "repair" — but on this evidence there was no failure to repair: the absence was the outcome the
   agent *recommended* and you never overrode. The backfill may have reversed a deliberate choice.
   That reframes this change's *Why* ("Symptom repaired 2026-07-16, cause still unknown") — the
   symptom may not have been a symptom.
4. **Gap census (2026-07-17, re-verified):** archived change files **74/74** across
   `docket`/`main`; **0** spec gaps across 67 specs; the only Accepted ADR absent from `main` is
   **0023** (`change: 44`, still `blocked`) — legitimately pending its change's close-out, not a
   gap. So #0043 was the only realized gap. Any detector must scope to *terminal* records and
   honor `terminal_publish`, or it fires on legitimately-pending records forever.

**Re-arm:** answer the question above, flip `auto_groomable: true`, and delete this section.
