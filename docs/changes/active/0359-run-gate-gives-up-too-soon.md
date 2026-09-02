---
id: 359
slug: run-gate-gives-up-too-soon
title: 'Run gate gives up too soon'
status: 'in-progress'
priority: high
type: fix
created: 2026-08-27
updated: '2026-09-02'
depends_on: []
stacked_on:
related: [237, 334, 342, 363, 396]
discovered_from: [333]
adrs: [24, 75, 95, 98]
spec: docs/superpowers/specs/2026-08-28-run-gate-gives-up-too-soon-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/run-gate-gives-up-too-soon'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-02T15:26:20Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-28-run-gate-gives-up-too-soon-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-28-run-gate-gives-up-too-soon-design.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0075](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0098](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0098-structured-gate-waiting-and-ownership-handoff.md) |
<!-- docket:artifacts:end -->

## Why

`gate-verdict` can escalate a healthy, still-running implement-next run to a terminal
`gate-stop` — forbidding any re-dispatch — while the build worker is legitimately still
working and simply has not committed its effects yet.

The gate verdict is a **durable-postcondition snapshot with no liveness or in-progress-activity
awareness**. `RunVerify` (`internal/app/run_verify.go`), the sole verdict authority, derives every
verdict (`run-complete` / `run-incomplete` / `run-halted` / `run-waiting` / `run-unclaimed`) purely
from committed state: git head, remote head, recorded PR, evidence receipts, `## Run halted`
markers, and fingerprinted gate-drive receipts on disk. There is deliberately **no** PID / process
/ signal check anywhere — `run_verify.go` even states it: `run-waiting` means "a safe local
continuation *exists*," not "a process might still be running." So the instant it is sampled during
a long **inline** measurement or build phase that has not yet committed, the run reads
`run-incomplete / not-implemented`, and once the single retry permit is spent
(`rungate_verdict.go`, `ConsumeGateRetry`), the next `run-incomplete` becomes a terminal
`gate-stop`. The verdict is correct *per its own contract* but **premature relative to a genuinely
progressing run**, and it fires exactly when the run is doing legitimate slow work (which is often
the whole point of the change being built).

This surfaced live implementing change 0333. Observed:

- Run 1 stopped early (parent yielded awaiting the plan-writer) → `gate-retry-once`.
- Run 2 (the retry) advanced further — the plan committed — but was sampled while the build worker
  was mid-flight running `go test -race ./internal/app` **inline and blocking** (a ~227s default /
  ~264s race package, which is precisely the >600s-ceiling problem 0333 exists to fix) → the durable
  snapshot read `run-incomplete / not-implemented`, and with the retry already spent →
  **`gate-stop`**.
- Process trace at that moment confirmed the worker was alive, un-orphaned, and progressing;
  moments later Task 1 committed (`36239f39`) and the log showed the measurement completing normally.

Net effect: the contract shut the door on a run that was fine, and — because `gate-stop` forbids
re-dispatch — a human is now required to recover a run that needed no intervention. The retry budget
is also spent on non-signal (a slow-but-healthy phase), so it is unavailable for a genuinely failed
retry later.

### Recurrence evidence (this session) — the gate never once told the truth about a healthy run

The 0333 build was driven through **four** dispatches. In every one, the parent's prose and the
gate verdict read as "waiting / incomplete / stop," while the fan-out build workers were in fact
healthy and committed their tasks. The gate systematically **under-reported a run that was
succeeding task-by-task** — it never emitted a "this run is progressing, keep going" signal, only
`retry-once`, `stop`, or (on resume) `no-attributable-claim`.

| Dispatch | Parent prose read as | Gate verdict | What actually landed in git |
|---|---|---|---|
| 1 (initial) | waiting on plan-writer | `gate-retry-once` | plan-writer healthy; plan committed `606fca6c` |
| 2 (the retry) | waiting on Task 1 worker | `gate-stop` (retry spent, terminal) | Task 1 worker healthy; committed `36239f39` seconds later |
| 3 (human resume A) | "blocking on Task 5 worker" | attributed `gate-done no-attributable-claim`; observe `run-incomplete` | Tasks 2–5 all committed: `df74c284`, `b1bbdfe2`, `b5d9ffd8`, `48066711` |
| 4 (human resume B) | in progress | — | continuing Task 6→8 |

