<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0384 — Launch compositional Docket agents in coordinator-capable harness contexts](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-01-0384-launch-compositional-docket-agents-in-coordinator-capable-ha.md)**
<!-- docket:backlink:end -->

# Harness-native coordinator invocation

**Change:** 0384 · **Date:** 2026-08-31 · **Type:** fix · **Priority:** critical

## Problem

Change 0365 corrected a false capability inference: a Docket agent must not conclude that nested
dispatch is unavailable merely because a nested tool inventory omits top-level collaboration
controls. It added a Codex-specific instruction to every generated wrapper and automated tests for
the generated prose. Its results explicitly left the fresh-process, two-entry-path live
certification unchecked.

The implement-next run for change 0364 became that missing live test and failed. The outer session
successfully used Codex's native registered-agent control to start `docket-implement-next`. Inside
that registered agent, Step 4 required a foreground `docket-plan-writer` dispatch, but the active
top-level tool surface exposed no collaboration control with which to make it. The agent therefore
applied the existing Tier-C rule and durably halted before planning. No plan, build, review, or PR
was produced.

A separate harmless probe reproduced the topology independent of Docket role prose: a root session
could start a child, while that child had no control for starting a grandchild. The root's native
registry included the installed Docket agent types. Official Codex multi-agent documentation
supports children, grandchildren, and deeper descendants. The defect to solve is therefore the
way Docket enters or configures a composition-owning agent in this Codex integration, rather than
the existence of nested agents as a product capability.

Change 0365 changed instructions inside the launched session without first proving that the launch
gave that session the capability those instructions named. More agent prose cannot create a tool
that the invocation omitted.

## Goals

- Identify and certify the native Codex invocation that starts a registered Docket agent in a
  coordinator-capable context.
- Make both supported Docket entry paths use a launch that preserves every nested dispatch required
  by the active workflow.
- Keep semantic topology, dispatch payloads, foreground posture, return protocols, and verification
  in the caller skills where they currently live.
- Keep each child's behavior in its common `agents/docket-*.md` source.
- Put native launch mechanics in the harness adapter or generated harness surface, without creating
  a handwritten agent-by-harness instruction matrix.
- Turn the live root-to-coordinator-to-child check into required completion evidence rather than an
  unchecked follow-up.
- Preserve the existing loud failure posture when a native launch or nested dispatch is actually
  rejected or denied.

## Existing decisions and related work

- ADR-0036 governs repository-owned parent routing and machine-local Codex wrappers. If the proven
  launch requires Codex-specific parent syntax that ADR-0036's shared byte-identical block cannot
  express, implementation must record the narrow superseding decision rather than smuggling vendor
  syntax into shared prose.
- ADR-0059 still governs capability evidence: only an attempted operation's rejection or an explicit
  policy denial proves unavailability.
- ADR-0060 still requires generated wrappers and routing surfaces to conform to the target harness's
  actual contract.
- ADR-0094 still owns the pinned plan-writer, its single-artifact receipt and Git proof, and its
  Tier-C posture.
- Change 0359 owns run-gate waiting, continuation, and recovery. This change does not alter that
  state machine.
- Change 0364 supplies the failed live transcript and durable `## Run halted` record. It is not
  modified by this change; after this fix lands, resuming 0364 is the real-work confirmation.

## Design

### 1. Prove the launch primitive before changing generation

Begin with a disposable, fresh-process Codex fixture. It must contain two synthetic registered
roles: a coordinator whose only operation is to start a named child and block for its return, and a
leaf that returns a unique sentinel without touching a repository. The probe records:

- the Codex version and feature configuration;
- the entry path and exact native invocation shape;
- whether the coordinator's active top-level surface contains the collaboration control;
- whether the named child actually starts; and
- whether the coordinator consumes the child's exact sentinel.

Exercise the two entry paths separately:

1. a plain-language request routed by the repository's managed Docket dispatch surface; and
2. direct invocation of the registered coordinator agent through Codex's supported agent entry
   surface.

Test the current Docket launch first so the fixture reproduces the observed leaf-session failure.
Then test every accessible native coordinator launch exposed by the same Codex version. Candidate
parameters or entry modes count only when the grandchild starts; inspecting a schema or tool list is
not success evidence.

No production design is selected merely because an instruction sounds correct. The selected launch
must pass the fixture. If no accessible native path can preserve coordination, stop and report that
finding before implementing a relay or fallback. A parent relay is a separate design decision and
is not silently admitted by this change.

### 2. Preserve three distinct sources of truth

Docket keeps these concerns separate:

1. **Workflow edge:** the caller skill names the child, foreground/background posture, dispatch
   payload, return receipt, and durable verification. For example, implement-next Step 4 continues
   to own the `docket-plan-writer` edge.
2. **Role behavior:** `agents/docket-*.md` continues to define what the launched agent does, its
   scope, and its success or failure output.
3. **Harness launch:** the target harness adapter and its generated parent/wrapper surfaces encode
   how a role is entered and how a coordinator starts a named child on that harness.

The first two remain harness-neutral. The third may name native Codex concepts because it is the
place where those concepts are true.

### 3. Encode the proven coordinator launch at the narrowest harness boundary

After the prototype identifies the working mechanism, encode it according to where Codex exposes
the distinction:

- If coordinator capability is a property of the registered wrapper, emit the required setting or
  instruction from `internal/harness/codex`; do not duplicate it in shared agent bodies.
- If it is a property of the parent's invocation, make the Codex repository dispatch emitter use
  the proven entry operation. Keep the common semantic routing policy shared, but do not preserve
  byte identity at the cost of an incorrect native call.
