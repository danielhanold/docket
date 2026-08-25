<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0342 — Harden autonomous build/implement agents against the suite-yield deadlock (ADR-0024)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-25-0342-harden-autonomous-build-implement-agents-against-the-suite-y.md)**
<!-- docket:backlink:end -->

# Resumable native gate driver — bounded waits with durable ownership

**Change:** 0342 · **Date:** 2026-08-24 · **Type:** fix · **Priority:** high

## Problem

Docket has already solved long-running suite supervision at the operating-system layer. The native
Go supervisor introduced by change 0314 creates a detached session on Darwin and Linux, records the
child's exact exit or signal outcome durably, and proves ownership before stopping or recovering a
run. That work fulfilled the process-level goal behind the killed Bash design in change 0285.

The layer above it still assumes that one caller can remain synchronously occupied until the suite
finishes. `docket-build`'s canonical caller contract launches the raw gate and enters one Bash
`while` loop for as much as `gate_observation_budget`, currently 30 minutes. The loop sleeps and
parses repeated `docket gate observe --json` documents with `jq`. The production
`processFinalizeGate.observeToTerminal` path independently implements the same architecture as a Go
loop with a 30-minute bound.

Those loops are bounded in elapsed time, but not in the duration of one foreground tool call. When
the full suite reaches or exceeds a harness's foreground-call ceiling, a forked AI worker is tempted
to background the call and yield. ADR-0024 explains why that cannot work: the fork has neither a
human channel nor a task-notification channel through which its background child can wake it. The
returned prose can say “waiting” or even look complete while no authoritative workflow transition
occurred.

Changes 0339 and 0341 demonstrated the resulting failure mode. Repeated attempts to resume the run
created overlapping agents and monitors, the suite result was separated from the agent that needed
to consume it, and the workflow stopped before publication. One monitor also matched its own
`pgrep -f` command line. That self-match made the incident worse, but banning one spelling would not
fix the architecture: workflow agents should not be implementing process liveness at all.

The missing layer is a durable, native continuation above the raw supervisor. It must keep each
synchronous interaction comfortably below the harness ceiling, preserve one logical deadline and
one process identity across interactions, and let responsibility move to a fresh agent without
creating a second suite or trusting transcript prose.

## Terminology

- A **raw run** is one process-supervisor run directory created by the existing native `launch`
  primitive. It has one operating-system process tree and one durable terminal record.
- A **drive** is the higher-level logical suite execution introduced here. It owns one raw run at a
  time and may own a second only after the first is proven dead and the one-relaunch policy admits
  it.
- A **slice** is one short synchronous period during which the Go driver observes a live raw run.
  A slice ending is not a suite timeout and not a test result.
- An **owner** is the workflow agent or controller holding the current opaque claim generation for
  a drive.
- A **handoff** is an explicit, durable transfer offer. It invalidates the old owner's authority and
  can be claimed once by a fresh owner after the repository state is revalidated.

## Constraints and invariants

The implementation must preserve these properties:

1. The existing native process supervisor remains the sole authority for process identity, terminal
   status, session isolation, durable logs, stop, recovery, and cleanup.
2. A workflow call never stays blocked for the full observation budget. It either reaches a
   terminal or returns a typed nonterminal result after a short slice.
3. The total observation deadline is set once when the drive starts. Slices, process restarts,
   agent handoffs, CLI restarts, and the one permitted suite relaunch never extend it.
4. At most one owned raw process tree is live for a drive. Uncertain ownership fails closed and can
   never authorize another launch.
5. A passed result certifies the exact repository bytes that were present when the drive began.
   HEAD, staged changes, unstaged changes, untracked files, modes, and symlink values cannot drift
   across a continuation.
6. `FAILED` means the suite itself completed red. Process death, malformed state, deadline expiry,
   identity uncertainty, and handoff mismatch are never converted into test failures.
7. Waiting is private runtime state, not docket metadata and not a new change status.
8. AI agents keep responsibility for judgment: editing, repair choices, task completion, review,
   and publication. Go owns deterministic mechanics: launch, time, identity, persistence, state
   transition, and ownership transfer.
