<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0494 — A publish killed mid-flight wedges its run's cancel and closeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0494-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos.md)**
<!-- docket:backlink:end -->

# A publish killed mid-flight wedges its run's cancel and closeout — design

## Problem

Before `pr.publish` or `workspace.publish` does its remote work for a tracked run, it writes an `admitted` entry in the owning run's mutation journal. When the remote work returns, the same process rewrites the entry as `completed` or `uncertain`. If the process dies in between, the entry says `admitted` forever, and the run can neither complete nor be cancelled.

The design was traced on main at 756fea9fe and re-checked at 1fc28e872, after 0491 and 0492 merged. Those two changes renamed tokens (`stale-run-id` became `run-superseded`) and removed the run id, but left the journal logic unchanged.

The trace found these facts.

1. **The window is the whole remote call.** `admitWorkflowMutation` (`internal/app/runtracker_fence.go`) appends the `admitted` entry under `runRecordCAS` and returns a `mutationJournalDone` callback. `PRPublish` (`pr_publish.go`, step 8a/8b) and `WorkspacePublish` (`workspace_ops.go`) call that callback only after `EnsurePullRequest` or `PublishHead` returns. The `docket` CLI installs no signal handler for these operations: `signal.Notify` appears only in `codexentry`, `suiterunner`, and the process supervisor. So Ctrl-C, SIGTERM, SIGKILL, or a crash during the GitHub call or the push all leave `admitted`.
2. **Nothing settles `admitted`.** `settleUncertainPublications` and `publicationRetryMatch` (`runtracker_publication.go`, change 0444) consider only an `uncertain` original. A later identical publish that completes verified leaves the `admitted` entry untouched.
3. **Every journal reader blocks on any entry that is not `completed`.**
   - `reconcileRunTeardown` step (7) serves `run.cancel`, the agent death guardian (`guardianFenceAndReap`), and the `agent.enter` lifecycle cancel. It reports `mutation-pending:<op>`, and cancel stays `cancellation-pending`.
   - `verifyTerminalRunQuiescence` serves a repeat cancel of a terminal run (which is `refused`) and `validateResumeQuiescence` (whose resume is refused).
   - `accountCompletionMutations` serves the keyed closeout's steps (3) and (4). `run.verdict` prints `run-stop <key> run-tracker-unavailable completion-unaccounted`, and the run stays `completing`.
4. **The same wedge exists for `uncertain` with no retry.** An `uncertain` entry is settled only by a later verified identical retry in the same run. Once cancel fences the run (`cancelling`) or the closeout fences it (`completing`), `admitWorkflowMutation` refuses every new publication (`run-cancelled`, `run-completed`), so that retry can never be journaled. 0444's spec named both cases (a killed producer, and an uncertain entry with no completed retry) as limits that "can still require human investigation". No docket operation clears an entry; the only remedy is hand-editing `run.json` under `.git/docket/run-tracker/<key>/`.
5. **An entry carries no producer identity.** `AdmittedMutation` holds `op_key`, `status`, `publication`, and `verified`, so nothing can tell a live producer from a dead one.
6. **The kernel-lock pattern already runs in production.** `process.TryExclusiveLock` (change 0490) is a non-blocking exclusive `flock` with a typed busy result. `live.lock` (ADR-0095) and the worktree `busy.lock` (ADR-0132) use the pattern: a held lock proves a live holder, and the kernel releases it when the holder exits or dies. ADR-0132's rules apply: release by closing only (never `LOCK_UN`), never delete a lock file, and only try a lock, never wait on it.
7. **`run-complete` already proves both publications live.** `RunVerify` (`run_verify.go`) probes the remote feature ref, which must equal the local head (`remote-head-mismatch` otherwise). It also finds exactly one open PR at that head targeting the resolved base (`pr-unverified` otherwise). The keyed closeout runs only after a verified `run-complete`, so it does not need the journal to prove that a publication landed.
8. **Both publications converge on a retry.** `PublishHead` re-proves the expected head under its own lock (change 0451) and re-probes after the push. `EnsurePullRequest` is create-or-adopt with post-mutation verification. A replacement run's publish therefore reaches the right state whatever an earlier unknown attempt did.
9. **No incident is on record.** This machine's 19 run records hold 10 journal entries (6 `workspace.publish`, 3 `pr.publish`, 1 `change.attach-plan`), all `completed`. The evidence is the code reading above.

