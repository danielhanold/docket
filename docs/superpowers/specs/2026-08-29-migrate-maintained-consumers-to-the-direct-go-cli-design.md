<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0369 — Migrate retained lifecycle consumers to typed Go operations](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-30-0369-migrate-maintained-consumers-to-the-direct-go-cli.md)**
<!-- docket:backlink:end -->

# Migrate Retained Lifecycle Consumers to Typed Go Operations

## Summary

Change 0369 migrates only the maintained planning, maintenance, implementation, and finalize
consumers whose required behavior is already represented by a public typed Go operation or is an
atomic side effect of one. It is a consumer adaptation, not a capability expansion.

The change deliberately leaves three other classes alone: native host dispatch belongs to 0371,
the retirement of explicitly deferred Go-v1 features belongs to 0372, and physical Bash deletion
belongs to 0370. The merged state is an intentionally mixed but independently green transition:
retained lifecycle work uses Go directly while the remaining Bash paths are frozen for their
named follow-ups.

## Context and correction

The earlier design assumed every maintained Bash caller implied a missing Go verb. That was
incorrect. Go v1 intentionally excludes cross-harness delegation, opportunistic stub minting,
automated learning-index and harvest machinery, and terminal publishing. Conversely, ADR-index
rendering is already owned atomically by the supported ADR transactions and needs no standalone
verb.

This design therefore classifies behavior before editing it. A textual Bash reference is evidence
to inspect, not permission to invent a replacement operation.

## Goals

- Route retained lifecycle consumers through existing public typed Go commands.
- Preserve their ordering, durable identifiers, conflict/wait/halt outcomes, and failure posture.
- Remove caller-owned follow-up work already guaranteed atomically by a Go transaction.
- Update canonical executable instructions and regenerate their embedded or installed copies.
- Prove migrated consumers do not fall back to the Bash facade.
- Leave a coherent, reviewable intermediate state for 0371, 0372, and 0370.

## Non-goals

This change does not:

- add or expand a Go command;
- add `runner-dispatch`, `mint-stub`, an ADR-index verb, a learning-index verb,
  `terminal-publish`, or `mark-publish-deferred`;
- add a forwarding shim or reconstruct lifecycle behavior with raw Git, GitHub, or shell edits;
- migrate native named-agent dispatch or generated dispatch blocks;
- retire automatic capture, learning automation, terminal publication, or their configuration;
- establish the final repository-wide no-callers seal;
- delete the facade, helpers, legacy runner, or mechanism-only tests;
- alter release, rollback, or real-host self-test work; or
- rewrite archived or otherwise point-in-time records.

## Preconditions and abort boundary

Implementation begins from merged 0318. The source checkout's implemented command and JSON
contracts are authoritative; exact spellings and fields are resolved during planning.

The run halts for human re-grooming if reconciliation finds that any proposed migration requires:

- a new or behaviorally expanded public operation;
- a bespoke compatibility adapter rather than an invocation/structured-output adaptation;
- native-dispatch generator work;
- retirement of a deferred feature;
- a repository-wide final absence invariant; or
- substantial independent redesign in more than one workflow family.

Unmapped candidates are left unchanged and reported. Partial behavior must never be smuggled into
0369 merely to make a scan green.

## Consumer classification

Every maintained executable candidate belongs to exactly one class.

### Class A — existing typed Go operation

The required effect is completely represented by an implemented public command. The consumer is
migrated to the repository's established source-resolved Go invocation and uses structured output
when a machine branches on the result.

Eligible examples include implemented change, plan, board, workspace, run-gate, review, archive,
ADR, learning-record, maintenance, recovery, and finalize operations only where the public command
fully covers the caller's need.

### Class B — native host dispatch

The consumer launches a registered agent. It remains untouched for 0371, which owns the shared
native-dispatch policy, four host adapters, generated blocks, and host-specific proof.

### Class C — intentionally deferred capability

The consumer activates automatic capture or minting, automated learning-index/harvest/capacity/
promotion work, terminal publication, or publication-deferral markers. It remains visible and
frozen for retirement in 0372.

### Class D — transaction-absorbed behavior

The consumer performs a separate follow-up already guaranteed by a supported Go transaction. The
redundant call is removed, not replaced. The known example is standalone ADR-index rendering after
ADR record, reverse, or supersede; transaction tests must prove atomic index ownership before the
caller step is deleted.

### Historical/non-executable or unresolved

Archived changes, accepted specifications, results, ADR prose, learnings, historical fixtures, and
genuine explanation are not active callers. A maintained instruction in Markdown can be executable
and is not exempt merely because it looks like documentation. Ambiguous or partial mappings fail
closed and are reported rather than edited.

## Inventory

Planning derives candidates from syntax and behavior, not a filename allowlist. Discovery covers:

- direct or variable-composed facade invocation;
- top-level production-script execution or runtime/helper sourcing;
- commands embedded in skills, agent definitions, templates, Go strings, Markdown, YAML, or JSON;
- canonical generators and their checked-in/embedded/installed products; and
- instructions that direct an agent to execute a legacy path.

For each candidate, the build evidence records canonical ownership, generated status, required
effect, classification, corresponding Go operation or absorbing transaction, follow-up owner for
Class B/C, and disposition. Search failures or unknown classifications are not absence evidence.

## Invocation and data contracts

Repository validation invokes the Go CLI from the source checkout so it cannot pass against a
stale installed binary. Human-facing installed-tool instructions may continue to use the installed
`docket` command where that is the intended product contract.