9. No notification channel or long-lived coordination daemon is introduced. The existing per-run
   Go supervisor is the only long-lived Go process.

## Decision summary

1. Add a high-level native Go gate driver that composes the existing process service directly.
2. Persist a versioned drive record under the repository's private Git runtime and advance it in
   short synchronous slices, with a production slice target of 30 seconds.
3. Return one of four closed outcomes: `WAITING`, `PASSED`, `FAILED`, or `HALTED`.
4. Make replacement-agent continuation possible only through an explicit, fingerprinted,
   compare-and-swap handoff.
5. Bubble waiting only to the nearest live workflow owner. Restarting the outermost
   `docket-implement-next` agent is a recovery path, not normal gate sequencing.
6. Migrate every executable workflow caller—including finalize's in-process loop—to the driver in
   the same change. Retain the five raw verbs as primitive and operator APIs.
7. Surface a safely resumable outer handoff as `run-waiting`, derived exclusively from agreeing
   local receipts.
8. Record the structured-waiting and ownership-transfer decision in a new ADR while retaining
   ADR-0024's prohibition on background-and-yield.

## Architecture

### Agent topology remains unchanged

The build controller and task workers are AI roles, not Go processes:

```text
top-level harness session
└── docket-implement-next AI agent
    ├── docket-build controller role (inside the same implement-next agent)
    │   ├── named task-worker AI agent, one task at a time
    │   └── repair-worker AI agent when policy admits repair
    └── evidence, review, publication, and mark-implemented phases
```

For one suite execution, the process topology is:

```text
current AI owner
└── short-lived `docket gate drive ...` Go invocation
    └── existing detached native gate supervisor
        └── configured test command and its process tree
```

Each driver invocation exits after one slice. The detached supervisor and test process survive
between invocations. A later invocation reads the same durable drive and raw-run records; it does
not rediscover the process with `pgrep`, infer state from a log, or rely on a notification.

Application code such as finalize composes the same Go driver service in-process. It must not shell
out to docket's own CLI. The CLI and application seam are two adapters over one state machine.

### The driver sits above the raw verbs

The five existing raw verbs retain their narrow meanings:

| Raw verb | Primitive retained |
|---|---|
| `launch` | Start one detached native-supervisor run and return its raw run-directory handle. |
| `observe` | Read one snapshot of a raw run's durable state without waiting or changing policy. |
| `stop` | Perform ownership-proven bounded `TERM`→`KILL`, or report the already-terminal no-op. |
| `recover` | Mark only ownership-proven abandoned raw runs; never signal or delete by inference. |
| `cleanup` | Remove eligible logs while preserving the terminal receipt and diagnostics required by policy. |

They remain callable by the driver implementation, primitive-level tests, diagnostics, recovery,
cleanup, and operator workflows. They are not high-level workflow APIs. Build, implement-next, and
finalize callers cannot compose them directly after this change.

The high-level CLI surface is:

```text
docket gate drive start
docket gate drive advance
docket gate drive handoff
docket gate drive claim
```

`start` creates a drive, validates and fingerprints the execution context, launches the first raw
run through the internal process service, and advances through at most one slice. `advance` resumes
the current attempt through at most one slice. Both return the same typed outcome document.

`handoff` requires the current owner claim, revalidates repository and process identity, invalidates
that claim, and writes a single-use transfer receipt. `claim` recomputes the identity, consumes that
receipt with a compare-and-swap transition, and returns a new opaque owner generation. A claim that
lost the race or no longer matches never acquires partial authority.

The exact CLI flag layout is implementation detail, but every operation takes opaque drive and
claim identifiers rather than asking an agent to pass process IDs, process-group IDs, raw state, or
deadlines. The typed protocol is versioned and shared by the CLI, Go application seam, and tests.

### Typed outcomes

Every successful `start` or `advance` operation returns exactly one workflow outcome:

