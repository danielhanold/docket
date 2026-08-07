<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0223 — The build gate contract never states an execution posture for a suite that outgrows a single foreground call](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0223-the-build-gate-contract-never-states-an-execution-posture-for.md)**
<!-- docket:backlink:end -->

# Design — Gate execution posture: the build gate must survive a foreground boundary

Change #0223.

## Problem

`skills/docket-build/SKILL.md` § *The build gate* tells the implementer to run the whole suite once
and defines what green and red mean, but says nothing about **how** the run is executed. That
silence was survivable while the suite fit inside a single foreground call. It no longer does: 79
test files run right at the maximum foreground timeout the Claude Code harness allows, so identical
runs split nondeterministically between completing and being killed. There is no larger value to
raise the timeout to.

The change 0203 implementer solved it on its own — backgrounding each gate to a log file and
blocking on a sentinel — and passed three gates that way. The workaround is correct but unwritten,
so the next implementer re-derives it or does not.

The workaround also has a consequence the contract must name. Parking on the suite makes the run
yield, and the yield surfaces to the dispatcher as a `status: completed` notification carrying the
agent's stale pre-park text. On 0203 that produced three false stops on one change, each disproved
only by reading git state.

## Non-goals

- Reducing suite runtime (change 0225).
- Keying green/red on the exit code (change 0224).
- Any change to ADR-0024 or the dispatched-subagent never-yield rule.

## The contract

A new *Gate execution posture* subsection in `skills/docket-build/SKILL.md` § *The build gate*,
stated by capability and harness-neutral throughout. No tool names, no process mechanics, no literal
timeout values.

1. The gate MUST NOT depend on a single foreground harness invocation remaining attached until the
   suite completes.
2. Gate execution MUST be able to outlive any individual foreground call used to start or observe
   it.
3. The gate MUST write its eventual outcome to a durable result artifact.
4. Gate completion MUST be established from that artifact, never from the caller-visible completion
   signal of the command that started the gate.
5. The agent MAY yield while the gate executes, and subsequently make short observations of the
   durable result.
6. Observation MUST be bounded — the agent MUST NOT wait indefinitely for a result artifact.
7. If no terminal result artifact exists when the observation budget is exhausted, the gate MUST
   fail closed rather than infer success.
8. The artifact-observation budget is a docket execution policy, distinct from any foreground-call
   timeout a particular harness imposes.

The observation interval is an implementation detail. The correctness requirement is that
observation consists of bounded, short-lived operations and that the overall observation period has
a finite budget.

### The lifecycle

```
start durable gate execution
         |
         v
    gate running
         |
         +----> yield is permitted
         |
         v
  observe result artifact
         |
   +-----+-----+
   |           |
terminal    not terminal
   |           |
   v           v
pass/fail   budget remains?
               |
          +----+----+
          |         |
         yes        no
          |         |
          v         v
        yield    fail closed
        and
      observe
       again
```

### The false-completion rule

A caller-visible completion signal is never gate completion. Reciprocally, a stale pre-yield report
is not evidence of a crashed run: an observer that sees a `completed` signal carrying pre-park text
MUST resolve the run's state from git and from the durable artifact before concluding anything. The
convention already states the reciprocal rule for dispatched subagents; this extends it to the gate.

### Relationship to harness foreground timeouts

A harness's foreground-call timeout MUST NOT define the maximum duration of the build gate. A suite
requiring more time than a harness will hold a foreground call open still satisfies this contract
provided the underlying execution survives the foreground boundary, its output and terminal state
are recorded durably, later observations can distinguish running from finished, and the total stays
within the observation budget. Any specific figure observed in a particular harness is not a docket
contract value and MUST NOT be encoded as one.

### Relationship to ADR-0024

This does not relax ADR-0024's prohibition on yielding from a dispatched subagent. Two different
boundaries are in play: a dispatched docket subagent yielding control in violation of its execution
contract, and an external gate process continuing independently while the responsible agent performs
bounded observations of its durable result. The latter is permitted here and MUST NOT be read as
permission for dispatched agents generally to yield across execution phases.

Because this distinction reads as an ADR-0024 violation until someone explains why it is not, it is
recorded in the ledger rather than left to skill prose: a new ADR stating the boundary, listed in
the change's `adrs:` so it publishes with the change.

### The durable artifact

The contract states the artifact by **property**, not by path. It MUST be stable across the yield,
outside the committed tree, and non-colliding between concurrent gates. Where it actually lives is a
per-harness adapter decision. This deliberately avoids both a committed `.gitignore` obligation and
a normative scratch-path rule.

### Fail-closed is a halt, not a red

Budget exhaustion with no terminal artifact halts per *Halting conditions*. It MUST NOT mint a
repair task: an unfinished run is not a failing suite. This mirrors the treatment the existing
configuration-gap case already gets, where a non-zero exit that is not a real test failure is
explicitly refused as RED.

## Placement

The contract lives in `docket-build` — the gate's owner. `docket-finalize-change`'s local gate cites
it by reference rather than restating it.

