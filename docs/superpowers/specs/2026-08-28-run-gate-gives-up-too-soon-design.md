<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0359 — Run gate gives up too soon](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0359-run-gate-gives-up-too-soon.md)**
<!-- docket:backlink:end -->

# Tracked test continuation and parent takeover

**Change:** 0359 · **Date:** 2026-08-28 · **Type:** fix · **Priority:** high

## Problem

`docket-implement-next` is a hierarchy of agents. A top-level harness dispatches the implement-next
controller, and that controller dispatches task workers. Those agents routinely run tests that
outlive one agent call.

Docket already has the correct process layer for those tests. The native gate driver starts a
detached, supervised process, persists its identity and exact terminal result, and lets successive
short calls observe the same run. Its production observation slice is 30 seconds and its default
overall observation budget is 30 minutes.

The workflow does not use that layer consistently or transfer ownership soon enough. A task worker
uses the driver only when it predicts that a focused test may be long. After a `WAITING` slice it may
continue observing by itself. If the worker or its implement-next controller returns while the test
is still running, the parent sees a completed child call but no explicit handoff. The process can
finish normally, yet no live agent remains to consume the result, commit the work, and continue the
workflow.

The outer run gate then samples only durable completion postconditions. Without a valid unclaimed
gate-drive handoff, the work reads as `run-incomplete`. That can consume the one retry and then
become terminal `gate-stop` even though the tracked test is healthy. Change 0333 showed this while
`go test -race ./internal/app` was still running; change 0363 showed the same shape with
`go test -tags integration ./internal/app/`, where the coordinator returned, the OS test process
continued, and nobody was alive to consume its eventual result.

A second defect makes recovery worse. A gate armed before explicitly resuming an already
`in-progress` change cannot attribute that run: current attribution only accepts a claim stamped
after the gate's dispatch epoch. The recovery path the gate asks a human to take therefore loses the
gate key's attribution and retry accounting.

## Goals

- Track every test execution in implement-next through Docket's native gate driver from launch to
  terminal consumption.
- Make a task worker transfer a test to its controller after the first 30-second `WAITING` slice.
- Let the nearest parent safely recover the same drive when its direct child returns before making
  the normal handoff.
- Keep a live tracked test nonterminal at the outer run gate and preserve the same gate key without
  spending the retry.
- Attribute an explicit resume of an already-in-progress change to the newly armed gate key.
- Use one Docket-owned protocol across Codex, Claude, Cursor, and OpenCode without requiring a
  harness-native child-tree query.

## Terms

- A **test execution** is a command run for test or verification intent inside implement-next. The
  workflow role determines that intent; Docket does not guess from command spelling.
- A **drive** is the existing durable gate-driver record for one logical test execution.
- An **owner** is the agent currently authorized to advance that drive.
- A **parent** is the controller that synchronously dispatched that owner as its direct child.
- A **normal handoff** is the existing owner-authorized `handoff` followed by the next owner's
  fingerprinted, single-use `claim`.
- A **parent takeover** is the new exceptional transfer used only after the parent's direct child
  has returned or stopped without producing the normal handoff.
- A **slice** is one driver observation call of at most 30 seconds. It is not a test timeout.

## Existing invariants retained

ADR-0024 still forbids a forked or dispatched agent from backgrounding work and yielding for a
notification. ADR-0095 remains authoritative for native session ownership, process identity, and
the exact terminal record. ADR-0098 remains the basis for structured `WAITING`, fixed deadlines,
fingerprinted ownership, and nearest-owner continuation. ADR-0075 remains the conservative run-gate
attribution baseline.

This change adds a parent-authorized recovery path to ADR-0098's cooperative-only transfer model.
Because that is a material ownership-policy extension, implementation records a new ADR that
supersedes ADR-0098 while preserving its normal handoff and fail-closed guarantees.

The following invariants do not change:

1. The native supervisor is the sole authority for the test process and its terminal status.
2. Repository HEAD and the full dirty-worktree fingerprint must match across every continuation.
3. At most one owner can advance a drive, and at most one raw process tree can be live for it.
4. `FAILED` means the test itself completed red. Process uncertainty, ownership ambiguity,
   deadline expiry, and malformed state are `HALTED`.
5. The fixed overall deadline never resets during slices, handoffs, takeovers, agent resumptions, or
   the driver's existing one permitted process relaunch.
6. A passed test is a phase result, not implement-next completion. Evidence, review, publication,
   PR creation, and `mark-implemented` still have to finish.

## Design

### 1. Every implement-next test uses the native driver

Every test execution launched by implement-next or one of its task/repair roles starts through the
high-level gate driver. This includes:

- baseline, RED, GREEN, focused, and ad-hoc tests run by a task worker;
- integration-repair tests and re-tests;
- the configured full-suite build gate;
- evidence re-mints and post-review re-gates.

There is no duration prediction and no threshold before the driver applies. A quick test normally
returns on the driver's next process observation, currently about 250 ms. A slower test may consume
the rest of the first slice and return `WAITING`. Thirty seconds is therefore the maximum duration
of one observation call, not the minimum duration of a test.

The workflow determines which commands are tests. The implementation must not maintain a list such
as `go test`, `pytest`, or `npm test`, because focused and ad-hoc commands vary by repository and a
spelling list will miss the command that matters.

This remains a test driver, not a general-purpose command runner. Ordinary editing commands and
non-test shell operations keep their existing execution path.

### 2. First `WAITING` always transfers to the build controller

A task worker may consume the first synchronous driver call. Its disposition is handled as follows:

| First result | Worker action |
|---|---|
| `PASSED` | Continue self-review and the task commit. |
| `FAILED` | Apply the existing task repair policy. |
| `HALTED` | Return the existing blocking disposition with the typed cause. |
| `WAITING` | Immediately perform a normal handoff and return structured `WAITING`. |

After the first `WAITING`, the task worker must not call `advance` again. It hands the drive to the
build controller even if the worker could theoretically remain active. This deterministic boundary
keeps a long test from being hidden below a nested task worker.

The build controller claims the same drive and advances it in 30-second slices. It starts no other
task in the shared worktree. When the drive becomes terminal, the controller dispatches a fresh
worker for the same task and worktree when agent judgment is needed:

- `PASSED`: consume the trusted result and finish self-review/commit without rerunning the test;
- `FAILED`: continue the existing bounded repair path with the real red result;
- `HALTED`: stop automation and retain the diagnostic handle.

`WAITING` consumes neither repair nor escalation budget.

Controller-owned full-suite, re-gate, and evidence drives use the same short calls, but they do not
move downward to a task worker merely to wait. If the controller remains live, it continues them
itself.

### 3. Prepare parent recovery authority before each dispatch

Normal handoff cannot recover an owner that vanished before calling `handoff`. Docket therefore
adds a durable **recovery scope** at each relevant parent/child dispatch boundary.

Before dispatching a task worker, the build controller prepares a scope for that exact change,
task, phase, branch, worktree, and run-gate key. Preparation returns two separate opaque
capabilities:

- a child capability passed to the worker and bound to any drive it starts in that scope;
- a parent recovery capability retained by the controller and never exposed to the worker.

The top-level `gate-before` operation prepares the equivalent outer scope for the dispatched
implement-next controller. On a fresh run the scope begins unbound and may bind once to the one
claim the existing conservative attribution rule accepts. On an explicit resume it is pre-bound to
the verified in-progress change. The gate key identifies the parent's durable record; the
dispatched controller receives only its child-side continuation context. Every nested drive remains
linked to the same change and outer gate key.

Recovery scopes follow the current owner. When a task worker normally hands a drive to the build
controller—or the controller takes it over—the task scope closes and the drive activates the
controller's already-prepared outer-parent scope. The top-level parent can therefore recover work
only after its direct implement-next child returns; it cannot use its capability to skip the live
controller and seize a worker-owned drive.

