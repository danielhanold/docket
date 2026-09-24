<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0444 — Reconcile uncertain publication records so cancellation and resume can finish](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-24-0444-reconcile-uncertain-publication-records-so-cancellation-and.md)**
<!-- docket:backlink:end -->

# Reconcile uncertain publication records

## Intent and scope

Fix the case where publication returns an unknown outcome, an identical retry succeeds, and cancellation still waits forever on the first attempt's journal entry. Once a later admission in the same epoch with an identical publication identity has completed, and all other obligations are accounted for, cancellation must finish and the existing resume path must admit its one replacement. Apply the same rule to successful-run closeout so this repair does not leave the identical bookkeeping defect there.

This is a bounded extension of the existing journal. No implementation or implementation plan is part of grooming.

## Investigation and prior decisions

Design baseline: main `442770e11bd05baf6200bec53e9c4757622811a6`, inspected on 2026-09-23. The separate-session reproduction is user-reported; this grooming verified the causal path in source, not by replaying actions against a live run.

- `internal/app/rungate_epoch.go`, `AdmittedMutation`, stores only operation key and status. There is no repository/ref/head or PR request identity from which to relate an earlier attempt to a later one.
- `internal/app/rungate_fence.go`, `admitWorkflowMutation`, atomically checks the epoch and appends an admission. Its callback updates only that appended index. A retry therefore cannot settle an older uncertain entry. `mutationJournalStatus` conservatively maps external failure/interruption to uncertain. The callback's write is currently best-effort.
- `PRPublish` journals immediately before `EnsurePullRequest`; `WorkspacePublish` does so before `PublishHead`. A repository-wide search of `admitWorkflowMutation` also found the transaction admission hook: its immediate completion has a separate existing contract and is outside this repair.
- `reconcileEpochTeardown` reloads the journal but merely checks statuses. `verifyTerminalEpochQuiescence` and `accountCompletionMutations` have the same status-only predicate. `validateResumeQuiescence` consumes the terminal check before resume. Existing fencing, participant accounting and slot retirement are already present.
- `internal/githubcli/ensure.go` already owns PR adoption and post-mutation verification: PR publication verifies repository, head branch/commit, base, ready/open state, title and assembled body. `internal/workspace/publish.go`, `PublishHead` and `reprobeAfterPush`, already re-probe the exact remote ref against the intended full commit. A retry that reaches `completed` has therefore already had its postcondition verified by the adapter that performed it.
- The existing `TestInFlightMutationReconcilesBeforeCancelled` manually calls the original completion callback. It does not cover an uncertain result followed by an independent successful retry or a fresh cancellation process.

Reviewed changes: 0313 introduced probe/act/verify and lost-response adoption; 0375 introduced epoch fencing and the admitted-mutation accounting requirement; 0437 added pending-launch accounting; 0435 added ownership retirement and terminal-cancel/resume checks; 0441 added successful closeout. All are done. Active 0422 concerns retry-grant accounting, not publication reconciliation, and creates no dependency. Recent 0443 is unrelated gate-operation documentation.

ADR-0118 requires full task/process/mutation accounting before cancellation and resume. ADR-0124 requires successful closeout to use observation-only evidence and preserve the same exclusion. Neither authorizes treating missing evidence as success. The proposed repair fulfills these decisions; it does not change lifecycle or cancellation policy and needs no new ADR for a separate coordination mechanism.

Relevant learnings reviewed: `groomed-root-cause-is-a-hypothesis`, `idempotency-keying`, `probe-error-is-not-clean-absence`, `cas-re-read-fresh-origin`, and `relax-the-policy-before-building-the-workaround`. The blocker here is missing evidence/reconciliation machinery, not a configurable approval requirement.

## Alternatives and decision

1. Clear earlier entries whenever any later publication succeeds, or when no process is running. Rejected: neither identifies the original effect; a different head, repository, base or body can succeed independently.
2. Extend the existing admission entry with immutable publication identity, and settle an uncertain entry only when a later completed entry in the same epoch carries an identical identity. Selected: the completed retry's adapter already verified the exact postcondition, so the match is a local journal comparison with no new remote observation.
3. Re-probe GitHub/Git during cancellation to settle uncertain entries, including ones with no successful retry. Rejected for this change: the reported failure always involves a completed identical retry, and live re-observation adds a read-only adapter, network error handling and destination-binding concerns that nothing reported requires.
4. Add a separate effect ledger, background reconciler, force-clear command or new retry policy. Rejected: no evidence justifies these mechanisms. Existing cancel and keyed-verdict retries are adequate reconciliation entry points.

A retry continues to have its own admission. There is no journal deduplication or cross-attempt deletion. The original entry is settled only by an exact identity match with a later completed entry, never by the existence of some later success.

## Design

### Preserve original publication identity

Add an optional, typed publication descriptor to `AdmittedMutation` in the existing epoch record. Persist it with the admission under `epochCAS`, before entering the external publication adapter. Retain the append-only index as the entry locator within the exact epoch; no separate random effect id, ledger or lifecycle is needed.

For PR publication, capture the already-resolved GitHub host/owner/name, exact head branch and full requested commit, effective base branch, and deterministic digests of the requested title and fully assembled body. The ready/open requirement follows the existing operation. The PR number is unavailable before a create and must not be required. Do not use `PullRequest.Version` as the identity: `computeVersion` includes the server-assigned number. Reuse the existing digest conventions where suitable, with unambiguous field boundaries and exact bytes.