## Decision

A journal entry blocks cancel, resume, and the success closeout only while its publisher may still be running. A publisher holds a kernel lock from before its entry is written until after its outcome is written. A free lock therefore proves the publisher is gone. Once the publisher is gone, an unknown outcome is reported as information and never blocks. Wherever the evidence is unclear, behavior is exactly today's.

### 1. The publisher holds a per-entry lock (`pr.publish`, `workspace.publish`)

On the active-run path of `admitWorkflowMutation` with a publication descriptor (`pub != nil`):

- **Take the lock first.** Mint a random token and take `process.TryExclusiveLock` on `<run-key-dir>/publish-<token>.lock` BEFORE the admission `runRecordCAS`. `<run-key-dir>` is the directory `runKeyDir` resolves for the owning run, beside `run.json` and `run.lock`.
- **Journal the token.** The `admitted` entry stores the token in a new field, `AdmittedMutation.LockToken` (`json:"lock_token,omitempty"`). The field is additive, with no schema-version bump; an absent value decodes as "no lock".
- **Release last.** The returned `mutationJournalDone` callback writes the outcome first, then closes the lock file. It closes the file on every path, including when the outcome write fails. A fence refusal from the admission CAS closes the file before returning.
- **The order is load-bearing.** The lock is held before the entry becomes visible and released only after the outcome write. So `admitted` with a free lock means the publisher exited without recording an outcome.
- **No new refusal.** If the token cannot be minted, or the lock cannot be taken (an I/O error; a busy result on a fresh token is treated the same way), the entry is journaled without `LockToken` and the publish proceeds. That entry behaves exactly as today's.
- **Scope.** No lock is taken for metadata transactions (`MutationAdmissionHook`, `pub == nil`, completed at admission), or for an unfenced publish with no owning run, which writes no entry.
- Lock files are never deleted and never `LOCK_UN`ed, per ADR-0132.

### 2. One classifier for every journal reader

A single helper classifies each entry; the plan picks its name. Its lock probe never creates the file: it opens without `O_CREATE` and tries `LOCK_EX|LOCK_NB`, releasing at once. That matches `probeFlock`'s semantics, exported from `internal/process` as a typed probe (held, free, missing, unknown). `internal/process` stays standard-library-only.

| Entry | Lock probe | Blocks | Finding |
|---|---|---|---|
| `completed` | — | no | none |
| `uncertain` | — | no | `mutation-abandoned:<op>` |
| `admitted` with `LockToken` | free | no | `mutation-abandoned:<op>` |
| `admitted` with `LockToken` | held | yes | `mutation-pending:<op>` |
| `admitted` with `LockToken` | file missing, or probe error | yes | `mutation-pending:<op>` |
| `admitted` with no `LockToken` | — | yes | `mutation-pending:<op>` |
| any other status | — | yes | `mutation-pending:<op>` |

An `uncertain` entry is written only by the publisher's own callback, so its publisher has always returned.

The classifier replaces the inline `m.Status != mutationStatusCompleted` loops in:

- `reconcileRunTeardown` step (7): `run.cancel`, `guardianFenceAndReap`, and the `agent.enter` lifecycle cancel;
- `verifyTerminalRunQuiescence`: a repeat cancel of a terminal run, and `validateResumeQuiescence`;
- `accountCompletionMutations`: closeout steps (3) and (4), deduplicated as today.

It never returns an error a caller could turn into a refusal. Every unprovable case is the existing blocking `mutation-pending:<op>`.

### 3. The write paths record the abandonment, then 0444's match runs

`settleUncertainPublications` gains one step inside its existing `runRecordCAS`, before the 0444 match. For each `admitted` entry with a `LockToken` whose lock probe answers free, it sets `Status = uncertain`; `Verified` stays false. `publicationRetryMatch` then runs over the fresh record as today. A verified identical later retry therefore upgrades the original to `completed` and reports `mutation-settled:<op>` instead of `mutation-abandoned:<op>`.