This mirrors the arrangement already in place for the suite *command*: finalize owns the
`configured-bash-finalize` marker block and build points at it. Same single-source discipline,
opposite direction. It needs no new convention section, and it avoids the restatement class that
change 0154 exists to clean up.

## The per-harness reference

New file `skills/docket-build/references/gate-execution.md`, reached from § *The build gate* by a
blocking pointer, following the repo's skill-extraction and stub-pointer pattern.

It holds two things. First, the six required capabilities, as a table — these are required
capabilities, not required mechanisms, and each harness may satisfy them differently:

1. Starting a gate whose execution continues beyond the lifetime or timeout of the foreground call
   that initiated it — **including the harness's teardown of that call's process group**, not merely
   the exit of its immediate parent. See the Codex evidence below for why the weaker reading is not
   sufficient.
2. Preserving gate output in a durable location — with **every stream redirected away from the
   initiating call**, since a stream left attached blocks that call on at least one supported
   harness (see the OpenCode evidence below), independently of durability.
3. Recording an unambiguous terminal result.
4. Performing subsequent short-lived observations of that result.
5. Distinguishing *still running* from *completed successfully*, *completed unsuccessfully*, and
   *result unavailable*.
6. Enforcing the observation budget without depending on a single long-lived foreground call.

Second, one section per shipped harness recording evidence, citation, and a verdict of `supported`,
`unverified`, or `incompatible`.

All product-specific names — tools, settings, environment variables, observed timeout figures — are
quarantined in this file. That quarantine is what lets the skill body stay harness-neutral while the
rule stays actionable.

## Harness research

Research performed at grooming, 2026-08-06. The implementer re-verifies before writing verdicts and
supersedes anything below that has changed.

| Harness | Foreground timeout | Survives the boundary? | Provisional verdict |
|---|---|---|---|
| claude | default 120 000 ms, max 600 000 ms (`BASH_DEFAULT_TIMEOUT_MS` / `BASH_MAX_TIMEOUT_MS`) | Documented background execution returning a shell id, with separate output-read and kill operations; corroborated empirically by change 0203 | `supported` |
| cursor | undocumented and reported to have changed; 900 s+ failures reported | **Smoke-tested 2026-08-06 (cursor-agent 2026.01.23, `--print --force --sandbox disabled`).** Gate survived the session ending under both detach variants **when streams were redirected**. With stdout inherited the gate was **killed mid-run** — and a disambiguating run proved redirection, not `nohup`, is the operative variable: `nohup` with stdout inherited was killed too. The call never blocked | `supported` — requires stream redirection |
| opencode | default 2 min, SIGTERM on expiry; overridable by environment variable and per-command field | **Smoke-tested 2026-08-06 (opencode 1.18.14).** Gate survived the session ending under **both** plain-background and new-session detach. But an **unredirected** launch blocked the call for the job's full duration (51s call for a 45s job) — the inherited-FD hang, reproduced; the same launch with streams redirected returned in 5–7s | `supported` — requires stream redirection |
| codex | per-call, not the binding limit | **Smoke-tested 2026-08-06 (codex-cli 0.146.1, `--sandbox danger-full-access`).** A gate launched into a **new session** (fork + `setsid`) survived the entire `codex exec` session ending; the same gate launched with plain `nohup … &` was killed before writing its first line, while the launch command reported success | `supported` — **only** via new-session detach |

Two findings shape the design.

**The portable hazard is attachment to the caller, not the absence of backgrounding.** All four
shipped harnesses were tested. Every one of them can host a durable gate, and three of the four
**break under a naive launch** — by three different symptoms arising from the same cause:

| Harness | Symptom of the naive launch | Operative variable |
|---|---|---|
| claude | none — documented background mechanism | — |
| codex | gate **killed** before its first byte, while the launch reports success | new-session detach |
| opencode | caller **blocked** for the job's full duration (51s call, 45s job) | stream redirection |
| cursor | gate **killed** mid-run; call returns normally | stream redirection (`nohup` alone is insufficient — proven by a disambiguating run) |

Two of these are silent in the worst way: Codex's launch **reports success** while the gate is
already dead, and OpenCode's gate produces a perfectly valid artifact while having quietly stopped
being asynchronous. Only Cursor fails in a way a casual check would notice.

One mitigation covers all four: **detach into a new session and redirect every stream to a durable
location.** That is the same act that produces the durable result artifact — one requirement, three
payoffs. This convergence, measured rather than assumed, is what justifies stating the rule as a
common capability instead of per-harness advice.

It also settles the change's original open question in the affirmative: the posture **can** be stated
normatively without a harness-specific escape hatch, because a single discipline satisfies every
supported harness.

**Detachment must survive a process-GROUP kill, not merely a parent exit.** This is the smoke test's
main finding, and it is a correction to the design's own starting assumption. Codex was expected to
be incompatible on the strength of maintainer reports that the exec session is torn down at turn
end. It is in fact **compatible** — the teardown is a process-group kill, so a gate placed in a new
session survives it, and survives even the whole session ending. The reported failures were
describing the mechanism accurately and the consequence incorrectly.

