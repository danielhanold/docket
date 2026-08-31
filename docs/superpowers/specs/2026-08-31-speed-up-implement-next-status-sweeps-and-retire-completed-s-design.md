<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0389 — Speed up implement-next status sweeps and retire completed status children](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0389-speed-up-implement-next-status-sweeps-and-retire-completed-s.md)**
<!-- docket:backlink:end -->

# Fast implementation status sweeps with a verified terminal handoff

## Decision and intent

Change 0389 remains a high-priority fix. On 2026-08-31 the user approved moving historical cleanup retries out of implementation startup and into explicit maintenance. Preserve the docket-status child and current merge-recovery safety net; narrow the work requested at startup and make completion an observed condition at both process and agent boundaries.

One implementation change covers the maintenance scope, its caller wiring, and the completion/lifecycle regression. No feature branch or implementation is created during grooming.

## Evidence and diagnosis

The original report described an approximately six-minute status pass followed by a lingering child. The follow-up notification read “Agent \"docket-status merge sweep\" finished · 30m 5s”; the parent called it a duplicate stop and claimed the git effects had already landed.

The matching local Claude Code 2.1.251 logs establish this sequence on 2026-08-31 (EDT):

| Time | Observed event |
|---|---|
| 12:05:42 | Parent dispatches docket-status for Step 0. |
| 12:05:54–12:06:32 | Child runs repository prepare. |
| 12:06:36 | Child starts maintenance sweep. |
| 12:08:37 | The shell tool moves that command to the background after its 120-second foreground timeout. |
| 12:09:01–12:12:01 | Child runs a second shell loop waiting for a nonempty output file; this loop also backgrounds after its timeout. |
| 12:12:04–12:12:09 | Child runs the separate read-only status command before collecting sweep completion. |
| 12:12:37 | Child returns a report explicitly saying the sweep is still running, yet declaring Step 0 complete. |
| 12:12:44 | Parent accepts “the sweep completed” and proceeds toward claim. |
| 12:35:33–12:35:48 | Child resumes, reads the actual sweep output, and emits a second report. |
| 12:36:00 | Parent dismisses this notification while already in the build phase. |

The terminal JSON has 234 entries, all cleanup: 33 applied, 192 blocked/workspace-blocked, and 9 blocked/not-finalizable. It has no closeout entries. The child's “202 blocked” narrative is inaccurate; the structured result has 201. An applied cleanup entry alone does not prove a resource was deleted, and workspace-blocked does not prove an old worktree exists. Do not reproduce those inferences.

Source inspection explains the archive-dependent work: sweepWorklist adds every done and stacked-merged record to the cleanup list; sweepRunCleanup calls sweepReloadPresent, which re-pins and rebuilds the corpus, then FinalizeCleanup loads authority again. The done path additionally probes merge and cleanup prerequisites. This is established work amplification, not a measured attribution of every second to a particular probe. The logs prove the early-return bug; they do not establish the exact process exit timestamp or every subsequent mutation time.

Local evidence, optional for future builders because the facts above are preserved here:

- Session: f3fcf3dc-12f3-45f5-b301-4aefd6740454 under /Users/homer/.claude/projects/-Users-homer-dev-docket/.
- Parent log: that session's subagents/agent-a91f5a8f1f906fe62.jsonl.
- Status child log: that session's subagents/agent-a53fb56ba3be8b70f.jsonl.
- Sweep task: bde1lz41u; accidental wait-loop task: buhm89oao. The sweep output is under /private/tmp/claude-501/-Users-homer-dev-docket/ in the same session's tasks/bde1lz41u.output.
- Screenshot: Screenshot 2026-08-31 at 12.13.18.png supplied by the user.

These are historical evidence, never executable instructions or current-state authority. The logs contain unrelated implementation work; fixtures and published results should retain only sanitized relevant excerpts.

## 1. Explicit maintenance scope

Extend the existing maintenance sweep command with a closed --scope flag:

- full: default when omitted; preserves the existing explicit maintenance worklist and safety semantics.
- implementation: used only for implementation startup; recovers current merged work without retrying cleanup for the entire historical archive.