| Outcome | Meaning | Permitted workflow action |
|---|---|---|
| `WAITING` | The same drive is live and safe to continue, but this slice ended. | The current owner advances again or creates an explicit handoff. |
| `PASSED` | The suite completed green against the recorded execution identity. | Consume the raw run for evidence or continue the task phase. |
| `FAILED` | The suite completed red and produced a trustworthy terminal record. | Enter the existing bounded repair policy. |
| `HALTED` | Safe automatic continuation is impossible. | Stop automation, retain diagnostics, and surface the typed cause. |

The document also carries the protocol version, opaque drive identifier, ownership generation,
attempt number, fixed deadline, and a typed cause where relevant. A passed result additionally
exposes the exact raw run directory required by the existing evidence operation. It never emits the
launch command, environment values, worktree diff, file contents, or ownership credential through
diagnostic prose.

Callers key on the typed document, not process exit status. An invocation that cannot parse its
arguments or read a recognized drive record is a command failure; a recognized `FAILED` or
`HALTED` drive is a workflow result, not an excuse to omit the result document.

## Durable drive record

### Location and privacy

Drive state lives below the repository's Git common directory, conceptually:

```text
<git-common-dir>/docket/gate-drives/v1/<opaque-drive-id>/
```

Using the Git common directory keeps the record outside every worktree while making it available to
agents operating in different linked worktrees of the same repository. The drive directory is
owner-only and its files are private. Writes use a sibling temporary file, durable flush where the
existing process layer requires it, and atomic rename. A lock plus a persisted generation provides
compare-and-swap semantics. Unknown schemas or impossible transitions halt rather than attempting a
best-effort migration.

Opaque identifiers are generated with enough entropy that an agent cannot collide with or guess a
different drive. User-supplied identifiers are validated before path construction; symlinks and
path traversal cannot escape the private root.

### Persisted execution identity

The initial record contains enough information to prove that every continuation still refers to the
same work:

- repository identity, canonical worktree path, change and task/phase identity;
- branch/ref and full HEAD object ID;
- a content fingerprint covering the index, unstaged tracked bytes, untracked file bytes, file
  modes, deletion state, rename state, and symlink values;
- the resolved command and working directory used for the raw launch;
- the authoritative configuration provenance and observation budget;
- a canonical hash of the inherited launch environment, without persisted environment values;
- creation time, fixed absolute deadline, last accepted clock value, and protocol version;
- current raw run directory, raw ownership identity, attempt number, relaunch count, and terminal
  receipt when present;
- current owner generation or single-use handoff generation.

The fingerprint stores hashes and structural metadata, not file or diff content. Symlinks are hashed
by link value rather than by following their target. The driver recomputes the fingerprint at every
ownership boundary and before accepting a terminal pass. A changed worktree invalidates the result;
the driver stops a process it can still prove it owns and returns `HALTED`.

The exact launch argument vector is private because it is required for the one safe relaunch. The
environment is not copied into the record. Relaunch is allowed only when the current canonical
environment hash matches the initial hash; otherwise the driver cannot prove equivalent execution
and halts. This trades availability for avoiding a private cache of ambient secrets.

### Deadline semantics

The deadline is computed once from the resolved `gate_observation_budget`. The per-invocation slice
uses Go's monotonic clock while that process is alive. Across invocations, the persisted UTC deadline
and last accepted clock value bind elapsed time. A forward clock jump may expire the drive; a
backward jump that could lengthen the budget halts rather than silently granting more time.

The production slice target is 30 seconds—materially below the observed 10-minute foreground-call
ceiling. It is implementation plumbing, not a user-facing configuration knob. Tests inject clocks
and shorter slices. A configured budget of zero preserves the existing contract: launch, take one
observation, and if the run is still live stop it and halt without inventing a verdict.

## State transitions and recovery

### Normal advancement

`start` performs the native launch handshake, persists the raw run identity, and begins the first
slice. During a slice, only a trustworthy native `running` state is retryable. If the raw run
becomes terminal between slices, the next `advance` reads that durable terminal state immediately.

When a slice expires first, the driver atomically records its latest observation and returns
`WAITING`. The suite continues under the detached supervisor. No background shell monitor, sleep
loop, notification subscription, or task wake-up is created.

