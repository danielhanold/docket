<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0334 — Make Docket dispatch minimal, non-recursive, and mechanically gated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0334-claude-md-global-and-project-copies-out-of-sync-on-finalize-a.md)**
<!-- docket:backlink:end -->

# Minimal, non-recursive, mechanically gated dispatch contract — design (change 0334)

## Summary

Change 0334 consolidates two related defects into one high-priority fix:

1. A dispatched `docket-*` wrapper can read the parent-facing dispatch rule and dispatch another
   instance of itself, producing recursive agent trees.
2. The always-loaded `AGENTS.md`/`CLAUDE.md` block duplicates both the native agent registry's
   descriptions and a hand-executed run-gate algorithm, creating unnecessary context weight and
   additional drift surfaces.

Change 0294's proposed scope is absorbed here. The resulting design has four parts:

- Native harness agent definitions are authoritative for agent names and descriptions; the
  always-loaded block no longer repeats the 17-agent roster.
- Every generated wrapper receives an exact-name self-recursion guard through one shared generator
  path.
- The `docket-implement-next` run gate keeps its existing safety policy but moves attribution,
  durable state, retry accounting, and foreground/detached handling behind a compact facade.
- The hand-authored global `~/.claude/CLAUDE.md` dispatch copy is retired, leaving the generated
  per-repository surface as the parent-facing source for Claude, Codex, and OpenCode. Cursor retains
  its intentionally distinct routing rule but uses the same gate facade.

The change record remains `type: fix` and moves to `priority: high`. Change 0294 is related absorbed
scope and should be retired as killed with that reason after this replacement spec is accepted;
those are PM operations, not implementation behavior.

## Problem

### Parent instructions are reaching the child they dispatched

The generated parent-facing rule says that a requested Docket workflow should run through its
matching model/effort-pinned agent rather than inline. That rule preserves the harness-specific
wrapper selected under ADR-0016, including its dispatch contract and any skill preload.

The same rule can appear in a dispatched wrapper's context. Without a depth or identity boundary, a
wrapper already running as `docket-status` can interpret "dispatch `docket-status`" as an instruction
to dispatch another `docket-status`. Change 0317 captured the live result in Claude:

```text
docket-status
└── docket-status
    └── docket-status
        └── actual status charter
```

This is historical live evidence, not a standing claim about every current harness version or
invocation mode.

A broad "top-level only" guard would be incorrect. Docket controllers legitimately compose with
different agents: `docket-implement-next` dispatches status, ADR, plan, build, and review roles;
auto-groom dispatches a critic; finalize dispatches conflict resolution and integration repair. The
forbidden edge is only a wrapper dispatching another instance of its own exact agent for the
assignment it already holds.

### The always-loaded block restates native registry content

The generated block repeats the description of every Docket agent even though those descriptions
already live in the generated native agent definitions and are exposed through each harness's agent
registry.

That repetition creates three costs:

- every session pays for the same descriptions twice where the harness already exposes them;
- edits can drift between the registry and the prose roster; and
- the parent-facing instruction becomes harder to inspect because the operative dispatch rule is
  buried inside a large catalogue.

The parent needs a routing rule, not a second registry.

### The run gate exposes mechanism as prose

The parent-facing `docket-implement-next` gate currently asks the model to:

- preflight and snapshot claims;
- preserve an epoch outside the shell call;
- distinguish foreground, attributed detached, and unattributed launches;
- apply the three attribution filters;
- count candidates;
- invoke `verify-run`;
- interpret its report;
- permit at most one re-dispatch; and
- remember which terminal states prohibit further action.

Those are procedural mechanics already substantially present in `runner-dispatch.sh` and
`verify-run.sh`. Restating them in always-loaded prose accumulates its own guards and makes caller
variance part of the safety boundary. The policy remains necessary; the model should not be its
interpreter.

### The global Claude copy is a second source

A hand-authored `~/.claude/CLAUDE.md` copy supplied the dispatch rule to an unsynced change-0317
fixture. It now carries a manual draft recursion guard, and the originally cited finalize-agent
description mismatch is no longer present. Those facts do not make the copy authoritative; they
show that it has already evolved independently.

The global edit is a temporary stopgap. The original transcript remains valid historical evidence
of the recursion and of the risks created by two independently maintained copies.