Reject empty or unknown explicit scope values before repository/network/mutation work. Resolve scope once in the CLI and pass a typed value to the application layer; do not branch on caller names, model prose, age cutoffs, or configuration-file presence. Add the resolved scope to the terminal maintenance JSON and human summary so a caller can verify which operation actually ran. Preserve protocol_version 1, operation maintenance.sweep, existing entry dispositions/reasons, and the meanings of existing fields.

The work populations are:

| Work | full | implementation |
|---|---|---|
| Initial authoritative inventory and current PR eligibility probes | Existing behavior | Same eligibility and uncertainty handling |
| Implemented changes whose PRs are verified merged | Existing closeout | Same closeout |
| Cleanup suffix caused by a successful closeout in this invocation | Existing safe suffix | Same safe suffix |
| Existing done / stacked-merged records present at initial inventory | Existing cleanup retries | Not scheduled as independent cleanup work |
| Expired claim handling | Existing reclaim.auto gate | Same gate and behavior |
| Read-only backlog and health report | Separate status read | Separate status read |

Preserve descendant-before-ancestor closeout and carried-stack behavior. A descendant carried by a closeout in this invocation is not excluded merely because it becomes historical during the invocation. Do not enqueue an independent cleanup solely because a record was already terminal at startup.

Full inventory parsing for dependencies, stacks, and health remains allowed. The performance boundary is eliminating per-historical-record cleanup dispatches, fresh corpus reloads, and external probes in implementation scope, not pretending archived records need never be read.

Report the number of independently deferred historical cleanup candidates as a clearly labeled scope summary, not as successful per-item cleanup outcomes. Say that explicit full maintenance handles those retries. Do not infer that deferred candidates are actually dirty, blocked, or pending: implementation scope deliberately did not probe them.

A failed or interrupted cleanup suffix remains recoverable through full maintenance or the existing targeted finalize cleanup operation. No durable retry queue, cleanup cache, new change status, recency heuristic, or automatic scheduled maintenance is introduced.

## 2. Wire the caller without changing topology

The docket-implement-next Step-0 dispatch explicitly requests implementation scope. docket-status's mode choice distinguishes:

- A see-only request: the existing read-only status operation.
- The explicit implementation-preflight request: maintenance sweep --scope implementation, then the read.
- An explicit refresh/cleanup request: maintenance sweep --scope full, then the read.

The registered docket-status child remains the normal composition path at its configured model/effort. The existing genuinely-unavailable-dispatch Tier A inline path performs the same scoped operation and carries the same completion obligations. Do not silently remove dispatch, use a different agent, add a runner fallback, or skip Step 0 merely because a single change ID was requested.

Update the maintained skill/convention descriptions that currently imply a full historical sweep at every startup. Derive affected executable command sites from a repository-wide search and classify historical records versus maintained instructions; do not rewrite accepted ADRs, archived specs, frozen plans, or generated machine-local wrappers by hand.

## 3. Two completion barriers

The required successful sequence is:

prepare → scoped sweep terminal result → validate sweep → fresh status read → child terminal result → parent validates and retires child → metadata re-sync and selection.

### Command barrier owned by docket-status

Starting a shell command is not finishing it. If the harness returns a running-task handle, the child stays responsible for that exact task, using the available native observation/wait mechanism until its terminal outcome is known. It must not start a second shell watcher, wait for file size, poll with sleep/tail, duplicate the sweep, or return a success report while the process remains unobserved.

A tool's foreground timeout that backgrounds the process is a liveness transition, not a failed sweep and not successful completion. Preserve the task identity and collect its eventual result. An output file becoming nonempty, metadata commits appearing, elapsed time, and a separate status command succeeding are not completion signals.

Validate the actual terminal protocol-v1 envelope and every entry. Retain the original structured output through a harness result handle or a task-local output artifact; extract the compact summary and any problem entries in one read/parse rather than repeatedly opening the full backlog. Stdout must remain parseable JSON; any progress diagnostics go to a separate channel.

