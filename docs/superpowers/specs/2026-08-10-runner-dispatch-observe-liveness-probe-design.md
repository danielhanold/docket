<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0284 — runner-dispatch --observe is sentinel-only: adopt 0282's identity-checked liveness probe](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0284-runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi.md)**
<!-- docket:backlink:end -->

# `runner-dispatch --observe`: an identity-checked liveness probe — design

Change: 0284. Groomed interactively with the human, 2026-08-10 (`docket-groom-next`, dummy-mode
dialogue; the spec itself is written at full technical density per the convention's agent-safety
rule).

## Problem

`runner-dispatch.sh --observe` has **no process-liveness probe**. Its running-state predicate is
the absence of the `done` sentinel, so a delegated child that died without writing one — killed
externally, crashed, its host suspended — reads `still running` (exit 4) on every observation until
`DELEGATION_OBSERVATION_BUDGET` (default **60 minutes**) expires and the give-up path fires. The
contract states the posture explicitly: *"the sentinel is the **only** source of liveness — the
facade never [probes the process]"* (`scripts/runner-dispatch.md`, *Liveness vs correctness*).

This is the marker-keyed-versus-liveness-keyed defect change 0282 exists to remove, in the
**dispatch** lifecycle rather than the **gate** lifecycle, and at ten times the worst-case latency
(0282's `GATE_OBSERVATION_BUDGET` defaults to 30 minutes; this one to 60). 0282 shipped the correct
predicate for `gate-run.sh` and **deliberately excluded** `runner-dispatch.sh` from its site
rewiring — its surface is agent-dispatch-specific and change 0277 was expected to be reworking it —
recording the exclusion as a named residual with this stub minted against it.

The gap is narrower than "no identity check exists here": `runner-dispatch.sh` **already owns** the
identity conjuncts, inside `terminate_dispatch`, where they gate the group kill on the give-up path.
What is missing is consulting them as a **verdict input** on the still-running leg, one lifecycle
phase earlier.

## Design

### 1. `scripts/lib/docket-liveness.sh` — one predicate, two consumers

A new library in the established `scripts/lib/` family (seven libs today; `runner-dispatch.sh`
already sources `docket-root.sh` and `docket-dispatch-dir.sh`). Libraries in that directory carry
no co-located `.md` contract — the convention's *Script contracts* rule binds `scripts/<name>.sh`,
and no existing `scripts/lib/*.sh` has one — so the interface is documented in the file header and
pinned by its test file.

**Interface — values, never directory layouts.** The two consumers store their records in
incompatible formats (`gate-run.sh`: `$rd/launch` + a separate `$rd/identity` file, keyed
`pid`/`pgid`/`identity`; `runner-dispatch.sh`: `$DDIR/launch`, keyed `pgid`/`child_pid`/
`child_lstart`). Each consumer keeps its own record reader and passes the extracted values in. A
lib that took a run-dir path would have to know both layouts, which is the coupling 0282's
assumption 1 rejected when it declined to extend `runner-dispatch.sh` with a generic verb.

```
docket_identity_of <pid>
    -> the normalized `ps -o lstart=` token for <pid>; empty when the pid is gone.
       An EXACT STRING, never parsed into a date — the rendering is platform- and
       locale-dependent, and only equality is ever asked of it (gate-run.sh's rule,
       lifted verbatim).

docket_group_alive_and_ours <pgid> <expected-identity>   [sets DOCKET_LIVENESS_WHY]
    -> 0 when the group exists AND the process leading it started at <expected-identity>.
       Non-zero otherwise. Sets DOCKET_LIVENESS_WHY to a caller-printable reason on
       every non-zero return; empty on 0.
```

The conjuncts, in order: `<pgid>` is non-empty and numeric and not `0`/`1`; `kill -0 -<pgid>`
succeeds; `docket_identity_of <pgid>` is non-empty and string-equals `<expected-identity>`.

The `0`/`1` conjunct is a **no-op for `gate-run.sh`**, whose `recorded_pgid` already refuses
anything not `> 1` before the call — it is carried in the lib for `runner-dispatch.sh`, whose
`LPGID` is a raw `launch_field` read with no such filter. Stating it here so the refactor is not
mistaken for a behaviour change on the gate-run side.

**Fail-closed on every leg** (0282's rule, unchanged): an absent pgid, a non-numeric one, an empty
recorded token, an empty live token all return non-zero. The asymmetry is the justification — a
false `dead` costs one wasted observation, a false `alive` costs the caller's entire budget.

**`DOCKET_LIVENESS_WHY` is what makes this one predicate rather than two.** `terminate_dispatch`
needs the *reason* its identity check failed for its "NOT signalling process group …" diagnostic;
`--observe`'s new leg needs only the boolean. A lib returning only a boolean would have forced
`terminate_dispatch` to keep a private reason-producing copy — a second predicate wearing the first
one's answer, which is exactly the drift the `duplicated-gate-copies-the-whole-predicate` learning
describes.

**Both existing sites are refactored onto it**, not just the new one:

- `gate-run.sh` — `identity_of` and `group_alive_and_ours` are deleted and the lib sourced.
  `recorded_pgid`/`recorded_identity` stay (they read gate-run's layout) and become the call's
  arguments. `identity_matches` collapses into the lib call.
- `runner-dispatch.sh` — `ps_lstart` is deleted in favour of `docket_identity_of`, and
  `terminate_dispatch`'s inline conjunct ladder is replaced by one lib call plus
  `DOCKET_LIVENESS_WHY`.

### 2. The new `--observe` leg

`--observe`'s read order gains one step, inserted **after** the terminal-state reads and **before**
the clock/budget arithmetic:

1. `done` present → `report_done_disposition` *(unchanged)*
2. `killed` present → repeat the recorded give-up verdict *(unchanged)*
3. **NEW — liveness.** `docket_group_alive_and_ours "$LPGID" "$(launch_field "$DDIR" child_lstart)"`.
   Alive ⇒ fall through to the existing clock/budget/unenforceable machinery unchanged.
4. **NEW — dead ⇒ re-read the sentinel**, then dispose (§3).
5. Budget arithmetic, `still running` (exit 4), `terminate_dispatch` *(unchanged)*

**Why the probe precedes the clock reads.** Deadness is knowable without a readable clock. Placed
after, an unreadable clock or an unparseable `started_at` — the `note_unenforceable` family — would
keep a dead child spinning for three more observations before terminating on the *wrong* cause. The
`unenforceable` counter is untouched by this leg: it is reset or incremented only on the
still-running path, which step 3 now guards.

**Step 4's re-read is load-bearing, not defensive.** Steps 1–3 span a `ps` call and a `kill -0`;
the child has every chance to finish inside that window, and without the re-read a run that
**passed** is disposed as dead. The soundness argument is the one already written at
`runner-dispatch.sh`'s existing pair of give-up re-reads: the untrapped wrapper subshell is the
**only** writer of `done`, so a sentinel visible at step 4 was necessarily written by a child that
completed. On finding one, hand straight to `report_done_disposition` — which never returns.

### 3. Disposing of a dead child — git decides

A dead child is **not** automatically "no result". The disagreement rule already governing the
sentinel path — *liveness from the process, correctness from git* — is extended to this leg, because
a delegated run can commit its work, push its branch and open its PR and *then* be killed before the
wrapper's `mv -f` lands. Reporting `unavailable` over evidence sitting in git sends a human hunting
for work that is already committed (change 0258's failure, inverted).

The dispositions are the ones `report_done_disposition` already routes to, reached with **no exit
code to consult**:

| `AGENT` | Read | Verdict |
|---|---|---|
| `implement-next` | `observe_implement_next` (`verify-run`) | `run-complete` ⇒ exit **0**; `run-halted` / `run-incomplete` ⇒ exit **1**, wording preserved |
| `build-*` | `verify-run --build --worktree <anchor> --branch <launch record's branch> --since <launch record's since_sha>` | `task-committed` ⇒ exit **0**; anything else ⇒ exit **1** |
| any other | — | exit **1**, result unavailable |

**Re-wording is required, not optional.** Both dispositions today narrate a child that *"exited 0
but git disagrees"*. On this leg the child said nothing at all, so every message on the dead path
states the death first and the git verdict second — e.g. *"the child died without writing a
sentinel; git says `task-committed`, so the work landed"*. Asserting an exit code that was never
read is the fabricated-verdict failure `classify_record` refuses over a malformed record.

`relay_child_stdout` fires on this leg exactly as it does on the sentinel path: the child is
finished either way, so whatever it managed to write is the only evidence left.

### 4. Recording the verdict — reuse the give-up marker

`--observe`'s idempotence guarantee (*"a completed, failed or killed dispatch re-reports identically
forever"*) obliges a terminal record. This leg reuses `terminate_dispatch`'s existing marker rather
than minting a second terminal-state file: `cause=child-vanished`, `reason=group-already-gone`.

The `cause`/`reason` split already carries exactly these two axes — `cause` says *why the facade
gave up*, `reason` says *whether anything was signalled* — so the new cause needs no new field, and
the existing step-2 reader gains one `case` arm for its wording. Nothing is signalled and nothing
is killed: step 3 established the group is not provably ours, which is the precondition
`terminate_dispatch` already refuses to signal under.

**Consequence to state plainly in the contract:** a supervisor that died while processes it spawned
keep running is reported dead and those orphans are **not** reaped. This is the accepted residual
`runner-dispatch.md` already documents for the give-up path, now reaching a **verdict** rather than
only a kill decision. Killing them would mean signalling a group that cannot be proven still ours;
an unrelated process group dying is the worse failure and the unrecoverable one, while an orphan is
visible and reapable. The dead-path diagnostic therefore names the dispatch dir so a human can find
them.

### 5. Contract prose

`scripts/runner-dispatch.md`'s *Liveness vs correctness* section is rewritten. The repealed sentence
is *"The sentinel is the only source of liveness — the facade never infers 'still running' from git
state, and never infers 'finished' from anything but the wrapper's own sentinel."*

Its replacement states the three-source order — **terminal record first, liveness second, git
last** — retains the half that is still true (correctness never comes from liveness, and liveness
never comes from git), documents `cause=child-vanished`, and documents the orphan residual from §4.
The give-up path's existing residual paragraph is amended to say the same residual now also shapes a
verdict.

## Files touched

- `scripts/lib/docket-liveness.sh` — **new**.
- `scripts/runner-dispatch.sh` — source the lib; delete `ps_lstart`; new observe leg (§2); dead-path
  dispositions (§3); `cause=child-vanished` arm in the `killed` reader; `terminate_dispatch`
  refactored onto the lib.
- `scripts/runner-dispatch.md` — §5.
- `scripts/gate-run.sh` — delete `identity_of` / `group_alive_and_ours` / `identity_matches`; source
  the lib.
- `tests/test_docket_liveness.sh` — **new**.
- `tests/test_runner_dispatch_observe.sh` — new cases (§ Testing).
- `tests/runtime-budgets.tsv` — re-budget the two touched files if the suite reports `OVER BUDGET:`.

## Testing

Every new guard is **mutation-tested** — strip the thing it guards, watch it redden — per AGENTS.md.

**`tests/test_docket_liveness.sh`** (new): a live self-spawned group with a matching token ⇒ alive;
the same group after it exits ⇒ dead; a live group with a **mismatched** token ⇒ dead (the pid-reuse
case) and `DOCKET_LIVENESS_WHY` names the mismatch; empty / non-numeric / `0` / `1` pgid ⇒ dead with
a reason and **no `kill` issued** (assert the group was never probed); empty expected token ⇒ dead.
Mutation: drop the identity conjunct and the mismatched-token case must redden.

**`tests/test_runner_dispatch_observe.sh`** (extended):

1. **The headline.** Launch a child, kill its group without letting the wrapper write `done`,
   observe once ⇒ **not** exit 4, and the elapsed wall-clock of that single observation is seconds,
   not the budget. Mutation: remove the step-3 probe and this must redden into exit 4.
2. **Sentinel outranks liveness.** A dispatch whose child is dead **and** whose `done` exists ⇒ the
   sentinel disposition, exit unchanged. Pins step 1's precedence against step 3.
3. **The step-4 re-read.** A fixture that materializes `done` between the step-1 read and the step-3
   probe ⇒ the completed disposition, never `child-vanished`. This needs the same env-gated,
   inert-by-default barrier shape `gate-run.sh` uses to hold its TOCTOU window open
   (`barrier post-first-record`); without one the window cannot be entered deterministically.
   Mutation: delete the re-read and this must redden.
4. **Git decides.** Dead child + `implement-next` + a `verify-run` fixture returning `run-complete`
   ⇒ exit 0. Dead child + `run-halted` ⇒ exit 1 with the halted wording. Dead child + `build-*` +
   `task-committed` ⇒ exit 0. Dead child + no git evidence ⇒ exit 1 unavailable. Mutation: route the
   dead path straight to unavailable and the first three must redden.
5. **No fabricated exit code.** Assert the dead-path output does **not** contain the sentinel path's
   `exited 0` / `exited <n>` phrasing — a shape assert, not an enumerated wording list.
6. **Idempotence.** Observe a vanished dispatch twice ⇒ identical stdout, stderr and exit code, and
   the second observation performs no `verify-run` call (the marker short-circuits at step 2).
7. **Nothing is signalled.** On the dead path assert no `kill` reaches the recorded group — the
   orphan residual is a promise, so it needs a discriminating fixture (a surviving child under a
   dead leader must still be running after the observation returns).

`tests/test_gate_run.sh` and `tests/test_gate_run_stop.sh` must pass **unchanged** — the refactor in
§1 is behaviour-preserving for gate-run, and an edit to either file is the tell that it was not.

## Out of scope

- The `DELEGATION_OBSERVATION_BUDGET` value itself (change 0273 owns budget policy).
- Reaping orphans / any relaxation of the refuse-to-signal-unprovable-ownership rule. Reversing it
  is an ADR-level decision, not a code change.
- The dispatch directory layout, the harness adapters, the brief-file format (0277), the `--launch`
  verb's detachment mechanism (ADR-0080, measured and not in question).
- The sentinel's role as the source of **correctness** — that still comes from git via
  `verify-run.sh`. This change is about liveness only.
- `gate-run.sh`'s own semantics: it is refactored, never re-specified.

## Assumptions

1. **The lib takes values, not run dirs.** The two record layouts are incompatible and both are
   load-bearing; a layout-aware lib would couple the dispatch and gate lifecycles, which 0282
   assumption 1 rejected. Rejected alternative: a `--probe` verb on `gate-run.sh` that
   `runner-dispatch.sh` shells out to — it would additionally have to reconcile the two on-disk
   formats.
2. **Empty recorded identity fails closed, unifying a live divergence.** `terminate_dispatch` today
   *skips* the identity conjunct when `child_lstart` is empty, degrading to the pgid check alone;
   `gate-run.sh` fails closed. The lib adopts fail-closed and `terminate_dispatch` inherits it.
   This is behaviour-preserving on every **reachable** input: `--launch` records an empty
   `child_lstart` only when `ps` saw no process, i.e. the child had already finished, in which case
   the wrapper writes `done` and step 1 disposes before either leg is reached. The one transient —
   the child finished but the `mv -f` has not landed — is caught by step 4's re-read. The
   `test_docket_liveness.sh` empty-token case pins the new posture; the reachability argument is
   recorded here rather than asserted in code.
3. **`related`, not `depends_on`, against 0277 / 0208 / 0270.** 0277's spec declares *"any change to
   launch/observe semantics, the run gate, or detachment"* **out of scope**, and 0208 (input gates)
   and 0270 (config resolution) sit on the input-validation side; none touch the observe leg. 0277's
   own assumption 8 set the precedent — file collisions on `runner-dispatch.sh` are recorded as
   `related:` and reconciled at rebase by intent (`concurrent-edits-compose-at-rebase`). The stub's
   "let 0277 finish first" line was written before 0277's scope was read and is superseded here.
4. **`gate-run.sh` is refactored in this change, not left with its copy.** Leaving the copy in place
   would ship the two-predicate state the change exists to prevent, and the human chose the shared
   helper explicitly over per-file copies. The risk — editing a file that merged hours ago — is
   bounded by the unchanged-test rule in § Testing.
5. **The dead path reuses the `killed` marker rather than minting a terminal file.** The
   `cause`/`reason` fields already carry the two axes; a second terminal file would need its own
   precedence rule against `done` and `killed` and would be a third state for every reader to order.
6. **No new exit code.** `0` / `1` / `4` already span the outcomes: the dead path is terminal, and
   terminal outcomes are `0` or `1`. Minting one would hit `exit-code-encodes-a-non-failure` at
   every bare non-zero consumer, and the generated shim's loop keys on `4`.
