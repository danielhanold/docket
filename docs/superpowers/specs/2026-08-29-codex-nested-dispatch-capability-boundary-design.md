<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0365 — Make nested Docket dispatch reliable for every Codex agent invocation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0365-codex-nested-dispatch-capability-boundary.md)**
<!-- docket:backlink:end -->

# Codex nested dispatch capability boundary

**Change:** 0365 · **Date:** 2026-08-29 · **Type:** fix · **Priority:** critical

## Problem

Docket's Go installer correctly emits and registers every Codex agent wrapper, including the
internal `docket-plan-writer`. The failure occurs after a registered parent agent starts. In the
observed change-0361 run, `docket-implement-next` queried `ALL_TOOLS` from inside Codex's JavaScript
orchestration tool, found no agent-dispatch entry there, and declared native dispatch unavailable.
It then applied the Tier-C authorized-or-halt rule and stopped before producing a plan, code, tests,
or a PR.

That capability conclusion was invalid. Codex exposes collaboration controls on the active
agent's top-level tool surface; the nested JavaScript `tools.*` inventory intentionally omits those
controls. A nested inventory is therefore not an authoritative view of the active agent's
capabilities. The run never attempted the registered `docket-plan-writer` dispatch and received no
dispatch rejection or policy denial.

Both supported user paths reach this shape:

- prose requests are routed by the repository's managed `AGENTS.md` block to the registered
  same-name Docket agent; and
- direct `@docket-…` invocation starts that registered agent explicitly.

Change 0353 previously classified raw named-agent invocation as operator error and was killed after
the skill/fork path was shown to work. That conclusion is no longer sufficient: Docket's current
managed routing contract deliberately selects the named agent for prose, and direct `@agent`
invocation is a supported user surface. A generated Docket agent must be able to perform every
nested dispatch its charter requires on either path.

The defect is general, not plan-writer-specific. Implement-next composes status, planning, build,
review, and ADR agents; auto-groom composes its critic; build and review route their own workers;
and finalize conditionally composes conflict-resolution and integration-repair agents. Any Codex
agent can make the same false inference if it treats a nested tool inventory as authoritative.

## Goals

- Make prose-routed and direct `@agent` invocation first-class, working paths for every generated
  Docket agent under Codex.
- Teach every generated Codex wrapper where native named-agent dispatch lives and how it must be
  called.
- Make Docket's shared dispatch-capability rule distinguish the active authoritative tool surface
  from an intentionally incomplete nested inventory.
- Preserve the existing rule that only an actual failed dispatch or explicit policy denial proves
  unavailability.
- Preserve every existing Tier A, Tier B, Tier C, and finalize carve-out posture after genuine
  unavailability is established.
- Add structural, mutation, full-suite, and live-harness validation covering all current nested
  composition families without maintaining a hand-written site allowlist.
- Document wrapper installation and the requirement to start a fresh Codex process before relying
  on changed agent definitions.

## Existing decisions retained

- ADR-0036 remains authoritative for the committed, machine-neutral `AGENTS.md` dispatch block and
  machine-local Codex wrappers.
- ADR-0059 remains authoritative that dispatch capability is resolved rather than inferred from a
  tool name, and that only a failed attempt or policy denial establishes unavailability.
- ADR-0060 remains authoritative that the Codex emitter must express Docket's contract using
  Codex's actual wrapper and tool model.
- ADR-0094 remains authoritative for the pinned `docket-plan-writer`, its single-artifact contract,
  and Tier-C failure posture.

This change closes a delivery/evidence gap inside those decisions. It does not replace the agent
topology, relax a discipline boundary, or add a new fallback execution model. A new ADR is not
expected unless reconcile discovers a decision that changes one of those existing rules.

## Design

### 1. Every generated Codex wrapper carries the direct-dispatch boundary

The Go Codex renderer prepends one Codex-specific developer-instruction paragraph to every emitted
`docket-*.toml` agent definition. Inventory iteration remains the source of coverage: the renderer
does not list composition-capable agent names or try to infer capability from an agent's current
body.

The paragraph states three semantic requirements:

1. when the active charter requires another agent, use Codex's direct named-agent dispatch from the
   active top-level tool surface;
2. nested orchestration inventories omit top-level collaboration controls; and
3. absence from such an inventory cannot establish dispatch unavailability.

The instruction is universal because composition is a property of the invoked skill and may change
through configuration. An allowlist of today's dispatching wrappers would silently miss a future
custom binding or newly composed role. Leaf agents receiving a conditional instruction incur no
behavioral change.

The shared source agents remain harness-neutral. Claude, Cursor, and OpenCode output must remain
byte-unchanged; the Codex emitter is the correct target-contract boundary for Codex-specific tool
placement.

### 2. Shared capability resolution names the authoritative surface by shape

The convention's dispatch-capability section is strengthened without naming a vendor tool. Before
declaring a required dispatch unavailable, an agent must resolve from its own active/top-level tool
surface. A nested namespace or inventory exposed by another tool is explicitly non-authoritative
because it may omit controls that cannot be invoked from inside that tool.

If the direct dispatch capability is present, the agent calls it directly. If resolution remains
inconclusive, the existing one-trivial-dispatch requirement still applies. Only the direct attempt's
rejection or an explicit policy denial establishes genuine unavailability. Inspecting a nested
inventory, an absent spelling, or an irrelevant tool-search result never satisfies the rule.

After genuine unavailability is established, current consequences remain unchanged:

- Tier A deterministic work may run inline;
- Tier B adversarial work abstains;
- Tier C discipline work requires explicit `auto` authorization or halts; and
- finalize's resolver/repair carve-out aborts and reports.