When the raw run reports `passed`, the driver revalidates the execution fingerprint and records
`PASSED`. When it reports `failed`, the driver records `FAILED`. A native `stopped` state that was
not a proven transition initiated by this drive is `HALTED`, never red.

### Death and the single relaunch

`signaled` and `vanished` mean that the suite did not produce a verdict. The driver first uses the
native ownership record and stop/no-op semantics to prove that no owned process tree survives. If a
stop reports an already-terminal no-op, the driver re-observes and consumes that terminal state
before deciding anything else.

The driver may relaunch once only when all of these conditions hold:

1. the command is a workflow suite gate designated idempotent by the application contract;
2. the former process tree is proven gone;
3. no relaunch has already occurred;
4. the worktree, command, configuration, and environment identities still match;
5. the original deadline has time remaining.

The second raw run belongs to the same drive and original deadline. It is never started alongside
the first. A second death, uncertain stop, identity mismatch, exhausted deadline, or unreproducible
launch returns `HALTED` and preserves both attempts' diagnostics.

### Deadline, malformed state, and interruption

When the original deadline expires with a raw run still live, the driver calls the native stop
primitive, re-observes the result, and records `HALTED`. Deadline expiry does not earn a relaunch.
If ownership cannot be proven strongly enough to stop, the halt says so explicitly and retains all
records for operator recovery.

An unreadable or unrecognized native observation, corrupt drive transition, unknown schema, or
process-identity disagreement is terminal `HALTED`; only the exact `running` state is retryable.

Unexpected termination of a short-lived driver invocation is not deliberate abandonment. The
detached suite remains supervised, and the next invocation resumes from the last atomic record.
Deliberate workflow abandonment, identity drift, or deadline expiry stops any process tree whose
ownership can be proven. Raw operator `recover` and `stop` remain available when automation cannot
prove enough to act.

## Explicit handoff and nearest-owner continuation

### `WAITING` is not permission to replace an agent

A plain driver `WAITING` result leaves the current ownership generation valid. That owner may call
`advance` again. Another agent cannot claim the drive merely because it can see that the suite is
running.

Before an owner returns control, it must call `handoff`. The driver verifies that the owner token is
current, recomputes the repository fingerprint, records the workflow phase, invalidates the old
token, and creates a single-use handoff receipt. The old agent must then return and perform no more
work. A new agent calls `claim`; exact fingerprint validation and compare-and-swap consumption make
only one claimant authoritative.

The fresh agent inherits the same worktree and durable machine record, not the old transcript. Its
brief contains the change/task identity, workflow phase, and opaque handoff identifier. It trusts
the terminal receipt and repository fingerprint after claiming them; it does not trust prose saying
that tests passed or that a process is still alive.

Dirty pre-commit task work is a supported handoff state. No WIP commit is created solely to move
ownership. Staged, unstaged, untracked, mode, rename, deletion, and symlink differences are all part
of the identity and therefore must match exactly at claim time and terminal consumption.

### Waiting moves only as far upward as necessary

The normal build flow is:

1. A named task worker starts or advances its task gate.
2. While it can remain active, that worker consumes `WAITING` and advances the same drive again.
3. If the worker must unwind, it creates a handoff and returns a structured task-level `WAITING`
   report containing the task, phase, drive, and handoff identifiers.
4. The build controller—the nearest live owner—claims the handoff and drives the same suite through
   short calls. Waiting consumes neither the task's repair allowance nor its one escalation.
5. When agent judgment is required again, the controller dispatches a fresh named worker for the
   same task and same worktree with an explicit continuation. A trusted pass is not rerun merely
   because the transcript changed.

The build controller owns its final full-suite gate directly and advances it in slices. The
implement-next agent does the same for evidence re-mints and any re-gate after review changes.
Finalize's agent remains alive while the continuation-aware application operation returns
`WAITING`; it invokes the same phase again rather than restarting the whole finalize workflow.

Only if the nearest owner itself must unwind does it create another explicit handoff to its parent.
Restarting the outermost implement-next agent is therefore a last-resort recovery path. Finishing a
gate never normally restarts implement-next, discards its completed phases, or creates another
orchestrator.