A recovery scope may bind at most one nonterminal drive. Scope preparation and binding are durable
compare-and-swap transitions. Unknown schemas, mismatched change/task/phase identity, a second live
drive, or a missing capability fail closed.

The exact CLI flag spelling is an adapter detail, but the Go service and every harness-facing
adapter must share this state model. Parent recovery credentials must never appear in child prose,
drive human text, logs, or run-verdict output.

### 4. Parent takeover is event-authorized, not time-authorized

Normal first-slice handoff remains the expected path. Parent takeover is allowed only when all of
these conditions hold:

1. the parent has received the harness event that its directly dispatched child returned or
   stopped;
2. that child did not provide a valid normal handoff;
3. the parent's recovery capability names the exact prepared scope;
4. the scope contains exactly one nonterminal drive;
5. branch, worktree, HEAD, full dirty fingerprint, raw-run identity, fixed deadline, change, task,
   phase, and outer gate key still agree.

The harness event supplies the fact that the direct child has stopped. A quiet period, heartbeat,
claim age, process age, retry count, or 30-second slice ending never supplies that fact.

On an accepted takeover, Docket atomically invalidates the child's owner generation and mints a new
generation for the parent. A stale child call then fails as owner-superseded and cannot advance the
drive. If the compare-and-swap loses a race or any identity is ambiguous, takeover returns `HALTED`
and does not launch, stop, or duplicate the test.

This mechanism does not need a harness API that enumerates the whole child tree. The workflow
already has the direct synchronous dispatch boundary: the controller knows when its task-worker
dispatch returned, and the top-level caller knows when its implement-next dispatch returned. The
recovery capability proves which durable scope that parent is allowed to recover.

The nearest-parent rule is therefore mechanical:

```text
task worker returns without handoff
└── build controller takes over the task drive

implement-next controller returns without handoff
└── top-level gated caller takes over the outer continuation
```

A grandparent cannot skip a still-live parent. Deterministic first-`WAITING` transfer ensures a
long task test normally reaches the build controller before the outer boundary is involved.

### 5. The outer run gate continues tracked work instead of spending retry

`gate-verdict` keeps `RunVerify` as the durable-postcondition authority. Before mapping an
incomplete run to retry accounting, the attributed facade checks the gate key's recovery scope.
Because the facade is called only after the implement-next dispatch returned, it may perform the
parent takeover transition described above. A successful transition creates the same fully
receipt-backed continuation that `RunVerify` can validate as `run-waiting`.

After a successful outer takeover, the facade immediately creates a normal single-use handoff for
the resumed implement-next owner. `RunVerify` can then validate the existing fully receipt-backed
`run-waiting` predicate; it does not need a generic liveness branch. Add a nonterminal gate decision
for that case:

```text
gate-continue <key> run-waiting <change-id> <continuation-id> <phase>
```

`gate-continue` means the same implement-next attempt owns live or terminal-unconsumed tracked work.
It preserves the gate record, the attributed change id, and the unused retry marker. It never
authorizes a new claim or an unrelated implement-next run.

The continuation id is a redacted, single-use locator understood by the facade; it is not an owner
generation, parent recovery capability, raw run directory, or command. The top-level caller passes
it through unchanged and continues the exact change and phase:

- resume the existing implement-next agent when the harness supports a real resume; or
- dispatch `docket-implement-next` again with the explicit change id, drive continuation, and same
  gate key when the harness cannot resume the old agent.

The second form is a continuation dispatch, not `gate-retry-once`. It must claim the recovered drive
before doing other work, must not launch a replacement test, and does not consume retry. The gate
key remains active until implement-next reaches a true terminal disposition.

Only a `run-incomplete` result with no valid tracked drive, no terminal result waiting to be
consumed, and no recoverable continuation may consume the existing single retry. `gate-stop`
remains appropriate for a second genuinely quiescent incomplete return, `run-halted`, or unsafe
ownership.