### 3. Both invocation paths share one nested-composition contract

The managed repository dispatch block continues routing a prose request to the registered same-name
agent. Direct `@docket-…` invocation continues starting that same wrapper. Neither path is
documented as a workaround or second-class mode, and neither may require changing a workflow skill
binding to `auto`.

Once inside the wrapper, nested composition follows the direct-dispatch instruction above. The
parent stays foreground and consumes the child's normal return according to the existing role
contract. This change does not introduce background dispatch, notification waits, shell-launched
Codex children, or runner-facade substitution.

### 4. Genuine capability failures stay loud

This change corrects false negatives; it does not pretend that every Codex version or policy allows
nested dispatch. A direct named-agent call that is rejected because the role is unregistered, the
nesting limit is reached, or policy denies dispatch is real evidence. The caller applies its
existing tier or carve-out posture and reports the concrete rejection.

No inline fallback becomes newly authorized. In particular, plan writing, build, review, and fix
workers remain Tier C, and `docket-plan-writer` retains its independent model/effort pin.

### 5. Installed wrappers are process-start state

The Go installer regenerates the machine-local Codex TOML wrappers with the new paragraph. Codex
registers agent definitions at process start, so installation output and runtime availability are
separate checkpoints. Documentation must require a fresh Codex application/CLI process after an
install that changes wrappers; opening another conversation inside an already-running process is
not presented as sufficient.

## Testing

### Renderer regression

Add a Go test over the complete real Codex inventory returned by the renderer. Every emitted agent
must contain the three semantic clauses: direct named-agent dispatch, nested inventories omitting
top-level collaboration controls, and nested absence being insufficient evidence of
unavailability. The expectation is derived independently as literal behavioral clauses, not by
reusing the renderer constant.

Mutation requirement: removing the renderer paragraph must make the focused test fail for the
generated inventory. Regenerate every Codex golden after the test is red; other harness goldens
must not change.

### Convention guard

Extend the existing dispatch-capability test at its canonical convention producer. The guard binds
the active/top-level-surface requirement to the nested-inventory exclusion and retains the existing
failed-attempt/policy-denial rule. Mutation-test removing either side: a nested inventory must not
become sufficient evidence, and a real failed direct attempt must remain sufficient.

### Whole-site coverage

Derive nested-dispatch consumers from a whole-repository search, then classify prose records versus
executable/current skill and agent sources. Tests must demonstrate that the universal Codex emitter
reaches every generated wrapper rather than hand-listing today's consumers. The live validation
matrix samples every composition family currently found by that derivation: implement-next
composition, profile-routed build, rung-routed review, auto-groom critic, and finalize resolver and
repair dispatches.

### Live Codex validation

Extend `docs/codex/validation-runbook.md` with a disposable-fixture procedure that starts a fresh
Codex process after installing the source build and exercises both entry paths:

- a prose request routed through the managed repository dispatch rule; and
- a direct `@docket-…` invocation.

The fixture must produce observable child-return sentinels for each composition family without
mutating the real backlog or depending on model narration. A dispatch attempt counts only when the
child actually starts and its expected return is consumed. An unavailable result is valid only when
the transcript contains the direct rejection or policy denial. The runbook records the Codex
version used for certification.

### Full gate

Run the configured `finalize.test_command` (`scripts/run-tests.sh`) and inspect its budget findings.
Any `SERIAL CONFIRMED OVER BUDGET` line is a failure to address before completion; parallel
`BUDGET WATCH` lines retain their existing screening posture.

## Documentation

- Update `docs/codex/setup.md` to state that prose and direct `@agent` invocation are both supported
  and that nested agent composition uses Codex's direct top-level dispatch capability.
- Update `docs/codex/validation-runbook.md` with the live matrix, expected evidence, version capture,
  and fresh-process requirement.
- Update README invocation guidance only where it currently distinguishes or implies one of the two
  paths; do not duplicate the full runbook.
- Keep shared normative prose harness-neutral. Codex-specific tool placement belongs in the Codex
  emitter and Codex documentation.

## Prototype disposition

The pre-change working-tree patch that first added a Codex renderer paragraph, one Go test, and
regenerated goldens was a diagnostic prototype. It is not implementation history and must be
removed from `main` before this change is handed to `docket-implement-next`. The implementer starts
from a clean integration branch, writes the failing tests first, and recreates the final design on
the change's feature branch.

The prototype's successful full-suite run is evidence that the narrow renderer direction is
compatible with the current suite, not evidence that this change is implemented. It did not add the
shared convention guard, documentation, derived site coverage, or live two-path certification.

## Out of scope

- A runner-facade or subprocess fallback for Codex versions that genuinely reject nested dispatch.
- Adding skill wrappers for agent-only workers or changing plan-writer's runtime skill passthrough.
- Changing model/effort defaults, agent worktree scopes, return protocols, or tier assignments.
- Backgrounding child agents, yielding for notifications, or changing the foreground return
  contract.
- Treating inline `auto` as an implicit recovery for a failed discipline dispatch.
- Reworking the run-gate wait/continuation design tracked by change 0359.

## Success criteria

- A freshly installed Codex process can enter every Docket skill through either prose routing or a
  direct `@agent` invocation and complete all nested dispatches its active charter requires.
- No generated Codex agent treats a nested orchestration inventory as authoritative capability
  evidence.
- A genuinely rejected direct dispatch still reaches the existing tier or carve-out behavior.
- Automated tests cover the universal renderer, the shared capability rule, inventory-derived site
  reachability, unchanged non-Codex output, and the complete configured suite.
- Codex setup and validation documentation explain both invocation paths, live evidence, and the
  fresh-process requirement.
