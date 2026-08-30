<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0371 — Cut generated agent invocation over to native host dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-30-0371-cut-generated-agent-invocation-over-to-native-host-dispatch.md)**
<!-- docket:backlink:end -->

# Cut Generated Agent Invocation Over to Native Host Dispatch

## Summary

Change 0371 removes the maintained assumption that Docket agents are launched through Bash
`runner-dispatch`. Canonical harness assets and their generated installations instead direct
Claude, Codex, Cursor, and OpenCode to invoke registered `docket-*` agents through the current
host's native named-agent facility.

This is a dispatch-boundary migration only. The caller continues to own Docket's implement-next
gate protocol; native dispatch changes how the child starts, not how attribution, waiting, halt,
retry, or durable completion is decided.

## Preconditions and assumptions

- Change 0369 is merged.
- Change 0317's four-host registration/generation architecture remains the canonical source.
- Each supported host has a native mechanism capable of invoking a registered agent by identity.
- The installed `docket-*` name, not a description, file path, model, or shell command, is the
  stable dispatch identity.
- Host syntax may differ, but all four adapters derive semantics from one canonical policy.
- A missing registration or native capability fails visibly; it never falls back to a shell
  runner, another harness, a generic agent, or silent inline execution.
- Exact gate command spelling at implementation time comes from the supported Go interface.

The run halts for re-grooming if reconciliation shows there is no shared generator/registration
seam and the four hosts would require independent dispatch subsystems.

## Goals

- Invoke installed Docket agents through each host's native named-agent mechanism.
- Preserve the exact agent identity and unchanged user request, including IDs and constraints.
- Preserve caller-side gate-before/gate-verdict authority around implement-next.
- Keep canonical policy, host adapters, and generated installations deterministic and synchronized.
- Expose missing agent registration or native dispatch as a clear installation/capability failure.
- Prove fresh isolated installations without relying on developer-machine state.

## Non-goals

This change does not:

- add a Go delegation, `runner-dispatch`, or cross-harness operation;
- create a generic subprocess abstraction or let one host launch another host's agents;
- migrate retained lifecycle operations owned by 0369;
- retire deferred features or add the final global consumer seal owned by 0372;
- delete Bash runner/facade code or mechanism tests owned by 0370;
- rename agents or redesign their skills, prompts, models, or reasoning pins;
- change gate attribution, retry policy, or durable run state;
- add more harnesses; or
- perform release, rollback, or human four-host acceptance.

## Ownership model

The dispatch path has three separate owners.

1. **Host-native dispatch** resolves the exact registered agent, creates the child execution
   context, supervises it, and delivers an immediate result or later notification.
2. **Docket's caller-side gate** owns attribution, durable state, retry authorization, waiting and
   halt interpretation, and the final verdict.
3. **The registered Docket agent** owns the workflow, prompt, skill preload, model, and reasoning
   configuration associated with its exact name.

Successful native launch is not proof of successful workflow completion. Child prose, host tool
status, and process exit cannot supersede the gate verdict.

```text
caller
  -> resolve exact same-name docket-* registration
  -> obtain gate attribution when workflow is implement-next
  -> dispatch natively with the unchanged request
  -> receive immediate result or later completion notification
  -> obtain the Docket gate verdict
  -> obey that verdict exactly
```

When a workflow has no registered same-name agent, the existing inline or unavailable-capability
contract applies. Generated instructions do not invent a registration.

## Canonical dispatch policy

One generator-owned policy defines the semantics shared by all four adapters:

- the current host's native registry is authoritative for availability and names;
- dispatch the exact registered same-name `docket-*` agent;
- pass the user's request through unchanged;
- never reconstruct a lossy child prompt when the host can pass the original request;
- never invoke `runner-dispatch`, a shell runner, another harness, or a generic substitute;
- never run a registered-agent workflow inline merely because native dispatch failed;
- keep implement-next gate bracketing in the parent/caller context;
- treat the gate report as authoritative over host status and child prose;
- permit only the gate's explicit retry-once outcome, once; and
- report waiting, halted, stop, observe, and unattributed outcomes without launching a fresh child
  that cannot resume the prior execution.

The shared semantics are not maintained as four independent prose copies.

## Per-host adapters

### Claude

Generated Claude instructions use the registered-agent/subagent facility with the exact Docket
agent name, forward the request, retain gate ownership in the caller, handle immediate or detached
completion, and fail visibly when the registration is absent.

### Codex

Generated Codex instructions use native named-agent dispatch, preserve the original request and
the parent's attribution key, obtain the verdict after child completion/notification, and never
treat collaboration completion text as the authoritative Docket result.

### Cursor

Generated Cursor instructions use Cursor's native agent facility and deterministically map the
canonical Docket identity to any host-specific metadata. Native launch success is distinguished
from Docket workflow success. Missing registration cannot fall back to shell or inline work.

### OpenCode

Generated OpenCode instructions use its native task/agent mechanism with the exact generated
identity, forward the request, retain parent gate ownership, handle immediate or later completion,
and expose missing registration as failure. Host-specific syntax stays in the adapter.

## Gate preservation

Generated implement-next instructions preserve these invariants:

1. The caller obtains the attribution key before native dispatch when the gate can arm.
2. The caller retains the key outside the child context.
3. The child neither manufactures nor returns attribution.
4. The caller obtains the verdict after a result or completion notification.
5. A keyless launch uses the established unattributed verdict path; a known change ID is a hint,
   not reconstructed authority.