- **Callers are unchanged.** These are the two write paths that already call it: cancel teardown (step 5d) and the keyed closeout (step 1b). Read-only paths (`RunVerify`, unattributed and observe verdicts, `verifyTerminalRunQuiescence`) never write; the classifier gives them the same answer from the probe alone.
- **The write is bookkeeping, not evidence.** The lock probe is the evidence. If the write fails (`mutation-settle-failed`, as today), the classifier still reads the free lock and does not block. The write keeps the journal truthful ("outcome unknown") and lets 0444's match settle it.
- **No deadlock.** Probing a publish lock while holding `run.lock` only tries and never waits. A publisher holds its publish lock and waits on `run.lock` for its outcome write. A settler holds `run.lock`, finds the publish lock busy, leaves the entry `admitted`, and releases `run.lock`.

### 4. Vocabulary

- **`mutation-abandoned:<op>`** (new) is an informational finding: docket stopped waiting on a publication whose outcome it never observed. Either the publisher died, or it returned without seeing the remote answer, and no identical publish later confirmed it. It never blocks and never changes a disposition or verdict. It surfaces where accounted findings already surface: `RunCancelResult.Findings`, and `run.verdict`'s `completion_findings`, including on `run-complete`, as 0492's `tree-survives` does.
- **`mutation-pending:<op>`** keeps its spelling. It now means a publisher that may still be running, or an entry whose state cannot be proven.
- **Collision check (ADR-0129).** The `*-unobserved` tokens (`participant-unobserved`, `process-unobserved`) are blocking findings, and most `*-unknown` tokens mean a probe failed, so neither suffix fits an informational finding. `mutation-abandoned` pairs with ADR-0133's `launch-abandoned` (a launcher that provably died before acting) and with the existing `mutation-pending` / `mutation-settled`. A whole-repo grep at 1fc28e872 found no existing use.

### Failure posture

- **No new refusal.** A lock failure at admission degrades to today's no-lock entry, and the publish always runs.
- **No new block.** Every changed branch either unblocks on positive proof (a free lock, or a returned publisher) or keeps today's blocking. A missing lock file, a probe error, a legacy entry, and an unknown status all keep today's behavior.
- **No new effects.** Cancel, closeout, and resume gain no signal and no Git or GitHub call.
- **Never escalated.** No skill or reviewer escalates `mutation-abandoned` into a blocker; the glossary entry says so, as it does for `tree-survives`.

## Accepted losses

- **Unknown remote state after cancel.** After cancel reports `cancelled` with `mutation-abandoned`, a pushed branch or an opened PR may or may not exist. A resumed replacement adopts either, because both publications converge (fact 8). A human who abandons the change instead may find a stray branch or PR, which is the same remote state a killed publish leaves today, only no longer hidden behind a wedge.
- **Cancel's rule is narrowed.** ADR-0118 (change 0375) says a cancellation is never reported `cancelled` while an unresolved external effect remains. That becomes: never while a publisher may still be running.
- **Closeout's rule is narrowed.** ADR-0124 rule 3 says closeout fails closed on any uncertain obligation. It no longer does so for a publication; `RunVerify`'s live probes are the evidence instead (fact 7).
- **Old and lockless entries keep today's wedge.** An entry written before this change, or one whose lock could not be taken, still blocks until its publisher's callback writes. None exist on this machine.
- **Hand-deleted lock files are out of contract.** If a lock file is deleted and recreated while its publisher runs, the new inode reads free. Docket never deletes lock files (ADR-0132); hand-deleting one is outside the contract.

## Unchanged

- Metadata transactions' journal entries, completed at admission.
- The mutation fence: which run states admit or refuse a publication.
- 0444's match rules: identical descriptor, `Verified`, higher index, same operation.
- Cancel's and closeout's participant and launch-census accounting.
- Every disposition vocabulary, and `RunVerify`.
- The CLAUDE.md / AGENTS.md run-tracker block. Its `cancellation-pending` gloss ("a process or in-flight action still resolving") stays accurate.

## Prose and generated sites

