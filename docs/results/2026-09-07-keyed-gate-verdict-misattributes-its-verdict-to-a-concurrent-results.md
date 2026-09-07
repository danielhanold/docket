<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0407 — Keyed gate-verdict misattributes its verdict to a concurrent loop's change id under parallel implement-next runs](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0407-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent.md)**
<!-- docket:backlink:end -->
# Keyed gate-verdict misattributes its verdict to a concurrent loop — results
Change: #407 · Branch: fix/keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent · PR: <pending> · Plan: docs/superpowers/plans/2026-09-07-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent.md · ADRs: 75, 111

## Verify (human)

- No manual checks required beyond the automated suite. The whole suite (`go run ./cmd/docket development test`) is green at the PR head; the concurrency/ownership behavior is covered by the deterministic interleaving matrix and the race-tagged CAS test below.

## Findings

Controlled reproduction and mutation evidence (spec acceptance §8; every probe run with `-count=1`, uncached):

- **Task 1 — bind-once CAS.** Mutating `ReserveGateClaim`'s `os.Link` hard-link CAS to a non-exclusive `os.Rename` reddened `TestReserveGateClaimIsBindOnce` (`want binding-conflict, got <nil>`). Schema-v2 records fail closed as `corrupt-record` (`TestGateSchemaV2RecordFailsClosed`).
- **Task 3 — claim boundary.** (a) hard-coding `gateHash = ""` after hashing reddened `TestClaimGateContextReservesAndConfirms` (digest/receipt asserts); (b) deleting the `ReserveGateClaim` call reddened `TestClaimGateContextConflictRefused`.
- **Task 5 — verdict ownership.** (a) re-introducing before-set/epoch inference in the no-binding zero-proof branch reddened `TestVerdictOwnershipIgnoresBeforeSetAndEpoch` and `TestVerdictNoBindingNoProofIsNoAttributableClaim`; (b) dropping the `GateContextHash == rec.ChildContextHash` filter reddened `TestVerdictNoBindingNoProofIsNoAttributableClaim`; (c) dropping the claim-instance continuity comparison reddened `TestVerdictClaimReplacedStops`.
- **Deterministic interleaving matrix** (`internal/app/rungate_ownership_test.go`): two gates armed before either claim, all four completion/verdict orderings — each gate reports only its own change id, sibling id absent from the report line.

Whole-branch review (docket-review-deep) returned clean — 0 blocker, 1 important, 2 minor, all about this branch's own diff; all three fixed in-branch (see PR disposition table). Their remediation added:
- `TestVerdictUnconfirmedReservationSiblingContextHashIsNoAttributableClaim` — mutation-confirmed: dropping `&& p.GateContextHash == contextHash` in `gateProofForClaim` reddens it (finding 1).
- `TestRaceIntegrationAppConcurrencyReserveGateClaimBindsExactlyOnce` — race-tagged, 16 goroutines, exactly one binding wins; mutation-confirmed against `os.Rename` (finding 2).
- Comment accuracy fix in `ReserveGateClaim` (finding 3).

ADR: **ADR-0111** ("Run-gate attribution binds a dispatch to its successful claim transaction") supersedes **ADR-0075**'s snapshot/cardinality attribution while preserving its conservative safety principle and untouched halt semantics.

## Follow-ups

- None required by this change. Related work stays out of scope and separate: change 0345 (slash-command attribution context), change 0405 (test-drive prepare-scope/start handshake), change 0406 (release-determinism flake). This change supplies the claim-binding primitive that 0345/0405 may reuse.

## Notable plan deviations

- **Test partition (change 0333).** The plan assumed several verdict attribution/mapping/observe/concurrency tests lived in `rungate_verdict_test.go`; change 0333 had moved them into integration-tagged files (`change_integration_test.go`, `app_concurrency_race_integration_test.go`). Task 5 applied the plan's intent across both locations: deleted the four before-set/epoch/cardinality inference tests, converted the fresh-attribution mapping tests to the resume-verified `gateMintAttributed` shape (which `resolveGateOwnership` delegates to `RunVerify` unchanged), and preserved every load-bearing safety test (retry-CAS, continuation-ordering, observe-isolation, verified-resume).
- **Vocabulary registration (Task 3 completion gap).** Task 3's two new `claim_dispositions` consts (`gate-context-invalid`, `gate-context-conflict`) were not registered in `internal/app/schema_vocab.go`'s `claim_dispositions` vocabulary; `TestVocabularyConstCompleteness` caught it. Registered in a follow-up commit before the full-suite gate.
- **Bind-once fast-path dropped (Task 1).** The plan's optional pre-read short-circuit in `ReserveGateClaim` was omitted so the `os.Link` CAS is the sole match-or-conflict authority — this keeps the bind-once guard mutation-testable (a pre-read would catch the serial conflict before the CAS and mask a broken CAS). Behavior is identical: idempotent replay → nil, different claim → `binding-conflict`.