## Workflow migration

### Build task workers

The `docket-build-task` return vocabulary gains `WAITING` alongside `COMPLETE`,
`NEEDS_ESCALATION`, and `BLOCKED`. A valid task-level `WAITING` report must name an explicit driver
handoff; stale “still waiting” prose is not a valid return. The contract forbids direct raw-gate
verbs, background suite processes, agent-authored polling, and notification waits.

On `PASSED`, the current or freshly continued worker completes its self-review and commit contract.
On `FAILED`, a worker receives the trustworthy failure and existing repair limits apply. On
`HALTED`, it returns the established blocking/escalation disposition with the driver's cause and
diagnostic handle. `WAITING` itself is neither repair nor escalation.

### Build controller and implement-next

`docket-build` replaces the executable Bash fence and JSON parsing instructions with one typed
driver contract. The controller understands task-level `WAITING`, owns the continuation while the
worker is absent, and prevents another task from starting in the shared worktree.

The controller's final gate uses the same driver and remains responsible for the configured whole
suite. A passed task gate does not substitute for that final gate. The raw run directory from the
final pass remains available to the existing evidence operation.

Implement-next's evidence re-mint and every post-review re-gate use the driver rather than copying a
launch/observe loop. A gate pass is only a phase result: the agent must still record and verify
evidence, review, publish the branch, publish the PR, and run `mark-implemented`. Only the existing
run-completion postconditions make the overall workflow complete.

### Finalize

`processFinalizeGate.observeToTerminal` and its 30-minute synchronous polling loop are removed.
The production `FinalizeGate` composes the shared driver service and admits a nonterminal waiting
result. Its caller persists or carries the opaque continuation and re-enters the same local-gate
phase after a slice; it does not rerun completed rebase or repair phases.

Finalize still resolves `finalize.test_command` from authoritative config, never from agent input.
Only a trusted `PASSED` result can feed `EvidenceRecord`. A red result feeds the existing bounded
integration-repair policy; waiting does not. The post-repair gate resumes through the same driver
contract.

### Raw caller contract and generated surfaces

The Bash fence in `gate-caller-loop.md` retires, along with `jq` as a dependency of workflow gate
sequencing. Its replacement documents the typed driver operations and dispositions. Primitive and
operator documentation continues to explain the raw verbs without presenting them as workflow
recipes.

All generated skill copies, manifests, agent definitions, managed dispatch text, and golden assets
derived from the authored contracts are regenerated mechanically. Historical specs, results, and
accepted ADRs remain point-in-time records and are not rewritten.

## Local `run-waiting` verdict

Waiting does not edit the change file, add a status, refresh `claimed_at`, or move a board card. The
change remains `in-progress`, and the board continues to show that ordinary state.

The run-verification operation gains one success-shaped report:

```text
run-waiting <change-id> <opaque-handoff-id> <phase>
```

Like the other run-verification verdicts, it is one line, is consumed by its spelling rather than
its exit code, and does not expose an owner credential or command. It means “a safe local
continuation exists,” not merely “a process might still be running.” It is emitted only when all of
the following independently agree:

- the change is still the claimed in-progress change expected by the workflow;
- its recorded branch and linked worktree exist and match the handoff;
- HEAD and the full dirty-worktree fingerprint match the drive and handoff receipts;
- the driver record is recognized, has an explicit unclaimed handoff, and has not exceeded a live
  deadline unless a durable terminal result is already waiting to be consumed;
- the referenced raw run and native ownership receipt match the active driver attempt;
- the handoff's change, task/phase, drive, and generation form one unambiguous chain.

Completed run postconditions take precedence over a stale local handoff. A valid `run-waiting`
precedes ordinary `run-incomplete`; a deliberate persisted run-halt remains terminal. Missing local
state on another machine does not invent waiting.

Top-level dispatch consumers learn the new verdict. They never treat it as completed, failed, or as
permission to start another change. When an exact continuation dispatch is available, they resume
that handoff; otherwise they report the waiting continuation and stop safely. This outer path is a
recovery seam only—the nearest workflow owner should normally consume waiting before it reaches the
top-level run gate.