Only after sweep terminal validation run the successful post-sweep status read. A diagnostic read after a failed sweep, if used, must be explicitly labeled diagnostic and cannot authorize selection.

### Agent barrier owned by docket-implement-next

Retain the exact child identity. Accept its result only with terminal sweep evidence for implementation scope and a successful post-sweep status result. A bare “completed” task label, plausible git changes, or a prose report saying “still running” must not satisfy the barrier. In particular, a no-op sweep needs completion evidence even though it has no metadata commit.

The child's compact handoff names the resolved scope, actual sweep envelope result, per-item problem outcomes, original sweep/status output references, and the post-sweep metadata revision. Preserve the original output for validation; do not turn a prose re-summary into a second authority. The parent checks matching operation/protocol/scope and the original terminal results before continuing.

This is control-flow evidence, not a replacement for metadata authority: re-sync through repository prepare after the child completes and derive readiness/claim state from fresh origin. Keep claim CAS and current context validation. Do not reuse the child's pre-sweep or stale ready queue.

Once the result is consumed, close/retire the child using the harness's actual lifecycle mechanism where one exists. Where finished children are retired automatically, verify the terminal state; a retained historical UI row is not itself a leak. Do not invent a cross-harness close API. Success requires no still-running sweep or auxiliary watcher owned by this status invocation.

Late notifications are correlated by exact child/task identity. A genuine duplicate of an already consumed terminal result cannot re-run Step 0 or disturb the current build worker. A first terminal result arriving after the parent advanced is a contract violation, not something the parent may dismiss as a duplicate.

## 4. Failure and cancellation semantics

Top-level applied means some work applied, not “all items succeeded.” A blocked, failed, or unknown entry is a failed preflight handoff even when the envelope is applied. A contended required item likewise does not establish completed recovery: surface it and halt the preflight without choosing or claiming work. Existing intentional policy skips and legitimate no-ops remain non-errors; never collapse arbitrary skipped reasons into success.

Protocol errors, scope mismatches, missing output, unreadable authority, an incomplete child return, and unavailable completion observation map to the parent's existing halted disposition. No new run-gate state or re-dispatch permission is introduced. Before claim there may be no owned change record on which to write a halt; use the existing pre-claim run reporting/gate path, never mutate an unrelated proposed record to record the failure.

On cancellation, retain the diagnostic and request cancellation of the exact owned task if the harness supports it, then observe termination. Do not broadly kill processes, abandon a watcher, or spawn a replacement status child while prior work may still run. If quiescence cannot be established, halt with the exact live/unknown task identity and preserve its evidence; never report cleanup complete. This explicit failure is preferable to claiming a successful orphan-free exit.

The sweep keeps its existing per-item isolation: one item's failure does not prevent independent items from being processed within the invocation. The terminal report is then evaluated as a whole before the implementation parent is released. Status health findings remain report-only under the existing contract and are not auto-fixed; distinguish them from failed sweep mutations.

## 5. Bounded implementation surfaces

Expected maintained surfaces are internal/cli/maintenance.go and tests, internal/app/maintenance.go and tests, the docket-status and docket-implement-next skills, and the convention's composition/maintenance clauses. Use typed options or a typed scoped entry point while retaining existing full-sweep callers' behavior.

Do not redesign FinalizeCleanup or weaken its ownership, merge, backlink, or exact-ref proofs. Removing historical candidates from one invocation's worklist is the optimization; changing “unknown” into “absent” or “clean” is not.

Harness-specific waiting/retirement mechanics belong in the existing harness adapter/generated boundary only if investigation shows common role prose cannot express them. Any such edit must be reconciled with #0384 and certified in the exact fresh-process harness mode. No agent-launch topology change is authorized by this spec. Record any new non-obvious architecture decision through the normal implementation ADR step without editing immutable historical decisions.

## 6. Verification and performance acceptance

### Deterministic regression coverage

