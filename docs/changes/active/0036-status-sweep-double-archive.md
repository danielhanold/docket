---
id: 36
slug: status-sweep-double-archive
title: docket-status sweep — delegate archiving to archive-change.sh (remove the double-archive)
status: in-progress
priority: low
created: 2026-06-21
updated: 2026-06-21
depends_on: [35]
related: [35]
adrs: []
spec: docs/superpowers/specs/2026-06-21-status-sweep-double-archive-design.md
plan: docs/superpowers/plans/2026-06-21-status-sweep-double-archive.md
results:
trivial: false
auto_groomable: true
branch: feat/status-sweep-double-archive
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-06-21-status-sweep-double-archive-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-status-sweep-double-archive-design.md) |
| Plan | [2026-06-21-status-sweep-double-archive.md](https://github.com/danielhanold/docket/blob/feat/status-sweep-double-archive/docs/superpowers/plans/2026-06-21-status-sweep-double-archive.md) |
<!-- docket:artifacts:end -->

## Why

`docket-status`'s merge sweep archives a merged change twice. Steps c–e do a **manual**
archive — `git mv active/ → archive/`, set `status: done` / `results:` / `updated:`, run the
link renderer, then commit the change-file-only and push. Step f then **re-invokes**
`scripts/archive-change.sh`, which does the same `git mv` + `status: done` + change-file-only
commit + push over again. The second pass is idempotent (it reuses the already-archived file
and no-ops when bytes match), so the sweep is *correct* today — but it is convoluted: two code
paths that must stay in lock-step describe one operation, and `archive-change.sh` already
exists precisely to be the single archive primitive (it was extracted in change 0026 to remove
hand-staging failure modes). `docket-finalize-change` already delegates its archive entirely to
the script; the sweep should too. This was surfaced by the change #0035 whole-branch review as
a pre-existing tidy (not introduced there).

## What changes

Make the sweep delegate archiving entirely to `archive-change.sh`, byte-aligned with
`docket-finalize-change`'s step 3 (a skill-body edit to `skills/docket-status/SKILL.md`; no
script changes). Settled design — full detail in the linked spec:

- Drop the manual `git mv` + field-edit + commit steps (c–e); let `archive-change.sh` own the
  dated move, the `status: done` / `results:` / `updated:` writes, the change-file-only commit,
  and the push-with-rebase-retry. A field-by-field diff confirms the script reproduces every
  manual behavior and adds fail-closed self-verification — nothing is silently dropped.
- Preserve the `## Artifacts` re-render that #0035 placed in the old step d, sequenced as
  finalize does it: a follow-on renderer commit **after** `archive-change.sh` returns and
  **before** terminal-publish, pushed to `origin/docket` first (terminal-publish copies from
  there; a stale block would otherwise publish — the #0035 footgun). A failed re-render skips
  publish.
- Keep the sweep's **best-effort failure posture** (log-and-continue per change, self-heal next
  sweep) — deliberately distinct from finalize step 3's abort-and-report, which fits a
  single-change close-out, not a bulk janitor.
- Keep the two "must not diverge" notes (one each in the `docket-status` and
  `docket-finalize-change` SKILL bodies) accurate once the sweep matches finalize.

## Out of scope

- `terminal-publish.sh` mechanics and the publish copy-set (unchanged).
- `docket-finalize-change`'s archive (already delegates to the script — only the sweep is being
  brought into line).
- The board pass, health checks, and learnings harvest (untouched).

## Open questions

Both resolved in the spec (groomed 2026-06-21):

- *Does collapsing to the script silently drop a manual-step behavior?* No — the spec's
  field-by-field diff against `archive-change.sh` shows it reproduces every behavior and adds
  fail-closed self-verification.
- *Where does the renderer re-render sit?* As a follow-on commit after `archive-change.sh` and
  before terminal-publish (pushed to `origin/docket` first), exactly as finalize step 3
  sequences it. #0035 is `done`, so that call already exists in both skills.

One presentation call is open at build time (recorded in the spec): whether the sweep
*references* finalize step 3 as the single source for the archive+render+publish sequence
(overriding only the failure posture) or byte-aligns the prose.

## Reconcile log

- 2026-06-21 — Reconciled against `origin/main` (the feature-branch base, advanced past local
  `main` by the #0034/PR-#45 merge) before planning. Findings:
  - **Premise holds.** `skills/docket-status/SKILL.md`'s merge sweep on `origin/main` still does
    the double-archive exactly as the spec describes: steps c–e hand-roll the archive (`git mv
    active/ → archive/`, set `status: done`/`results:`/`updated:`, re-render `## Artifacts`,
    commit change-file-only + push) and step f then re-invokes `archive-change.sh` +
    `terminal-publish.sh`. Nothing has pre-fixed this. Scope unchanged.
  - **Convergence target confirmed.** `docket-finalize-change`'s step 3 (on `origin/main`) is the
    byte-confirmed model: `archive-change.sh` → renderer follow-on commit+push (before publish) →
    terminal-publish; its failure posture is `non-zero ⇒ abort-and-report`. The sweep must adopt
    the same *sequence* but keep its own **log-and-continue** posture (spec A6) — a genuine,
    deliberate divergence.
  - **Dependency satisfied.** #0035 is `done` (PR #44 merged, archived 2026-06-21), so the
    renderer call already exists in both skills; no design-ahead gap.
  - **Script interface verified.** `archive-change.sh` accepts the exact flags both skills invoke
    (`--changes-dir --id --outcome --date [--results] --message`). No script change needed.
  - **No collision.** The parallel change #0038 touches only `tests/test_docket_metadata_branch.sh`
    — disjoint from `skills/docket-status/SKILL.md` and the sweep tests this change edits.
  - **No new ADR** (spec A8): this applies ADR-0012 (script-vs-model boundary), the #0026
    archive-primitive extraction, and the #0035 renderer-ordering learning; it makes no new
    decision. `adrs: []` stays.
  - **Presentation call (spec A9) resolved for build:** byte-align the sweep's prose to finalize
    step 3's sequence while keeping the sweep's own explicit log-and-continue failure posture in
    its own words — not reference-by-link — because the sweep's failure handling legitimately
    diverges and its surrounding prose (best-effort safety-net framing) is already self-contained.
    Final wording locked in the plan.