For workspace publication, capture canonical repository identity, remote name, exact feature ref and the full intended commit used by publication. No remote-destination binding is recorded: both entries are compared within one epoch of one run, and this change does not attempt to detect a remote repointed between attempts.

Captured head identity must agree with the head actually handed to the publication operation. Preserve the existing workspace reinspection/locking boundary; do not reconstruct the original target later from a mutable manifest, current HEAD or current PR prose.

Keep epoch schema v1 with additive optional fields, following the compatibility pattern used by 0441. Existing entries still decode. Validate descriptor completeness and operation/descriptor agreement before trusting it. Never emit body bytes, raw remote URLs, credentials or argv in journal records or findings.

### Match against a completed identical retry

Add one narrow matching helper over the journal, used by the existing accounting seams. It is a pure function of the durable epoch record and performs no Git or GitHub calls.

- An entry is eligible when its status is `uncertain` and it carries a valid descriptor.
- It is settled by a match: a later entry (higher index) in the same epoch with the same operation, a valid descriptor equal field-for-field to the original's, and status `completed`.
- No match, a missing or malformed descriptor on either side, a differing field, or a later identical entry that is itself `admitted` or `uncertain` leaves the original unaccounted.

The recovery target is an `uncertain` publication whose adapter invocation has returned, in the reported identical-successful-retry case. A still-`admitted` entry is not proof that its producer has returned and remains pending. An uncertain entry with no completed identical retry also remains pending, even if the remote happens to show the desired effect; recovering that case, a killed producer, or a failed final callback write that leaves only `admitted` is outside this bounded repair. Existing task/process/launch accounting remains independently necessary.

### Settle the existing obligation durably

After finding a match, use `epochCAS` to re-read and update only the original entry in the expected epoch, re-checking index, operation, immutable descriptor, eligible status and the matching completed entry. Advance uncertain to completed, preserving the entry and identity. Already-completed is idempotent. Never downgrade completed to uncertain or overwrite unrelated entries/participants/state when a callback or another cancellation races.

A failed write is not a completed obligation: retain exclusion and report a bounded finding. Repeating the same cancel/keyed verdict re-evaluates the match and retries the existing write. An interruption after a successful write is harmless. An appended unrelated entry cannot be cleared by an older snapshot.

Wire this into cancellation's existing re-enumeration/accounting path, and reload the durable journal before terminal cancellation or slot retirement. A successful retry without cancellation can otherwise leave 0441 stuck too, so the attributed keyed successful-closeout path uses the same matching helper and persists only journal-derived evidence. It must still stop no task, launch no mutation, and settle no never-launched reservation. Read-only `RunVerify` and unattributed verdicts gain no writes.

Terminal-cancel repair and resume consume the same completed-journal predicate. Resume does not publish, repair remote state or bypass the cancelling predecessor. Existing one-replacement reservation, cancellation authority, slot ownership checks and budgets remain unchanged.

## Acceptance and verification

Use existing app fixtures; no live PR mutation or remote observation is needed for automated acceptance.

1. Reproduce uncertain PR publication followed by an identical successful retry. Both admissions exist; with no live participants, cancellation matches the retry, durably completes the original entry, reports cancelled, and the existing resume operation admits exactly one replacement. Assert the original record changed, not merely the result string.
2. Cover the analogous workspace publication case. An uncertain entry with no completed identical retry stays pending and cancellation stays `cancellation-pending`.
3. Cover successful-run closeout through the real keyed-verdict path using the same matcher; prove it performs no cancellation or remote mutation. An unattributed/read-only path must not settle entries.
4. A later completed entry that differs in repository, remote name, ref, full head, base, title or body cannot clear the original PR/branch obligation. Neither can an identical later entry that is `admitted` or `uncertain`, an identical completed entry with a lower index, or one in a different epoch. A subsequent identical completed retry permits the same pending cancellation to finish.
5. Legacy missing descriptors, malformed descriptors, unknown operations and still-admitted entries remain pending. Completed legacy entries remain accepted. No forced migration or fabricated history.
6. Inject persistence failure, interrupted reconciliation, concurrent callbacks/cancels and appended unrelated admissions. Verify completed never regresses, the exact epoch/entry is checked, failure retains exclusion, and repeated cancellation converges once writes succeed.
7. Verify production wiring reaches the matcher, not just an injected helper. Capture adapter calls to prove reconciliation issues no Git or GitHub calls. Seed sensitive-looking body/URL values and verify the journal and diagnostics do not leak them.
8. Preserve all existing admission-fence, launch accounting, retirement, standalone/no-epoch and resume protections. At implementation, run the whole configured `build.test_command` from source through the Go suite runner and inspect its budget report; mutation-test any newly introduced guard. Grooming itself changes metadata only.

## Explicit limits

No new CLI command, configuration, daemon, persistent store, retry allowance, ownership policy, publication rollback, journal compaction or bulk historical cleanup. No live Git/GitHub re-observation during reconciliation and no remote-destination binding. No expansion into metadata transaction outcomes or finalize publication receipts. Historical entries lacking identity cannot be safely reconstructed automatically. An uncertain publication with no completed identical retry in the same epoch, and an admission lacking proof its producer returned, can still require human investigation; this change must describe that limit honestly rather than weakening cancellation safety.