The consequence for the contract is that "run it in the background" is too weak a formulation. Plain
`nohup … &` satisfies the plain-language reading and still fails here — and fails in the worst
available way, with the launch command **reporting success** while the gate is killed before writing
its first output. A contract that licenses that shape ships a gate that lies. Required capability 1
is therefore stated as surviving the harness's teardown of the initiating call *and its process
group*, not as surviving the parent's exit.

Docket MUST NOT weaken the common contract to preserve nominal support for a harness that cannot
meet it. Should a future harness prove genuinely incompatible, the implementer records
`incompatible` with its evidence and mints a follow-up stub rather than relaxing the requirement.
Change 0203's empirical evidence supports the Claude Code verdict only and MUST NOT be generalized.

Where official documentation establishes the semantics clearly, the implementer may rely on it
without an exploratory test. Where documentation is absent, ambiguous, or contradicted by observed
behavior, a targeted smoke test comes first. A harness is never represented as supporting this
contract on assumption.

## Configuration

New top-level key, shipped end-to-end:

```yaml
gate_observation_budget: 30    # minutes; how long docket awaits a terminal gate result
```

Integer minutes, default 30. Exported by the resolver as `GATE_OBSERVATION_BUDGET` and read from
that variable by skill bodies, never re-parsed from YAML. It governs how long docket is willing to
await a terminal durable result; it does not control the timeout of any individual harness
operation.

Shipped end-to-end means the resolver, the export, `.docket.example.yml`, the README, and the layer
classification table land in this change — a knob is not done when it merely works.

**Classification: global-able, not coordination-fenced.** It is local execution timing, not shared
non-re-derivable state, so per-machine tuning is legitimate.

Two rejected namings. Nesting under `finalize:` is wrong because the key binds the build gate too.
A new top-level `gate:` block collides with `finalize.gate`, which already means the gate *mode* —
a permanent reading hazard for a key that would be read under time pressure.

Harness adapters may choose their own observation interval, appropriate to the capabilities and cost
model of the harness. The interval stays an implementation detail unless a concrete portability
requirement emerges.

## Guards

One new test file. Each assertion mutation-tested — strip the clause, watch it redden — or it is
decoration.

- The durable-result contract is present in § *The build gate*.
- The finite observation-budget requirement is present.
- The fail-closed-on-exhaustion clause is present.
- The default budget agrees across the skill body, `.docket.example.yml`, the README, and the
  resolver's default.
- `docket-finalize-change`'s local gate cites the contract.
- **A verdict is recorded in the reference for every harness in `HD_SHIPPED_HARNESSES`.**

The last one is the structurally valuable assertion: it reddens automatically when a fifth harness
is added, so harness support can never silently go undeclared. Per the repo's house rule, guards key
on shape rather than an enumerated list of spellings; the reflow-tolerance question that change 0171
is settling applies to the prose-anchored assertions here.

The runtime persistence behavior of an external harness is not fully guard-testable from inside
docket. Where behavior depends on external harness semantics, the guards verify docket's adapter and
configuration contract while the reference records the external capability evidence.

## Scope note

This flips the change from `type: docs` to `type: feat` — the configuration knob is real code.

The probe script used for the Codex verdict is worth re-deriving rather than preserving: it launches
a gate that writes its sentinel **last**, ends the harness session immediately, and then observes
from **outside** the agent, so the agent's own report is never the evidence. An inconclusive run
(gate never started) establishes nothing and MUST NOT be recorded as `incompatible` — the first two
Codex runs were inconclusive for unrelated reasons (a `setsid` EPERM caused by the harness running
the launcher as a process-group leader) and would have produced a false verdict if believed.

The probe must also measure the **duration of the launch call**, not only whether the artifact
appears. OpenCode's failure mode leaves the artifact perfectly intact and is invisible to a
survival-only check — the gate "works," it has simply stopped being asynchronous. A blocked launch
is a contract failure even though every artifact assertion passes.

A probe that changes two variables at once proves nothing about either. The first Cursor
hang-probe dropped both `nohup` and the redirection and the gate died; only a second run holding
`nohup` while inheriting stdout established that redirection was the operative variable. The
implementer re-verifies each verdict before writing it, and disambiguates the same way.

## Risks

- **The verdicts are version-scoped.** Each row names the version tested. Per the repo's own
  learning that harness behavior is mode- and version-scoped, the implementer re-probes rather than
  inheriting these rows on faith — particularly Cursor, whose timeout behavior is undocumented and
  reported to have changed.
- **The neutrality constraint is the hard part.** A rule stated so abstractly that no implementer can
  act on it fails as surely as a harness-specific one. The reference file is what carries the
  actionable detail; if the skill-body prose cannot be made actionable with the reference in hand,
  that is the signal to revisit placement rather than to relax neutrality.
- **Overlap with the in-progress #0190.** That change touches the build-evidence record and
  finalize's suite-skip predicate. This one touches how the gate *runs*, not what it mints, but the
  two land in adjacent prose; expect to reconcile at rebase by intent rather than by choosing.
