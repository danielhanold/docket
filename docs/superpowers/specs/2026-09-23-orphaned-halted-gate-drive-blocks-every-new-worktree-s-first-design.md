<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0446 — Orphaned halted gate drive blocks every new worktree's first gate admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0446-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md)**
<!-- docket:backlink:end -->

# Change 0446 — Keep historical bookkeeping from blocking current execution

## Required outcome

Docket must be able to implement and finalize changes despite accumulated records from older runs. A start may be refused for an execution/owner conflict only when evidence connects the conflict to the requested canonical worktree or the explicitly requested run. Historical uncertainty is not repository-wide ownership.

The same rule applies to cancellation, resume, successful ownership closeout, and the transition into finalize. Fix the class of false blockers, not the particular HALTED cause, schema, change id, age, or temporary path in the incident. A new exception for `stopped-not-initiated` is not an acceptable implementation of this spec.

Preserve the real exclusion guarantee: two executions cannot run concurrently in one worktree, an active workflow owns its worktree between sequential tests, and an old fenced execution cannot launch after a replacement is admitted. Genuine unresolved obligations of the affected worktree/run must be accounted for, with an exact locator and applicable recovery action.

## Grounding and prior work

Inspected main `442770e11bd05baf6200bec53e9c4757622811a6` and metadata revision `d1c7776dde985f679d1f45b97186a9079f8ca3fb` on 2026-09-23. The original change record at that metadata revision preserves the incident, quarantine locator, and manually edited epoch/backup details. No live runtime records were changed during grooming.

- **#0375 and ADR-0118:** the original spec's compatibility section says to inventory existing drives **“for that canonical worktree.”** It also requires quiescing old executors during a protocol upgrade; it does not promise that old binaries honor new fences. The current `reserveWorktreeExecution` instead calls a repository-wide `inventoryLegacyDrives` on every absent worktree slot. Any retained record vetoes admission. Every newly allocated feature worktree encounters this condition.
- **#0428 and ADR-0120:** `loadHistoricalDrive` separates historical assessment from executable compatibility, and `classifyLegacyDrive` lets PASSED/FAILED bypass removed paths. Its remaining rule treats an unresolvable historical path as possible ownership of every requested worktree. An unreadable record or a HALTED record without temporary evidence can therefore block unrelated work indefinitely. Keep the historical reader and manual cleanup; correct their authority boundary.
- **#0437:** `accountEpochLaunches` reads every drive, reports unreadable records or lost linkage against every cancelled epoch, and reaches `reconcileEpochDrive`'s existing terminal shortcut only after linkage resolution. `resolveDriveEpoch` is appropriate for authorizing a particular drive's new execution; using every failure of that predicate as an obligation of every epoch is not. Scopeless linkage depends on a slot token that legitimate later executions replace. The review fix intentionally protected genuinely attributable pending launches; preserve that protection without its global inference.
- **#0435:** `runCancel`, `repairTerminalEpoch`, and `validateResumeQuiescence` already share accounting and ownership-checked retirement. Repair those shared inputs. Keep exact-token/epoch retirement and protection of a successor's slot.
- **#0441 and ADR-0124:** success uses the same launch census in observation-only mode, then retires the released slot. Consequently unrelated historical findings can also prevent success closeout and block finalize. `accountCompletionSlot` additionally reopens temporary run evidence even after the matching slot records released state. Preserve durable completion facts instead of allowing scratch cleanup to undo them.
- **#0439:** a completed raw launch intentionally retains its slot until stop under the current contract. `WorktreeAdmissionRefusal` and `rawStaleEpochRefusal` can refuse before authoritative admission is reached. A repair only inside the final reserve function would be unreachable for those callers. The revised scope explicitly includes bounded reconciliation of already-finished incumbents through the common admission path, including raw runs.
- **Owner selection:** `findEpochByWorktree` returns the first matching epoch and excludes only completed epochs. A superseded predecessor can mask its live replacement. `FindEpochByChange` already resolves replacement chains rather than relying on directory order. Extend that established approach to worktree ownership; do not add an owner registry.
- **Existing precedents:** `findEpochByWorktree`, `scanEpochsByID`, and `FindScopeDriveIDs` already distinguish unmatchable historical siblings from errors reading a specifically required record. `epochOwnsWorktree` matches stored canonical identity directly or through a resolvable alias; it does not make a deleted path match every worktree.
- **Incident evidence:** the preserved schema-4 HALTED drive for #0368 has one stopped attempt and deleted runtime evidence. Separately, a read-only census of the present registry found 623 PASSED/FAILED records; obsolete linkage explains the reported cancellation failures, but grooming did not replay cancellation. #0444 remains the blocked consumer, with its independent publication-journal fix and uncommitted work preserved.

