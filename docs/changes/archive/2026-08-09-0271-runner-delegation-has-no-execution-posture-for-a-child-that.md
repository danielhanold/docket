---
id: 271
slug: runner-delegation-has-no-execution-posture-for-a-child-that
title: 'Runner delegation has no execution posture for a child that outlives its foreground call'
status: done
priority: high
type: fix
created: 2026-08-08
updated: 2026-08-09
depends_on: [269]
related: [223, 249, 237, 269, 231, 227]
discovered_from: [258]
adrs: [80]
spec: docs/superpowers/specs/2026-08-08-runner-delegation-detached-execution-posture-design.md
plan: docs/superpowers/plans/2026-08-09-runner-delegation-detached-execution-posture.md
results: docs/results/2026-08-09-runner-delegation-has-no-execution-posture-for-a-child-that-results.md
trivial: false
auto_groomable:
branch: feat/runner-delegation-has-no-execution-posture-for-a-child-that
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/188
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-08-runner-delegation-detached-execution-posture-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-08-runner-delegation-detached-execution-posture-design.md) |
| Plan | [2026-08-09-runner-delegation-detached-execution-posture.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-08-09-runner-delegation-detached-execution-posture.md) |
| Results | [2026-08-09-runner-delegation-has-no-execution-posture-for-a-child-that-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-08-09-runner-delegation-has-no-execution-posture-for-a-child-that-results.md) |
| PR | [#188](https://github.com/danielhanold/docket/pull/188) |
| ADRs | [ADR-0080](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0080-detached-delegation-execution-posture-launch-then-observe.md) |
<!-- docket:artifacts:end -->

## Why

### The defect

Runner delegation bounds an entire delegated agent run inside **one foreground call of the parent
harness**. Any delegated task that outlasts that call is killed mid-work, and the kill is
indistinguishable — to the caller — from the child having failed.

Three layers each hard-code the foreground posture:

1. **The generated shim wrapper.** `sync-agents.sh`'s `emit_shim` bakes this instruction verbatim
   into every runner-delegated agent wrapper:

   > Make exactly ONE foreground Bash call, with the maximum timeout (600000):
   > `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh runner-dispatch <flags> [-- <caller args>]`
   > Block until it completes — **never background it, never poll.**

   `600000` ms is not a tunable — it is the maximum the Claude Code Bash tool accepts. There is no
   larger value to raise it to. The wrapper further forbids both escapes: "never retry silently, and
   never run the skill inline on this harness as a fallback."

2. **The facade.** `scripts/runner-dispatch.sh` calls the adapter in the foreground and returns its
   verbatim exit code (`"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"; rc=$?`, ~line 217).
   Change 0237 deliberately replaced the original `exec` with call-and-return so the facade regains
   control at that seam — but the call is still synchronous and still inside the parent's window.

3. **The adapter.** `scripts/runners/opencode.sh` § *foreground execution + relay* runs
   `opencode run --dir "$DOCKET_REPO_ROOT" …` in the foreground and relays stdout. The codex and
   cursor adapters have the same shape.

So the ceiling is structural: nothing on the path can outlive the parent's Bash call, and the
contract at the top explicitly forbids the one mechanism that would let it.

### Observed failure

`docket-implement-next 258`, 2026-08-08, immediately after change 0269's shim-pin fix made opencode
delegation actually reach the runner for the first time:

- `docket-build` routed plan Task 1 to `docket-build-standard`; the wrapper made its single facade
  call exactly as written —
  `docket.sh runner-dispatch --runner opencode --agent build-standard --model openrouter/deepseek/deepseek-v4-flash-0731 --effort high --worktree /Users/homer/dev/docket/.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and`
- The opencode child ran **real work** in the named feature worktree on its branch: a green baseline
  plus four completed mutation proofs, each showing the plan's expected redden/restore behavior,
  including the count-stable `BOARD_SURFACES`→`BOARD_SURFACE` rename.
- `runner-dispatch` then exited **143** — killed at the 600000 ms ceiling — before the task's own
  verification and before its Step 7 commit.
- Per its contract the worker returned `BLOCKED`, `docket-build` halted, and the run stopped with
  +64 lines of already-mutation-proved work stranded **uncommitted** in the worktree. Branch still
  at the plan commit `7bd45872`.

The task was not pathological. One run of `tests/test_docket_config.sh` (2868 lines) costs ~60 s and
the task as planned serializes a baseline plus six whole-file runs; it completed five in ten
minutes. Any delegated task on this repo whose verification touches the config suite more than a
handful of times will hit the same wall. Task 2 of the same plan has five more proofs.

### Docket already solved this shape one layer up

Change **0223** (`done`, PR #166) established a **gate execution posture** for `docket-build`'s
full-suite gate, after the identical symptom: the 79-file suite ran right at the maximum foreground
timeout, one run completed while an identical run returned `Exit code 143 / Command timed out after
10m 0s`. Its six required capabilities live in `skills/docket-build/references/gate-execution.md`:

1. Start a run that continues beyond the lifetime or timeout of the initiating foreground call —
   including the harness's teardown of that call's **process group**, not merely its parent's exit.
2. Preserve output in a durable location with **every stream redirected away** from the initiating
   call.
3. Record an **unambiguous terminal result** distinguishable from partial output.
4. Perform subsequent **short-lived observations** of that result.
5. Distinguish **four** states: still running / completed successfully / completed unsuccessfully /
   result unavailable.
6. Enforce the observation budget without a single long-lived foreground call.

One mitigation satisfied all six on all four shipped harnesses: **detach into a new session and
redirect every stream to a durable location**, with the measured precondition that the new session
must be fully established before the initiating call returns or the harness's teardown wins the
race. `docket-build/SKILL.md` §§ *Gate execution posture* + *Halting conditions* carry the normative
clauses, `gate_observation_budget` (flat top-level key, default 30 minutes, resolved
repo-local > repo-committed > global in `scripts/docket-config.sh:685-691`, exported as
`GATE_OBSERVATION_BUDGET`) bounds the observation, and the posture fails closed.

Change **0249** propagated it to the worker contract: `skills/docket-build-task/SKILL.md:59-66` now
tells a worker that when its narrowest honest verification may outlast a foreground call, it must run
under those capabilities, never yield (a dispatched worker has no resumption channel), observe by
blocking, keep it finite, and return `BLOCKED` rather than infer success.

### Why that does not cover this

Both 0223 and 0249 bind a **test command** executed *by* an agent. Neither binds the **delegation
boundary** — the call that launches the agent itself. The gap is not cosmetic:

- **The posture is unreachable from inside.** 0249 tells the delegated worker to run its verification
  under the six capabilities. But the *whole worker*, verification included, is already inside the
  shim's 600000 ms window. A worker that perfectly obeys 0249 still dies when the window closes.
  That is exactly what happened on 258: the child was doing the right thing when it was killed.
- **The mitigation cannot be re-derived per run.** For a test command an implementer can invent the
  workaround in the moment (0203's implementer did, three times). The shim wrapper is a *generated
  artifact* that makes exactly one call and forbids deviation, so there is no per-run judgment to
  exercise. The fix has to be in the generator, the facade, or both.
- **The harness verdicts do not transfer.** `gate-execution.md`'s per-harness verdicts were measured
  for a *gate* launch (a test command), and each is explicitly version- and scope-scoped —
  claude `supported — interactive session, two foreground calls; forked mode unmeasured`, codex
  `supported — race-free new-session launch shape only`, opencode `supported — standard launch shape
  only; un-detached behavior unmeasured`, cursor `supported`. A delegated *adapter* launch is a
  different shape and must be re-probed, not inherited.

### The crash detector already exists and should be reused

The facade's **run gate** (change 0237) already refuses to trust the child's exit code, and instead
establishes disposition from **durable git state**:

- `resync_metadata` re-syncs from **fresh origin** on both sides of the hand-off — deliberately
  symmetric, because an asymmetric pair misattributes an abandoned claim from an earlier session to
  this run (`LEARNINGS: cas-re-read-fresh-origin`).
- It snapshots the in-progress claim set before and after (`verify-run.sh --in-progress-ids
  --with-claimed-at`), stamping `DISPATCH_EPOCH` *after* the before-read so a claim landing in the
  gap is excluded either way, and uses `claimed_at` to tell its own claim from a foreign one.
- `scripts/verify-run.sh` — a pure reader, git and filesystem only — returns one of exactly four
  verdicts: `run-complete <id>` / `run-halted <id>` / `run-incomplete <id> <unmet…>` (tokens:
  `status pr branch`) / `run-unclaimed <id>`. Exit 0 whenever a verdict was produced;
  `run-incomplete` is a **finding, not a script failure**
  (`LEARNINGS: exit-code-encodes-a-non-failure`).
- The facade acts on the verdict: one re-dispatch for an unfinished run, exit 1 on the second
  strike, exit 3 on a halted run, exit 0 when a re-dispatch drove the run to completion over a stale
  first code.

That is capability 5's four-state distinction, already implemented against git state rather than
against a process. It is fenced to one agent by a single line —
`GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1` (`runner-dispatch.sh:163`) — so a `build-*`
delegation gets no disposition at all and the caller is left reading exit 143.

## What changes

### 1. State the posture for the delegation boundary

Add a delegation execution posture that **cites** `skills/docket-build/references/gate-execution.md`
as the single source rather than restating the six capabilities (change 0154 exists to clean up
restatement; 0223's own design put the quarantine file in this role and `docket-finalize-change`
already cites rather than restates). The posture must say that a delegated run may outlive the call
that started it, and it must live where the *generator* can honor it — the wrapper text in
`emit_shim`, plus `scripts/runner-dispatch.md`.

### 2. Change what `emit_shim` bakes

The "ONE foreground Bash call … never background it, never poll" instruction is the defect made
literal. Replace it with a launch-and-observe shape. Constraints on the edit:

- ADR-0038's chokepoint property must survive: still exactly one dispatch seam, still no inline
  fallback, still no silent retry.
- The wrapper is byte-compared in `tests/test_sync_agents.sh`; the build-worker `--worktree` slot and
  its rule block (change 0206) are emitted conditionally and must stay byte-stable for non-build
  shims.
- Change 0269 is in flight in the same function (`emit_shim`'s model/effort parameters). Sequence or
  rebase deliberately — likely depends_on 0269.

### 3. Detach the adapter, durably

Apply the one mitigation that satisfied all six capabilities: detach into a new session with every
stream redirected to a durable, non-colliding per-dispatch location, established before the
initiating call returns. Decide the owner — the facade (one implementation for all runners) or each
adapter (per-runner launch shapes already differ; codex needs `exec --skip-git-repo-check
--sandbox danger-full-access`, opencode needs `run --dir` and `permissions: auto-approve`, cursor
needs `--print --force --sandbox disabled`). The facade is the better default; per-adapter overrides
only where a harness demands it.

Re-probe all four harnesses for the **adapter** launch shape and record verdicts with the same
version-scoped discipline `gate-execution.md` uses, including the `unverified` vs `incompatible`
distinction and the "a probe that changes two variables at once proves nothing" rule.

### 4. Extend the run gate past `implement-next`

Widen `runner-dispatch.sh:163` so a `build-*` delegation also gets a durable disposition. This needs
a **`build-*` postcondition**, because a build task's terminal state is a commit on the feature
branch, not a PR — so `verify-run.sh` grows a second verdict family rather than having the
implement-next conjuncts (`status` / `pr` / `branch`) stretched to fit. Candidate conjuncts: the
worktree is on the expected branch, the branch tip advanced past the dispatch-time SHA, the tree is
clean. The dispatch-time SHA is the natural analogue of `DISPATCH_EPOCH` and should be captured the
same way — after the before-read.

Keep `verify-run.sh` a pure reader; only `runner-dispatch.sh` acts on a verdict.

### 5. Budget

Add a sibling key `delegation_observation_budget` (default **60**) with the same layering shape and
fail-closed integer check as `gate_observation_budget` (`scripts/docket-config.sh:670-691`,
`scripts/docket-config.md:134`, `.docket.example.yml:216-227`), exported as
`DELEGATION_OBSERVATION_BUDGET`. 0223's semantics carry over: a budget of `0` is legal and buys
exactly one observation, not a disabled gate. On budget exhaustion the facade kills the detached
session before reporting failure — no unwatched agent ever keeps working after the run was declared
failed (honors 0231).

### 6. Guards

Mutation-tested coverage for the statically representable parts: the wrapper no longer contains the
foreground/never-background instruction; a build shim's baked flags are unchanged; the run gate fires
for a `build-*` agent; each new `verify-run` verdict is produced by a fixture. Derive any per-harness
population from `HD_SHIPPED_HARNESSES` rather than hand-listing, per the established idiom in
`tests/test_docket_review.sh`, `tests/test_docket_example_yml.sh`, `tests/test_cursor_contract_docs.sh`,
including its non-vacuity floor.

## Out of scope

- **Reducing suite runtime**, and **any edit to change 0258's plan.** Narrowing 0258's mutation
  proofs to exercise only the `0258 L1` / `L2` sections instead of re-running the whole 2868-line
  file is worth doing on its own merits and would unblock 0258 today — but it treats the symptom.
  0258 must **not** be blocked on this change. (Suite runtime generally is change 0227.)
- **Change 0269** — the shim's own model/effort pin. Disjoint defect on the same code path, now
  `done` and merged. This change depended on it landing; it does not subsume it.
- **`runners.opencode.permissions` locality** — `.docket.local.yml` is gitignored, so a fresh feature
  worktree has no copy and a worker anchored there resolves `auto-approve` back to the default `ask`
  and is refused. Named as out of scope by 0269 as well; now owned by change **0270**.
- **ADR-0024's never-yield rule for dispatched subagents.** 0223 settled the boundary: an external
  process observed by its owner is not a subagent yielding. This change must not be written as
  touching that rule, and clause 4 of the posture (only a top-level session agent may yield; a
  dispatched child observes by blocking) applies here unchanged.
- Any change to what the dispatch flags mean or how their values resolve.

## Reconcile log

### 2026-08-09 — reconciled at claim

Re-read against `origin/docket`, `origin/main` (tip `05fbb224`), the cited source, and the related
changes. **The design stands; scope unchanged.** Every code citation in the change body and spec was
re-verified against current source and all of them still hold:

- `runner-dispatch.sh:163` — `GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1`, byte-exact.
- `runner-dispatch.sh:217` — `"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"`, the synchronous
  adapter call, still call-and-return per 0237.
- `docket-config.sh:670-691` — the `gate_observation_budget` block, the layering shape the new
  `delegation_observation_budget` key mirrors.
- `.docket.example.yml:216-227` and `docket-config.md:134` — the documentation slots for it.
- `skills/docket-build/references/gate-execution.md` — present; still the single source of the six
  capabilities this change cites rather than restates.
- `emit_shim` — **at `sync-agents.sh:1552`, in the repo ROOT, not `scripts/`** (the change body
  names the function without a path, so nothing was wrong; recorded here so the build does not go
  looking under `scripts/`). Its heredoc still bakes the defect verbatim: "Make exactly ONE
  foreground Bash call, with the maximum timeout (600000)" / "never background it, never poll".

Two facts drifted since grooming, both recorded and both narrowing the work:

1. **`depends_on: [269]` is now satisfied.** 0269 was `in-progress` at grooming and is now `done`
   and merged onto `main`. The spec's §9 sequencing step ("rebase deliberately onto its landed
   shape") is therefore already discharged by cutting the feature branch from `origin/main` — no
   rebase-onto-0269 work remains. 0269's landed edit touched `emit_shim`'s frontmatter pin
   (`resolve_shim_pins` / the `$2`/`$3` shim-model/effort parameters); this change touches the
   heredoc BODY below it, so the two edits are adjacent but non-overlapping.
2. **The `runners.opencode.permissions` locality gap is no longer unowned.** Both the change body
   and the spec called it "still unowned"; change **0270** now owns it. It stays out of scope here;
   only the ownership claim was stale.

Auto-capture: enabled, 0 stubs minted. The one adjacent gap this pass surfaced — the
`.docket.local.yml`-in-a-fresh-worktree problem — is already filed as 0270, so it fails admission
gate 1 (in scope of an existing change) rather than clearing all six.

## Design decisions (settled at grooming, 2026-08-08)

All open questions resolved in the spec:

- **Liveness vs correctness:** done-file sentinel written as the detached child's last act answers
  "is it done"; `verify-run.sh` git verdicts answer "did it succeed"; a sentinel claiming success
  with no matching git evidence is a failure. No time floor in `verify-run.sh`; it stays a pure
  reader.
- **Orphans:** kill on giving up — budget exhaustion terminates the detached session before
  reporting failure.
- **Resumability:** none — observe, kill, or complete; never re-enter.
- **Concurrency key:** agent name + mint (timestamp + PID), in a docket-owned durable dir.
- **`build-*` re-dispatch:** observe-only, no auto-retry; `implement-next` keeps its existing
  one-re-dispatch policy.
- **Exit codes:** synthesized from disposition (0 complete / 3 halted / 1 failed or unavailable,
  distinct diagnostics), stated normatively in `scripts/runner-dispatch.md`.
- **Shim shape:** launch-then-observe — one `--launch` facade call that detaches and returns, then
  bounded short `--observe` calls; still one dispatch seam (ADR-0038), never yield (ADR-0024).
