<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0282 — Launch-and-wait contract for long-running child processes — liveness-keyed, not marker-keyed](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0282-launch-and-wait-contract-for-long-running-child-processes-li.md)**
<!-- docket:backlink:end -->

# Launch-and-wait contract for long-running child processes — design

Change: 0282. Auto-groomed 2026-08-09 (default-biased self-brainstorm; critic-gated, two rounds,
0 needs-human verdicts). Round 2's four remaining wrong-but-fixable items were closed by applying
the critic's own prescribed repairs verbatim (recorded per assumption below) — no post-gate design
was introduced by the designer. The human audit trail is `## Assumptions` below.

**Amended 2026-08-09, human spec review:** the `--stop` verb (assumption 13) is human-directed
post-groom design — it reverses assumption 5's parked residual and re-homes assumption 4's
group-kill onto the verb. It did not pass the auto-groom critic; everything else here did.

## Problem

Finalizing change 0276 burned ~20 minutes polling a process that had been dead 19 of them. Two
stacked defects:

1. **Launch** — the suite was tied to its Bash tool call's lifetime and took a `TERM` at 18s when
   the call was torn down.
2. **Wait** — the loop keyed on a success marker (`until grep -q "^EXIT="`) in a file only a live
   child could write. Marker-keyed and liveness-keyed waits differ exactly when the child dies —
   the one moment the guard exists for. The loop ran to its bound twice (582s + 584s) before the
   agent read the 131-byte log saying "interrupted (TERM)".

Same defect shape as the harvested finding `refusal-keyed-on-residue-not-condition.md`: keying on
a downstream artifact instead of the condition itself.

The launch half already has a normative home — `docket-build` § *Gate execution posture* plus its
`references/gate-execution.md` (detached new session, every stream to a durable location, terminal
sentinel, `GATE_OBSERVATION_BUDGET`-bounded observation, fail-closed). What that contract does
**not** yet fix: the observation predicate is left to the model, so hand-rolled marker-only loops
keep appearing; and gate-execution's capability 5 names four states (*running / passed / failed /
unavailable*) but "the child **died** before producing a result" has no vocabulary and no prompt
detection anywhere. 0276's run had no way to say it.

The in-repo precedent for the *shape* is `runner-dispatch.sh`'s launch/observe verb split with a
durable per-run directory and a terminal sentinel. Its observation predicate, however, is
sentinel-only by design ("the sentinel is the *only* source of liveness" — no process probe on
observe), so it carries the same dead-child latency gap this change attacks; see assumption 6.
This change gives the plain-child-process case — the suite gate first, any long-running shell
child in general — a predicate that is anchored on the process itself.

## Design

### One shared helper script: `scripts/gate-run.sh` (+ `scripts/gate-run.md` contract)

Two verbs, mirroring the runner-dispatch launch/observe split:

- **`--launch -- <command…>`** — start the command detached in a **new session** (the
  gate-execution mitigation, mechanized: fully established before the launch call returns, every
  stream redirected into a per-run directory under a caller-supplied or mktemp'd location). Record
  `pid`/`pgid`, the command line, and an opaque identity token (assumption 9). A wrapper writes a
  terminal `EXIT=<code>` sentinel when — and only when — the child itself exits, as its **own
  separate sentinel file** (the runner-dispatch `done`-file shape), atomically: temp file beside
  its destination, `mv -f`. Prints the run directory on stdout; that path is the handle.
- **`--observe <run-dir>`** — one short-lived observation. Verdict by exit code + one stdout
  report line, five states:

  | state | condition |
  |---|---|
  | `running` | no sentinel, process group alive **and identity-confirmed** (see assumption 9) |
  | `passed` | sentinel `EXIT=0` |
  | `failed` | sentinel `EXIT=<nonzero>` (the child *ran and went red*) |
  | `died` | **no sentinel AND process group gone or identity-mismatched** — reported with the log's tail as diagnostic |
  | `unavailable` | run dir or launch records unreadable or malformed |

  The liveness probe is anchored on the process (`kill -0` on the recorded pgid, cross-checked
  against a recorded identity token so a recycled pgid never reads alive — the
  `runner-dispatch.md` `child_lstart` shape), never solely on the marker: a dead child is detected
  on the **next observation** after death — seconds, not a wait-loop bound. `died` and `failed`
  never collapse into one line; each carries its own report vocabulary. Callers key on the
  **stdout report line, never the exit code** — the house pattern (`scripts/docket-status.md`);
  the exit-code mapping exists for scripting completeness and lives in `gate-run.md`.

- **`--stop <run-dir> [--reason <text>]`** — terminate the recorded run. Identity-checked *before*
  any signal is sent (the assumption 9 token, so a recycled pgid is never signaled — the guard
  matters more here than on `--observe`, where a mis-read is a wrong verdict but a mis-signal is a
  killed bystander), then `TERM` to the recorded process group, a bounded grace, then `KILL`.
  **Idempotent**: a second call, or a call on an already-terminal run, is a no-op and not an error,
  so a halt path and a `died` relaunch can both call it without coordinating. It records a
  `stopped` marker (timestamp, caller-supplied reason) in the run dir and writes **no** `EXIT=`
  sentinel — a stopped child never finished, so a later `--observe` still reports `died`, annotated
  from the marker so a deliberate stop is distinguishable from a teardown death in the diagnostic.
  One stdout report line — `stopped` / `already-terminal` / `unavailable` — and **no sixth
  `--observe` state** (assumption 13). Children that escaped the recorded group (double-fork, own
  session) survive it; that narrowed case remains a named residual.

The helper is deliberately generic (any command), stateless beyond its run dir, and performs no
docket metadata operations. It does not poll internally — the *caller* owns the loop and its
budget, so the existing observation-posture prose (short-lived observations, blocking for
dispatched children, `GATE_OBSERVATION_BUDGET`) is unchanged in structure and gains a mechanized
predicate.

### Call-site posture on `died`

**Scoped to idempotent children** — the suite gate and read-only/verify-gated work: one bounded
relaunch, **preceded by `--stop` on the run dir** (the runner-dispatch give-up shape, mechanized as
a verb rather than a hand-rolled kill at each site), so a leader-dead-orphans-alive state can never
race a second run in the same worktree (fresh `--launch`, the first death's diagnostic recorded in
the run output); a second `died` is abort-and-report (halt per the caller's existing halting
conditions). Any child that escaped the recorded group survives `--stop` and is a named residual,
as runner-dispatch names its own. Mirrors the
AGENTS.md run gate's re-dispatch-once rule; grounded in 0276, where the single relaunch succeeded.
A **non-idempotent** child (a publish, a rebase — anything whose first attempt may have taken side
effects before dying) is never auto-relaunched: `died` there follows the site's existing failure
posture (for finalize's steps, `gate-failure.md`'s abort-and-report), with the death diagnostic
attached. `failed` keeps its existing semantics everywhere (a red suite is a red suite — never
relaunched by this rule).

**Call-site posture on abandoning a live child.** A caller that stops observing while `--observe`
still reports `running` — a `GATE_OBSERVATION_BUDGET` exhaustion, or any halt or abort taken with
the child alive — calls `--stop` before it reports. This is the rule assumption 5 originally parked:
without it a halted run leaves the suite executing against the worktree the human is about to
inspect. `--stop`'s idempotence is what makes the rule safe to state unconditionally — a caller
never has to first establish whether the child is still alive to decide whether stopping is legal.
The budget itself is unchanged and no knob is added: this governs the *cleanup* on abandonment, not
when abandonment happens.

### Site rewiring

- `docket-build` § *Gate execution posture* / `references/gate-execution.md`: the mitigation names
  the helper as the shipped implementation of the detach-and-sentinel discipline; the posture
  gains the liveness-keyed-wait sentence plus the scoped one-relaunch rule. The **five-state
  vocabulary lives in `gate-run.md`'s contract** and the posture cites it — the distinction is
  mechanized harness-independently by the shipped script, so gate-execution's version-scoped
  harness-capability list is the wrong owner for it; capability 5 gains only a pointer and no
  harness verdict is rewritten or re-probed (assumption 10). `docket-finalize-change` inherits by citation (its SKILL.md already defers to
  build's posture — no restatement added).
- The full executable-site scope is derived at plan time by whole-repo grep (per the AGENTS.md
  never-hand-list rule) over launch/poll shapes — `nohup`, `setsid`, `kill -0`,
  `until … grep`, `tail -f`, background-run idioms — across `skills/`, `scripts/`, `agents/`,
  `cursor-rules/`, sorted prose vs executable. One **conscious** exclusion: `runner-dispatch.sh`
  stays untouched — not because it is already liveness-keyed (it is not: its `--observe` is
  sentinel-only, "no sentinel ⇒ still running", no process probe — so it carries this same
  dead-child latency gap up to its observation budget), but because its surface is
  agent-dispatch-specific and 0277 is actively reworking it. Its gap is recorded as a named
  residual **here and in the change's results file** (durable regardless of capture policy), and
  a follow-up stub proposing runner-dispatch adopt the same identity-checked liveness probe is
  minted at the implementer's **reconcile** pass (a legal mint site; the mint is best-effort and
  never the residual's only record).

### Mutation test

`tests/test_gate_run.sh`: launch a long-sleeping child, `kill -9` its process group mid-wait,
assert the next `--observe` returns `died` promptly and the report carries the log tail; plus the
`passed`/`failed`/`running`/`unavailable` paths, the sentinel-write-on-exit guarantee, and the
**identity guard** — a fixture that substitutes a live foreign process group under the recorded
pgid must still read `died`, so the mutation "drop the identity cross-check" reddens. A wait
that cannot be shown to notice a dead child is decoration — this is the repo's own guard rule
applied to the guard this change ships.

`--stop` carries its own asserts, each keyed to a mutation that must redden: stopping a live child
leaves the recorded group gone; a child that ignores `TERM` is still gone after the grace, so
dropping the `KILL` escalation reddens; a second `--stop`, and a `--stop` on an
already-`passed` run, both report `already-terminal` and exit non-error, so losing idempotence
reddens; `--stop` writes no `EXIT=` sentinel, so a subsequent `--observe` reports `died` **and**
carries the stop annotation, which reddens both a spurious sentinel write and a dropped marker; and
the **identity guard on the signal path** — the recycled-pgid fixture must have `--stop` signal
nothing at all, so removing the pre-signal identity check reddens rather than killing a bystander.
New row in `tests/runtime-budgets.tsv`.

## Assumptions

Every decision an interactive brainstorm would have raised, the committed default, and why.

1. **Shared helper script, not convention prose alone.** Chosen: a new `scripts/gate-run.sh` +
   contract. Rejected: prose-only (the defect is precisely that each agent hand-rolls the loop —
   0276 proves prose does not reach the moment of launch; ADR-0012's script-vs-model boundary
   argues for one CAS-correct implementation). Rejected: extending `runner-dispatch.sh` with a
   generic verb (its surface is agent-dispatch-specific — dispatch dirs, harness adapters, brief
   files — and 0277 is actively reworking it; a generic child-process verb would couple two
   unrelated lifecycles).
2. **The helper owns both launch and observe.** Rejected: wait-only (leaves the TERM-at-teardown
   launch defect to prose); launch-only (leaves the marker-keyed wait, the expensive half).
3. **Death is a fifth first-class state, distinct from failure.** Rejected: folding `died` into
   `failed` (0276's report had no vocabulary for "never finished", and an unfinished run must not
   mint integration-repair work — the existing fail-closed clause already draws this line for the
   budget case; death gets the same treatment).
4. **`died` ⇒ kill the recorded group, then one bounded relaunch, scoped to idempotent children
   only; non-idempotent sites keep their existing failure postures.** (Revised twice: round 1 —
   the original unscoped rule would auto-re-run a publish or rebase whose first attempt may have
   taken side effects before dying; round 2 — `died` includes leader-dead-orphans-alive and
   identity-mismatch states in which suite orphans may still be running, so an immediate relaunch
   would race them in the same worktree, so the relaunch is conditioned on first killing the
   recorded process group, the runner-dispatch give-up shape, with surviving orphans a named
   residual. Revised again at human spec review: that kill is now `--stop`, assumption 13, rather
   than a hand-rolled kill restated at each call site.) Rejected: zero relaunches even for the suite
   (0276's single retry succeeded; pure abort converts a transient teardown race into a
   human-needing halt); unbounded retries (masks a systematically dying gate).
5. **No new budget knob; a live child past `GATE_OBSERVATION_BUDGET` keeps today's fail-closed
   posture.** The stub's open question 4 resolves to: liveness detection changes the *dead*-child
   cost only; the live-slow case is already governed. Rejected: a separate abandon-live-child
   bound (a second knob for a case the existing budget covers). **Its parked residual is reversed
   at human spec review** — the helper does ship a `--stop` verb (assumption 13), and abandoning a
   live child now obliges the caller to call it, so an abandoned child no longer outlives a halted
   run. The no-new-knob decision stands: `--stop` governs cleanup on abandonment, never *when* a
   live child is abandoned, which the existing budget still decides.
6. **Site scope is derived, not enumerated; `runner-dispatch.sh` is a conscious exclusion with
   its gap named.** (Revised after critic round 1: the original premise "runner-dispatch is
   already liveness-keyed" was false — its `--observe` is sentinel-only with no process probe, so
   it shares the dead-child latency gap.) The exclusion survives on the other grounds
   (agent-dispatch-specific surface; 0277's concurrent rework), and the gap becomes a named
   residual carried in this spec and the results file, with a follow-up stub minted at the
   implementer's **reconcile** pass (round-2 correction: build workers perform no metadata
   operations and "build time" is not a mint site — implement-next's sites are reconcile and
   review; the mint is best-effort/policy-gated, so it is never the residual's sole record;
   auto-groom itself never mints). Rejected: admitting runner-dispatch as a site here (couples
   this change to 0277's churn and doubles its blast radius).
7. **`docket-convention` is not edited.** The gate posture's single home stays
   `docket-build` (cited by finalize); adding a convention section would restate it, which the
   convention's own compression rule forbids.
8. **Coupling posture** (frontmatter, not prose-only): `related: [251, 260, 273, 275, 277]` —
   0251/0273 both re-seed `tests/runtime-budgets.tsv`, which this change adds a row to (rebase
   collision, whichever lands second re-cuts); 0260 touches finalize's `gate-failure.md`, adjacent
   to where the `died` posture surfaces through finalize's citation; 0275 and 0277 are the same
   detached-launch/observation design family (run-gate template, runner-dispatch) and the specs
   must stay mutually consistent on the detach-and-attribute shape. No `depends_on`: any landing
   order works; collisions are ordinary rebases.
9. **Liveness is identity-checked, never bare `kill -0`.** (Surfaced by critic round 1.) A
   recycled pgid would make a dead run read `running` indefinitely. The launch record carries an
   opaque identity token (the `runner-dispatch.md` `child_lstart` shape — process start time as
   identity), and `--observe` reads `running` only when the pgid is alive AND the token matches;
   a mismatch is `died`. Mutation-tested (identity-guard fixture). Rejected: bare `kill -0`
   (green mutation test would hide the guard's absence; the repo already solved this once).
10. **The five-state vocabulary lives in `gate-run.md`; the skill posture cites it.** (Surfaced
    by critic round 1; justification corrected in round 2: no existing `supported` verdict claims
    capability 5 — the standard probe covers capabilities 1–3 only and the four-state distinction
    "went unobserved on every harness" — so the earlier "silently changes every row" claim was
    false.) The correct ground: capability 5's distinction is mechanized **harness-independently**
    by the shipped script, so a harness-capability list is the wrong owner for it — the script
    contract owns the vocabulary, gate-execution gains only a pointer, and no harness verdict is
    rewritten or re-probed.
11. **Callers key on the stdout report line, never the exit code.** (Surfaced by critic round 1.)
    The house pattern (`scripts/docket-status.md`); the exit-code mapping is documented in
    `gate-run.md` for scripting completeness only.
12. **Shell-hygiene details are bound to the repo's promoted rules, not left to the build.**
    (Surfaced by critic round 1; reconciled with the launch design in round 2: the sentinel is
    its own separate file — the runner-dispatch `done`-file shape — not a line appended to a
    shared status file, so the atomic write and the launch records cannot conflict.) The run-dir
    default uses a templated mktemp (`"${TMPDIR:-/tmp}/gate-run.XXXXXX"` shape, per the AGENTS.md
    mktemp rule); the sentinel write is atomic — temp file beside its destination, `mv -f`.
13. **A third verb, `--stop`, owns termination.** (Human spec review, post-groom — not
    critic-gated.) Two call sites need to kill a recorded run and neither should hand-roll it: the
    `died` relaunch (assumption 4, which already required a group kill) and abandonment of a live
    child (assumption 5's parked residual). Chosen: `--stop <run-dir>`, identity-checked before
    signaling, `TERM` → bounded grace → `KILL`, idempotent, recording a `stopped` marker and no
    `EXIT=` sentinel. Rejected: leaving termination to each call site (three sites hand-rolling a
    signal is the hand-rolled-loop defect this change exists to remove, and only the helper holds
    the identity token that makes signaling safe). Rejected: a sixth `--observe` state `stopped`
    (a stopped child *did* die without finishing — `died` is the honest state, and the marker
    supplies the *why* as diagnostic rather than as a state every caller must now branch on).
    Rejected: `--stop` synthesizing a terminal `EXIT=` sentinel (it would report a run that never
    finished as one that did, the exact conflation assumption 3 exists to prevent). Residual,
    narrowed from assumption 4's: a child that escaped the recorded process group survives
    `--stop`; the helper signals the group it recorded and does not hunt descendants.

## Out of scope

Unchanged from the stub: suite speed (0280 owns `OVER BUDGET` shards); finalize gate semantics;
the early-yield defect (covered by the never-yield rule and the
`yielded-worker-return-closes-every-door` finding — a controller-side classification problem, not
a child-process one).
