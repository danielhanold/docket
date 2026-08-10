<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0282 — Launch-and-wait contract for long-running child processes — liveness-keyed, not marker-keyed](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0282-launch-and-wait-contract-for-long-running-child-processes-li.md)**
<!-- docket:backlink:end -->

# Launch-and-wait contract for long-running child processes — design

Change: 0282. Auto-groomed 2026-08-09 (default-biased self-brainstorm; critic-gated, two rounds,
0 needs-human verdicts). Round 2's four remaining wrong-but-fixable items were closed by applying
the critic's own prescribed repairs verbatim (recorded per assumption below) — no post-gate design
was introduced by the designer. The human audit trail is `## Assumptions` below.

**Amended 2026-08-09, human spec review — two rounds, post-groom, not critic-gated.** Round 1 added
the `--stop` verb (assumption 13), reversing assumption 5's parked residual and re-homing assumption
4's group-kill onto the verb. **Round 2 (human review of that draft) found four soundness defects in
it and corrects them here:** the leader-dead relaunch claim was false (assumption 22), `failed` was
still marker-keyed rather than termination-keyed (16), and both `--observe` and `--stop` carried
completion-versus-liveness TOCTOU races (19, 21). Assumptions 14–22 are all round 2. Everything
outside assumptions 13–22 passed the auto-groom critic; assumptions 13–22 did not — they carry human
review only, and several of them exist because round 1's own draft was wrong.

Round 2's corrections are **transplants, not inventions**: `runner-dispatch.sh` already solved the
record-outranks-signal ordering, the two-sided re-read, and the refuse-to-signal-unprovable-ownership
posture, and this spec now cites those invariants rather than re-deriving them.

**Amended 2026-08-09, adversarial critic re-review — round 3, five findings, closed by the critic's
own prescribed repairs.** (1) The `died`-relaunch flow's `already-terminal` re-observe was not
total: a record-classified signal death — the 0276 headline shape — matched no leg and cycled
`died → --stop → already-terminal → re-observe → died` forever; the enumeration now keys on the
re-observe's own state (assumption 22). (2) The `state=` stdout grammar claimed all three verbs but
fit only `--observe`; stdout payloads are now defined per verb (assumption 18). (3) Two "five-state"
references pre-dating the sixth state are corrected. (4) A deliberate `--stop` whose own `TERM` the
wrapper recorded as `kind=signal` read as accidental death forever after; `--stop`'s post-kill
record-found path now writes the stop marker as an annotation (new assumption 23). (5) Assumption
15's session-primitive escalation was near-certain on macOS, not contingent, and is re-resolved into
a probe ladder with an honestly-narrowed per-platform fallback. Also: `cause` gained real values
(`signal`/`vanished`, retiring the vestigial `unexpected`), the mutation-test log-tail assert is
pinned to stderr, and the test-only barriers must be env-gated and inert by default.

**Amended 2026-08-09, adversarial critic re-review — round 4, on the round-3 repairs.** Round 3's
finding-1 and finding-4 repairs each shipped a defect of their own; all five round-4 findings are
closed here. (A) The seven-step `--stop` wrote the `stopped` marker unconditionally, so a `--stop`
on an already-vanished group minted a marker for a signal never sent and made the vanished-death
relaunch leg unreachable — the marker is now gated on the group having actually been signalled, and
the absent-group branch reports `already-terminal` and writes nothing (assumption 21). (A2)
Assumption 23's annotation had a crash window its own scope rule forbade the idempotent retry from
healing; a stop-intent record written immediately before the signal closes it (assumption 23,
rewritten). (B) The record-classified `died` relaunch accepted undetected live orphans — the state
the `unavailable` leg calls the 0231 double-run state; `--stop`'s record-present path now probes the
recorded group and reports `unavailable` when anything is still alive under it (new assumption 24).
(C) `--launch` failure had no stdout report while assumption 11 forbids keying on the exit code;
failure now prints a single `launch-failed` token (new assumption 25). (D) The `script(1)` rung's
probe criterion widens from "delivers a session" to the full capability set — stream separation, no
isatty flip on the child's redirected stdio, no injected framing, no new SIGHUP path (assumption
15).

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

Three verbs: `--launch` and `--observe` mirror runner-dispatch's lifecycle split; `--stop` owns
termination.

