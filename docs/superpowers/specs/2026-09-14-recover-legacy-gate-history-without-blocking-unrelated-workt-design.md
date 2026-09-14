<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0428 — Recover legacy gate history without blocking unrelated worktree admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-14-0428-recover-legacy-gate-history-without-blocking-unrelated-workt.md)**
<!-- docket:backlink:end -->

# Change 0428 — unblock admission after a gate-history schema upgrade

## Goal

An implementation using the current Docket binary must start normally in a repository containing completed schema-2 gate drives from before change 0375. It must not require manual deletion of .git files, a maintenance run, or a fresh worktree to bypass history. If a genuinely unresolved execution prevents admission, identify the exact historical drive and report what remains unresolved.

This is an admission compatibility fix with a small recovery entry point. It is not a new lifecycle for retiring drives. Success is a previously blocked implementation reaching its baseline through one ordinary start request, with one launch and the normal attempt charge.

## Confirmed cause and design boundary

Baseline main: 06ebb52c058894b564ac2a8432922ddf4b2d56b3. The execution reader accepts drive schemas 3/4 and rejects 2. inventoryLegacyDrives uses that reader before identifying whether a drive is relevant to the requested worktree. It also resolves the historical worktree path before recognizing a completed drive; a removed historical worktree can therefore block admission unnecessarily. The application drops the internal inventory-legacy-drive-<id> locator and emits the misleading generic “this worktree” message.

Keep the current execution reader's schema policy, worktree admission records, scopes, epochs, and attempt budgets. Change the historical admission check, not the executable drive state machine. Do not add retirement receipts, a retired-drive state, schema migration, a registry of historical references, or another retry controller.

## One shared history check

Introduce a small history reader/classifier used by the existing first-admission inventory and by the manual recovery operation. It explicitly understands schema 2 for historical assessment and uses supported readers for schemas 3/4. Validate the schema-2 envelope and required identity/outcome fields against historical source and fixtures. Do not merely decode an unknown document into the current struct with zero-valued missing fields. A history read never enables advance, takeover, relaunch, or upgrading a schema-2 execution record.

For each record, follow this order:

1. Validate the directory, record, supported historical schema, and repository identity. Unknown schemas, corrupt bytes, unsafe paths, and read failures are unresolved; carry a bounded locator. Preserve the current behavior for record-less directories created before a record is published.
2. Recognize trustworthy completed PASSED/FAILED history before resolving its worktree path. Validate that the historical version's outcome contract represents a persisted supervisor completion, with no contradictory launch state. Such history is nonblocking even when its worktree or temporary run directory was subsequently removed. Missing old temporary files must not undo sufficient durable evidence already present in the drive record.
3. For history that is not conclusively completed, establish the canonical worktree binding. A valid binding to a different worktree is irrelevant to this admission, regardless of whether that other worktree's drive is active. Do not equate an unresolvable path with proof of unrelatedness or teardown.
4. For a terminal HALTED record that lacks sufficient teardown evidence, use the existing process recovery machinery to check the exact recorded run and attempts. Reuse its positive group-absence and completed-stop evidence and, where needed, its existing abandoned marker. A HALTED record whose worktree is gone may be assessed using trustworthy repository/run identities without requiring that path to exist. A bare Observe result of vanished is not sufficient: it can mean only that the supervisor lock is free. Reuse or extract the existing process predicate; do not add a second liveness implementation.
5. A live/nonterminal drive, a pending launch or relaunch, ambiguous ownership, an identity mismatch, or an unprovable probe remains unresolved when it could occupy the requested worktree. Do not recover a WAITING/reserved drive by guessing that it is dead or rewriting its outcome. Current admission and epoch fences remain independently authoritative.

Because original records are neither removed nor retired, existing references to them remain valid. There is no need to prove that every scope and historical epoch stopped referencing a completed record before ignoring it as an execution blocker. This avoids turning stale references into a new permanent blocker.

## Automatic recovery within the original admission

Run the shared check inside the existing authoritative first-admission path, before reserving an execution, charging a suite attempt, or launching a process. Schema-2 PASSED/FAILED history normally needs only a compatible read; HALTED history may need one exact-run recovery assessment. Process each relevant historical drive at most once in the inventory pass, handling multiple obsolete records together.

This is the automatic cleanup/recovery attempt requested for the reported failure, placed before returning the avoidable refusal. Do not intentionally fail, shell out to a cleanup command, and call start again. The manual command and automatic admission call the same underlying service. There is no new start retry, agent dispatch, outer gate arm, receipt journal, or permission token.

If all relevant history is nonblocking, continue the original admission using its original command, caller, scope, epoch, and gate context. The ordinary admission lock and subsequent cancellation/ownership checks still control the one allowed launch. Recovery must observe request cancellation and the existing deadline; it cannot reset either. If any blocker remains, return once with its locator and recovery finding. No polling or recursive recovery.

This logic is shared by the first-admission paths used by scoped/scopeless gate.drive.start and participating raw gate.launch. It must not run as a response to a test failure, a post-launch HALTED result, a busy current admission slot, a stale/cancelled epoch, or an ambiguous launch response.

