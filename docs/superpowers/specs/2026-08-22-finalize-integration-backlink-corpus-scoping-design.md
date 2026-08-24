<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0337 — Finalize's integration-ref backlink leg refuses on unrelated pre-existing corpus errors](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-22-0337-finalize-leaves-a-permanent-terminal-backlink-pending-leg-un.md)**
<!-- docket:backlink:end -->

# Finalize's integration-ref backlink leg refuses on unrelated pre-existing corpus errors

**Change:** 0337 · **Type:** fix · **Status:** design settled (spec)
**Discovered from:** 0336 (dogfood finalize run, 2026-08-22)

## Problem

When a change's PR merges, its build artifacts — the `plan:` and `results:` files, each carrying a
generated `docket:backlink` block — land on the integration branch. At close-out the change file
moves from `active/<id>-<slug>.md` to `archive/<YYYY-MM-DD>-<id>-<slug>.md`, so finalize's
**integration-ref backlink leg** re-stamps those on-branch backlink blocks to point at the archive
path.

That leg never lands. `finalize closeout` emits a `terminal-backlink-pending` warning and
`finalize cleanup` returns `disposition: pending` / `reason: terminal-backlink-pending`
**permanently** — every retry, including the maintenance sweep, reproduces the same coarse
`invalid-state`. The on-branch plan/results artifacts are left backlinking to a stale `active/…`
path that no longer exists on any branch. The change itself is correctly `done` and archived; only
this best-effort leg is stuck.

Observed live finalizing 0336: PR #227 merged and archived to `done`, but cleanup stayed `pending`
across two retries, and `docs/superpowers/plans/2026-08-21-finalize-effective-merge-method.md` plus
`docs/results/2026-08-21-…-results.md` on `origin/main` still point at `docs/changes/active/0336-…md`.