- `docs/reference/glossary.md`: a new `### Cancel finding \`mutation-abandoned\`` entry beside `tree-survives`. It says what the finding means, that it never blocks, and that a resumed run's publish adopts whatever landed.
- `docs/concepts/run-tracker.md`: one paragraph in the cancel section saying that a publish killed mid-flight no longer wedges cancel, resume, or the closeout.
- Code comments that state the old rule, derived by a grep of `internal/app` for `uncertain`, `admitted-not-completed`, `mutation-pending`, and `FAIL CLOSED`, never a hand list. The known ones:
  - the `runtracker_fence.go` status-constant and `mutationJournalDone` docs;
  - the `runtracker_publication.go` header ("the ONLY settling evidence");
  - the `runtracker_cancel.go` header and the `mutationStatusCompleted` comment;
  - the `runtracker_complete.go` FAIL CLOSED paragraph.
- No protocol schema descriptor changes: `AdmittedMutation` is run-record state, not a request or result.

## Tests

1. **Killed publisher, cancel.** A run whose `workspace.publish` entry is `admitted` with a `LockToken` and a free lock. Cancel reports `cancelled` with `mutation-abandoned:workspace.publish`; the stored entry is now `uncertain`; `run.start --resume` admits exactly one replacement.
2. **Killed publisher, closeout.** The same shape for `pr.publish` on a verified `run-complete`. The keyed verdict reports `run-complete`, `completion_findings` carries `mutation-abandoned:pr.publish`, and the run is `completed`.
3. **Live publisher.** The test holds the lock. Cancel reports `cancellation-pending` with `mutation-pending`, and the closeout reports `completion-unaccounted`. After the test releases the lock, a repeat cancel reports `cancelled`.
4. **Unprovable entries block.** No `LockToken`; a missing lock file; a probe error through a test seam. Each blocks exactly as today.
5. **Killed, then a verified identical retry** journaled before the verdict: `mutation-settled:<op>` is reported, with no `mutation-abandoned`.
6. **`uncertain` with no retry.** Cancel reports `cancelled` with `mutation-abandoned`, the closeout reports `run-complete` with it, a repeat cancel reports `already-cancelled`, and resume admits. This deliberately reverses the premise of 0444's no-retry tests, such as `TestIntegrationRunCancelStaysPendingWithoutCompletedIdenticalRetry` and `TestIntegrationRunCompletionCompleteSuccessfulRunStillBlocksWithoutRetry`. Find the full set with a grep of `internal/app` tests for an `uncertain` entry expected to block. Rewrite each to the new rule; do not delete any.
7. **Admission.**
   - The entry carries `lock_token`.
   - An adapter seam asserts that the lock is busy during the remote call.
   - The lock is free after the callback, and the file still exists.
   - With lock acquisition failing (seam), the publish still runs and the entry has no token.
   - A fence refusal leaves the lock free: a following `TryExclusiveLock` succeeds.
8. **Real process** (CI on `macos-15`, like 0490's). A helper child takes the lock and is SIGKILLed; the classifier then reads free and reports `mutation-abandoned`.
9. **No deadlock.** A settler holding `run.lock` while a publisher holds its publish lock leaves the entry `admitted`, and the publisher's outcome write then completes. Run it under a timeout.
10. **Mutation checks.** Each must turn a named test red:
    - the classifier treats held as free → test 3;
    - the classifier treats a missing file as free → test 4;
    - drop the admitted→uncertain step → test 5 reports `mutation-abandoned` instead of `mutation-settled`;
    - close the lock before the outcome write → test 7's ordering assertion.

## Decision record

- **New ADR:** "The publish journal blocks only on a publisher that may still be running". Its Context is this spec's facts, its Decision is sections 1–4 and the failure posture, and its Consequences are the accepted losses. It relates to ADR-0118, ADR-0124, ADR-0132, ADR-0133, and ADR-0134.
- **Dated Update notes**, appended and never editing the decision:
  - ADR-0124: rule 3's fail-closed-on-uncertain no longer covers a publication journal entry; `RunVerify`'s live probes are the evidence.
  - ADR-0118: cancellation's "unresolved external effect" condition is narrowed to "a publisher that may still be running".

## Out of scope

- A signal handler in the `docket` CLI for the publish operations. The lock covers every kind of death, SIGKILL included.
- Live Git or GitHub re-observation inside cancel, closeout, or resume.
- A force-clear command or flag.
- Rewriting or migrating existing journal entries, and deleting lock files.
- Metadata transactions, and `finalize.publish`, which is not journaled.
- Changing which run states the fence admits.
