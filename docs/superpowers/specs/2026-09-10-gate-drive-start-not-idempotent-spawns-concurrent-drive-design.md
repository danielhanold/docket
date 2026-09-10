<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0375 — `docket gate drive start` is not idempotent — a re-run spawns a second concurrent drive](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0375-gate-drive-start-not-idempotent-spawns-concurrent-drive.md)**
<!-- docket:backlink:end -->

# Worktree-wide gate admission and explicit stop/resume ownership

## Intent

Make repeated gate starts and human stop/resume safe: one canonical worktree may have at most one reserved or running top-level Docket gate execution, and a replacement implementation run cannot enter while an earlier run can still act on that worktree. A suite's own subprocesses belong to that one execution.

The chosen duplicate-start behavior is refusal, with a safe locator for the existing drive. A repeated start never recovers ownership credentials, returns a fabricated successful start, or launches a second process. Human Stop means cancel owned work and require explicit resume. Cancellation revokes authority immediately at its durable transition; successful shutdown is reported only after the owned tasks, processes, and admitted mutations have been accounted for.

## Basis and current state

This design follows the human-approved direction for change 0375 on 2026-09-10 and was checked against main at `0f84b9e312b6cbb96973c0a30af3ce774ed079fa`. Change 0405 is merged and archived as done; it is a satisfied dependency.

- `Driver.startScoped` now persists a drive reservation and wins `reserveScopeDrive` before process launch. A scope carries sequential tests through explicit predecessor acknowledgement, followed by `Driver.Acknowledge` for the last result. Preserve this implementation and ADR-0117.
- Those reservations serialize one scope. Separate scopes can still target the same worktree. `Driver.startScopeless` retains launch-before-record behavior.
- `driveSlice` can launch a recovery candidate before `driveAndPersist` arbitrates competing advances. `TestConcurrentSameOwnerAdvanceRelaunchesOnce` currently permits both candidates to launch, then requires the loser to be stopped. That permits transient duplicate load.
- `RunGateBefore` with a resume id verifies the change/workspace and prepares a fresh outer scope. It does not invalidate or shut down the previous run.
- `process.Service.Launch` deliberately detaches the native supervisor. It executes the supplied command, normally a test suite; it does not itself decide to commit, push, open a PR, or mark a change implemented. The reported post-Stop workflow activity therefore also requires addressing surviving coordinators, workers, or background continuations. The incident record is evidence of the symptom, not proof that the test supervisor performs those actions.
- Change 0376 already requires capturing the first ownership-bearing JSON response. This change preserves its credential boundary.

## Scope and assumptions

The guarantee covers Docket-managed gates and workflow participants on one machine, on the supported Darwin/Linux process backend. Different registered worktrees remain independently usable, including linked worktrees sharing a Git common directory. Parallel Docket gate commands in the same worktree are deliberately serialized; callers needing independent concurrent execution use distinct worktrees.

The existing process backend remains the authority for exact owned process identity, signalling, and teardown. The harness adapter remains the authority for exact task/turn identity and cancellation. A test result, a free flock, an old timestamp, quiet logs, a process-name match, or loss of an RPC response is not proof that a writer has stopped.

Native UI Stop integration requires an actual observable lifecycle event from its adapter. Do not promise that every third-party Stop button delivers such an event. The required portable operator path is an explicit cataloged cancellation operation; an adapter may advertise automatic Stop propagation only after its delivery and termination-proof paths are implemented and exercised. A missing integration is reported explicitly, and never permits an unsafe resume. The Codex `agent.enter` route is an in-scope first implementation of lifecycle registration and cancellation, because Docket owns that foreground entry boundary.

## Chosen structure

Two related authorities protect different intervals:

| Authority | Key and lifetime | Responsibility |
|---|---|---|
| Worktree execution slot | Canonical Git repository/worktree identity; reservation through confirmed process teardown | Prevent overlapping gate starts or relaunches, across scopes and without scopes |
| Workflow run authority | Exact claim instance, worktree and run epoch; dispatch through completion or confirmed cancellation | Fence an old coordinator and its descendants and serialize replacement dispatch |

Keep these records in the repository's private local state under the Git common directory. They are versioned, atomically replaced, and mutated through shared Go application/store seams. They do not add a change lifecycle status or put process credentials in tracked metadata.