> **Note — the stub's original framing was wrong.** The stub attributed this to
> `terminal_publish: false` ("the archive path doesn't exist on the integration branch, so the
> re-stamp target is invalid"). Source investigation disproved that: the leg does **not** gate on
> `terminal_publish` (it gates only on docket mode), an absent artifact or missing block on the
> integration ref is a benign skip → no-op (not a refusal), and the re-stamp target is the archive
> path *inside the block content*, whose validity does not depend on the change file existing on the
> integration branch. The title and framing are corrected here.

## Root cause (confirmed)

The `invalid-state` token is the fallback result for a transaction whose disposition is **Refused**
(`mapOutcome(res, execErr, ResultInvalidState)`). A push rejection (e.g. branch protection) would be
`Failed`/`external`, a different token — so the refusal happens **before any push**.

The refusal originates in the transaction engine's `LoadBefore` step. Both legs
(`runCloseoutBacklinkLeg` and `finalizeCleanupBacklinkRepair`) execute against
`TargetRef = refs/heads/<integration_branch>` with `Loader = newPlanningLoader(...)`. The engine's
per-attempt order loads the **complete state visible through the target ref's tree** and, if
`before.Report.HasErrors()`, returns `refusedOutcome(StageLoadBefore, …)`. `planningLoader.Load`
reads the entire corpus named by `corpusPrefixes` — `active/`, `archive/`, `adrs/`, and
(when enabled) `learnings/` — under `refs/heads/<integration_branch>`, parses every record, and
builds a validation snapshot. A single record whose bytes fail `document.Parse` becomes a
`SeverityError` parse finding (`planningParseFinding`), which makes `HasErrors()` true and refuses
the whole transaction.

**The mutation itself never touches a corpus record.** The backlink patch changes only the
`docket:backlink` block of plan/results artifacts under `docs/superpowers/…` and `docs/results/…`,
which are **not** in `corpusPrefixes`. So the leg validates a body of records it has no intention of
modifying, and any pre-existing error anywhere in that body refuses a patch that provably cannot
affect it.

**The concrete trigger in this repo:** `origin/main` carries a frozen `docs/adrs/0024-…md` whose
`title:` is an **unquoted scalar containing a colon-space** (`` `context: fork` ``), so
`document.Parse` fails with `yaml: mapping values are not allowed`. The corrected (quoted) ADR-0024
exists on `origin/docket` but was never republished to `main` (ADRs are immutable on the integration
branch once published; the quoting fix post-dated 0024's publish). Confirmed by running the real
`BuildSnapshot` validator over `origin/main`'s on-disk corpus: **1 error** among 365 records — the
ADR-0024 parse failure — with 66 non-error findings.

**Why the leg's failure is invisible.** The two legs deliberately discard the typed
`transaction.Failure` (stage / kind / detail) and fold only the coarse result token into the
warning — "it surfaces only the coarse result token, not the typed transaction.Failure". So the
finding reads `invalid-state` with no hint that the cause is a parse failure on a specific ADR. This
is why diagnosis required a full source dive.

**Why existing tests pass.** `TestCloseoutBacklinkLegDocketMode` (and the lighter assertion in
`finalize_git_test.go`) fixtures a small, internally-consistent integration corpus, so `LoadBefore`
has no errors. No test drives the leg against an integration corpus carrying a pre-existing,
mutation-unrelated error, so the refusal path shipped uncovered.

## Design — A + D + C

### A — Scope the leg's gate to what it mutates (the class fix)

A backlink-only patch must not be refused by the health of records it does not touch. Change the two
integration-ref backlink legs so their transaction no longer refuses on pre-existing, unrelated
corpus errors on the integration branch.

Preferred realization: give these legs a **loader/gate scoped to the artifacts actually patched**
rather than `newPlanningLoader` over the full corpus — validate that each targeted plan/results
document parses and carries the block being replaced, and drop the whole-corpus `HasErrors()` gate
for this operation. The engine's other guarantees stay intact: exact-path expectations, the
`verify-delta` declared-vs-actual check, idempotent empty-plan no-op, and the CAS lease push are
unchanged. An absent artifact / missing block / already-retargeted block remains a benign skip.

Design constraints this must satisfy:

- **The integration branch legitimately holds a partial corpus.** Under docket's branch model it
  carries `archive/` + `adrs/` but no `active/` changes — validating it as if complete was never
  correct for this leg. A aligns the gate with the mutation's actual reach, so correctness does not
  depend on the integration corpus happening to be error-free.
- **Idempotency and retry-safety are preserved.** The leg must stay a clean no-op on replay and land
  exactly the generated-only patch when pending — so a repo's backlog of stuck backlinks self-heals
  on the next `finalize cleanup` / maintenance sweep after the fix ships (see *Cross-repo rollout*).
- **No new config knob.** The fix is behavioral in the runtime, not a repo-tunable.

Alternative considered — **B, refuse only on errors the mutation introduces** (diff before/after
corpus error sets, refuse only on new ones): also fixes the class, but keeps loading and validating
the full corpus every attempt to prove a delta that is structurally always empty for a patch that
touches no corpus record. Rejected as more machinery than A for no added safety.

### D — Surface the typed failure (diagnosability)

Stop discarding the `transaction.Failure` on these legs. The `terminal-backlink-pending` finding
must carry the transaction's stage / kind / detail (e.g. "load-before: parse failure on
`docs/adrs/0024-…md`") so the next occurrence of *any* still-in-scope refusal is self-diagnosing
instead of an opaque `invalid-state`. This is independent of A and warranted regardless of it —
after A, an in-scope artifact-level problem is the only thing that can still block the leg, and D is
what names it.

### C — Republish the corrected ADR-0024 to `main` (repo-local hygiene)

Republish the quoted, parse-clean `docs/adrs/0024-…md` from `origin/docket` onto `origin/main` so
the frozen integration-branch record is no longer malformed. This is docket-repo-specific data
hygiene, **not** a functional dependency of A/D: after A the malformed record is already harmless to
close-outs. C is included because a genuinely malformed Accepted ADR should be corrected on every
branch that carries it, and doing it now clears the one real error on `main`.

## Cross-repo rollout (design constraint, not just narrative)

This bug lives in the compiled `docket` runtime, so the fix's reach is defined by how the pieces
distribute — and that shapes the design:

- **A and D are binary-level and must require zero per-repo action.** Every docket repo runs the
  same runtime; upgrading to the binary carrying A + D fixes the class everywhere with no config,
  migration, or branch surgery. A must therefore be a pure behavioral change in the leg — **no new
  knob, no repo-specific data assumption** — so a repo with *any* pre-existing integration-corpus
  error (malformed, stale, partial) is fixed by upgrading alone.
- **C is repo-local and non-load-bearing for others.** Other repos do not have docket's ADR-0024
  and do not need us to perform a "C" for them; A makes their own pre-existing errors harmless. If a
  repo has its own malformed record it wishes to correct, D now hands it the exact pointer.
- **The backlog self-heals.** Because the leg is idempotent and retryable (a constraint A must
  preserve), after upgrading, each repo's next maintenance sweep re-runs the leg over changes closed
  while the bug was live and lands the previously-stuck re-stamps — correcting stale `active/…`
  backlinks to `archive/…` with no merge redone. Docket's own `main` (0336's plan/results currently
  pointing at `active/0336…`) heals the same way once A ships and the sweep runs.

## Testing

- **Regression (the coverage gap):** a test that drives an integration-ref backlink leg against an
  integration corpus carrying a pre-existing, mutation-unrelated error (e.g. an ADR that fails
  `document.Parse`) and asserts the leg **lands** the plan/results re-stamp and emits **no**
  `terminal-backlink-pending` finding. This test must fail before A and pass after.
- **Diagnosability (D):** a test that forces a still-in-scope leg failure and asserts the finding
  carries the typed stage/kind/detail, not a bare `invalid-state`.
- **Idempotency preserved:** the existing no-op/replay assertions
  (`TestCloseoutBacklinkLegDocketMode`, the `finalize_git_test.go` assertion) continue to pass —
  clean no-op on replay, integration commit touching exactly `{plan, results}`.
- Full suite at the build gate per the repo's `finalize.test_command`.

## Out of scope

- The `terminal_publish: true` publish leg (the script-driven copy of the archived change record +
  spec + Accepted ADRs onto the integration branch) — a different mechanism, unaffected here.
- Change 0336's merge-method selection work (already merged).
- What artifacts a feature branch merges onto the integration branch.
- A general "validate and repair the integration branch's frozen corpus" facility — out of scope;
  A's point is that this leg should not be gating on that corpus at all.

## Resolved questions

- *Is `invalid-state` intended, or a latent bug?* Latent bug — a domain refusal at `LoadBefore` on
  an unrelated corpus error, surfaced through a coarse token. Fixed by A; made legible by D.
- *Where should the on-branch backlink point?* At the archive path, exactly as the leg already
  computes — the target was never the problem; the whole-corpus gate was. No change to the block
  content model.
- *One-time backfill of stale backlinks?* Not needed as separate machinery — the idempotent leg
  self-heals the backlog on the next sweep after A ships (see *Cross-repo rollout*).