Docket must not edit personal global instructions. Removing this copy is a maintainer-owned
acceptance precondition.

## Decision

### 1. Replace the roster with a compact parent routing contract

The generated `docket:dispatch` block for `AGENTS.md`/`CLAUDE.md` will no longer enumerate or
describe all Docket agents.

Its routing rule will state, in substance:

> When a requested Docket workflow has a registered same-name `docket-*` agent, dispatch that agent
> instead of running the workflow inline. The harness's native agent registry is authoritative for
> agent names, descriptions, and availability. If no same-name agent is registered, do not invent
> one; follow the workflow's own inline or unavailable-capability contract.

This preserves the wrapper boundary established by ADR-0016 without duplicating the registry.

"Registered" means actually discoverable through the active harness's capability surface.
Availability is resolved rather than inferred from a hard-coded tool name, following ADR-0059.
Internal controllers continue to select build, review, critic, ADR, status, plan, repair, and
resolver agents through their own active charters; the parent block does not catalogue those
composition edges.

The compact block may retain a small number of essential never-rules for
`docket-implement-next`, but only where the facade's report vocabulary cannot encode the
instruction completely. It must not restate attribution filters, timestamps, candidate-selection
steps, or retry bookkeeping.

### 2. Put an exact-name recursion guard in every generated wrapper

The self-recursion guard belongs in the generated wrapper, where the wrapper's exact identity is
known, rather than in the parent roster.

One shared wrapper-emission path will inject wording equivalent to:

> You are already running as `<exact-agent-name>`. Carry out this wrapper's assigned charter
> directly. Do not dispatch another `<exact-agent-name>` merely to perform the current assignment.
> Dispatches to different agents explicitly required by the active charter remain required.

The emitter substitutes the exact generated wrapper name, such as `docket-status` or
`docket-review-standard`. The guard must not rely on:

- a `name:` field being present in every harness format;
- the wrapper having a preloaded skill;
- the wrapper's skill name matching its agent name; or
- the model inferring its identity from surrounding prose.

This wording therefore works for wrappers with no skill preload and for build/review wrappers that
share a role skill under wrapper-specific names.

The guard prohibits only this edge:

```text
docket-X → docket-X for the assignment docket-X already holds
```

It explicitly preserves this edge:

```text
docket-X → docket-Y required by docket-X's active charter
```

The shared emitter must cover every harness-specific wrapper renderer. A new harness renderer must
consume the same guard source rather than copying its wording.

### 3. Move the run gate behind a durable facade

The run gate remains parent-facing because the child that stops early cannot reliably verify its
own terminal disposition. This boundary is unchanged from ADR-0078.

The shell facade will expose two operations, with final command spelling allowed to follow the
repository's existing `docket.sh` conventions:

```text
docket.sh gate-before implement-next
docket.sh gate-verdict <opaque-key>
```

If no valid key was obtained before dispatch, the caller uses:

```text
docket.sh gate-verdict --unattributed [<hint-id>...]
```

#### `gate-before`

`gate-before implement-next` will:

1. re-sync the metadata worktree to fresh origin state;
2. read the current in-progress claim set;
3. capture the dispatch epoch after the before-read;
4. create a durable, concurrency-isolated gate record;
5. persist the target wrapper, repository identity, before-set, epoch, and retry state; and
6. print one opaque key.

The parent retains only the key. It must not retain, parse, or reconstruct timestamps or claim
sets.

If a key cannot be created, the facade reports that the gate is unarmed. Dispatch may still proceed
under the existing tolerant posture, but its return is necessarily handled through unattributed
mode and can never authorize a re-dispatch.

#### Durable state

Gate state will reuse or generalize the repository-wide dispatch-record machinery already owned by
`runner-dispatch.sh`, including its machine-local state root, safe key validation, atomic
adjacent-file replacement, and retention lifecycle. It will not write tracked repository metadata.

Each record contains at least:

- schema/version;
- repository identity;
- target agent (`docket-implement-next`);
- creation time and dispatch epoch;
- fresh-origin before-set;
- attributed change id, once established;
- retry state (`unused` or `consumed`);
- latest gate disposition; and
- terminal/cleanup state.

The opaque key is a lookup token, not encoded state. The caller cannot infer an id, timestamp, path,
or retry permission from it.