Where an operation accepts structured input, callers use that typed request rather than editing
metadata through shell text processing. Automation consumes the documented JSON envelope and
retains load-bearing IDs such as entity versions, request IDs, gate keys, handoff IDs, workspace
identities, and typed outcomes.

Callers must:

- distinguish domain outcomes from process failure;
- reject malformed or incomplete machine output;
- preserve waiting, halted, conflict, continuation, and retry semantics;
- propagate failure without a Bash fallback; and
- stop caller-owned secondary mutations after a failed transaction.

Human-only display paths may use human output when no later branch parses it.

## Transaction ownership

The public transaction is the sole owner of the metadata and derived views it promises to change
atomically. A consumer must not repeat board, artifact-link, ADR-index, or other secondary mutation
already included in the transaction.

For each removed Class D call, existing or focused tests prove that primary and derived changes
commit together, failure leaves no partial state, and retry/CAS behavior remains correct. This
change may fill a missing assertion but does not redesign the transaction.

## Workflow-family scope

### Planning

Migrate maintained authored-change, grooming, plan/backlink, dependency, board, and ADR instructions
only where an exact public operation exists. Automatic capture is excluded. Redundant standalone
ADR index work is removed after atomic-ownership proof.

### Maintenance

Migrate retained status, board, reclaim, health, reconciliation, and metadata-maintenance calls
with exact Go equivalents. Automated learning indexing, harvesting, capacity, and promotion are
excluded. Manual learning record/update remains eligible when already supported.

### Implementation lifecycle

Migrate retained claim, reconcile, workspace, gate, review-state, archive, recovery, and related
calls with exact public equivalents. Agent launch transport and its generated content remain
unchanged for 0371.

### Finalize

Migrate supported metadata-only closeout and cleanup through existing finalize transactions.
Terminal publication and deferred-publication marking remain visibly excluded for 0372.

## Canonical and generated content

Edits begin at the canonical skill, template, generator, or installer source. Existing generation
produces checked-in and embedded copies. Managed-block order and balance are validated before
rewrite, unrelated user content is preserved, and a second unchanged generation must produce no
diff. Native dispatch regions remain owned by 0371 even when adjacent to lifecycle instructions.

## Testing

Focused contract tests cover each migrated invocation shape: correct public operation, valid typed
input, structured-output validation, durable identifier propagation, failure propagation, and no
Bash fallback.

Representative workflow tests cover planning, maintenance, implementation lifecycle, and
metadata-only finalize. Transaction-absorption tests cover every removed duplicate step, including
ADR-index consistency and atomic failure behavior.

A stage-local shape-derived guard covers only the Class A/D surfaces migrated here. Restoring a
representative legacy invocation in each workflow family must make it fail. The guard explicitly
does not claim repository-wide zero callers and must permit the Class B/C and frozen 0370 surfaces.

Generation tests prove canonical/product correspondence, isolated installation or rendering where
applicable, and clean repeat generation.

The authoritative full-suite gate is:

```sh
go run ./cmd/docket development test
```

It runs from source. `scripts/run-tests.sh` remains the frozen parity oracle. Budget screening and
serial-confirmed breach lines retain their documented meanings.

## Sequencing

1. Derive and classify the candidate inventory.
2. Quantify the actual Class A/D surface and apply the abort boundary.
3. Verify every mapped public command or absorbing transaction.
4. Migrate canonical planning and maintenance consumers.
5. Migrate canonical implementation and metadata-only finalize consumers.
6. Remove proven Class D duplicate work.
7. Regenerate maintained products twice.
8. Run focused contract, workflow, mutation, generation, and full-suite tests.
9. Merge 0369 before starting 0371.

## Acceptance criteria

1. A shape-derived inventory classifies every candidate inspected by this change.
2. Every edited consumer has an exact pre-existing public Go operation or proven absorbing
   transaction.
3. In-scope planning, maintenance, implementation-lifecycle, and metadata-only finalize consumers
   invoke supported Go operations directly.
4. Machine consumers use and validate the existing structured contracts.
5. Durable conflict, waiting, halt, continuation, and retry outcomes are preserved.
6. Failures propagate without Bash fallback or caller-owned partial mutation.
7. Class D follow-up work is removed only after atomic-ownership tests pass.
8. No Go operation, shim, raw-mutation substitute, or new orchestration policy is introduced.
9. Canonical sources and generated/embedded copies agree after deterministic repeat generation.
10. Native agent dispatch remains untouched for 0371.
11. Automatic capture, learning automation, terminal publication, and marker paths remain for 0372.
12. Deferred configuration and point-in-time history are unchanged.
13. The Bash facade, frozen parity runner, and deletion-owned mechanism tests remain for 0370.
14. A stage-local mutation-tested guard detects restored legacy calls in each migrated workflow
    family without asserting repository-wide zero callers.
15. Focused tests cover typed input, structured output, failure propagation, transaction absorption,
    and absence of fallback.
16. Unmapped candidates remain unchanged and are recorded as discrepancies.
17. `go run ./cmd/docket development test` passes with no unresolved authoritative serial-confirmed
    budget breach.
18. The PR merges independently and leaves a coherent input for 0371, 0372, and 0370.

## Size verdict

This design fits one autonomous implementation PR only under the stated abort boundary. The live
surface is concentrated in canonical workflow instructions and their generated copies; it adds no
production operation, host adapter, deferred-feature retirement, global seal, or deletion work.
Raw repository match counts do not represent independent edit sites because history, frozen code,
and generated copies dominate them. Reconciliation must nevertheless quantify Class A/D before
planning and halt if the work is more than invocation/structured-output adaptation across the four
coherent workflow families.
