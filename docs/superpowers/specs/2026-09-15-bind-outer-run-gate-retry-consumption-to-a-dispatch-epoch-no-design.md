<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0422 — Bind outer run-gate retry consumption to a dispatch epoch, not each observation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0422-bind-outer-run-gate-retry-consumption-to-a-dispatch-epoch-no.md)**
<!-- docket:backlink:end -->

# Change 0422: reuse the same retry marker for repeated attempt observations

## Goal and trust boundary

Fix repeated observations of one unfinished attempt consuming successive retry allowances. Keep the existing gate, ownership proof, retry-marker storage and grant-before-dispatch ordering.

The human selected: if launch or response delivery is uncertain, the retry stays spent and the coordinator stops. Automatic recovery or refund is out of scope.

The coordinator already knows which native dispatch it launched and must collect its terminal result before requesting its verdict. Retain that trust boundary: Docket verifies change ownership and grants once per supplied attempt number. It does not independently certify that a child process started or finished. Enforcing child admission or exactly-once launch is separate work, not a guarantee of this fix.

## Existing mechanism

`ConsumeGateRetry` in `internal/app/rungate_store.go` already accepts an attempt number and exclusively creates `retry-consumed-<n>` to grant that numbered attempt's successor once. `TestConsumeGateRetryPerAttemptCAS` already covers repeating the same number.

The defect is in `RunGateVerdict`: it derives the observed attempt from `1 + GateRetryUsage`. A grant changes that count, so a later observation of the same dispatch targets another marker. Reading the count earlier only narrows the race.

Fix how the observed attempt reaches the existing reservation mechanism. Do not replace that mechanism.

## Proposed change

Add positive-integer `--attempt <n>` to attributed `run.gate-verdict`, threading it through the app call. The default is always 1, never a value inferred from grant counts, timestamps, run epoch or change state. Reject an explicitly invalid integer and use with `--unattributed`.

Keep existing ownership checks and run-status precedence. On the eligible quiescent incomplete path, pass the supplied number to the existing per-attempt reservation logic. Before allowing n greater than 1 to grant, require the exact predecessor grant marker for n-1; the legacy bare marker remains equivalent to attempt 1's marker. Missing or unreadable predecessor evidence cannot grant. This is a check of existing storage, not a new ledger.

An existing marker for n returns the existing non-granting `gate-stop ... run-incomplete` response. Never replay a grant from saved disposition text. Completion, human halt, continuations, tracked-drive takeover and unattributed observations retain their existing behavior. Do not describe a consumed retry as `no-attributable-claim`; ownership has not disappeared.

Keep the snapshotted `AttemptLimit` and hard bound. `AttemptsUsed` reports the observed attempt ordinal on retry-accounting responses. Add only one result field, `retry_attempt`: n+1 on a newly won grant, absent on every non-granting result. The facade computes it; callers do not calculate allowances or infer the next dispatch number from marker counts.

## Caller changes

The coordinator associates attempt 1 with the initial dispatch. After collecting that dispatch's terminal output through the existing native wait mechanism, it calls the keyed verdict with its retained number.

On `gate-retry-once`, capture `retry_attempt` from JSON and associate that label with exactly one authorized retry dispatch. Request that attempt's verdict only after observing the actual dispatch's terminal result. Repeated observations reuse the label. Continuations keep it and use the existing continuation protocol. The key, context, run epoch and claim remain unchanged.

A lost grant response or failed/uncertain launch earns no replacement dispatch. If the coordinator loses the association between number and dispatch, stop; never guess or reconstruct launch history from markers. A marker proves reservation, not execution.

Carry the label in existing parent dispatch assignment/state. No new child startup step, child argument, task registry or adapter callback is required. Update parent-facing generated instructions and actual executable verdict callers discovered by repository search. Capture JSON for the additional field; report-line tokens and shapes stay unchanged.

Key-only calls always observe attempt 1. They can win its first grant, as today, but repeated calls cannot advance to another allowance. Later dispatched attempts require explicit identification to use higher limits. If an older binary rejects the new flag, the updated caller stops; it never retries without the flag.

## Scope

Keep GateRecord schema v4, marker names, legacy-marker interpretation, gate arming, cancellation and human-directed resume semantics. No stored state is migrated. Exclusive marker creation remains the point where a retry is spent, before dispatch permission is returned.

In scope: verdict argument handling, predecessor-marker validation, the additive result field and catalog/schema exposure, parent label propagation, regression tests and documentation of the corrected rule. Record the successor decision to ADR-0115 during implementation; preserve its accepted historical body.

Out of scope: versioned attempt ledgers, random tokens, child admission, new CLI operations, cancellation readers or lock redesign, claim/workspace resume redesign, launch registration/recovery, build/finalize budget changes, and repairs to older binaries' obsolete behavior.

Change 0421 is done. ADR-0111 remains the attribution contract; ADR-0118 remains the cancellation contract. Reconcile overlapping parent-instruction changes 0425/0426 at build time. Change 0427's recovery-worktree fix is separate. No dependency or stacked base is added merely for touching nearby code.

## Acceptance and validation

1. At limit 4, repeated verdicts for attempt 1, including omitted `--attempt`, grant once and leave one marker. Repeating an older attempt after observing a later one still cannot grant again.
2. Explicit attempts 1, 2 and 3 each grant once after their predecessor reservation; attempt 4 cannot grant. Preserve limits 1 and 2. A future attempt without its predecessor marker cannot grant.
3. Concurrent observers of the same number, including readers starting after the first grant finishes, produce one grant total. Do not rely on synchronized observer timing.
4. Losing a response after marker creation or failing to launch leaves the marker spent. Repeating the old verdict grants nothing. Recovering a coordinator without its dispatch association never falls back to count-based inference.
5. Preserve ownership, replaced-claim, halt and continuation coverage. Add only the argument-propagation assertions needed to demonstrate unchanged behavior on those paths.
6. Cover CLI parsing, the predecessor's legacy-marker case, the additive result field and generated parent wiring. Mutation-test replacing the supplied number with count-derived numbering and removing the predecessor check; prove the mutations landed and the relevant tests fail.
7. Update the counted-limit verdict test to supply distinct numbers: its current loop merely repeats one observation and encodes the bug as expected behavior. Retain existing store CAS tests.
8. Run the configured complete build suite through the Go runner from the feature source checkout and inspect its budget report.