State transitions must be atomic and concurrency-safe. Two simultaneous `gate-verdict` calls for
one key may not both receive retry permission. Retry permission is consumed when the facade emits
it, before control returns to the caller. If the caller then fails to dispatch, the safe result is
a lost retry, not a second permit.

Records survive shell calls, parent-process restarts, and detached notifications. A repeated
verdict call is idempotent except for the intentional transition that consumes the one retry
permit. Terminal records may be pruned under the existing dispatch-record retention policy; live or
nonterminal records must not be age-pruned merely because the originating process exited.

A record loaded in the wrong repository, with an unsupported schema, with missing attribution
inputs, or with malformed state produces a stop/unavailable report. The facade never reconstructs
missing inputs from model-supplied values.

#### `gate-verdict` with an attributed key

On the first verdict for a valid key, the facade:

1. re-syncs metadata to fresh origin state;
2. reads current in-progress claims;
3. applies ADR-0075/ADR-0084's existing three filters:
   - absent from the fresh before-set;
   - readable `claimed_at`; and
   - `claimed_at >= dispatch_epoch`;
4. counts the surviving candidates;
5. stores the single attributed id when exactly one survives; and
6. delegates the run predicate to `verify-run` rather than reimplementing it.

Candidate handling remains conservative:

- zero candidates: no claim is attributable; the gate is done with no corrective action;
- one candidate: verify that id; and
- more than one candidate: attribution is ambiguous, so stop and report without verifying or
  re-dispatching any candidate.

The accepted ADR-0075 residual remains: a run that claimed nothing while exactly one concurrent run
claimed inside the window can still be misattributed. This change moves the algorithm; it does not
claim to solve that protocol limitation.

Once an id is attributed, subsequent calls for the same key verify that stored id directly. They do
not repeat attribution against a later claim set.

`verify-run`'s report-line distinctions remain authoritative:

- `run-complete <id>`
- `run-unclaimed <id>`
- `run-incomplete <id> <unmet...>`
- `run-halted <id>`
- `run-waiting <id> <handoff-id> <phase>`

The facade adds an action prefix without rewriting the underlying verdict:

```text
gate-done <key> no-attributable-claim
gate-done <key> run-complete <id>
gate-done <key> run-unclaimed <id>
gate-retry-once <key> run-incomplete <id> <unmet...>
gate-stop <key> run-incomplete <id> <unmet...>
gate-stop <key> run-halted <id>
gate-stop <key> run-waiting <id> <handoff-id> <phase>
gate-stop <key> ambiguous-claims <id...>
gate-stop <key> gate-unavailable <reason-token>
```

These are the complete parent-facing attributed-mode action classes.

Their meanings are:

- `gate-done`: no further gate action.
- `gate-retry-once`: dispatch the same `docket-implement-next` wrapper once for the named id and
  unmet conjuncts, retaining the same key; no other target or new change is authorized.
- `gate-stop run-incomplete`: the one permit was already consumed and the run remains incomplete;
  report loudly.
- `gate-stop run-halted`: a human is needed; never re-dispatch.
- `gate-stop run-waiting`: report the exact handoff and phase and stop. A fresh implement-next
  dispatch is not a continuation. Resumption, if available, must use the separate exact-handoff
  mechanism.
- `gate-stop ambiguous-claims`: attribution was unsafe; never select one candidate.
- `gate-stop gate-unavailable`: the facade could not establish a trustworthy result; never
  improvise the missing procedure.

ADR-0088's distinction is preserved: halt and waiting are run dispositions, independent of how
they were discovered. The stable report line, not a child's prose and not a generic process exit
code, governs the parent action.

A malformed, absent, or unknown facade report is a stop, never permission to retry.

#### Unattributed mode

Unattributed mode covers:

- slash-command or notification-first runs for which no before-record exists;
- a dispatch launched before `gate-before`;
- a failed or lost `gate-before` result; and
- any session that receives a child completion without the opaque key.

A supplied id is a hint to verify, never attribution evidence. When ids are supplied, the facade
verifies each. When none are supplied, it re-syncs and verifies each current in-progress id. It
emits one of:

```text
gate-observe run-complete <id>
gate-observe run-unclaimed <id>
gate-observe run-incomplete <id> <unmet...>
gate-observe run-halted <id>
gate-observe run-waiting <id> <handoff-id> <phase>
gate-observe no-current-run
gate-observe gate-unavailable <reason-token>
```