The execution slot and run authority are not interchangeable. Finishing a test may free its execution slot while the same authorized workflow continues editing. Cancelling a workflow must prevent its next action even when no test is running.

## Worktree execution admission

### Identity and state

Resolve the repository and registered worktree through the existing Git identity machinery. Use its canonical worktree root and registration identity, anchored in the canonical common directory. Resolve symlink aliases; a different run root, command, scope, owner, task, phase, or path spelling cannot buy another slot for the same worktree. A removed or replaced registration, inaccessible identity, or mismatched durable record is an explicit refusal.

The slot records its physical CAS generation, logical execution generation, drive id when applicable, optional scope/run-epoch linkage, launch reservation, raw run identity when known, and state. States distinguish reserved, executing, stopping, unresolved, and released. Released records remain historical evidence rather than being inferred from absence of a process name.

An unresolved reservation occupies the slot. An unknown schema, corrupt record, failed probe, or partially committed transition never means the worktree is free.

### Admission and release

Validate request shape, scope capability and predecessor eligibility, run authority when applicable, and canonical identity before side effects. Under worktree authority, revalidate the decisive state and durably reserve the next execution before any raw process can start. Compose the reservation with 0405's scope reservation using a journaled transition; no second slot implementation bypasses `reserveScopeDrive`.

The reservation identifies the intended drive and allocation before launch. Extend the process allocation/establishment seam so that a launcher killed after spawning but before receiving the handle can resolve that exact reservation. A lost response must not create an unindexed process. Recovery distinguishes proven never-launched, an identified running/terminal process, and unresolved establishment. Only the first two can be completed mechanically; unresolved establishment blocks admission.

Confirmation attaches the exact raw run identity and marks the reserved execution running. Persistence failure retains ownership and triggers bounded, identity-proven cleanup; failure to prove cleanup leaves the slot unresolved.

Release requires durable proof that the recorded execution cannot still run. A durable verdict and the process backend's teardown evidence are reconciled together; a merely terminal drive document, particularly HALTED, is insufficient. Preserve the result and scope acknowledgement history after releasing the process slot.

0405's predecessor rules remain independent: releasing a process slot does not acknowledge a result or retire recovery authority. A successor inside the same scope still supplies the current predecessor id and generation. Final acknowledgement still closes the scope. Retired predecessors never become recovery candidates again.

### Duplicate and legitimate new starts

An occupied worktree returns a typed conflict identifying the existing drive and its non-secret state, where authorization permits disclosure. It returns no owner generation, parent/child capability, or successful start document. It performs zero launches and does not stop the incumbent.

This holds for identical commands, different commands, different scopes, scopeless calls, and scope/scopeless combinations. Once the earlier execution is proven stopped and any same-scope acknowledgement prerequisites are satisfied, an intentional later start creates a new drive and new evidence, even if command and fingerprint are identical.

An admitted but interrupted launch is recovered through its recorded transition, not by treating another `start` as credential recovery.

## Automatic relaunch and locking

All Docket gate process creation routes, including death recovery from `Advance`, use the same admission primitive. Derive the complete population of executable launch sites from a repository-wide search at implementation time and test that each reaches admission.

The design-time search also found `app.GateLaunch`, the public raw `gate.launch` entry point. When its cwd resolves inside a registered worktree, it must acquire the same execution slot and register a raw-run locator before launch; lack of a drive/scope does not exempt it. A raw launch into a workflow-owned worktree must carry that workflow's current authority or be refused. Raw supervision outside a Git worktree retains its existing contract. Compose admission once at the public launch boundary or with a validated reservation handle at the backend; the driver's backend call must not reserve a second slot for itself. The suite runner's internal test processes are children of the admitted execution, not independent gate launches.

A relaunch first proves the previous raw process tree gone, validates current owner/run epoch and original fingerprint/deadline, and atomically reserves the drive's one permitted replacement. Only the reservation winner may call the process backend. A competing advance observes the existing transition or returns the authoritative state; it never launches a candidate for later disposal.