The point is not any single premature stop — it is that across a multi-hour, five-task build that
was **working the whole time**, the gate produced zero true-positive "healthy" readings and one
false-terminal `gate-stop`. A point-in-time durable snapshot taken against a worker that commits
seconds later is structurally a coin-flip: dispatch 2's `gate-stop` and dispatch 3's Task-1-already-
done state were the same run, read moments apart, giving opposite pictures.

### Second defect: the gate has no attributed verdict for a *resumed* in-progress run

`gate-stop` forbids re-dispatch and says "a human is needed." But when the human does exactly that —
arms a fresh gate and re-dispatches to resume the already-`in-progress` change — the attributed
`gate-verdict <key>` returns **`gate-done no-attributable-claim`**. Attribution binds only a claim
whose `claimed_at` is **at or after the dispatch epoch** (`rungate_verdict.go`
`attributeGateClaim`), and a resumed change was claimed long before, so there is *nothing to
attribute*. The gate's own prescribed recovery path therefore lands in a blind spot the gate cannot
verify: the CLAUDE.md bracket (`gate-before` → dispatch → `gate-verdict <key>`) is unusable for a
resume, and you must fall back to observe mode (`gate-verdict --unattributed <id>`), which has no
retry accounting and cannot authorize anything. So the gate both (a) forces a human handoff on a
healthy run and (b) cannot then track the handoff it forced. These two defects compound: the
premature `gate-stop` is what pushes you into the un-attributable resume path.

## What changes

Run every test launched by implement-next through Docket's native gate driver, including baseline,
RED/GREEN, focused, ad-hoc, repair, configured full-suite, evidence, and re-gate tests. Do not try to
predict which test will be slow. The 30-second driver slice is a maximum observation call, not a
minimum test duration; quick tests return as soon as they finish.

Make the ownership boundary deterministic. If a task worker's first slice returns `WAITING`, it
always hands the same drive to the build controller. The controller observes it to a terminal
result, then resumes the same task without rerunning a passed test. If the worker returns before
making that handoff, the controller uses a parent-only recovery capability prepared before dispatch
to take over the same drive. If the whole implement-next controller returns, the top-level gated
parent performs the equivalent takeover. These transfers are authorized by the direct child's real
return event, never by a timer, heartbeat, or quiet log.

Teach the run-gate facade that a tracked drive is a nonterminal continuation. It keeps the same gate
key, does not consume `gate-retry-once`, and resumes the same implement-next attempt. Only a truly
incomplete run with no tracked work alive may spend the retry. Extend `gate-before` with explicit
resume attribution so an already-`in-progress` change can remain bound to a fresh key.

Verify the complete behavior through the real named-agent paths in Codex, Claude, Cursor, and
OpenCode. The design does not require any harness to expose its child tree; it uses Docket's durable
driver, parent-scoped recovery capability, and the direct dispatch-return event every supported path
must provide.

## Design decisions

- Every test uses the driver from launch; test intent comes from the workflow, not command spelling.
- Keep the 30-second production slice and configurable 30-minute default overall deadline.
- A task worker always hands off after the first `WAITING` slice.
- Normal fingerprinted handoff remains preferred; parent takeover is the exceptional missing-
  handoff path and invalidates the old owner atomically.
- A live or terminal-unconsumed tracked drive produces nonterminal `gate-continue`, never
  `gate-retry-once` or terminal `gate-stop`.
- Resume the existing implement-next agent when the harness supports it; otherwise dispatch the same
  role for the same change with the explicit continuation and same key. That is continuation, not
  retry.
- Re-probe all four harnesses in their exact supported modes and installed versions before calling
  the design portable.
- Record a new ADR superseding ADR-0098's cooperative-only ownership-transfer decision while keeping
  its structured-waiting and fail-closed guarantees.

## Out of scope

- Recovering an agent that disappears while only editing code and owns no tracked test drive.
- Replacing `RunVerify`'s durable-postcondition model with generic agent liveness.
- Timer-, heartbeat-, log-activity-, claim-age-, or process-name-based liveness guesses.
- Requiring native child-tree status from a harness, adding a daemon/notification bus, or making the
  driver a general shell-command runner.
- Changing the 30-minute default observation budget, speeding up the suite, or redesigning the
  native process supervisor.

## Reconcile log

### 2026-09-02

### 2026-09-02 — reconcile (docket-implement-next)

Confirmed the design holds against current `main` and is still needed; not invalidated, not obsolete.

