<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0393 — Enter compositional Docket roles as coordinator-capable Codex root threads](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0393-enter-compositional-docket-roles-as-coordinator-capable-code.md)**
<!-- docket:backlink:end -->

# Coordinator-capable Codex root entry for compositional Docket roles

**Change:** 0393 · **Date:** 2026-09-01 · **Type:** fix · **Priority:** critical

## Decision

Enter a Docket role that owns nested composition as a coordinator-capable Codex root thread, seeded from that role's installed definition. Do not enter that coordinator through the ordinary registered-agent child launch.

The parent-facing operation remains a named Docket-role dispatch and remains foreground. The change is at the Codex harness boundary: the host creates a root thread for the selected coordinator, applies the registered role contract to it, waits for the role's terminal return, and presents that return as the dispatch result. Once entered, the coordinator continues to launch its existing registered children directly. No parent participates in or relays those child edges.

## Why change 0384 did not fix the production path

Change 0384 concluded that native registered-agent launch was universally coordinator-capable. Its implementation therefore changed no production launch code; it added evidence and documentation around the existing depth-1 registered-agent path. The production resume of change 0364 then repeated the same failure: `docket-implement-next` had no top-level collaboration control with which to dispatch `docket-plan-writer` and correctly halted at Step 4.

The failure is not caused by a missing TOML registration. `/Users/homer/.codex/agents/docket-plan-writer.toml` registers the leaf, but registration answers only whether a parent can name that role. It does not give a depth-1 registered child a coordinator tool surface.

## Empirical decision evidence

The following sequential spikes were completed on 2026-09-01 before this change was created. Each used a fresh sentinel and was adjudicated from the persisted Codex session, not from a nested tool inventory.

1. **Broad collaboration for every spawned agent — generic runtime pass, production-role fail.** A generic depth-1 agent spawned a depth-2 leaf and consumed sentinel `UNIVERSAL-7E3C9A`, proving that Codex does not universally suppress nested collaboration. The exact newly spawned registered `docket-implement-next` role then reported `REGISTERED_UNIVERSAL_FAIL=top-level collaboration.spawn_agent unavailable in the exposed tool surface`. This candidate does not fix Docket's actual entry path and was rejected.
2. **Coordinator-capable root entry — pass and selected.** A disposable app-server root thread was seeded with a role developer token, model `gpt-5.6-luna`, reasoning effort `max`, and a skill token. It launched the registered leaf, which returned `LEAF_SENTINEL=ROOT-ENTRY-41C6B2`; the root returned `ROOT_DEV_TOKEN=DEV-41C6B2`, `ROOT_SKILL_TOKEN=SKILL-41C6B2`, and `ROOT_ENTRY_PASS=ROOT-ENTRY-41C6B2`. The persisted session source was `vscode`. This proves the supported root-entry primitives can preserve role instructions, model, effort, skill input, and nested named-agent dispatch in the target host path.
3. **Typed parent relay — pass but rejected.** A coordinator emitted a typed relay request, the parent launched the leaf, and the same coordinator resumed to return `RELAY_PASS=RELAY-9B40D7`. It is a viable fallback, but it adds a new continuation protocol and makes the parent part of every child edge. The selected root-entry solution does not use it.

The point-in-time transcripts are under `/Users/homer/.codex/sessions/2026/09/01/`: the generic nesting root `rollout-2026-09-01T07-07-05-01a05ca6-bb32-7501-8f51-06178d2dee33.jsonl`, exact registered-role failure `rollout-2026-09-01T07-16-48-01a05caf-a077-7b81-a16c-1042dd6f4104.jsonl`, selected root entry `rollout-2026-09-01T07-14-47-01a05cad-c761-7a31-b93f-9b5710343bcd.jsonl`, its leaf `rollout-2026-09-01T07-15-11-01a05cae-2458-7d72-8853-7ab6e3776bc1.jsonl`, and relay coordinator `rollout-2026-09-01T07-16-03-01a05cae-ee71-7cb2-b57c-80badbafe3f4.jsonl`.

## Architecture

### 1. Represent root-entry posture explicitly

Add one closed, machine-readable launch posture at the common agent-inventory boundary, with at least ordinary child and root coordinator values. Mark roles by behavior: a role whose active contract owns a nested agent dispatch requires root-coordinator entry when invoked from a Codex parent-facing Docket workflow. Leaves remain ordinary registered children.

Do not hand-list coordinator filenames in tests. Derive the population from the authoritative agent inventory and add a correspondence guard against the syntactic shape of dispatch-owning contracts. Mutation-test the guard by removing or misclassifying a known coordinator and proving the test fails.

Other harnesses ignore the Codex-only launch effect and retain byte-identical generated output unless they independently consume the neutral posture field.

### 2. Use one role contract for registration and root entry

The installed/generated Codex role definition remains the source of truth for:

- role name and developer instructions, including the recursion guard;
- configured model and reasoning effort;
- required Docket skill preload or turn skill input; and
- role description and charter.

The root-entry adapter must consume that same structured render input or parse the installed Docket-owned TOML through a typed reader. It must not maintain a second prose template or a second model/effort resolver. A drift test proves ordinary registration and root entry receive identical role values.

### 3. Enter through Codex's native root-thread protocol