Delegated runner dispatch must recognize and faithfully relay the new verdict, but this change does
not redesign its separate agent-detachment protocol or observation budget.

## Evidence and cleanup

A driver pass exposes the exact terminal raw run directory. The existing evidence operation remains
an independent verifier: it checks the configured command, exact head, native terminal record, and
other evidence invariants rather than trusting the driver's label alone. Logs and the raw receipt
remain available until evidence recording and any review diagnostics are complete.

A relaunch preserves both attempts. Evidence can be minted only from the terminal passed attempt,
while the drive record links the dead first attempt and explains why the second was admitted. Failed
and halted attempts retain diagnostics under existing cleanup policy. The driver does not weaken
raw cleanup eligibility or recovery ownership.

## Guarding the architectural boundary

A derived whole-repository guard finds raw gate use by syntactic shape, then classifies each match
as executable workflow use, primitive implementation/test use, operator use, or point-in-time prose.
It does not hand-list today's call sites or search only Markdown. At minimum it covers:

- raw `docket gate launch`, `observe`, and `stop` command shapes in skills, agent definitions, and
  executable scripts;
- direct application-orchestration calls to `GateLaunch`, `GateObserve`, and `GateStop` outside the
  high-level driver layer;
- workflow copies that parse raw observation state or recreate a sleep/poll loop;
- task-level `WAITING` contracts that omit an explicit handoff identity.

The guard permits the raw CLI implementation, the native process/application primitive packages,
primitive-level tests and fixtures, operator diagnostics/recovery/cleanup documentation, and
immutable historical records. Categories are derived from architectural source shape; the guard
does not carry an allowlist of individual current files.

The guard is mutation-tested. A test injects a structurally valid direct raw call into a
workflow-shaped fixture and proves rejection. Separate mutations remove the driver call or handoff
requirement from an authored workflow contract and prove that its contract test becomes red. A
guard whose mutation remains green is not accepted.

## Verification strategy

### State-machine tests

Use table-driven Go tests with injected clock, slice, process, filesystem, and repository seams.
Cover every state/outcome pair and every forbidden transition, including:

- several `WAITING` slices retaining one drive, raw run, attempt, and fixed deadline;
- terminal pass or failure arriving between slices;
- zero observation budget taking one observation and then stopping a live child;
- forward and backward clock changes;
- malformed/unknown observation state failing closed;
- stop applied, stop no-op followed by re-observation, and uncertain stop;
- one admitted death relaunch under the original deadline and every reason a second launch is
  refused;
- driver-process interruption between atomic state writes;
- schema/version mismatch and corrupt transition records.

No regular unit test sleeps for production durations. Injected time makes deadline assertions exact
and mutation-friendly.

### Process integration tests

Run a real native supervisor with test-only short slices and a child that spans multiple slices.
Prove that:

- each driver invocation returns in its slice bound;
- the supervisor and child identity remain the same across invocations;
- terminating a driver CLI invocation does not terminate or duplicate the child;
- a fresh CLI process resumes from disk and consumes the eventual exact terminal status;
- a process-tree death permits at most one non-overlapping relaunch;
- deadline expiry stops the entire owned process tree;
- durable logs and passed-run evidence remain usable.

The integration test observes recorded PIDs/session identity from the native receipt. It never uses
process-name matching as its oracle.

### Handoff and repository tests

Exercise clean and dirty handoffs. Mutate staged bytes, unstaged bytes, untracked bytes, names,
deletions, executable modes, and symlink values one dimension at a time and prove each mismatch
rejects claim or terminal consumption. Prove that identical dirty state succeeds without a WIP
commit. Race two claimants against one receipt and prove exactly one wins.

Also prove that an old owner cannot advance after handoff, a plain `WAITING` drive cannot be claimed,
and a fresh owner can consume a terminal written while no agent was active.

### Workflow tests

Contract and integration tests cover the complete ownership chain:

- a task worker returns structured `WAITING` only after handoff;
- the build controller resumes that exact task and starts no competing task;
- waiting consumes neither repair nor escalation;
- a fresh task worker accepts a matching continuation and finishes its commit contract;
- controller-owned final gate, implement-next evidence re-mint, review re-gate, and finalize local
  gate all use the high-level driver;
- a waiting run can eventually proceed through evidence, review, branch and PR publication, and
  `mark-implemented`;
- `run-waiting` appears only for a fully agreeing local receipt chain, and every receipt mutation
  makes it disappear;
- top-level and delegated verdict consumers neither mark waiting complete nor launch unrelated work.

Finalize tests specifically prove that one application call returns after a slice and that resuming
the local-gate phase does not repeat completed rebase or repair work.

### Repository gates

Run the architectural boundary guard and its mutations, authored-to-generated asset drift tests,
relevant Go tests under the race detector where supported, shell contract shards, and the configured
whole suite. Investigate every trailing `OVER BUDGET:` report even though it is not itself a suite
failure.

## Migration and compatibility

Implementation may build the internal state machine in stages on the feature branch, but the merge
boundary is atomic:

1. land the versioned driver model, persistence, clock, ownership, and repository-fingerprint
   seams with tests;
2. add the shared Go service and typed CLI adapters;
3. migrate build tasks, the build controller, implement-next re-mints/re-gates, and finalize;
4. add `run-waiting` and update every verdict consumer;
5. replace the Bash caller contract, remove workflow `jq` parsing, and regenerate derived assets;
6. enable the whole-repository structural guard and its mutation proofs;
7. record the ADR and run the complete verification gate.

No merged workflow may choose between the raw loop and the driver. Existing raw verbs and their
protocol remain backward-compatible for operators and primitive tests. A new driver does not adopt
an arbitrary legacy raw run lacking a matching drive receipt; such a run remains an operator
recovery concern rather than grounds for inferred ownership.

## ADR relationship

The implementation records a new ADR for first-class structured waiting, explicit ownership
handoff, and nearest-owner continuation.

ADR-0024 remains correct for unstructured background work: a forked agent still cannot yield and
expect a notification from its child. The new design does not add that channel. It makes every short
call synchronous, stores the continuation outside the transcript, and requires a deliberate
handoff before another agent can act.

ADR-0095 remains authoritative for raw native supervision. This change composes that process layer;
it does not replace or weaken its session, handshake, ownership, terminal-state, stop, or recovery
guarantees.

## Out of scope

- Making the full suite faster or implementing suite sharding.
- Moving build-controller, task-worker, repair, review, or publication judgment into Go.
- Adding a daemon, background notification bus, or same-transcript wake-up mechanism.
- Adding `waiting` to change frontmatter or changing board lifecycle states.
- Creating WIP commits solely to transfer dirty task work.
- Redesigning the delegated runner's own detachment protocol; only its consumption of
  `run-waiting` changes.
- Reworking the native supervisor's platform/session implementation or raw protocol.
- Parallelizing task workers in one shared worktree.
- Adopting unknown pre-migration raw runs into a drive.

## Acceptance criteria

The change is complete when all of the following are true:

- no production workflow contains a full-budget synchronous polling loop;
- no executable workflow calls or composes raw gate primitives outside the driver boundary;
- every foreground driver call is slice-bounded while one suite continues across calls;
- one persisted deadline and execution fingerprint survive CLI restart and agent handoff;
- only an explicit exact-match handoff can authorize a fresh owner;
- waiting normally terminates at the nearest workflow controller and never consumes repair or
  escalation;
- test failure, process death, deadline expiry, and ambiguous ownership remain distinct outcomes;
- at most one safe, non-overlapping relaunch occurs under the original deadline;
- `run-waiting` is local, read-only, receipt-derived, and understood by every run-verdict consumer;
- finalize, evidence re-mint, post-review re-gate, and build gates all use the shared driver;
- a gate pass still cannot bypass evidence, review, publication, or `mark-implemented`;
- the architectural guard is mutation-proven and the configured whole suite passes.
