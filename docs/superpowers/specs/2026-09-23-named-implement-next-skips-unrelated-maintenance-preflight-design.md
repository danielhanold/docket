<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0448 — Named implement-next skips unrelated maintenance preflight](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-24-0448-named-implement-next-skips-unrelated-maintenance-preflight.md)**
<!-- docket:backlink:end -->

# Change 0448 — Named implement-next skips unrelated maintenance preflight

## Origin and ordering

Split out of change 0446 on 2026-09-23 (its former §7 and acceptance test 9). 0446 keeps the gate-history and run-bookkeeping fix. This change removes the incidental maintenance sweep from a named implementation start. Change 0449, which depends on this one, scopes metadata validation and board rendering. The three are built in succession (0446 → 0448 → 0449), which `depends_on` enforces. There is no code dependency on 0446. The ordering is the maintainer's build sequence.

## Required outcome

An explicitly named change B must be able to start, retry, and resume implementation even when another change A has a failing closeout, cleanup, or reclaim. A's maintenance problems stay visible to anyone who deliberately runs maintenance, but they cannot halt B before B's own claim/gate. The ID is preserved throughout; a named request never falls back to selecting another change.

Shared repository/configuration/transport failures, defects in B, B's real unmet dependencies, and genuine ownership conflicts remain valid refusals. A defect attributed only to independent A does not.

## Grounding

Inspected main `442770e11bd05baf6200bec53e9c4757622811a6` on 2026-09-23 (read-only).

- **#0389 / ADR-0101 and #0397 / ADR-0106:** historical done/stacked-merged cleanup is already deferred out of implementation startup (`sweepWorklist` skips it when `scope == SweepScopeImplementation`). But `sweepWorklist` still enqueues merged-implemented closeouts and expired-claim reclaims at both scopes, and `sweepRunCloseout` always appends the cleanup suffix after an applied or no-op closeout. When `reclaim.auto` is false a reclaim becomes `skipped`, which is not a problem disposition.
- `preflightVerdict` (`maintenance_preflight.go`) makes any entry in `problemDispositions` (blocked, failed, unknown, contended) a `problem`, and so is a whole-sweep result that is neither applied nor no-op.
- `skills/docket-implement-next/SKILL.md` Step 0 says: "before selection, run the implementation preflight … On `preflight: problem` … halt before claiming". It has no explicit-ID exemption. The `agents/docket-implement-next.md` wrapper only preloads the skill. Other named-entry callers must be derived by search.
- Changing gate admission (0446) cannot fix this earlier refusal. Relaxing the metadata gate (0449) cannot fix it either: the sweep's closeouts run through the same strict transaction gate, so an invalid A still yields blocked/failed entries and a halt.
- **Dependency semantics:** `EvaluateDependencies` (`internal/domain/graph.go`) treats a dependency as satisfied only when it is `done`; an `implemented` dependency yields `needs-merge`. `EvaluateReadiness` reports `waiting-dependency` before anything else.

Relevant learnings: verify the hypothesis against code; test the producer/consumer path and its negative counterpart; printed remedies must work in the state that produced them.

## Design

### 1. Named start goes straight to B

After the existing repository preparation/configuration checks, an implement-next request naming B goes directly to the existing authoritative `context.implementation --id B` and named claim/resume path. Do not invoke `maintenance.preflight`, a status sweep, or another maintenance child first. Do not depend on the global `status.ready` queue to certify a named resume. Keep readiness, entity-version, dependency, effective-base, claim eligibility, gate context, and resume/cancellation checks for B. A malformed, absent, not-ready, or improperly resumed B returns its existing local refusal; it never silently becomes a request for another change.

This is a workflow change at the existing explicit-ID branch, not a new preflight command, scope flag, or verdict policy. Update the maintained implement-next instructions and every executable named-entry wrapper that currently imposes the sweep; derive those callers by search. Initial named calls, attributed retries, and named resumes must all take this path. Gate arming and attribution remain unchanged.

### 2. Deliberate maintenance keeps its honest outcomes

The existing no-ID implementation preflight, explicit `maintenance.preflight`, and `maintenance.sweep` keep their current outcomes. A user deliberately running maintenance still sees A's failures. `maintenance sweep --scope full` and the existing targeted closeout/cleanup/reclaim operations remain the places to recover that work. Add no deferred queue, background job, or implicit cleanup after B. Named finalize likewise must not acquire a new unrelated maintenance prerequisite.

### 3. B's own merged dependencies

Skipping the sweep never declares B's dependency satisfied. It does create a behavior change that must be handled. Today, when B `depends_on` an A whose PR has merged but which has not been closed out, the preflight sweep's merged-recovery closeout moves A to `done` and B proceeds. Without the sweep, B would refuse with `waiting-dependency`/`needs-merge`.

Preferred handling: the named path runs the existing targeted closeout for **B's own** unmet `depends_on`/stack ancestors that are in `needs-merge` state with a merged PR, and nothing else. A failure there is a failure of B's real dependency and refuses B locally with A's locator. A failure in any other change's closeout cannot reach B.

Fallback, if implementation cannot bound that closeout to B's dependency set through existing operations: a local refusal whose printed remedy names A and the existing finalize/closeout operation that settles it. Never silently wait or select another change. Record the chosen behavior in the ADR (below).

## Acceptance tests

1. **Named startup:** with A's merged-but-unclosed record and failing closeout/cleanup/reclaim seams, invoke implementation explicitly for B. Prove no maintenance operation is invoked and B reaches its own claim/gate. Repeat for attributed retries and named resume.
2. **Local refusals still hold:** missing/invalid B, unmet actual dependencies, active same-worktree owners, and unsupported shared configuration still refuse; no fallback selection.
3. **B's merged dependency:** B depending on a merged-but-not-closed-out A either proceeds after the bounded closeout of its own dependency, or refuses locally with a working remedy naming A (whichever §3 behavior is implemented). An unrelated change's failing closeout is never attempted. A failing closeout of B's own dependency refuses B with A's locator.
4. **Unchanged behavior:** regression coverage for no-ID implementation preflight, explicit `maintenance.preflight`, and `maintenance.sweep` outcomes.
5. **Mutation checks:** restore mandatory maintenance on a named request, or let the bounded dependency closeout touch a non-dependency; the corresponding test must fail.
6. **Build gate:** run the whole suite via the source Go runner using resolved `build.test_command`, and inspect budget findings. Use existing fixtures and seams; no new test framework.

## Architecture and scope limits

Through the ADR workflow, narrow ADR-0101/ADR-0106's mandatory startup-preflight clauses for explicit IDs, and record the §3 dependency-closeout choice. Accepted ADR text is not rewritten in place.

No new commands, force flags, preflight scope flags, verdict policies, background cleanup, or deferred queues. Automatic (no-ID) selection, automatic finalize candidate ordering, and driver stop/continue behavior are out of scope. Gate admission and run bookkeeping belong to 0446. Metadata validation and board rendering belong to 0449.
