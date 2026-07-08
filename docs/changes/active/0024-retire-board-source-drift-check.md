---
id: 24
slug: retire-board-source-drift-check
title: Retire or downgrade the inline board/source-drift health check once rendering is deterministic
status: in-progress
priority: low
created: 2026-06-18
updated: 2026-07-08
depends_on: [22]
related: [23]
adrs: []
spec: docs/superpowers/specs/2026-07-08-retire-board-source-drift-check-design.md
plan: docs/superpowers/plans/2026-07-08-retire-board-source-drift-check.md
results:
trivial: false
auto_groomable:
branch: feat/retire-board-source-drift-check
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-08-retire-board-source-drift-check-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-08-retire-board-source-drift-check-design.md) |
| Plan | [2026-07-08-retire-board-source-drift-check.md](https://github.com/danielhanold/docket/blob/feat/retire-board-source-drift-check/docs/superpowers/plans/2026-07-08-retire-board-source-drift-check.md) |
<!-- docket:artifacts:end -->

## Why

The **board/source-drift** health check exists because the `inline` board is
rendered by the model: a writer skill could regenerate `BOARD.md` inconsistently
with the change files, so `docket-status` re-renders in-memory and warns on any
disagreement. Now that change 0022 has made `inline` rendering **deterministic**
(a script that emits byte-identical output from the same change files), that whole
failure class for `inline` disappears — a script cannot "render the board wrong"
the way a model can.

That leaves a question worth its own decision (spun out of 0023): does the
`inline` board/source-drift check still earn its keep, and if so in what reduced
form?

## What changes

**Decided (auto-groomed 2026-07-08 — see [the spec](../../superpowers/specs/2026-07-08-retire-board-source-drift-check-design.md)): retire the `inline` drift check.**

- **Retire** the `inline` board/source-drift check from `docket-status`'s
  Health-checks section. It is now vacuous: change 0022 killed the
  "board rendered *wrong*" failure class (a deterministic script cannot), and the
  surviving "board-refresh *skipped*" class is unobservable where the check runs —
  `docket-status`'s Board pass unconditionally re-renders `BOARD.md` **before** the
  Health-checks pass, healing any staleness first. The board is a self-healing
  derived view; the convention's **Board refresh on status writes** invariant is
  the real defense and stays.
- **Keep** the **`github`** surface's mirror-reachability visibility flag (split
  out of the same bullet) — best-effort, self-healing, unaffected by 0022.
- **No scripted replacement** now. A future `docket`-branch CI `--strict` gate is
  the only thing that would justify a `board-stale` byte-compare in
  `board-checks.sh`; no such consumer exists, so it is deferred (YAGNI) — cheap to
  add later since `render-board.sh` is deterministic. See spec §A2.
- **No new ADR, no convention edit** — retiring a vacuous warn-only check follows
  from 0022's determinism (itself ADR-free) and reverses no Accepted ADR (ADR-0012's
  script-vs-model boundary is untouched); the convention enumerates no such check.
- Update `docket-status`'s Health-checks bullet plus the two live tests that lock
  the check in place (`tests/test_board_checks.sh`, `tests/test_board_refresh_on_transition.sh`).

## Out of scope

- The board render script itself (change 0022) and the sweep/health-check
  scripting decision (change 0023).
- The `github`-surface mirror-reachability flag.

## Open questions

Resolved at auto-groom 2026-07-08 (see the spec). None blocking; build-ready.

- **Is a staleness check still useful once rendering is deterministic?** No — the
  must-land board-refresh discipline plus `docket-status`'s unconditional Board-pass
  re-render already keep `BOARD.md` fresh, and a check placed after that re-render
  cannot observe staleness. Resolved: retire it (spec §3, §A1).
- **ADR or convention edit?** Neither — it is a skill + test edit only. Retiring a
  vacuous check reverses no ADR and the convention enumerates no such check
  (spec §A3).

## Reconcile log

- **2026-07-08** — Reconciled at claim, just-in-time before planning. Verified against
  current `origin/main` @ `394cead` — unchanged since the spec was authored today, so the
  spec's assumptions hold verbatim. Ran the spec's build-time regrep (`board/source[- ]drift`
  over `skills/`, `tests/`, `scripts/` on `origin/main`): it returns **exactly** the three
  touch-points the spec names — `skills/docket-status/SKILL.md:185`,
  `tests/test_board_checks.sh:327-328`, `tests/test_board_refresh_on_transition.sh:28-30` — and
  no additional live consumer. Dependency `#22` (render-board.sh determinism) is `done`;
  related `#23` is `done`. Scope unchanged: retire the `inline` drift check, keep the `github`
  mirror-reachability flag, no scripted replacement, no ADR, no convention edit. Nothing dropped
  or folded in; the change and spec are current as written. Build-ready.
