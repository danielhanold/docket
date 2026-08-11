---
id: 275
slug: run-gate-has-no-runnable-path-for-slash-command-or-backgroun
title: 'Run gate has no runnable path for slash-command or backgrounded implement-next dispatch'
status: done
priority: high
type: fix
created: 2026-08-09
updated: 2026-08-11
depends_on: []
related: [242, 271]
discovered_from: [271]
adrs: [84]
spec: docs/superpowers/specs/2026-08-09-run-gate-detached-dispatch-path-design.md
plan: docs/superpowers/plans/2026-08-11-run-gate-detached-dispatch-path.md
results: docs/results/2026-08-11-run-gate-detached-dispatch-path-results.md
trivial: false
auto_groomable: true
branch: feat/run-gate-has-no-runnable-path-for-slash-command-or-backgroun
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/196
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-run-gate-detached-dispatch-path-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-run-gate-detached-dispatch-path-design.md) |
| Plan | [2026-08-11-run-gate-detached-dispatch-path.md](https://github.com/danielhanold/docket/blob/feat/run-gate-has-no-runnable-path-for-slash-command-or-backgroun/docs/superpowers/plans/2026-08-11-run-gate-detached-dispatch-path.md) |
| Results | [2026-08-11-run-gate-detached-dispatch-path-results.md](https://github.com/danielhanold/docket/blob/feat/run-gate-has-no-runnable-path-for-slash-command-or-backgroun/docs/results/2026-08-11-run-gate-detached-dispatch-path-results.md) |
| PR | [#196](https://github.com/danielhanold/docket/pull/196) |
| ADRs | [ADR-0084](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md) |
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

## Reconcile log

### 2026-08-11 — reconciled against current `main`

Design holds; scope unchanged. Verified against reality rather than the 2026-08-09 snapshot:

- **Oracle still carries what the detached path needs.** `scripts/verify-run.sh` has
  `--in-progress-ids --with-claimed-at` and `--iso-to-epoch`, and `runner-dispatch.sh` still
  implements the three-filter attribution the spec ports as prose (`BEFORE`, `DISPATCH_EPOCH`,
  the readable-`claimed_at` and `>= DISPATCH_EPOCH` legs). No script change is needed, as the
  spec assumed.
- **Template and its guard are green today.** `cursor-rules/run-gate.md` is 25 lines — exactly at
  the brevity bound — and `tests/test_sync_agents_run_gate.sh` passes in ~3.5s against its 10s
  budget row, including the AGENTS.md currency assert. The bound raise and the regeneration
  recipe are both still required exactly as the spec describes.
- **#0277 (`--brief-file`) does not reach this gate.** 0277 moved *delegated* task briefs off
  argv for the `runner-dispatch.sh` facade path, and its refused shape is brief-file-plus-argv.
  The caller-side gate never invokes the facade — it uses only `docket.sh preflight` and
  `docket.sh verify-run`, and its step-4 re-dispatch is a **native named-agent dispatch** whose
  retry context rides the dispatch prompt, not an argv tail. The new detached prose inherits that
  and must stay native-dispatch-only; it must not be phrased as a facade invocation.
- **#0286 (`gate-run --observe` loop shape) does not apply.** The detached path is
  notification-driven — the session regains control once, at the notification — so it generates
  no polling loop and must not grow one. `scripts/gate-run.md`'s taught capture-then-match loop
  is therefore not a dependency here.
- **Site list derived from a whole-repo grep, not hand-listed** (the #0208 lesson). The gate text
  has exactly one authored source, `cursor-rules/run-gate.md`, spliced by
  `sync-agents.sh:assemble_run_gate` into the Cursor rule and the committed `AGENTS.md` block
  (`CLAUDE.md` is the same physical file). The fourth prose population, `cursor-rules/dispatch/`,
  holds per-agent roster fragments only and carries no gate text.
- **New finding — a second countermanding site the spec did not enumerate.**
  `cursor-rules/dispatch.head.md` item 2 also says "never background it and never poll", and on
  the Cursor surface it is spliced *above* the gate. That is the same countermand the critic
  caught in step 2, on a page the spec's delivery list does not name. Resolved **without
  widening scope**: rather than editing `dispatch.head.md` — whose directive governs every docket
  agent dispatch, not just implement-next runs — the new Detached section opens by naming its own
  precondition explicitly (it governs a dispatch that was *not* foreground-blocked, whoever
  backgrounded it), so the gate is self-consistent on both surfaces from its own text.
- **Auto-capture:** nothing minted. The one discovery above is drift inside this change's own
  scope and is fixed in-branch, which fails the materiality bar by construction.