No `gate-observe` report can authorize a retry. This preserves ADR-0084's rule that re-dispatch
permission depends on mechanical attribution capability, not on foreground/detached launch labels
or on an id named by the child.

#### Compact parent instruction

The always-loaded gate text will state only:

1. Before dispatching `docket-implement-next`, call `gate-before implement-next` and retain its
   opaque key.
2. After the run returns or its detached completion arrives, call `gate-verdict` with that key;
   without a key, use unattributed mode.
3. Obey the facade's `gate-*` report exactly.
4. Only `gate-retry-once` authorizes one same-agent retry; every `gate-stop` and every
   `gate-observe` forbids it.
5. Never hand-reimplement attribution or infer permission from child prose, launch shape,
   timestamps, ids, or process exit codes.

Foreground and attributed detached calls therefore share one state machine. "Detached dispatch"
is no longer a separate prose algorithm.

### 4. Keep parent-facing surfaces single-source without conflating Cursor

For Claude, Codex, and OpenCode, the managed `docket:dispatch` block written into the repository's
`CLAUDE.md`/`AGENTS.md` surface is the generated parent-facing source. ADR-0078's
one-physical-instructions-file policy remains unchanged: a symlinked pair receives one physical
managed block, not two diverging copies.

`tests/test_sync_agents_claude_surface.sh` already proves that a Claude-only repository receives a
real `CLAUDE.md` containing the managed block. That is settled structural evidence, not an open
design question.

Cursor remains intentionally different. Its always-applied dispatch rule is a distinct routing
surface and must not be described as consuming the `AGENTS.md`/`CLAUDE.md` managed block. It will,
however, carry the same compact gate trigger and call the same facade, so gate policy and report
vocabulary do not fork by harness.

The source layout must make this distinction explicit:

- one shared compact gate text or emitter consumed by the managed block and Cursor rule;
- one shared wrapper-guard emitter consumed by every wrapper renderer;
- no copied roster; and
- no copied attribution procedure.

### 5. Retire the global Claude copy at the merge gate

Docket will not inspect, edit, truncate, or delete `~/.claude/CLAUDE.md`.

After the implementation has synced the repository assets, the maintainer must remove the
hand-authored global Docket dispatch block. This is a merge-gate acceptance precondition, not an
optional follow-up. External behavioral acceptance then runs only after:

1. the project assets are freshly synced;
2. the global block is absent;
3. all stale harness processes that could retain process-start instructions are terminated; and
4. wholly fresh harness processes are started.

The current global draft guard and the corrected finalize descriptions are acknowledged as
temporary current state. They do not replace this acceptance condition.

## Failure posture

The facade prefers false negatives over unsafe action:

- unreadable or malformed state: stop;
- no opaque key: observe only;
- unreadable `claimed_at`: exclude the candidate;
- multiple candidates: stop as ambiguous;
- missing or malformed `verify-run` report: stop;
- concurrent verdict calls: at most one retry permit;
- retry permit emitted but not used: do not restore it;
- process restart: reload durable state;
- waiting continuation: surface the handoff, never mint a fresh run;
- halt: surface the human checkpoint;
- second incomplete verdict: stop loudly; and
- unknown report vocabulary: stop.

No failure path chooses a candidate, invents an agent, reconstructs a timestamp, trusts the child's
completion claim, or grants an additional retry.

## Acceptance

### Automated structural acceptance

Tests must prove all of the following.

#### Compact parent surface

- The emitted managed block contains the compact registered-agent routing rule.
- It does not contain the duplicated 17-agent description roster.
- It identifies the native registry as authoritative for names, descriptions, and availability.
- It includes the short `gate-before`/`gate-verdict` trigger and no hand-executed attribution
  algorithm.
- The Cursor rule consumes the same gate facade text without being asserted to share the managed
  `AGENTS.md`/`CLAUDE.md` block.
- Existing managed-marker order/balance, idempotency, `--check`, and machine-neutrality tests remain
  green.
- `tests/test_sync_agents_claude_surface.sh` remains green.

#### Wrapper recursion guard

- Every generated Docket wrapper in every supported harness emitter contains its own exact agent
  name in the self-recursion guard.