Relevant learnings: verify the root-cause hypothesis against code; do not build recovery machinery around an unnecessarily broad policy; do not mistake missing evidence for a fact about some unrelated subject; test the actual producer/consumer path and the negative counterpart; printed remedies must work in the state that produced them.

## Design

### 1. Establish relevance before deciding whether uncertainty blocks

Use the existing canonical worktree identity, execution slot, scope/epoch links, reservation tokens, and registered participants. Reuse/extract their matching predicates; do not create a new registry, identity format, liveness probe, or policy framework.

There are two different reads:

- **Discovery:** a historical record is encountered while looking for ownership. It does not become an obligation merely because it is unreadable, has an unsupported schema, refers to a deleted worktree, or has lost its old linkage. Unmatched history can produce bounded diagnostics in the existing findings/cleanup surface, but cannot refuse another worktree or run.
- **Required evidence:** an authoritative record for the requested worktree/run names an execution, drive, scope, or reservation. Failure to read or settle that exact obligation remains a local blocker. Missing a named current record must never be mistaken for an empty slot.

A nonempty, validated stored worktree identity matching the requested canonical root, or an existing resolvable alias of it, establishes same-worktree relevance. Failure to resolve some other historical path does not establish a match. Exact epoch linkage establishes run relevance. Current slot/scope references remain evidence of relevance even when the referenced document cannot be loaded. Reject contradictory identities on a positively identified current obligation; never spread that refusal to unrelated worktrees.

### 2. Make admission depend on the requested worktree

Keep the existing admission slot as the authority for modern executions. An absent slot triggers only the narrowly scoped pre-slot compatibility assessment: supported historical records positively bound to this worktree that have no applicable modern admission record. Modern historical drives do not constitute a second admission authority after their reservation has been released or replaced.

A matched pre-slot execution that could still run in this worktree retains the existing process assessment and upgrade/quiescence contract. Other worktrees' records, unmatchable damaged records, stray registry entries, unknown historical schemas without a current reference, and absent old scratch roots cannot veto admission. A failure of historical discovery alone is diagnostic, not evidence that every worktree is occupied; errors accessing this worktree's authoritative slot remain errors. Never infer safety for a named current obligation from a failed read.

A released slot is durable execution-release evidence. Its historical DriveID, ScopeID, RawRunDir, and overwritten predecessor tokens are not fresh obligations. Preserve live epoch ownership between drives until its existing cancellation/success closeout completes. Do not recover every old drive before allocating a new worktree.

Keep `gate.history.cleanup` as the explicit repository-wide assessment surface. It may honestly retain damaged history without implying that every retained record blocks a start. Correct the current shared comments/reporting that equate cleanup's repository-wide retained count with worktree admission refusal. No new cleanup command, forced abandonment marker, retirement receipt, or registry mutation is needed.

### 3. Reconcile a finished incumbent at the normal admission boundary

A slot may still say executing/stopping/unresolved because the process finished before its owner persisted release. Before refusing a fresh start, the existing admission flow must inspect the exact incumbent and settle only facts the existing machinery can prove, then continue that same admission. This is a bounded synchronous reconciliation of one slot, not a retry controller or background sweeper.

Reuse the current drive outcome, process observation/recovery, reservation resolution, release, and epoch-retirement helpers. A matching PASSED/FAILED drive is already authoritative completion evidence. For other incumbent shapes, require the existing positive teardown or never-launched proof. A bare HALTED label, absent file, expired timestamp, free lock, or changed process id is insufficient. Do not introduce a cause-name allowlist.

Protect against pending launch and relaunch: acquire the existing nonblocking claimant where applicable, validate the exact reservation, and retain exclusion while any already-admitted launcher can still act. In particular, `never-launched` is not enough if a delayed ticket retains launch authority. Settle that authority through the existing fenced/terminal path before releasing its slot. An active or unprovable incumbent is not stopped merely because another caller wants to start; use the existing explicit cancellation/stop authority.

Probe outside outer locks; apply changes under existing CAS/lock ordering with the expected reservation and epoch. If a successor wins meanwhile, leave it untouched and report/re-evaluate that actual incumbent once. Do not clear a slot using a newly observed successor token. A failed release/retirement write cannot be reported as successful admission.

Use one behavior for task, build, finalize, scoped/scopeless, and participating raw starts. Adjust or remove early advisory short-circuits in `startBudgetedBuild`, `WorktreeAdmissionRefusal`, and `rawStaleEpochRefusal` where they would bypass this reconciliation. Admission still precedes suite charging, and a refused start launches nothing and consumes no suite attempt.

For a completed raw run, positive existing terminal/teardown proof is sufficient for normal admission to persist release; another manual stop should not be required solely to update bookkeeping. This deliberately narrows ADR-0118's explicit-stop-only rule for raw-slot release. It grants no automatic signalling authority and does not release an active workflow's between-drive epoch ownership.