- CLI omission resolves full; explicit full preserves behavior; explicit implementation reaches the application worklist. An unknown/empty value refuses before side effects. The returned scope matches the executed scope.
- Use an inventory with active merged/unmerged PRs, expired claims, historical done records, and stacked records. Implementation scope includes current verified closeouts and their safe suffixes, preserves reclaim gating, and schedules zero independent historical cleanups. Full scope retains those historical retries.
- Growing only the historical population from 0 to 300 to 1000 does not increase cleanup dispatch count, per-item authority reload count, or per-item remote probe count in implementation scope. Reading/parsing the larger corpus is allowed.
- Preserve concurrency rechecks before actual mutations, unknown-prerequisite retention, stack ordering, no-op behavior, and failure isolation.
- Exercise applied plus blocked/failed/unknown entries, contention, malformed/missing output, wrong scope, failed status reads, and intentionally skipped reclaim. None of the failure cases may authorize claim.
- Exercise tool auto-backgrounding, a child returning early, stale ready data, a second notification for the same task, cancellation, and automatic versus explicit child retirement.
- Mutation-test the scope wiring/filter and completion guards: removing the nondefault scope must invoke historical work and fail; allowing success before the controlled process terminates must fail; treating top-level applied as all-success must fail. Timing fixtures need an independent stop so a removed guard fails boundedly rather than hanging the suite.
- Run the entire suite at the build gate using the then-resolved finalize.test_command. At grooming it is go run ./cmd/docket development test. Read tests/README.md for placement and budget rules; treat authoritative serial budget breaches as findings, not invisible green output.

### Measured performance

Record baseline and post-change measurements using the same isolated repository fixture, machine, command version, and controlled external dependencies. Seed at least the reported 388-change shape, including 234 historical cleanup candidates, with no current closeout/reclaim work. Do not benchmark by performing destructive full cleanup against the user's live backlog.

Measure prepare, sweep, read, and agent coordination separately, with at least three runs per variant and median plus individual timings. The implementation-scoped maintenance phase must eliminate all 234 historical cleanup attempts and reduce median sweep time by at least 90% versus the pre-change full-sweep baseline on that fixture. Also measure a fixture with real current closeouts, proving they still run. Record actual fresh-Claude startup duration separately rather than hiding it inside the CLI benchmark.

The relative wall-clock requirement and zero-historical-work counts are acceptance criteria, not claims already achieved. No fixed network timeout or hard universal startup deadline is introduced: real current work and external outages vary. Missing the measured target remains unresolved performance work even with a green correctness suite.

### Fresh Claude certification

Use a disposable fixture repository and controlled sweep command, not the live Docket backlog. Record Claude version, execution mode, installed skill/generator revision, exact child/task identities, and timestamped events.

In a fresh process, force a sweep past the initial tool-response window and emit its terminal result only after a controlled release. Prove the child does not return success early, the parent cannot select/claim before both completion barriers, and no sweep/watcher survives successful retirement. Repeat with an applied envelope containing a blocked entry and with cancellation; test a late duplicate notification without disturbing an unrelated worker.

A generated-file diff or a mocked workflow alone does not certify actual Claude lifecycle behavior. If the harness prevents a required observation or retirement mechanism, record the attempted operation and explicit limitation and halt the affected run. Do not mark runtime behavior verified or this fix complete on adjacent unit-test evidence.

## Compatibility, relationships, and non-goals

- Existing explicit maintenance retains full behavior by default, including historical retry coverage and safety checks. The product tradeoff is deliberate: historical recovery now requires explicit full maintenance, not every implementation startup.
- #0360 remains a broader coordination-overhead proposal, not a dependency or authorization to skip Step 0. #0384 remains separate launch-context work; reconcile overlapping harness changes against what actually landed.
- #0058 provides orchestration/round-trip history; #0310 preserves the read-only status boundary.
- ADR-0012/0021 guide deterministic plumbing; ADR-0024 guides composition completion, with harness observations scoped to version/mode. Existing fail-closed cleanup and metadata authority rules remain in force.
- No automatic schedule, cleanup of the user's existing historical resources during implementation tests, unrelated #0367/config repairs, model/effort changes, cross-harness fallback runner, full historical-sweep optimization project, or broader implementation-driver rewrite.