The generated parent contract must no longer map `run-waiting` to terminal `gate-stop`. It must
recognize `gate-continue`, carry the same key, and distinguish it structurally from
`gate-retry-once`.

### 6. Explicit resume attribution

Extend gate arming with an optional explicit change id for a user- or continuation-requested resume.
When supplied, `gate-before` re-syncs metadata and binds `AttributedID` only if that exact change is
currently `in-progress` and its branch/worktree identity is valid. It does not use timestamps to
pretend the old claim was new.

Without an explicit id, existing conservative new-claim attribution remains unchanged: before-set,
dispatch epoch, cardinality, and fail-closed ambiguity still apply.

A continuation under an existing key never re-arms. It reuses the already bound id and retry state.
An explicit id is authorization to verify that requested change, not authority to bypass its claim,
branch, worktree, or run-verification predicates.

### 7. Slice and deadline policy

Keep the production slice at 30 seconds. Raising it to one minute gives no correctness benefit and
only doubles the longest foreground call. Fast tests still return immediately, while slow tests
hand off after at most one 30-second slice.

Keep `gate_observation_budget` configurable with its current 30-minute default. It is the overall
deadline for one logical drive, not an agent timeout. If a test is still running when that deadline
expires, the driver applies its existing ownership-proven stop path and returns `HALTED`. Deadline
expiry is not a test failure and does not earn run-gate retry.

## Four-harness contract

The design depends on only three harness facts:

1. a parent can synchronously dispatch a named child;
2. the parent receives an event when that direct dispatch returns or stops;
3. the parent can either resume that child or dispatch the same named role with an explicit
   continuation.

It does not depend on native child-tree enumeration, periodic check-ins, task heartbeats, or a
specific tool name.

Current gate-execution evidence is not sufficient to declare support because it is version- and
mode-scoped. The implementation must re-probe the exact supported paths on the installed versions
at build time:

| Harness | Version to re-probe | Required path |
|---|---:|---|
| Claude Code | 2.1.251 | Both interactive and the forked/dispatched `docket-implement-next` path. |
| Cursor | 3.17.21 | Registered named-agent dispatch and continuation dispatch. |
| Codex | 0.150.1 | Named-agent dispatch, same-agent resume when available, and fresh continuation fallback. |
| OpenCode | 1.18.23 | Named-agent dispatch and continuation dispatch. |

`skills/docket-build/references/gate-execution.md` records the exact mode, version, invocation, and
result. A harness is supported only after its real implement-next path passes the acceptance probe;
an interactive-only observation cannot stand in for a forked or dispatched path.

Each harness probe covers:

1. a fast test that returns before 30 seconds;
2. a test spanning the first slice, with deterministic worker-to-controller handoff;
3. a worker that returns before handoff, followed by controller takeover of the same process;
4. an implement-next controller that returns, followed by top-parent continuation of the same
   process and same gate key;
5. no duplicate process, no new task, and no retry consumption while the drive is active;
6. terminal pass and terminal failure consumed by the correct resumed role;
7. explicit resume of an already-in-progress change remaining attributable.

If a harness cannot provide a trustworthy direct-child return event or cannot carry an explicit
continuation, autonomous implement-next is unsupported on that path. Docket must report that
capability gap; it must not substitute a timer or heartbeat guess.

## Verification strategy

### Driver and ownership tests

Add table-driven and race tests for recovery scopes and parent takeover:

- parent and child capabilities are distinct and redacted from every public document;
- a drive binds once to the exact change/task/phase/key scope;
- normal handoff still invalidates the old owner and claims once;
- parent takeover invalidates an owner only with the exact parent capability;
- old-owner `advance`, late normal handoff, two parent claimants, wrong parent, and grandparent skip
  all fail closed;