### 4. Account for the target epoch's obligations, not all history

Retain one shared inventory for `ReconcileEpochLaunches` and `ObserveEpochLaunches`. First identify obligations using the target worktree's slot, scopes linked to the target epoch, exact reservation matches, and existing participant links supplied by the app layer where relevant. Follow current/pending scope references so a corrupt or missing named drive is still detected. Supported readable drives with positive epoch linkage remain candidates, including pending/replacement launches that have not yet appeared in participants or the slot's raw-run field.

Existing records suffice: scopes carry RunEpochID and current/pending drive ids; slots carry epoch/scope/reservation identity; drive records carry scope and admission/relaunch reservations. Enumerating existing stores for matching references is allowed. Adding a global reverse index or another persistent per-run journal is not needed.

Apply the existing terminal-drive shortcut before obsolete linkage lookups. For nonterminal candidates, retain the claimant, delayed-ticket, exact-reservation, and replacement-process checks from #0437. A corrupt required drive, a corrupt scope named by this epoch's current slot, or an unresolved pending reservation must still keep that epoch unaccounted. Strengthen #0437's lost-linkage regression to create a real admitted drive and retain its independent current-slot reference before corrupting the scope; a hand-seeded orphan with no surviving ownership association cannot justify a repository-wide veto. An unlinked orphan elsewhere is an informational historical finding, not `linkage-unresolved` for every epoch. The same rule holds for old runs on a reused worktree when current ownership references establish a different epoch/reservation.

Do not relax `resolveDriveEpoch` when it authorizes a particular drive to launch/relaunch. Losing that drive's ownership proof still refuses its execution. The correction is to census attribution, not to launch authority.

Cancellation continues stopping only its own processes. Success closeout remains observation-only. Resume and terminal repair consume the corrected shared accounting, with no copied exception lists. The existing mutation journal and #0444's uncertain-publication semantics remain intact for genuinely owned unfinished effects.

### 5. Keep current ownership and settled facts stable

Replace first-directory-match ownership selection with deterministic resolution from the existing replacement chain and exact worktree/claim/slot bindings. A superseded predecessor cannot mask its successor; a completed run is not an ambient owner. A cancelled predecessor with a verified current replacement is historical. Multiple genuinely current owners remain a local, explicitly diagnosed contradiction; do not choose by timestamp or silently ignore one.

Keep the distinction between ambient lookup and an explicit old identity: an operation carrying a cancelled/superseded/completed epoch is still refused even when a new owner uses the same path. Preserve the cancelled-run mutation fence until the existing recovery/replacement workflow authorizes current work; simply dropping all terminal epochs from every lookup is not a substitute for resolving the current owner.

Reconcile a stale RunEpochID on a released slot using existing exact-token retirement and the epoch's applicable completion/cancellation accounting. Do not ask a user to cancel a successfully completed run. A genuinely active, cancelling, or completing owner with outstanding obligations continues to hold the worktree.

A durably released execution, completed epoch, or confirmed terminal observation must not become unknown merely because optional scratch evidence later disappears. Consumers must use the sufficient exact durable proof already recorded before reopening ephemeral evidence. In particular, remove redundant process re-observation of a matching released slot in completion accounting; this proves that slot's execution only, not unaccounted participants or launches. For execution-participant checks, accept an exact matching existing released-slot or persisted PASSED/FAILED drive record as the same sufficient execution proof; preserve native participants' separate recorded terminal-status requirement. Do not synthesize terminal evidence from missing files or add process observations to native turn fields. Keep genuinely pending journals and contradictory current references blocking. Do not add a new receipt store.

Audit the existing terminal-result and cleanup paths as part of this change. `Driver.Advance` currently discards release errors, and `recordedDoc` exposes RunRoot for every terminal outcome, including HALTED. Do not make a still-required process record disposable merely because the drive has stopped advancing. Persist/check sufficient release or completion evidence before advertising/removing its temporary root; retain evidence and surface a local persistence/teardown finding if that step failed. Finalize's `mapDriveOutcome` cleanup must obey the same ordering. This extends existing release and cleanup calls, not the storage topology. Test interruption at this boundary so normal cleanup cannot recreate the reported time bomb.

### 6. Diagnostics and recovery must preserve progress

Each admission refusal must identify the affected worktree/current owner or an exact matched legacy execution, the unresolved obligation, and the applicable existing continuation, cancellation, stop, or repair action. Do not present historical cleanup as a mandatory precondition for unrelated work or claim repeating an operation can recreate lost evidence.

History-only findings remain inspectable but do not change the operation's success disposition. Repeated cancellation, completion, and admission after safe reconciliation converge using existing operations; no manual edits or deletion of `.git/docket` records are part of the design.

## Acceptance tests: the class of failure

