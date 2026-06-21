---
id: 39
slug: trim-docket-status-archive-prose
title: Trim docket-status's residual archive-internals prose onto scripts/archive-change.md
status: done
priority: low
created: 2026-06-21
updated: 2026-06-21
depends_on: [37]
related: [36, 37]
adrs: []
spec:
plan: docs/superpowers/plans/2026-06-21-trim-docket-status-archive-prose.md
results:
trivial: true
auto_groomable:
branch: feat/trim-docket-status-archive-prose
pr: https://github.com/danielhanold/docket/pull/49
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-06-21-trim-docket-status-archive-prose.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-06-21-trim-docket-status-archive-prose.md) |
| PR | [#49](https://github.com/danielhanold/docket/pull/49) |
<!-- docket:artifacts:end -->

## Why

Change #37 gave every `scripts/<name>.sh` a co-located `scripts/<name>.md` contract and
stripped the script-*internals* prose out of the always-loaded skill bodies onto those
contracts — but it left `skills/docket-status/SKILL.md` untouched. That was deliberate: while
#37 was building, change #36 ("status sweep — delegate archiving to `archive-change.sh`")
merged and restructured the sweep close-out, so #37 rebased onto #36 and kept #36's freshly
reviewed docket-status wording rather than re-stripping it mid-flight (see #37's results +
reconcile log).

The consequence: docket-status's sweep **step c** still narrates what `archive-change.sh`
owns internally — the dated `active/ → archive/<merge-date>-<id>-<slug>.md` move with
reuse-existing-file idempotency, the `status: done` / `updated:` / `results:` writes, the
**change-file-only** commit, the push-with-rebase-retry on `origin/docket`, and the
fail-closed self-verification. That is now **duplicated** by `scripts/archive-change.md`
(authored in #37). Per #37's own §3 body↔contract boundary, those internals belong in the
contract; the body should keep only the operational facts. This change finishes the job #37
deliberately deferred, applying the same boundary docket-status's sibling bodies already got.

## What changes

Trim `docket-status`'s sweep **step c** (and any adjacent archive-internals narration) to the
**operational facts** the skill needs to act — which script, the exact args, "trust the exit
code" — and let `scripts/archive-change.md` carry the internals, reachable by the convention's
§2 naming rule (`$DOCKET_SCRIPTS_DIR/archive-change.md`). This mirrors exactly what #37 did to
`docket-finalize-change`, `docket-new-change`, and `docket-implement-next`.

**The crux — preserve #36's subtle semantics (do NOT disturb them):**

- **Step d's #0035 re-render-before-publish ordering** — the `## Artifacts` block is
  re-rendered on the archived file as a **separate follow-on commit that must land on
  `origin/docket` before `terminal-publish.sh` runs** (publish copies from `origin/docket`, so
  a stale block would otherwise ship). This ordering is operational and **stays**.
- **The per-change log-and-continue failure posture** — the sweep is a bulk best-effort
  janitor: on a non-zero exit from `archive-change.sh` / the re-render / `terminal-publish.sh`,
  it **logs and continues to the next change**, deliberately divergent from
  `docket-finalize-change`'s single-change `abort-and-report`. This posture is operational and
  **stays** — the trim must not rewrite it toward finalize's wording (the exact trap #37's
  Task-10 strip fell into before the rebase corrected it).

Net: a small body-only edit to one skill, guarded by the existing sweep sentinels
(`test_board_refresh_on_transition`, `test_learnings_ledger`, `test_results_artifact`,
`test_docket_metadata_branch`, `test_closeout`) plus whole-branch review for content fidelity.

## Out of scope

- **Any change to the sweep's behaviour or semantics** — #0035 ordering and the
  log-and-continue posture are preserved verbatim in meaning; this is prose relocation only.
- Rewriting `scripts/archive-change.md` (it already carries the internals from #37).
- The repo-root scripts and `scripts/lib/` — same exclusions #37 pinned.
- `docket-finalize-change`'s step-3 archive prose — that is single-change `abort-and-report`
  and was already handled in #37; leave it as-is.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-06-21 — reconcile (docket-implement-next)

Premise **confirmed** against current code (`origin/main` @ `77491d8`). `skills/docket-status/SKILL.md`
sweep **step c** still narrates `archive-change.sh`'s internals — the dated `active/ → archive/` move
with reuse-existing idempotency, the `status` / `updated` / `results` writes, the change-file-only
commit, the push-with-rebase-retry, and the fail-closed self-verification — every one of which is now
carried by `scripts/archive-change.md` (Purpose + Behavior §3/§5/§6/§7/§8/§9 + Invariants). The trim is
pure relocation; it loses nothing.

**Mirror target:** #37's already-trimmed `docket-finalize-change` step 3 — keep the invocation, the exact
args, "trust the exit code", and the one fact downstream steps rely on (it commits **the change file
only**), and defer the mechanics to `scripts/archive-change.md`. **Divergence preserved:** the sweep keeps
its own `non-zero ⇒ log-and-continue` posture, **not** finalize's `abort-and-report` (the #36 semantic, and
the exact trap #37's Task-10 strip hit before its rebase corrected it). Untouched by the trim: step d's
#0035 re-render-before-publish ordering, the per-change failure-posture paragraph (steps c–e), the
determinism invariant, and the "identical to finalize" note.

**Guardrails verified real:** the five sweep sentinels (`test_board_refresh_on_transition`,
`test_learnings_ledger`, `test_results_artifact`, `test_docket_metadata_branch`, `test_closeout`) exist;
no test asserts the trimmed prose tokens verbatim, so a markdown-only edit keeps them green. World unchanged
since drafting (same day) — no scope or spec adjustment needed; `trivial: true`, body-only. Proceeding to
plan.
