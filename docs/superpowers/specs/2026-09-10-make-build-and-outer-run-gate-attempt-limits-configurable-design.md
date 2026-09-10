<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0421 — Make build and outer run gate attempt limits configurable](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-10-0421-make-build-and-outer-run-gate-attempt-limits-configurable.md)**
<!-- docket:backlink:end -->

# Configurable build and outer run gate attempt limits

## Agreed outcome

Give the build gate more room to repair failing code while keeping the outer implementation-run allowance conservative. The user approved two independently configurable positive-integer limits, both counting the initial attempt:

| Configuration key | Built-in default | Meaning |
| --- | --- | --- |
| `build.max_attempts` | 4 | Initial build full-suite run plus at most three repair-and-rerun cycles |
| `run.max_attempts` | 2 | Initial implementation run plus at most one eligible retry |

A value of 1 disables retries. Zero, negative, fractional, string, boolean, and collection values are invalid. Stop early on success. Unlimited retries and progress heuristics are outside this change.

## Context and boundaries

The build workflow currently turns a red full-suite result into one synthetic integration-repair task with the existing premium-to-max escalation ladder, then halts when that bounded repair path cannot finish. The outer gate independently uses `ConsumeGateRetry` to grant a single retry for an attributed, quiescent `run-incomplete`; `run-halted` is terminal, and tracked work is continued without consuming the retry. The new settings replace these fixed budget assumptions while preserving ownership and safety boundaries.

Change 0419 addresses finalize repair and resolver settings separately. This change does not change finalize, focused task-test policy, the post-review fix-loop's separate suite bound, or human merge approval. No dependency on 0419 is required; reconcile overlapping configuration/documentation edits at build time.

## Configuration and exposure

Add both leaves to the canonical configuration schema and defaults. Apply the existing per-field precedence: machine-local repository override, committed repository configuration, global user configuration, then built-in. These are execution-policy settings, available at all normal layers. Validate them consistently with existing positive-integer attempt settings.

Expose effective values and provenance in configuration diagnostics and schema descriptions. Carry the resolved build value through typed implementation context to the build workflow; the outer gate resolves its own run value through authoritative configuration. Each budget is snapshotted when its owning gate scope begins. Configuration edits affect newly started scopes and do not increase or reset an already-owned budget.

## Build gate behavior

Count logical full-suite attempts in the build phase, including the initial run. For the default 4, a red initial run can be followed by repair and suite attempts 2, 3, and 4. A green run ends the phase immediately and supplies evidence for its exact tested HEAD. A red final permitted run takes the existing explicit halt path with an exhaustion reason.

Each additional run requires a repair of the observed failure. Retain existing worker verification, safe escalation, regression coverage, and no-test-weakening rules. Worker escalation and focused diagnostics do not themselves spend a full-suite attempt; any build full-suite rerun performed by a repair worker uses this same budget. No worker may bypass the cap by treating its suite rerun as outside the controller's accounting. Do not invoke review while the build is red.

Reserve an attempt durably before launching its logical suite run, scoped to the existing owning build phase. Re-observation, ownership transfer, handoff, continuation, and the driver's existing recovery of that same logical run do not reserve another attempt. Preserve consumed budget across controller interruption and continuation. A skipped build gate launches no suite and follows existing skipped-evidence behavior.

Infrastructure errors, unavailable results, invalid configuration, unsafe worker outcomes, and exhausted observation budgets retain their existing halt/fail-closed handling. They are not newly retryable red-suite results. Exhaustion is a bound, not a requirement to continue despite another halt condition.

## Outer gate behavior

Bind the snapshotted `run.max_attempts` and durable attempt state to the existing gate key and attributed claim. The original implementation dispatch is attempt 1. Only the facade may authorize each next attempt for the same attributed change, and only after the prior run has returned and the existing checks establish an eligible quiescent incomplete run.

At a configured maximum N, permit at most N-1 retries, then issue the existing terminal stop disposition. The default 2 preserves current behavior. A limit of 1 stops at the first eligible incomplete result without granting a retry. Higher settings allow the corresponding additional eligible retries.

Retry grants must be atomic and tied to an attempt transition: concurrent or repeated observations of the same completed attempt cannot authorize duplicate dispatches or spend several future attempts. Preserve conservative handling of interrupted grant publication; uncertain state cannot fabricate permission. Continuations keep the same gate key and attempt number. Neither `run-waiting`, tracked-drive recovery, nor routine observation consumes an additional attempt.

Preserve claim attribution, observe-only behavior for unattributed calls, terminal-result behavior, and explicit human-halt precedence. A build that records `run-halted` cannot acquire more build repairs through an outer retry. An explicit human resume uses the existing resume path; this change creates no automatic re-arming path after exhaustion.

Keep the existing `gate-retry-once` disposition as a grant for exactly one next dispatch; it is not permission to loop locally. Generalize maintained caller instructions so the facade can grant another single dispatch on a later eligible attempt when a non-default budget allows it. Do not infer permission from counters, process exits, or child prose. Carry usage and maximum through structured diagnostics without breaking existing report-line parsing.

Handle older durable records explicitly: preserve their original fixed allowance and already-consumed state when readable; otherwise refuse safely with an actionable diagnostic. Never reinterpret an older consumed marker as an unused configurable budget. Version state formats as required.

## Documentation and implementation scope

Update canonical config/schema surfaces, typed context, gate accounting, build/repair workflow contracts, and parent gate instructions together. Derive affected sites by repository-wide search; include embedded assets and generated harness surfaces through their existing generators. Replace fixed-limit language only where it describes these two budgets. Accepted ADRs and historical specs/results remain immutable; record any required replacement decision through the existing ADR workflow at implementation time.

## Acceptance and validation

- Defaults resolve to build 4 and run 2; explicit overrides retain precedence and provenance. Test 1 and values above each default, and reject invalid types and non-positive values.
- With build limit 4, a sequence of three red runs and a fourth green succeeds; four red runs halt and a fifth run is never launched. Early green stops further repairs. Limit 1 never dispatches a repair-and-rerun cycle.
- With run limit 2, one eligible retry is possible and the next incomplete result stops. Limit 1 grants none; a larger configured cap grants exactly the corresponding number of distinct eligible attempt transitions.
- Repeat and concurrent observations cannot duplicate grants or exceed either bound. Resume and ownership-transfer tests preserve usage and do not charge continuations twice. Config changes during an owned scope do not rewrite its snapshot.
- Explicit halts, invalid or unattributed ownership, infrastructure failures, and unavailable test results remain non-retryable under the existing rules. Build-off behavior and exact-HEAD evidence remain correct.
- Validate legacy durable-state handling, typed context propagation, diagnostics, and generated caller/build instruction consistency. Mutation-test any new structural guards so removing the protected behavior makes them fail.
- Run the configured whole-suite build gate from the source checkout through the Go suite runner, and read its budget report, following repository rules.

## Alternatives considered

Keeping both hardcoded bounds leaves the user's problem unresolved. A shared limit would conflate test-repair iterations with entire implementation retries. Failure-only or retry-only counting makes configuration harder to explain. Unbounded or progress-sensitive retries add policy and detection complexity the user explicitly excluded from this first change.