Preserve the existing relaunch eligibility conjunction, fixed deadline, logical drive identity, and single-relaunch limit. Reserve and journal consumption before launch. An uncertain replacement launch consumes its reservation and blocks further launch; it is not refunded or treated as a fresh test.

Lock order is worktree authority, scope, then drive. Run-authority transitions use the worktree authority as their outer serialization boundary. No path acquires that outer authority while holding an inner scope/drive lock. Do not hold these locks across a 30-second observation slice or a network call: use durable reservations and revalidate generations after returning. Concurrent cancellation either prevents a not-yet-started launch or owns stopping the already-admitted launch; it cannot lose the process between records.

The slot remains effective after the CLI exits; holding a flock for one command is not the lifetime guarantee.

## Run cancellation and resume

### Registration and authority

Bind a run epoch to the existing verified claim instance and canonical workspace. Reuse run-gate attribution proofs; never infer which run owns a worktree from creation time or a lone surviving change. A fresh claim binds the epoch before allocating or entering feature work; an explicit resume resolves the existing epoch before dispatch.

Register each coordinator/worker/task boundary and its native handle before permitting work. Register gate reservations under that run epoch through the existing gate-context/scope linkage. Scope-less workflow gates still carry the applicable run identity; omission cannot detach an owned worktree from its run. Standalone gates use the execution slot without pretending to own an implementation epoch.

Pass lifecycle linkage to `agent.enter` as structured catalog/schema fields, in addition to preserving the unchanged human request. Do not parse authority from request prose, a task title, an environment guess, or an agent's final message. Feature children inherit the registered parent linkage without receiving the parent's cancellation authority.

### Explicit cancellation operation

Add semantic operation `run.cancel` to the capability catalog and schema. Its request names the exact run-gate key and expected run epoch, plus an authored reason. The facade validates the current repository, claim and parent-held authority. The key is a locator, not a substitute for those checks. A parent invokes it for an explicit human cancellation or a registered lifecycle-cancellation event; a child failure or an ordinary dispatch return does not imply human cancellation.

Under authority, cancellation durably changes active to cancelling and fences the epoch before requesting process/task shutdown. Registration, new starts, relaunches, successor starts, handoffs/claims that would resume work, and new workflow mutations all reject that epoch. Cleanup and read-only observation remain permitted under the cancellation authority.

Then cancel registered native tasks/turns and call the existing ownership-proven process Stop path for each owned raw run. Re-enumerate the durable registration/reservation journal to catch launches already admitted when cancellation won. Signal only exact owned identities, with the existing bounded TERM/KILL behavior; never broad `pkill`, time-based takeover, or deleting a lock.

Return a structured disposition distinguishing cancelled, already-cancelled, cancellation-pending, and refused, with safe findings naming any unproven participant or transition. Cancelled means all participants are confirmed stopped or terminal, process slots are released, and admitted workflow mutations have completed or been reconciled. An interrupted cancel is repeatable against the same epoch and resumes cleanup without restoring authority. Cancellation-pending retains exclusion and cannot authorize re-dispatch.

Already completed work stays recorded. Stop does not roll back commits, remove files, undo a push, or erase an already-created PR.

### Native Stop and abrupt owner loss

For Docket-controlled foreground entry, connect explicit native cancellation and handled termination signals to `run.cancel`. Keep observing the exact native operation until its terminal state is collected; killing the transport alone is not a termination receipt.

For uncatchable owner death, establish a private lifecycle channel or equivalent exact native death notification at registration. Its lifetime belongs to the long-lived run owner, not an individual start/advance call. A registered cancellation guardian may survive long enough to fence and reap that run; it is scoped to this run, never an ambient reaper. EOF/owner death fences and cancels owned work; it never grants takeover or launches replacement work. Normal completed dispatch or explicit ownership handoff must be recorded before closing/transferring the channel, so ordinary return and tool-call timeout are not mistaken for human Stop.

The guardian receives only the authority needed to cancel and observe its registered run; it cannot become a workflow writer. Give it its own establishment receipt and exact lifetime handle, and reap it after completion, cancellation or successful transfer. If the guardian also disappears, durable exclusion still blocks replacement until explicit recovery proves the participants quiescent.