**Current-reality anchors (verified in code):**

- Both target defects are still live. `internal/app/rungate_verdict.go` maps `VerdictRunWaiting` to a **terminal `gate-stop`** (the run-waiting to stop mapping section 5 must replace), and `attributeGateClaim` still binds only a claim whose `claimed_at >= DispatchEpoch`, so an explicit resume of an already-`in-progress` change yields `gate-done no-attributable-claim` (the section 6 resume-attribution gap). `gate-before` (`internal/app/rungate_before.go`) accepts only `target == "implement-next"` with no change-id/resume argument.
- No `gate-continue` decision exists anywhere in the Go tree; the run-gate verdict vocabulary today is `gate-done`/`gate-retry-once`/`gate-stop`/`gate-observe`.
- Change 0342 (done, ADR-0098) built the native gate driver foundation this change extends: `internal/gatedrive/{drive,driver}.go` with `Start/Advance/Handoff/Claim`, structured `WAITING`, fixed deadline, 30s slice, single relaunch, `OwnerGeneration` + single-use fingerprinted `HandoffGeneration`. There is **no** recovery-scope, parent/child opaque capability, or parent-takeover concept yet — sections 3/4 are net-new, including new `driveRecord`/store fields (schema-version bump) and a new driver operation + app/CLI seam.
- ADR-0098 (`docs/adrs/0098-*.md`) is still `Accepted`, `supersedes: []`, unsuperseded — section 6's new superseding ADR is net-new.
- The dispatch/gate-bracket contract is generated from the `run-gate.md` asset (`internal/assets/embedded/tree/cursor-rules/run-gate.md`, rendered by `internal/harness/dispatch.go`) into CLAUDE.md and the cursor rules; it currently maps `run-waiting` to a terminal stop and knows no `gate-continue`. Regenerating those surfaces is part of the work.
- Task routing today is prediction-gated in `skills/docket-build-task/SKILL.md` (driver used only when a test may outlast a single foreground call) and WAITING handling is discretionary — section 1 (every test via the driver, no duration prediction / no command-spelling list) and section 2 (first `WAITING` always hands off to the controller) are net-new. The `internal/repoguard` boundary from 0342 guards raw-primitive composition, not 'a workflow ran a test directly instead of via the driver'; the new structural guard classifying workflow-shaped test sites is net-new.

**Newly related:** added 396 to `related`. Change 0396 (done) added an adjacent finalize-side gate-continuation mechanism (rebase-receipt continuation pair, WAITING resume, clear-continuation-on-terminal, live-drive-overrides-evidence-skip). It is a distinct surface (finalize, not the implement-next run gate) but is directly relevant prior art for the continuation-id/continuation-pair persistence pattern section 5 introduces; the build should reuse its shape where it fits rather than diverge gratuitously.

**Scope / verification-boundary decision (autonomous run):** the Docket-owned mechanism — recovery-scope persistence + parent/child capability separation + event-authorized parent takeover, the `gate-continue` verdict and continuation-id, facade-side outer takeover, `gate-before` resume attribution, every-test-through-driver routing, first-`WAITING` handoff, the new structural guard, the superseding ADR, and the regenerated authored-to-installed surfaces (CLAUDE.md / cursor rules / build skills) — is fully buildable and covered by unit, table-driven, race, injected-time/process-seam, and real-detached-child integration tests, and gated by the configured whole suite. The design's **four-harness real-world acceptance probes** (Claude Code 2.1.251, Cursor 3.17.21, Codex 0.150.1, OpenCode 1.18.23; the 7 scenarios in the four-harness contract) validate only that each harness supplies the three required facts (synchronous named dispatch, a direct-child return/stop event, and same-agent resume or explicit continuation dispatch). Those probes require driving four separately-installed harnesses and cannot be performed inside a single autonomous implement-next run, and their evidence must never be fabricated. They are therefore treated as a **required human merge-gate verification**: `skills/docket-build/references/gate-execution.md`'s harness-acceptance rows are left as an explicit pre-merge TODO for a human, and the PR body + results file document the four-harness re-probe as the outstanding gate before merge. This is a verification-boundary carve, not a scope reduction of the mechanism: all mechanism code and its Docket-owned automated tests are in scope and must be suite-green before the PR opens. This honors the migration's atomic-merge invariant — a human runs the harness probes and, only then, merges the single atomic PR.