- The guard does not refer to "your preloaded skill."
- The same-agent prohibition and different-agent preservation clause are tested together.
- Removing the exact-name clause, broadening the rule to prohibit all nested dispatch, or removing
  the different-agent clause makes the test fail.
- The expected wrapper set is derived from the generated definitions rather than maintained as a
  second hand-written list.
- Removing the shared guard injection from any renderer makes that renderer's test fail.

The guard is code for test purposes: its structural assertions must be mutation-sensitive, not
presence-only decoration.

#### Gate facade

Hermetic facade tests cover:

- foreground attributed use;
- detached attributed use across separate processes;
- unattributed use with and without hint ids;
- zero, one, and multiple candidate claims;
- each three-filter rejection independently;
- `run-complete`;
- `run-unclaimed`;
- first `run-incomplete`;
- second `run-incomplete`;
- `run-halted`;
- `run-waiting`;
- exact preservation of handoff id and phase;
- one-retry authorization and atomic consumption;
- repeated verdict calls after retry consumption;
- two concurrent callers competing for one retry permit;
- multiple concurrent gate records in one repository;
- wrong-repository and malformed-key rejection;
- partial/corrupt/unsupported records;
- process restart between before, first verdict, retry, and second verdict;
- terminal-record cleanup;
- nonterminal-record retention;
- cleanup racing an active verdict;
- exact `gate-done`, `gate-retry-once`, `gate-stop`, and `gate-observe` vocabulary; and
- fail-closed behavior for an unknown or malformed underlying verdict.

The tests must exercise the post-pass state, not merely prove that a command was invoked. In
particular, a retry test must prove the durable record is consumed and cannot mint a second permit
after restart or concurrent access.

Existing `verify-run` verdict tests and `runner-dispatch` attribution tests remain green. Shared
logic should be extracted or delegated so those consumers do not acquire subtly different
predicates.

#### Repository verification

- `sync-agents.sh --check` passes.
- Existing surface, marker, dispatch, run-gate, machine-neutral, and size-budget guards pass.
- The compact block lowers the relevant always-loaded size bound rather than merely raising it.
- The full suite resolved from `finalize.test_command` passes, including its budget diagnostics.

### External harness acceptance

Generated agent definitions and always-loaded instructions are process-start artifacts. They cannot
be certified behaviorally in the editing session.

After syncing assets and deleting the global Claude block, terminate stale processes and run fresh
sessions in:

- Claude;
- Codex;
- Cursor; and
- OpenCode.

For every observation, record:

- harness name;
- exact harness version;
- invocation mode;
- whether entry used native skill invocation, named-agent dispatch, slash command, or another route;
- relevant generated artifact paths; and
- the observed agent tree.

Each harness must demonstrate:

1. Native agent discovery exposes the same-name wrapper without the prose roster.
2. A request for a registered Docket workflow dispatches exactly one entry wrapper.
3. That wrapper does not dispatch another instance of itself for the current assignment.
4. Its assigned charter runs exactly once.
5. The configured model/effort pin and wrapper contract hold, as scoped by ADR-0016 and the observed
   harness.
6. A representative controller can still dispatch a different required child, proving the guard
   did not flatten legitimate composition.
7. `docket-implement-next` can be bracketed by the facade in a foreground/attributed path.
8. A detached completion can be resolved with the same opaque-key facade.
9. A notification-first or otherwise keyless completion enters observe-only mode and never
   receives retry permission.
10. No behavior depends on the deleted global `~/.claude/CLAUDE.md` block.

No observation may be generalized beyond the recorded harness version and mode. A failed native
discovery check is a release blocker because removal of the roster depends on it.

The acceptance record should retain change 0317's transcript as historical evidence while clearly
separating that older observation from the new fresh-process results.

## Consequences

### Benefits

- Same-agent recursion is prevented at the only layer that knows the exact wrapper identity.
- Legitimate cross-agent composition remains explicit and mandatory.
- Native registries become the sole authority for names and descriptions.
- Parent-facing context becomes materially smaller.
- Foreground, detached, and unattributed gate behavior share one implementation.
- Retry accounting survives tool boundaries and process restarts.
- Global/project description drift is eliminated by retiring the global copy.
- Gate behavior becomes mechanically testable instead of depending on a model reproducing an
  attribution algorithm.
