<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0411 — Steer post-completion durable-write failures to rebase-continue, not abort](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0411-steer-post-completion-durable-write-failures-to-rebase-conti.md)**
<!-- docket:backlink:end -->

# Recover a completed resolver continuation after a durable-write failure

## Goal and scope

Make the existing recovery path discoverable when Git has completed an owned resolver continuation but Docket could not persist the reservation reconciliation. Direct the operator to retry `finalize.rebase-continue` with the same attempt and resolver report, preserving the completed work instead of defaulting to abort.

This is a diagnostic and workflow-guidance change. Preserve the rebase state machine, reservation admission, budget accounting, ownership checks, gate composition, and abort mechanics. Change 0411 remains low-priority `docs`, with a linked specification because the recovery boundary needs explicit acceptance coverage.

## Current context

Inspected main revision `3ccf9fac511f370c200315674a1edf97766b0a5b` and the existing proposal on the metadata branch. Change 0349 is done; its ADR-0113 already defines durable reservation and response-loss reconciliation. Changes 0396, 0408, and 0413 are also done and supply gate continuation, completed-gate checkpointing, and generated-bundle conflict handling. No prerequisite change or stacked branch is needed.

The original proposal correctly identifies the risk of losing completed rebase work, but its example location needs refinement:

- `finalizeRebaseContinueBudgeted` in `internal/app/finalize_rebase.go` writes `ResolverContinuationStarted` before staging/continuing Git. After Git returns, it clears the reservation on the receipt. That final write can fail after either rebase completion or advancement to another conflicting commit; the current message says only that the continue completed and the reservation could not be reconciled.
- `finalizeRebaseReconcileStarted` already recovers an outstanding, started continuation without replaying staging/continue. It proves either a different stopped commit or no active rebase with HEAD descending the recorded base. Its two reconciliation-write failures currently return the underlying error alone.
- `FinalizeResolverReserve` in `internal/app/finalize_reserve.go` is pre-dispatch admission. An outstanding reservation returns `pending`; a new reservation write failure proves nothing about rebase completion. Do not relabel this path as post-completion recovery or add a completion probe to it.
- The resolver loop and abort flow in `skills/docket-finalize-change/SKILL.md`, plus `references/gate-failure.md`, do not explain this exception. ADR-0105 separately requires a `waiting` gate result to resume through `finalize.rebase`.

Aborting restores the recorded original feature head and can discard the completed local rewrite. A completed rebase is not a completed PR merge or proof that the suite passed.

## Required guidance

### Diagnostic messages

Update the three reservation-reconciliation write-failure sites: the write after `StageAndContinueRebase`, and the advanced-conflict and completed-rebase branches of `finalizeRebaseReconcileStarted`.

Each message must identify the failed durable reservation/receipt write, preserve the underlying error detail, and name `finalize.rebase-continue` as the recovery operation using the **same change id, owned attempt, and original resolved report, including its reservation token**. Explain that recovery rechecks live state and reconciles the outstanding continuation; it does not authorize another resolver dispatch. Do not print report bodies or invent a report path that the operation does not know.

Wording must match the evidence already available at that site:

| Failure site | What the message may say |
|---|---|
| Immediately after Git continuation returned | Git continuation returned, but reservation reconciliation could not be persisted. Do not assert that the whole rebase finished: another conflict may remain. |
| Recovery proved a different stopped commit | The prior continuation advanced to another conflict, but its reservation reconciliation could not be persisted. |
| Recovery proved completion | The owned rebase completed, but its reservation reconciliation could not be persisted. |

Keep protocol version, operation, result, disposition, reason (`receipt-write-failed`), identity/count fields, and receipt schema unchanged. These are operation `message` diagnostics exposed through the existing JSON result. The compact `HumanText` summary currently omits `Message`; this change does not introduce a general message renderer or echo arbitrary error/report text there. The maintained skill must relay the relevant diagnostic and recovery instructions to the human.

Use the existing branches and facts to select wording. Do not add Git mutations, probes, a new recovery classification, receipt fields, or retry machinery merely to generate a message. Do not apply the special remedy globally to `receipt-write-failed`, which is also used for writes before Git starts and for gate bookkeeping.

### Finalize workflow documentation

Add a concise exception beside the resolver continuation/abort guidance in the main skill, with the full explanation in `references/gate-failure.md`. Both must agree:

1. When a continuation reports that only reservation reconciliation failed after Git advanced or completed, preserve the workspace, receipt, and original resolver report. Surface a human recovery instruction to rerun `finalize.rebase-continue` with the same `--id`, `--attempt`, and `--input`; resolve its executable invocation from capabilities and report shape from schema as usual.
2. Do not route this persistence failure to `finalize.rebase-abort`, restart the rebase, reserve or dispatch another resolver, fabricate a replacement report, or edit/delete the receipt. A generic `blocked` disposition or `receipt-write-failed` token alone is insufficient to diagnose this window. The operation's existing ownership and live-state checks remain authoritative; the skill must not reproduce them with handwritten Git probes.
3. This supplies an operator remedy, not an autonomous retry loop. If persistence still fails or the original report is unavailable, halt and retain the work, report the actual diagnostic and missing input, and use the existing finalize-block recording path where possible. Failure to record the block is reported honestly and never authorizes abort.
4. Once recovery returns, follow its actual result. A new conflict requires normal reserve-before-dispatch admission; an exhausted budget keeps its existing abort/halt route. A completed rebase still follows the normal gate and publication checks. A `waiting`/`gate-waiting` result resumes via the original identical `finalize.rebase` invocation, **not** another `rebase-continue` or a direct gate-drive call. Successful reconciliation consumes the outstanding reservation; do not keep replaying the old report afterward.
5. Keep existing verified abort guidance for stuck/unavailable resolvers, same-commit ambiguous continuation, foreign or unprovable state, legacy receipts, and exhausted conflict budgets. Preserve the requirement to establish resolver-child completion before any abort. This change does not authorize abort on an unproven state or bypass an existing refusal.

Write normative guidance in harness-neutral terms: owned attempt, resolver report, operation, and native child-completion evidence. No vendor-specific tool names, approval syntax, session identifiers, model names, or timing assumptions.

## Implementation boundaries

Expected authored surfaces are `internal/app/finalize_rebase.go`, the two finalize skill documents, and focused tests in their existing app/CLI or repository-guard test families. Read `finalize_reserve.go` as a negative control; it needs no behavioral change. Regenerate bundled skill assets through the existing asset generator when maintained skill text changes; never edit embedded copies manually. Respect any existing versioned-fixture drift protocol.

Do not change `finalize.rebase-continue` or `finalize.rebase-abort` control flow, reservation limits or refunds, cleanup, gate/evidence policy, merge authorization, or the generated-bundle fast path. Keep historical specs, plans, results, and Accepted ADRs intact. ADR-0113 and ADR-0105 already cover the decisions; no new architecture decision is required for wording that explains them.

## Acceptance criteria

1. **Failure after Git completion:** inject a receipt-write failure only at post-continue reservation reconciliation, after the continuation-started marker was durably written. Assert the unchanged error result/disposition/reason, a message naming the same-attempt `finalize.rebase-continue` remedy, and preservation of the rewritten HEAD, outstanding reservation, started marker, and used count. The gate must not run before reconciliation succeeds.
2. **Recovery, including another failed write:** retry with the original report. Exercise a failed reconciliation write in the completed-rebase recovery branch, then restore writes and retry successfully. Assert the diagnostic remains actionable; staging/continue is not repeated, HEAD is preserved, the reservation clears only through the successful existing recovery, and no resolver opportunity is charged or refunded. Normal gate composition follows.
3. **Advanced conflict:** cover both the immediate post-continue write failure when another conflict remains and the failed write in recovery after a different stopped commit is proven. Neither message may claim that the whole rebase completed. Recovery surfaces the next conflict or existing exhaustion result, without replaying the prior continuation or bypassing fresh admission for any subsequent resolver.
4. **Negative boundaries:** existing pre-dispatch reserve-write and pre-continue marker-write failures do not acquire a post-completion claim. Same-stopped-commit ambiguity, failed state/ancestry probes, a head not descending the base, missing/stale/consumed reservation, mismatched attempt, unresolved report, and legacy receipt retain their current refusals and mutation protections. Reuse existing behavior tests where they already establish these boundaries.
5. **Reachable, consistent guidance:** verify that JSON messages contain the applicable operation and reuse instructions, and that the main skill's abort flow links or includes the exception so the on-demand reference is discoverable before abort. Check the WAITING distinction, persistent-failure retention, and unchanged normal abort routes. If adding prose guards, make them reflow-tolerant and prove that removing the exception or substituting abort as this recovery remedy makes them fail; mere word-presence checks are insufficient.
6. **Repository validation:** regenerate embedded assets, run the focused failure/recovery tests and applicable documentation/asset guards, then the configured full build gate at implementation time. Record actual results and limitations in the required results artifact. This grooming session validates source grounding, specification consistency, and metadata publication; it does not claim these future implementation tests have run.