Implement a Codex harness-native entry adapter over the supported app-server protocol demonstrated by the spike:

1. create a root thread in the caller's repository and permission context;
2. apply the role's developer instructions and configured model at thread start;
3. apply configured reasoning effort and the role's resolved skill input at turn start;
4. pass the user's Docket request through unchanged as the role's user input;
5. wait in the foreground for the turn-completed or terminal-error event; and
6. return the role's final output to the original Docket dispatch boundary.

The adapter must use the VS Code-backed host connection or another supported in-process/native app-server entry surface. It must not shell to `codex exec`, start an unrelated cross-harness runner, or depend on the under-reporting exec JSON item stream. The root coordinator's persisted source and context are evidence inputs, not inferred from process names.

If the current VS Code integration cannot expose the demonstrated root-thread operation to the parent-facing Docket dispatch surface, stop with a precise unsupported-host error. Do not silently fall back to ordinary child launch or to the typed relay.

### 4. Preserve Docket workflow contracts

`docket-implement-next` continues to own the Step-4 edge and dispatch payload. `docket-plan-writer` remains an ordinary registered leaf and retains ADR-0094's `PLAN_PATH=<path>` receipt and Git proof. Model/effort pins, skills, worktree boundaries, foreground posture, Tier-C authorized-or-halt behavior, gate attribution, and all other child contracts remain unchanged.

The parent-facing managed dispatch surface routes a root-coordinator role through the new entry adapter. Direct child dispatches issued from inside that coordinator continue to use Codex's native named-agent control. There is no new agent-to-agent message schema and no parent relay state.

### 5. Keep failure attribution explicit

Different failures remain distinguishable:

- missing or invalid installed role definition;
- root-thread creation rejected by the host;
- role contract could not be applied;
- coordinator turn failed before child dispatch;
- named child dispatch rejected; and
- coordinator completed without the caller-required durable receipt.

Only the first three are root-entry adapter failures. Child-dispatch and receipt failures continue through the owning Docket workflow's existing halt or verification posture.

## Testing and acceptance

### Automated contract tests

- Test launch-posture parsing, unknown-value refusal, and inventory-derived coordinator correspondence.
- Test that root entry and ordinary wrapper registration consume identical developer instructions, model, effort, skill binding, and recursion guard.
- Test the app-server request/event mapping, foreground completion, error classification, working directory, permission context, and unchanged user payload.
- Prove unaffected harness goldens remain byte-identical.
- Retain ADR-0059's negative-evidence rule: a nested inventory omission alone is still not dispatch rejection.

### Behavioral regression

Run an isolated fresh-process fixture through both sides of the defect:

- old ordinary registered-child entry: the exact coordinator cannot start the named leaf;
- new root-coordinator entry: the same role contract starts the same registered leaf and consumes a fresh sentinel.

Then run the actual installed `docket-implement-next` definition through root entry in a disposable repository or otherwise non-backlog-mutating fixture and require a real `docket-plan-writer` launch and consumed return. Evidence must show the coordinator's root status, persisted `vscode` source, developer-instruction marker, model, effort, skill marker, leaf role identity, and matching sentinel/receipt.

Mutation-test the production route by forcing `docket-implement-next` back through ordinary child launch and showing the real-edge assertion fails at coordinator-to-plan-writer dispatch.

These checks run against the feature implementation before completion. This change does not add a generic post-merge or unchecked-verification process requirement.

### Full gate

Run `go run ./cmd/docket development test` from source and handle budget findings under the repository's existing gate rules.

## Documentation and decisions

- Update the Codex setup and validation material to distinguish role registration, ordinary child launch, and root-coordinator entry.
- Update the agent-layer reference with launch-posture configuration and the single-source role-contract rule.
- Record a narrow ADR if the production root-entry surface changes ADR-0036's committed machine-neutral parent-routing boundary; do not silently overload that accepted decision.
- Correct change 0384 only by reference. Its archived change, spec, plan, and results remain frozen point-in-time records.

## Out of scope

- A generic process rule for checks that could not be performed before merge.
- Granting broad collaboration controls to every spawned agent.
- The typed parent-relay candidate or any relay fallback.
- Generic-agent substitutes, shell runners, `codex exec` subprocess sessions, or cross-harness delegation.
- Changes to Docket's role topology, child payloads or receipts, Tier-C authorization, model/effort pins, skill bindings, worktree scopes, or run-gate continuation.
- Editing or recharacterizing the frozen artifacts of changes 0365 or 0384.

## Success criteria

- A parent-facing Codex Docket dispatch enters `docket-implement-next` as a coordinator-capable root thread rather than an ordinary depth-1 agent.
- Root entry uses the same authoritative installed role contract as registration and preserves instructions, model, effort, skill input, request, working directory, permissions, and recursion guard.
- The root coordinator directly dispatches registered `docket-plan-writer`, consumes its return, and advances beyond the Step-4 boundary that halted change 0364.
- Reverting the production route to ordinary child launch makes the behavioral test fail.
- Leaves and unaffected harnesses retain their existing contracts and generated bytes.
- No typed relay, generic runner, subprocess session, broad-spawn delegation grant, or generic verification-process requirement is introduced.
- The complete repository build gate passes.