- every fingerprint dimension, branch/head drift, raw-run mismatch, expired deadline, ambiguous
  drive, and malformed schema rejects takeover;
- a terminal result written after the child returned is consumed without rerunning the test;
- no accepted takeover launches or stops a second process.

Use injected time and process seams; regular tests do not sleep for production durations.

### Workflow and gate tests

Mutation-prove every widened clause:

- baseline, RED, GREEN, focused, ad-hoc, repair, final-suite, re-mint, and re-gate test paths all
  route through the driver by workflow shape rather than command spelling;
- the first task-level `WAITING` always hands off and never advances again in the worker;
- the controller starts no other task while a drive is waiting;
- a direct-child return without handoff reaches parent takeover, not repair, escalation, or retry;
- `gate-continue` is nonterminal, retains the same key, and cannot reach the retry-consumption CAS;
- only quiescent `run-incomplete` can emit `gate-retry-once`;
- an explicit resumed id is bound only when the current metadata identity agrees;
- unattributed observe mode remains structurally unable to authorize retry or continuation.

The whole-repository boundary guard continues to derive executable gate/test sites from syntactic
shape and classify them; it must not hand-list current filenames or test command spellings. Inject a
new direct test execution into a workflow-shaped fixture and prove the guard turns red.

### Process and harness acceptance

Use a real detached child in integration tests to prove that fast completion returns immediately,
the first slice returns by 30 seconds, takeover keeps the same PID/session/run identity, and terminal
consumption occurs from a fresh process. Run the four real harness probes above and update their
version-scoped evidence before merge. Finally run the configured whole suite and inspect every
authoritative serial budget finding.

## Migration

The merge boundary is atomic:

1. add recovery-scope persistence, capability separation, and parent takeover with state-machine
   tests;
2. make gate-before create the outer scope and support explicit resume attribution;
3. route every implement-next test-intent path through the driver;
4. enforce first-`WAITING` task handoff and controller ownership;
5. add `gate-continue`, preserve retry across continuation, and update every generated consumer;
6. record the superseding ownership ADR and regenerate authored-to-installed surfaces;
7. re-probe all four harnesses and run the whole suite.

No merged intermediate may allow both direct test execution and driver execution as supported
workflow paths, or map a tracked live drive to both continuation and retry.

## Out of scope

- Detecting or recovering an agent that disappears while only editing code and has no tracked test
  drive. The existing foreground-parent/no-yield rule still governs that case.
- Replacing `RunVerify`'s durable completion postconditions with generic agent liveness.
- Treating heartbeat freshness, elapsed time, log activity, claim timestamps, or OS process-name
  searches as proof an agent is alive or dead.
- Adding native child-tree queries, a daemon, a notification bus, or periodic 30-second parent
  check-ins.
- Turning the gate driver into a general shell-command runner.
- Changing the configured 30-minute default budget, making the suite faster, or redesigning the
  native supervisor.
- Parallel task workers in one shared worktree or WIP commits solely for ownership transfer.

## Acceptance criteria

Change 0359 is complete when:

- every implement-next test starts through the native driver without duration prediction;
- quick tests return immediately and every observation call is bounded by the 30-second slice;
- the first task-level `WAITING` always transfers to the build controller;
- a direct parent can recover the same drive after its child returns without handoff, while every
  timer-only or ambiguous takeover fails closed;
- a tracked drive produces nonterminal `gate-continue`, keeps the same gate key, and spends no
  retry;
- no continuation duplicates the test or launches an unrelated implement-next run;
- explicit resumes of already-in-progress changes are attributable;
- the 30-minute overall deadline remains fixed and expires to `HALTED`, not `FAILED` or retry;
- generic non-test agent disappearance remains outside the recovery mechanism;
- the exact supported Codex, Claude, Cursor, and OpenCode paths pass the version-scoped acceptance
  probe;
- ownership, workflow, structural-guard, integration, race, generated-surface, and configured
  whole-suite tests pass.