6. Host success, child prose, timestamps, branches, and process status never authorize redispatch.
7. Only retry-once permits one second invocation of the same agent for the named unmet work.
8. Waiting, halted, stop, or observe outcomes end the dispatch loop as the gate directs.

This change may update command spelling to the current supported Go gate surface but does not
duplicate or redesign the gate state machine.

## Canonical and generated ownership

The repository has one identifiable canonical owner for shared dispatch semantics and one adapter
or template owner per host. All checked-in, embedded, or installed copies are derivatives.

Regeneration must:

- start from canonical policy/adapter sources;
- enumerate outputs from generator ownership or installation manifests rather than a caller-file
  allowlist;
- validate managed-marker order and balance before editing;
- preserve unrelated user content outside managed blocks;
- refuse malformed blocks without partial writes;
- render byte-stable output; and
- support an isolated destination without depending on an existing user installation.

A second unchanged generation produces no diff. Generated copies are not hand-edited as the
primary fix.

## Compatibility and failure handling

- **Missing agent:** name the unavailable registration and direct the operator to installation or
  registration repair; do not substitute another path.
- **Native dispatch unavailable:** report the host capability failure; do not emulate it.
- **Launch succeeds but workflow halts:** obtain and report the gate verdict.
- **Only a completion notification arrives:** use the retained parent attribution.
- **No key exists:** use the gate's unattributed path; do not infer one.
- **Unknown workflow:** use only the pre-existing no-registration inline/unavailable rule.
- **Stale installation:** deterministic install tests expose its drift.

There is no maintained dual-path compatibility interval. Historical runner references remain in
point-in-time records; active dispatch blocks switch as one mergeable unit.

## Testing

Canonical-policy tests prove exact-name native dispatch, unchanged request forwarding, no invented
agent or shell/cross-harness fallback, parent gate ownership, and verdict authority.

Each host adapter has fixture coverage proving its native dispatch shape, identity mapping,
absence of runner-dispatch in maintained output, gate ordering, visible missing-agent failure, and
stability of unrelated generated content.

Fresh installation tests generate into an isolated destination for all four hosts. They must not
inherit home-directory files, installed plugins, host configuration, repository-visible generated
outputs, or another host's installation. When a proprietary host cannot run in CI, the hermetic
adapter fixture consumes installed artifacts and models both exact resolution and missing-agent
failure; a prose-substring assertion alone is insufficient.

Shared gate-flow coverage exercises immediate and detached completion, retained attribution,
verdict authority over success-shaped child prose, retry-once bounds, and no fresh dispatch for
waiting or halted outcomes.

Mutation tests must redden when:

- runner-dispatch is restored;
- native dispatch is removed from one host;
- gate-before or gate-verdict guidance is deleted;
- exact identity is weakened to generic inference; or
- missing registration falls back to inline or shell execution.

The full suite runs from source:

```sh
go run ./cmd/docket development test
```

## ADR handling

The ADR audit preserves decisions establishing machine-neutral generated dispatch and gate
authority, establishes native host dispatch as authoritative for registered Docket agents, and
confirms cross-harness delegation is outside Go v1. Runner-era decisions that normatively conflict
with this architecture are superseded or reversed through supported ADR transactions; accepted
bodies and other historical evidence are not rewritten. A redundant new ADR is avoided when the
existing accepted decisions already state the complete target architecture.

## Rollout

1. Derive the canonical policy, host-adapter, generated-output, and fixture inventory.
2. Update the shared policy and all four adapters as one cutover.
3. Regenerate every owned artifact twice.
4. Reconcile necessary ADR dispositions.
5. Run isolated adapter/install, gate-flow, mutation, and full-suite tests.
6. Merge 0371 before beginning 0372.

External installations receive the new contract through their normal install/update path. Real
four-host acceptance remains human-attended work outside this change.

## Acceptance criteria

1. One canonical policy owns shared native-dispatch semantics across all four hosts.
2. Each host deterministically renders its native adapter from that policy.
3. Generated instructions invoke the exact registered same-name `docket-*` agent and preserve the
   original request.
4. No maintained canonical/generated dispatch block invokes or recommends runner-dispatch.
5. No maintained dispatch path requires a new Go delegation operation or another harness.
6. Missing registration/capability fails visibly without shell, generic-agent, cross-harness, or
   silent-inline fallback.
7. Implement-next gate-before and gate-verdict remain caller-side and correctly ordered.
8. Only the gate report authorizes redispatch; retry-once permits at most one retry.
9. Waiting, halted, stop, observe, and unattributed outcomes remain fail closed.
10. Generator-owned outputs are complete, marker-safe, deterministic, and clean on repeat render.
11. Fresh isolated tests cover Claude, Codex, Cursor, and OpenCode without host-state leakage.
12. Tests cover exact identity, request forwarding, missing-agent failure, immediate/detached
    completion, gate authority, and retry bounds.
13. Required mutations fail for the intended reason.
14. Conflicting runner-era ADR decisions are formally dispositioned without rewriting history.
15. The source-derived maintained native-dispatch surface has no remaining runner-dispatch call.
16. The PR does not absorb 0369, 0372, 0370, or release/self-host work.
17. `go run ./cmd/docket development test` passes under the documented budget semantics.

## Size verdict

This fits one autonomous PR because it is one shared dispatch contract rendered through four
existing adapters. It adds no CLI operation, lifecycle migration, deferred-feature retirement,
global seal, facade deletion, or live multi-host acceptance. If the assumed common generator seam
does not exist, reconciliation halts instead of creating four independent subsystems.