**Stream discipline, all three verbs (assumption 18).** **stdout carries exactly one
machine-readable report line and nothing else**, and the payload is verb-specific: `--launch` prints
the minted run-directory path (the handle) on success and a single `launch-failed` token on launch
failure (assumption 25 — the two are shape-distinguishable, a path is absolute and the token carries
no slash); `--observe` prints `state=<state> [cause=<cause>]`;
`--stop` prints one of its three report tokens (`stopped` / `already-terminal` / `unavailable`). The
`state=` grammar belongs to `--observe` alone — a caller parsing it off another verb matches
nothing, by design. Every diagnostic — the log tail on `died`, the reason a signal was withheld, the
malformed-record detail — goes to **stderr**. This is the runner-dispatch rule (*"Every diagnostic this verb prints goes to
stderr"*) and it is what makes "callers key on the stdout report line" (assumption 11) actually
parseable: a log tail is multiline and arbitrary, so it can never share the channel with the
protocol.

- **`--launch [--root <dir>] -- <command…>`** — start the command detached in a **new session**.
  - **The detachment handshake is part of the contract, not the implementation (assumption 14).**
    `--launch` MUST NOT return a run handle until the supervisor has acknowledged that its new
    session exists, its durable streams are open, `pid`/`pgid`/identity are captured, and it is
    ready to supervise. Failure to establish that within a fixed launch-establishment bound is a
    **launch failure** that returns no usable handle and reports the single `launch-failed` token on
    stdout (assumption 25). gate-execution measured this precondition
    rather than reasoning about it: on codex, a launch that returned before session detachment
    completed produced a child that never started, and that harness's `supported` verdict is
    explicitly scoped *"race-free new-session launch shape only"*.
  - **Mechanism constraint, named rather than assumed (assumption 15).** `runner-dispatch.sh`'s
    `set -m` own-process-group technique is **not** sufficient here — it makes the child a group
    leader, which is a weaker separation than the new session capability 1 requires. But
    `runner-dispatch.md` also records the platform floor: *"`setsid` is absent on macOS and docket
    takes no `perl` dependency."* That floor makes the no-primitive outcome the **expected** one on
    macOS, not a contingency — a POSIX shell cannot create a session without an external primitive —
    so the spec resolves the question instead of deferring it (assumption 15): the plan probes, in
    order, `setsid` where present, then a platform-shipped pty-based session primitive (macOS's
    `script(1)` runs its command as the leader of a new session on a fresh pty — a candidate to
    **verify, not assume**). **A rung passes only when the full capability set holds under it, not
    the session bit alone** (critic re-review round 4): a pty is not a free new session — the probe
    must also show every stream still landing unmerged in its durable run-dir location with no
    primitive-injected framing (BSD `script` writes typescript headers), the child observing
    non-interactive stdio (an isatty flip re-awakens the prompting/recoloring class the promoted
    non-interactive-flags rule exists for), and no new hangup path — pty teardown must not deliver
    `SIGHUP` to a still-running child, the very signal family this change defends against. Where
    the ladder is exhausted, the contract for that platform is
    **honestly narrowed** in `gate-run.md` to own-process-group plus the unchanged detachment
    handshake, recorded as a per-platform capability note and escalated as a design finding for a
    human to accept or supersede with a dependency — the build records and continues rather than
    stalling, and the contract never claims "new session" anywhere it does not deliver one.
  - Records `pid`/`pgid`, the command line, and an opaque identity token (assumption 9). A wrapper
    writes the terminal record when — and only when — the child itself exits, as its **own separate
    file** (the runner-dispatch `done`-file shape), atomically: temp file beside its destination,
    `mv -f`.
  - **Run dir (assumption 17):** `--root <dir>` names the parent; the helper always mints the unique
    run directory beneath it, so callers cannot collide. Default root is the templated
    `"${TMPDIR:-/tmp}/gate-run.XXXXXX"` shape (assumption 12). Created under `umask 077` — it holds
    arbitrary command output and argv. An existing run dir is **refused, never reused** (the
    runner-dispatch rule: silent reuse would overwrite a live run's sentinel). Prints the run
    directory on stdout; that path is the handle.

- **The terminal record encodes termination *kind*, not a bare integer (assumption 16).** Flat
  `KEY=value`: `kind=exit code=<n>` or `kind=signal signal=<n>`. This is what makes the
  `failed`/`died` distinction real rather than an artifact of whether the supervisor happened to
  outlive the child: a suite killed by `TERM` while its wrapper survives *did not go red*, it never
  finished, and `failed` is the state that may feed repair work.
  - **The POSIX-shell floor, and which way it is biased.** A shell wrapper sees only `$?`, which
    conflates a genuine `exit 143` with death by signal 15; `kind=signal` cannot be derived with
    certainty from a bare shell. The rule is therefore: a code in `129..192` is recorded
    `kind=signal`, and the misreading it admits — a child that genuinely exits in that band — is a
    **named residual**, accepted because the two errors are not symmetric. Reading a signal death as
    `failed` mints integration-repair work for tests that never ran, which is exactly what
    assumption 3 exists to forbid; reading a genuine `exit 143` as `died` costs one relaunch that
    reproduces the same code and then halts. If the plan finds a portable way to retain the true
    wait-status distinction, it supersedes this heuristic and the residual is closed.

- **`--observe <run-dir>`** — one short-lived observation, six states. **The read order is
  load-bearing and the terminal record always wins** (assumption 19 — the runner-dispatch rule,
  transplanted):

  1. **Read the terminal record.** Present ⇒ classify and stop: `kind=exit code=0` → `passed`;
     `kind=exit` nonzero → `failed`; `kind=signal` → `stopped` if a completed stop marker **or a
     stop-intent record** exists (assumption 23 — the recorded signal may be a deliberate `--stop`'s
     own `TERM`, including one whose `--stop` crashed before annotating), else
     `died cause=signal`. Present but unparseable → `unavailable` (a malformed record means the *supervisor* did not finish cleanly,
     and a verdict read out of garbage is fabricated).
  2. **Probe liveness** — `kill -0` on the recorded pgid, cross-checked against the identity token
     (assumption 9), so a recycled pgid never reads alive. Alive and identity-confirmed → `running`.
  3. **Dead or identity-mismatched ⇒ RE-READ the terminal record.** Appeared → classify by step 1.
     Still absent → `stopped` if a completed stop marker exists (assumption 13), else
     `died cause=vanished`, reported with the log tail on stderr.

  **Step 3's second read is the whole point** and is not covered by atomic writes — atomicity
  prevents partial reads, not staleness. Without it the sequence *observer reads "no record" → child
  finishes and writes `EXIT=0` → observer probes, group gone → reports `died`* turns a run that
  **passed** into a `died` that triggers a relaunch. The correctness argument is the ordering's, and
  it is runner-dispatch's: the untrapped wrapper is the only writer of the record, so a record
  visible after the liveness probe was necessarily written by a child that completed.

  | state | meaning | retryable? |
  |---|---|---|
  | `running` | alive, identity-confirmed | **the only non-terminal state** — observe again within the caller's budget |
  | `passed` | `kind=exit code=0` | terminal |
  | `failed` | `kind=exit`, nonzero — the child *ran and went red* | terminal |
  | `died` (`cause=signal` \| `cause=vanished`) | `kind=signal` with no stop marker (`signal`), or no record and the group gone or identity-mismatched (`vanished`) | terminal for this attempt |
  | `stopped` | terminated deliberately by `--stop` (assumption 13) | terminal — **never relaunched** |
  | `unavailable` | run dir, launch record, or terminal record unreadable or malformed | terminal, fail-closed |

  **Only `running` is retryable (assumption 20).** The other five are terminal, `unavailable`
  included: an unreadable or malformed run dir does not heal after launch has returned, and polling
  it to a budget bound would recreate the very latency defect this change exists to remove — in a
  new place. This is runner-dispatch's own argument that a state returning the loop condition
  unconditionally is a state the loop can never leave.

- **`--stop <run-dir> [--reason <text>]`** — terminate the recorded run. **The terminal record
  outranks the stop at every step** (assumption 21), in this order:

  1. Terminal record present ⇒ **probe the recorded group before reporting** (assumption 24):
     anything still alive under the recorded pgid ⇒ report `unavailable` (`orphans-detected` on
     stderr) and signal nothing — the leader is dead, so ownership of the survivors is unprovable,
     but detection is possible even where safe signalling is not, and relaunching over live orphans
     is the 0231 double-run state. Group confirmed empty ⇒ report `already-terminal`.
  2. Validate identity (the assumption 9 conjuncts).
  3. **Re-read the terminal record immediately before signaling** — nothing but the stop-intent
     write and the `kill` separate the test from the act.
  4. **Probe the recorded group.** Confirmed absent ⇒ re-read the terminal record once more; still
     none ⇒ report `already-terminal` and write **nothing** — the vanished case must remain
     relaunchable (assumption 21). Otherwise write the **stop-intent** record (atomic; distinct
     from the completed marker — assumption 23), then `TERM` the recorded process group; bounded
     grace; `KILL` if still present.
  5. **Verify the group is gone.**
  6. **Re-read the terminal record again**, after the kill and before any marker is written.
  7. Only now — **the group having actually been signalled** and verified gone — write the
     completed `stopped` marker (timestamp, caller-supplied reason).

  Steps 3 and 6 are the runner-dispatch give-up path verbatim, and for its stated reason: the path
  is entered off a "no record" read taken several syscalls earlier, and without them `--stop` kills a
  run that had **already completed successfully** and then reports it as terminated. A record found
  at step 1 or step 3 takes the terminal disposition, nothing is signalled, and no completed marker
  is written by *this* call — this call's `TERM` never happened. Whether such a record's
  `kind=signal` was deliberate is decided by durable evidence, not by this call's history:
  `--observe` reads `kind=signal` as `stopped` when a completed stop marker **or a stop-intent
  record** exists (assumption 23), so a signal sent by a prior `--stop` that crashed after its
  intent write still classifies as deliberate. A record found at step 6 also takes the terminal
  disposition and reports `already-terminal`, but when it is `kind=signal` the completed marker
  **is** written as an annotation (assumption 23): the signal it records is almost certainly the
  step-4 `TERM`, and without reclassification a deliberately cancelled run reads `died` forever and
  an idempotent site relaunches a cancellation. The record still outranks intent and marker for
  exit kind; they only reclassify the signal death as deliberate.
  **The completed marker is never written before termination is verified** (step 5 before step 7) —
  otherwise a `--stop` that dies halfway leaves a marker claiming a kill that never happened, and a
  later idempotent call no-ops on it while the child runs on. The stop-intent record claims nothing
  about termination — it records only that a signal was about to be sent — so writing it pre-kill
  does not weaken this rule.

  **Report line — three states, and they gate the relaunch (assumption 22):**

  | report | condition |
  |---|---|
  | `stopped` | identity confirmed, group signalled, **and verified gone** |
  | `already-terminal` | a terminal record exists **and the recorded group is confirmed empty** (assumption 24), **or** the recorded pgid is confirmed absent |
  | `unavailable` | the group exists but ownership **cannot be safely established** — or the run dir/launch record is unreadable |

  `unavailable` is the honest state for the leader-dead-orphans-alive case, and it exists because
  the identity guard makes that case **unfixable, not fixable** — see assumption 22. Its sub-reason
  (`ownership-unprovable`, `orphans-detected`, `rundir-unreadable`) goes to stderr, so the two verbs' `unavailable`
  tokens stay distinguishable in diagnosis without splitting the protocol.

  **Idempotent**: a second call, or a call on an already-terminal run, reports `already-terminal`
  and is not an error — which is what lets a halt path and a `died` relaunch both call it without
  coordinating, and lets the abandon rule below be stated unconditionally. Children that escaped the
  recorded group (double-fork, own session) survive it; that narrowed case remains a named residual.

The helper is deliberately generic (any command), stateless beyond its run dir, and performs no
docket metadata operations. It does not poll internally — the *caller* owns the loop and its
budget, so the existing observation-posture prose (short-lived observations, blocking for
dispatched children, `GATE_OBSERVATION_BUDGET`) is unchanged in structure and gains a mechanized
predicate.

### Call-site posture on `died`

**Scoped to idempotent children** — the suite gate and read-only/verify-gated work. The relaunch is
**gated on `--stop`'s report, not merely preceded by the call** (assumption 22):

```
died — either cause  (idempotent child only)
  -> --stop
       stopped           -> group verified gone  -> one fresh --launch
       already-terminal  -> RE-OBSERVE, and key on the state it returns:
                              passed / failed -> verdict stands; NO relaunch
                              died            -> one fresh --launch
                                                 (record-classified signal death, or group
                                                  confirmed absent with still no record)
                              stopped         -> NO relaunch; deliberate cancellation
                              unavailable     -> abort-and-report; NEVER relaunch
       unavailable       -> abort-and-report; NEVER relaunch
```

The `unavailable` leg is the correction that makes this sound. `died` includes the
leader-dead-orphans-alive state, and in **exactly** that state the identity guard cannot prove the
surviving pgid is still this run's — so `--stop` refuses to signal it, and the orphans are still
running. Relaunching there would race a second suite against the first in the same worktree, which
is the state change 0231 exists to prevent. The earlier claim that `--stop` made that race
impossible was **false**: the guard's whole purpose is to decline that kill, and runner-dispatch
already accepts the identical residual in its give-up path (*"a group whose leader died while
processes it spawned keep running is not signalled"*). Refusing to relaunch is the only posture the
guard leaves available.

The `already-terminal` re-observe is likewise load-bearing, and follows from the record-outranks-stop
invariant: `already-terminal` covers both *a record appeared* and *the group is confirmed absent*.
Relaunching on `passed`/`failed` would re-run a suite that had **finished** and throw away its
verdict. The re-observe's enumeration is **total over the state vocabulary, and every leg is
terminal** (critic re-review round 3): a record-classified `died cause=signal` — the 0276 headline
shape, a `TERM`ed suite whose wrapper survived to write the record — reaches the one-relaunch
decision *at the re-observe*, not through another `--stop`, so the cycle
`died → --stop → already-terminal → re-observe → died → …` cannot occur. `running` is unreachable
here: `already-terminal` requires a terminal record or a confirmed-absent group, and the identity
token cannot re-match a recycled pgid. Live orphans under the recorded pgid are not silently
accepted on this leg (critic re-review round 4, assumption 24): the `--stop` this leg passed through
probes the recorded group even on its record-present path, and anything still alive under the
recorded pgid reports `unavailable` — the abort leg — before a relaunch can race it. The posture is
fail-closed: a recycled pgid reads as live orphans and aborts; only a group probed empty reaches the
relaunch.

A second `died` after a relaunch is abort-and-report (halt per the caller's existing halting
conditions). Mirrors the AGENTS.md run gate's re-dispatch-once rule; grounded in 0276, where the
single relaunch succeeded. A **non-idempotent** child (a publish, a rebase — anything whose first
attempt may have taken side effects before dying) is never auto-relaunched: `died` there follows the
site's existing failure posture (for finalize's steps, `gate-failure.md`'s abort-and-report), with
the death diagnostic attached. `failed` keeps its existing semantics everywhere (a red suite is a
red suite — never relaunched by this rule), and `stopped` is never relaunched by anyone: a
deliberate cancellation is not an accident to retry.

**Call-site posture on abandoning a live child.** A caller that stops observing while `--observe`
still reports `running` — a `GATE_OBSERVATION_BUDGET` exhaustion, or any halt or abort taken with
the child alive — calls `--stop` before it reports:

```
budget exhausted / halt / abort while running
  -> --stop
       stopped | already-terminal -> halt and report
       unavailable                -> halt and report LOUDLY: a child may still be
                                     running that could not be proven ours to kill
```

This is the rule assumption 5 originally parked: without it a halted run leaves the suite executing
against the worktree the human is about to inspect. `--stop`'s idempotence is what makes the rule
safe to state unconditionally — a caller never has to first establish whether the child is still
alive to decide whether stopping is legal. Every leg halts (unlike the `died` path, nothing here is
retried), so `unavailable` changes only the loudness and the diagnostic, not the outcome — but it
must change them, because it is the one leg where the human inherits a live process. The budget
itself is unchanged and no knob is added: this governs the *cleanup* on abandonment, not when
abandonment happens.

### Site rewiring

- `docket-build` § *Gate execution posture* / `references/gate-execution.md`: the mitigation names
  the helper as the shipped implementation of the detach-and-sentinel discipline; the posture
  gains the liveness-keyed-wait sentence plus the scoped one-relaunch rule. The **six-state
  vocabulary lives in `gate-run.md`'s contract** and the posture cites it — the distinction is
  mechanized harness-independently by the shipped script, so gate-execution's version-scoped
  harness-capability list is the wrong owner for it; capability 5 gains only a pointer and no
  harness verdict is rewritten or re-probed (assumption 10). `docket-finalize-change` inherits by citation (its SKILL.md already defers to
  build's posture — no restatement added).
- The full executable-site scope is derived at plan time by whole-repo grep (per the AGENTS.md
  never-hand-list rule) over launch/poll shapes — `nohup`, `setsid`, `kill -0`, `until … grep`,
  `tail -f`, background-run idioms — sorted prose vs executable. **The grep runs from the repository
  root with only known non-source trees excluded** (`.git`, worktrees, generated/cache dirs); it does
  **not** enumerate `skills/`, `scripts/`, `agents/`, `cursor-rules/`. An enumerated directory set is
  itself the hand-list the rule forbids — a listed-directories sweep is exactly how root-level
  executable scripts become the blind spot, and the spelling you miss is the target file's own house
  idiom. One **conscious** exclusion: `runner-dispatch.sh`
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
assert the next `--observe` reports `died` promptly on stdout with the log tail on **stderr**
(assumption 18 — the tail never shares the protocol channel); plus the
`passed`/`failed`/`running`/`unavailable` paths, the sentinel-write-on-exit guarantee, and the
**identity guard** — a fixture that substitutes a live foreign process group under the recorded
pgid must still read `died`, so the mutation "drop the identity cross-check" reddens. A wait
that cannot be shown to notice a dead child is decoration — this is the repo's own guard rule
applied to the guard this change ships. **Launch failure** (assumption 25): a `--launch` driven past
its establishment bound prints the single `launch-failed` token on stdout with diagnostics on
stderr, so a caller keying on the report line detects it — an empty-stdout implementation reddens.

`--stop` carries its own asserts, each keyed to a mutation that must redden: stopping a live child
leaves the recorded group verified gone; a child that ignores `TERM` is still gone after the grace,
so dropping the `KILL` escalation reddens; a second `--stop`, and a `--stop` on an already-`passed`
run, both report `already-terminal` and exit non-error, so losing idempotence reddens; `--stop`
writes no terminal record, so a subsequent `--observe` reports `stopped` and not `passed`, which
reddens a spurious record write; and the **identity guard on the signal path** — the recycled-pgid
fixture must have `--stop` signal **nothing at all** and report `unavailable`, so removing the
pre-signal identity check reddens rather than killing a bystander. Two round-4 asserts join them:
the **vanished-group gate** (assumption 21) — a `--stop` on a run with no record and a
confirmed-absent group reports `already-terminal` and writes **no** marker, and a subsequent
`--observe` still reports `died cause=vanished`, so the mutation "write the marker unconditionally
at step 7" reddens rather than silently making the vanished-death relaunch leg unreachable; and the
**orphan probe on the record-present path** (assumption 24) — a fixture that plants a live process
in the recorded pgid under a present `kind=signal` record must have `--stop` report `unavailable`
and signal nothing, so dropping the step-1 probe reddens rather than relaunching over live orphans.

**The five race asserts are the ones that cannot be argued into existence — each needs a
deterministic interleaving fixture, not a sleep.** The helper takes test-only synchronization points
(a wait-for-file barrier the fixture releases) so the observer or stopper can be held at a chosen
step while the child is driven to completion. Each barrier is **env-gated and inert by default** —
unset means no-op at full speed — so the fixture hook can never itself become a hang site in
production:

- **Observe TOCTOU (assumption 19).** Hold the observer between its first record read and its
  liveness probe; let the wrapper write `kind=exit code=0` and exit; release. The observation must
  report **`passed`**, never `died`. Deleting the step-3 re-read must redden — this is the mutation
  that otherwise ships a `died` on a run that passed, and with it a spurious relaunch.
- **Stop-versus-completion (assumption 21).** Hold `--stop` after its identity check and before its
  `TERM`; let the child complete; release. It must report `already-terminal`, signal nothing, and
  write no marker. Deleting either the pre-signal re-read (step 3) or the post-kill re-read (step 6)
  must redden.
- **Marker-before-verification (assumption 21).** Kill `--stop` between its signal and its marker
  write; assert **no** `stopped` marker exists and a subsequent `--stop` still attempts termination
  rather than no-opping on a marker that claims a kill that never happened.
- **Deliberate stop recorded as signal death (assumption 23).** Hold `--stop` between its `TERM` and
  its step-6 re-read; let the wrapper reap the signal death and write `kind=signal`; release.
  `--stop` must report `already-terminal` and write the stop marker, and a subsequent `--observe`
  must report `stopped`, never `died`. Deleting the step-6 annotation write reddens — this is the
  mutation that relaunches a cancellation.
- **Intent survives a crashed annotation (assumption 23).** Kill `--stop` between its step-6
  record-found read and the annotation write; the stop-intent record already exists, so a
  subsequent `--observe` must still report `stopped`, never `died`. Deleting the pre-signal intent
  write reddens — the idempotent retry is never required to reconstruct a history only the intent
  record can prove.

- **Signal versus nonzero exit (assumption 16).** A child killed by `TERM` while its wrapper survives
  must observe as `died`, not `failed`; a child that exits 1 must observe as `failed`. The mutation
  "record the bare `$?` integer and classify by zero/nonzero" reddens — this is the assert that
  keeps a never-ran suite from minting integration-repair work.

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
   residual. Revised again at human spec review round 1: that kill is now `--stop`, assumption 13,
   rather than a hand-rolled kill restated at each call site. **Corrected at review round 2:** the
   kill does *not* make the relaunch unconditionally safe — where the group leader is dead the
   identity guard refuses to signal, so the relaunch is gated on `--stop`'s report and the
   unprovable-ownership leg aborts instead. See assumption 22.) Rejected: zero relaunches even for the suite
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
10. **The six-state vocabulary lives in `gate-run.md`; the skill posture cites it.** (Surfaced
    by critic round 1; justification corrected in round 2: no existing `supported` verdict claims
    capability 5 — the standard probe covers capabilities 1–3 only and the four-state distinction
    "went unobserved on every harness" — so the earlier "silently changes every row" claim was
    false.) The correct ground: capability 5's distinction is mechanized **harness-independently**
    by the shipped script, so a harness-capability list is the wrong owner for it — the script
    contract owns the vocabulary, gate-execution gains only a pointer, and no harness verdict is
    rewritten or re-probed.
11. **Callers key on the stdout report line, never the exit code.** (Surfaced by critic round 1.)
    The house pattern (`scripts/docket-status.md`); the exit-code mapping is documented in
    `gate-run.md` for scripting completeness only. This binds `--launch` too (round 4): its success
    line is the run-dir path and its failure line is the `launch-failed` token (assumption 25) —
    shape-keyed, still never the exit code.
12. **Shell-hygiene details are bound to the repo's promoted rules, not left to the build.**
    (Surfaced by critic round 1; reconciled with the launch design in round 2: the sentinel is
    its own separate file — the runner-dispatch `done`-file shape — not a line appended to a
    shared status file, so the atomic write and the launch records cannot conflict.) The run-dir
    default uses a templated mktemp (`"${TMPDIR:-/tmp}/gate-run.XXXXXX"` shape, per the AGENTS.md
    mktemp rule — see assumption 17 for the `--root` argument that overrides it); the terminal-record
    write is atomic — temp file beside its destination, `mv -f`. Atomicity covers partial reads only;
    the *staleness* races are assumptions 19 and 21.
13. **A third verb, `--stop`, owns termination — and `stopped` is a sixth `--observe` state.** (Human
    spec review, post-groom — not critic-gated.) Two call sites need to kill a recorded run and
    neither should hand-roll it: the `died` relaunch (assumption 4, which already required a group
    kill) and abandonment of a live child (assumption 5's parked residual). Chosen: `--stop
    <run-dir>`, identity-checked before signaling, `TERM` → bounded grace → `KILL`, idempotent,
    recording a `stopped` marker and no terminal record. Rejected: leaving termination to each call
    site (three sites hand-rolling a signal is the hand-rolled-loop defect this change exists to
    remove, and only the helper holds the identity token that makes signaling safe). Rejected:
    `--stop` synthesizing a terminal `kind=exit code=0` record (it would report a run that never
    finished as one that did — the exact conflation assumption 3 exists to prevent).
    **Reversed at review round 2:** the first draft folded a deliberate stop into
    `died` + annotation to keep the enum at five. That breaks the contract's own premise that a
    caller keys on the state: `died` may be relaunched by an idempotent site and `stopped` must
    never be, so state alone would no longer determine behavior and every caller would have to
    parse the annotation to stay correct. Cancellation is not accidental death; the sixth enum
    member buys a state machine that is actually sufficient for the rule built on it. Residual,
    narrowed from assumption 4's: a child that escaped the recorded process group survives
    `--stop`; the helper signals the group it recorded and does not hunt descendants.
14. **The detachment handshake is contractual.** `--launch` returns no handle until the new session,
    the durable streams, and the identity records are established; exceeding a fixed
    launch-establishment bound is a launch failure. Grounds: gate-execution *measured* this — codex
    produced a never-started child when the parent returned before detachment completed, and its
    verdict is scoped to the race-free shape. Rejected: "fully established before the call returns"
    as prose (that is the sentence the current spec already had, and prose is what 0276 proved does
    not reach the moment of launch).
15. **The session primitive is resolved by a probe ladder; exhaustion narrows the contract honestly
    and escalates without stalling.** (Re-resolved at critic re-review round 3 — the first draft
    left this "open, escalate if none found", but the escalation leg was near-certain, not
    contingent: a POSIX shell cannot create a session without an external primitive, and the
    recorded floor — `setsid` absent on macOS, no `perl` dependency — rules out the obvious ones,
    so the plan would have stalled on a design question this spec is the right altitude to answer.)
    Capability 1 requires a new **session**; `set -m` yields only an own process group. Resolved:
    probe `setsid` where present, then a platform-shipped pty-based session primitive (`script(1)`
    as the macOS candidate — verified at plan time, never assumed); if none delivers a genuine new
    session on some supported platform, `gate-run.md` records an honestly-narrowed per-platform
    contract (own process group plus the unchanged detachment handshake) and the narrowing is
    escalated as a design finding for a human to accept or supersede with a dependency. Rejected:
    copying `set -m` while the contract keeps claiming "new session" (a contract that overstates
    its own guarantee is how the leader-dead claim in assumption 22 got written in the first
    place); a hard stall on escalation (converts a near-certain platform fact into a permanent
    halt). (Widened at round 4: a rung passes only when the **full capability set** holds under
    it — genuine new session, every stream unmerged in its durable location with no
    primitive-injected framing, non-interactive child stdio with no isatty flip, and no new
    `SIGHUP` path on pty teardown. A probe that checks the session bit alone could green-light a
    rung that reintroduces the defect sideways.)
16. **The terminal record encodes kind, not a bare integer.** `kind=exit code=<n>` /
    `kind=signal signal=<n>`; signal termination classifies `died`, never `failed`. Grounds: with a
    bare integer, a child `TERM`ed while its wrapper survives records `EXIT=143` and reads as
    `failed` — "ran and went red" — so it may feed the repair path, which assumption 3 forbids.
    Whether that happens would depend on whether the supervisor outlived the child, i.e. the
    distinction would be an accident. **Named residual:** a POSIX shell sees only `$?` and cannot
    separate a true `exit 143` from signal 15, so `129..192` is recorded as `kind=signal`; the
    asymmetry justifies the bias (a misread signal death mints phantom repair work; a misread
    genuine 143 costs one relaunch and a halt). A portable wait-status mechanism found at plan time
    supersedes the heuristic.
17. **`--root <dir>`, helper-minted run dir, `umask 077`.** The prior text said "caller-supplied or
    mktemp'd" with no argument defining it. Chosen: `--root` names the parent only; the helper always
    mints the unique subdirectory, so callers cannot collide, and an existing run dir is refused
    rather than reused (the runner-dispatch rule). `umask 077` because the dir holds arbitrary
    command output and argv. Rejected: a caller-supplied full run-dir path (hands collision-avoidance
    to every call site).
18. **stdout is the protocol; every diagnostic goes to stderr.** A `died` log tail is multiline and
    arbitrary and cannot share a channel with a line callers parse — the two requirements as
    originally written contradicted each other. This is runner-dispatch's existing rule, adopted
    verbatim. (Corrected at critic re-review round 3: the first amendment declared the
    `state=<state>` grammar for all three verbs while `--launch`'s own bullet printed a bare path —
    a contract a verbatim implementer could not satisfy. What all three verbs share is
    one-machine-line-on-stdout; the payload is defined per verb — `--launch` the run-dir path on
    success and `launch-failed` on failure (assumption 25), `--observe` the `state=` line, `--stop`
    its report token — and the `state=` grammar is `--observe`'s alone.)
19. **`--observe` re-reads the terminal record after a dead liveness probe.** Without it the
    interleaving *no-record read → child completes → dead probe* reports `died` for a run that
    **passed**, and triggers a relaunch on it. Atomic writes do not address this: atomicity prevents
    partial reads, not stale ones. The precedence rule (record wins, always) and its correctness
    argument are runner-dispatch's. Mutation-tested with a deterministic interleaving fixture, not a
    sleep.
20. **Only `running` is retryable; the other five states are terminal.** `unavailable` in particular
    does not heal after launch has returned, and polling it to a bound would recreate this change's
    own defect in a new location.
21. **`--stop` re-reads the terminal record on both sides of the kill, and writes its marker only
    after verifying termination.** Symmetric to assumption 19: a stop entered off a stale "no record"
    read kills a run that had already succeeded. A marker written before verification lets a
    half-dead `--stop` leave a false claim of termination that a later idempotent call no-ops on
    while the child runs. Both transplanted from the runner-dispatch give-up path rather than
    re-derived. (Completed at critic re-review round 4: step 7 is additionally gated on **the group
    having actually been signalled**. The unconditional step list let a `--stop` on an
    already-vanished group — no record, nothing to kill — fall through to the marker write, minting
    a `stopped` for a signal never sent; the report and the marker then disagreed —
    `already-terminal` against a marker claiming deliberate termination — and the vanished-death
    relaunch leg became unreachable, because the re-observe read `stopped` and declined the
    relaunch. The step-4 absent-group branch now reports `already-terminal` and writes nothing,
    pinned by the vanished-group mutation assert.)
22. **`--stop` reports three states, and they gate the relaunch; `unavailable` never relaunches.**
    **This corrects an unsound claim in the first review draft**, which said calling `--stop` before
    relaunch meant a leader-dead-orphans-alive state "can never race a second run". It can. `died`
    includes precisely that state, and there the identity guard *cannot prove* the surviving pgid is
    still this run's — so `--stop` declines to signal it, exactly as runner-dispatch declines and
    accepts the orphans as a residual. The guard and the guarantee were mutually exclusive; the
    guard is right, so the guarantee goes. Hence: `stopped` (verified gone) → relaunch;
    `already-terminal` → **re-observe first**, because it also covers "a record appeared", and
    relaunching a finished run would discard its verdict; `unavailable` (ownership unprovable) →
    abort-and-report, never relaunch, because relaunching is the 0231 double-run state.
    **Completed at critic re-review round 3:** the re-observe is keyed on the state it returns, and
    the enumeration is total — `passed`/`failed` keep the verdict, `died` (either cause) takes the
    one relaunch, `stopped` and `unavailable` never relaunch. The first amendment enumerated only
    two outcomes, and the missing leg was the headline one: a record-classified signal death — the
    0276 shape itself — matched neither, so a literal executor cycled
    `died → --stop → already-terminal → re-observe → died` with no terminal leg. Both relaunch legs
    are additionally orphan-checked at round 4 — `already-terminal` now certifies the recorded
    group empty, not merely the record present; see assumption 24.
23. **Deliberate termination is reclassifiable from durable evidence: a stop-intent record written
    immediately before the signal, plus the completed-marker annotation on `--stop`'s post-kill
    record-found path.** (Critic re-review round 3; rewritten at round 4.) The window: `--stop`
    `TERM`s the group, the wrapper reaps the child's signal death and lands the `kind=signal`
    record before its own pending `TERM` takes it. Without reclassification, that deliberately
    cancelled run reads `died` on every later `--observe`, and an idempotent site relaunches a
    cancellation — violating "`stopped` is never relaunched by anyone". Two mechanisms, layered:
    (a) step 4 writes a **stop-intent** record — atomic, distinct from the completed marker,
    claiming only that a signal is about to be sent, never that termination happened, so the
    marker-before-verification invariant (assumption 21) is untouched; (b) a record found at step 6
    that is `kind=signal` additionally gets the completed marker as an annotation. `--observe` step
    1 reads `kind=signal` as `stopped` when **either** exists. The intent is what closes the crash
    window round 3 shipped while claiming closure: kill `--stop` between its step-6 read and the
    annotation write, and the round-3 design read `died` forever — the idempotent retry could not
    heal it, because a record found at steps 1/3 is never annotated by the finding call, and the
    round-3 justification that such a record "cannot be our `TERM`'s" was false in exactly that
    history. With the intent, no later call needs to reconstruct the history; the evidence is
    already on disk before the signal exists. The record still outranks both for exit kind; intent
    and marker only reclassify the signal death as deliberate. Named residual, accepted (narrowed
    from round 3's, unchanged in kind): an external signal landing after the intent is written is
    recorded as deliberate — the caller had already asked for the run's death, so the
    classification matches intent either way. Mutation-tested with two deterministic interleaving
    fixtures (the deliberate-stop and crashed-annotation race asserts). Rejected: having the
    idempotent retry annotate (the retry cannot distinguish its predecessor's `TERM` from an
    external signal — the intent record is the only honest evidence); naming the window as a
    residual without closing it (the repair is mechanical and the violated invariant — state alone
    determines relaunch behavior — is the contract's own premise, assumption 13).
24. **`--stop`'s record-present path probes the recorded group; live orphans report `unavailable`,
    and only a group probed empty reports `already-terminal`.** (Critic re-review round 4.) The
    round-3 flow accepted, without a probe, the same live-orphans world on the record-classified
    `died` relaunch leg that the `unavailable` leg names the 0231 double-run state — and its
    equation to assumption 13's residual was wrong: 13's escapees left the recorded group and are
    undetectable in principle, while these orphans sit in the recorded pgid, one `kill -0` away
    from detection. Detection is possible even where safe signalling is not: the leader is dead, so
    ownership of the survivors is unprovable and they are never signalled — only reported. Hence
    step 1: record present ⇒ probe; anything alive under the recorded pgid ⇒ `unavailable`
    (`orphans-detected` on stderr), which the died-flow maps to abort-and-report. Fail-closed by
    construction: a recycled pgid reads as live orphans and aborts rather than relaunching.
    Mutation-tested (planted-orphan fixture). Rejected: relaunching with the orphans named as a
    residual (a residual is for what cannot be detected, not for what was not probed).
25. **`--launch` failure reports a single `launch-failed` token on stdout.** (Critic re-review
    round 4.) Assumption 14 defines launch failure and assumption 11 forbids keying on the exit
    code — but the round-3 payload definition gave `--launch` only a success line, leaving failure
    with no signal a caller was permitted to read, so every call site would have hand-rolled its
    own failure detection: the exact defect class this change removes. Success and failure stay
    shape-distinguishable on one channel: an absolute path is the handle; the slash-free
    `launch-failed` token is not a handle. Diagnostics, as everywhere, on stderr. Rejected:
    empty-stdout-means-failure (absence is not a report line and is indistinguishable from a
    truncated write); scoping assumption 11 to give `--launch` an exit-code contract (two keying
    disciplines across three verbs is how callers get one of them wrong).

## Out of scope

Unchanged from the stub: suite speed (0280 owns `OVER BUDGET` shards); finalize gate semantics;
the early-yield defect (covered by the never-yield rule and the
`yielded-worker-return-closes-every-door` finding — a controller-side classification problem, not
a child-process one).
