---
id: 337
slug: finalize-leaves-a-permanent-terminal-backlink-pending-leg-un
title: Finalize's integration-ref backlink leg refuses on unrelated pre-existing corpus errors
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-22
updated: '2026-08-22'
depends_on: []
stacked_on:
related: []
discovered_from: [336]
adrs: []
spec: docs/superpowers/specs/2026-08-22-finalize-integration-backlink-corpus-scoping-design.md
plan: 'docs/superpowers/plans/2026-08-22-finalize-backlink-leg-corpus-scoping.md'
results:
trivial: false
auto_groomable:
branch: 'feat/finalize-leaves-a-permanent-terminal-backlink-pending-leg-un'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-22T13:33:42Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-22-finalize-integration-backlink-corpus-scoping-design.md` |
| Plan | `docs/superpowers/plans/2026-08-22-finalize-backlink-leg-corpus-scoping.md` |
<!-- docket:artifacts:end -->

## Why

Finalize's integration-ref backlink leg — the part of `finalize closeout` / `finalize cleanup` that
re-stamps the `docket:backlink` block of merged plan/results artifacts to the archive path — never
lands. `closeout` emits a `terminal-backlink-pending` warning and `cleanup` returns
`disposition: pending` **permanently**: every retry, including the maintenance sweep, reproduces the
same coarse `invalid-state`, and the on-branch artifacts are left backlinking to a stale `active/…`
path. The change itself closes out correctly; only this best-effort leg is stuck.

Root cause (confirmed by source investigation and by running the real validator over `origin/main`):
the leg runs against the integration branch with the **full-corpus** planning loader, which reads and
validates *every* change/ADR/learning record on that branch and refuses the transaction if any of
them has an error — even though the patch touches only plan/results backlink blocks, which are not
corpus records at all. `origin/main` carries a frozen `docs/adrs/0024-…md` whose unquoted title
contains a colon-space (`` `context: fork` ``) and fails YAML parse; that single error refuses a
patch it has nothing to do with. Compounding it, the leg discards the typed transaction failure, so
the finding says only `invalid-state` with no pointer to the offending record.

The stub's original framing (a `terminal_publish: false` special case) was disproved: the leg does
not gate on `terminal_publish`, an absent artifact is a benign no-op, and the re-stamp target is
valid regardless of whether the change file exists on the integration branch. This change is retitled
accordingly.

Discovered finalizing 0336 (dogfood run, 2026-08-22): its plan/results on `origin/main` still point
at `docs/changes/active/0336-…md`.

## What changes

Design settled as **A + D + C** (see the linked spec):

- **A — scope the leg's gate to what it mutates.** The two integration-ref backlink legs stop
  refusing on unrelated pre-existing corpus errors on the integration branch: validate only the
  plan/results artifacts actually patched, not the whole integration-branch corpus. No new config
  knob; the integration branch legitimately holds a partial corpus, so full-corpus validation was
  never the right gate for a backlink-only patch. Idempotency/retry-safety preserved so the backlog
  self-heals on the next sweep.
- **D — surface the typed failure.** The `terminal-backlink-pending` finding carries the
  transaction stage/kind/detail so any remaining in-scope refusal is self-diagnosing rather than a
  bare `invalid-state`.
- **C — republish the corrected ADR-0024 to `main`.** Repo-local data hygiene (the quoted, clean
  ADR already lives on `docket`); not a functional dependency of A/D, which make the malformed
  record harmless anyway.

The fix is binary-level (A + D): every docket repo gets the class fix by upgrading, with no per-repo
migration, and its backlog of stuck backlinks self-heals on the next maintenance sweep.

## Out of scope

- The `terminal_publish: true` publish leg (copying archived records onto the integration branch) —
  a different mechanism, unaffected.
- Change 0336's merge-method selection (already merged).
- What artifacts a feature branch merges onto the integration branch.
- A general facility to validate/repair the integration branch's frozen corpus — A's point is that
  this leg should not gate on that corpus at all.

## Reconcile log

### 2026-08-22

**2026-08-22** — Reconciled against current `origin/docket` and `origin/main`. Every claim in the spec re-verified from source, and the design (A + D + C) holds unchanged:

- **A/D bug is live and matches the spec.** Both integration-ref backlink legs — `runCloseoutBacklinkLeg` (`internal/app/finalize_closeout.go`) and `finalizeCleanupBacklinkRepair` (`internal/app/finalize_cleanup.go`) — still execute against `refs/heads/<integration_branch>` with `Loader: newPlanningLoader(cc.eff)` (the full-corpus loader) and still discard the typed transaction failure via `result, _ := mapOutcome(res, execErr, ResultInvalidState)`, folding only the coarse token into the warning.
- **C trigger confirmed.** `docs/adrs/0024-claude-context-fork-skill-dispatch.md` on `origin/main` carries an unquoted `title:` containing a colon-space (`` `context: fork` ``) that fails `document.Parse`; the quoted, parse-clean form already lives on `origin/docket`.
- **Scope note on C.** A + D are the binary-level code fix and are the PR's deliverable. C (republishing the already-clean ADR-0024 from `docket` onto `main`) is repo-local data hygiene, explicitly not a functional dependency of A/D, and does not fit the feature-branch model (feature branches never modify docket metadata / ADRs). It is recorded here and surfaced to the human at the merge gate as a separate direct-to-`main` publish, not carried on the feature branch.

No scope change; `discovered_from: [336]` retained. reconciled → true.
