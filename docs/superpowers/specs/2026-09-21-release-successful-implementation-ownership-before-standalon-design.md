<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0441 — Release successful implementation ownership before standalone finalize](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0441-release-successful-implementation-ownership-before-standalon.md)**
<!-- docket:backlink:end -->

# Release successful implementation ownership before standalone finalize

## Intent and scope

After a keyed implementation run is verified complete and its registered work is proven settled, a standalone finalize gate must be able to use the same worktree without presenting the implementation epoch or requiring a human cancellation. Preserve exclusion while implementation still has work outstanding.

This is a lifecycle fix in existing machinery. It does not redesign coordinator topology, run attribution, cancellation, budgets, or finalize continuation. No dependency or stack is required: changes 0375, 0407, 0435, and 0437 are done. Related proposed change 0433 concerns coordinator topology; this fix must work on the currently shipped route without waiting for it.

## Investigation and prior decisions

Design baseline: main 329fa0a0729a72707916450b9609ef91544ecf3b.

- `RunGateVerdict` in internal/app/rungate_verdict.go verifies the bound change through `RunVerify`, then `persistGateVerdict` saves a terminal run-complete report best-effort. It neither fences nor retires the epoch.
- `ReleaseWorktreeExecution` deliberately retains RunEpochID. `reserveWorktreeExecution` checks an epoch mismatch before considering released state. `processFinalizeGate.RunLocalGate` starts its standalone drive without an epoch, reproducing the incompatible handoff in the code.
- Change 0375 and ADR-0118 established worktree-wide exclusion and explicit cancellation. Change 0437 added `epochLaunchGate`, delayed-ticket/relaunch fencing, and `Driver.ReconcileEpochLaunches`. Its scope explicitly excludes normal successful-run retirement.
- Change 0435 added `RetireWorktreeExecutionEpoch`, `retireWorktreeSlotOwnership`, historical cancellation repair, and resume quiescence checks. Its design also explicitly excludes successful-run retirement. Reuse these ownership checks and write ordering.
- Cancellation accounting is not a pure observer: `reconcileEpochTeardown` cancels/stops participants; `ReconcileEpochLaunches` may stop identified processes and settle a reserved relaunch. `verifyTerminalEpochQuiescence` does not re-prove native participants. Calling these wholesale from a success report would silently turn completion into cancellation.
- `EpochParticipant` records registration but no terminal observation. Codex entry already observes the exact thread/turn through `Client.waitTurn`; the CLI currently discards that lifecycle evidence after reporting it. The guardian's owner-complete.marker is also written on normal return/handoff and cannot prove successful run completion.
- `findEpochByWorktree` currently returns historical matching epochs without a state filter. Merely clearing the admission slot would leave standalone workflow mutations encountering the old epoch.
- ADR-0111 requires claim-receipt attribution, ADR-0087 distinguishes unprovable liveness from proven termination, ADR-0095 supplies exact process terminal evidence, and ADR-0105 keeps finalize continuation in its existing receipt. Preserve all four.
- Relevant learnings reviewed: verify-the-claim, yielded-worker-return-closes-every-door, transient-resource-lifecycle, assert-pins-outcome-not-mechanism, and relax-the-policy-before-building-the-workaround. This blocker is durable ownership state, not a configurable approval policy. Historical counts reported by the originating investigation (15 successful reports with active epochs; 14 released slots retaining epochs) were not independently recounted and are not a current-blockage count.

## Chosen approach and YAGNI rationale

Extend the existing epoch lifecycle and keyed verdict boundary. Reuse the epoch lock/CAS, participant list, mutation journal, drive reservation and process records, admission lock, and ownership-checked retirement. Do not add another registry, completion command, background sweeper, retry layer, or lease.

Two small extensions are necessary:

1. Add `completing` and `completed` states to the existing epoch record. Completing is a durable fence with unfinished accounting; completed means retirement finished. Active cannot prevent late admissions. Cancelling/cancelled assert explicit Stop and feed replacement-resume behavior; superseded asserts a replacement exists. None truthfully represents successful completion or its interrupted cleanup. Do not encode success as cancellation.
2. Persist exact native-participant terminal observation in the existing participant entry, using the current adapter's observation boundary. Registration alone and the owner-complete marker cannot establish quiescence. This is lifecycle evidence, not a new generic task ledger or a user-supplied completion assertion.

Apply the existing versioned-record compatibility discipline to these additions. Missing historical terminal evidence means unproven, never implicitly complete. Preserve existing readable cancellation history.

Rejected alternatives: clear the epoch on every test release (permits overlap between drives); give finalize the old epoch (extends stale authority); call run.cancel automatically (changes authority and stops work); inspect only the successful PR/report (does not prove writers stopped); create a new ownership service (existing records already hold the required coordination).

## Successful completion flow

Only the attributed, keyed `RunGateVerdict` path may drive completion. Keep `RunVerify` read-only and unattributed verdicts observe-only. Child prose, test success alone, a terminal mirror flag, or an epoch locator supplies no extra authority.