The implementation is incomplete if it merely makes the quarantined record nonblocking. Derive the caller and registry-reader population from repository searches; do not test only a handpicked entry point.

1. **History isolation matrix:** seed old PASSED, FAILED, HALTED (several causes), WAITING, missing-scope, mismatched-token, missing/empty-run-directory, removed-worktree, unsupported-schema, malformed-record, and stray-entry history. With no ownership connection to the target, new task/build/finalize drives and participating raw starts succeed. Repeat with many mixed records, different orderings, and before/after deleting scratch files. No historical record is rewritten or deleted.
2. **Same-worktree generations:** after proven release or replacement, old records for the same path cannot block the next permitted drive, cancellation, resume, or finalize. Cover delete/recreate of an old worktree path with a properly released slot, symlink aliases, several superseded epochs, and terminal records whose ownership links no longer resolve. Also prove a genuinely live incumbent at that same canonical path still blocks.
3. **Targeted corruption:** corrupt an unrelated record and the operation still succeeds. Corrupt the exact drive/scope/slot named by current ownership and that worktree/run refuses with the right locator. The companion unrelated worktree continues. Preserve positive-reference coverage across reserved-before-launch and replacement-before-attach windows.
4. **Finished incumbent:** leave a proven-finished driven or raw execution's slot occupied by interrupting release. The next normal admission settles it and starts once, without a manual stop or second start. Cover lost final epoch retirement and completed-run-to-finalize. Busy claimant, live process, unresolved establishment, delayed ticket, pending relaunch, and failed release write remain safe refusals. Admission sends no stop signal to make room.
5. **Real cancellation/resume flow:** with unrelated damaged/obsolete history present, cancel an otherwise quiescent epoch, then arm exactly one replacement and start its gate. Cover both cancelled and superseded resume branches, interrupted retirement, repeated cancellation, and new history added between cancellation and resume. Owned pending mutations, processes, or launch reservations still prevent premature completion.
6. **Real implementation-to-finalize flow:** complete a run, close out ownership, remove its optional scratch files, then start finalize on the same feature worktree. Include a superseded predecessor whose directory sorts first and unrelated unaccountable history. Use the production shared census/admission paths, not permissive fake accounting that bypasses them. This is an acceptance requirement alongside fresh-worktree admission.
7. **Concurrency and fencing:** race same-worktree starts across owners/scopes/raw and symlink paths and get one launch; allow independent worktrees to progress. Race reconciliation with successor reservation, cancellation, delayed StartAdmitted, and automatic/recovered relaunch. Old tokens never release a successor, old epochs never regain launch authority, and pre-admitted work cannot first appear after cancellation is reported complete.
8. **Mutation checks:** restore the global historical veto, restore linkage-before-terminal ordering, remove relevance checks, restore first-match epoch selection, or bypass the incumbent reconciliation from an advisory precheck; each relevant behavioral test must fail. Separately remove current-ownership/claim/token checks and prove the safety countertests fail. Update old tests that enshrined repository-wide blockage only alongside these stronger paired tests.
9. **Build gate:** run the whole suite via the source Go runner using resolved `build.test_command`, and inspect budget findings. Use existing fixtures and process seams; no new test framework. Grooming claims no passing implementation tests.

The sanitized quarantine fixture at `/Users/homer/dev/docket-quarantine/gate-drives/b66ce1405cd813e5519c344360f61bd4/record.json` is one matrix case. Copy it into an isolated fixture repository, never into the live registry. Preserve the original and epoch backup; the manually edited live cancelled state is not a test oracle.

## Architecture and scope limits

Record the changed authority boundary in a narrowly scoped ADR during implementation: historical discovery has no global veto; authoritative local uncertainty remains protected; positive teardown can settle a finished raw incumbent during admission. Supersede the conflicting clauses of ADR-0120/ADR-0118 through the ADR workflow, preserving their other guarantees and ADR-0087/0095 process evidence rules. ADR-0124's observation-only successful closeout remains binding.

No new commands, force flags, daemon, background cleanup, registry, persistent schema, lease/TTL, age cutoff, cross-store transaction, configuration, retry allowance, or second liveness implementation. No blanket trust in HALTED, no dropping positively owned pending work, no automatic cancellation to make room, and no trusting change status as process-death evidence. No storage relocation or general garbage collection. This is a correction across existing ownership/admission/accounting paths, not a replacement orchestration system.

No dependencies or stack base. Related changes: 368, 375, 428, 435, 437, 439, 441, 444. Discovered from 444. ADR references: 87, 95, 118, 120, 124.

After implementation merges and the installed binary is rebuilt, verify #0444's recovery through its existing blocked/halted lifecycle and gate workflow, preserving uncommitted work. The change is currently blocked; its bare resume-arm command is insufficient until the owning workflow restores the required lifecycle state. Grooming does not implement or resume 444.
