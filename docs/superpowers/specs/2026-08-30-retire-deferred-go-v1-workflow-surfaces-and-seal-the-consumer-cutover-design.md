<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0372 — Retire deferred Go v1 workflow surfaces and seal the consumer cutover](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-30-0372-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consume.md)**
<!-- docket:backlink:end -->

# Retire Deferred Go v1 Workflow Surfaces and Seal the Consumer Cutover

## Summary

Change 0372 removes active workflow paths for capabilities explicitly deferred from Go v1:

- automatic change-stub capture and `mint-stub`;
- automated learning harvest, index rendering, capacity enforcement, and promotion; and
- terminal publication and `mark-publish-deferred`.

It does not implement replacements. It makes the deferral honest and enforceable: maintained
workflows stop invoking Bash-era implementations, active instructions stop promising the features,
legacy configuration and records remain readable, unsupported requests fail closed, and a final
shape-derived guard prevents active facade callers from returning.

The Bash files remain frozen until 0370. After 0372, their deletion is behaviorally safe because no
maintained workflow depends on either retained or deferred Bash behavior.

## Preconditions and assumptions

- Changes 0369 and 0371 are merged.
- Automatic capture and terminal publication remain disabled by the Go-v1 configuration boundary.
- Manual authored change creation and explicit learning record/update operations remain supported.
- ADR index maintenance is supported atomic ADR-transaction behavior and is not retired.
- Existing configuration keys, learning data, indexes, publication markers, and history remain
  parseable evidence even when their old automation is inactive.
- Unsupported requests can be rejected before mutation through existing validation/workflow seams.
- The final seal targets maintained executable paths, not point-in-time history or frozen deletion
  evidence.

The run halts for re-grooming if retiring a listed leg requires a new production subsystem, or if
the ADR audit finds a still-current decision requiring that capability to remain active.

## Goals

- Remove every maintained executable activation path for the three deferred feature families.
- Preserve configuration and repository data needed to read pre-cutover repositories.
- Produce stable, capability-specific diagnostics for unsupported requests.
- Point users and agents to supported explicit alternatives where they exist.
- Add a shape-derived, mutation-tested final consumer seal.
- Leave 0370 with physical deletion and mechanism-test removal only.

## Non-goals

This change does not:

- implement automatic capture or a Go mint operation;
- implement automated learning harvest, indexing, capacity, or promotion;
- change explicit learning record/update behavior;
- change atomic ADR-index behavior or add a standalone ADR-index command;
- redo 0369 lifecycle migration or 0371 native dispatch;
- delete Bash files, the facade, or frozen parity/deletion evidence;
- remove configuration keys or existing persisted records;
- create generic unsupported-feature infrastructure beyond these surfaces;
- perform release, tag, rollback, or real-host acceptance; or
- rewrite archived changes, accepted ADR bodies, specs, results, or learnings.

## Retirement model

Every deferred surface ends in one of three states:

1. **Supported explicit alternative:** active guidance points to an already-supported deliberate
   operation.
2. **Preserved inactive configuration:** legacy keys remain readable but cannot route execution to
   Bash or approximate the old feature.
3. **Explicitly unsupported request:** an attempt to require the automation stops before mutation
   with a stable explanation and any valid manual alternative.

No unrelated Go operation is used merely to simulate continuity.

## Retirement mapping

### Automatic capture and `mint-stub`

Remove maintained calls, hooks, and instructions that opportunistically synthesize a change stub.
Explicit authored `change create` remains the supported path when a user deliberately invokes it
with complete inputs. Work discovered during another workflow is reported, not silently minted or
discarded. Legacy auto-capture configuration remains readable and inactive; an attempt to activate
it explains that automation is deferred and names authored creation as the alternative.

### Automated learning harvest

Remove harvest from review, finalize, closeout, status, and other workflow tails. Explicit learning
record/update operations remain available. The absence of harvest is not a finalize failure, and
the workflow does not fabricate an empty harvest result. A direct request or enabled path that
requires harvest is rejected before learning mutation.

### Learning-index rendering

Remove maintained standalone renderer calls and claims that explicit learning mutations refresh
the index when they do not. Existing index bytes and learning records are preserved. The guard must
distinguish the retired learning index from supported atomic ADR-index rendering.