Expose a compact optional history-recovery summary in JSON and human output when legacy compatibility/recovery was relevant: checked legacy records, newly recovered records, and any remaining safe blocker locators/reasons. Include it on a successful start as well as a refusal so an operator can tell recovery occurred. A recovery report is not a passing test result. Normal starts with no relevant legacy history need no extra narration.

## Small manual recovery command

Add the cataloged operation gate.history.cleanup:

```text
docket gate history cleanup --repo-dir <repository> [--drive-id <id>] [--dry-run] [--json]
```

It runs the same history check without starting an execution. By default scan this repository's historical drive inventory in deterministic id order; --drive-id limits assessment/recovery to one validated id. The command works locally from the primary checkout or any linked worktree, without metadata preparation, a remote fetch, or the continued existence of a historical worktree.

Report each candidate as nonblocking, recovered, or retained with its reason. Nonblocking means the record does not represent an occupying historical execution; it does not certify that all current admission slots are free. --dry-run previews assessment and any possible existing-marker write without modifying gate state. A required recovery that cannot be performed in dry-run is reported as recoverable, not recovered. Repeated applied runs produce no additional state changes once the existing process marker is present. Unknown/corrupt/live/unprovable records remain visible; a mixed result cannot claim complete recovery.

Here “cleanup” means resolving obsolete execution blockers while retaining their evidence. The operation does not reclaim disk space or delete old drives/logs. Remove the earlier draft's --older-than and --prune-logs options and general current-format garbage collection from this change. Those can be considered separately if needed; no additional change is created during this grooming.

## Locking and diagnostics

Keep the current worktree-admission → scope → drive lock order. Reuse the process recovery locking and identity checks; do not acquire an outer gate lock from an inner one. Re-read mutable evidence under the applicable existing lock before deciding. Do not remove lock files or invent a repository-wide lifetime lock. Empty lock files do not establish a live execution or prove abandonment.

Keep the compatible reason unresolved-execution, but propagate the typed inventory stage and inventory-legacy-drive-<id> locator through JSON and human output. Validate ids before rendering; arbitrary directory names must not enter the diagnostic. Use a safe inventory-level locator when there is no valid drive id. Do not say “this worktree” when its ownership was never established.

For retained history, name the manual history-cleanup operation as an inspection/recovery route and explain why automatic assessment retained the record. Repeating it is not promised to repair unknown or corrupt state without additional evidence. Prescribe run.cancel only when there is an actual owning epoch to cancel. Do not expose raw drive contents, argv, environment values, gate context, or ownership credentials.

Register the command/result schema, update maintained recovery guidance and generated assets, and document the supported upgrade boundary. No claim is made that an old binary deliberately launched concurrently will honor new admission rules.

## Acceptance criteria

1. Use real pre-0375 schema-2 fixtures to reproduce the reported refusal, then prove an ordinary implementation start reaches and executes its baseline automatically. No prior cleanup command, manual file deletion, or second start call is allowed. Exactly one process launches and the normal single suite attempt is charged where applicable.
2. Cover multiple completed legacy drives, unrelated live worktrees, and completed history whose worktree and raw temporary directories are gone. Terminal history must be recognized before resolving removed paths.
3. Same-worktree live/nonterminal state, unknown schemas, corrupt records, contradictory identities, and unresolved launch/relaunch transitions still block with the exact safe locator. No new execution reader compatibility is granted to schema 2.
4. HALTED history can recover through positive existing process evidence. A free lock, bare vanished observation, surviving descendant group, failed probe, or missing evidence cannot certify teardown. An injected probe error retains the record.
5. Scoped, scopeless, and raw first-admission callers share the behavior. Concurrent starts retain the one-execution invariant; a concurrent history assessment cannot delete evidence, deadlock, or waive cancellation. Cancellation or deadline expiry prevents launch.
6. Manual cleanup and --dry-run use the same classifier and produce consistent findings. Repeated recovery reuses existing markers and changes nothing further. The command reports partial/retained outcomes without claiming repository readiness.
7. Successful and refused starts show the relevant recovery summary. Failed tests, post-launch failures, and current admission/epoch refusals do not trigger a cleanup/start retry. Budgets, scopes, and run identities remain unchanged by history assessment.
8. The fix preserves original drive records, logs, and lock files. There is no retirement store, reference census, age filter, log pruning, or schema rewrite. Schema/capability and secret-redaction tests cover the new public surface.
9. Run the whole suite through the configured build.test_command from the source checkout, inspect its budget report, and mutation-test any added guard. These targeted behavioral cases supplement the required build gate.

## Scope and architecture records

Relates to changes 0375 and 0427; discovered from 0375. No dependencies or stack base. The separate verdict-path epoch binding fix in 0427 remains outside this change. Preserve ADR-0087/0095 evidence requirements and ADR-0118's worktree admission invariant. If implementation records an ADR, describe the narrow distinction between historical assessment and executable compatibility; do not introduce a retirement architecture or rewrite an accepted ADR.

Grooming writes only this spec and change metadata. It does not implement the fix or touch a consumer repository's gate state.