Prove this mechanism for the actual adapter launch shape, including descriptor inheritance and task descendants. If a native host supplies no reliable cancellation/death event, report that automatic UI Stop propagation is unavailable and expose the explicit cancellation remedy. If it cannot prove task termination, cancellation remains pending even after gate tests have stopped. Do not claim a universal UI integration based on a signal-handler-only unit test.

### Preventing later workflow actions

Cancellation also fences the coordinator, not only its test supervisor. Bind the registered run context to the shared application mutation boundaries used by its workflow. At mutation admission, revalidate the current epoch under the same authority and journal the admitted operation before performing it. Derive the covered mutations from maintained consumers, including publish, PR creation and the implemented transition; do not rely on a remembered list.

After cancellation is recorded, no new mutation from the old epoch is admitted. An operation already in flight can finish or have an uncertain remote outcome; cancellation stays pending until that operation is observed/reconciled. Do not describe Stop as undoing an external request already accepted.

Direct Git/file activity inside a native worker is covered by cancelling that exact worker and collecting termination, not by pretending an application token can intercept arbitrary shell writes. Confirmed cancellation requires both the task/process proof and the mutation journal. Unregistered or unprovable background writers prevent automatic resume and are surfaced as a concrete finding.

### Resume and existing continuation

`run.gate-before --resume` and direct implement-next resume must share the same admission path. If the prior run is still active, refuse with its safe locator and explicit cancel/continue remedy. If cancelling or unresolved, resume cleanup/observation and admit no replacement. After confirmed cancellation, atomically supersede the old epoch and reserve one replacement dispatch. Two concurrent resumes produce one winner.

Lost responses and repeat arms observe/recover that reservation; they do not create another epoch or re-dispatch. Persist dispatch admission before entering the native task. A later failure to establish dispatch is resolved from that reservation, with an unresolved outcome remaining closed.

Preserve ADR-0107 cooperative handoff and event-authorized direct-parent takeover for ordinary non-cancelled work. A valid continuation retains its gate key, run epoch, deadline and attempt accounting; it is not cancellation and does not require stopping a healthy gate. Parent takeover cannot revive a cancelled epoch.

An explicit human resume after confirmed cancellation may arm a new outer gate according to existing human-resume policy. This is distinct from autonomous continuation/retry. It never resets the change-owned full-suite repair budget or turns a human halt into automatic retry permission.

## Accounting, diagnostics and compatibility

Worktree/run admission precedes charging a new full-suite attempt. A duplicate/busy/stale-epoch rejection reserves no test attempt. Once an admissible start has reserved an attempt, preserve ADR-0116's no-refund rule for later launch/persistence failure. Driver relaunch, observation, cancellation and continuation do not reserve a new full-suite repair attempt. Do not add an epoch dimension to the change-owned suite budget or redesign change 0422's outer retry accounting.

Keep existing gate dispositions WAITING/PASSED/FAILED/HALTED and the outer gate's stop/observe/continue discipline. Cancellation is never a test failure or permission to retry. Define stable reasons for worktree-busy, unresolved-execution, run-cancelled, cancellation-pending, stale-run-epoch, and owner-lifecycle-unavailable on the owning schema surface, mapping them consistently through app/CLI and human output. Do not publish credentials in diagnostics.

Version every changed durable schema. Old/unknown records cannot silently become free slots or live new epochs. On first use of the new worktree registry, inventory existing drive records for that canonical worktree, including scopeless and unconsumed terminal/HALTED records. Import only ownership and execution facts the supported old schema and native backend can prove. Missing/ambiguous launch handles or unreadable records block admission with an explicit recovery remedy. Do not migrate old credentials into a fresh owner or infer safety from a missing new slot file.

Upgrade requires quiescing old executors before resuming under the new protocol: an old binary cannot be assumed to honor a new fence. Preserve old evidence and expose unresolved legacy owners instead of deleting their records. Cross-version running workers are not silently grandfathered into the single-writer guarantee.

## Acceptance and verification

Use behavioral concurrency tests with barriers and launch counters, fault-injected persistence seams, and bounded real-process integration tests. Every negative test has independent cleanup and a deadline so a removed guard fails rather than hanging. Count backend launches, not only surviving processes or final RelaunchCount.

