---
id: 83
slug: terminal-publish-gap-detection
title: A terminal record can silently never reach the integration branch — mark deferred publishes, stop the checker lying
status: in-progress
priority: medium
created: 2026-07-16
updated: 2026-07-21
depends_on: []
related: [33, 43, 64, 95]
adrs: [1]
spec: docs/superpowers/specs/2026-07-18-terminal-publish-gap-detection-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/terminal-publish-gap-detection
claimed_at: 2026-07-21T01:14:05Z
pr:
blocked_by:
reconciled: true
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

<!-- Resolved during grooming; the spec's §6 build-time calls (marker section name,
     writer-script boundary, reason-line format) are settled in the reconcile log below. -->

## Reconcile log

### 2026-07-21 — reconciled against current `main`; spec §6 calls settled

**Verdict: build as designed.** Nothing in the design is invalidated. The gap is still real and
still unbuilt — `## Publish deferred`, `publish-deferred`, and `mark-publish-deferred` appear
nowhere in `scripts/`, `skills/`, or `tests/`. All five touch points named in spec §4 exist and
are unchanged in intent.

**What moved under the change since the spec (2026-07-18).** Change **#0104** landed on `main`
on 2026-07-20 and materially reshaped `scripts/board-checks.sh` — the file this change edits:

- Findings now go through `emit()`, which **sanitizes** TAB/CR in the change-id and message
  columns; the new check must use `emit`, never hand-built `FINDINGS+=`.
- A `cid` convention now governs the change-id column (validated integer id, else the
  filename-derived padded id) — the new check adopts it rather than emitting `$id` raw.
- The check-id vocabulary grew by two (`field-domain`, `board-row-dropped`), and 0104's own
  close-out **repaired pre-existing drift** across the enumeration sites. `publish-deferred` is
  therefore registered in *three* places, not one: `scripts/board-checks.sh`'s header set,
  `scripts/board-checks.md`'s *Check enumeration*, and `scripts/docket-status.md:344`'s closed
  `check <check-id>` set. Missing any one reproduces exactly the drift 0104 had to repair.
- 0104 also introduced the `DROPPED`/`EXPLAINED` suppression machinery. The new check is
  **orthogonal** to it: `## Publish deferred` is a body section, not a frontmatter field, and it
  cannot drop a board row — so the check neither marks `EXPLAINED` nor participates in
  `board-row-dropped`.

**Spec §6 open questions — settled (no scope change):**

1. **Section name: `## Publish deferred`** (the spec's own recommendation). Adopted.
2. **Writer boundary: shape 1** — a dedicated `scripts/mark-publish-deferred.sh` (+ `.md`
   contract) with add/remove modes and a `docket.sh` facade verb; `terminal-publish.sh`'s success
   path calls the remove. Adopted over folding a "don't publish, just mark" mode into
   `terminal-publish.sh`. Reinforced by current code: `docket.sh`'s `WRAPPED_OPS` allowlist is
   the permission inventory and takes a new op cleanly, whereas `terminal-publish.sh` already
   carries two no-op guards, two publish modes, and a CAS loop.
3. **Reason line: a short fixed prefix (`deferred` | `blocked`) plus free text**, matching
   `## Auto-groom blocked`.

**Two constraints the spec could not have known, folded in:**

- **This repo now runs `terminal_publish: true`** (resolved config at claim time; the spec was
  written when `false` was the live value). The marker path is therefore *live* here, not
  latent — the suppression carve-outs (`--enabled false`, `main`-mode) must be tested rather
  than assumed unreachable.
- **The `## Finalize blocked` precedent (change #0099) must NOT be applied by analogy.** 0099
  settled that a marker need not be stripped at archive *because every automated reader is scoped
  to a change short of `done`*. `## Publish deferred` inverts both halves: it is written **onto
  the archived file**, and its reader (the new check) is **archive-scoped**. So
  removal-on-successful-publish is genuinely load-bearing here — it is the presence-encoded-state
  rule discharged by its usual *means*, not waived. The build must not "simplify" it away citing
  0099.

**Detection pattern confirmed available.** `lib/docket-frontmatter.sh` already exposes
`has_section` (whole-line `grep -qxF`, deliberately anchored so prose *mentions* of a marker do
not read as state) and the `finalize_blocked()` helper built on it. A `publish_deferred()` twin
belongs beside it — a closer precedent than the `## Auto-groom blocked` parallel the spec cites.

**Noted, not a dependency.** Change **#0111** (*guard the board-checks check-id enumerations
against drift*, build-ready, unbuilt) is the standing guard for precisely the three-site
registration this change performs by hand. No `depends_on` added — 0083 does not need it, and
gating on it would stall a no-regret correctness win — but 0111's eventual guard should cover
`publish-deferred` for free, since it enumerates rather than hardcodes.

**Auto-capture.** One stub minted from spec §5's deliberately-called-out omission (deferred
**ADR**-publish visibility, `docket-adr`'s publish path behind the same protected-`main` wall) —
see the run report.
