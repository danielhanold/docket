<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0271 — Runner delegation has no execution posture for a child that outlives its foreground call](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0271-runner-delegation-has-no-execution-posture-for-a-child-that.md)**
<!-- docket:backlink:end -->

# Runner delegation — detached execution posture (change 0271) — Design

**Date:** 2026-08-08
**Change:** 0271 — Runner delegation has no execution posture for a child that outlives its foreground call

## Problem

Runner delegation bounds an entire delegated agent run inside one foreground call of the parent
harness, hard-capped at 600000 ms (the Bash tool's maximum — not tunable). Three layers hard-code
the posture: the generated shim wrapper ("ONE foreground Bash call … never background it, never
poll"), the facade (`runner-dispatch.sh` calls the adapter synchronously and returns its code), and
each adapter (foreground `opencode run` / `codex exec` / `cursor` + stdout relay). Any delegated
task that outlasts the window is killed mid-work (observed: change 0258, exit 143, ~10 minutes of
mutation-proved work stranded uncommitted), and the kill is indistinguishable from failure.

Docket already solved this shape for the build gate (change 0223,
`skills/docket-build/references/gate-execution.md`: six capabilities; one mitigation — detach into a
new session with every stream durably redirected — satisfied all six on all four shipped harnesses).
Change 0249 propagated it to the worker contract. Neither binds the delegation boundary itself: a
worker that perfectly obeys 0249 still dies when the shim's window closes.

## Decisions

### 1. Delegation execution posture (prose)

Add a **delegation execution posture** to `scripts/runner-dispatch.md` and the shim wrapper text
emitted by `emit_shim`. It **cites** `skills/docket-build/references/gate-execution.md` as the
single source of the six capabilities — no restatement (the 0154 discipline; 0223's own design put
the quarantine file in this role). Normative content: a delegated run may outlive the call that
launched it; the shim makes its dispatch call and the facade owns detachment, observation, and
disposition. ADR-0038's chokepoint property survives untouched: exactly one dispatch seam, no inline
fallback, no silent retry.

### 2. Detachment is owned by the facade

`runner-dispatch.sh` launches the adapter **detached into a new session**, every stream redirected
to a durable per-dispatch location, with the new session fully established before the initiating
call returns (0223's measured race precondition). Adapters keep their per-runner launch shapes
(codex `exec --skip-git-repo-check --sandbox danger-full-access`, opencode `run --dir` +
`permissions: auto-approve`, cursor `--print --force --sandbox disabled`); a per-adapter detachment
override exists only if re-probing shows a harness demands one.

**Re-probe all four shipped harnesses for the adapter launch shape** — the gate-execution verdicts
were measured for a *gate* launch and are explicitly version- and scope-scoped, so they do not
transfer. Record new verdicts with the same discipline: version-scoped, `unverified` vs
`incompatible` distinguished, one variable per probe.

### 3. Done-file sentinel — liveness from the artifact, correctness from git

The detached child's **last act** is writing a terminal sentinel file into the per-dispatch result
directory, carrying the adapter's exit code. This is capability 3 (unambiguous terminal result) and
resolves the `verify-run.sh` precondition problem: the facade may now observe while the child is
alive, so liveness must never be inferred from git state.

- **Sentinel answers "is it done."** Absent sentinel + budget remaining → still running; keep
  observing.
- **Git answers "did it succeed."** `verify-run.sh` verdicts, unchanged in role.
- **Disagreement rule:** a sentinel claiming success with no matching git-state evidence is a
  **failure** (correctness wins). A missing sentinel at budget exhaustion is `result unavailable`.
- `verify-run.sh` grows **no time floor** and stays a pure reader (git and filesystem only); only
  `runner-dispatch.sh` acts on verdicts.

### 4. Watcher loop, budget, and orphan policy

The facade observes with short-lived checks bounded by a new config knob:

- **`delegation_observation_budget`** — sibling of `gate_observation_budget`, **default 60**
  (minutes). Same layering shape (repo-local > repo-committed > global), same fail-closed integer
  check (`scripts/docket-config.sh` pattern at ~670–691), exported as
  `DELEGATION_OBSERVATION_BUDGET`; documented in `scripts/docket-config.md` and
  `.docket.example.yml`. 0223 semantics carry over: a budget of `0` is legal and buys exactly one
  observation, never a disabled gate.
- **Kill on giving up.** When the budget expires with no sentinel, the facade **terminates the
  detached session** before reporting failure. No unwatched agent ever keeps working on the repo
  after the run was declared failed — this is the resolution 0231 demands (no presumed-dead worker
  waking to race its replacement). Partial work stays on the branch/worktree for human inspection.
- **Resumability: none.** A delegated child is observed, killed, or completed — never re-entered.
  The facade only ever re-observes within one dispatch's budget.

### 5. Run gate widened to `build-*`

Open the fence at `runner-dispatch.sh:163` (`[ "$AGENT" = "implement-next" ]`) so `build-*`
dispatches also get a durable disposition. `verify-run.sh` grows a **second verdict family** for
build tasks (never stretching the implement-next conjuncts). Build postcondition conjuncts:

1. the worktree is on the expected branch;
2. the branch tip advanced past the **dispatch-time SHA** — captured after the before-read, the
   direct analogue of `DISPATCH_EPOCH`;
3. the working tree is clean.

**No auto re-dispatch for `build-*`** — observe-only. A build task may have left partial commits;
re-running on top of them is the "never escalate onto a stray commit" hazard `docket-build`'s
halting conditions name. `implement-next` keeps its existing one-re-dispatch policy and 1/3/0 exit
codes unchanged.

### 6. Synthesized exit codes

Under detachment there is no adapter exit code at dispatch time, and the shim's rule is bare
non-zero → abort-and-report. The facade **synthesizes** the caller-visible code from the
disposition; the mapping is stated normatively in `scripts/runner-dispatch.md` alongside the
existing gate codes:

| Disposition | Exit |
|---|---|
| run/task complete (verdict green) | 0 |
| halted (implement-next `run-halted`) | 3 |
| failed — verdict red, sentinel-failure, or disagreement | 1 |
| result unavailable — budget exhausted, child killed | 1 (with a distinct diagnostic line) |

(Enumerate consumers before finalizing per `LEARNINGS: exit-code-encodes-a-non-failure`; only
non-failures earn non-1 codes.)

### 7. Per-dispatch result key

The durable result directory is keyed on **agent name + a mint (timestamp + PID)** — unique per
dispatch regardless of change id or worktree, so concurrent dispatches (including two for the same
change) never collide. Location: a docket-owned durable dir (same family as the gate's), never the
feature worktree.

### 8. `emit_shim` rewrite

Replace the "ONE foreground Bash call … never background it, never poll" instruction with a
launch-and-observe shape. A single blocking facade call cannot do the observing itself — the
facade call is a foreground Bash call and inherits the same 600000 ms ceiling. The shim's contract
therefore becomes **launch, then observe**:

1. One foreground facade call `docket.sh runner-dispatch … --launch` that detaches the child and
   returns immediately with the dispatch key.
2. Bounded short-lived observation calls `docket.sh runner-dispatch … --observe <key>` (each well
   under the ceiling) until a terminal disposition or budget exhaustion, per the posture in
   `runner-dispatch.md`. Never yield (ADR-0024 unchanged: a dispatched child observes by blocking on
   short calls, only a top-level session agent may background-and-await).

Still one dispatch seam (both verbs are the same facade), still no inline fallback, still no silent
retry.

Constraints: the build-worker `--worktree` slot and its rule block (change 0206) stay byte-stable
for non-build shims; `tests/test_sync_agents.sh` byte-comparisons updated in the same change.

### 9. Sequencing

`depends_on: [269]` — 0269 is `in-progress` in the same function (`emit_shim`); rebase deliberately
onto its landed shape. 0258 is explicitly **not** blocked on this change.

## Out of scope

- Reducing suite runtime (change 0227) and any edit to change 0258's plan.
- Change 0269 (shim's own model/effort pin) — dependency, not subsumed.
- `runners.opencode.permissions` locality in fresh worktrees (still unowned).
- Any change to ADR-0024's never-yield rule, dispatch flag semantics, or value resolution.

## Guards (mutation-tested where statically representable)

- The emitted wrapper no longer contains the foreground/never-background instruction.
- A build shim's baked flags are unchanged; non-build shim bytes stable.
- The run gate fires for a `build-*` agent.
- Each new `verify-run` verdict (build family) is produced by a fixture.
- The sentinel disagreement rule (sentinel-success + red git state → failure) has a fixture.
- Per-harness populations derive from `HD_SHIPPED_HARNESSES` with the non-vacuity floor.
