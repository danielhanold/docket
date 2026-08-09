---
id: 275
slug: run-gate-has-no-runnable-path-for-slash-command-or-backgroun
title: 'Run gate has no runnable path for slash-command or backgrounded implement-next dispatch'
status: proposed
priority: high
type: fix
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: [242, 271]
discovered_from: [271]
adrs: []
spec: docs/superpowers/specs/2026-08-09-run-gate-detached-dispatch-path-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-run-gate-detached-dispatch-path-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-run-gate-detached-dispatch-path-design.md) |
<!-- docket:artifacts:end -->

## Why

The run gate promoted into `AGENTS.md` ("Run gate — verify a dispatched implement-next run before
you report it") assumes one dispatch shape: the session takes a `verify-run --in-progress-ids`
snapshot, dispatches **foreground**, blocks on the return, re-snapshots, and diffs the two to
identify which change the run claimed.

When the run is launched as a backgrounded slash command — `/docket-implement-next 271`, which is
how a human actually starts one — that shape is unreachable. The dispatch happens within the same
user turn that requests it, so there is no point at which the session can take the before-snapshot,
and the run is not foreground. Steps 1–3 are structurally unrunnable on that path.

Observed live on change 0271 (2026-08-09), the first session to load the gate after 0242 created
docket's Claude surface. Only step 4 (`verify-run <id>`) could be run, and only because the agent's
report happened to name the id. Had the run died before reporting, the session would have had no id
to verify and no snapshot diff to recover one from — precisely the silent-failure case the gate
exists to catch.

## What

Settled design (auto-groomed 2026-08-09; see the linked spec's Assumptions for the audit trail).
Amend the gate template (`cursor-rules/run-gate.md`, single source) only — no script changes; the
oracle already carries `--with-claimed-at` / `--iso-to-epoch` (0271):

- Keep the foreground path primary, with step 2's "never background" prohibition **scoped** so it
  no longer countermands the new section on the same page.
- Add a named **Detached dispatch** section. A session-issued backgrounded dispatch takes the
  full before-snapshot plus a `DISPATCH_EPOCH` before launching, and attributes at the
  notification with the runner gate's full three-filter rule (not-in-before-set, `claimed_at`
  parses, `claimed_at` >= epoch); one survivor follows step 4's verdict table unchanged.
- A slash-command / notification-first launch has no before-set, and a timestamp alone cannot
  attribute (live foreign runs re-stamp `claimed_at`): it enters a named **unattributed** mode —
  verify-and-report only, **never re-dispatch** (mirrors the runner facade's observe-only seam).
- Reword the ordering obligation: the notification is the child's claim, not the session's
  report — verify before *relaying* it as an outcome.
- Delivery: raise the template test's 25-line brevity bound deliberately with recorded rationale,
  adjust the step-2 asserts, and regenerate via the test's documented recipe (a bare
  `sync-agents.sh` run is a no-op in this repo).