- Future attribution-policy changes have one implementation and one report vocabulary.

### Costs

- The facade gains durable state, concurrency control, cleanup, and schema responsibilities.
- Native registry discoverability becomes a release-time dependency that must be verified per
  harness version/mode.
- A lost retry after permit emission is accepted as the fail-safe outcome.
- The known single-concurrent-claim attribution residual from ADR-0075 remains.
- Fresh-process four-harness acceptance remains a human checkpoint because repository tests cannot
  prove external harness behavior.

## Out of scope

- Changing `docket-implement-next`'s terminal postcondition.
- Changing `verify-run`'s `run-*` predicates or precedence.
- Solving ADR-0075's exactly-one-concurrent-claim residual through a child-reported claim id.
- Changing model or reasoning-effort selections.
- Reclassifying ADR-0059's capability tiers.
- Unifying Cursor's routing surface with `AGENTS.md`/`CLAUDE.md`.
- Editing a user's personal `~/.claude/CLAUDE.md`.
- Changing the semantics of `run-waiting` or implementing a new handoff-resumption channel.
- Pruning unrelated always-loaded rules tracked outside absorbed change 0294.
- Rewriting historical change 0317 evidence to match current harness behavior.

## Assumptions

1. **Native registry discoverability:** Each supported harness can expose registered Docket agent
   names and descriptions without the prose roster. This is a design dependency subject to fresh
   version/mode-specific acceptance; failure in any supported harness blocks roster removal for that
   harness rather than licensing an invented fallback list.
2. **Wrapper identity:** Every generated wrapper can receive an exact literal agent name from the
   generator even when its harness file format lacks a native `name:` field. The guard relies on
   this generated literal, not on runtime introspection.
3. **Wrapper charter wording:** "Carry out this wrapper's assigned charter" is applicable to
   wrappers with no preloaded skill and to wrappers sharing a role skill under a different wrapper
   name.
4. **Facade state authority:** The existing repository-wide dispatch-record location and lifecycle
   can be safely generalized for parent run-gate records. The implementation may refine the
   internal file layout, but it must preserve opaque keys, repository scoping, atomic transitions,
   restart durability, concurrent isolation, and safe cleanup.
5. **Retry consumption:** Consuming retry permission when it is emitted is preferable to consuming
   it after a child launch, because a lost retry is recoverable by a human while two concurrently
   authorized retries are not.
6. **No global copy during acceptance:** Behavioral acceptance is valid only after the hand-authored
   global Docket dispatch block has been removed and stale harness processes have been terminated.
7. **Structural versus external evidence:** Repository tests can prove emitted content,
   state-machine behavior, mutation sensitivity, and facade vocabulary. They cannot prove that a
   particular external harness version loads an artifact, exposes native agents, obeys the wrapper
   instruction, or applies the configured pin; those claims require the recorded fresh-process
   checkpoint.
8. **Cursor distinction:** Cursor continues to use its own routing rule. Sharing the compact gate
   facade does not imply that Cursor reads or should read the same managed parent block as Claude,
   Codex, or OpenCode.
9. **Policy preservation:** ADR-0075, ADR-0084, and ADR-0088 remain normative for attribution, retry
   permission, halt, and waiting behavior. This design relocates those mechanics and does not
   silently revise them.
10. **Historical evidence:** Change 0317's recursive tree and the original global/project mismatch
    remain valid point-in-time evidence even though the current global draft has a guard and its
    finalize descriptions no longer differ.

## References

- ADR-0016 — harness-first agent configuration and per-harness model/effort resolution.
- ADR-0024 — Claude routing through native fork/agent dispatch and the distinction between routing
  and parent behavior.
- ADR-0059 — dispatch capability is resolved, not inferred from a tool name.
- ADR-0075 — conservative claim attribution, candidate cardinality, and terminal halt treatment.
- ADR-0078 — the parent-facing Claude gate boundary and one-physical-instructions-file policy.
- ADR-0084 — retry permission depends on mechanical attribution capability, not launch shape.
- ADR-0088 — halt disposition is independent of its discovery path.
- Change 0294 — absorbed roster de-duplication and run-gate facade scope.
- Change 0317 — historical live recursion evidence and four-harness acceptance precedent.