1. Same-scope first/successor races retain 0405's one-launch result. Baseline, RED, GREEN and final acknowledgement still work with distinct drives and exact predecessor receipts.
2. Identical/different commands in different scopes, two scopeless starts, and mixed scoped/scopeless or raw/drive starts against one worktree launch exactly once. Symlink aliases, different run roots, owners and phases do not bypass admission. Raw launches outside Git retain their existing behavior. Distinct linked worktrees can progress concurrently.
3. Two same-owner advances after proven raw-process death admit exactly one replacement launch. Cancellation racing either advance admits no post-fence launch; a pre-admitted process is found and stopped. No losing launch is needed for correctness.
4. Crash each reservation/allocation/launch/confirmation/release boundary, including a lost launch response. Recovery resolves the exact execution or refuses. A freed lock, missing response or HALTED document never releases a possibly live process.
5. Stop a registered run during a test, between tests, during worker edits, and at publish/PR/implemented admission barriers. No new action is admitted after the cancellation transition. Already-admitted external actions are reconciled, and cancelled is not reported while an owned writer or unresolved effect remains.
6. Exercise native foreground cancellation and abrupt owner death in an adapter-level process test. Tool-call timeout with the long-lived owner still present preserves the gate. A completed/handoff return does not trigger accidental cancellation. No callback/termination-proof adapter returns the documented limitation/pending state.
7. Two resumes racing a live, cancelling, cancelled or unresolved predecessor never produce two authorized coordinators. Revoked child capabilities, old generations, delayed advances and stale publication calls cannot act under the replacement epoch. Ordinary direct-parent continuation still works.
8. Wrong repository/claim/worktree identity, unknown schema, corrupt record, permission/probe failure, PID reuse and unprovable descendants fail closed without signalling unrelated processes or allowing replacement work. Legacy live/unresolved records cannot be ignored by an empty new registry.
9. Busy/refused starts do not spend suite attempts; admitted failing launches do; cancellation/continuation do not reset deadlines, relaunch limits, full-suite budgets or automatic retry permissions. Verify both 0405 and 0421 behavior.
10. Mutate admission/fencing and the discovered launch-site population to prove the guards redden. Retain 0376 JSON capture/redaction and exact caller scope identity. Update maintained workflow instructions, schemas/catalog entries, adapter contracts and generated assets together.

Run focused package checks during implementation, then the entire configured build test command from source through the Go suite runner. At this design baseline it resolves to `go run ./cmd/docket development test`; resolve it again at build time. Read budget screening findings even on a green run and act on serially confirmed breaches. Verify the configured finalize gate later without substituting a copied command. Grooming itself does not launch processes or run the implementation suite.

## Alternatives and consequences

Get-or-create by command/fingerprint was rejected: identical test commands can be deliberate new executions, and returning ownership to a caller with a lost response creates a separate credential-recovery protocol. Refusal composes with 0405's explicit predecessor acknowledgement.

A worktree flock alone was rejected: short-lived CLI calls release it while detached work survives. Post-launch winner selection was rejected: killing the loser afterward still permits duplicate load and actions. Timer/PID-name reapers were rejected: they do not establish cancellation authority or writer shutdown. Stopping only the test supervisor was rejected: it leaves coordinator publication possible.

The cost is durable admission/cancellation state, adapter lifecycle plumbing, and more explicit pending failures when shutdown cannot be proven. The benefit is a mechanically enforced distinction between one continuing run and a replacement, with no reliance on the operator remembering which detached process survived.

Record a new ADR during implementation covering worktree-wide admission and explicit human-cancellation authority. Relate it to ADR-0087, ADR-0095, ADR-0107, ADR-0111 and ADR-0117. If superseding ADR-0107 to widen its authorization model, state exactly which rule changes and carry forward its non-cancelled takeover rules; never partially rewrite the Accepted decision. Preserve ADR-0115/0116 budget semantics. Accepted ADR prose and merged plans/results remain immutable.

## Out of scope

Fixing the test-load flake, changing test budgets, redesigning forked-agent foreground/yield behavior tracked by 0412, changing review/finalize retry policies, cross-machine ownership, arbitrary non-Docket programs launched outside registered participants, generic OS-wide process policing, automatic rollback of completed work, credential recovery by repeating start, and implementing or planning the feature during grooming.