- If the workflow must remain in the root session while assuming a registered role, generate that
  supported role-entry operation explicitly. Do not reconstruct the Docket workflow inline from a
  description and do not replace the registered role with a generic agent.

The selected path must retain the role's configured model and reasoning effort, skill preload,
developer instructions, worktree scope, and recursion guard. A launch that restores nesting by
bypassing the wrapper is not equivalent and does not satisfy the change.

### 4. Use per-agent metadata only when the harness requires it

Do not create separate Codex, Claude, Cursor, and OpenCode prose files for every dispatch edge.
Native syntax belongs once per harness; edge semantics belong once per caller skill.

If the proven Codex mechanism requires distinguishing coordinators from leaves, add one closed,
machine-readable launch-posture field to the common agent inventory and have renderers consume it.
Derive the initial population from a whole-repository scan of active dispatch-owning contracts,
including roles that invoke configurable skills whose own contracts may dispatch. Add a
correspondence guard so a new dispatch-owning role cannot remain classified as a leaf. Harnesses
that do not need the distinction must render unchanged output.

If Codex can safely give every registered Docket agent coordinator capability, prefer that simpler
universal rule. In that case no role classification is added.

### 5. Keep nested calls native and foreground

Once a Docket coordinator is entered correctly, its existing skill issues each required named-child
dispatch through the native collaboration control and blocks for the return. Docket does not add a
shell subprocess, another harness, a generic worker, a background notification wait, or an
agent-to-agent message relay.

Existing child contracts remain unchanged. The plan-writer still returns `PLAN_PATH=<path>` and is
proved through Git; build workers and reviewers retain their current receipts and durable evidence;
critic, resolver, and integration-repair returns remain in context. A bare child completion remains
insufficient where the caller currently requires durable proof.

### 6. Make live certification completion evidence

The branch's results record must contain a completed certification section, not an unchecked
human-verification checklist. It records the exact Codex version, both entry paths, selected launch
shape, coordinator sentinel, child sentinel, and the observed failed-current/fixed-new comparison.

The validation runbook provides the reproducible fresh-process fixture, but the recorded run is the
acceptance evidence. Automated generator tests cannot substitute for it. A PR may be opened for
review while an external limitation is clearly reported, but the change must not be finalized as
done without the successful nested sentinel or an explicitly approved redesign.

After the fix is installed in a fresh process, resume change 0364 through its existing
`resume-halted` contract. Reaching and successfully consuming its real `docket-plan-writer` return
is the production confirmation; it supplements the disposable fixture and does not replace it.

## Testing

### Prototype regression

Keep a deterministic fixture or probe script that reproduces the old invocation and validates the
new one. The assertion is behavioral: root starts coordinator, coordinator starts named leaf, leaf
returns the unique sentinel, and coordinator returns proof that it consumed that sentinel.

Mutation requirement: force the generator or routing surface back to the old launch shape and show
that the nested sentinel check fails at the coordinator-to-child edge.

### Generator and adapter tests

- Assert the Codex adapter emits the selected native launch contract at the correct boundary.
- Derive coverage from the real agent inventory; do not hand-list generated artifact paths.
- If launch-posture metadata is introduced, reject unknown values and prove the correspondence guard
  fails when a dispatch-owning role is classified as a leaf.
- Preserve byte-identical output for unaffected harnesses and roles. Any intentional change to a
  shared parent surface must have focused ownership and coexistence tests for every harness that
  reads that file.
- Keep the change-0365 negative-evidence tests: nested inventory absence still cannot prove
  dispatch unavailability.

### Live fresh-process matrix

Run the fixture through both supported Codex entry paths after installing the branch and restarting
the process. Record exact sentinels and version in the results file. Then exercise at least one real
Docket composition edge without mutating the active backlog; change 0364 is resumed only after the
fix is merged and installed.

### Full gate

Run `go run ./cmd/docket development test` from source and inspect all budget findings under the
repository's existing gate rules.

## Documentation

- Update the Codex setup guide with the proven entry and nested launch mechanics.
- Update the Codex validation runbook with the disposable two-role fixture, expected sentinels,
  version capture, process restart, and failed-current/fixed-new comparison.
- Explain the workflow-edge, role-behavior, and harness-launch separation in the agent-layer
  reference so future changes place instructions at the correct boundary.
- Do not document an unproved `@agent`, fork, spawn parameter, or tool spelling as supported.

## Out of scope

- Changing existing Docket composition topology, dispatch payloads, return protocols, model or
  effort pins, worktree scopes, Tier assignments, or `auto` authorization.
- Parent relays, generic-agent substitutes, shell runners, subprocess Codex sessions, or
  cross-harness fallback.
- Reworking run-gate waiting and continuation from change 0359.
- Treating the current halted change 0364 as abandoned or resetting its preserved workspace.
- Claiming that every Codex client or version has the same launch shape; certification is scoped to
  the recorded version and integration.

## Success criteria

- The old launch is behaviorally reproduced as a coordinator-to-child failure in the disposable
  fixture.
- The selected native launch starts the same registered coordinator with its wrapper contract and
  gives it working named-child dispatch.
- Both supported entry paths complete root-to-coordinator-to-child and consume the exact sentinel in
  a fresh Codex process.
- Docket's caller skills and child role bodies remain the single harness-neutral sources for edge
  semantics and role behavior.
- No handwritten agent-by-harness dispatch matrix is introduced.
- Genuine launch or child-dispatch rejection still reaches the existing loud failure posture.
- Results contain completed live evidence with the Codex version; no mandatory certification item
  is left unchecked when the change is marked done.
- After installation, resumed change 0364 can dispatch `docket-plan-writer` and continue beyond its
  previous Step-4 halt.
