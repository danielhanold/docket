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
   that initiated it.
2. Preserving gate output in a durable location.
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
| cursor | undocumented and reported to have changed; 900 s+ failures reported | An experimental background-terminal setting exists alongside a silence-timeout setting | `unverified` — smoke-test |
| opencode | default 2 min, SIGTERM on expiry; overridable by environment variable and per-command field | No first-class background mechanism; shell-level backgrounding is reported to hang the tool when the child inherits the caller's output pipes | `unverified` — smoke-test |
| codex | per-call, not the binding limit | Negative signal: the exec session is torn down at **turn** end and anything spawned from it dies with it; no cross-turn background mechanism | likely `incompatible` — smoke-test to confirm |

Two findings shape the design.

**The portable hazard is inherited file descriptors, not backgrounding.** OpenCode's hang and
Codex's teardown are both really *the child is still attached to the caller*. The capability that
matters is detachment plus durable redirection of every stream — the same act that produces the
durable artifact. One requirement, two payoffs.

**Codex breaks at the turn boundary, not the foreground-call boundary.** The contract permits the
agent to yield while the gate runs; on Codex, yielding ends the turn and kills the gate. So the
incompatibility is precise: Codex could satisfy "outlive a foreground call" only by never yielding,
which re-caps the gate at the foreground timeout — the exact constraint the contract exists to
escape.

If the smoke test confirms it, the implementer records `incompatible` with its evidence, mints a
follow-up stub, and leaves the contract unweakened. Docket MUST NOT weaken the common contract to
preserve nominal support for a harness that cannot meet it. Change 0203's empirical evidence
supports the Claude Code verdict only and MUST NOT be generalized.

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

## Risks

- **Codex may not comply.** Handled explicitly above: surface the incompatibility, do not weaken the
  contract. The change ships either way.
- **The neutrality constraint is the hard part.** A rule stated so abstractly that no implementer can
  act on it fails as surely as a harness-specific one. The reference file is what carries the
  actionable detail; if the skill-body prose cannot be made actionable with the reference in hand,
  that is the signal to revisit placement rather than to relax neutrality.
- **Overlap with the in-progress #0190.** That change touches the build-evidence record and
  finalize's suite-skip predicate. This one touches how the gate *runs*, not what it mints, but the
  two land in adjacent prose; expect to reconcile at rebase by intent rather than by choosing.