### Learning capacity and promotion

Remove automated capacity gates, archival/disposition actions, candidate selection, promotion
state mutation, and automatic editing of `AGENTS.md` or another graduation destination. Existing
capacity configuration and promotion-state data remain readable. Human-directed ledger stewardship
and explicit record updates remain possible; they are not presented as an automated pipeline.

### Terminal publication

Remove publication from finalize and closeout. Supported Go metadata closeout becomes the complete
automated closeout boundary. The legacy key remains parseable and cannot reactivate the Bash
publisher. A request specifically requiring published terminal artifacts stops before claiming
that outcome, even if an independently transactional metadata step could succeed.

### `mark-publish-deferred`

Remove creation/update calls and active instructions. Existing markers remain untouched as
historical or recovery data. Supported closeout neither requires nor falsely reports creation of a
new marker.

## Configuration compatibility

Legacy keys remain in the existing schema and precedence system:

- disabled values remain valid and cause no action;
- enabled stored values are not silently rewritten;
- enabling a deferred capability cannot invoke Bash;
- when execution would require the capability, the workflow returns its capability-specific
  unsupported diagnostic before that leg mutates state; and
- configuration inspection accurately distinguishes parseable legacy data from supported
  behavior.

No shadow schema or second configuration source is introduced.

## Data preservation

This change does not bulk-edit or delete existing stubs/discovery links, learning records and
promotion states, learning indexes, publication/deferred-publication evidence, configuration files,
archived changes, results, specs, or ADRs. Preservation does not imply the old automation can update
the data.

Supported explicit change creation, learning record/update, atomic ADR transactions, metadata-only
finalize, and native host dispatch retain their successful behavior.

## Failure behavior

An unsupported result must:

- occur before the deferred leg mutates state;
- name the specific unavailable capability and say it is deferred from Go v1;
- identify a supported manual alternative when one exists;
- avoid suggesting that reinstalling or retrying will make a missing verb appear;
- avoid all Bash fallback; and
- leave already completed independent transactions accurately represented.

When a formerly optional tail followed a supported transaction, remove the tail rather than turn a
successful supported workflow into a misleading overall failure. When the user explicitly asks for
the deferred outcome, never claim it occurred because an earlier supported transaction succeeded.

## Maintained-consumer inventory

Implementation derives candidates through whole-repository syntactic shapes and classifies them as:

- maintained executable workflows/scripts;
- canonical skill, agent, template, and generator sources;
- generated or embedded maintained copies;
- active tests and fixtures;
- active user/operator documentation;
- configuration/schema definitions;
- frozen parity/deletion material retained for 0370;
- point-in-time history; or
- unresolved, which blocks completion.

Markdown and similar files can contain executable instructions. Conversely, historical prose is
not an active caller. The evidence records why every changed or preserved category is safe.

Canonical sources change before their generated outputs. Existing deterministic generation is run
twice; generated dispatch content is touched only where it carries deferred-feature instructions,
not to redo 0371.

## Final cutover seal

The seal rejects:

1. maintained executable calls into the Bash facade or top-level operation scripts;
2. executable maintained references to retired operations or aliases;
3. active instructions directing an agent/user to invoke a retired operation;
4. configuration-to-execution wiring that can reactivate a deferred capability; and
5. generated/embedded maintained output that restores a prohibited caller.

The seal permits:

- historical records and accepted ADR text;
- archived specs, results, and changes;
- the frozen Bash facade and parity/deletion evidence awaiting 0370;
- schema and stored legacy configuration;
- explanatory active documentation that clearly labels a capability unsupported/deferred; and
- supported ADR-index and explicit change, learning, finalize, or native-dispatch behavior.

### Shape derivation and diagnostics

The prohibited operation set is derived from the facade/operation structure and narrowed through
the explicit retirement classification. Maintained roots and historical/frozen exclusions are
structural categories, not an enumerated caller-file list. Any narrow exception has a reason and a
test proving it cannot hide an executable caller.

A failure identifies the path, matched executable/directive shape, violated retired surface or
facade boundary, and canonical/generated classification.

## Mutation evidence

Tests inject and detect at least:

1. a direct facade invocation in a maintained workflow;
2. an auto-capture or `mint-stub` instruction;
3. an executable learning-index renderer call;
4. an automated harvest, capacity, or promotion leg;
5. a terminal-publication or marker call;
6. a prohibited call restored through generated/embedded output; and
7. configuration wiring from an enabled deferred key to Bash.

Each mutation must fail for the intended reason and restore its fixture/worktree even on assertion
failure. Negative controls remain green for history, frozen parity material, schema data,
explanatory deferred documentation, and supported ADR-index transactions.

## ADR handling

The implementation individually audits facade-era ADRs 0014, 0029, 0030, and 0033 against the
accepted Go-v1 decisions. Conflicting normative conclusions are superseded or reversed through
supported ADR transactions; history remains intact. Decisions establishing native dispatch,
gate authority, one Go-v1 metadata topology, atomic ADR-index rendering, and human-gated learning
promotion are preserved.

No ADR is bulk-dispositioned merely because its number appeared in the change record. If current
normative architecture still requires a retired capability, implementation halts.

## Documentation

Active maintained instructions explain which capabilities are deferred, which explicit alternatives
exist, why no missing replacement verb is expected, why retained configuration does not activate
the feature, and why the frozen Bash files are not a supported fallback. Historical documents remain
unchanged.

## Testing and rollout

Focused tests cover legacy configuration parsing, enabled-value failure, explicit alternatives,
ADR-index preservation, metadata closeout without publication, data/marker preservation,
capability-specific diagnostics, generation, the final seal, required mutations, and negative
controls. Fresh isolated processes/repositories are used where configuration or installation state
could leak.

The authoritative full suite is:

```sh
go run ./cmd/docket development test
```

Rollout order:

1. Confirm 0369 and 0371 are merged.
2. Derive and classify the full candidate inventory.
3. Remove deferred legs from canonical workflow sources.
4. Add fail-closed handling for legacy enabled configuration and explicit requests.
5. Regenerate maintained outputs twice.
6. Add the seal and mutation evidence.
7. Complete the ADR audit and formal dispositions.
8. Run focused and full-suite tests.
9. Merge before 0370 deletes frozen implementation material.

There is no Bash/Go dual-path compatibility window.

## Acceptance criteria

1. No maintained executable workflow invokes any of the three deferred feature families.
2. No maintained instruction presents a retired operation as supported.
3. Explicit authored Go change creation and explicit learning record/update remain supported.
4. Atomic Go ADR-index behavior remains supported and is not blocked by the seal.
5. Metadata-only finalize succeeds without terminal publication or a new deferred marker.
6. Legacy configuration remains parseable and stored values are not silently rewritten.
7. Enabled deferred configuration cannot reactivate or fall back to Bash.
8. A direct deferred-capability request fails before that capability mutates state with a stable,
   specific diagnostic.
9. Existing records, indexes, markers, configuration, and history are preserved.
10. Canonical and generated/embedded content agrees after deterministic repeat generation.
11. A repository-wide shape-derived seal rejects new executable facade callers and deferred paths.
12. Structural exclusions do not hide maintained callers and permit the documented negative controls.
13. All seven mutation classes fail for the intended reason.
14. Guard diagnostics identify path, prohibited shape, surface, and canonical/generated ownership.
15. ADRs 0014, 0029, 0030, and 0033 are individually audited and formally dispositioned where
    they conflict; accepted bodies remain unchanged.
16. Active documentation describes the honest deferred boundary and supported alternatives.
17. The PR contains neither 0369/0371 work nor physical Bash deletion.
18. The frozen facade and deletion-stage evidence remain for 0370.
19. Focused tests and `go run ./cmd/docket development test` pass with no unresolved authoritative
    serial-confirmed budget breach.

## Size verdict

This fits one autonomous PR because every edit serves one invariant: remove activation of already
deferred features and seal that state. The work is primarily deletion/simplification, compatibility
diagnostics, generated-content refresh, one guard, and its mutation evidence. It adds no replacement
feature, lifecycle adapter, dispatch subsystem, physical facade deletion, or live acceptance work.
Splitting by feature would duplicate the shared inventory, documentation, generator, guard, and ADR
work while leaving intermediate states that cannot truthfully install the final seal.