1. Resolve the existing confirmed claim binding and current RunVerify postconditions. Verify key/repository/epoch/change/worktree agreement using established checks. A success-shaped historic report does not skip live verification.
2. Once success is verified, CAS active to completing under the existing epoch lock. Recheck state/binding where necessary at this serialized decision. A cancelling, cancelled, superseded, or mismatched run is never relabelled successful. A completing replay resumes this same closeout.
3. Completing rejects new native registrations, mutation admissions, starts, delayed start tickets, successors, takeover, and automatic/recovered relaunches for this epoch. Extend the existing state checks/revocation resolver; do not introduce a parallel fence. Existing admitted operations may still record their terminal evidence and reconcile their journal entries.
4. Outside the epoch lock, prove existing obligations settled. Reuse cancellation's attribution, enumeration, reservation resolution, process evidence, and mutation predicates through narrowly factored observation helpers. The success path must not invoke native cancellation or signal a live process. A live, busy, pending, uncertain, or unreadable obligation blocks completion. Re-enumerate participants and mutation entries before retirement. Do not equate a released slot or a terminal drive outcome alone with full accounting.
5. For native tasks, record terminal observation against the exact registered handle and observed turn at the adapter boundary, after that invocation's terminal response and transport teardown are accounted for. Record terminal failure as termination evidence too; RunVerify independently decides implementation success. A yielded return, final-message text without the matching terminal event, transport loss, or failed probe is not termination evidence. Completion of an existing participant is allowed after the completing fence; registering or reopening work is not. Preserve missing-adapter evidence as unresolved.
6. Require all epoch-linked launches/reservations settled, all registered execution processes terminal with proven teardown, all native participants terminal-observed, and every admitted mutation completed. Share drive traversal and ownership predicates with existing cancellation accounting; provide observation-only behavior for success rather than copying a second inventory. Outstanding never-launched tickets/relaunches remain pending until their established fenced settlement completes; do not invent timeout expiry.
7. Retire only this epoch's released slot through `RetireWorktreeExecutionEpoch` and the existing expected-token/expected-epoch checks. Preserve history. An absent/already-detached slot is acceptable only after independent old-run accounting. A foreign successor is left untouched; do not acquire its token or stop it.
8. Persist completing to completed, then durably save the successful gate report. Only then return gate-done run-complete. Completion-path persistence failures must be reported, not hidden by the existing best-effort report save. Preserve the read-only run verification verdict as a fact separate from ownership closeout.

Keep existing lock ordering. Never hold epoch/admission locks across process observation, transport shutdown, network work, or a per-drive claimant probe. The completing fence and existing reservation accounting close the pre-fence launch window.

## Failure, replay, and consumer behavior

Unresolved accounting or a persistence failure uses the existing gate-stop / gate-unavailable channel with a bounded reason and a remedy to repeat the same keyed verdict after the named evidence is settled. It grants no re-dispatch, suite attempt, retry permission, or budget reset. No new automatic polling policy is introduced.

Before retirement fails, ownership remains. If retirement succeeds and the final epoch/report write fails, completing remains fenced and replay revalidates evidence, accepts safe prior detachment, and finishes. Never restore ownership or overwrite a successor. Repeating a fully completed verdict is idempotent and does not depend on old process scratch directories remaining forever; the durable completed state is the closeout receipt.

Exclude fully completed epochs from ambient worktree-owner lookup so standalone finalize's gate and subsequent supported mutations can proceed. Completing remains an owner until closeout finishes. Explicit references to completed epochs remain revoked. Audit existing epoch consumers by repo-wide search, including participant registration, mutation fencing, launch/relaunch/recovery/takeover, guardian, cancellation, change/epoch lookup, and resume. Resume cannot turn completing/completed into a cancelled predecessor or reserve a replacement. Explicit human cancellation may win from completing using its existing authority and accounting; completion must then lose without reporting success. Cancelling a completed run is a no-op/refusal with a completed-run explanation, never a state regression.

Historical active epochs with run-complete reports can pass through the same keyed completion flow when current authority, success, and terminal evidence are provable. Missing evidence produces a specific diagnostic and the existing explicit cancellation remedy; do not fabricate receipts, bulk-edit history, or add migration heuristics. Existing confirmed-cancellation repair remains unchanged.

## Acceptance criteria

1. An epoch-backed implementation fixture reaches verified success with a released, epoch-owned slot. Before closeout, an epoch-less finalize start gets stale-run-epoch; after closeout, the production finalize path admits and advances its gate using its own continuation receipt. Its subsequent workflow mutations are not trapped by the completed implementation epoch.
2. Ordinary test release preserves RunEpochID and still rejects foreign/epoch-less starts between drives. Successful closeout preserves slot history and consumes/resets no suite or outer retry budgets.
3. Success verification alone cannot retire a run with a live/unobserved native task, executing process, busy launch claim, delayed ticket, pending relaunch, late participant, or admitted/uncertain mutation. The success path sends no cancellation or stop signals.
4. Adapter terminal observation is persisted for the exact participant; transport loss, yielded text, mismatched turn, marker-only completion, and terminal-evidence write failure cannot satisfy it. Existing native cancellation behavior remains intact.
5. Fenced/completed epochs cannot launch, recover, mutate, register, take over, or resume as replacements. Concurrent completion and cancellation has one valid lifecycle outcome. Unattributed/read-only verification never changes ownership.
6. Inject fence, retirement, final epoch, participant-evidence, and gate-report write failures; replay converges after recovery. Test interruption before/after slot retirement, duplicate verdicts, and a successor reserving after safe detachment. Assert exact state and ownership, not just report text.
7. Historical success can be repaired only with sufficient evidence. Unprovable history fails closed; completed receipt replay is safe after normal scratch cleanup. Existing cancellation/resume regression cases stay green.
8. Mutation-test the completion preconditions, ordinary-release fence, successor protection, and persistence-error guards. Run the full source-resolved build suite through the configured Go runner and inspect the budget report.

## Exclusions and implementation boundary

No implementation or implementation plan is part of grooming. At build time record the success-lifecycle extension to ADR-0118 through the ADR workflow without rewriting accepted history. Keep changes limited to the existing run gate, epoch/participant lifecycle, shared accounting, admission retirement integration, current adapter observation hook, their tests, and directly affected caller/schema documentation. No coordinator-topology work from 0433, broader mutation-owner redesign, new recovery framework, configurable completion policy, cross-machine ownership, or cleanup of arbitrary historical runs.
