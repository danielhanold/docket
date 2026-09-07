<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0345 — Slash-command implement dispatch isn't agent-owned — attribution gap forces human-in-the-loop](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0345-slash-command-implement-dispatch-attribution-gap.md)**
<!-- docket:backlink:end -->

# Change 0345: attributed command entry across all supported harnesses

Design approved by Danny on 2026-09-07.

## Goal

Make the normal user-invoked implement command enter the same attributed run-gate workflow as an assistant-initiated dispatch on Claude, Codex, Cursor, and OpenCode. Preserve each harness's native named implementer, configured model and effort, existing child isolation, and human merge gate.

The user keeps the ergonomic command or explicit skill invocation. The parent performs a short coordination sequence before launching the autonomous implementer and after it returns. No additional coordinator subagent is introduced.

## Current evidence and scope correction

Source inspected: `main` at `0d1a7e1b36af36c1f805952cd3836e84dbad5b6f`; live proposal: change 0345 on `docket`.

- `internal/harness/harness.go` defines the supported set in `harness.Order`: `claude`, `codex`, `cursor`, `opencode`. All four are required delivery targets.
- `skills/docket-implement-next/SKILL.md` still declares `context: fork` and `agent: docket-implement-next`. The original report concerned a direct slash-command fork bypassing the parent's pre-dispatch gate. This observation must not be generalized to every harness or version.
- Change 0407 is done. `RunGateBefore` already returns a gate key and dispatch context. `change.claim --gate-context` commits a durable dispatch-to-claim proof. `RunGateVerdict` resolves that proof, detects replacement claims, and rejects missing or conflicting proof. Its before-set and timestamp are diagnostic only.
- Change 0393 is implemented but unmerged ([PR 265](https://github.com/danielhanold/docket/pull/265), open at review). It adds Codex's native root-coordinator entry for compositional roles. This design depends on 0393 reaching done and must reconcile its landed entry/catalog contract before implementation. Generated TOML alone does not establish coordinator capability.
- ADR-0111 supersedes ADR-0075's attribution inference. ADR-0084's conservative, observe-only posture remains relevant; this change must clarify its now-historical snapshot prescription without restoring it as authority.
- ADR-0100 requires native named-agent dispatch and rejects runner, generic-agent, cross-harness, and inline implementation fallbacks. Older runner-delegation documentation is not a supported route for this change.
- `internal/harness/guard.go` already emits an explicit assigned-agent charter and self-recursion guard. The launcher/worker boundary must preserve it.

The remaining gap is producing and retaining the gate context at command entry, handing it to the worker, and reaching the existing verdict loop. A second attribution token, a new claim format, and a new retry mechanism are unnecessary.

## Selected approach

Use the current parent session as the command coordinator. Separate its short entry procedure from the autonomous worker instructions, while retaining the public workflow name and native `docket-implement-next` agent identity.

The public skill executes in the current session. Its entry contract distinguishes two contexts using the actual assigned wrapper charter:

1. **Ordinary parent invocation:** run the launch-and-verdict procedure, then enter the registered native `docket-implement-next` role using the harness's required launch posture. This is native named-child dispatch where supported, and Codex's 0393 root-coordinator entry for that compositional role.
2. **Already executing that named agent's assigned charter:** load and execute the autonomous worker procedure directly. Do not launch another copy of the same agent or arm a second gate.

Move the existing long worker procedure into a reference within the same skill directory and keep one canonical copy. Generated agent instructions explicitly identify the worker path. Merely mentioning an agent name, passing an id, or loading a similarly named skill does not establish the worker charter. The existing generated recursion guard remains authoritative.

The shared public skill must not retain unconditional auto-fork metadata. Product-specific command metadata or dispatch syntax belongs only in the relevant adapter. Parent-facing routing instructions must explicitly bracket implement-next dispatch; the generic same-name dispatch rule must not bypass the pre-dispatch gate. An already assigned worker remains exempt from dispatching itself.

This preserves the existing agent roster and implementation depth. The parent runs coordination only; selection, claiming, reconciliation, planning, building, review, and PR creation remain with the named worker.

## Alternatives considered

**Pre-fork shell injection or a vendor hook:** can potentially create a token on a particular host, but does not by itself give the parent the key or ensure the post-run verdict executes. Hook availability and evaluation order differ by host and mode. It is not the shared contract and is not required by this design.

**A new supervisor subagent:** can own the gate, but adds an agent level before the implementer, consumes nesting capacity, and complicates model/effort and continuation ownership. The current parent can perform this work without another agent.

**Document the restriction only:** preserves safety but leaves the requested recovery improvement undelivered. Observe-only remains the fallback for genuinely unprovable runs, not the success criterion for a supported command entry.

## Shared launch and return contract

### Fresh invocation

1. Preserve the request's repository, optional single id or id allowlist, and existing selection semantics. An explicit id does not imply resume and does not override readiness, dependencies, or another agent's claim.
2. Resolve the native same-name registration and the cataloged Docket operations through the existing convention. Missing registration or required operations is a visible capability failure; do not substitute another agent or execute the implementer inline.
3. Before native launch, invoke `run.gate-before` for `implement-next`. Retain its exact key in the parent and pass its exact dispatch context to the worker. Validate the catalog and operation result according to the existing protocol contract. One initial invocation creates one gate; no wrapper or worker creates another gate for it.
4. Dispatch the named implementer through the current harness's native mechanism, preserving the requested scope and resolved model/effort. For a Codex root-coordinator, use 0393's catalog-resolved entry operation with the exact role contract, request, working directory, and current permission context; do not launch an ordinary child instead. Do not add a shell runner or require a cross-harness process.
5. The worker passes the context to `change.claim` and the existing gate-drive operations that require it. Invalid or conflicting context remains a refusal; never retry the claim without the token. Losing a selection CAS may follow the existing re-selection procedure, but must not bind the token to a second successful claim.
6. After the native run returns or the correlated completion notification arrives, invoke `run.gate-verdict` with the retained key. The child report may inform the human-facing summary but cannot supply ownership or retry authority.

### Return handling

- `gate-retry-once`: dispatch the same implementer for the attributed id and unmet conjuncts once, retaining the same key and dispatch context. Do not re-arm the gate or restore the consumed retry allowance. Evaluate the same gate after the attempt returns.
- `gate-continue`: resume the same attempt with the exact change id, continuation id, phase, and gate key. Preserve the dispatch context needed by nested gate operations. The worker redeems the continuation through `run.gate-claim` before ordinary implementation work. No new claim, replacement test run, new gate, or retry allowance is created.
- `gate-stop` and `gate-observe`: stop or report as directed; neither authorizes another dispatch. `run-halted` still requires human action.
- Terminal completion: report only the gate-verified outcome. A child completion message or a successful tool exit is not a successful implementation verdict.

Use the existing gate vocabulary and atomic retry accounting. Do not implement another decision table in an adapter or infer liveness from an id, elapsed time, timestamp, process exit, or notification.

### Failure boundaries

Preserve `gate-unarmed` behavior: an initial ungated dispatch remains allowed by the existing contract, accompanied by a clear loss-of-attribution diagnostic and observe-only verification. It never earns autonomous recovery. A supported installation must demonstrate the armed path; documenting only this fallback is insufficient.

A failed or uncertain launch never authorizes blind repeated dispatch. Query the retained gate when possible and follow its outcome. A failure before any successful claim can yield `no-attributable-claim`; this change does not add a pre-claim transport-retry policy.

If a parent loses its key, a notification supplies no trustworthy correlation, or a new session cannot access the originating local gate store, verification remains unattributed. Never scan for a recent gate, borrow a sibling run's key, or manufacture authority from the child's prose. Full recovery after parent-session loss or on another machine is outside this change.

Explicit, authorized resume retains the existing `run.gate-before --resume` verification and halted/quiescence requirements. Passing an already claimed id to the ordinary command is not permission to take it over. Two separate user invocations remain two attempts subject to claim CAS; they are not automatically deduplicated by id.

## Harness delivery matrix

The behavior is common; the invocation spelling and generated wrapper format remain native.

| Harness | Required user entry | Required delivery and validation |
|---|---|---|
| Claude | Existing `/docket-implement-next` command/skill name | Remove the public skill's unconditional `context: fork` / agent shortcut. Enter parent coordination, then dispatch the native Markdown agent with its configured pins. Verify both explicit command entry and ordinary assistant dispatch in a fresh Claude session. |
| Codex | Explicit native skill invocation, including `$docket-implement-next` where supported | Use the linked skill and 0393's native root-coordinator entry, sourced from the same role contract as TOML registration. Bracket that entry with the gate; an ordinary registered-child launch is not a substitute. Preserve the adapter's top-level collaboration requirements, working directory, and permission context. Do not infer missing dispatch from a nested tool inventory or assume Claude slash syntax. Validate the native entry in a fresh process and record mode/version. |
| Cursor | Native explicit skill/command entry | Use Cursor's skill, Markdown agent, and managed dispatch rule. Ensure the rule does not dispatch before arming, and verify no extra nesting level was introduced. Acceptance must run in Cursor IDE; a CLI proxy cannot establish IDE behavior. |
| OpenCode | Explicit skill entry; provide a Docket-owned `/docket-implement-next` command artifact when needed to expose that entry | Keep command execution in the current primary context and avoid direct subagent targeting or forced subtask execution at command expansion. Then dispatch the existing native Markdown implementer. Honor XDG roots and installer ownership; do not overwrite a foreign command or change provider permissions. |

The implementation must use each adapter's current roots and generation machinery, including any shared skill directories. It must not write incompatible variants into a directory shared by multiple harnesses. If the installed host/version cannot perform the proposed parent-to-native-worker transition, record that concrete capability failure and revisit the adapter design; do not silently exclude the harness or route it through a different host.

Official documentation checked on 2026-09-07 supports the design direction, but does not constitute live acceptance: [Claude's skill execution modes](https://code.claude.com/docs/en/skills), [Codex's explicit skill invocation](https://learn.chatgpt.com/docs/build-skills), [Cursor's skills](https://cursor.com/docs/skills), and [OpenCode's command agent/subtask controls](https://opencode.ai/docs/commands/).

## Affected maintained surfaces

- `skills/docket-implement-next/SKILL.md` and its worker reference: explicit entry/worker boundary, one canonical implementation procedure, request and context propagation.
- `agents/docket-implement-next.md` and generated wrappers: assigned worker charter, unchanged pins and self-recursion protection.
- `cursor-rules/run-gate.md`, `internal/harness/dispatch.go`, and generated repository instruction surfaces: one reachable pre-dispatch and post-return gate path for commands and ordinary dispatch.
- Relevant `internal/harness/{claude,codex,cursor,opencode}` renderers, installer targets, and owned-artifact upgrade/check handling. Add only command artifacts needed by the matrix; preserve unrelated files and settings.
- Embedded asset generation, inventories, goldens, maintained invocation documentation, and regression guards affected by moving the worker body or changing public entry metadata.

No new production CLI operation or ownership-store schema is planned beyond consuming 0393's existing entry operation. Its landed catalog exposure and request schema must be usable through the current convention; include focused integration fixes if its older branch lacks that exposure. If investigation exposes a missing ownership primitive, stop and reconcile this design instead of constructing a parallel attribution mechanism.

Record an ADR for the command-entry/worker separation and its relationship to ADRs 0024, 0026, 0060, 0084, 0100, 0103, and 0111. Supersede only conflicting decisions, preserving their historical text and the conservative ownership invariant. ADR-0111 remains the claim-proof authority; ADR-0103 governs Codex's required root entry.

## Acceptance criteria

### Hermetic and regression evidence

1. Derive harness coverage from `harness.Order` and assert set equality: Claude, Codex, Cursor, and OpenCode each have entry, wrapper, generation, and upgrade coverage. A missing adapter must fail the check.
2. Trace each generated entry through parent arming, one native launch, exact context delivery, successful claim proof, and keyed verdict. A strings-present test alone cannot establish this order or reachability.
3. Show that worker preload enters implementation directly, does not redispatch itself, and does not arm a second gate. Ordinary parent invocation must never execute the worker inline. Native model/effort and required downstream dispatch behavior remain intact. Codex must follow the root-coordinator posture, and guard logic deriving composition from skill contents must follow the moved worker reference rather than accidentally classify it as a leaf.
4. Exercise no-id, single-id, id-allowlist, empty eligible scope, claim contention, authorized resume, and refusal to take over a foreign claim. Entry routing preserves the existing selection and scope rules.
5. Exercise an attributed incomplete run: one permitted retry, same gate and claim binding, and no second grant under duplicate or concurrent return handling. Completion before the first verdict still resolves the correct completed change.
6. Exercise live/unfinished gate work: the existing continuation path is followed with the exact token and phase before ordinary worker work; no replacement test or fresh claim occurs and no retry is spent.
7. Exercise two overlapping command launches, distinct contexts, reversed notification order, one early completion, conflicting context, and replacement claims. Neither run may verify, resume, or retry the other's claim. Reuse change 0407's ownership tests instead of copying its implementation.
8. Exercise missing registration, arming failure, missing or malformed output, lost key, invalid context, uncertain launch outcome, failure before claim, halted runs, and child prose naming a sibling id. Every case must preserve the documented stop/observe-only boundary.
9. Exercise fresh install and upgrade for all four harnesses, both development and embedded/release assets. Changed public skills and any new command artifacts must be owned and checked by the installer; foreign files remain untouched. No stale direct-fork path may continue to shadow the new entry.
10. Mutation-test load-bearing guards by bypassing gate arming, dropping context propagation, adding a second arm/retry grant, restoring direct auto-fork, or removing one harness's coverage. Each corresponding guard must fail.

The build and finalize gates run their configured full suite through Docket's maintained Go runner. Focused tests are development feedback, not substitutes for those required gates.

### Fresh-session native acceptance

All four harnesses require their own recorded native-session evidence before claiming this feature works across the supported set. Use a disposable repository and safe deterministic fixtures; never induce failure on another agent's live claim.

For each harness, record the exact version and mode, OS/architecture, candidate source identity, fresh-process evidence, actual explicit invocation, native named-child identity, ordering of gate-before versus launch, context-to-claim proof, and the final gate decision. Verify a normal completion, a controlled incomplete run that uses at most one authorized retry, and a waiting run that continues without replacing its test. Check ordinary assistant dispatch as well as user command entry. The concurrency/isolation cases remain required hermetic coverage.

Cursor evidence must come from the IDE. A run through another harness, a stale registry, a simulated transcript, a generated-file golden, or a missing observation does not count as a native pass. If the implementing environment lacks a host, the PR and results record name that pending acceptance explicitly; the harness remains in scope.

## Dependencies and scope boundaries

- Change 0407 is done and supplies the durable dispatch-to-claim binding. Reuse that primitive and ADR-0111.
- Change 0393 is implemented and awaiting merge at grooming time. It supplies Codex's coordinator-capable native entry. Implementation waits until it is done, then reconciles its landed contract and catalog exposure. This dependency does not remove Codex from the delivery or acceptance matrix.
- Changes 0334, 0359, and 0371 provide related gate, continuation, and native-dispatch context. Change 0405 owns the separate test-drive prepare-scope/start handshake investigation.

Out of scope: relaxing ownership or live-claim safety; introducing another claim/token format or retry mechanism; vendor 529/overload or pre-claim transport retries; repairing change 0405's handshake; recovery across machines or after loss of the parent gate key; additional harnesses; new supervisor agents or cross-harness runners; changes to merge approval or provider permissions; and implementation or build planning during grooming.
